package generator

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/lite-lake/infra-yamlops/internal/application/generator/gate"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
)

func TestCollectHostRoutes_TunnelInjection(t *testing.T) {
	gen := NewGenerator("prod", t.TempDir())

	gw := &entity.InfraService{
		Name: "gateway-node5",
		Type: entity.InfraServiceTypeGateway,
		ServiceBase: entity.ServiceBase{
			Server: "srv-node5",
		},
		Image:        "gate:latest",
		GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443},
	}

	frps := &entity.InfraService{
		Name: "frps-node5",
		Type: entity.InfraServiceTypeTunnelServer,
		ServiceBase: entity.ServiceBase{
			Server: "srv-node5",
		},
		Image: "frps:latest",
		TunnelServerConfig: &entity.TunnelServerConfig{
			Ports:     entity.TunnelServerPorts{BindPort: 30070, VhostHTTPPort: 80},
			AuthToken: *valueobject.NewSecretRefPlain("token"),
			TunnelRoutes: []entity.TunnelRoute{
				{Hostname: "pay-dev.mesh.example.net"},
				{Hostname: "wechat-dev.mesh.example.net"},
			},
		},
	}

	config := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"},
		InfraServices: []entity.InfraService{*gw, *frps},
	}

	hosts, err := gen.collectHostRoutes(gw, config)
	if err != nil {
		t.Fatalf("collectHostRoutes: %v", err)
	}

	if len(hosts) != 2 {
		t.Fatalf("expected 2 tunnel host routes, got %d", len(hosts))
	}

	expected := map[string]bool{
		"pay-dev.mesh.example.net":    false,
		"wechat-dev.mesh.example.net": false,
	}
	for _, h := range hosts {
		if _, ok := expected[h.Name]; ok {
			expected[h.Name] = true
			if h.Port != 80 {
				t.Errorf("host %q: Port = %d, want 80", h.Name, h.Port)
			}
			if h.SSLPort != 443 {
				t.Errorf("host %q: SSLPort = %d, want 443", h.Name, h.SSLPort)
			}
			if len(h.Backend) != 1 || h.Backend[0] != "http://frps-node5:80" {
				t.Errorf("host %q: Backend = %v, want [http://frps-node5:80]", h.Name, h.Backend)
			}
			if !h.PreserveHostHeader {
				t.Errorf("host %q: PreserveHostHeader = false, want true", h.Name)
			}
			if h.HealthCheckEnabled == nil || *h.HealthCheckEnabled {
				t.Errorf("host %q: HealthCheckEnabled should be false", h.Name)
			}
			if h.HealthCheck != "/" {
				t.Errorf("host %q: HealthCheck = %q, want '/'", h.Name, h.HealthCheck)
			}
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("expected host %q not found", name)
		}
	}
}

func TestCollectHostRoutes_TunnelDifferentServer(t *testing.T) {
	gen := NewGenerator("prod", t.TempDir())

	gw := &entity.InfraService{
		Name: "gateway-node5",
		Type: entity.InfraServiceTypeGateway,
		ServiceBase: entity.ServiceBase{
			Server: "srv-node5",
		},
		Image:        "gate:latest",
		GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443},
	}

	frps := &entity.InfraService{
		Name: "frps-other",
		Type: entity.InfraServiceTypeTunnelServer,
		ServiceBase: entity.ServiceBase{
			Server: "srv-other",
		},
		Image: "frps:latest",
		TunnelServerConfig: &entity.TunnelServerConfig{
			Ports:     entity.TunnelServerPorts{BindPort: 30070, VhostHTTPPort: 80},
			AuthToken: *valueobject.NewSecretRefPlain("token"),
			TunnelRoutes: []entity.TunnelRoute{
				{Hostname: "pay-dev.mesh.example.net"},
			},
		},
	}

	config := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"},
		InfraServices: []entity.InfraService{*gw, *frps},
	}

	hosts, err := gen.collectHostRoutes(gw, config)
	if err != nil {
		t.Fatalf("collectHostRoutes: %v", err)
	}
	if len(hosts) != 0 {
		t.Errorf("expected 0 hosts (tunnel on different server), got %d", len(hosts))
	}
}

func TestCollectHostRoutes_BizAndTunnel(t *testing.T) {
	gen := NewGenerator("prod", t.TempDir())

	gw := &entity.InfraService{
		Name: "gateway-node5",
		Type: entity.InfraServiceTypeGateway,
		ServiceBase: entity.ServiceBase{
			Server: "srv-node5",
		},
		Image:        "gate:latest",
		GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443},
	}

	svc := &entity.BizService{
		Name: "web-service",
		ServiceBase: entity.ServiceBase{
			Server: "srv-node5",
		},
		Image: "web:latest",
		Gateways: []entity.ServiceGatewayRoute{
			{Hostname: "web.mesh.example.net", ContainerPort: 80, HTTP: true, HTTPS: true},
		},
	}

	frps := &entity.InfraService{
		Name: "frps-node5",
		Type: entity.InfraServiceTypeTunnelServer,
		ServiceBase: entity.ServiceBase{
			Server: "srv-node5",
		},
		Image: "frps:latest",
		TunnelServerConfig: &entity.TunnelServerConfig{
			Ports:     entity.TunnelServerPorts{BindPort: 30070, VhostHTTPPort: 80},
			AuthToken: *valueobject.NewSecretRefPlain("token"),
			TunnelRoutes: []entity.TunnelRoute{
				{Hostname: "pay-dev.mesh.example.net"},
			},
		},
	}

	config := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"},
		InfraServices: []entity.InfraService{*gw, *frps},
		Services:      []entity.BizService{*svc},
	}

	hosts, err := gen.collectHostRoutes(gw, config)
	if err != nil {
		t.Fatalf("collectHostRoutes: %v", err)
	}
	if len(hosts) != 2 {
		t.Fatalf("expected 2 hosts (1 biz + 1 tunnel), got %d", len(hosts))
	}

	foundBiz := false
	foundTunnel := false
	for _, h := range hosts {
		if h.Name == "web.mesh.example.net" {
			foundBiz = true
			if len(h.Backend) != 1 || h.Backend[0] != "http://web-service:80" {
				t.Errorf("biz host: Backend = %v, want [http://web-service:80]", h.Backend)
			}
		}
		if h.Name == "pay-dev.mesh.example.net" {
			foundTunnel = true
			if len(h.Backend) != 1 || h.Backend[0] != "http://frps-node5:80" {
				t.Errorf("tunnel host: Backend = %v, want [http://frps-node5:80]", h.Backend)
			}
			if h.HealthCheckEnabled == nil || *h.HealthCheckEnabled {
				t.Error("tunnel host: HealthCheckEnabled should be false")
			}
		}
	}
	if !foundBiz {
		t.Error("biz host not found")
	}
	if !foundTunnel {
		t.Error("tunnel host not found")
	}
}

func TestCollectHostRoutes_TunnelDefaultVhostPort(t *testing.T) {
	gen := NewGenerator("prod", t.TempDir())

	gw := &entity.InfraService{
		Name: "gateway-node5",
		Type: entity.InfraServiceTypeGateway,
		ServiceBase: entity.ServiceBase{
			Server: "srv-node5",
		},
		Image:        "gate:latest",
		GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443},
	}

	frps := &entity.InfraService{
		Name: "frps-node5",
		Type: entity.InfraServiceTypeTunnelServer,
		ServiceBase: entity.ServiceBase{
			Server: "srv-node5",
		},
		Image: "frps:latest",
		TunnelServerConfig: &entity.TunnelServerConfig{
			AuthToken: *valueobject.NewSecretRefPlain("token"),
			TunnelRoutes: []entity.TunnelRoute{
				{Hostname: "pay-dev.mesh.example.net"},
			},
		},
	}

	config := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"},
		InfraServices: []entity.InfraService{*gw, *frps},
	}

	hosts, err := gen.collectHostRoutes(gw, config)
	if err != nil {
		t.Fatalf("collectHostRoutes: %v", err)
	}
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(hosts))
	}
	if hosts[0].Backend[0] != "http://frps-node5:80" {
		t.Errorf("expected backend http://frps-node5:80 (default vhost port), got %s", hosts[0].Backend[0])
	}
}

func TestCollectHostRoutes_AllowedIPsPassthrough(t *testing.T) {
	gen := NewGenerator("prod", t.TempDir())

	gw := &entity.InfraService{
		Name: "gateway-node6",
		Type: entity.InfraServiceTypeGateway,
		ServiceBase: entity.ServiceBase{
			Server: "srv-node6",
		},
		Image:        "gate:latest",
		GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443},
	}

	svc := &entity.BizService{
		Name: "app-gentlecaj-api",
		ServiceBase: entity.ServiceBase{
			Server: "srv-node6",
		},
		Image: "gentlecaj:latest",
		Gateways: []entity.ServiceGatewayRoute{
			{
				Hostname:      "app-gentlecaj-api.node6.v-prod.mesh.example.net",
				ContainerPort: 80,
				HTTPS:         true,
				AllowedIPs:    []string{"10.72.0.0/24"},
			},
		},
	}

	config := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"},
		InfraServices: []entity.InfraService{*gw},
		Services:      []entity.BizService{*svc},
	}

	hosts, err := gen.collectHostRoutes(gw, config)
	if err != nil {
		t.Fatalf("collectHostRoutes: %v", err)
	}
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(hosts))
	}
	if hosts[0].Name != "app-gentlecaj-api.node6.v-prod.mesh.example.net" {
		t.Errorf("Name = %q", hosts[0].Name)
	}
	if len(hosts[0].AllowedIPs) != 1 || hosts[0].AllowedIPs[0] != "10.72.0.0/24" {
		t.Errorf("AllowedIPs = %v, want [10.72.0.0/24]", hosts[0].AllowedIPs)
	}

	// Verify it renders into the generated gate YAML
	content, err := gen.gateGen.Generate(gen.buildGatewayConfig(gw), hosts)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if !strings.Contains(content, "allowed_ips:") || !strings.Contains(content, "10.72.0.0/24") {
		t.Errorf("generated gate config missing allowed_ips, got:\n%s", content)
	}
}

