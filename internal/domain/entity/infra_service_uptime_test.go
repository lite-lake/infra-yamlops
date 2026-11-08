package entity

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/lite-lake/infra-yamlops/internal/domain"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
	"gopkg.in/yaml.v3"
)

func uptimeKumaTestYAML() string {
	return `
name: uptime-kuma-node1
type: uptime-kuma
server: srv-node1
image: mirror.mesh.example.net/hub/louislam/uptime-kuma:2
admin:
  username: admin
  password: {secret: kuma_admin_password}
db:
  type: mariadb
  hostname: 192.168.180.234
  port: 39675
  name: kuma_dev
  username: kuma_dev
  password: {secret: kuma_db_password}
gateways:
  - hostname: uptime.env-dev.mesh.example.net
    https: true
    container_port: 3001
sync:
  image: docker.cnb.cool/lite-lake/lite-polyglot/app-uptimekuma-sync-svc:dev
  reconcile_interval: 60s
  relogin_interval: 1h
notification:
  enabled: true
  url: "https://mc.env-dev.mesh.example.net/webhook/wecom/lite-lake-service-event"
monitors:
  - name: litelake-首页
    type: http
    url: https://www.example.com
    interval: 60
    accepted_statuscodes: ["200-299"]
    maxredirects: 10
    expiryNotification: true
    domainExpiryNotification: true
  - name: 网关443-TLS
    type: port
    hostname: uptime.mesh.example.net
    port: 443
`
}

func TestInfraService_UnmarshalYAML_UptimeKuma(t *testing.T) {
	var svc InfraService
	if err := yaml.Unmarshal([]byte(uptimeKumaTestYAML()), &svc); err != nil {
		t.Fatalf("UnmarshalYAML failed: %v", err)
	}
	if svc.Type != InfraServiceTypeUptimeKuma {
		t.Fatalf("Type = %q, want %q", svc.Type, InfraServiceTypeUptimeKuma)
	}
	if svc.UptimeKumaConfig == nil {
		t.Fatal("UptimeKumaConfig is nil")
	}
	uk := svc.UptimeKumaConfig
	if uk.Admin.Username != "admin" {
		t.Errorf("Admin.Username = %q, want admin", uk.Admin.Username)
	}
	if uk.Admin.Password.Secret() != "kuma_admin_password" {
		t.Errorf("Admin.Password secret = %q, want kuma_admin_password", uk.Admin.Password.Secret())
	}
	if uk.DB.Type != "mariadb" || uk.DB.Hostname != "192.168.180.234" || uk.DB.Port != 39675 ||
		uk.DB.Name != "kuma_dev" || uk.DB.Username != "kuma_dev" {
		t.Errorf("DB = %+v, unexpected", uk.DB)
	}
	if uk.DB.Password.Secret() != "kuma_db_password" {
		t.Errorf("DB.Password secret = %q, want kuma_db_password", uk.DB.Password.Secret())
	}
	if len(uk.Gateways) != 1 {
		t.Fatalf("Gateways len = %d, want 1", len(uk.Gateways))
	}
	gw := uk.Gateways[0]
	if gw.Hostname != "uptime.env-dev.mesh.example.net" || !gw.HTTPS || gw.ContainerPort != 3001 {
		t.Errorf("Gateways[0] = %+v, unexpected", gw)
	}
	if uk.Sync.Image != "docker.cnb.cool/lite-lake/lite-polyglot/app-uptimekuma-sync-svc:dev" {
		t.Errorf("Sync.Image = %q, unexpected", uk.Sync.Image)
	}
	if uk.Sync.ReconcileInterval != "60s" {
		t.Errorf("Sync.ReconcileInterval = %q, want 60s", uk.Sync.ReconcileInterval)
	}
	if uk.Sync.ReloginInterval != "1h" {
		t.Errorf("Sync.ReloginInterval = %q, want 1h", uk.Sync.ReloginInterval)
	}
	if !uk.Notification.Enabled || uk.Notification.URL == "" {
		t.Errorf("Notification = %+v, unexpected", uk.Notification)
	}
	if len(uk.Monitors) != 2 {
		t.Fatalf("Monitors len = %d, want 2", len(uk.Monitors))
	}
	m := uk.Monitors[0]
	if m.Name != "litelake-首页" || m.Type != "http" || m.URL != "https://www.example.com" ||
		m.Interval != 60 || !m.ExpiryNotification || !m.DomainExpiryNotification {
		t.Errorf("Monitors[0] = %+v, unexpected", m)
	}
	if len(m.AcceptedStatuscodes) != 1 || m.AcceptedStatuscodes[0] != "200-299" {
		t.Errorf("Monitors[0].AcceptedStatuscodes = %v, want [200-299]", m.AcceptedStatuscodes)
	}
	if m.MaxRedirects != 10 {
		t.Errorf("Monitors[0].MaxRedirects = %d, want 10", m.MaxRedirects)
	}
	if err := svc.Validate(); err != nil {
		t.Errorf("Validate() unexpected error = %v", err)
	}
}

