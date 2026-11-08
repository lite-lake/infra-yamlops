package generator

import (
	"os"
	"strings"
	"testing"

	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
)

func hstsPtr(s string) *string { return &s }

// secondHopCase 描述一组「第二跳条目 hsts 标注告警」判定用例。
type secondHopCase struct {
	name string
	env  string
	cfg  *entity.Config
	// want 期望命中的第二跳条目（"服务名/host 条目名"）与来源服务列表，顺序与配置一致
	want []struct {
		service  string
		hostname string
		sources  []string
	}
}

// edgeProxy 构造一个服务：在 server 上以 external_backends 指向 target（可选显式 override_host），
// 并声明 hostnames 作为入口条目（可多个，模拟一个服务多条目）。
func edgeProxy(name, server, target string, hostnames ...string) entity.BizService {
	svc := entity.BizService{
		Name:             name,
		ServiceBase:      entity.ServiceBase{Server: server},
		ExternalBackends: []string{target},
	}
	for _, h := range hostnames {
		svc.Gateways = append(svc.Gateways, entity.ServiceGatewayRoute{Hostname: h, HTTP: true, HTTPS: true})
	}
	return svc
}

// innerEntry 构造第二跳侧服务：image 服务（无 external_backends），hostname + 可选 hsts 标注。
func innerEntry(name, server, hostname string, hsts *string) entity.BizService {
	return entity.BizService{
		Name:        name,
		ServiceBase: entity.ServiceBase{Server: server},
		Image:       "demo:latest",
		Gateways: []entity.ServiceGatewayRoute{
			{Hostname: hostname, HTTPS: true, HSTS: hsts},
		},
	}
}