func TestCollectHostRoutes_AllowedIPsEmptyNoRestriction(t *testing.T) {
	gen := NewGenerator("prod", t.TempDir())

	gw := &entity.InfraService{
		Name: "gateway-node6",
		Type: entity.InfraServiceTypeGateway,
		ServiceBase: entity.ServiceBase{
			Server: "srv-node6",
		},
		Image:        "gate:latest",
		GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443},
	}

	svc := &entity.BizService{
		Name: "app-public-api",
		ServiceBase: entity.ServiceBase{
			Server: "srv-node6",
		},
		Image: "public:latest",
		Gateways: []entity.ServiceGatewayRoute{
			{Hostname: "public.example.com", ContainerPort: 80, HTTPS: true},
		},
	}

	config := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"},
		InfraServices: []entity.InfraService{*gw},
		Services:      []entity.BizService{*svc},
	}

	hosts, err := gen.collectHostRoutes(gw, config)
	if err != nil {
		t.Fatalf("collectHostRoutes: %v", err)
	}
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(hosts))
	}
	if len(hosts[0].AllowedIPs) != 0 {
		t.Errorf("AllowedIPs = %v, want empty (no restriction)", hosts[0].AllowedIPs)
	}
}

func TestCollectHostRoutes_WAFRequestBodyAccessPassthrough(t *testing.T) {
	gen := NewGenerator("prod", t.TempDir())

	gw := &entity.InfraService{
		Name: "gateway-node6",
		Type: entity.InfraServiceTypeGateway,
		ServiceBase: entity.ServiceBase{
			Server: "srv-node6",
		},
		Image:        "gate:latest",
		GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443},
	}

	falseVal := false
	svc := &entity.BizService{
		Name: "sc-api",
		ServiceBase: entity.ServiceBase{
			Server: "srv-node6",
		},
		Image: "sc:latest",
		Gateways: []entity.ServiceGatewayRoute{
			{
				Hostname:      "sc.mesh.example.net",
				ContainerPort: 80,
				HTTPS:         true,
				WAF: &entity.RouteWAFConfig{
					Mode:              "detect",
					RequestBodyAccess: &falseVal,
				},
			},
		},
	}

	config := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"},
		InfraServices: []entity.InfraService{*gw},
		Services:      []entity.BizService{*svc},
	}

	hosts, err := gen.collectHostRoutes(gw, config)
	if err != nil {
		t.Fatalf("collectHostRoutes: %v", err)
	}
	if len(hosts) != 1 {
		t.Fatalf("expected 1 host, got %d", len(hosts))
	}
	if hosts[0].WAFMode != "detect" {
		t.Errorf("WAFMode = %q, want detect", hosts[0].WAFMode)
	}
	if hosts[0].WAFRequestBodyAccess == nil || *hosts[0].WAFRequestBodyAccess {
		t.Error("WAFRequestBodyAccess should be false")
	}

	// Verify it renders into the generated gate YAML
	content, err := gen.gateGen.Generate(gen.buildGatewayConfig(gw), hosts)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if !strings.Contains(content, "request_body_access: false") {
		t.Errorf("generated gate config missing request_body_access: false, got:\n%s", content)
	}
}

// parseGateYAML 将生成的 gate YAML 解析为 map，用于结构化断言
func parseGateYAML(t *testing.T, content string) map[string]any {
	t.Helper()
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(content), &parsed); err != nil {
		t.Fatalf("failed to parse generated gate YAML: %v\n%s", err, content)
	}
	return parsed
}

// findGateHost 在解析后的 gate YAML 中按 name 查找 host 条目
func findGateHost(t *testing.T, parsed map[string]any, name string) map[string]any {
	t.Helper()
	hosts, ok := parsed["hosts"].([]any)
	if !ok {
		t.Fatalf("hosts missing in generated YAML: %#v", parsed)
	}
	for _, h := range hosts {
		hm, ok := h.(map[string]any)
		if !ok {
			continue
		}
		if hm["name"] == name {
			return hm
		}
	}
	t.Fatalf("host %q not found in generated YAML: %#v", name, parsed)
	return nil
}

func TestGenerateGatewayBlacklist_GlobalSection(t *testing.T) {
	gen := NewGenerator("prod", t.TempDir())

	gw := &entity.InfraService{
		Name: "gateway-node5",
		Type: entity.InfraServiceTypeGateway,
		ServiceBase: entity.ServiceBase{
			Server: "srv-node5",
		},
		Image:        "gate:latest",
		GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443},
		GatewayBlacklist: &entity.GatewayBlacklistConfig{
			Enabled:     true,
			DefaultMode: "detect",
			Whitelist:   []string{"203.0.113.10"},
		},
	}

	svc := &entity.BizService{
		Name: "web-service",
		ServiceBase: entity.ServiceBase{
			Server: "srv-node5",
		},
		Image: "web:latest",
		Gateways: []entity.ServiceGatewayRoute{
			{Hostname: "web.mesh.example.net", ContainerPort: 80, HTTP: true, HTTPS: true},
		},
	}

	config := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"},
		InfraServices: []entity.InfraService{*gw},
		Services:      []entity.BizService{*svc},
	}

	hosts, err := gen.collectHostRoutes(gw, config)
	if err != nil {
		t.Fatalf("collectHostRoutes: %v", err)
	}
	content, err := gen.gateGen.Generate(gen.buildGatewayConfig(gw), hosts)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	t.Logf("generated gate config:\n%s", content)

	parsed := parseGateYAML(t, content)
	bl, ok := parsed["blacklist"].(map[string]any)
	if !ok {
		t.Fatalf("global blacklist section missing, got:\n%s", content)
	}
	if bl["enabled"] != true {
		t.Errorf("blacklist.enabled = %v, want true", bl["enabled"])
	}
	if bl["default_mode"] != "detect" {
		t.Errorf("blacklist.default_mode = %v, want detect", bl["default_mode"])
	}
	sources, ok := bl["sources"].([]any)
	if !ok || len(sources) != 1 {
		t.Fatalf("blacklist.sources = %v, want exactly 1 source", bl["sources"])
	}
	src, ok := sources[0].(map[string]any)
	if !ok {
		t.Fatalf("blacklist.sources[0] is not a map: %#v", sources[0])
	}
	if src["name"] != "ustc" {
		t.Errorf("blacklist.sources[0].name = %v, want ustc", src["name"])
	}
	if src["url"] != "https://blackip.ustc.edu.cn/list.php?txt" {
		t.Errorf("blacklist.sources[0].url = %v, want default USTC URL", src["url"])
	}
	wl, ok := bl["whitelist"].([]any)
	if !ok || len(wl) != 1 || wl[0] != "203.0.113.10" {
		t.Errorf("blacklist.whitelist = %v, want [203.0.113.10]", bl["whitelist"])
	}
	// 不生成 gate 侧默认值已满足的字段
	for _, key := range []string{"update_interval", "timeout", "ignore_private_ips"} {
		if _, exists := bl[key]; exists {
			t.Errorf("blacklist.%s should not be generated, got:\n%s", key, content)
		}
	}
}

func TestGenerateGatewayBlacklist_NotConfigured(t *testing.T) {
	gen := NewGenerator("prod", t.TempDir())

	gw := &entity.InfraService{
		Name: "gateway-node5",
		Type: entity.InfraServiceTypeGateway,
		ServiceBase: entity.ServiceBase{
			Server: "srv-node5",
		},
		Image:        "gate:latest",
		GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443},
	}

	svc := &entity.BizService{
		Name: "web-service",
		ServiceBase: entity.ServiceBase{
			Server: "srv-node5",
		},
		Image: "web:latest",
		Gateways: []entity.ServiceGatewayRoute{
			{Hostname: "web.mesh.example.net", ContainerPort: 80, HTTP: true, HTTPS: true},
		},
	}

	config := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"},
		InfraServices: []entity.InfraService{*gw},
		Services:      []entity.BizService{*svc},
	}

	hosts, err := gen.collectHostRoutes(gw, config)
	if err != nil {
		t.Fatalf("collectHostRoutes: %v", err)
	}
	content, err := gen.gateGen.Generate(gen.buildGatewayConfig(gw), hosts)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if strings.Contains(content, "blacklist") {
		t.Errorf("legacy config must not contain blacklist key, got:\n%s", content)
	}
}

func TestGenerateGatewayBlacklist_SourceURLOverride(t *testing.T) {
	gen := NewGenerator("prod", t.TempDir())

	gw := &entity.InfraService{
		Name: "gateway-node5",
		Type: entity.InfraServiceTypeGateway,
		ServiceBase: entity.ServiceBase{
			Server: "srv-node5",
		},
		Image:        "gate:latest",
		GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443},
		GatewayBlacklist: &entity.GatewayBlacklistConfig{
			Enabled:   true,
			SourceURL: "https://mirror.example.com/blacklist.txt",
		},
	}

	content, err := gen.gateGen.Generate(gen.buildGatewayConfig(gw), nil)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	parsed := parseGateYAML(t, content)
	bl, ok := parsed["blacklist"].(map[string]any)
	if !ok {
		t.Fatalf("global blacklist section missing, got:\n%s", content)
	}
	sources := bl["sources"].([]any)
	src := sources[0].(map[string]any)
	if src["url"] != "https://mirror.example.com/blacklist.txt" {
		t.Errorf("blacklist.sources[0].url = %v, want overridden URL", src["url"])
	}
	if src["name"] != "ustc" {
		t.Errorf("blacklist.sources[0].name = %v, want ustc", src["name"])
	}
	if _, exists := bl["default_mode"]; exists {
		t.Errorf("blacklist.default_mode should be omitted when empty, got:\n%s", content)
	}
	if _, exists := bl["whitelist"]; exists {
		t.Errorf("blacklist.whitelist should be omitted when empty, got:\n%s", content)
	}
}

