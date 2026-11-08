package usecase

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
)

func TestInfraServiceHandler_EntityType(t *testing.T) {
	h := NewInfraServiceHandler()
	if h.EntityType() != "infra_service" {
		t.Errorf("expected 'infra_service', got %s", h.EntityType())
	}
}

// validGateYAMLForVerify 网关部署成功后产物校验钩子（内部PRD §6.4）回读的
// 最小合法 gateway.yml。新规则下产物缺 tracker_shipper 段一律 FAIL，唯一合法豁免形态
// 是显式 enabled:false；成功路径用例的实体（GatewayTrackerShipper{Enabled:false}）与
// 此缓存一致。mock 未上传 gateway.yml 时 sudo cat 回退到 runStdout，故以合法 yaml 作桩。
const validGateYAMLForVerify = "server:\n  port: 80\ntracker_shipper:\n  enabled: false\nhosts: []\n"

func TestInfraServiceHandler_Apply_ServerNotDetermined(t *testing.T) {
	h := NewInfraServiceHandler()
	ctx := context.Background()
	deps := newMockDeps()

	change := valueobject.NewChange(valueobject.ChangeTypeCreate, "infra_service", "gateway1").
		WithNewState(map[string]interface{}{})

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure when server cannot be determined")
	}
}

func TestInfraServiceHandler_Apply_ServerNotRegistered(t *testing.T) {
	h := NewInfraServiceHandler()
	ctx := context.Background()
	deps := newMockDeps()

	change := valueobject.NewChange(valueobject.ChangeTypeCreate, "infra_service", "gateway1").
		WithNewState(map[string]interface{}{
			"server": "unknown",
		})

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure when server not registered")
	}
}

func TestInfraServiceHandler_Apply_SSHError(t *testing.T) {
	h := NewInfraServiceHandler()
	ctx := context.Background()

	deps := newMockDeps()
	deps.sshErr = errors.New("connection failed")
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4", Port: 22, User: "root"}

	change := valueobject.NewChange(valueobject.ChangeTypeCreate, "infra_service", "gateway1").
		WithNewState(map[string]interface{}{
			"server": "server1",
		})

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure for SSH error")
	}
}

func TestInfraServiceHandler_Apply_MkdirError(t *testing.T) {
	h := NewInfraServiceHandler()
	ctx := context.Background()

	mockSSH := &mockSSHClient{mkdirErr: errors.New("permission denied")}
	deps := newMockDeps()
	deps.sshClient = mockSSH
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4", Port: 22, User: "root"}
	deps.env = "test"

	change := valueobject.NewChange(valueobject.ChangeTypeCreate, "infra_service", "gateway1").
		WithNewState(map[string]interface{}{
			"server": "server1",
		})

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure for mkdir error")
	}
}

func TestInfraServiceHandler_Apply_Deploy(t *testing.T) {
	h := NewInfraServiceHandler()
	ctx := context.Background()

	mockSSH := &mockSSHClient{runStdout: validGateYAMLForVerify}
	deps := newMockDeps()
	deps.sshClient = mockSSH
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4", Port: 22, User: "root"}
	deps.serverEntities["server1"] = &entity.Server{Name: "server1"}
	deps.env = "test"
	deps.workDir = t.TempDir()

	change := valueobject.NewChange(valueobject.ChangeTypeCreate, "infra_service", "gateway1").
		WithNewState(&entity.InfraService{
			ServiceBase: entity.ServiceBase{
				Server: "server1",
			},
			Name: "gateway1",
			Type: entity.InfraServiceTypeGateway,
			// 内部PRD §6.4：产物校验要求显式 tracker_shipper 段（缺段一律 FAIL），
			// 成功路径用例的实体与上传缓存保持一致：显式 enabled:false（唯一合法豁免形态）。
			GatewayTrackerShipper: &entity.GatewayTrackerShipperConfig{Enabled: false},
		})

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got error: %v", result.Error)
	}
}