// 第二跳条目判定与 hsts 三态（内部PRD §5.6，决策 D5：仅告警不阻断不自动注入）。
func TestCheckSecondHopHSTS(t *testing.T) {
	const meshTarget = "app-demo.node1.v-dev.mesh.example.net"

	// 第二跳 + hsts 未配置（nil）→ 告警
	missingNil := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"}, Services: []entity.BizService{
		edgeProxy("demo-node8-proxy", "srv-node8", "https://"+meshTarget, "demo.mesh.example.net"),
		innerEntry("demo-web-dev", "srv-node1", meshTarget, nil),
	}}

	// 第二跳 + hsts: "off" → 不告警
	off := hstsPtr("off")
	markedOff := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"}, Services: []entity.BizService{
		edgeProxy("demo-node8-proxy", "srv-node8", "https://"+meshTarget, "demo.mesh.example.net"),
		innerEntry("demo-web-dev", "srv-node1", meshTarget, off),
	}}

	// 第二跳 + 字面值 → 告警（第二跳同样会下发 HSTS）
	literal := hstsPtr("max-age=31536000; includeSubDomains")
	markedLiteral := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"}, Services: []entity.BizService{
		edgeProxy("demo-node8-proxy", "srv-node8", "https://"+meshTarget, "demo.mesh.example.net"),
		innerEntry("demo-web-dev", "srv-node1", meshTarget, literal),
	}}

	// 第二跳 + "OFF"（gate GetEffectiveHSTS 大小写不敏感）→ 不告警
	offUpper := hstsPtr("OFF")
	markedOffUpper := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"}, Services: []entity.BizService{
		edgeProxy("demo-node8-proxy", "srv-node8", "https://"+meshTarget, "demo.mesh.example.net"),
		innerEntry("demo-web-dev", "srv-node1", meshTarget, offUpper),
	}}

	// 第二跳 + ""（不下发 HSTS 但不符 "off" 约定）→ 告警（任务口径：非 "off" 即告警）
	empty := hstsPtr("")
	markedEmpty := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"}, Services: []entity.BizService{
		edgeProxy("demo-node8-proxy", "srv-node8", "https://"+meshTarget, "demo.mesh.example.net"),
		innerEntry("demo-web-dev", "srv-node1", meshTarget, empty),
	}}

	// 非第二跳条目（无任何 external_backends 指向它）+ 未配置 → 不告警
	notSecondHop := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"}, Services: []entity.BizService{
		edgeProxy("other-node8-proxy", "srv-node8", "https://app-other.node1.v-dev.mesh.example.net", "other.mesh.example.net"),
		innerEntry("demo-web-dev", "srv-node1", meshTarget, nil),
	}}

	// 多跳映射：两个 external_backends 服务指向同一条目 → 单条告警，来源去重
	multiSourceMissing := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"}, Services: []entity.BizService{
		edgeProxy("demo-node8-proxy", "srv-node8", "https://"+meshTarget, "demo.mesh.example.net"),
		edgeProxy("demo-node7-proxy", "srv-node7", "https://"+meshTarget, "demo-us.mesh.example.net"),
		innerEntry("demo-web-dev", "srv-node1", meshTarget, nil),
	}}

	// 多跳映射 + 已标注 off → 不误报
	multiSourceOff := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"}, Services: []entity.BizService{
		edgeProxy("demo-node8-proxy", "srv-node8", "https://"+meshTarget, "demo.mesh.example.net"),
		edgeProxy("demo-node7-proxy", "srv-node7", "https://"+meshTarget, "demo-us.mesh.example.net"),
		innerEntry("demo-web-dev", "srv-node1", meshTarget, off),
	}}

	// 一个 external_backends 服务多条目（多入口共用一个第二跳目标）→ 不放大告警
	multiRouteOff := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"}, Services: []entity.BizService{
		edgeProxy("callbacks-node8-proxy", "srv-node8", "https://"+meshTarget,
			"pay.mesh.example.net", "wechat.mesh.example.net", "alipay.mesh.example.net"),
		innerEntry("demo-web-dev", "srv-node1", meshTarget, off),
	}}
	multiRouteMissing := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"}, Services: []entity.BizService{
		edgeProxy("callbacks-node8-proxy", "srv-node8", "https://"+meshTarget,
			"pay.mesh.example.net", "wechat.mesh.example.net", "alipay.mesh.example.net"),
		innerEntry("demo-web-dev", "srv-node1", meshTarget, nil),
	}}

	// 第三方 / 容器名 / 裸 IP 目标：不是本仓内层网关的第二跳，即便 hostname 恰好同名也不告警
	thirdPartyTarget := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"}, Services: []entity.BizService{
		edgeProxy("disguise-proxy", "srv-node7", "https://api.example.com", "api.example.com"),
		innerEntry("demo-web-dev", "srv-node1", "api.example.com", nil),
	}}
	containerTarget := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"}, Services: []entity.BizService{
		edgeProxy("ct-dicom-proxy-dev", "srv-node3", "http://dicom-web-proxy:80", "ct.env-dev.mesh.example.net"),
		innerEntry("nas-proxy-dev", "srv-node1", "nas2.ngc4486.com", nil),
	}}
	// 显式 override_host 钉住公网域名（nas 形态）：目标是自己的一跳入口，不属第二跳
	nasPinned := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"}, Services: []entity.BizService{
		{
			Name:             "nas-proxy-dev",
			ServiceBase:      entity.ServiceBase{Server: "srv-node1"},
			ExternalBackends: []string{"http://192.168.60.80:5000"},
			Gateways: []entity.ServiceGatewayRoute{
				{Hostname: "nas2.ngc4486.com", HTTPS: true, OverrideHost: "nas2.ngc4486.com"},
			},
		},
	}}

	// 显式 override_host 指向 mesh 域名（warden 链形态）→ 同样构成第二跳映射，未标注即告警
	explicitMeshTarget := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"}, Services: []entity.BizService{
		{
			Name:             "pinned-proxy",
			ServiceBase:      entity.ServiceBase{Server: "srv-node3"},
			ExternalBackends: []string{"https://warden.v-local.mesh.example.net"},
			Gateways: []entity.ServiceGatewayRoute{
				{Hostname: "warden.ngc4486.com", HTTPS: true},
				{Hostname: "warden.v-dev.mesh.example.net", HTTPS: true, OverrideHost: "warden.v-dev.mesh.example.net"},
			},
		},
	}}

	// 跨环境后缀（dev 环境下指向 v-prod mesh 域名）→ 非本环境 mesh 目标，不告警
	crossEnv := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"}, Services: []entity.BizService{
		edgeProxy("cross-proxy", "srv-node8", "https://app-demo.node5.v-prod.mesh.example.net", "demo.mesh.example.net"),
		innerEntry("demo-prod-entry", "srv-node5", "app-demo.node5.v-prod.mesh.example.net", nil),
	}}

	// 目标 host 无对应条目（如 dev 第三跳的 local 网关，不属本仓部署面）→ 不告警
	noMatchingEntry := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"}, Services: []entity.BizService{
		edgeProxy("warden-node8-proxy", "srv-node8", "https://warden.v-dev.mesh.example.net", "warden.ngc4486.com"),
	}}

	tests := []secondHopCase{
		{
			name: "第二跳条目未配置 hsts 告警",
			env:  "dev",
			cfg:  missingNil,
			want: []struct {
				service  string
				hostname string
				sources  []string
			}{{"demo-web-dev", meshTarget, []string{"demo-node8-proxy"}}},
		},
		{
			name: "第二跳条目已标 off 不告警",
			env:  "dev",
			cfg:  markedOff,
		},
		{
			name: "第二跳条目配置字面值告警",
			env:  "dev",
			cfg:  markedLiteral,
			want: []struct {
				service  string
				hostname string
				sources  []string
			}{{"demo-web-dev", meshTarget, []string{"demo-node8-proxy"}}},
		},
		{
			name: "off 大小写不敏感不告警",
			env:  "dev",
			cfg:  markedOffUpper,
		},
		{
			name: "空字符串按非 off 口径告警",
			env:  "dev",
			cfg:  markedEmpty,
			want: []struct {
				service  string
				hostname string
				sources  []string
			}{{"demo-web-dev", meshTarget, []string{"demo-node8-proxy"}}},
		},
		{
			name: "非第二跳条目未配置不告警",
			env:  "dev",
			cfg:  notSecondHop,
		},
		{
			name: "多服务指向同一条目单条告警且来源去重",
			env:  "dev",
			cfg:  multiSourceMissing,
			want: []struct {
				service  string
				hostname string
				sources  []string
			}{{"demo-web-dev", meshTarget, []string{"demo-node8-proxy", "demo-node7-proxy"}}},
		},
		{
			name: "多服务指向同一条目且已标注不误报",
			env:  "dev",
			cfg:  multiSourceOff,
		},
		{
			name: "一个服务多条目且已标注不误报",
			env:  "dev",
			cfg:  multiRouteOff,
		},
		{
			name: "一个服务多条目未标注仍只报一条",
			env:  "dev",
			cfg:  multiRouteMissing,
			want: []struct {
				service  string
				hostname string
				sources  []string
			}{{"demo-web-dev", meshTarget, []string{"callbacks-node8-proxy"}}},
		},
		{
			name: "第三方域名目标不告警",
			env:  "dev",
			cfg:  thirdPartyTarget,
		},
		{
			name: "容器名目标不告警",
			env:  "dev",
			cfg:  containerTarget,
		},
		{
			name: "显式 override 钉住自身公网入口不告警",
			env:  "dev",
			cfg:  nasPinned,
		},
		{
			name: "显式 override 指向 mesh 域名构成第二跳并告警",
			env:  "dev",
			cfg:  explicitMeshTarget,
			want: []struct {
				service  string
				hostname string
				sources  []string
			}{{"pinned-proxy", "warden.v-dev.mesh.example.net", []string{"pinned-proxy"}}},
		},
		{
			name: "跨环境 mesh 目标不告警",
			env:  "dev",
			cfg:  crossEnv,
		},
		{
			name: "目标无对应条目不告警（第三跳不在本仓部署面）",
			env:  "dev",
			cfg:  noMatchingEntry,
		},
		{
			name: "nil 配置安全返回空",
			env:  "dev",
			cfg:  nil,
		},
		{
			name: "空配置返回空",
			env:  "dev",
			cfg:  &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckSecondHopHSTS(tt.cfg, tt.env)
			if len(got) != len(tt.want) {
				t.Fatalf("CheckSecondHopHSTS() = %d warnings %v, want %d", len(got), got, len(tt.want))
			}
			for i, w := range tt.want {
				if got[i].Service != w.service || got[i].Hostname != w.hostname {
					t.Errorf("warning[%d] = %s/%s, want %s/%s", i, got[i].Service, got[i].Hostname, w.service, w.hostname)
				}
				if strings.Join(got[i].Sources, ",") != strings.Join(w.sources, ",") {
					t.Errorf("warning[%d] sources = %v, want %v", i, got[i].Sources, w.sources)
				}
			}
		})
	}
}

