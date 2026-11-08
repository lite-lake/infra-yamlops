package generator

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/lite-lake/infra-yamlops/internal/domain"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
)

func uptimeKumaTestInfra(name string) *entity.InfraService {
	return &entity.InfraService{
		Name:        name,
		Type:        entity.InfraServiceTypeUptimeKuma,
		ServiceBase: entity.ServiceBase{Server: "srv-node1"},
		Image:       "mirror.mesh.example.net/hub/louislam/uptime-kuma:2",
		UptimeKumaConfig: &entity.UptimeKumaConfig{
			Admin: entity.UptimeKumaAdminConfig{
				Username: "admin",
				Password: *valueobject.NewSecretRefSecret("kuma_admin_password"),
			},
			DB: entity.UptimeKumaDBConfig{
				Type:     "mariadb",
				Hostname: "192.168.180.234",
				Port:     39675,
				Name:     "kuma_dev",
				Username: "kuma_dev",
				Password: *valueobject.NewSecretRefSecret("kuma_db_password"),
			},
			Gateways: []entity.UptimeKumaGatewayConfig{
				{Hostname: "uptime.env-dev.mesh.example.net", HTTPS: true, ContainerPort: 3001},
			},
			Sync: entity.UptimeKumaSyncConfig{
				Image:             "docker.cnb.cool/lite-lake/lite-polyglot/app-uptimekuma-sync-svc:dev",
				ReconcileInterval: "60s",
				ReloginInterval:   "1h",
			},
			Notification: entity.UptimeKumaNotificationConfig{
				Enabled: true,
				URL:     "https://mc.env-dev.mesh.example.net/webhook/wecom/lite-lake-service-event",
			},
			Monitors: []entity.UptimeKumaMonitorConfig{
				{
					Name:                     "litelake-首页",
					Type:                     "http",
					URL:                      "https://www.example.com",
					Interval:                 60,
					AcceptedStatuscodes:      []string{"200-299"},
					MaxRedirects:             10,
					ExpiryNotification:       true,
					DomainExpiryNotification: true,
				},
				{
					Name:     "网关443-TLS",
					Type:     "port",
					Hostname: "uptime.mesh.example.net",
					Port:     443,
				},
			},
		},
	}
}

