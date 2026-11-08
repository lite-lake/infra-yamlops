package cli

import (
	"strings"
	"testing"

	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
)

func uptimeKumaShowInfra(name, server string) *entity.InfraService {
	return &entity.InfraService{
		Name:        name,
		Type:        entity.InfraServiceTypeUptimeKuma,
		ServiceBase: entity.ServiceBase{Server: server},
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
				{Name: "litelake-首页", Type: "http", URL: "https://www.example.com", Interval: 60},
				{Name: "网关443-TLS", Type: "port", Hostname: "uptime.mesh.example.net", Port: 443},
			},
		},
	}
}

func TestFormatUptimeKumaDetail(t *testing.T) {
	infra := uptimeKumaShowInfra("uptime-kuma-node1", "srv-node1")
	lines := formatUptimeKumaDetail(infra)
	out := strings.Join(lines, "\n")

	checks := []string{
		"  Endpoints:  https://uptime.env-dev.mesh.example.net",
		"  Gateways:   uptime.env-dev.mesh.example.net (https)",
		"  Health:     http://127.0.0.1:3001 (interval: 10s)",
		"  Monitors:",
		"  - 名称: litelake-首页 | type: http | url: https://www.example.com",
		"  - 名称: 网关443-TLS | type: port | hostname: uptime.mesh.example.net:443",
		"  Notification: enabled: true | url: https://mc.env-dev.mesh.example.net/webhook/wecom/lite-lake-service-event",
		"  Sync agent: image: docker.cnb.cool/lite-lake/lite-polyglot/app-uptimekuma-sync-svc:dev | reconcile_interval: 60s | relogin_interval: 1h",
		"  Admin: username: admin | password: {secret: kuma_admin_password}",
		"  DB: type: mariadb | hostname: 192.168.180.234 | name: kuma_dev | username: kuma_dev",
	}
	for _, c := range checks {
		if !strings.Contains(out, c) {
			t.Errorf("formatUptimeKumaDetail() missing %q\n---\n%s", c, out)
		}
	}

	for _, secretVal := range []string{"admin-secret-val", "db-secret-val", "kuma_admin_password\"", "password: admin"} {
		if strings.Contains(out, secretVal) {
			t.Errorf("formatUptimeKumaDetail() must not leak password, found %q\n---\n%s", secretVal, out)
		}
	}
}

func TestFormatUptimeKumaDetail_HTTPSOff(t *testing.T) {
	infra := uptimeKumaShowInfra("uptime-kuma-node1", "srv-node1")
	infra.UptimeKumaConfig.Gateways[0].HTTPS = false
	out := strings.Join(formatUptimeKumaDetail(infra), "\n")
	if !strings.Contains(out, "  Endpoints:  http://uptime.env-dev.mesh.example.net") {
		t.Errorf("expected http endpoint when https off\n---\n%s", out)
	}
}

func TestFormatUptimeKumaDetail_Defaults(t *testing.T) {
	infra := uptimeKumaShowInfra("uptime-kuma-node1", "srv-node1")
	uk := infra.UptimeKumaConfig
	uk.Sync.ReconcileInterval = ""
	uk.Sync.ReloginInterval = ""
	uk.Notification.Enabled = false
	uk.Admin.Password = *valueobject.NewSecretRefPlain("plain-admin")
	uk.Monitors = nil

	out := strings.Join(formatUptimeKumaDetail(infra), "\n")
	checks := []string{
		"  Notification: enabled: false",
		"  Sync agent: image: docker.cnb.cool/lite-lake/lite-polyglot/app-uptimekuma-sync-svc:dev | reconcile_interval: 60s | relogin_interval: 1h (agent default)",
		"  Admin: username: admin",
		"  Monitors:   (none)",
	}
	for _, c := range checks {
		if !strings.Contains(out, c) {
			t.Errorf("formatUptimeKumaDetail() missing %q\n---\n%s", c, out)
		}
	}
	if strings.Contains(out, "plain-admin") {
		t.Errorf("formatUptimeKumaDetail() must not leak plaintext admin password\n---\n%s", out)
	}
}

func TestUptimeKumaMonitorTarget(t *testing.T) {
	tests := []struct {
		name string
		m    entity.UptimeKumaMonitorConfig
		want string
	}{
		{"http url", entity.UptimeKumaMonitorConfig{Type: "http", URL: "https://a.com"}, "url: https://a.com"},
		{"port hostname:port", entity.UptimeKumaMonitorConfig{Type: "port", Hostname: "up.example.com", Port: 443}, "hostname: up.example.com:443"},
		{"ping hostname only", entity.UptimeKumaMonitorConfig{Type: "ping", Hostname: "up.example.com"}, "hostname: up.example.com"},
		{"no target", entity.UptimeKumaMonitorConfig{Type: "push"}, "target: (none)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := uptimeKumaMonitorTarget(tt.m); got != tt.want {
				t.Errorf("uptimeKumaMonitorTarget() = %q, want %q", got, tt.want)
			}
		})
	}
}