func TestInfraService_MarshalYAML_UptimeKuma(t *testing.T) {
	svc := testUptimeKumaInfra()
	data, err := yaml.Marshal(svc)
	if err != nil {
		t.Fatalf("MarshalYAML failed: %v", err)
	}
	s := string(data)
	checks := []string{
		"type: uptime-kuma",
		"image: mirror.mesh.example.net/hub/louislam/uptime-kuma:2",
		"username: admin",
		"secret: kuma_admin_password",
		"hostname: 192.168.180.234",
		"port: 39675",
		"uptime.env-dev.mesh.example.net",
		"app-uptimekuma-sync-svc:dev",
		"reconcile_interval: 60s",
		"relogin_interval: 1h",
		"enabled: true",
		"expiryNotification: true",
		"domainExpiryNotification: true",
		"accepted_statuscodes:",
		"- 200-299",
		"maxredirects: 10",
	}
	for _, c := range checks {
		if !strings.Contains(s, c) {
			t.Errorf("marshaled yaml missing %q\n---\n%s", c, s)
		}
	}
}

func TestInfraService_MarshalYAML_UptimeKuma_Roundtrip(t *testing.T) {
	var first InfraService
	if err := yaml.Unmarshal([]byte(uptimeKumaTestYAML()), &first); err != nil {
		t.Fatalf("first Unmarshal failed: %v", err)
	}
	marshaled, err := yaml.Marshal(&first)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var second InfraService
	if err := yaml.Unmarshal(marshaled, &second); err != nil {
		t.Fatalf("second Unmarshal failed: %v", err)
	}
	if second.Type != InfraServiceTypeUptimeKuma || second.UptimeKumaConfig == nil {
		t.Fatal("roundtrip lost uptime-kuma config")
	}
	uk1, uk2 := first.UptimeKumaConfig, second.UptimeKumaConfig
	if uk1.Admin.Username != uk2.Admin.Username || uk1.Admin.Password.Secret() != uk2.Admin.Password.Secret() {
		t.Errorf("admin mismatch: %+v vs %+v", uk1.Admin, uk2.Admin)
	}
	if uk1.DB != uk2.DB {
		t.Errorf("db mismatch: %+v vs %+v", uk1.DB, uk2.DB)
	}
	if len(uk1.Gateways) != len(uk2.Gateways) || uk1.Gateways[0] != uk2.Gateways[0] {
		t.Errorf("gateways mismatch: %+v vs %+v", uk1.Gateways, uk2.Gateways)
	}
	if len(uk1.Monitors) != len(uk2.Monitors) {
		t.Errorf("monitors length mismatch: %d vs %d", len(uk1.Monitors), len(uk2.Monitors))
	}
}