func TestGenerateUptimeKumaConfig(t *testing.T) {
	tmpDir := t.TempDir()
	gen := NewGenerator("prod", tmpDir)

	infra := uptimeKumaTestInfra("uptime-kuma-node1")
	config := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"},
		Secrets: []entity.Secret{
			{Name: "kuma_admin_password", Value: "admin-secret-val"},
			{Name: "kuma_db_password", Value: "db-secret-val"},
		},
	}

	if err := gen.generateUptimeKumaConfig(tmpDir, infra, config); err != nil {
		t.Fatalf("generateUptimeKumaConfig() error = %v", err)
	}

	t.Run("targets yaml", func(t *testing.T) {
		content, err := os.ReadFile(filepath.Join(tmpDir, "uptime-kuma-node1.kuma.targets.yaml"))
		if err != nil {
			t.Fatalf("failed to read targets: %v", err)
		}
		s := string(content)
		checks := []string{
			"version: 1",
			"enabled: true",
			"https://mc.env-dev.mesh.example.net/webhook/wecom/lite-lake-service-event",
			"name: litelake-首页",
			"type: http",
			"url: https://www.example.com",
			"interval: 60",
			"accepted_statuscodes:",
			"- 200-299",
			"maxredirects: 10",
			"expiryNotification: true",
			"domainExpiryNotification: true",
			"tags:",
			"- yamlops",
			"name: 网关443-TLS",
			"type: port",
			"hostname: uptime.mesh.example.net",
			"port: 443",
		}
		for _, c := range checks {
			if !strings.Contains(s, c) {
				t.Errorf("targets.yaml missing %q\n---\n%s", c, s)
			}
		}
		if len(infra.UptimeKumaConfig.Monitors) != 2 {
			t.Fatalf("expected 2 monitors, got %d", len(infra.UptimeKumaConfig.Monitors))
		}
	})

	t.Run("env file", func(t *testing.T) {
		content, err := os.ReadFile(filepath.Join(tmpDir, "uptime-kuma-node1.env"))
		if err != nil {
			t.Fatalf("failed to read env: %v", err)
		}
		// 0600 权限断言仅 POSIX 语义：Windows 的 os.Stat 不反映 POSIX perm（恒为 0666），跳过
		if runtime.GOOS != "windows" {
			info, _ := os.Stat(filepath.Join(tmpDir, "uptime-kuma-node1.env"))
			if info != nil && info.Mode().Perm() != 0600 {
				t.Errorf("env file mode = %v, want 0600", info.Mode().Perm())
			}
		}
		s := string(content)
		checks := []string{
			"UPTIME_KUMA_DB_TYPE=mariadb",
			"UPTIME_KUMA_DB_HOSTNAME=192.168.180.234",
			"UPTIME_KUMA_DB_PORT=39675",
			"UPTIME_KUMA_DB_NAME=kuma_dev",
			"UPTIME_KUMA_DB_USERNAME=kuma_dev",
			"UPTIME_KUMA_DB_PASSWORD=db-secret-val",
			"UPTIME_KUMA_WS_ORIGIN_CHECK=bypass",
			"KUMA_URL=http://uptime-kuma-node1:3001",
			"KUMA_ADMIN_USERNAME=admin",
			"KUMA_ADMIN_PASSWORD=admin-secret-val",
			"KUMA_TARGETS_FILE=/targets.yaml",
			"RECONCILE_INTERVAL=60s",
			"KUMA_RELOGIN_INTERVAL=1h",
			"WEBHOOK_LISTEN_PORT=37402",
			"KUMA_NOTIFY_WEBHOOK_URL=http://uptime-kuma-node1-agent:37402/webhook/kuma",
		}
		for _, c := range checks {
			if !strings.Contains(s, c) {
				t.Errorf("env file missing %q\n---\n%s", c, s)
			}
		}
	})

	t.Run("compose dual services", func(t *testing.T) {
		content, err := os.ReadFile(filepath.Join(tmpDir, "uptime-kuma-node1.compose.yaml"))
		if err != nil {
			t.Fatalf("failed to read compose: %v", err)
		}
		s := string(content)
		checks := []string{
			"image: mirror.mesh.example.net/hub/louislam/uptime-kuma:2",
			"container_name: yo-prod-uptime-kuma-node1",
			"image: docker.cnb.cool/lite-lake/lite-polyglot/app-uptimekuma-sync-svc:dev",
			"container_name: yo-prod-uptime-kuma-node1-agent",
			"yamlops-prod",
			"- ./data:/app/data",
			"- ./uptime-kuma-node1.kuma.targets.yaml:/targets.yaml:ro",
			"- uptime-kuma-node1.env",
			"- CMD",
			"- curl",
			"- -fsS",
			"- http://127.0.0.1:3001",
			"interval: 10s",
			"retries: 5",
			"start_period: 20s",
			"service_healthy",
			"host.docker.internal:host-gateway",
		}
		for _, c := range checks {
			if !strings.Contains(s, c) {
				t.Errorf("compose missing %q\n---\n%s", c, s)
			}
		}
		if strings.Contains(s, "admin-secret-val") || strings.Contains(s, "db-secret-val") {
			t.Errorf("compose must not contain resolved secrets\n---\n%s", s)
		}
		// depends_on 需要引用主容器实际 compose key（yo-prod-uptime-kuma-node1）
		if !strings.Contains(s, "yo-prod-uptime-kuma-node1") ||
			!strings.Contains(s, "condition: service_healthy") {
			t.Errorf("compose depends_on should reference main container key\n---\n%s", s)
		}
	})
}

func TestGenerateUptimeKumaConfig_DefaultInterval(t *testing.T) {
	tmpDir := t.TempDir()
	gen := NewGenerator("prod", tmpDir)

	infra := uptimeKumaTestInfra("uptime-kuma-node1")
	infra.UptimeKumaConfig.Sync.ReconcileInterval = ""
	infra.UptimeKumaConfig.Sync.ReloginInterval = ""
	config := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"},
		Secrets: []entity.Secret{
			{Name: "kuma_admin_password", Value: "a"},
			{Name: "kuma_db_password", Value: "d"},
		},
	}

	if err := gen.generateUptimeKumaConfig(tmpDir, infra, config); err != nil {
		t.Fatalf("generateUptimeKumaConfig() error = %v", err)
	}
	content, err := os.ReadFile(filepath.Join(tmpDir, "uptime-kuma-node1.env"))
	if err != nil {
		t.Fatalf("failed to read env: %v", err)
	}
	if !strings.Contains(string(content), "RECONCILE_INTERVAL=60s") {
		t.Errorf("expected default RECONCILE_INTERVAL=60s, got:\n%s", string(content))
	}
	if strings.Contains(string(content), "KUMA_RELOGIN_INTERVAL") {
		t.Errorf("empty relogin_interval must not write KUMA_RELOGIN_INTERVAL (agent default 1h), got:\n%s", string(content))
	}
}