// TestInfraServiceHandler_Apply_GatewayCacheDirPrepared 验证网关部署时
// 会预先创建 cache 目录并 chmod 777（容器内 appuser 需要写权限）。
func TestInfraServiceHandler_Apply_GatewayCacheDirPrepared(t *testing.T) {
	h := NewInfraServiceHandler()
	ctx := context.Background()

	mockSSH := &mockSSHClient{runStdout: validGateYAMLForVerify}
	deps := newMockDeps()
	deps.sshClient = mockSSH
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4", Port: 22, User: "root"}
	deps.serverEntities["server1"] = &entity.Server{Name: "server1"}
	deps.env = "test"
	deps.workDir = t.TempDir()

	change := valueobject.NewChange(valueobject.ChangeTypeCreate, "infra_service", "gateway1").
		WithNewState(&entity.InfraService{
			ServiceBase: entity.ServiceBase{
				Server: "server1",
			},
			Name: "gateway1",
			Type: entity.InfraServiceTypeGateway,
			// 内部PRD §6.4：产物校验要求显式 tracker_shipper 段（缺段一律 FAIL），
			// 成功路径用例的实体与上传缓存保持一致：显式 enabled:false（唯一合法豁免形态）。
			GatewayTrackerShipper: &entity.GatewayTrackerShipperConfig{Enabled: false},
		})

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got error: %v", result.Error)
	}

	foundCachePrep := false
	for _, cmd := range mockSSH.commandsRun {
		if strings.Contains(cmd, "mkdir -p") && strings.Contains(cmd, "chmod -R 777") && strings.Contains(cmd, "/cache") {
			foundCachePrep = true
			break
		}
	}
	if !foundCachePrep {
		t.Errorf("expected gateway cache dir preparation command, commands run: %v", mockSSH.commandsRun)
	}
}

func TestInfraServiceHandler_Apply_Delete(t *testing.T) {
	h := NewInfraServiceHandler()
	ctx := context.Background()

	mockSSH := &mockSSHClient{runStdout: "removed"}
	deps := newMockDeps()
	deps.sshClient = mockSSH
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4", Port: 22, User: "root"}
	deps.env = "test"

	change := valueobject.NewChange(valueobject.ChangeTypeDelete, "infra_service", "gateway1").
		WithOldState(map[string]interface{}{
			"server": "server1",
		})

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got error: %v", result.Error)
	}
}

func TestInfraServiceHandler_Delete_RemoveError(t *testing.T) {
	h := NewInfraServiceHandler()
	ctx := context.Background()

	mockSSH := &mockSSHClient{runErr: errors.New("rm failed"), runStderr: "permission denied"}
	deps := newMockDeps()
	deps.sshClient = mockSSH
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4", Port: 22, User: "root"}
	deps.env = "test"

	change := valueobject.NewChange(valueobject.ChangeTypeDelete, "infra_service", "gateway1").
		WithOldState(map[string]interface{}{
			"server": "server1",
		})

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Error("expected failure for remove error")
	}
}

func TestInfraServiceHandler_ExecuteServiceDeploy_InvalidState(t *testing.T) {
	mockSSH := &mockSSHClient{}
	deps := newMockDeps()
	deps.sshClient = mockSSH
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4"}
	deps.serverEntities["server1"] = &entity.Server{Name: "server1"}
	deps.env = "test"

	change := valueobject.NewChange(valueobject.ChangeTypeCreate, "infra_service", "gateway1").
		WithNewState("invalid state")

	deployCtx := &ServiceDeployContext{
		ServerName: "server1",
		Client:     mockSSH,
		RemoteDir:  "/opt/test",
	}
	result, err := ExecuteServiceDeploy(change, deployCtx, deps, DeployServiceOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success (invalid state is tolerated in common deploy), got error: %v", result.Error)
	}
}

