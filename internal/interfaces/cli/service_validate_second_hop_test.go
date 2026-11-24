package cli

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
)

// writeSecondHopFixture 落盘最小可校验 userdata 夹具：三个文件（zones/servers/services_biz），
// 一个 external_backends 入口服务 + 一个被指为上游目标的第二跳条目（hsts 由 hstsLine 控制）。
func writeSecondHopFixture(t *testing.T, hstsLine string) string {
	t.Helper()
	dir := t.TempDir()
	envDir := filepath.Join(dir, "userdata", "dev")
	if err := os.MkdirAll(envDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	files := map[string]string{
		"mesh.yaml": `mesh:
  domain_zone: mesh.example.net
`,
		"zones.yaml": `zones:
  - name: node9
    region: 广州
  - name: node10
    region: 香港
`,
		"servers.yaml": `servers:
  - name: srv-node1
    zone: node9
    os: ubuntu
    ip:
      public: 10.0.0.1
    ssh:
      host: 10.0.0.1
      port: 22
      user: ubuntu
      password: fixture-password
  - name: srv-node8
    zone: node10
    os: ubuntu
    ip:
      public: 10.0.0.2
    ssh:
      host: 10.0.0.2
      port: 22
      user: ubuntu
      password: fixture-password
`,
		"services_biz.yaml": `services:
  - name: demo-node8-proxy
    server: srv-node8
    external_backends:
      - https://app-demo.node1.v-dev.mesh.example.net
    healthcheck:
      path: /healthz
    gateways:
      - hostname: demo.mesh.example.net
        http: true
        https: true
  - name: demo-web-dev
    server: srv-node1
    image: demo:latest
    healthcheck:
      path: /healthz
    gateways:
      - hostname: app-demo.node1.v-dev.mesh.example.net
        https: true
` + hstsLine,
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(envDir, name), []byte(content), 0644); err != nil {
			t.Fatalf("WriteFile %s: %v", name, err)
		}
	}
	return dir
}

// runServiceValidateCapture 捕获 runServiceValidate 的 stdout（PASSED 路径不调用 os.Exit）
func runServiceValidateCapture(t *testing.T, configDir string) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe: %v", err)
	}
	os.Stdout = w

	// 先独立走一遍加载+校验：若夹具本身不合法，runServiceValidate 会 os.Exit(2) 杀掉测试进程
	wf := NewWorkflow("dev", configDir)
	if _, err := wf.LoadAndValidate(context.Background()); err != nil {
		os.Stdout = old
		_ = w.Close()
		_ = r.Close()
		t.Fatalf("fixture must load and validate cleanly: %v", err)
	}

	runServiceValidate(&Context{Env: "dev", ConfigDir: configDir, Concurrency: 1}, ServiceCmdFilters{})
	_ = w.Close()
	os.Stdout = old
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	_ = r.Close()
	return string(out)
}

// 部署前 service validate 的第二跳告警（内部PRD §5.6）：
// 漏标 → 出现在 warnings 汇总且 Result 仍 PASSED；已标 off → 无告警、Result 仍 PASSED。
func TestServiceValidate_SecondHopHSTS(t *testing.T) {
	const warningMsg = "gateway host 'app-demo.node1.v-dev.mesh.example.net' is a second-hop entry"

	tests := []struct {
		name         string
		hstsLine     string
		wantWarning  bool
		wantWarnWord string
	}{
		{
			name:         "第二跳漏标 → 告警且 PASSED",
			hstsLine:     "",
			wantWarning:  true,
			wantWarnWord: "[WARN] 1 warning",
		},
		{
			name:         "第二跳已标 off → 无告警且 PASSED",
			hstsLine:     "        hsts: \"off\"\n",
			wantWarning:  false,
			wantWarnWord: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := runServiceValidateCapture(t, writeSecondHopFixture(t, tt.hstsLine))

			if !strings.Contains(out, "Result: PASSED") {
				t.Errorf("Result must stay PASSED (warning 不得影响结论)，got:\n%s", out)
			}
			if strings.Contains(out, "FAILED") || strings.Contains(out, "ERRORS:") {
				t.Errorf("warning 不得进入 errors，got:\n%s", out)
			}
			if tt.wantWarning {
				if !strings.Contains(out, warningMsg) {
					t.Errorf("output missing second-hop warning %q, got:\n%s", warningMsg, out)
				}
				if !strings.Contains(out, tt.wantWarnWord) {
					t.Errorf("output missing %q, got:\n%s", tt.wantWarnWord, out)
				}
				// 定位信息：服务名 + host 条目名 + 应配 hsts: "off" 指引 + PRD 编号
				suggestion := `Add hsts: "off" to gateway host 'app-demo.node1.v-dev.mesh.example.net' in service 'demo-web-dev'`
				if !strings.Contains(out, suggestion) {
					t.Errorf("output missing suggestion %q, got:\n%s", suggestion, out)
				}
				if !strings.Contains(out, "the edge gateway is the single sender") {
					t.Errorf("output missing suggestion detail, got:\n%s", out)
				}
			} else {
				if strings.Contains(out, warningMsg) {
					t.Errorf("已标 hsts off 不应告警，got:\n%s", out)
				}
				if strings.Contains(out, "[WARN]") {
					t.Errorf("不应有告警，got:\n%s", out)
				}
			}
		})
	}
}

// secondHopHSTSWarnings 只产出 warnings（不并入 errors），文案与生成器侧一致。
func TestSecondHopHSTSWarnings_WarningsOnly(t *testing.T) {
	off := "off"
	cfg := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"}, Services: []entity.BizService{
		{
			Name:             "demo-node8-proxy",
			ServiceBase:      entity.ServiceBase{Server: "srv-node8"},
			ExternalBackends: []string{"https://app-demo.node1.v-dev.mesh.example.net"},
			Gateways:         []entity.ServiceGatewayRoute{{Hostname: "demo.mesh.example.net", HTTP: true, HTTPS: true}},
		},
		{
			Name:        "demo-web-dev",
			ServiceBase: entity.ServiceBase{Server: "srv-node1"},
			Image:       "demo:latest",
			Gateways: []entity.ServiceGatewayRoute{
				{Hostname: "app-demo.node1.v-dev.mesh.example.net", HTTPS: true},
			},
		},
	}}

	warns := secondHopHSTSWarnings(cfg, "dev")
	if len(warns) != 1 {
		t.Fatalf("warnings = %d, want 1 (%v)", len(warns), warns)
	}
	if !strings.Contains(warns[0].msg, "app-demo.node1.v-dev.mesh.example.net") ||
		!strings.Contains(warns[0].suggestion, `hsts: "off"`) {
		t.Errorf("warning 文案缺定位信息: %+v", warns[0])
	}

	// 已标注 off：零告警
	cfg.Services[1].Gateways[0].HSTS = &off
	if warns := secondHopHSTSWarnings(cfg, "dev"); len(warns) != 0 {
		t.Errorf("已标 off 应零告警，got %v", warns)
	}
}