func TestGenerateUptimeKumaConfig_ReloginIntervalZero(t *testing.T) {
	tmpDir := t.TempDir()
	gen := NewGenerator("prod", tmpDir)

	infra := uptimeKumaTestInfra("uptime-kuma-node1")
	infra.UptimeKumaConfig.Sync.ReloginInterval = "0s"
	config := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"},
		Secrets: []entity.Secret{
			{Name: "kuma_admin_password", Value: "a"},
			{Name: "kuma_db_password", Value: "d"},
		},
	}

	if err := gen.generateUptimeKumaConfig(tmpDir, infra, config); err != nil {
		t.Fatalf("generateUptimeKumaConfig() error = %v", err)
	}
	content, err := os.ReadFile(filepath.Join(tmpDir, "uptime-kuma-node1.env"))
	if err != nil {
		t.Fatalf("failed to read env: %v", err)
	}
	if !strings.Contains(string(content), "KUMA_RELOGIN_INTERVAL=0s") {
		t.Errorf("expected KUMA_RELOGIN_INTERVAL=0s passthrough, got:\n%s", string(content))
	}
}

func TestGenerateUptimeKumaConfig_MissingSecret(t *testing.T) {
	tmpDir := t.TempDir()
	gen := NewGenerator("prod", tmpDir)

	infra := uptimeKumaTestInfra("uptime-kuma-node1")
	config := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"}, Secrets: nil}

	err := gen.generateUptimeKumaConfig(tmpDir, infra, config)
	if err == nil {
		t.Fatal("expected error for missing secret")
	}
	if !strings.Contains(err.Error(), "admin password") {
		t.Errorf("error should mention admin password, got: %v", err)
	}
}

// TestGenerateUptimeKumaTargets_ExtraFields 验证白名单外 camelCase 字段经 Extra 原样写入 targets.yaml。
func TestGenerateUptimeKumaTargets_ExtraFields(t *testing.T) {
	uk := uptimeKumaTestInfra("uptime-kuma-node1").UptimeKumaConfig
	uk.Monitors = append(uk.Monitors, entity.UptimeKumaMonitorConfig{
		Name:     "dns-高级",
		Type:     "dns",
		Hostname: "example.com",
		Extra: map[string]any{
			"dns_resolve_type": "A",
			"packetSize":       512,
			"method":           "POST",
			"timeout":          10000,
		},
	})

	content, err := generateUptimeKumaTargets(uk, nil)
	if err != nil {
		t.Fatalf("generateUptimeKumaTargets() error = %v", err)
	}
	checks := []string{
		"name: dns-高级",
		"type: dns",
		"hostname: example.com",
		"dns_resolve_type: A",
		"packetSize: 512",
		"method: POST",
		"timeout: 10000",
	}
	for _, c := range checks {
		if !strings.Contains(content, c) {
			t.Errorf("targets.yaml missing %q\n---\n%s", c, content)
		}
	}
}

