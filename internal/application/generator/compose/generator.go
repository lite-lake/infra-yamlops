package compose

import (
	"fmt"

	"github.com/lite-lake/infra-yamlops/internal/constants"
	domainerr "github.com/lite-lake/infra-yamlops/internal/domain"
	"gopkg.in/yaml.v3"
)

type Generator struct{}

func NewGenerator() *Generator {
	return &Generator{}
}

func (g *Generator) Generate(svc *ComposeService, env string) (string, error) {
	if svc == nil {
		return "", fmt.Errorf("%w: service cannot be nil", domainerr.ErrServiceInvalid)
	}
	if svc.Name == "" {
		return "", fmt.Errorf("%w: service name cannot be empty", domainerr.ErrRequired)
	}
	if svc.Image == "" {
		return "", fmt.Errorf("%w: service image cannot be empty", domainerr.ErrRequired)
	}
	if env == "" {
		env = "dev"
	}

	serviceName, service := buildComposeService(svc, env)

	networks := make(map[string]*ExternalNetwork)
	if svc.NetworkMode != "host" {
		for _, netName := range svc.Networks {
			networks[netName] = &ExternalNetwork{External: true}
		}
	}

	compose := ComposeFile{
		Version: constants.ComposeVersion,
		Services: map[string]Service{
			serviceName: service,
		},
		Networks: networks,
	}

	data, err := yaml.Marshal(&compose)
	if err != nil {
		return "", fmt.Errorf("%w: %w", domainerr.ErrComposeGenerateFailed, err)
	}

	return string(data), nil
}

// GenerateMulti 生成单文件多服务 compose。服务 key 为 yo-{env}-{svc.Name}，
// 与单服务 Generate 的网络/别名逻辑保持一致。
func (g *Generator) GenerateMulti(services []*ComposeService, env string) (string, error) {
	if len(services) == 0 {
		return "", fmt.Errorf("%w: services cannot be empty", domainerr.ErrRequired)
	}
	if env == "" {
		env = "dev"
	}

	names := make(map[string]bool, len(services))
	for _, svc := range services {
		if svc == nil {
			return "", fmt.Errorf("%w: service cannot be nil", domainerr.ErrServiceInvalid)
		}
		if svc.Name == "" {
			return "", fmt.Errorf("%w: service name cannot be empty", domainerr.ErrRequired)
		}
		if svc.Image == "" {
			return "", fmt.Errorf("%w: service image cannot be empty", domainerr.ErrRequired)
		}
		if names[svc.Name] {
			return "", fmt.Errorf("%w: duplicate service name '%s' in multi compose", domainerr.ErrServiceInvalid, svc.Name)
		}
		names[svc.Name] = true
	}

	serviceMap := make(map[string]Service, len(services))
	networks := make(map[string]*ExternalNetwork)
	for _, svc := range services {
		serviceName, service := buildComposeService(svc, env)
		service.DependsOn = resolveDependsOn(svc.DependsOn, names, env)
		serviceMap[serviceName] = service
		if svc.NetworkMode != "host" {
			for _, netName := range svc.Networks {
				networks[netName] = &ExternalNetwork{External: true}
			}
		}
	}

	compose := ComposeFile{
		Version:  constants.ComposeVersion,
		Services: serviceMap,
		Networks: networks,
	}

	data, err := yaml.Marshal(&compose)
	if err != nil {
		return "", fmt.Errorf("%w: %w", domainerr.ErrComposeGenerateFailed, err)
	}

	return string(data), nil
}

// buildComposeService 将 ComposeService 转换为 compose 的 Service（共享网络/别名/资源逻辑）。
func buildComposeService(svc *ComposeService, env string) (string, Service) {
	serviceName := "yo-" + env + "-" + svc.Name

	restart := svc.Restart
	if restart == "" {
		restart = constants.DefaultRestartPolicy
	}

	service := Service{
		Image:         svc.Image,
		ContainerName: serviceName,
		NetworkMode:   svc.NetworkMode,
		Privileged:    svc.Privileged,
		CapAdd:        svc.CapAdd,
		Devices:       svc.Devices,
		Ulimits:       svc.Ulimits,
		Command:       svc.Command,
		Ports:         svc.Ports,
		Environment:   svc.Environment,
		EnvFiles:      svc.EnvFiles,
		Volumes:       svc.Volumes,
		HealthCheck:   svc.HealthCheck,
		Restart:       restart,
		ExtraHosts:    svc.ExtraHosts,
		Logging:       defaultLogRotation(),
	}

	if svc.NetworkMode != "host" {
		networkConfigs := make(map[string]*NetworkConfig)
		for _, netName := range svc.Networks {
			networkConfigs[netName] = &NetworkConfig{
				Aliases: []string{svc.Name},
			}
		}
		service.Networks = networkConfigs
	}

	if svc.Resources != nil {
		service.Deploy = &Deploy{
			Resources: svc.Resources,
		}
	}

	return serviceName, service
}

// defaultLogRotation 返回 json-file 日志轮转默认配置，防止单容器日志无限
// 增长吃满宿主机磁盘。默认 10MB × 3 份（每容器最多 ~30MB）。
func defaultLogRotation() *LogRotation {
	return &LogRotation{
		Driver: "json-file",
		Options: map[string]string{
			"max-size": constants.DefaultLogRotateSize,
			"max-file": constants.DefaultLogRotateFiles,
		},
	}
}

// resolveDependsOn 把 depends_on 中以兄弟服务 Name 为 key 的条目改写为
// 实际的 compose 服务 key（yo-{env}-{name}）；非兄弟名称原样保留。
func resolveDependsOn(deps map[string]DependsOnEntry, names map[string]bool, env string) map[string]DependsOnEntry {
	if len(deps) == 0 {
		return nil
	}
	resolved := make(map[string]DependsOnEntry, len(deps))
	for k, v := range deps {
		if names[k] {
			resolved["yo-"+env+"-"+k] = v
		} else {
			resolved[k] = v
		}
	}
	return resolved
}