func TestGenerateGatewayBlacklist_HostOverride(t *testing.T) {
	gen := NewGenerator("prod", t.TempDir())

	gw := &entity.InfraService{
		Name: "gateway-node5",
		Type: entity.InfraServiceTypeGateway,
		ServiceBase: entity.ServiceBase{
			Server: "srv-node5",
		},
		Image:        "gate:latest",
		GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443},
		GatewayBlacklist: &entity.GatewayBlacklistConfig{
			Enabled:     true,
			DefaultMode: "detect",
		},
	}

	svc := &entity.BizService{
		Name: "web-service",
		ServiceBase: entity.ServiceBase{
			Server: "srv-node5",
		},
		Image: "web:latest",
		Gateways: []entity.ServiceGatewayRoute{
			{
				Hostname:      "internal.mesh.example.net",
				ContainerPort: 80,
				HTTPS:         true,
				BlacklistMode: "disabled",
			},
			{
				Hostname:      "public.mesh.example.net",
				ContainerPort: 80,
				HTTPS:         true,
			},
		},
	}

	config := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"},
		InfraServices: []entity.InfraService{*gw},
		Services:      []entity.BizService{*svc},
	}

	hosts, err := gen.collectHostRoutes(gw, config)
	if err != nil {
		t.Fatalf("collectHostRoutes: %v", err)
	}
	if len(hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(hosts))
	}
	if hosts[0].BlacklistMode != "disabled" {
		t.Errorf("hosts[0].BlacklistMode = %q, want disabled", hosts[0].BlacklistMode)
	}
	if hosts[1].BlacklistMode != "" {
		t.Errorf("hosts[1].BlacklistMode = %q, want empty", hosts[1].BlacklistMode)
	}

	content, err := gen.gateGen.Generate(gen.buildGatewayConfig(gw), hosts)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	t.Logf("generated gate config:\n%s", content)

	parsed := parseGateYAML(t, content)

	overrideHost := findGateHost(t, parsed, "internal.mesh.example.net")
	hbl, ok := overrideHost["blacklist"].(map[string]any)
	if !ok {
		t.Fatalf("host blacklist override missing for internal.mesh.example.net, got:\n%s", content)
	}
	if hbl["default_mode"] != "disabled" {
		t.Errorf("host blacklist.default_mode = %v, want disabled", hbl["default_mode"])
	}

	inheritHost := findGateHost(t, parsed, "public.mesh.example.net")
	if _, exists := inheritHost["blacklist"]; exists {
		t.Errorf("host without blacklist_mode must not have blacklist key, got:\n%s", content)
	}
}

func TestGenerateGatewayTrustedProxies_Configured(t *testing.T) {
	gen := NewGenerator("prod", t.TempDir())

	want := []string{"10.72.0.0/24", "10.71.0.0/24"}
	gw := &entity.InfraService{
		Name: "gateway-node1",
		Type: entity.InfraServiceTypeGateway,
		ServiceBase: entity.ServiceBase{
			Server: "srv-node1",
		},
		Image:                 "gate:latest",
		GatewayPorts:          &entity.GatewayPorts{HTTP: 80, HTTPS: 443},
		GatewayTrustedProxies: want,
	}

	content, err := gen.gateGen.Generate(gen.buildGatewayConfig(gw), nil)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	t.Logf("generated gate config:\n%s", content)

	parsed := parseGateYAML(t, content)
	server, ok := parsed["server"].(map[string]any)
	if !ok {
		t.Fatalf("server section missing, got:\n%s", content)
	}
	tp, ok := server["trusted_proxies"].([]any)
	if !ok {
		t.Fatalf("server.trusted_proxies missing, got:\n%s", content)
	}
	if len(tp) != len(want) {
		t.Fatalf("server.trusted_proxies = %v, want %v", tp, want)
	}
	for i := range want {
		if tp[i] != want[i] {
			t.Errorf("server.trusted_proxies[%d] = %v, want %q", i, tp[i], want[i])
		}
	}
	// buildGatewayConfig 映射校验（透传链 entity → gate.GatewayConfig → serverConfig）
	if got := gen.buildGatewayConfig(gw).TrustedProxies; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("buildGatewayConfig().TrustedProxies = %v, want %v", got, want)
	}
}

func TestGenerateGatewayTrustedProxies_NotConfigured(t *testing.T) {
	gen := NewGenerator("prod", t.TempDir())

	gw := &entity.InfraService{
		Name: "gateway-node5",
		Type: entity.InfraServiceTypeGateway,
		ServiceBase: entity.ServiceBase{
			Server: "srv-node5",
		},
		Image:        "gate:latest",
		GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443},
	}

	content, err := gen.gateGen.Generate(gen.buildGatewayConfig(gw), nil)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if strings.Contains(content, "trusted_proxies") {
		t.Errorf("legacy config must not contain trusted_proxies key, got:\n%s", content)
	}

	// server 段键集合必须与历史输出完全一致（存量输出逐字节不变的守卫）
	parsed := parseGateYAML(t, content)
	server, ok := parsed["server"].(map[string]any)
	if !ok {
		t.Fatalf("server section missing, got:\n%s", content)
	}
	wantKeys := map[string]bool{
		"port":           true,
		"g_zip_enabled":  true,
		"g_zip_level":    true,
		"g_zip_min_size": true,
		"http2_enabled":  true,
	}
	if len(server) != len(wantKeys) {
		t.Errorf("server section keys = %v, want exactly %v", server, wantKeys)
	}
	for key := range server {
		if !wantKeys[key] {
			t.Errorf("unexpected server key %q in legacy output:\n%s", key, content)
		}
	}
}

func TestIsMeshHostname(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		env      string
		want     bool
	}{
		// mesh 域名：倒数第三段为 v-{env}
		{"mesh dev", "app-uc-api.node1.v-dev.mesh.example.net", "dev", true},
		{"mesh prod", "app-gentlecaj-api.node6.v-prod.mesh.example.net", "prod", true},
		{"mesh dev another node", "app-ssl-api.node1.v-dev.mesh.example.net", "dev", true},
		// env 必须精确匹配，跨环境后缀不误标
		{"dev hostname under prod env", "app-uc-api.node1.v-dev.mesh.example.net", "prod", false},
		{"prod hostname under dev env", "app-gentlecaj-api.node6.v-prod.mesh.example.net", "dev", false},
		// 外部域名 / 常规 litelake 域名不命中
		{"external domain", "customer.example.cn", "dev", false},
		{"external domain ngc", "warden.ngc4486.com", "prod", false},
		{"env-dev litelake host", "hfmirror.env-dev.mesh.example.net", "dev", false},
		{"plain litelake host", "pay-dev.mesh.example.net", "dev", false},
		{"apex domain", "mesh.example.net", "dev", false},
		{"bare v-env host without label", "v-dev.mesh.example.net", "dev", false},
		// 后缀伪装：v-{env} 段必须紧邻 zone 根域
		{"suffix spoof", "app-uc-api.node1.v-dev.mesh.example.net.evil.com", "dev", false},
		// 空值防护
		{"empty hostname", "", "dev", false},
		{"empty env", "app-uc-api.node1.v-dev.mesh.example.net", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isMeshHostname(tt.hostname, tt.env, "mesh.example.net"); got != tt.want {
				t.Errorf("isMeshHostname(%q, %q, %q) = %v, want %v", tt.hostname, tt.env, "mesh.example.net", got, tt.want)
			}
		})
	}
}

// TestCollectHostRoutes_MeshForwardedForAppend 验证 v1.1 mesh 域名自动打标（dev 环境）：
// mesh host 生成 forwarded_for: append，外部域名 / 常规 litelake 域名不含该键。
func TestCollectHostRoutes_MeshForwardedForAppend(t *testing.T) {
	gen := NewGenerator("dev", t.TempDir())

	gw := &entity.InfraService{
		Name: "gateway-node1",
		Type: entity.InfraServiceTypeGateway,
		ServiceBase: entity.ServiceBase{
			Server: "srv-node1",
		},
		Image:        "gate:latest",
		GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443},
	}

	svc := &entity.BizService{
		Name: "app-uc-api",
		ServiceBase: entity.ServiceBase{
			Server: "srv-node1",
		},
		Image: "uc:latest",
		Gateways: []entity.ServiceGatewayRoute{
			// mesh 域名：应被打标
			{Hostname: "app-uc-api.node1.v-dev.mesh.example.net", ContainerPort: 80, HTTPS: true},
			// 外部域名：不应打标
			{Hostname: "customer.example.cn", ContainerPort: 80, HTTPS: true},
			// 形态相近的常规 litelake 域名：不应打标
			{Hostname: "hfmirror.env-dev.mesh.example.net", ContainerPort: 80, HTTPS: true},
		},
	}

	config := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"},
		InfraServices: []entity.InfraService{*gw},
		Services:      []entity.BizService{*svc},
	}

	hosts, err := gen.collectHostRoutes(gw, config)
	if err != nil {
		t.Fatalf("collectHostRoutes: %v", err)
	}
	if len(hosts) != 3 {
		t.Fatalf("expected 3 hosts, got %d", len(hosts))
	}
	for _, h := range hosts {
		want := gate.ForwardedForAppend
		if h.Name != "app-uc-api.node1.v-dev.mesh.example.net" {
			want = ""
		}
		if h.ForwardedFor != want {
			t.Errorf("host %q ForwardedFor = %q, want %q", h.Name, h.ForwardedFor, want)
		}
	}

	content, err := gen.gateGen.Generate(gen.buildGatewayConfig(gw), hosts)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	t.Logf("generated gate config:\n%s", content)

	parsed := parseGateYAML(t, content)

	meshHost := findGateHost(t, parsed, "app-uc-api.node1.v-dev.mesh.example.net")
	if meshHost["forwarded_for"] != gate.ForwardedForAppend {
		t.Errorf("mesh host forwarded_for = %v, want %q, got:\n%s", meshHost["forwarded_for"], gate.ForwardedForAppend, content)
	}

	for _, name := range []string{"customer.example.cn", "hfmirror.env-dev.mesh.example.net"} {
		host := findGateHost(t, parsed, name)
		if _, exists := host["forwarded_for"]; exists {
			t.Errorf("host %q must not contain forwarded_for key, got:\n%s", name, content)
		}
	}
}