// 告警文案可定位（PRD §5.6 要求）：含服务名、host 条目名、应配 hsts: "off" 的指引与 PRD 编号。
func TestSecondHopHSTS_MessageAndSuggestion(t *testing.T) {
	const meshTarget = "app-demo.node1.v-dev.mesh.example.net"
	cfg := &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"}, Services: []entity.BizService{
		edgeProxy("demo-node8-proxy", "srv-node8", "https://"+meshTarget, "demo.mesh.example.net"),
		innerEntry("demo-web-dev", "srv-node1", meshTarget, nil),
	}}

	got := CheckSecondHopHSTS(cfg, "dev")
	if len(got) != 1 {
		t.Fatalf("warnings = %d, want 1", len(got))
	}
	msg := got[0].Message()
	for _, want := range []string{"demo-web-dev", meshTarget, "demo-node8-proxy"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q missing %q", msg, want)
		}
	}
	suggestion := got[0].Suggestion()
	for _, want := range []string{"hsts: \"off\"", meshTarget, "demo-web-dev", "the edge gateway is the single sender"} {
		if !strings.Contains(suggestion, want) {
			t.Errorf("suggestion %q missing %q", suggestion, want)
		}
	}

	// 字面值形态的文案需说明当前值仍会下发 HSTS
	literal := hstsPtr("max-age=60")
	cfg.Services[1].Gateways[0].HSTS = literal
	got = CheckSecondHopHSTS(cfg, "dev")
	if len(got) != 1 {
		t.Fatalf("warnings = %d, want 1", len(got))
	}
	if msg := got[0].Message(); !strings.Contains(msg, `"max-age=60"`) || !strings.Contains(msg, "still sends HSTS") {
		t.Errorf("literal message = %q, want configured value + still-sends explanation", msg)
	}
}