// TestGenerateUptimeKumaTargets_ExtraSecretResolution 表驱动验证 monitor Extra 的
// {secret: name} 递归解析（内部PRD V6）：解析成功 / secret 缺失 fail-closed 报错 /
// 普通值透传不受影响 / headers 嵌套 map 内层解析 / 切片内递归 / 非 {secret:} 形状不误解析。
func TestGenerateUptimeKumaTargets_ExtraSecretResolution(t *testing.T) {
	secrets := map[string]string{
		"kuma_auth_header": "Bearer s3cr3t",
		"kuma_token":       "tok-1",
	}

	tests := []struct {
		name        string
		extra       map[string]any
		wantContain []string // 期望 targets.yaml 文本包含
		wantAbsent  []string // 期望 targets.yaml 文本不包含
		wantErr     error    // 期望 errors.Is 命中的 sentinel；nil = 期望成功
		wantErrText []string // 期望错误信息包含（monitor 名 / secret 名）
	}{
		{
			name: "headers 嵌套 map 内层 {secret:} 解析",
			extra: map[string]any{
				"headers": map[string]any{
					"Authorization": map[string]any{"secret": "kuma_auth_header"},
					"X-Flag":        "plain-value",
				},
			},
			wantContain: []string{"Authorization: Bearer s3cr3t", "X-Flag: plain-value"},
			wantAbsent:  []string{"{secret:", "kuma_auth_header"},
		},
		{
			name: "secret 缺失 fail-closed 报错",
			extra: map[string]any{
				"headers": map[string]any{
					"Authorization": map[string]any{"secret": "no_such_secret"},
				},
			},
			wantErr:     domain.ErrMissingSecret,
			wantErrText: []string{"m-secret", "no_such_secret"},
		},
		{
			name:        "普通值透传不受影响",
			extra:       map[string]any{"packetSize": 512, "method": "POST", "timeout": 10000, "ok": true},
			wantContain: []string{"packetSize: 512", "method: POST", "timeout: 10000", "ok: true"},
		},
		{
			name: "切片内递归解析",
			extra: map[string]any{
				"push_token_chain": []any{"plain-item", map[string]any{"secret": "kuma_token"}, map[string]any{"secret": "kuma_auth_header"}},
			},
			wantContain: []string{"- plain-item", "- tok-1", "- Bearer s3cr3t"},
			wantAbsent:  []string{"kuma_token"},
		},
		{
			name:        "恰好一个 secret key 才是引用：多 key map 透传",
			extra:       map[string]any{"odd": map[string]any{"secret": "x", "other": 1}},
			wantContain: []string{"secret: x", "other: 1"},
		},
		{
			name:        "secret 值非 string 不是引用：透传",
			extra:       map[string]any{"numeric": map[string]any{"secret": 123}},
			wantContain: []string{"secret: 123"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uk := uptimeKumaTestInfra("uptime-kuma-node1").UptimeKumaConfig
			uk.Monitors = []entity.UptimeKumaMonitorConfig{{
				Name:  "m-secret",
				Type:  "http",
				URL:   "https://example.com",
				Extra: tt.extra,
			}}

			content, err := generateUptimeKumaTargets(uk, secrets)
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("generateUptimeKumaTargets() expected error, got content:\n%s", content)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("generateUptimeKumaTargets() error = %v, want errors.Is %v", err, tt.wantErr)
				}
				for _, want := range tt.wantErrText {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("error %q should contain %q", err, want)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("generateUptimeKumaTargets() unexpected error = %v", err)
			}
			for _, want := range tt.wantContain {
				if !strings.Contains(content, want) {
					t.Errorf("targets.yaml missing %q\n---\n%s", want, content)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(content, absent) {
					t.Errorf("targets.yaml should not contain %q\n---\n%s", absent, content)
				}
			}
		})
	}
}

func TestCollectUptimeKumaHostRoutes(t *testing.T) {
	gen := NewGenerator("prod", t.TempDir())

	gw := &entity.InfraService{
		Name:         "gateway-node1",
		Type:         entity.InfraServiceTypeGateway,
		ServiceBase:  entity.ServiceBase{Server: "srv-node1"},
		Image:        "gate:latest",
		GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443},
	}

	kuma := uptimeKumaTestInfra("uptime-kuma-node1")
	kuma.ServiceBase.Server = "srv-node1"

	config := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"},
		InfraServices: []entity.InfraService{*gw, *kuma},
	}

	hosts := gen.collectUptimeKumaHostRoutes(gw, config)
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host route, got %d", len(hosts))
	}
	h := hosts[0]
	if h.Name != "uptime.env-dev.mesh.example.net" {
		t.Errorf("Name = %q, want uptime.env-dev.mesh.example.net", h.Name)
	}
	if h.Port != 80 {
		t.Errorf("Port = %d, want 80", h.Port)
	}
	if h.SSLPort != 443 {
		t.Errorf("SSLPort = %d, want 443", h.SSLPort)
	}
	if len(h.Backend) != 1 || h.Backend[0] != "http://uptime-kuma-node1:3001" {
		t.Errorf("Backend = %v, want [http://uptime-kuma-node1:3001]", h.Backend)
	}
	if !h.PreserveHostHeader {
		t.Error("PreserveHostHeader should be true")
	}
	if h.HealthCheck != "/" {
		t.Errorf("HealthCheck = %q, want /", h.HealthCheck)
	}
	if h.HealthCheckInterval != "60s" {
		t.Errorf("HealthCheckInterval = %q, want 60s", h.HealthCheckInterval)
	}
	if h.HealthCheckTimeout != "10s" {
		t.Errorf("HealthCheckTimeout = %q, want 10s", h.HealthCheckTimeout)
	}
}