// TestCollectHostRoutes_MeshForwardedForAppend_Prod 验证 prod 环境下后缀判定精确匹配 v-prod，
// dev 后缀的 host 在 prod 生成器下不被打标（env 精确匹配守卫）。
func TestCollectHostRoutes_MeshForwardedForAppend_Prod(t *testing.T) {
	gen := NewGenerator("prod", t.TempDir())

	gw := &entity.InfraService{
		Name: "gateway-node6",
		Type: entity.InfraServiceTypeGateway,
		ServiceBase: entity.ServiceBase{
			Server: "srv-node6",
		},
		Image:        "gate:latest",
		GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443},
	}

	svc := &entity.BizService{
		Name: "app-gentlecaj-api",
		ServiceBase: entity.ServiceBase{
			Server: "srv-node6",
		},
		Image: "caj:latest",
		Gateways: []entity.ServiceGatewayRoute{
			{Hostname: "app-gentlecaj-api.node6.v-prod.mesh.example.net", ContainerPort: 80, HTTPS: true},
			// dev 后缀混入 prod 配置属异常形态：按 env 精确匹配规则不打标
			{Hostname: "app-legacy.node1.v-dev.mesh.example.net", ContainerPort: 80, HTTPS: true},
		},
	}

	config := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"},
		InfraServices: []entity.InfraService{*gw},
		Services:      []entity.BizService{*svc},
	}

	hosts, err := gen.collectHostRoutes(gw, config)
	if err != nil {
		t.Fatalf("collectHostRoutes: %v", err)
	}
	if len(hosts) != 2 {
		t.Fatalf("expected 2 hosts, got %d", len(hosts))
	}
	if hosts[0].ForwardedFor != gate.ForwardedForAppend {
		t.Errorf("prod mesh host ForwardedFor = %q, want %q", hosts[0].ForwardedFor, gate.ForwardedForAppend)
	}
	if hosts[1].ForwardedFor != "" {
		t.Errorf("cross-env host ForwardedFor = %q, want empty", hosts[1].ForwardedFor)
	}

	content, err := gen.gateGen.Generate(gen.buildGatewayConfig(gw), hosts)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	t.Logf("generated gate config:\n%s", content)

	parsed := parseGateYAML(t, content)
	meshHost := findGateHost(t, parsed, "app-gentlecaj-api.node6.v-prod.mesh.example.net")
	if meshHost["forwarded_for"] != gate.ForwardedForAppend {
		t.Errorf("prod mesh host forwarded_for = %v, want %q, got:\n%s", meshHost["forwarded_for"], gate.ForwardedForAppend, content)
	}
	crossEnvHost := findGateHost(t, parsed, "app-legacy.node1.v-dev.mesh.example.net")
	if _, exists := crossEnvHost["forwarded_for"]; exists {
		t.Errorf("cross-env host must not contain forwarded_for key, got:\n%s", content)
	}
}

// TestShouldAppendForwardedFor 验证打标判定（生成器内部规则，非用户字段）：v1.0.21 起
// 回归单规则——hostname 匹配 mesh 域名后缀 .{节点}.v-{env}.mesh.example.net（v1.0.19）即打标。
// v1.0.20 的规则 2（hostname 命中 external_backends 服务 gateways 声明全集）已随 v1.2
// override_host 模型移除：二级链路第二跳由 override_host 改写后的 mesh 域名条目承接，
// external_backends 公网 hostname 不再因规则 2 打标。
func TestShouldAppendForwardedFor(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		env      string
		want     bool
	}{
		{"mesh suffix dev", "app-uc-api.node1.v-dev.mesh.example.net", "dev", true},
		{"mesh suffix prod", "app-gentlecaj-api.node6.v-prod.mesh.example.net", "prod", true},
		{"mesh suffix env mismatch", "app-uc-api.node1.v-dev.mesh.example.net", "prod", false},
		{"external domain not hit", "customer.example.cn", "dev", false},
		// 规则 2 已移除：external_backends 服务声明的公网 hostname 不再打标
		{"external backends public hostname no longer flagged", "myip-dev.mesh.example.net", "dev", false},
		{"neither rule", "test.example.com", "dev", false},
		{"empty hostname", "", "dev", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldAppendForwardedFor(tt.hostname, tt.env, "mesh.example.net"); got != tt.want {
				t.Errorf("shouldAppendForwardedFor(%q, %q) = %v, want %v", tt.hostname, tt.env, got, tt.want)
			}
		})
	}
}

// TestCollectHostRoutes_ExternalBackendsAutoOverrideHost 验证 v1.2 override_host 模型
// （v1.0.21）：外层代理服务（myip-node8-proxy，无容器，external_backends 指向目标服务的
// mesh 域名）在其所在公网网关（node8）生成的 host 条目自动带 override_host = external_backends
// URL 的 host（目标侧 B 网关的 mesh 域名）——上游请求 Host 改写为 mesh 域名，由 B 按 mesh
// 域名条目路由；公网 hostname（myip-dev.mesh.example.net）不再打标 forwarded_for（v1.0.20 规则 2
// 已移除）。目标服务（app-ipregion-api）在 B 侧网关（node1）声明的 mesh 域名条目（v1.2：B 侧
// 旧公网同名条目已删除）由规则 1（mesh 后缀）打标 append，且作为普通 image 服务不生成
// override_host 键。
func TestCollectHostRoutes_ExternalBackendsAutoOverrideHost(t *testing.T) {
	gen := NewGenerator("dev", t.TempDir())

	gwHk1a := &entity.InfraService{
		Name: "gateway-node8",
		Type: entity.InfraServiceTypeGateway,
		ServiceBase: entity.ServiceBase{
			Server: "srv-node8",
		},
		Image:        "gate:latest",
		GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443},
	}
	gwGn1a := &entity.InfraService{
		Name: "gateway-node1",
		Type: entity.InfraServiceTypeGateway,
		ServiceBase: entity.ServiceBase{
			Server: "srv-node1",
		},
		Image:        "gate:latest",
		GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443},
	}

	// 外层代理服务：external_backends 服务无 image（BizService.Validate 互斥约束），
	// gateways 声明公网域名 myip-dev.mesh.example.net。
	proxy := &entity.BizService{
		Name:             "myip-node8-proxy",
		ServiceBase:      entity.ServiceBase{Server: "srv-node8"},
		ExternalBackends: []string{"https://app-ipregion-api.node1.v-dev.mesh.example.net"},
		Gateways: []entity.ServiceGatewayRoute{
			{Hostname: "myip-dev.mesh.example.net", ContainerPort: 80, HTTPS: true},
		},
	}
	// 目标服务：v1.2 模型下仅在 B 侧网关声明 mesh 域名条目（旧公网同名条目已删除）。
	target := &entity.BizService{
		Name:        "app-ipregion-api",
		ServiceBase: entity.ServiceBase{Server: "srv-node1"},
		Image:       "ipregion:latest",
		Gateways: []entity.ServiceGatewayRoute{
			{Hostname: "app-ipregion-api.node1.v-dev.mesh.example.net", ContainerPort: 80, HTTPS: true},
		},
	}

	config := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"},
		InfraServices: []entity.InfraService{*gwHk1a, *gwGn1a},
		Services:      []entity.BizService{*proxy, *target},
	}

	// 外层代理所在公网网关 node8：external_backends host 条目自动 override_host = mesh 域名
	// （backend 指向外部 mesh 地址、健康检查禁用），公网域名不打标 forwarded_for。
	hk1aHosts, err := gen.collectHostRoutes(gwHk1a, config)
	if err != nil {
		t.Fatalf("collectHostRoutes: %v", err)
	}
	if len(hk1aHosts) != 1 {
		t.Fatalf("expected 1 host on gateway-node8, got %d", len(hk1aHosts))
	}
	h := hk1aHosts[0]
	if h.Name != "myip-dev.mesh.example.net" {
		t.Fatalf("gateway-node8 host name = %q, want %q", h.Name, "myip-dev.mesh.example.net")
	}
	if h.OverrideHost != "app-ipregion-api.node1.v-dev.mesh.example.net" {
		t.Errorf("gateway-node8 host OverrideHost = %q, want %q", h.OverrideHost, "app-ipregion-api.node1.v-dev.mesh.example.net")
	}
	if len(h.Backend) != 1 || h.Backend[0] != "https://app-ipregion-api.node1.v-dev.mesh.example.net" {
		t.Errorf("gateway-node8 host Backend = %v, want [https://app-ipregion-api.node1.v-dev.mesh.example.net]", h.Backend)
	}
	if h.HealthCheckEnabled == nil || *h.HealthCheckEnabled {
		t.Errorf("gateway-node8 external backends host HealthCheckEnabled must be false, got %v", h.HealthCheckEnabled)
	}
	if h.ForwardedFor != "" {
		t.Errorf("gateway-node8 public hostname ForwardedFor = %q, want empty (rule 2 removed)", h.ForwardedFor)
	}

	hk1aContent, err := gen.gateGen.Generate(gen.buildGatewayConfig(gwHk1a), hk1aHosts)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	t.Logf("generated gateway-node8 config:\n%s", hk1aContent)

	hk1aParsed := parseGateYAML(t, hk1aContent)
	proxyHost := findGateHost(t, hk1aParsed, "myip-dev.mesh.example.net")
	if proxyHost["override_host"] != "app-ipregion-api.node1.v-dev.mesh.example.net" {
		t.Errorf("proxy host override_host = %v, want mesh domain, got:\n%s", proxyHost["override_host"], hk1aContent)
	}
	if _, exists := proxyHost["forwarded_for"]; exists {
		t.Errorf("proxy public host must not contain forwarded_for key (rule 2 removed), got:\n%s", hk1aContent)
	}

	// 目标侧内网网关 node1：mesh 域名条目由规则 1 打标 append；普通 image 服务不生成
	// override_host 键。
	gn1aHosts, err := gen.collectHostRoutes(gwGn1a, config)
	if err != nil {
		t.Fatalf("collectHostRoutes: %v", err)
	}
	if len(gn1aHosts) != 1 {
		t.Fatalf("expected 1 host on gateway-node1, got %d", len(gn1aHosts))
	}
	meshHost := gn1aHosts[0]
	if meshHost.Name != "app-ipregion-api.node1.v-dev.mesh.example.net" {
		t.Fatalf("gateway-node1 host name = %q, want mesh domain", meshHost.Name)
	}
	if meshHost.ForwardedFor != gate.ForwardedForAppend {
		t.Errorf("gateway-node1 mesh host ForwardedFor = %q, want %q", meshHost.ForwardedFor, gate.ForwardedForAppend)
	}
	if meshHost.OverrideHost != "" {
		t.Errorf("gateway-node1 normal service host OverrideHost = %q, want empty", meshHost.OverrideHost)
	}

	gn1aContent, err := gen.gateGen.Generate(gen.buildGatewayConfig(gwGn1a), gn1aHosts)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	t.Logf("generated gateway-node1 config:\n%s", gn1aContent)

	gn1aParsed := parseGateYAML(t, gn1aContent)
	meshEntry := findGateHost(t, gn1aParsed, "app-ipregion-api.node1.v-dev.mesh.example.net")
	if meshEntry["forwarded_for"] != gate.ForwardedForAppend {
		t.Errorf("gateway-node1 mesh host forwarded_for = %v, want %q, got:\n%s", meshEntry["forwarded_for"], gate.ForwardedForAppend, gn1aContent)
	}
	if _, exists := meshEntry["override_host"]; exists {
		t.Errorf("normal service host must not contain override_host key, got:\n%s", gn1aContent)
	}
}

