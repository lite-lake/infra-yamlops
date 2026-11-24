package service

import (
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
)

// BuildOrphanAllowSet 由配置构建孤儿判定放行集合：已登记业务服务名 + 基础设施服务名。
func BuildOrphanAllowSet(serviceMap map[string]*entity.BizService, infraServiceMap map[string]*entity.InfraService) map[string]struct{} {
	allow := make(map[string]struct{}, len(serviceMap)+len(infraServiceMap))
	for name := range serviceMap {
		allow[name] = struct{}{}
	}
	for name := range infraServiceMap {
		allow[name] = struct{}{}
	}
	return allow
}

// IsOrphanServiceName 判定剥离 yo-{env}- 前缀后的资源名是否为孤儿。
func IsOrphanServiceName(serviceName string, allowSet map[string]struct{}) bool {
	_, ok := allowSet[serviceName]
	return !ok
}