func testUptimeKumaInfra() *InfraService {
	return &InfraService{
		Name:        "uptime-kuma-node1",
		Type:        InfraServiceTypeUptimeKuma,
		ServiceBase: ServiceBase{Server: "srv-node1"},
		Image:       "mirror.mesh.example.net/hub/louislam/uptime-kuma:2",
		UptimeKumaConfig: &UptimeKumaConfig{
			Admin: UptimeKumaAdminConfig{
				Username: "admin",
				Password: *valueobject.NewSecretRefSecret("kuma_admin_password"),
			},
			DB: UptimeKumaDBConfig{
				Type:     "mariadb",
				Hostname: "192.168.180.234",
				Port:     39675,
				Name:     "kuma_dev",
				Username: "kuma_dev",
				Password: *valueobject.NewSecretRefSecret("kuma_db_password"),
			},
			Gateways: []UptimeKumaGatewayConfig{
				{Hostname: "uptime.env-dev.mesh.example.net", HTTPS: true, ContainerPort: 3001},
			},
			Sync: UptimeKumaSyncConfig{
				Image:             "docker.cnb.cool/lite-lake/lite-polyglot/app-uptimekuma-sync-svc:dev",
				ReconcileInterval: "60s",
				ReloginInterval:   "1h",
			},
			Notification: UptimeKumaNotificationConfig{
				Enabled: true,
				URL:     "https://mc.env-dev.mesh.example.net/webhook/wecom/lite-lake-service-event",
			},
			Monitors: []UptimeKumaMonitorConfig{
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
			},
		},
	}
}

