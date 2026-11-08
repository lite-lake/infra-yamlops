package service

import (
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
)

// OrphanDerivedAgentSuffix uptime-kuma 同步 agent 的派生名后缀。
// generator/uptime.go 以 infra.Name + "-agent" 生成 agent 容器（及 /data/yamlops/ 下同名目录），
// 本常量与其命名保持一致；新增派生形态时同步扩展 BuildOrphanAllowSet。
const OrphanDerivedAgentSuffix = "-agent"

// BuildOrphanAllowSet 由配置构建孤儿判定放行集合：
// 已登记业务服务名 + 基础设施服务名 + yamlops 自管派生名
// （配置了 UptimeKumaConfig 的 infra 服务追加 <服务名>-agent）。
// 集合只增不减：已登记名的判定结果与 v1.0.23 精确匹配完全一致，
// 仅新增派生名放行（修复 uptime-kuma agent 被整库 service cleanup 误判为孤儿）。
func BuildOrphanAllowSet(serviceMap map[string]*entity.BizService, infraServiceMap map[string]*entity.InfraService) map[string]struct{} {
	allow := make(map[string]struct{}, len(serviceMap)+len(infraServiceMap))
	for name := range serviceMap {
		allow[name] = struct{}{}
	}
	for name, infra := range infraServiceMap {
		allow[name] = struct{}{}
		if infra != nil && infra.UptimeKumaConfig != nil {
			allow[name+OrphanDerivedAgentSuffix] = struct{}{}
		}
	}
	return allow
}

// IsOrphanServiceName 判定剥离 yo-{env}- 前缀后的资源名是否为孤儿。
// 采用显式放行集合而非 "-agent" 后缀宽松规则：
// 真实孤儿恰好叫 foo-agent（foo 已登记但未配置 uptime-kuma）时不漏删。
func IsOrphanServiceName(serviceName string, allowSet map[string]struct{}) bool {
	_, ok := allowSet[serviceName]
	return !ok
}