func TestCollectUptimeKumaHostRoutes_CustomContainerPort(t *testing.T) {
	gen := NewGenerator("prod", t.TempDir())

	gw := &entity.InfraService{
		Name:         "gateway-node1",
		Type:         entity.InfraServiceTypeGateway,
		ServiceBase:  entity.ServiceBase{Server: "srv-node1"},
		Image:        "gate:latest",
		GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443},
	}

	kuma := uptimeKumaTestInfra("uptime-kuma-node1")
	kuma.UptimeKumaConfig.Gateways = []entity.UptimeKumaGatewayConfig{
		{Hostname: "up.example.com", HTTPS: true, ContainerPort: 3005},
	}

	config := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"},
		InfraServices: []entity.InfraService{*gw, *kuma},
	}

	hosts := gen.collectUptimeKumaHostRoutes(gw, config)
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host route, got %d", len(hosts))
	}
	if hosts[0].Backend[0] != "http://uptime-kuma-node1:3005" {
		t.Errorf("Backend = %v, want http://uptime-kuma-node1:3005", hosts[0].Backend)
	}
}

// TestCollectUptimeKumaHostRoutes_HTTPSOff 验证 https: false 的 gateway host 不注册 SSLPort
// （与业务服务 buildHostRoute 语义一致：只有显式开启 https 才在 443 提供服务）。
func TestCollectUptimeKumaHostRoutes_HTTPSOff(t *testing.T) {
	gen := NewGenerator("prod", t.TempDir())

	gw := &entity.InfraService{
		Name:         "gateway-node1",
		Type:         entity.InfraServiceTypeGateway,
		ServiceBase:  entity.ServiceBase{Server: "srv-node1"},
		Image:        "gate:latest",
		GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443},
	}

	kuma := uptimeKumaTestInfra("uptime-kuma-node1")
	kuma.UptimeKumaConfig.Gateways = []entity.UptimeKumaGatewayConfig{
		{Hostname: "up.example.com", HTTPS: false, ContainerPort: 3001},
	}

	config := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"},
		InfraServices: []entity.InfraService{*gw, *kuma},
	}

	hosts := gen.collectUptimeKumaHostRoutes(gw, config)
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host route, got %d", len(hosts))
	}
	if hosts[0].SSLPort != 0 {
		t.Errorf("SSLPort = %d, want 0 when https is false", hosts[0].SSLPort)
	}
	if hosts[0].Port != 80 {
		t.Errorf("Port = %d, want 80", hosts[0].Port)
	}
	if !hosts[0].PreserveHostHeader {
		t.Error("PreserveHostHeader should be true")
	}
}

func TestCollectUptimeKumaHostRoutes_DifferentServer(t *testing.T) {
	gen := NewGenerator("prod", t.TempDir())

	gw := &entity.InfraService{
		Name:         "gateway-node5",
		Type:         entity.InfraServiceTypeGateway,
		ServiceBase:  entity.ServiceBase{Server: "srv-node5"},
		Image:        "gate:latest",
		GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443},
	}

	kuma := uptimeKumaTestInfra("uptime-kuma-node1")
	kuma.ServiceBase.Server = "srv-other"

	config := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"},
		InfraServices: []entity.InfraService{*gw, *kuma},
	}

	hosts := gen.collectUptimeKumaHostRoutes(gw, config)
	if len(hosts) != 0 {
		t.Errorf("expected 0 hosts (kuma on different server), got %d", len(hosts))
	}
}