// TestBuildHostRoute_ExternalBackendsOverrideHostDecisions 验证自动 override_host 的决策分支
// （v1.2，v1.0.21）：单 URL / 多 URL 同 host → 取该 host；显式配置 override_host → 用户值
// 优先不覆盖（对齐 prod 存量 token-plan 伪装第三方案）；多 URL host 不同 / URL 非法 →
// 保持现状不 override（告警见生成日志，此处仅断言行为）。
func TestBuildHostRoute_ExternalBackendsOverrideHostDecisions(t *testing.T) {
	gen := NewGenerator("dev", t.TempDir())

	gw := &entity.InfraService{
		Name: "gateway-node1",
		Type: entity.InfraServiceTypeGateway,
		ServiceBase: entity.ServiceBase{
			Server: "srv-node1",
		},
		Image:        "gate:latest",
		GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443},
	}

	tests := []struct {
		name         string
		backends     []string
		explicit     string
		wantOverride string
	}{
		{
			name:         "single url auto override",
			backends:     []string{"https://app-x.node1.v-dev.mesh.example.net"},
			wantOverride: "app-x.node1.v-dev.mesh.example.net",
		},
		{
			name:         "explicit override_host wins",
			backends:     []string{"https://app-x.node1.v-dev.mesh.example.net"},
			explicit:     "token-plan.example.com",
			wantOverride: "token-plan.example.com",
		},
		{
			name:         "multiple urls same host",
			backends:     []string{"https://app-x.node1.v-dev.mesh.example.net", "https://app-x.node1.v-dev.mesh.example.net/api"},
			wantOverride: "app-x.node1.v-dev.mesh.example.net",
		},
		{
			name:         "multiple urls different hosts keep current behavior",
			backends:     []string{"https://a.example.com", "https://b.example.com"},
			wantOverride: "",
		},
		{
			name:         "invalid url keep current behavior",
			backends:     []string{"://invalid-url"},
			wantOverride: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &entity.BizService{
				Name:             "svc-proxy",
				ServiceBase:      entity.ServiceBase{Server: "srv-node1"},
				ExternalBackends: tt.backends,
			}
			route := &entity.ServiceGatewayRoute{
				Hostname:      "pub.example.com",
				ContainerPort: 80,
				HTTPS:         true,
				OverrideHost:  tt.explicit,
			}
			got, err := gen.buildHostRoute(svc, route, gw, nil)
			if err != nil {
				t.Fatalf("buildHostRoute: %v", err)
			}
			if got.OverrideHost != tt.wantOverride {
				t.Errorf("buildHostRoute().OverrideHost = %q, want %q", got.OverrideHost, tt.wantOverride)
			}
		})
	}

	// 普通 image 服务：不自动 override。
	normalSvc := &entity.BizService{
		Name:        "app-normal",
		ServiceBase: entity.ServiceBase{Server: "srv-node1"},
		Image:       "normal:latest",
	}
	normalRoute := &entity.ServiceGatewayRoute{Hostname: "normal.example.com", ContainerPort: 80, HTTPS: true}
	got, err := gen.buildHostRoute(normalSvc, normalRoute, gw, nil)
	if err != nil {
		t.Fatalf("buildHostRoute: %v", err)
	}
	if got.OverrideHost != "" {
		t.Errorf("normal service buildHostRoute().OverrideHost = %q, want empty", got.OverrideHost)
	}
}

// ===== tracker_shipper 透传（内部PRD §7.1） =====

// trackerShipperGatewayFixture 构建 1 网关（可带 tracker_shipper）+ 1 业务服务的最小生成环境
func trackerShipperGatewayFixture(t *testing.T, env string, ts *entity.GatewayTrackerShipperConfig) (*Generator, *entity.InfraService, *entity.Config) {
	t.Helper()
	gen := NewGenerator(env, t.TempDir())
	gw := &entity.InfraService{
		Name:         "gateway-node1",
		Type:         entity.InfraServiceTypeGateway,
		ServiceBase:  entity.ServiceBase{Server: "srv-node1"},
		Image:        "gate:latest",
		GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443},

		GatewayTrackerShipper: ts,
	}
	svc := &entity.BizService{
		Name:        "svc-demo",
		ServiceBase: entity.ServiceBase{Server: "srv-node1"},
		Image:       "demo:latest",
		Gateways: []entity.ServiceGatewayRoute{
			{Hostname: "demo.env-dev.mesh.example.net", HTTP: true, HTTPS: true},
		},
	}
	config := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"},
		InfraServices: []entity.InfraService{*gw},
		Services:      []entity.BizService{*svc},
	}
	return gen, gw, config
}

// 存量兼容硬约束：未配置 tracker_shipper 的 gateway，产物不得出现该键，
// 顶层键集合与历史输出完全一致（逐字节不变的守卫）。
func TestGenerateGateYAML_TrackerShipperNotConfigured(t *testing.T) {
	gen, gw, config := trackerShipperGatewayFixture(t, "dev", nil)

	result, err := gen.buildGatewayRoutes(gw, config)
	if err != nil {
		t.Fatalf("buildGatewayRoutes: %v", err)
	}
	content, err := gen.gateGen.Generate(result.gatewayConfig, result.hosts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if strings.Contains(content, "tracker_shipper") {
		t.Errorf("legacy config must not contain tracker_shipper key, got:\n%s", content)
	}

	parsed := parseGateYAML(t, content)
	wantKeys := map[string]bool{"server": true, "logger": true, "waf": true, "hosts": true}
	if len(parsed) != len(wantKeys) {
		t.Errorf("top-level keys = %v, want exactly %v\ngot:\n%s", parsed, wantKeys, content)
	}
	for key := range parsed {
		if !wantKeys[key] {
			t.Errorf("unexpected top-level key %q in legacy output:\n%s", key, content)
		}
	}
}

// 完整配置正确透传 + secret 引用解析为明文：顶层 tracker_shipper 段与 server/hosts 平级，
// 八个子字段全部就位，secret 引用名不泄露到产物。
func TestGenerateGateYAML_TrackerShipperFullPassthrough(t *testing.T) {
	ts := &entity.GatewayTrackerShipperConfig{
		Enabled:               true,
		Endpoint:              "https://app-tc-api.node1.v-dev.mesh.example.net/api/internal/gateway-ingest",
		SharedSecret:          *valueobject.NewSecretRefSecret("tc_gateway_ingest_secret"),
		NodeName:              "yo-dev-gateway-node1",
		BufferSize:            20000,
		BatchSize:             800,
		FlushInterval:         "2s",
		FailureSampleLogRatio: 0.05,
	}
	gen, gw, config := trackerShipperGatewayFixture(t, "dev", ts)
	config.Secrets = []entity.Secret{{Name: "tc_gateway_ingest_secret", Value: "s3cret"}}

	result, err := gen.buildGatewayRoutes(gw, config)
	if err != nil {
		t.Fatalf("buildGatewayRoutes: %v", err)
	}
	content, err := gen.gateGen.Generate(result.gatewayConfig, result.hosts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	t.Logf("generated gate config:\n%s", content)

	parsed := parseGateYAML(t, content)
	seg, ok := parsed["tracker_shipper"].(map[string]any)
	if !ok {
		t.Fatalf("top-level tracker_shipper section missing, got:\n%s", content)
	}
	checks := []struct {
		key  string
		want any
	}{
		{"enabled", true},
		{"endpoint", "https://app-tc-api.node1.v-dev.mesh.example.net/api/internal/gateway-ingest"},
		{"shared_secret", "s3cret"}, // {secret: name} 引用已解析为明文
		{"node_name", "yo-dev-gateway-node1"},
		{"buffer_size", 20000},
		{"batch_size", 800},
		{"flush_interval", "2s"},
		{"failure_sample_log_ratio", 0.05},
	}
	for _, c := range checks {
		if seg[c.key] != c.want {
			t.Errorf("tracker_shipper.%s = %v, want %v\ngot:\n%s", c.key, seg[c.key], c.want, content)
		}
	}
	if strings.Contains(content, "tc_gateway_ingest_secret") {
		t.Errorf("secret reference name must not leak into generated config, got:\n%s", content)
	}
}

// 缺 secret：生成阶段直接报错，不产出半成品配置（照 auth.password 先例）。
func TestBuildGatewayRoutes_TrackerShipperSecretMissing(t *testing.T) {
	ts := &entity.GatewayTrackerShipperConfig{
		Enabled:      true,
		Endpoint:     "https://app-tc-api.node1.v-dev.mesh.example.net/api/internal/gateway-ingest",
		SharedSecret: *valueobject.NewSecretRefSecret("missing_secret"),
		NodeName:     "yo-dev-gateway-node1",
	}
	gen, gw, config := trackerShipperGatewayFixture(t, "dev", ts)
	// secrets 中不含 missing_secret → 生成必须报错
	if _, err := gen.buildGatewayRoutes(gw, config); err == nil {
		t.Fatalf("secret resolve failure must propagate as error")
	} else if !strings.Contains(err.Error(), "missing_secret") {
		t.Errorf("error = %v, want mentioning secret name", err)
	}
}

// node_name 注入逻辑：显式配置值优先；未配置时自动注入容器命名 yo-{env}-{服务名}
// （服务名取 gateway infra service 的 name，如 gateway-node1 → yo-dev-gateway-node1）。
func TestBuildTrackerShipperConfig_NodeNameInjection(t *testing.T) {
	tests := []struct {
		name         string
		env          string
		nodeName     string
		wantNodeName string
	}{
		{"explicit node_name wins", "dev", "custom-node-01", "custom-node-01"},
		{"auto inject dev", "dev", "", "yo-dev-gateway-node1"},
		{"auto inject prod", "prod", "", "yo-prod-gateway-node1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := &entity.GatewayTrackerShipperConfig{
				Enabled:      true,
				Endpoint:     "https://app-tc-api.node1.v-dev.mesh.example.net/api/internal/gateway-ingest",
				SharedSecret: *valueobject.NewSecretRefPlain("s3cret"),
				NodeName:     tt.nodeName,
			}
			gen, gw, config := trackerShipperGatewayFixture(t, tt.env, ts)
			got, err := gen.buildTrackerShipperConfig(gw, config.GetSecretsMap())
			if err != nil {
				t.Fatalf("buildTrackerShipperConfig: %v", err)
			}
			if got.NodeName != tt.wantNodeName {
				t.Errorf("NodeName = %q, want %q", got.NodeName, tt.wantNodeName)
			}
		})
	}
}