func TestInfraService_Validate_UptimeKuma(t *testing.T) {
	valid := func() *UptimeKumaConfig {
		return &UptimeKumaConfig{
			Admin: UptimeKumaAdminConfig{
				Username: "admin",
				Password: *valueobject.NewSecretRefSecret("kuma_admin_password"),
			},
			DB: UptimeKumaDBConfig{
				Type:     "mariadb",
				Hostname: "192.168.0.1",
				Port:     3306,
				Name:     "kuma",
				Username: "kuma",
				Password: *valueobject.NewSecretRefSecret("kuma_db_password"),
			},
			Gateways: []UptimeKumaGatewayConfig{{Hostname: "uptime.example.com", HTTPS: true}},
			Sync:     UptimeKumaSyncConfig{Image: "sync:1", ReconcileInterval: "60s"},
		}
	}
	tests := []struct {
		name    string
		mutate  func(*UptimeKumaConfig)
		wantErr error
	}{
		{"valid", func(c *UptimeKumaConfig) {}, nil},
		{"missing admin username", func(c *UptimeKumaConfig) { c.Admin.Username = "" }, domain.ErrRequired},
		{"empty admin password", func(c *UptimeKumaConfig) { c.Admin.Password = valueobject.SecretRef{} }, domain.ErrEmptyValue},
		{"missing db type", func(c *UptimeKumaConfig) { c.DB.Type = "" }, domain.ErrRequired},
		{"invalid db type", func(c *UptimeKumaConfig) { c.DB.Type = "postgres" }, domain.ErrInvalidType},
		{"missing db hostname", func(c *UptimeKumaConfig) { c.DB.Hostname = "" }, domain.ErrRequired},
		{"invalid db port", func(c *UptimeKumaConfig) { c.DB.Port = 0 }, domain.ErrInvalidPort},
		{"missing db name", func(c *UptimeKumaConfig) { c.DB.Name = "" }, domain.ErrRequired},
		{"missing db username", func(c *UptimeKumaConfig) { c.DB.Username = "" }, domain.ErrRequired},
		{"empty db password", func(c *UptimeKumaConfig) { c.DB.Password = valueobject.SecretRef{} }, domain.ErrEmptyValue},
		{"empty gateways", func(c *UptimeKumaConfig) { c.Gateways = nil }, domain.ErrRequired},
		{"gateway empty hostname", func(c *UptimeKumaConfig) { c.Gateways = []UptimeKumaGatewayConfig{{}} }, domain.ErrRequired},
		{"missing sync image", func(c *UptimeKumaConfig) { c.Sync.Image = "" }, domain.ErrRequired},
		{"invalid reconcile_interval", func(c *UptimeKumaConfig) { c.Sync.ReconcileInterval = "abc" }, domain.ErrInvalidType},
		{"non-positive reconcile_interval", func(c *UptimeKumaConfig) { c.Sync.ReconcileInterval = "-5s" }, domain.ErrInvalidType},
		{"zero reconcile_interval", func(c *UptimeKumaConfig) { c.Sync.ReconcileInterval = "0s" }, domain.ErrInvalidType},
		{"valid reconcile_interval", func(c *UptimeKumaConfig) { c.Sync.ReconcileInterval = "90s" }, nil},
		{"empty relogin_interval uses agent default", func(c *UptimeKumaConfig) { c.Sync.ReloginInterval = "" }, nil},
		{"valid relogin_interval", func(c *UptimeKumaConfig) { c.Sync.ReloginInterval = "1h" }, nil},
		{"zero relogin_interval disables fallback", func(c *UptimeKumaConfig) { c.Sync.ReloginInterval = "0s" }, nil},
		{"invalid relogin_interval", func(c *UptimeKumaConfig) { c.Sync.ReloginInterval = "abc" }, domain.ErrInvalidType},
		{"negative relogin_interval", func(c *UptimeKumaConfig) { c.Sync.ReloginInterval = "-1h" }, domain.ErrInvalidType},
		{"notification enabled without url", func(c *UptimeKumaConfig) {
			c.Notification = UptimeKumaNotificationConfig{Enabled: true}
		}, domain.ErrRequired},
		{"monitor invalid type", func(c *UptimeKumaConfig) {
			c.Monitors = []UptimeKumaMonitorConfig{{Name: "m1", Type: "udp"}}
		}, domain.ErrInvalidType},
		{"monitor missing name", func(c *UptimeKumaConfig) {
			c.Monitors = []UptimeKumaMonitorConfig{{Type: "http", URL: "https://x.com"}}
		}, domain.ErrRequired},
		{"monitor http invalid url", func(c *UptimeKumaConfig) {
			c.Monitors = []UptimeKumaMonitorConfig{{Name: "m1", Type: "http", URL: "ftp://x.com"}}
		}, domain.ErrInvalidURL},
		{"monitor port missing hostname", func(c *UptimeKumaConfig) {
			c.Monitors = []UptimeKumaMonitorConfig{{Name: "m1", Type: "port", Port: 443}}
		}, domain.ErrRequired},
		{"monitor port invalid port", func(c *UptimeKumaConfig) {
			c.Monitors = []UptimeKumaMonitorConfig{{Name: "m1", Type: "port", Hostname: "up", Port: 0}}
		}, domain.ErrInvalidPort},
		{"monitor http missing url", func(c *UptimeKumaConfig) {
			c.Monitors = []UptimeKumaMonitorConfig{{Name: "m1", Type: "http"}}
		}, domain.ErrRequired},
		{"monitor http url no host", func(c *UptimeKumaConfig) {
			c.Monitors = []UptimeKumaMonitorConfig{{Name: "m1", Type: "http", URL: "https://"}}
		}, domain.ErrInvalidURL},
		{"monitor http-browser valid", func(c *UptimeKumaConfig) {
			c.Monitors = []UptimeKumaMonitorConfig{{Name: "m1", Type: "http-browser", URL: "https://x.com"}}
		}, nil},
		{"monitor keyword-browser missing url", func(c *UptimeKumaConfig) {
			c.Monitors = []UptimeKumaMonitorConfig{{Name: "m1", Type: "keyword-browser"}}
		}, domain.ErrRequired},
		{"monitor ping missing hostname", func(c *UptimeKumaConfig) {
			c.Monitors = []UptimeKumaMonitorConfig{{Name: "m1", Type: "ping"}}
		}, domain.ErrRequired},
		{"monitor dns missing hostname", func(c *UptimeKumaConfig) {
			c.Monitors = []UptimeKumaMonitorConfig{{Name: "m1", Type: "dns"}}
		}, domain.ErrRequired},
		{"monitor ping valid", func(c *UptimeKumaConfig) {
			c.Monitors = []UptimeKumaMonitorConfig{{Name: "m1", Type: "ping", Hostname: "example.com"}}
		}, nil},
		{"monitor dns valid", func(c *UptimeKumaConfig) {
			c.Monitors = []UptimeKumaMonitorConfig{{Name: "m1", Type: "dns", Hostname: "example.com"}}
		}, nil},
		{"monitor extra not yaml-serializable", func(c *UptimeKumaConfig) {
			c.Monitors = []UptimeKumaMonitorConfig{{
				Name: "m1", Type: "http", URL: "https://x.com",
				Extra: map[string]any{"bad": make(chan int)},
			}}
		}, domain.ErrInvalidType},
		{"valid http monitor", func(c *UptimeKumaConfig) {
			c.Monitors = []UptimeKumaMonitorConfig{{Name: "m1", Type: "http", URL: "https://x.com"}}
		}, nil},
		{"valid port monitor", func(c *UptimeKumaConfig) {
			c.Monitors = []UptimeKumaMonitorConfig{{Name: "m1", Type: "port", Hostname: "up.example.com", Port: 443}}
		}, nil},
		{"duplicate monitor name", func(c *UptimeKumaConfig) {
			c.Monitors = []UptimeKumaMonitorConfig{
				{Name: "m1", Type: "ping", Hostname: "a"},
				{Name: "m1", Type: "ping", Hostname: "b"},
			}
		}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := valid()
			tt.mutate(c)
			err := c.Validate()
			if tt.name == "duplicate monitor name" {
				if err == nil || !strings.Contains(err.Error(), "is duplicated") {
					t.Errorf("Validate() = %v, want duplicated error", err)
				}
				return
			}
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Validate() error = %v, want %v", err, tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("Validate() unexpected error = %v", err)
			}
		})
	}
}