func TestInfraServiceHandler_ExecuteServiceDeploy_WithComposeFile(t *testing.T) {
	tmpDir := t.TempDir()
	serverDir := filepath.Join(tmpDir, "deployments", "server1")
	if err := os.MkdirAll(serverDir, 0755); err != nil {
		t.Fatal(err)
	}
	composeContent := `version: '3'
services:
  app:
    image: nginx:latest`
	composeFile := filepath.Join(serverDir, "gateway1.compose.yaml")
	if err := os.WriteFile(composeFile, []byte(composeContent), 0644); err != nil {
		t.Fatal(err)
	}

	mockSSH := &mockSSHClient{runStdout: "started"}
	deps := newMockDeps()
	deps.sshClient = mockSSH
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4"}
	deps.serverEntities["server1"] = &entity.Server{Name: "server1"}
	deps.env = "prod"
	deps.workDir = tmpDir

	change := valueobject.NewChange(valueobject.ChangeTypeCreate, "infra_service", "gateway1").
		WithNewState(&entity.InfraService{
			ServiceBase: entity.ServiceBase{
				Server: "server1",
			},
			Name: "gateway1",
			Type: entity.InfraServiceTypeGateway,
			// 内部PRD §6.4：产物校验要求显式 tracker_shipper 段（缺段一律 FAIL），
			// 成功路径用例的实体与上传缓存保持一致：显式 enabled:false（唯一合法豁免形态）。
			GatewayTrackerShipper: &entity.GatewayTrackerShipperConfig{Enabled: false},
		})

	deployCtx := &ServiceDeployContext{
		ServerName: "server1",
		Client:     mockSSH,
		RemoteDir:  "/opt/test",
	}
	result, err := ExecuteServiceDeploy(change, deployCtx, deps, DeployServiceOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success, got error: %v", result.Error)
	}
}

func TestInfraServiceHandler_DeployGatewayType_WithFile(t *testing.T) {
	h := NewInfraServiceHandler()

	tmpDir := t.TempDir()
	serverDir := filepath.Join(tmpDir, "deployments", "prod", "server1")
	if err := os.MkdirAll(serverDir, 0755); err != nil {
		t.Fatal(err)
	}
	gatewayFile := filepath.Join(serverDir, "gateway1.gate.yaml")
	if err := os.WriteFile(gatewayFile, []byte("gateway config"), 0644); err != nil {
		t.Fatal(err)
	}

	mockSSH := &mockSSHClient{}
	deps := newMockDeps()
	deps.sshClient = mockSSH
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4"}
	deps.env = "prod"
	deps.workDir = tmpDir

	deployCtx := &ServiceDeployContext{
		ServerName: "server1",
		Client:     mockSSH,
		RemoteDir:  "/opt/test",
	}

	err := h.deployGatewayType("gateway1", deployCtx, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mockSSH.uploaded) == 0 {
		t.Error("expected file to be uploaded")
	}
}