// 缺省值：buffer_size/batch_size/flush_interval/failure_sample_log_ratio 未配置时
// 由 yamlops 显式填入 gate 侧缺省值（产物完整自描述）。
func TestBuildTrackerShipperConfig_Defaults(t *testing.T) {
	ts := &entity.GatewayTrackerShipperConfig{
		Enabled:      true,
		Endpoint:     "https://app-tc-api.node1.v-dev.mesh.example.net/api/internal/gateway-ingest",
		SharedSecret: *valueobject.NewSecretRefPlain("s3cret"),
	}
	gen, gw, config := trackerShipperGatewayFixture(t, "dev", ts)

	got, err := gen.buildTrackerShipperConfig(gw, config.GetSecretsMap())
	if err != nil {
		t.Fatalf("buildTrackerShipperConfig: %v", err)
	}
	if got.BufferSize != gate.DefaultTrackerShipperBufferSize {
		t.Errorf("BufferSize = %d, want %d", got.BufferSize, gate.DefaultTrackerShipperBufferSize)
	}
	if got.BatchSize != gate.DefaultTrackerShipperBatchSize {
		t.Errorf("BatchSize = %d, want %d", got.BatchSize, gate.DefaultTrackerShipperBatchSize)
	}
	if got.FlushInterval != gate.DefaultTrackerShipperFlushInterval {
		t.Errorf("FlushInterval = %q, want %q", got.FlushInterval, gate.DefaultTrackerShipperFlushInterval)
	}
	if got.FailureSampleLogRatio != gate.DefaultTrackerShipperFailureSampleLogRatio {
		t.Errorf("FailureSampleLogRatio = %v, want %v", got.FailureSampleLogRatio, gate.DefaultTrackerShipperFailureSampleLogRatio)
	}
}

// 段配置存在但 enabled: false：产物仍输出完整段（缺省形态，PRD §7.1），gate 侧不启动 shipper。
func TestGenerateGateYAML_TrackerShipperDisabledStillGenerated(t *testing.T) {
	ts := &entity.GatewayTrackerShipperConfig{
		Enabled:      false,
		SharedSecret: *valueobject.NewSecretRefPlain("s3cret"),
	}
	gen, gw, config := trackerShipperGatewayFixture(t, "dev", ts)

	result, err := gen.buildGatewayRoutes(gw, config)
	if err != nil {
		t.Fatalf("buildGatewayRoutes: %v", err)
	}
	content, err := gen.gateGen.Generate(result.gatewayConfig, result.hosts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	parsed := parseGateYAML(t, content)
	seg, ok := parsed["tracker_shipper"].(map[string]any)
	if !ok {
		t.Fatalf("top-level tracker_shipper section missing for disabled config, got:\n%s", content)
	}
	if seg["enabled"] != false {
		t.Errorf("tracker_shipper.enabled = %v, want false\ngot:\n%s", seg["enabled"], content)
	}
	if seg["buffer_size"] != gate.DefaultTrackerShipperBufferSize {
		t.Errorf("tracker_shipper.buffer_size = %v, want default, got:\n%s", seg["buffer_size"], content)
	}
}

// ===== 密码门 auth 透传（内部PRD） =====

// authGatewayFixture 构建 1 网关 + 1 业务服务的最小生成环境
func authGatewayFixture(t *testing.T, route entity.ServiceGatewayRoute) (*Generator, *entity.InfraService, *entity.Config) {
	t.Helper()
	gen := NewGenerator("dev", t.TempDir())
	gw := &entity.InfraService{
		Name:         "gateway-node1",
		Type:         entity.InfraServiceTypeGateway,
		ServiceBase:  entity.ServiceBase{Server: "srv-node1"},
		Image:        "gate:latest",
		GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443},
	}
	svc := &entity.BizService{
		Name:        "svc-demo",
		ServiceBase: entity.ServiceBase{Server: "srv-node1"},
		Image:       "demo:latest",
		Gateways:    []entity.ServiceGatewayRoute{route},
	}
	config := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"},
		InfraServices: []entity.InfraService{*gw},
		Services:      []entity.BizService{*svc},
	}
	return gen, gw, config
}

func TestCollectHostRoutes_AuthPassthrough(t *testing.T) {
	route := entity.ServiceGatewayRoute{
		Hostname: "gated.env-dev.mesh.example.net",
		HTTP:     true,
		HTTPS:    true,
		Auth: &entity.RouteAuthConfig{
			Password:    valueobject.NewSecretRefSecret("gate_demo_password"),
			TTL:         "12h",
			PublicPaths: []string{"/healthz"},
		},
	}
	gen, gw, config := authGatewayFixture(t, route)
	config.Secrets = []entity.Secret{{Name: "gate_demo_password", Value: "s3cret"}}

	hosts, err := gen.collectHostRoutes(gw, config)
	if err != nil {
		t.Fatalf("collectHostRoutes: %v", err)
	}
	if len(hosts) != 1 {
		t.Fatalf("hosts = %d, want 1", len(hosts))
	}
	h := hosts[0]
	if h.AuthPassword != "s3cret" {
		t.Errorf("AuthPassword = %q, want secret-resolved value", h.AuthPassword)
	}
	if h.AuthTTL != "12h" {
		t.Errorf("AuthTTL = %q, want 12h", h.AuthTTL)
	}
	if len(h.AuthPublicPaths) != 1 || h.AuthPublicPaths[0] != "/healthz" {
		t.Errorf("AuthPublicPaths = %v", h.AuthPublicPaths)
	}
}

func TestCollectHostRoutes_AuthSecretResolveFailure(t *testing.T) {
	route := entity.ServiceGatewayRoute{
		Hostname: "gated.env-dev.mesh.example.net",
		HTTP:     true,
		Auth:     &entity.RouteAuthConfig{Password: valueobject.NewSecretRefSecret("missing_secret")},
	}
	gen, gw, config := authGatewayFixture(t, route)
	// secrets 中不含 missing_secret → 生成必须报错，不产出半成品配置
	if _, err := gen.collectHostRoutes(gw, config); err == nil {
		t.Fatalf("secret resolve failure must propagate as error")
	} else if !strings.Contains(err.Error(), "missing_secret") {
		t.Errorf("error = %v, want mentioning secret name", err)
	}
}

// 存量兼容硬约束：未配置 auth 时生成输出与旧版逐字节一致；
// 配置 auth 时 host 段生成 auth 块且其他部分不变。
func TestGenerateGateYAML_AuthByteCompatibility(t *testing.T) {
	buildConfig := func(withAuth bool) *entity.Config {
		route := entity.ServiceGatewayRoute{
			Hostname: "gated.env-dev.mesh.example.net",
			HTTP:     true,
			HTTPS:    true,
		}
		if withAuth {
			route.Auth = &entity.RouteAuthConfig{Password: valueobject.NewSecretRefPlain("pw")}
		}
		gen, _, config := authGatewayFixture(t, route)
		_ = gen
		return config
	}

	gw := &entity.InfraService{
		Name:         "gateway-node1",
		Type:         entity.InfraServiceTypeGateway,
		ServiceBase:  entity.ServiceBase{Server: "srv-node1"},
		Image:        "gate:latest",
		GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443},
	}
	gen := NewGenerator("dev", t.TempDir())

	// 未配置 auth：生成内容不含 auth 键（与旧版逐字节一致的保证）
	result, err := gen.buildGatewayRoutes(gw, buildConfig(false))
	if err != nil {
		t.Fatalf("buildGatewayRoutes(no auth): %v", err)
	}
	legacyOut, err := gen.gateGen.Generate(result.gatewayConfig, result.hosts)
	if err != nil {
		t.Fatalf("Generate(no auth): %v", err)
	}
	if strings.Contains(legacyOut, "auth") {
		t.Errorf("legacy output must not contain auth key, got:\n%s", legacyOut)
	}

	// 配置 auth：host 段出现 auth 块（password/ttl），其余部分与 legacy 完全一致
	resultAuth, err := gen.buildGatewayRoutes(gw, buildConfig(true))
	if err != nil {
		t.Fatalf("buildGatewayRoutes(auth): %v", err)
	}
	authOut, err := gen.gateGen.Generate(resultAuth.gatewayConfig, resultAuth.hosts)
	if err != nil {
		t.Fatalf("Generate(auth): %v", err)
	}
	if !strings.Contains(authOut, "auth:") ||
		!strings.Contains(authOut, "password: pw") {
		t.Errorf("auth output missing auth block, got:\n%s", authOut)
	}

	// 除 auth 块外其余行与 legacy 完全一致（去掉 auth 三行后逐字节比较）
	stripAuth := func(s string) string {
		var kept []string
		for _, line := range strings.Split(s, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "auth:" || trimmed == "password: pw" {
				continue
			}
			kept = append(kept, line)
		}
		return strings.Join(kept, "\n")
	}
	if got, want := stripAuth(authOut), stripAuth(legacyOut); got != want {
		t.Errorf("non-auth part changed:\n--- legacy\n%s\n--- auth\n%s", want, got)
	}
}

