package generator

import (
	"fmt"
	"strings"

	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/logger"
)

// SecondHopHSTS 一处「第二跳条目未标 hsts: "off"」的告警项（内部PRD §5.6）。
//
// 「第二跳条目」= 被其它服务的 external_backends 指为上游 Host 目标的 host 条目：
// 生成器对 external_backends 服务的 gateway host 自动把上游请求的 Host 改写为
// external_backends URL 的 host（v1.2 模型，v1.0.21；route.override_host 显式配置优先，
// 见 buildHostRoute），该 Host 即内层网关按 mesh 域名（app-xxx.{节点}.v-{env}.{zone}，zone 见 meshDomainZone）
// 命中的条目——于是同一次响应在内层网关又下发一条 HSTS，与边缘那条构成双发（PRD §4.1）。
// 按 W3-2 约定（内部约定 A10 / D8）第二跳条目应标 hsts: "off"，本项即对该约定的漂移告警。
//
// 仅告警（PRD 决策 D5）：不自动注入、不改写任何配置或渲染产物、不阻断生成。
type SecondHopHSTS struct {
	// Service 承载该 host 条目的服务名（services_biz.yaml 中 gateways 所在服务，即第二跳条目侧）
	Service string
	// Hostname host 条目名（gateway hostname）
	Hostname string
	// Sources 把该条目当作上游目标的 external_backends 服务名（去重，按配置顺序）
	Sources []string
	// Configured 该条目当前 hsts 值：nil = 未配置（下发 gate 默认值）；非 nil = 显式配置值
	Configured *string
}

// Message 告警正文：含服务名、host 条目名、信息来源（external_backends 服务）与当前 hsts 状态。
func (w SecondHopHSTS) Message() string {
	state := "is not configured (the gateway default value will be sent)"
	if w.Configured != nil {
		state = fmt.Sprintf("is set to %q, which still sends HSTS", *w.Configured)
	}
	return fmt.Sprintf("Service '%s': gateway host '%s' is a second-hop entry (upstream Host target of external_backends in service %s) but hsts %s",
		w.Service, w.Hostname, quoteNames(w.Sources), state)
}

// Suggestion 处置指引（PRD §5.6 / 决策 D5：仅告警，不自动注入）。
func (w SecondHopHSTS) Suggestion() string {
	return fmt.Sprintf("Add hsts: \"off\" to gateway host '%s' in service '%s' (a second-hop entry must not send HSTS: the edge gateway is the single sender; 内部PRD §5.6)",
		w.Hostname, w.Service)
}

// CheckSecondHopHSTS 检查被 external_backends 指为上游目标的 host 条目是否标注 hsts: "off"
// （内部PRD §5.6）。返回空切片 = 无告警。纯只读：不改配置、不改产物、不影响生成结果。
//
// 判定与生成器的 override 决策保持一致（buildHostRoute）：
//   - 上游目标 Host = route.override_host（显式配置优先），未显式配置时取
//     external_backends URL 的 host；多个 URL host 不统一 / URL 非法时不自动 override
//     （按原 Host 转发），此时没有可用的第二跳映射信息，跳过；
//   - 仅当目标 Host 是本环境 mesh 域名（isMeshHostname → .{节点}.v-{env}.{zone}）时才算
//     第二跳：第三方站点 / 同网络容器名 / 裸 IP 目标不经本仓内层网关，不存在内层再次下发
//     HSTS 的问题，不计入（避免误报）；
//   - 目标 Host 必须确实命中本配置内的某个 host 条目（内层网关按该 Host 路由）；
//   - 命中条目已是 hsts: "off"（大小写不敏感，与 gate 的 GetEffectiveHSTS 同口径）→ 不告警；
//     nil（未配置）或任何非 "off" 值（字面值同样会在第二跳下发 HSTS；"" 亦按任务口径告警）
//     → 告警。
func CheckSecondHopHSTS(cfg *entity.Config, env string) []SecondHopHSTS {
	if cfg == nil {
		return nil
	}

	// 第一遍：收集第二跳目标 host → 指向它的 external_backends 服务名
	targetSources := make(map[string][]string)
	for i := range cfg.Services {
		svc := &cfg.Services[i]
		if len(svc.ExternalBackends) == 0 {
			continue
		}
		autoTarget, autoOK := externalBackendsOverrideHost(svc.ExternalBackends)
		for j := range svc.Gateways {
			route := &svc.Gateways[j]
			if !route.HasGateway() {
				continue
			}
			target := route.OverrideHost
			if target == "" {
				if !autoOK {
					continue // 未自动 override（多 host/URL 非法）：按原 Host 转发，无第二跳映射
				}
				target = autoTarget
			}
			if !isMeshHostname(target, env, meshDomainZone(cfg)) {
				continue // 非本环境 mesh 域名目标：不经本仓内层网关
			}
			targetSources[target] = appendUnique(targetSources[target], svc.Name)
		}
	}
	if len(targetSources) == 0 {
		return nil
	}

	// 第二遍：命中目标 host 的条目逐个检查 hsts 标注（按配置顺序输出，稳定可复现）
	var warnings []SecondHopHSTS
	for i := range cfg.Services {
		svc := &cfg.Services[i]
		for j := range svc.Gateways {
			route := &svc.Gateways[j]
			if !route.HasGateway() {
				continue
			}
			sources, ok := targetSources[route.Hostname]
			if !ok {
				continue
			}
			if route.HSTS != nil && strings.EqualFold(*route.HSTS, "off") {
				continue // 已按约定标注，正确
			}
			warnings = append(warnings, SecondHopHSTS{
				Service:    svc.Name,
				Hostname:   route.Hostname,
				Sources:    sources,
				Configured: route.HSTS,
			})
		}
	}

	return warnings
}

// warnSecondHopHSTS 生成期输出第二跳漏标告警（内部PRD §5.6）。
// 仅告警：不自动注入、不改写产物字节、不阻断生成（决策 D5）。
func (g *Generator) warnSecondHopHSTS(config *entity.Config) {
	for _, w := range CheckSecondHopHSTS(config, g.env) {
		logger.Warn(w.Message(), "env", g.env, "service", w.Service, "hostname", w.Hostname,
			"suggestion", w.Suggestion())
	}
}

// appendUnique 追加不重复项，保持首次出现顺序（不排序：输出顺序跟随配置顺序，稳定可复现）
func appendUnique(list []string, item string) []string {
	for _, existing := range list {
		if existing == item {
			return list
		}
	}
	return append(list, item)
}

// quoteNames 将名称列表渲染为 'a', 'b' 形式
func quoteNames(names []string) string {
	quoted := make([]string, 0, len(names))
	for _, name := range names {
		quoted = append(quoted, fmt.Sprintf("'%s'", name))
	}
	return strings.Join(quoted, ", ")
}
