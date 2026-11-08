package usecase

// 网关 tracker_shipper 产物不变式校验器单测（内部PRD §6.4）：
// SSH 执行通道注入 mockShipperRunner（实现 contract.SSHRunner 窄接口），
// 三种 FAIL（读取/解析失败、缺段、enabled 不一致）+ 一致 PASS 全覆盖。

import (
	"errors"
	"strings"
	"testing"

	"github.com/lite-lake/infra-yamlops/internal/domain/contract"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
)

// mockShipperRunner SSH 只读执行桩：返回预置 stdout/err，并记录收到的命令。
type mockShipperRunner struct {
	stdout string
	stderr string
	err    error
	cmds   []string
}

func (m *mockShipperRunner) Run(cmd string) (string, string, error) {
	m.cmds = append(m.cmds, cmd)
	return m.stdout, m.stderr, m.err
}

func (m *mockShipperRunner) RunWithStdin(stdin string, cmd string) (string, string, error) {
	return m.Run(cmd)
}

var _ contract.SSHRunner = (*mockShipperRunner)(nil)

// gatewayShipperFor 构造带/不带 tracker_shipper 的网关实体。
func gatewayShipperFor(name string, shipper *entity.GatewayTrackerShipperConfig) *entity.InfraService {
	return &entity.InfraService{
		Name:                  name,
		Type:                  entity.InfraServiceTypeGateway,
		ServiceBase:           entity.ServiceBase{Server: "srv-node1"},
		Image:                 "ghcr.io/lite-lake/infra-gate:latest",
		GatewayTrackerShipper: shipper,
	}
}

func gatewayShipperEnabled(enabled bool) *entity.GatewayTrackerShipperConfig {
	return &entity.GatewayTrackerShipperConfig{
		Enabled:      enabled,
		Endpoint:     "https://app-tc-api.node1.v-dev.mesh.example.net/api/internal/gateway-ingest",
		SharedSecret: *valueobject.NewSecretRefPlain("s3cr3t"),
	}
}

// 产物样例：无 tracker_shipper 段（陈旧 checkout / 旧版生成器渲染的存量形态，缺段即 FAIL）。
const gatewayYAMLNoShipper = "server:\n  port: 80\n  log_level: 3\nhosts: []\n"

// gatewayYAMLWithShipperEnabled 产物样例：含 tracker_shipper 段。
func gatewayYAMLWithShipperEnabled(enabled bool) string {
	return "server:\n  port: 80\ntracker_shipper:\n  enabled: " + boolYAML(enabled) +
		"\n  endpoint: https://app-tc-api.node1.v-dev.mesh.example.net/api/internal/gateway-ingest\nhosts: []\n"
}