// ===== 逐条目 http→https 301 + 规范域收敛 + HSTS per-host（内部总体方案 W3-2 / D8） =====

// entity → gate.HostRoute → gate.yaml 产物三字段全链路透传
func TestCollectHostRoutes_RedirectFieldsPassthrough(t *testing.T) {
	tr := true
	off := "off"
	route := entity.ServiceGatewayRoute{
		Hostname:      "www.env-dev.mesh.example.net",
		HTTP:          true,
		HTTPS:         true,
		RedirectHTTPS: &tr,
		RedirectHost:  "env-dev.mesh.example.net",
		HSTS:          &off,
	}
	gen, gw, config := authGatewayFixture(t, route)

	hosts, err := gen.collectHostRoutes(gw, config)
	if err != nil {
		t.Fatalf("collectHostRoutes: %v", err)
	}
	if len(hosts) != 1 {
		t.Fatalf("hosts = %d, want 1", len(hosts))
	}
	h := hosts[0]
	if h.RedirectHTTPS == nil || !*h.RedirectHTTPS {
		t.Errorf("HostRoute.RedirectHTTPS = %v, want *true", h.RedirectHTTPS)
	}
	if h.RedirectHost != "env-dev.mesh.example.net" {
		t.Errorf("HostRoute.RedirectHost = %q, want env-dev.mesh.example.net", h.RedirectHost)
	}
	if h.HSTS == nil || *h.HSTS != "off" {
		t.Errorf("HostRoute.HSTS = %v, want *\"off\"", h.HSTS)
	}

	// 渲染进产物核对
	content, err := gen.gateGen.Generate(gen.buildGatewayConfig(gw), hosts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	parsed := parseGateYAML(t, content)
	hostYAML := findGateHost(t, parsed, "www.env-dev.mesh.example.net")
	if hostYAML["redirect_https"] != true {
		t.Errorf("redirect_https = %v, want true\ngot:\n%s", hostYAML["redirect_https"], content)
	}
	if hostYAML["redirect_host"] != "env-dev.mesh.example.net" {
		t.Errorf("redirect_host = %v, want env-dev.mesh.example.net\ngot:\n%s", hostYAML["redirect_host"], content)
	}
	if hostYAML["hsts"] != "off" {
		t.Errorf("hsts = %v, want off\ngot:\n%s", hostYAML["hsts"], content)
	}
}

// 存量兼容硬约束：未配置三字段时生成输出不含 redirect_https/redirect_host/hsts 键
// （与旧版逐字节一致）；配置后除新增三行外其余部分与 legacy 逐字节一致。
func TestGenerateGateYAML_RedirectFieldsByteCompatibility(t *testing.T) {
	buildConfig := func(withFields bool) *entity.Config {
		route := entity.ServiceGatewayRoute{
			Hostname: "www.env-dev.mesh.example.net",
			HTTP:     true,
			HTTPS:    true,
		}
		if withFields {
			tr := true
			off := "off"
			route.RedirectHTTPS = &tr
			route.RedirectHost = "env-dev.mesh.example.net"
			route.HSTS = &off
		}
		gen, _, config := authGatewayFixture(t, route)
		_ = gen
		return config
	}

	gw := &entity.InfraService{
		Name:         "gateway-node1",
		Type:         entity.InfraServiceTypeGateway,
		ServiceBase:  entity.ServiceBase{Server: "srv-node1"},
		Image:        "gate:latest",
		GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443},
	}
	gen := NewGenerator("dev", t.TempDir())

	// 未配置：产物不含三个新键（存量输出逐字节不变的保证）
	result, err := gen.buildGatewayRoutes(gw, buildConfig(false))
	if err != nil {
		t.Fatalf("buildGatewayRoutes(no fields): %v", err)
	}
	legacyOut, err := gen.gateGen.Generate(result.gatewayConfig, result.hosts)
	if err != nil {
		t.Fatalf("Generate(no fields): %v", err)
	}
	for _, key := range []string{"redirect_https", "redirect_host", "hsts"} {
		if strings.Contains(legacyOut, key) {
			t.Errorf("legacy output must not contain %q key, got:\n%s", key, legacyOut)
		}
	}

	// 配置后：三个新键按行出现在 host 段（"off" 属 YAML 布尔形词，yaml.v3 生成带引号的
	// `hsts: "off"`，保证按字符串解析）
	resultNew, err := gen.buildGatewayRoutes(gw, buildConfig(true))
	if err != nil {
		t.Fatalf("buildGatewayRoutes(fields): %v", err)
	}
	newOut, err := gen.gateGen.Generate(resultNew.gatewayConfig, resultNew.hosts)
	if err != nil {
		t.Fatalf("Generate(fields): %v", err)
	}
	for _, want := range []string{"redirect_https: true", "redirect_host: env-dev.mesh.example.net", `hsts: "off"`} {
		if !strings.Contains(newOut, want) {
			t.Errorf("output missing %q, got:\n%s", want, newOut)
		}
	}

	// 除新增三行外其余与 legacy 逐字节一致
	stripNew := func(s string) string {
		var kept []string
		for _, line := range strings.Split(s, "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "redirect_https: true" ||
				trimmed == "redirect_host: env-dev.mesh.example.net" ||
				trimmed == `hsts: "off"` {
				continue
			}
			kept = append(kept, line)
		}
		return strings.Join(kept, "\n")
	}
	if got, want := stripNew(newOut), stripNew(legacyOut); got != want {
		t.Errorf("non-redirect part changed:\n--- legacy\n%s\n--- new\n%s", want, got)
	}
}

// ===== CRS failure_policy / retry_count 透传（内部PRD §5.4/§5.7） =====

// TestGenerateGateYAML_CRSFailurePolicyPassthrough 设置 crs_failure_policy 后：
// buildGatewayConfig 透传到 gate.GatewayConfig，产物 waf.crs 含 failure_policy（degrade/fail）
// 与恒输出的 download.retry_count: 2（内部PRD：gate 新版将 timeout 当总预算、
// retry_count 当轮数）。
func TestGenerateGateYAML_CRSFailurePolicyPassthrough(t *testing.T) {
	tests := []struct {
		name          string
		failurePolicy string
		wantLine      string
	}{
		{"degrade policy", "degrade", "failure_policy: degrade"},
		{"fail policy", "fail", "failure_policy: fail"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewGenerator("dev", t.TempDir())
			gw := &entity.InfraService{
				Name:         "gateway-node1",
				Type:         entity.InfraServiceTypeGateway,
				ServiceBase:  entity.ServiceBase{Server: "srv-node1"},
				Image:        "gate:latest",
				GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443},
				GatewayWAF: &entity.GatewayWAFConfig{
					// 当前 WAF 全局关闭（2026-09-15 事故后），crs_failure_policy 仅预置策略
					Enabled:          false,
					DefaultMode:      "detect",
					CRSMirrorURL:     "https://crsmirror.env-dev.mesh.example.net",
					CRSFailurePolicy: tt.failurePolicy,
				},
			}

			gc := gen.buildGatewayConfig(gw)
			if gc.CRSFailurePolicy != tt.failurePolicy {
				t.Errorf("buildGatewayConfig().CRSFailurePolicy = %q, want %q", gc.CRSFailurePolicy, tt.failurePolicy)
			}

			content, err := gen.gateGen.Generate(gc, nil)
			if err != nil {
				t.Fatalf("Generate() error = %v", err)
			}
			t.Logf("generated gate config:\n%s", content)
			if !strings.Contains(content, tt.wantLine) {
				t.Errorf("generated config missing %q, got:\n%s", tt.wantLine, content)
			}
			if !strings.Contains(content, "retry_count: 2") {
				t.Errorf("generated config missing retry_count: 2, got:\n%s", content)
			}

			parsed := parseGateYAML(t, content)
			waf, ok := parsed["waf"].(map[string]any)
			if !ok {
				t.Fatalf("waf section missing, got:\n%s", content)
			}
			crs, ok := waf["crs"].(map[string]any)
			if !ok {
				t.Fatalf("waf.crs section missing, got:\n%s", content)
			}
			if crs["failure_policy"] != tt.failurePolicy {
				t.Errorf("waf.crs.failure_policy = %v, want %q\ngot:\n%s", crs["failure_policy"], tt.failurePolicy, content)
			}
			dl, ok := crs["download"].(map[string]any)
			if !ok {
				t.Fatalf("waf.crs.download section missing, got:\n%s", content)
			}
			if dl["retry_count"] != 2 {
				t.Errorf("waf.crs.download.retry_count = %v, want 2\ngot:\n%s", dl["retry_count"], content)
			}
		})
	}
}