func TestInfraServiceHandler_GetComposeFilePath(t *testing.T) {
	deps := newMockDeps()
	deps.workDir = "/tmp"
	deps.servers["server1"] = &ServerInfo{}

	change := valueobject.NewChange(valueobject.ChangeTypeNoop, "", "gateway1").
		WithNewState(map[string]interface{}{
			"server": "server1",
		})

	result := GetComposeFilePath(change, deps)
	expected := filepath.Join("/tmp", "deployments", "server1", "gateway1.compose.yaml")
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestInfraServiceHandler_GetGatewayFilePath(t *testing.T) {
	h := &InfraServiceHandler{}

	deps := newMockDeps()
	deps.workDir = "/tmp"

	result := h.getGatewayFilePath("server1", "gateway1", deps)
	expected := filepath.Join("/tmp", "deployments", "server1", "gateway1.gate.yaml")
	if result != expected {
		t.Errorf("expected %s, got %s", expected, result)
	}
}

func TestInfraServiceHandler_DeployGatewayType_NoFile(t *testing.T) {
	h := NewInfraServiceHandler()

	mockSSH := &mockSSHClient{}
	deps := newMockDeps()
	deps.sshClient = mockSSH
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4"}
	deps.env = "prod"
	deps.workDir = t.TempDir()

	deployCtx := &ServiceDeployContext{
		ServerName: "server1",
		Client:     mockSSH,
		RemoteDir:  "/opt/test",
	}

	err := h.deployGatewayType("gateway1", deployCtx, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestInfraServiceHandler_Apply_UptimeKumaDataDirAndTargets 验证 uptime-kuma 部署时：
// 1) 预先创建 data 目录并 chmod 777（仅目录，不递归放开 data 内 db-config.json 等敏感文件）；
// 2) 同步 kuma.targets.yaml 到远端。
func TestInfraServiceHandler_Apply_UptimeKumaDataDirAndTargets(t *testing.T) {
	h := NewInfraServiceHandler()
	ctx := context.Background()

	tmpDir := t.TempDir()
	serverDir := filepath.Join(tmpDir, "deployments", "test", "server1")
	if err := os.MkdirAll(serverDir, 0755); err != nil {
		t.Fatal(err)
	}
	targetsFile := filepath.Join(serverDir, "uptime-kuma-node1.kuma.targets.yaml")
	if err := os.WriteFile(targetsFile, []byte("version: 1\nmonitors: []\n"), 0644); err != nil {
		t.Fatal(err)
	}

	mockSSH := &mockSSHClient{runStdout: validGateYAMLForVerify}
	deps := newMockDeps()
	deps.sshClient = mockSSH
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4", Port: 22, User: "root"}
	deps.serverEntities["server1"] = &entity.Server{Name: "server1"}
	deps.env = "test"
	deps.workDir = tmpDir

	change := valueobject.NewChange(valueobject.ChangeTypeCreate, "infra_service", "uptime-kuma-node1").
		WithNewState(&entity.InfraService{
			ServiceBase: entity.ServiceBase{
				Server: "server1",
			},
			Name:             "uptime-kuma-node1",
			Type:             entity.InfraServiceTypeUptimeKuma,
			UptimeKumaConfig: &entity.UptimeKumaConfig{},
		})

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got error: %v", result.Error)
	}

	foundDataPrep := false
	for _, cmd := range mockSSH.commandsRun {
		if strings.Contains(cmd, "mkdir -p") && strings.Contains(cmd, "chmod 777") && strings.Contains(cmd, "/data") {
			if strings.Contains(cmd, "chmod -R") {
				t.Errorf("uptime-kuma data dir must not use recursive chmod -R 777 (leaks db-config.json)\ncmd: %s", cmd)
			}
			foundDataPrep = true
			break
		}
	}
	if !foundDataPrep {
		t.Errorf("expected uptime-kuma data dir preparation command, commands run: %v", mockSSH.commandsRun)
	}

	foundTargetsUpload := false
	for _, remote := range mockSSH.uploaded {
		if strings.HasSuffix(remote, "uptime-kuma-node1.kuma.targets.yaml") {
			foundTargetsUpload = true
			break
		}
	}
	if !foundTargetsUpload {
		t.Errorf("expected kuma.targets.yaml upload, uploaded: %v", mockSSH.uploaded)
	}
}

// TestInfraServiceHandler_Apply_UptimeKumaNoTargetsFile 验证本地无 targets 文件（旧状态）时静默跳过。
func TestInfraServiceHandler_Apply_UptimeKumaNoTargetsFile(t *testing.T) {
	h := NewInfraServiceHandler()
	ctx := context.Background()

	mockSSH := &mockSSHClient{runStdout: validGateYAMLForVerify}
	deps := newMockDeps()
	deps.sshClient = mockSSH
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4", Port: 22, User: "root"}
	deps.serverEntities["server1"] = &entity.Server{Name: "server1"}
	deps.env = "test"
	deps.workDir = t.TempDir()

	change := valueobject.NewChange(valueobject.ChangeTypeCreate, "infra_service", "uptime-kuma-node1").
		WithNewState(&entity.InfraService{
			ServiceBase: entity.ServiceBase{
				Server: "server1",
			},
			Name:             "uptime-kuma-node1",
			Type:             entity.InfraServiceTypeUptimeKuma,
			UptimeKumaConfig: &entity.UptimeKumaConfig{},
		})

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got error: %v", result.Error)
	}
	for _, remote := range mockSSH.uploaded {
		if strings.HasSuffix(remote, ".kuma.targets.yaml") {
			t.Errorf("expected silent skip when local targets file missing, got upload: %v", remote)
		}
	}
}

// TestInfraServiceHandler_Apply_UptimeKumaEnvFilePerm 验证 uptime-kuma 的 {svc}.env（含明文 DB/ADMIN 密码）
// 远端同步权限为 0600，而不是默认 644。
func TestInfraServiceHandler_Apply_UptimeKumaEnvFilePerm(t *testing.T) {
	h := NewInfraServiceHandler()
	ctx := context.Background()

	tmpDir := t.TempDir()
	serverDir := filepath.Join(tmpDir, "deployments", "test", "server1")
	if err := os.MkdirAll(serverDir, 0755); err != nil {
		t.Fatal(err)
	}
	composeFile := filepath.Join(serverDir, "uptime-kuma-node1.compose.yaml")
	if err := os.WriteFile(composeFile, []byte("version: '3'\nservices: {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	envFile := filepath.Join(serverDir, "uptime-kuma-node1.env")
	if err := os.WriteFile(envFile, []byte("UPTIME_KUMA_DB_PASSWORD=db-secret-val\n"), 0600); err != nil {
		t.Fatal(err)
	}

	mockSSH := &mockSSHClient{runStdout: validGateYAMLForVerify}
	deps := newMockDeps()
	deps.sshClient = mockSSH
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4", Port: 22, User: "root"}
	deps.serverEntities["server1"] = &entity.Server{Name: "server1"}
	deps.env = "test"
	deps.workDir = tmpDir

	change := valueobject.NewChange(valueobject.ChangeTypeCreate, "infra_service", "uptime-kuma-node1").
		WithNewState(&entity.InfraService{
			ServiceBase: entity.ServiceBase{
				Server: "server1",
			},
			Name:             "uptime-kuma-node1",
			Type:             entity.InfraServiceTypeUptimeKuma,
			UptimeKumaConfig: &entity.UptimeKumaConfig{},
		})

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got error: %v", result.Error)
	}

	envPerm := ""
	for remote, perm := range mockSSH.uploadedPerm {
		if strings.HasSuffix(remote, "/uptime-kuma-node1.env") {
			envPerm = perm
			break
		}
	}
	if envPerm == "" {
		t.Fatalf("expected env file upload, uploadedPerm: %v", mockSSH.uploadedPerm)
	}
	if envPerm != "600" {
		t.Errorf("uptime-kuma env file perm = %s, want 600", envPerm)
	}
}

// TestInfraServiceHandler_Apply_NonKumaEnvFilePermDefault 验证非 uptime-kuma 服务的 env 文件
// 仍走默认 644（不改变既有行为）。
func TestInfraServiceHandler_Apply_NonKumaEnvFilePermDefault(t *testing.T) {
	h := NewInfraServiceHandler()
	ctx := context.Background()

	tmpDir := t.TempDir()
	serverDir := filepath.Join(tmpDir, "deployments", "test", "server1")
	if err := os.MkdirAll(serverDir, 0755); err != nil {
		t.Fatal(err)
	}
	composeFile := filepath.Join(serverDir, "gateway1.compose.yaml")
	if err := os.WriteFile(composeFile, []byte("version: '3'\nservices: {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	envFile := filepath.Join(serverDir, "gateway1.env")
	if err := os.WriteFile(envFile, []byte("FOO=bar\n"), 0600); err != nil {
		t.Fatal(err)
	}

	mockSSH := &mockSSHClient{runStdout: validGateYAMLForVerify}
	deps := newMockDeps()
	deps.sshClient = mockSSH
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4", Port: 22, User: "root"}
	deps.serverEntities["server1"] = &entity.Server{Name: "server1"}
	deps.env = "test"
	deps.workDir = tmpDir

	change := valueobject.NewChange(valueobject.ChangeTypeCreate, "infra_service", "gateway1").
		WithNewState(&entity.InfraService{
			ServiceBase: entity.ServiceBase{
				Server: "server1",
			},
			Name: "gateway1",
			Type: entity.InfraServiceTypeGateway,
			// 内部PRD §6.4：产物校验要求显式 tracker_shipper 段（缺段一律 FAIL），
			// 成功路径用例的实体与上传缓存保持一致：显式 enabled:false（唯一合法豁免形态）。
			GatewayTrackerShipper: &entity.GatewayTrackerShipperConfig{Enabled: false},
		})

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got error: %v", result.Error)
	}

	envPerm := ""
	for remote, perm := range mockSSH.uploadedPerm {
		if strings.HasSuffix(remote, "/gateway1.env") {
			envPerm = perm
			break
		}
	}
	if envPerm == "" {
		t.Fatalf("expected env file upload, uploadedPerm: %v", mockSSH.uploadedPerm)
	}
	if envPerm != "644" {
		t.Errorf("non-kuma env file perm = %s, want 644", envPerm)
	}
}

// TestInfraServiceHandler_Apply_StaleCheckoutMissingShipperFails 内部PRD §6.4
// 核心反例：陈旧 checkout 的实体未配置 tracker_shipper（nil），其渲染/缓存产物同样缺段，
// 原样下发后部署后校验必须 FAIL——「双方一致」不构成豁免，该网关判部署失败
// （errors.Is ErrGatewayShipperSegmentMissing），且不自动回滚（容器 up 已执行、无 down 补偿）。
func TestInfraServiceHandler_Apply_StaleCheckoutMissingShipperFails(t *testing.T) {
	h := NewInfraServiceHandler()
	ctx := context.Background()

	tmpDir := t.TempDir()
	serverDir := filepath.Join(tmpDir, "deployments", "test", "server1")
	if err := os.MkdirAll(serverDir, 0755); err != nil {
		t.Fatal(err)
	}
	// 陈旧产物：缺 tracker_shipper 段（旧版生成器/陈旧 checkout 的渲染形态）。
	staleGate := "server:\n  port: 80\nhosts: []\n"
	if err := os.WriteFile(filepath.Join(serverDir, "gateway1.gate.yaml"), []byte(staleGate), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(serverDir, "gateway1.compose.yaml"), []byte("version: '3'\nservices: {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	mockSSH := &mockSSHClient{}
	deps := newMockDeps()
	deps.sshClient = mockSSH
	deps.servers["server1"] = &ServerInfo{Host: "1.2.3.4", Port: 22, User: "root"}
	deps.serverEntities["server1"] = &entity.Server{Name: "server1"}
	deps.env = "test"
	deps.workDir = tmpDir
	// deps.config 保持 nil：不重渲染，缓存原样下发（正是陈旧 checkout 的下发行为）。

	change := valueobject.NewChange(valueobject.ChangeTypeCreate, "infra_service", "gateway1").
		WithNewState(&entity.InfraService{
			ServiceBase: entity.ServiceBase{
				Server: "server1",
			},
			Name: "gateway1",
			Type: entity.InfraServiceTypeGateway,
			// 实体未配置 tracker_shipper（nil）：陈旧 checkout 形态，期望产物必须含段而不可得。
		})

	result, err := h.Apply(ctx, change, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Fatalf("stale checkout (missing tracker_shipper in artifact) must fail the deploy, got success")
	}
	if result.Error == nil {
		t.Fatal("expected result.Error to be set on shipper verification failure")
	}
	if !errors.Is(result.Error, ErrGatewayShipperSegmentMissing) {
		t.Errorf("error = %v, want errors.Is ErrGatewayShipperSegmentMissing", result.Error)
	}
	for _, want := range []string{"server1", "gateway1", "dev/prod 网关必须显式配置该段", "checkout 疑似陈旧"} {
		if !strings.Contains(result.Error.Error(), want) {
			t.Errorf("error %q should contain %q", result.Error, want)
		}
	}

	// 不自动回滚：容器 up 命令已执行（部署动作真实发生过），且无任何 down 补偿命令。
	foundUp := false
	for _, cmd := range mockSSH.commandsRun {
		if strings.Contains(cmd, "docker compose") && strings.Contains(cmd, "up -d") {
			foundUp = true
		}
		if strings.Contains(cmd, "down") {
			t.Errorf("verification failure must not trigger rollback, got down command: %s", cmd)
		}
	}
	if !foundUp {
		t.Errorf("expected docker compose up to have run before verification, commands: %v", mockSSH.commandsRun)
	}
}
