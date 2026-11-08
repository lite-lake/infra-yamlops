package cli

// `yamlops cli service verify`（内部PRD §6.4）：网关服务渲染产物不变式的
// 独立只读校验入口。加载实体、过滤 type=gateway（--service 可再过滤，缺省=该环境
// 全部网关），逐台 SSH 读回远端 gateway.yml 执行与部署 post-hook 同一套校验逻辑，
// 输出每节点 PASS/FAIL 行 + 汇总；任一 FAIL 退出码 1。不含 plan/confirm 环节。

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/lite-lake/infra-yamlops/internal/application/usecase"
	"github.com/lite-lake/infra-yamlops/internal/constants"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/ssh"
)

func newServiceVerifyCommand(ctx *Context, filters *ServiceCmdFilters) *cobra.Command {
	return &cobra.Command{
		Use:   "verify",
		Short: "Verify gateway tracker_shipper artifact on remote servers (read-only)",
		Long: `Verify the tracker_shipper invariant of deployed gateway services (read-only).

For every gateway infra service (type=gateway) in the environment (optionally
narrowed by --service / --server / --zone), read the rendered artifact
/data/yamlops/yo-{env}-{name}/gateway.yml via SSH and check the unconditional
invariant: the artifact MUST contain the tracker_shipper section, and its
enabled value must match the deployment source entity (entity nil = expects
false; explicit enabled:false is the only sanctioned exemption).

Nothing is modified; no plan/confirm stage. Exit code 1 when any gateway fails.

Examples:
  yamlops cli service verify -e prod
  yamlops cli service verify -e dev --service gateway-a,gateway-b`,
		Run: func(cmd *cobra.Command, args []string) {
			runServiceVerify(ctx, *filters)
		},
	}
}

// selectGatewayServices 选出待校验的网关实体（纯函数，便于单测）：
// 固定 type=gateway；--service/--server/--zone 可再过滤（取交集），缺省=该环境全部网关。
func selectGatewayServices(cfg *entity.Config, filters ServiceCmdFilters) []*entity.InfraService {
	var result []*entity.InfraService
	for i := range cfg.InfraServices {
		svc := &cfg.InfraServices[i]
		if svc.Type != entity.InfraServiceTypeGateway {
			continue
		}
		if !matchesFilter(svc.Server, filters.Server) {
			continue
		}
		if srv := cfg.GetServerMap()[svc.Server]; srv != nil && !matchesFilter(srv.Zone, filters.Zone) {
			continue
		}
		if !matchesFilter(svc.Name, filters.Service) {
			continue
		}
		result = append(result, svc)
	}
	return result
}

func runServiceVerify(ctx *Context, filters ServiceCmdFilters) {
	cfg, err := loadConfigFromContext(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	gateways := selectGatewayServices(cfg, filters)
	if len(gateways) == 0 {
		DisplayInfo("No gateway services matched for verify.")
		return
	}

	fmt.Printf("Verifying gateway tracker_shipper artifact: env=%s, gateways=%d\n\n", ctx.Env, len(gateways))

	verifier := usecase.NewGatewayShipperVerifier(ctx.Env)
	checks := make([]usecase.GatewayShipperCheck, 0, len(gateways))
	for _, gw := range gateways {
		check := verifyGatewayNode(ctx, cfg, verifier, gw)
		checks = append(checks, check)
		if check.Pass {
			fmt.Printf("[PASS] %s @ %s\n", check.ServiceName, check.ServerName)
		} else {
			fmt.Printf("[FAIL] %s @ %s: %v\n", check.ServiceName, check.ServerName, check.Err)
		}
	}

	failed := 0
	for _, c := range checks {
		if !c.Pass {
			failed++
		}
	}
	fmt.Printf("\nRESULT: %d pass, %d fail\n", len(checks)-failed, failed)
	if failed > 0 {
		os.Exit(1)
	}
}

// verifyGatewayNode 建立单节点 SSH 连接并执行校验（连接失败/服务器未登记计为该节点 FAIL）。
func verifyGatewayNode(ctx *Context, cfg *entity.Config, verifier *usecase.GatewayShipperVerifier, gw *entity.InfraService) usecase.GatewayShipperCheck {
	fail := func(format string, args ...interface{}) usecase.GatewayShipperCheck {
		return usecase.GatewayShipperCheck{
			ServiceName: gw.Name,
			ServerName:  gw.Server,
			Pass:        false,
			Err:         fmt.Errorf("节点 %s 服务 %s: %s", gw.Server, gw.Name, fmt.Sprintf(format, args...)),
		}
	}

	srv, ok := cfg.GetServerMap()[gw.Server]
	if !ok {
		return fail("server not found: %s", gw.Server)
	}
	password, err := srv.SSH.Password.Resolve(cfg.GetSecretsMap())
	if err != nil {
		return fail("cannot resolve ssh password: %v", err)
	}
	strictHostKeyChecking := true
	if !srv.SSH.StrictHostKeyChecking {
		strictHostKeyChecking = false
	}
	client, err := ssh.NewClientWithConfig(srv.SSH.Host, srv.SSH.Port, srv.SSH.User, password,
		&ssh.SSHConfig{StrictHostKeyChecking: strictHostKeyChecking, Timeout: constants.DefaultSSHTimeout})
	if err != nil {
		return fail("connection failed: %v", err)
	}
	defer client.Close()

	return verifier.VerifyNode(client, gw.Server, gw)
}