// 生成期告警不改变渲染产物：同一配置告警前后产物逐字节一致（D5：仅告警、不自动注入）。
func TestGenerateWithScope_SecondHopWarningKeepsArtifactsByteIdentical(t *testing.T) {
	const meshTarget = "app-demo.node1.v-dev.mesh.example.net"
	build := func(hsts *string) *entity.Config {
		return &entity.Config{Mesh: &entity.MeshConfig{DomainZone: "mesh.example.net"},
			InfraServices: []entity.InfraService{
				{
					Name:         "gateway-node1",
					Type:         entity.InfraServiceTypeGateway,
					ServiceBase:  entity.ServiceBase{Server: "srv-node1"},
					Image:        "gate:latest",
					GatewayPorts: &entity.GatewayPorts{HTTP: 80, HTTPS: 443},
				},
			},
			Services: []entity.BizService{
				edgeProxy("demo-node8-proxy", "srv-node8", "https://"+meshTarget, "demo.mesh.example.net"),
				innerEntry("demo-web-dev", "srv-node1", meshTarget, hsts),
			},
		}
	}

	render := func(hsts *string) map[string]string {
		gen := NewGenerator("dev", t.TempDir())
		cfg := build(hsts)
		if err := gen.Generate(cfg); err != nil {
			t.Fatalf("Generate: %v", err)
		}
		// 网关所在服务器目录下的产物（.env 为 map 顺序写入、逐次不同，不纳入比对）
		dir := gen.outputDir + "/dev/srv-node1"
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("ReadDir: %v", err)
		}
		out := make(map[string]string, len(entries))
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".env") {
				continue
			}
			data, err := os.ReadFile(dir + "/" + e.Name())
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			out[e.Name()] = string(data)
		}
		return out
	}

	// 漏标（告警）与已标注（无告警）两种形态：产物除 hsts 键外逐字节一致
	missing := render(nil)
	marked := render(hstsPtr("off"))
	if len(missing) != len(marked) {
		t.Fatalf("artifact file count differs: %d vs %d", len(missing), len(marked))
	}
	for name, missingContent := range missing {
		markedContent, ok := marked[name]
		if !ok {
			t.Fatalf("artifact %s missing in marked render", name)
		}
		if strings.Contains(missingContent, "hsts") {
			t.Errorf("漏标条目不应生成 hsts 键（不自动注入），got %s:\n%s", name, missingContent)
		}
		if got := stripHSTSLine(missingContent); got != stripHSTSLine(markedContent) {
			t.Errorf("artifact %s changed beyond hsts line:\n--- missing\n%s\n--- marked\n%s",
				name, stripHSTSLine(missingContent), stripHSTSLine(markedContent))
		}
	}
}

// stripHSTSLine 去掉 hsts 行（"off" 标注形态），用于比对「除标注外逐字节一致」
func stripHSTSLine(s string) string {
	var kept []string
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) == `hsts: "off"` {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}
