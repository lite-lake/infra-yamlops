package service

import (
	"testing"

	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
)

// orphanTestAllowSet 构造与真实调用路径一致的放行集合：
// 业务服务 2 个（含一个恰好以 -agent 结尾的已登记名），infra 服务 2 个。
func orphanTestAllowSet(t *testing.T) map[string]struct{} {
	t.Helper()
	cfg := &entity.Config{
		Services: []entity.BizService{
			{Name: "app-html2any-api-dev"},
			{Name: "foo"},
		},
		InfraServices: []entity.InfraService{
			{Name: "gateway-node1", Type: entity.InfraServiceTypeGateway},
			{Name: "easytier-node1", Type: entity.InfraServiceTypeEasyTier},
		},
	}
	return BuildOrphanAllowSet(cfg.GetServiceMap(), cfg.GetInfraServiceMap())
}

func TestBuildOrphanAllowSet(t *testing.T) {
	allow := orphanTestAllowSet(t)

	wantAllowed := []string{
		"app-html2any-api-dev", // 已登记业务服务
		"foo",
		"gateway-node1",  // 已登记 infra 服务
		"easytier-node1", // 已登记 infra 服务
	}
	for _, name := range wantAllowed {
		if _, ok := allow[name]; !ok {
			t.Errorf("BuildOrphanAllowSet() missing %q", name)
		}
	}

	wantDenied := []string{
		"gateway-node1-agent", // 不再有派生 agent 名
		"app-html2any-api",    // 未登记名
	}
	for _, name := range wantDenied {
		if _, ok := allow[name]; ok {
			t.Errorf("BuildOrphanAllowSet() unexpectedly contains %q", name)
		}
	}
}

func TestIsOrphanServiceName(t *testing.T) {
	allow := orphanTestAllowSet(t)

	tests := []struct {
		name        string
		serviceName string
		wantOrphan  bool
	}{
		{"已登记业务服务名", "app-html2any-api-dev", false},
		{"已登记 infra 服务名（gateway）", "gateway-node1", false},
		{"已登记 infra 服务名（easytier）", "easytier-node1", false},
		{"派生 agent 名不再是放行项", "gateway-node1-agent", true},
		{"真实孤儿（已删注册的残留）", "app-gone-svc-dev", true},
		{"恰好 foo-agent 且 foo 已登记（不放行宽松后缀）", "foo-agent", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsOrphanServiceName(tt.serviceName, allow); got != tt.wantOrphan {
				t.Errorf("IsOrphanServiceName(%q) = %v, want orphan = %v", tt.serviceName, got, tt.wantOrphan)
			}
		})
	}
}

func TestBuildOrphanAllowSet_NilInfraEntry(t *testing.T) {
	// 防御：infra 服务映射出现 nil 指针时不 panic
	allow := BuildOrphanAllowSet(
		map[string]*entity.BizService{"app-a-dev": {Name: "app-a-dev"}},
		map[string]*entity.InfraService{"easytier-x": nil},
	)
	if _, ok := allow["app-a-dev"]; !ok {
		t.Error("BuildOrphanAllowSet() missing biz service name")
	}
	if _, ok := allow["easytier-x"]; !ok {
		t.Error("BuildOrphanAllowSet() missing infra service name")
	}
}