func boolYAML(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func TestGatewayShipperVerifier_VerifyNode(t *testing.T) {
	tests := []struct {
		name       string
		infra      *entity.InfraService
		stdout     string
		stderr     string
		runErr     error
		wantPass   bool
		wantErr    error    // 期望 errors.Is 命中的 sentinel；nil = 期望 PASS
		wantErrSub []string // 期望 FAIL 文案包含（节点名 / 产物 vs 部署源值）
	}{
		{
			name:     "一致 PASS：源 enabled=true 产物 enabled=true",
			infra:    gatewayShipperFor("gateway-node1", gatewayShipperEnabled(true)),
			stdout:   gatewayYAMLWithShipperEnabled(true),
			wantPass: true,
		},
		{
			name:       "FAIL：陈旧 checkout（实体 nil + 产物缺段）无条件 FAIL，不因双方一致放行",
			infra:      gatewayShipperFor("gateway-node1", nil),
			stdout:     gatewayYAMLNoShipper,
			wantPass:   false,
			wantErr:    ErrGatewayShipperSegmentMissing,
			wantErrSub: []string{"srv-node1", "gateway-node1", "dev/prod 网关必须显式配置该段", "checkout 疑似陈旧"},
		},
		{
			name:     "一致 PASS：源配置但 enabled=false 产物 enabled=false",
			infra:    gatewayShipperFor("gateway-node1", gatewayShipperEnabled(false)),
			stdout:   gatewayYAMLWithShipperEnabled(false),
			wantPass: true,
		},
		{
			name:     "一致 PASS：豁免形态（实体 nil 期望 false，产物显式 enabled=false）",
			infra:    gatewayShipperFor("gateway-node1", nil),
			stdout:   gatewayYAMLWithShipperEnabled(false),
			wantPass: true,
		},
		{
			name:       "FAIL：cat 命令失败（文件缺失/权限）",
			infra:      gatewayShipperFor("gateway-node1", gatewayShipperEnabled(true)),
			runErr:     errors.New("exit status 1"),
			stderr:     "No such file or directory",
			wantPass:   false,
			wantErr:    ErrGatewayShipperArtifactUnreadable,
			wantErrSub: []string{"srv-node1", "gateway-node1", "No such file or directory"},
		},
		{
			name:       "FAIL：产物非合法 YAML",
			infra:      gatewayShipperFor("gateway-node1", gatewayShipperEnabled(true)),
			stdout:     "\tbroken: [",
			wantPass:   false,
			wantErr:    ErrGatewayShipperArtifactUnreadable,
			wantErrSub: []string{"srv-node1", "解析失败"},
		},
		{
			name:       "FAIL：源配置 enabled=true 产物缺 tracker_shipper 段",
			infra:      gatewayShipperFor("gateway-node1", gatewayShipperEnabled(true)),
			stdout:     gatewayYAMLNoShipper,
			wantPass:   false,
			wantErr:    ErrGatewayShipperSegmentMissing,
			wantErrSub: []string{"srv-node1", "gateway-node1", "dev/prod 网关必须显式配置该段"},
		},
		{
			name:       "FAIL：enabled 不一致（产物 false vs 部署源 true）",
			infra:      gatewayShipperFor("gateway-node1", gatewayShipperEnabled(true)),
			stdout:     gatewayYAMLWithShipperEnabled(false),
			wantPass:   false,
			wantErr:    ErrGatewayShipperEnabledMismatch,
			wantErrSub: []string{"srv-node1", "产物 false vs 部署源 true"},
		},
		{
			name:       "FAIL：enabled 不一致（产物 true vs 部署源 false，nil 实体期望 false）",
			infra:      gatewayShipperFor("gateway-node1", nil),
			stdout:     gatewayYAMLWithShipperEnabled(true),
			wantPass:   false,
			wantErr:    ErrGatewayShipperEnabledMismatch,
			wantErrSub: []string{"srv-node1", "产物 true vs 部署源 false"},
		},
		{
			name:       "FAIL：段在位但 enabled 键缺失按 false 处理",
			infra:      gatewayShipperFor("gateway-node1", gatewayShipperEnabled(true)),
			stdout:     "tracker_shipper:\n  endpoint: https://x.example.com\nhosts: []\n",
			wantPass:   false,
			wantErr:    ErrGatewayShipperEnabledMismatch,
			wantErrSub: []string{"产物 false vs 部署源 true"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &mockShipperRunner{stdout: tt.stdout, stderr: tt.stderr, err: tt.runErr}
			verifier := NewGatewayShipperVerifier("dev")

			check := verifier.VerifyNode(runner, "srv-node1", tt.infra)

			if check.Pass != tt.wantPass {
				t.Fatalf("VerifyNode() Pass = %v, want %v (err = %v)", check.Pass, tt.wantPass, check.Err)
			}
			if check.ServiceName != "gateway-node1" || check.ServerName != "srv-node1" {
				t.Errorf("VerifyNode() check identity = %s/%s, want gateway-node1/srv-node1", check.ServiceName, check.ServerName)
			}
			if tt.wantErr != nil {
				if check.Err == nil {
					t.Fatalf("VerifyNode() expected error wrapping %v, got nil", tt.wantErr)
				}
				if !errors.Is(check.Err, tt.wantErr) {
					t.Errorf("VerifyNode() error = %v, want errors.Is %v", check.Err, tt.wantErr)
				}
				for _, sub := range tt.wantErrSub {
					if !strings.Contains(check.Err.Error(), sub) {
						t.Errorf("VerifyNode() error %q should contain %q", check.Err, sub)
					}
				}
			} else if check.Err != nil {
				t.Errorf("VerifyNode() unexpected error = %v", check.Err)
			}

			// 只读：唯一命令必须是 sudo cat 远端 gateway.yml 绝对路径（路径经 ShellEscape 单引号包裹）。
			wantCmd := "sudo cat '/data/yamlops/yo-dev-gateway-node1/gateway.yml'"
			if len(runner.cmds) != 1 || runner.cmds[0] != wantCmd {
				t.Errorf("VerifyNode() commands = %v, want exactly [%s]", runner.cmds, wantCmd)
			}
		})
	}
}

func TestAggregateGatewayShipperChecks(t *testing.T) {
	pass := GatewayShipperCheck{ServiceName: "gw-ok", ServerName: "srv-1", Pass: true}
	failA := GatewayShipperCheck{ServiceName: "gw-a", ServerName: "srv-2",
		Err: errors.New("节点 srv-2 服务 gw-a: 缺 tracker_shipper 段")}
	failB := GatewayShipperCheck{ServiceName: "gw-b", ServerName: "srv-3",
		Err: errors.New("节点 srv-3 服务 gw-b: tracker_shipper enabled 不一致: 产物 false vs 部署源 true")}

	t.Run("全部 PASS 返回 nil", func(t *testing.T) {
		if err := AggregateGatewayShipperChecks([]GatewayShipperCheck{pass, {ServiceName: "gw-c", ServerName: "srv-4", Pass: true}}); err != nil {
			t.Errorf("AggregateGatewayShipperChecks() error = %v, want nil", err)
		}
	})

	t.Run("任一 FAIL 返回多行聚合 error（每 FAIL 节点一行）", func(t *testing.T) {
		err := AggregateGatewayShipperChecks([]GatewayShipperCheck{pass, failA, failB})
		if err == nil {
			t.Fatal("AggregateGatewayShipperChecks() expected aggregated error, got nil")
		}
		msg := err.Error()
		if !strings.Contains(msg, "2 台网关产物校验失败") {
			t.Errorf("aggregated error should mention fail count, got: %s", msg)
		}
		for _, want := range []string{"srv-2 服务 gw-a", "srv-3 服务 gw-b"} {
			if !strings.Contains(msg, want) {
				t.Errorf("aggregated error should contain %q, got:\n%s", want, msg)
			}
		}
		if lines := strings.Count(msg, "\n"); lines < 2 {
			t.Errorf("aggregated error should be multi-line (one per failed node), got %d newlines:\n%s", lines, msg)
		}
	})
}
