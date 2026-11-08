package cli

import (
	"reflect"
	"testing"

	"github.com/spf13/cobra"

	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
)

// TestSelectGatewayServices 表驱动验证 verify 命令的目标筛选：
// 固定 type=gateway，--service/--server/--zone 取交集，缺省=该环境全部网关。
func TestSelectGatewayServices(t *testing.T) {
	cfg := &entity.Config{
		Servers: []entity.Server{
			{Name: "srv-node1", Zone: "node9"},
			{Name: "srv-node6", Zone: "node11"},
		},
		InfraServices: []entity.InfraService{
			{Name: "gateway-node1", Type: entity.InfraServiceTypeGateway, ServiceBase: entity.ServiceBase{Server: "srv-node1"}},
			{Name: "gateway-node6", Type: entity.InfraServiceTypeGateway, ServiceBase: entity.ServiceBase{Server: "srv-node6"}},
			{Name: "tunnel-frps", Type: entity.InfraServiceTypeTunnelServer, ServiceBase: entity.ServiceBase{Server: "srv-node1"}},
			{Name: "easytier-node1", Type: entity.InfraServiceTypeEasyTier, ServiceBase: entity.ServiceBase{Server: "srv-node1"}},
			{Name: "uptime-kuma-node1", Type: entity.InfraServiceTypeUptimeKuma, ServiceBase: entity.ServiceBase{Server: "srv-node1"}},
		},
		Services: []entity.BizService{
			{Name: "app-biz", ServiceBase: entity.ServiceBase{Server: "srv-node1"}},
		},
	}

	tests := []struct {
		name    string
		filters ServiceCmdFilters
		want    []string
	}{
		{"缺省=该环境全部网关（排除 tunnel/easytier/kuma/biz）", ServiceCmdFilters{}, []string{"gateway-node1", "gateway-node6"}},
		{"--service 单个", ServiceCmdFilters{Service: "gateway-node6"}, []string{"gateway-node6"}},
		{"--service 逗号多个", ServiceCmdFilters{Service: "gateway-node1,gateway-node6"}, []string{"gateway-node1", "gateway-node6"}},
		{"--service 命中非网关类型为空", ServiceCmdFilters{Service: "tunnel-frps,app-biz,uptime-kuma-node1"}, nil},
		{"--service 不存在的名字为空", ServiceCmdFilters{Service: "no-such-gw"}, nil},
		{"--server 过滤", ServiceCmdFilters{Server: "srv-node6"}, []string{"gateway-node6"}},
		{"--zone 过滤", ServiceCmdFilters{Zone: "node9"}, []string{"gateway-node1"}},
		{"--server+--service 取交集", ServiceCmdFilters{Server: "srv-node1", Service: "gateway-node1,gateway-node6"}, []string{"gateway-node1"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectGatewayServices(cfg, tt.filters)
			names := make([]string, 0, len(got))
			for _, gw := range got {
				if gw.Type != entity.InfraServiceTypeGateway {
					t.Errorf("selectGatewayServices() returned non-gateway service %s (type=%s)", gw.Name, gw.Type)
				}
				names = append(names, gw.Name)
			}
			if len(names) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(names, tt.want) {
				t.Errorf("selectGatewayServices() = %v, want %v", names, tt.want)
			}
		})
	}
}

// TestServiceVerifyCommandRegistered 验证 verify 子命令已注册且为只读形态
// （无 --yes/--dry-run 等执行类 flag，不含 plan/confirm 环节）。
func TestServiceVerifyCommandRegistered(t *testing.T) {
	serviceCmd := newServiceCommand(NewContext())

	var verify *cobra.Command
	for _, c := range serviceCmd.Commands() {
		if c.Name() == "verify" {
			verify = c
			break
		}
	}
	if verify == nil {
		t.Fatal("verify subcommand not registered under 'service'")
	}
	for _, forbidden := range []string{"yes", "dry-run", "force"} {
		if verify.Flags().Lookup(forbidden) != nil {
			t.Errorf("verify command should be read-only, unexpected flag --%s", forbidden)
		}
	}
}