func TestInfraService_Validate_UptimeKuma_MissingConfig(t *testing.T) {
	svc := InfraService{
		Name:        "uptime-kuma-1",
		Type:        InfraServiceTypeUptimeKuma,
		ServiceBase: ServiceBase{Server: "srv-1"},
		Image:       "kuma:2",
	}
	err := svc.Validate()
	if !errors.Is(err, domain.ErrRequired) {
		t.Errorf("Validate() error = %v, want ErrRequired for missing config", err)
	}
}

// TestUptimeKumaMonitor_InvalidTypeMessage 验证 monitor type 错误文案与 uptimeKumaMonitorTypes 一致
// （列出全部允许类型，避免遗漏 http-browser/keyword-browser/push/group）。
func TestUptimeKumaMonitor_InvalidTypeMessage(t *testing.T) {
	m := UptimeKumaMonitorConfig{Name: "m1", Type: "udp"}
	err := m.Validate()
	if !errors.Is(err, domain.ErrInvalidType) {
		t.Fatalf("Validate() error = %v, want ErrInvalidType", err)
	}
	for _, typ := range []string{"http", "http-browser", "keyword", "keyword-browser", "json-query", "port", "ping", "dns", "push", "group"} {
		if !strings.Contains(err.Error(), typ) {
			t.Errorf("Validate() error %q should list allowed type %q", err, typ)
		}
	}
}

// TestUptimeKumaMonitor_JsonQueryTypeValid 验证 json-query（Kuma 1.23+）为合法监控类型，
// jsonPath/expectedValue 等 payload 字段走 Extra 透传，Validate 不额外设限。
func TestUptimeKumaMonitor_JsonQueryTypeValid(t *testing.T) {
	m := UptimeKumaMonitorConfig{
		Name: "api-json-健康",
		Type: "json-query",
		URL:  "https://api.example.com/status",
		Extra: map[string]any{
			"jsonPath":      "$.status",
			"expectedValue": "ok",
		},
	}
	if err := m.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil for json-query monitor", err)
	}
	if m.Extra["jsonPath"] != "$.status" || m.Extra["expectedValue"] != "ok" {
		t.Errorf("Extra not preserved: %#v", m.Extra)
	}
}

func uptimeKumaMonitorWithExtraYAML() string {
	return `
- name: dns-高级
  type: dns
  hostname: example.com
  dns_resolve_type: A
  packetSize: 512
  method: POST
  timeout: 10000
  retryInterval: 60
  headers:
    Authorization: Bearer x
`
}

