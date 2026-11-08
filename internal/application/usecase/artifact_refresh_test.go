package usecase

// 回归测试（内部PRD 批量部署下发陈旧产物缺陷修复）：
// 下发链路（deploy/restart）改为「下发前按当前全量配置重渲染该服务产物」，
// 磁盘 deployments/ 缓存只作 diff 参照不作下发源；重渲染失败 fail-closed。

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lite-lake/infra-yamlops/internal/domain/contract"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
)

const (
	// staleGateMarker 预置陈旧产物的标记串，出现在任何下发内容中即视为陈旧字节。
	staleGateMarker = "# stale artifact (old form, missing new section)"
	// tcSecretName / tcSecretValue 核心反例的 tracker_shipper secret 引用。
	tcSecretName  = "tc_gateway_ingest_secret"
	tcSecretValue = "s3cr3t-value"
)

// seedStaleArtifacts 在 workDir/deployments/{env}/{server}/ 预置陈旧缓存
// （模拟旧版二进制 dry-run 覆写/异进程写缓存后的形态），返回 server 目录路径。
func seedStaleArtifacts(t *testing.T, workDir, env, server string, files map[string]string) string {
	t.Helper()
	serverDir := filepath.Join(workDir, "deployments", env, server)
	if err := os.MkdirAll(serverDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(serverDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return serverDir
}

// newGatewayTestConfig 构造最小 entity.Config：带 tracker_shipper 的 gateway
// + 同 server 一个带 gateways 路由的 biz service。
func newGatewayTestConfig() *entity.Config {
	return &entity.Config{
		Secrets: []entity.Secret{{Name: tcSecretName, Value: tcSecretValue}},
		InfraServices: []entity.InfraService{{
			ServiceBase: entity.ServiceBase{Server: "server1"},
			Name:        "gateway1",
			Type:        entity.InfraServiceTypeGateway,
			Image:       "ghcr.io/lite-lake/infra-gate:latest",
			GatewayPorts: &entity.GatewayPorts{
				HTTP:  80,
				HTTPS: 443,
			},
			GatewayTrackerShipper: &entity.GatewayTrackerShipperConfig{
				Enabled:      true,
				Endpoint:     "https://app-tc-api.node1.v-test.mesh.example.net/api/internal/gateway-ingest",
				SharedSecret: *valueobject.NewSecretRefSecret(tcSecretName),
			},
		}},
		Services: []entity.BizService{{
			ServiceBase: entity.ServiceBase{Server: "server1"},
			Name:        "app1",
			Image:       "registry.example.com/app1:v2",
			Gateways: []entity.ServiceGatewayRoute{{
				Hostname: "app1.env-test.mesh.example.net",
				HTTPS:    true,
			}},
		}},
	}
}

// newRefreshTestDeps 构造带 config 注入的测试 deps（env=test，server1 已注册）。
func newRefreshTestDeps(t *testing.T, cfg *entity.Config) (*mockDeps, *mockSSHClient) {
	t.Helper()
	mockSSH := &mockSSHClient{runStdout: "ok"}
	deps := newMockDeps()
	deps.sshClient = mockSSH
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4", Port: 22, User: "root"}
	deps.serverEntities["server1"] = &entity.Server{Name: "server1"}
	deps.env = "test"
	deps.workDir = t.TempDir()
	deps.config = cfg
	return deps, mockSSH
}

// TestApply_GatewayRefreshesStaleArtifactsBeforeDeploy 核心反例：预置缺
// tracker_shipper 段的陈旧 gate.yaml 后执行 gateway 部署，断言下发到远端
// gateway.yml 的是按当前实体重渲染的新内容（含 tracker_shipper 段与已解析
// secret），且磁盘缓存已被刷新。
func TestApply_GatewayRefreshesStaleArtifactsBeforeDeploy(t *testing.T) {
	h := NewInfraServiceHandler()
	deps, mockSSH := newRefreshTestDeps(t, newGatewayTestConfig())

	staleContent := staleGateMarker + "\nserver:\n  log_level: 3\nhosts: []\n"
	seedStaleArtifacts(t, deps.workDir, deps.env, "server1", map[string]string{
		"gateway1.gate.yaml":    staleContent,
		"gateway1.compose.yaml": staleGateMarker + "\n",
	})

	change := valueobject.NewChange(valueobject.ChangeTypeUpdate, "infra_service", "gateway1").
		WithNewState(&entity.InfraService{
			ServiceBase: entity.ServiceBase{Server: "server1"},
			Name:        "gateway1",
			Type:        entity.InfraServiceTypeGateway,
			Image:       "ghcr.io/lite-lake/infra-gate:latest",
			GatewayPorts: &entity.GatewayPorts{
				HTTP:  80,
				HTTPS: 443,
			},
			GatewayTrackerShipper: &entity.GatewayTrackerShipperConfig{
				Enabled:      true,
				Endpoint:     "https://app-tc-api.node1.v-test.mesh.example.net/api/internal/gateway-ingest",
				SharedSecret: *valueobject.NewSecretRefSecret(tcSecretName),
			},
		})

	result, err := h.Apply(context.Background(), change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got error: %v", result.Error)
	}

	pushed, ok := mockSSH.uploadedContent["/data/yamlops/yo-test-gateway1/gateway.yml"]
	if !ok {
		t.Fatalf("expected gateway.yml upload, uploaded: %v", mockSSH.uploaded)
	}
	if strings.Contains(pushed, staleGateMarker) {
		t.Errorf("pushed gateway.yml is the stale artifact:\n%s", pushed)
	}
	for _, want := range []string{"tracker_shipper", tcSecretValue,
		"https://app-tc-api.node1.v-test.mesh.example.net/api/internal/gateway-ingest"} {
		if !strings.Contains(pushed, want) {
			t.Errorf("pushed gateway.yml missing %q:\n%s", want, pushed)
		}
	}

	// 磁盘缓存应被刷新为含段版本。
	fresh, err := os.ReadFile(filepath.Join(deps.workDir, "deployments", deps.env, "server1", "gateway1.gate.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(fresh), "tracker_shipper") {
		t.Errorf("on-disk gate.yaml not refreshed with tracker_shipper:\n%s", fresh)
	}
	if strings.Contains(string(fresh), staleGateMarker) {
		t.Errorf("on-disk gate.yaml still the stale artifact:\n%s", fresh)
	}
}

// TestApply_BizServiceRefreshesStaleArtifactsBeforeDeploy biz 服务同型反例：
// 预置旧 image 的陈旧 compose.yaml 后部署，断言下发的是按当前实体渲染的新 image。
func TestApply_BizServiceRefreshesStaleArtifactsBeforeDeploy(t *testing.T) {
	h := NewServiceHandler()
	cfg := &entity.Config{
		Services: []entity.BizService{{
			ServiceBase: entity.ServiceBase{Server: "server1"},
			Name:        "app1",
			Image:       "registry.example.com/app1:v2",
		}},
	}
	deps, mockSSH := newRefreshTestDeps(t, cfg)

	staleContent := staleGateMarker + "\nimage: registry.example.com/app1:v1\n"
	seedStaleArtifacts(t, deps.workDir, deps.env, "server1", map[string]string{
		"app1.compose.yaml": staleContent,
	})

	change := valueobject.NewChange(valueobject.ChangeTypeUpdate, "service", "app1").
		WithNewState(&entity.BizService{
			ServiceBase: entity.ServiceBase{Server: "server1"},
			Name:        "app1",
			Image:       "registry.example.com/app1:v2",
		})

	result, err := h.Apply(context.Background(), change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got error: %v", result.Error)
	}

	pushed, ok := mockSSH.uploadedContent["/data/yamlops/yo-test-app1/docker-compose.yml"]
	if !ok {
		t.Fatalf("expected docker-compose.yml upload, uploaded: %v", mockSSH.uploaded)
	}
	if strings.Contains(pushed, "app1:v1") || strings.Contains(pushed, staleGateMarker) {
		t.Errorf("pushed docker-compose.yml is the stale artifact:\n%s", pushed)
	}
	if !strings.Contains(pushed, "app1:v2") {
		t.Errorf("pushed docker-compose.yml missing new image app1:v2:\n%s", pushed)
	}

	fresh, err := os.ReadFile(filepath.Join(deps.workDir, "deployments", deps.env, "server1", "app1.compose.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(fresh), "app1:v2") {
		t.Errorf("on-disk compose.yaml not refreshed with new image:\n%s", fresh)
	}
}

// TestApply_RefreshFailClosedOnUnresolvableSecret fail-closed 反例：config 中
// tracker_shipper.shared_secret 引用不存在的 secret → 重渲染失败必须让该变更失败，
// 且不发生任何文件下发（绝不回落到陈旧缓存）。
func TestApply_RefreshFailClosedOnUnresolvableSecret(t *testing.T) {
	h := NewInfraServiceHandler()
	cfg := newGatewayTestConfig()
	// shared_secret 引用的 secret 名不在 Secrets 表中 → 生成阶段解析报错。
	cfg.InfraServices[0].GatewayTrackerShipper.SharedSecret = *valueobject.NewSecretRefSecret("missing_secret")
	deps, mockSSH := newRefreshTestDeps(t, cfg)

	staleContent := staleGateMarker + "\nserver:\n  log_level: 3\nhosts: []\n"
	seedStaleArtifacts(t, deps.workDir, deps.env, "server1", map[string]string{
		"gateway1.gate.yaml": staleContent,
	})

	change := valueobject.NewChange(valueobject.ChangeTypeUpdate, "infra_service", "gateway1").
		WithNewState(&entity.InfraService{
			ServiceBase: entity.ServiceBase{Server: "server1"},
			Name:        "gateway1",
			Type:        entity.InfraServiceTypeGateway,
			Image:       "ghcr.io/lite-lake/infra-gate:latest",
			GatewayPorts: &entity.GatewayPorts{
				HTTP:  80,
				HTTPS: 443,
			},
			GatewayTrackerShipper: &entity.GatewayTrackerShipperConfig{
				Enabled:      true,
				Endpoint:     "https://app-tc-api.node1.v-test.mesh.example.net/api/internal/gateway-ingest",
				SharedSecret: *valueobject.NewSecretRefSecret("missing_secret"),
			},
		})

	result, err := h.Apply(context.Background(), change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure when artifact refresh fails (fail-closed)")
	}
	if result.Error == nil {
		t.Fatal("expected result.Error to be set on refresh failure")
	}
	if !strings.Contains(result.Error.Error(), "refresh deployment artifacts for gateway1") {
		t.Errorf("error should mention refresh failure, got: %v", result.Error)
	}
	if len(mockSSH.uploaded) != 0 {
		t.Errorf("no file must be pushed when refresh fails, uploaded: %v", mockSSH.uploaded)
	}
}

// TestApply_NoConfigKeepsLegacyCacheReadBehavior 向后兼容反例：deps 未注入 config
// （Config() 返回 nil）时不做重渲染，行为与修复前一致——预置什么就下发什么。
// 预置缓存需含与实体一致的 tracker_shipper 段（部署后产物校验钩子要求一致，否则判失败）。
func TestApply_NoConfigKeepsLegacyCacheReadBehavior(t *testing.T) {
	h := NewInfraServiceHandler()
	deps, mockSSH := newRefreshTestDeps(t, nil) // config = nil

	staleContent := staleGateMarker + "\nserver:\n  log_level: 3\nhosts: []\n" +
		"tracker_shipper:\n  enabled: true\n  endpoint: https://legacy.example.com/ingest\n"
	seedStaleArtifacts(t, deps.workDir, deps.env, "server1", map[string]string{
		"gateway1.gate.yaml": staleContent,
	})

	change := valueobject.NewChange(valueobject.ChangeTypeUpdate, "infra_service", "gateway1").
		WithNewState(&entity.InfraService{
			ServiceBase: entity.ServiceBase{Server: "server1"},
			Name:        "gateway1",
			Type:        entity.InfraServiceTypeGateway,
			Image:       "ghcr.io/lite-lake/infra-gate:latest",
			GatewayPorts: &entity.GatewayPorts{
				HTTP:  80,
				HTTPS: 443,
			},
			GatewayTrackerShipper: &entity.GatewayTrackerShipperConfig{
				Enabled:      true,
				Endpoint:     "https://app-tc-api.node1.v-test.mesh.example.net/api/internal/gateway-ingest",
				SharedSecret: *valueobject.NewSecretRefSecret(tcSecretName),
			},
		})

	result, err := h.Apply(context.Background(), change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got error: %v", result.Error)
	}

	pushed, ok := mockSSH.uploadedContent["/data/yamlops/yo-test-gateway1/gateway.yml"]
	if !ok {
		t.Fatalf("expected gateway.yml upload, uploaded: %v", mockSSH.uploaded)
	}
	if pushed != staleContent {
		t.Errorf("legacy behavior must push the pre-seeded cache as-is, got:\n%s", pushed)
	}
}

// TestRestartInfraService_RefreshesStaleArtifactsBeforeSync restart 路径反例：
// RestartInfraService 前预置陈旧 gate.yaml，断言同步到远端 gateway.yml 的已是
// 重渲染后的新内容（含 tracker_shipper 段）。
func TestRestartInfraService_RefreshesStaleArtifactsBeforeSync(t *testing.T) {
	deps, mockSSH := newRefreshTestDeps(t, newGatewayTestConfig())

	staleContent := staleGateMarker + "\nserver:\n  log_level: 3\nhosts: []\n"
	seedStaleArtifacts(t, deps.workDir, deps.env, "server1", map[string]string{
		"gateway1.gate.yaml": staleContent,
	})

	mgr := NewServiceRestartManager(deps)
	result := mgr.RestartInfraService("server1", "gateway1")
	if result.Error != nil || !result.Success {
		t.Fatalf("expected restart success, got error: %v", result.Error)
	}

	pushed, ok := mockSSH.uploadedContent["/data/yamlops/yo-test-gateway1/gateway.yml"]
	if !ok {
		t.Fatalf("expected gateway.yml upload, uploaded: %v", mockSSH.uploaded)
	}
	if strings.Contains(pushed, staleGateMarker) {
		t.Errorf("restart pushed the stale artifact:\n%s", pushed)
	}
	if !strings.Contains(pushed, "tracker_shipper") || !strings.Contains(pushed, tcSecretValue) {
		t.Errorf("restart pushed gateway.yml missing tracker_shipper/secret:\n%s", pushed)
	}
}

// TestRestartBizService_RefreshesBeforeRestart restart biz 路径：重启前按当前实体
// 重渲染刷新磁盘缓存（新 image 落盘），重启成功。
func TestRestartBizService_RefreshesBeforeRestart(t *testing.T) {
	cfg := &entity.Config{
		Services: []entity.BizService{{
			ServiceBase: entity.ServiceBase{Server: "server1"},
			Name:        "app1",
			Image:       "registry.example.com/app1:v2",
		}},
	}
	deps, _ := newRefreshTestDeps(t, cfg)

	staleContent := staleGateMarker + "\nimage: registry.example.com/app1:v1\n"
	seedStaleArtifacts(t, deps.workDir, deps.env, "server1", map[string]string{
		"app1.compose.yaml": staleContent,
	})

	mgr := NewServiceRestartManager(deps)
	result := mgr.RestartBizService("server1", "app1")
	if result.Error != nil || !result.Success {
		t.Fatalf("expected restart success, got error: %v", result.Error)
	}

	fresh, err := os.ReadFile(filepath.Join(deps.workDir, "deployments", deps.env, "server1", "app1.compose.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(fresh), "app1:v1") || strings.Contains(string(fresh), staleGateMarker) {
		t.Errorf("on-disk compose.yaml still the stale artifact:\n%s", fresh)
	}
	if !strings.Contains(string(fresh), "app1:v2") {
		t.Errorf("on-disk compose.yaml not refreshed with new image:\n%s", fresh)
	}
}

// TestRestartInfraService_RefreshFailClosed restart infra 路径 fail-closed：重渲染失败
// （secret 无法解析）时重启中止，不继续走陈旧缓存下发。
func TestRestartInfraService_RefreshFailClosed(t *testing.T) {
	cfg := newGatewayTestConfig()
	cfg.InfraServices[0].GatewayTrackerShipper.SharedSecret = *valueobject.NewSecretRefSecret("missing_secret")
	deps, mockSSH := newRefreshTestDeps(t, cfg)

	staleContent := staleGateMarker + "\nserver:\n  log_level: 3\nhosts: []\n"
	seedStaleArtifacts(t, deps.workDir, deps.env, "server1", map[string]string{
		"gateway1.gate.yaml": staleContent,
	})

	mgr := NewServiceRestartManager(deps)
	result := mgr.RestartInfraService("server1", "gateway1")
	if result.Success {
		t.Error("expected restart failure when artifact refresh fails (fail-closed)")
	}
	if result.Error == nil {
		t.Fatal("expected result.Error to be set on refresh failure")
	}
	if !strings.Contains(result.Error.Error(), "refresh deployment artifacts for gateway1") {
		t.Errorf("error should mention refresh failure, got: %v", result.Error)
	}
	if len(mockSSH.uploaded) != 0 {
		t.Errorf("no file must be pushed when refresh fails, uploaded: %v", mockSSH.uploaded)
	}
}

// TestRestartBizService_RefreshFailClosed restart biz 路径 fail-closed：重渲染失败
// （env secret 无法解析）时重启中止。
func TestRestartBizService_RefreshFailClosed(t *testing.T) {
	cfg := &entity.Config{
		Services: []entity.BizService{{
			ServiceBase: entity.ServiceBase{Server: "server1"},
			Name:        "app1",
			Image:       "registry.example.com/app1:v2",
			Env: map[string]valueobject.SecretRef{
				"API_KEY": *valueobject.NewSecretRefSecret("missing_secret"),
			},
		}},
	}
	deps, _ := newRefreshTestDeps(t, cfg)

	mgr := NewServiceRestartManager(deps)
	result := mgr.RestartBizService("server1", "app1")
	if result.Success {
		t.Error("expected restart failure when artifact refresh fails (fail-closed)")
	}
	if result.Error == nil {
		t.Fatal("expected result.Error to be set on refresh failure")
	}
	if !strings.Contains(result.Error.Error(), "refresh deployment artifacts for app1") {
		t.Errorf("error should mention refresh failure, got: %v", result.Error)
	}
}

// mockSSHPool SSHPoolInterface 测试桩：按 ServerInfo.Host 返回预置的 mockSSHClient，
// 从不真实拨号。Host 即服务器唯一标识（RegisterServer 时每台配不同 Host）。
type mockSSHPool struct {
	byHost map[string]*mockSSHClient
}

func (p *mockSSHPool) Get(info *ServerInfo) (contract.SSHClient, error) {
	if c, ok := p.byHost[info.Host]; ok {
		return c, nil
	}
	return nil, errors.New("no mock ssh client for host: " + info.Host)
}

func (p *mockSSHPool) CloseAll() {}

// TestChangeExecutor_BatchDeployRefreshesAllStaleArtifacts 内部PRD 验收第 2 条
// 批量场景：同一 plan 内两个不同服务器的 gateway 变更（批量 = 多变更按服务器分组并行），
// 两台各自预置缺 tracker_shipper 段的陈旧产物，Apply 后每台下发的 gateway.yml 都必须是
// 按当前实体重渲染的新内容（含新段、已解析 secret、本节点 endpoint），且磁盘缓存同步刷新。
func TestChangeExecutor_BatchDeployRefreshesAllStaleArtifacts(t *testing.T) {
	const (
		server1, server2 = "server-node1", "server-node4"
		host1, host2     = "10.71.0.1", "10.71.0.2"
		gw1Name, gw2Name = "gateway-node1", "gateway-node4"
		endpoint1        = "https://app-tc-api.node1.v-test.mesh.example.net/ingest"
		endpoint2        = "https://app-tc-api.node4.v-test.mesh.example.net/ingest"
	)

	newGateway := func(name, server, endpoint string) entity.InfraService {
		return entity.InfraService{
			ServiceBase: entity.ServiceBase{Server: server},
			Name:        name,
			Type:        entity.InfraServiceTypeGateway,
			Image:       "ghcr.io/lite-lake/infra-gate:latest",
			GatewayPorts: &entity.GatewayPorts{
				HTTP:  80,
				HTTPS: 443,
			},
			GatewayTrackerShipper: &entity.GatewayTrackerShipperConfig{
				Enabled:      true,
				Endpoint:     endpoint,
				SharedSecret: *valueobject.NewSecretRefSecret(tcSecretName),
			},
		}
	}
	gw1 := newGateway(gw1Name, server1, endpoint1)
	gw2 := newGateway(gw2Name, server2, endpoint2)
	cfg := &entity.Config{
		Secrets:       []entity.Secret{{Name: tcSecretName, Value: tcSecretValue}},
		InfraServices: []entity.InfraService{gw1, gw2},
	}

	client1 := &mockSSHClient{runStdout: "ok"}
	client2 := &mockSSHClient{runStdout: "ok"}
	pool := &mockSSHPool{byHost: map[string]*mockSSHClient{host1: client1, host2: client2}}

	// 两台各自预置陈旧产物（模拟旧二进制 dry-run 覆写后的缺段形态）。
	staleContent := staleGateMarker + "\nserver:\n  log_level: 3\nhosts: []\n"
	workDir := t.TempDir()
	seedStaleArtifacts(t, workDir, "test", server1, map[string]string{gw1Name + ".gate.yaml": staleContent})
	seedStaleArtifacts(t, workDir, "test", server2, map[string]string{gw2Name + ".gate.yaml": staleContent})

	// 同一 plan 两个不同服务器的变更 → Apply 时按服务器分组并行执行。
	plan := valueobject.NewPlan()
	for i := range cfg.InfraServices {
		infra := &cfg.InfraServices[i]
		plan.AddChange(valueobject.NewChange(valueobject.ChangeTypeUpdate, "infra_service", infra.Name).
			WithNewState(infra))
	}

	executor := NewChangeExecutor(&ChangeExecutorConfig{Plan: plan, SSHPool: pool, Env: "test", Concurrency: 2})
	executor.SetConfig(cfg)
	executor.SetWorkDir(workDir)
	executor.RegisterServer(server1, host1, 22, "root", "", true)
	executor.RegisterServer(server2, host2, 22, "root", "", true)
	executor.SetServerEntities(map[string]*entity.Server{
		server1: {Name: server1},
		server2: {Name: server2},
	})

	registry := NewRegistry()
	registry.Register(NewDNSHandler())
	registry.Register(NewServiceHandler())
	registry.Register(NewInfraServiceHandler())
	registry.Register(NewServerHandler())
	RegisterNoopHandlers(registry)

	results := executor.Apply(context.Background(), registry)
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Error != nil {
			t.Fatalf("change %s failed: %v", r.Change.Name(), r.Error)
		}
	}

	// 每台下发的 gateway.yml 都必须是重渲染后的新内容，且不含对方节点的 endpoint（不串台）。
	for _, tc := range []struct {
		label    string
		server   string
		client   *mockSSHClient
		gwName   string
		endpoint string
		other    string
	}{
		{"server1/" + gw1Name, server1, client1, gw1Name, endpoint1, endpoint2},
		{"server2/" + gw2Name, server2, client2, gw2Name, endpoint2, endpoint1},
	} {
		pushed, ok := tc.client.uploadedContent["/data/yamlops/yo-test-"+tc.gwName+"/gateway.yml"]
		if !ok {
			t.Fatalf("[%s] expected gateway.yml upload, uploaded: %v", tc.label, tc.client.uploaded)
		}
		if strings.Contains(pushed, staleGateMarker) {
			t.Errorf("[%s] pushed gateway.yml is the stale artifact:\n%s", tc.label, pushed)
		}
		for _, want := range []string{"tracker_shipper", tcSecretValue, tc.endpoint} {
			if !strings.Contains(pushed, want) {
				t.Errorf("[%s] pushed gateway.yml missing %q:\n%s", tc.label, want, pushed)
			}
		}
		if strings.Contains(pushed, tc.other) {
			t.Errorf("[%s] pushed gateway.yml must not contain the other node's endpoint:\n%s", tc.label, pushed)
		}

		// 磁盘缓存同步刷新为含段版本。
		fresh, err := os.ReadFile(filepath.Join(workDir, "deployments", "test", tc.server, tc.gwName+".gate.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(fresh), "tracker_shipper") || strings.Contains(string(fresh), staleGateMarker) {
			t.Errorf("[%s] on-disk gate.yaml not refreshed:\n%s", tc.label, fresh)
		}
	}
}