// TestGenerateGateYAML_CRSFailurePolicyNotConfigured 未设置 crs_failure_policy 时（waf 整段
// 未配置、或已配置但未设置该字段两条路径），产物均不得含 failure_policy 键（存量输出
// 逐字节不变的守卫）；retry_count 恒输出（有意的版本门槛新增，旧版 gate 静默忽略）。
func TestGenerateGateYAML_CRSFailurePolicyNotConfigured(t *testing.T) {
	tests := []struct {
		name string
		waf  *entity.GatewayWAFConfig
	}{
		{"waf section absent", nil},
		{"waf present without failure policy", &entity.GatewayWAFConfig{
			Enabled:      false,
			DefaultMode:  "detect",
			CRSMirrorURL: "https://crsmirror.mesh.example.net",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewGenerator("prod", t.TempDir())
			gw := &entity.InfraService{
				Name:         "gateway-node5",
				Type:         entity.InfraServiceTypeGateway,
				ServiceBase:  entity.ServiceBase{Server: "srv-node5"},
				Image:        "gate:latest",
				GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443},
				GatewayWAF:   tt.waf,
			}

			gc := gen.buildGatewayConfig(gw)
			if gc.CRSFailurePolicy != "" {
				t.Errorf("buildGatewayConfig().CRSFailurePolicy = %q, want empty", gc.CRSFailurePolicy)
			}

			content, err := gen.gateGen.Generate(gc, nil)
			if err != nil {
				t.Fatalf("Generate() error = %v", err)
			}
			if strings.Contains(content, "failure_policy") {
				t.Errorf("legacy config must not contain failure_policy key, got:\n%s", content)
			}
			if !strings.Contains(content, "retry_count: 2") {
				t.Errorf("generated config missing retry_count: 2, got:\n%s", content)
			}

			parsed := parseGateYAML(t, content)
			waf := parsed["waf"].(map[string]any)
			crs := waf["crs"].(map[string]any)
			if _, exists := crs["failure_policy"]; exists {
				t.Errorf("waf.crs must not contain failure_policy key, got:\n%s", content)
			}
			dl := crs["download"].(map[string]any)
			if dl["retry_count"] != 2 {
				t.Errorf("waf.crs.download.retry_count = %v, want 2\ngot:\n%s", dl["retry_count"], content)
			}
		})
	}
}

// ===== gate 安全加固新增字段透传（backlog「yamlops 新字段透传补齐」） =====

// entity → gate.HostRoute：M3 防爆破参数与 L3 per-host WSS 跳过校验映射
func TestCollectHostRoutes_SecurityFieldsPassthrough(t *testing.T) {
	tr := true
	route := entity.ServiceGatewayRoute{
		Hostname: "gated.env-dev.mesh.example.net",
		HTTP:     true,
		HTTPS:    true,
		Auth: &entity.RouteAuthConfig{
			Password:     valueobject.NewSecretRefSecret("gate_demo_password"),
			MaxFailures:  3,
			LockDuration: "30m",
		},
		WebSocketInsecureSkip: &tr,
	}
	gen, gw, config := authGatewayFixture(t, route)
	config.Secrets = []entity.Secret{{Name: "gate_demo_password", Value: "s3cret"}}

	hosts, err := gen.collectHostRoutes(gw, config)
	if err != nil {
		t.Fatalf("collectHostRoutes: %v", err)
	}
	if len(hosts) != 1 {
		t.Fatalf("hosts = %d, want 1", len(hosts))
	}
	h := hosts[0]
	if h.AuthMaxFailures != 3 {
		t.Errorf("AuthMaxFailures = %d, want 3", h.AuthMaxFailures)
	}
	if h.AuthLockDuration != "30m" {
		t.Errorf("AuthLockDuration = %q, want 30m", h.AuthLockDuration)
	}
	if h.WebSocketInsecureSkip == nil || !*h.WebSocketInsecureSkip {
		t.Errorf("WebSocketInsecureSkip = %v, want true", h.WebSocketInsecureSkip)
	}

	// 未配置时保持零值（nil 指针 / 0 / 空串），产物不生成对应键
	route2 := entity.ServiceGatewayRoute{
		Hostname: "plain.env-dev.mesh.example.net",
		HTTP:     true,
		Auth:     &entity.RouteAuthConfig{Password: valueobject.NewSecretRefPlain("pw")},
	}
	gen2, gw2, config2 := authGatewayFixture(t, route2)
	hosts2, err := gen2.collectHostRoutes(gw2, config2)
	if err != nil {
		t.Fatalf("collectHostRoutes(plain): %v", err)
	}
	h2 := hosts2[0]
	if h2.AuthMaxFailures != 0 || h2.AuthLockDuration != "" || h2.WebSocketInsecureSkip != nil {
		t.Errorf("plain host security fields = (%d, %q, %v), want zero values",
			h2.AuthMaxFailures, h2.AuthLockDuration, h2.WebSocketInsecureSkip)
	}
}

// 全链路：网关级 H1 逃生口 / L3 全局 Deprecated / H2 CRS pin 与明文镜像放行 /
// L6 黑名单放行与上限 + host 级 auth M3 参数与 WSS 跳过，逐键断言产物 YAML 路径取值
func TestGenerateGateYAML_SecurityFieldsPassthrough(t *testing.T) {
	tr := true
	fa := false
	pin := strings.Repeat("ab", 32)
	route := entity.ServiceGatewayRoute{
		Hostname: "gated.env-dev.mesh.example.net",
		HTTP:     true,
		HTTPS:    true,
		Auth: &entity.RouteAuthConfig{
			Password:     valueobject.NewSecretRefPlain("pw"),
			MaxFailures:  3,
			LockDuration: "30m",
		},
		WebSocketInsecureSkip: &tr,
	}
	gen, gw, config := authGatewayFixture(t, route)

	gw.GatewayStrictPathCheck = &fa
	gw.GatewayWebSocketInsecureSkip = &tr
	gw.GatewayWAF = &entity.GatewayWAFConfig{
		Enabled:                    true,
		DefaultMode:                "block",
		CRSPinnedSHA256:            "sha256:" + pin,
		CRSMirrorAllowInsecureHTTP: &tr,
	}
	gw.GatewayBlacklist = &entity.GatewayBlacklistConfig{
		Enabled:         true,
		DefaultMode:     "block",
		AllowHTTPSource: &tr,
		MaxEntries:      300000,
	}

	result, err := gen.buildGatewayRoutes(gw, config)
	if err != nil {
		t.Fatalf("buildGatewayRoutes: %v", err)
	}
	content, err := gen.gateGen.Generate(result.gatewayConfig, result.hosts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	var doc map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &doc); err != nil {
		t.Fatalf("unmarshal generated yaml: %v\n%s", err, content)
	}

	server, ok := doc["server"].(map[string]interface{})
	if !ok {
		t.Fatalf("server section missing:\n%s", content)
	}
	if v, _ := server["strict_path_check"].(bool); v != false {
		t.Errorf("server.strict_path_check = %v, want false\n%s", server["strict_path_check"], content)
	}
	if v, _ := server["websocket_insecure_skip"].(bool); v != true {
		t.Errorf("server.websocket_insecure_skip = %v, want true\n%s", server["websocket_insecure_skip"], content)
	}

	waf, ok := doc["waf"].(map[string]interface{})
	if !ok {
		t.Fatalf("waf section missing:\n%s", content)
	}
	crs, _ := waf["crs"].(map[string]interface{})
	dl, _ := crs["download"].(map[string]interface{})
	if dl == nil {
		t.Fatalf("waf.crs.download section missing:\n%s", content)
	}
	if v, _ := dl["pinned_sha256"].(string); v != "sha256:"+pin {
		t.Errorf("waf.crs.download.pinned_sha256 = %v, want sha256:%s\n%s", dl["pinned_sha256"], pin, content)
	}
	if v, _ := dl["mirror_allow_insecure_http"].(bool); v != true {
		t.Errorf("waf.crs.download.mirror_allow_insecure_http = %v, want true\n%s", dl["mirror_allow_insecure_http"], content)
	}

	bl, ok := doc["blacklist"].(map[string]interface{})
	if !ok {
		t.Fatalf("blacklist section missing:\n%s", content)
	}
	if v, _ := bl["allow_http_source"].(bool); v != true {
		t.Errorf("blacklist.allow_http_source = %v, want true\n%s", bl["allow_http_source"], content)
	}
	if v, _ := bl["max_entries"].(int); v != 300000 {
		t.Errorf("blacklist.max_entries = %v, want 300000\n%s", bl["max_entries"], content)
	}

	hosts, _ := doc["hosts"].([]interface{})
	var host map[string]interface{}
	for _, raw := range hosts {
		if m, ok := raw.(map[string]interface{}); ok && m["name"] == route.Hostname {
			host = m
			break
		}
	}
	if host == nil {
		t.Fatalf("host %s missing:\n%s", route.Hostname, content)
	}
	if v, _ := host["web_socket_insecure_skip"].(bool); v != true {
		t.Errorf("hosts[].web_socket_insecure_skip = %v, want true\n%s", host["web_socket_insecure_skip"], content)
	}
	auth, _ := host["auth"].(map[string]interface{})
	if auth == nil {
		t.Fatalf("hosts[].auth section missing:\n%s", content)
	}
	if v, _ := auth["max_failures"].(int); v != 3 {
		t.Errorf("auth.max_failures = %v, want 3\n%s", auth["max_failures"], content)
	}
	if v, _ := auth["lock_duration"].(string); v != "30m" {
		t.Errorf("auth.lock_duration = %v, want 30m\n%s", auth["lock_duration"], content)
	}
}

// 存量兼容硬约束：新字段全部未配置时产物不出现任何新键（与旧版逐字节一致）
func TestGenerateGateYAML_SecurityFieldsByteCompatibility(t *testing.T) {
	route := entity.ServiceGatewayRoute{
		Hostname: "svc.env-dev.mesh.example.net",
		HTTP:     true,
		HTTPS:    true,
		Auth:     &entity.RouteAuthConfig{Password: valueobject.NewSecretRefPlain("pw")},
	}
	gen, gw, config := authGatewayFixture(t, route)
	// waf/blacklist 段仍配置（legacy 已有形态），但不含任何新字段
	gw.GatewayWAF = &entity.GatewayWAFConfig{
		Enabled:      true,
		DefaultMode:  "block",
		CRSMirrorURL: "https://crsmirror.env-dev.mesh.example.net",
	}
	gw.GatewayBlacklist = &entity.GatewayBlacklistConfig{Enabled: true, DefaultMode: "detect"}

	result, err := gen.buildGatewayRoutes(gw, config)
	if err != nil {
		t.Fatalf("buildGatewayRoutes: %v", err)
	}
	content, err := gen.gateGen.Generate(result.gatewayConfig, result.hosts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	for _, key := range []string{
		"strict_path_check", "websocket_insecure_skip", "web_socket_insecure_skip",
		"pinned_sha256", "mirror_allow_insecure_http",
		"allow_http_source", "max_entries",
		"max_failures", "lock_duration",
	} {
		if strings.Contains(content, key) {
			t.Errorf("legacy output must not contain %q, got:\n%s", key, content)
		}
	}
}