// TestUptimeKumaMonitor_UnmarshalExtra 验证自定义 UnmarshalYAML 保留白名单外的 camelCase 字段。
func TestUptimeKumaMonitor_UnmarshalExtra(t *testing.T) {
	var monitors []UptimeKumaMonitorConfig
	if err := yaml.Unmarshal([]byte(uptimeKumaMonitorWithExtraYAML()), &monitors); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if len(monitors) != 1 {
		t.Fatalf("monitors len = %d, want 1", len(monitors))
	}
	m := monitors[0]
	if m.Name != "dns-高级" || m.Type != "dns" || m.Hostname != "example.com" {
		t.Errorf("known fields not parsed: %+v", m)
	}
	if m.Extra["dns_resolve_type"] != "A" {
		t.Errorf("Extra[dns_resolve_type] = %#v, want A", m.Extra["dns_resolve_type"])
	}
	if m.Extra["packetSize"] != 512 {
		t.Errorf("Extra[packetSize] = %#v, want 512", m.Extra["packetSize"])
	}
	if m.Extra["method"] != "POST" {
		t.Errorf("Extra[method] = %#v, want POST", m.Extra["method"])
	}
	if m.Extra["timeout"] != 10000 || m.Extra["retryInterval"] != 60 {
		t.Errorf("Extra[timeout]/[retryInterval] = %#v/%#v, want 10000/60", m.Extra["timeout"], m.Extra["retryInterval"])
	}
	if h, ok := m.Extra["headers"].(map[string]any); !ok || h["Authorization"] != "Bearer x" {
		t.Errorf("Extra[headers] = %#v, want nested map with Authorization", m.Extra["headers"])
	}
}

// TestUptimeKumaMonitor_RoundtripExtra 验证已知字段 + Extra 经 Marshal→Unmarshal 后无损。
func TestUptimeKumaMonitor_RoundtripExtra(t *testing.T) {
	var first []UptimeKumaMonitorConfig
	if err := yaml.Unmarshal([]byte(uptimeKumaMonitorWithExtraYAML()), &first); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	marshaled, err := yaml.Marshal(&first)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var second []UptimeKumaMonitorConfig
	if err := yaml.Unmarshal(marshaled, &second); err != nil {
		t.Fatalf("second Unmarshal failed: %v", err)
	}
	if len(second) != 1 {
		t.Fatalf("monitors len = %d, want 1", len(second))
	}
	a, b := first[0], second[0]
	if a.Name != b.Name || a.Type != b.Type || a.Hostname != b.Hostname {
		t.Errorf("known fields mismatch: %+v vs %+v", a, b)
	}
	if len(a.Extra) != len(b.Extra) {
		t.Fatalf("Extra length mismatch: %d vs %d (%+v vs %+v)", len(a.Extra), len(b.Extra), a.Extra, b.Extra)
	}
	for k, v := range a.Extra {
		bv, ok := b.Extra[k]
		if !ok || !reflect.DeepEqual(v, bv) {
			t.Errorf("Extra[%s] mismatch: %#v vs %#v", k, v, bv)
		}
	}
	if err := second[0].Validate(); err != nil {
		t.Errorf("Validate() unexpected error = %v", err)
	}
}

// TestUptimeKumaMonitor_ExtraPassthrough 验证含 Extra 的 monitor 经 InfraService 级 roundtrip 保留。
func TestUptimeKumaMonitor_ExtraPassthrough(t *testing.T) {
	svc := testUptimeKumaInfra()
	svc.UptimeKumaConfig.Monitors = append(svc.UptimeKumaConfig.Monitors,
		UptimeKumaMonitorConfig{
			Name:     "dns-高级",
			Type:     "dns",
			Hostname: "example.com",
			Extra:    map[string]any{"dns_resolve_type": "A", "packetSize": 512},
		})

	data, err := yaml.Marshal(svc)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var reloaded InfraService
	if err := yaml.Unmarshal(data, &reloaded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	got := reloaded.UptimeKumaConfig.Monitors[1]
	if got.Extra["dns_resolve_type"] != "A" || got.Extra["packetSize"] != 512 {
		t.Errorf("Extra lost in roundtrip: %+v", got.Extra)
	}
}
