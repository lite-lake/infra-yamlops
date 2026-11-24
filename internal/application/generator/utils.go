package generator

import (
	"strings"

	"github.com/lite-lake/infra-yamlops/internal/application/generator/compose"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
)

// buildServiceResources 把实体层资源限制转换为 compose deploy.resources.limits；
// 两项都为空时返回 nil，产物不生成 deploy 段（存量输出不变）。
func buildServiceResources(res entity.ServiceResources) *compose.Resources {
	if res.CPU == "" && res.Memory == "" {
		return nil
	}
	return &compose.Resources{
		Limits: &compose.ResourceLimits{
			Cpus:   res.CPU,
			Memory: res.Memory,
		},
	}
}

func convertVolumeProtocol(v string) string {
	return strings.Replace(v, "volumes://", "./", 1)
}

func extractNamedVolume(v string) string {
	if strings.HasPrefix(v, "./") || strings.HasPrefix(v, "/") {
		return ""
	}
	parts := strings.SplitN(v, ":", 2)
	if len(parts) >= 1 && parts[0] != "" {
		return parts[0]
	}
	return ""
}
