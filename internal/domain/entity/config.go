package entity

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"

	"github.com/lite-lake/infra-yamlops/internal/domain"
)

// MaxPortNumber 最大端口号
const MaxPortNumber = 65535

// Config 配置实体，包含所有基础设施配置
type Config struct {
	Secrets       []Secret       `yaml:"secrets,omitempty"`
	ISPs          []ISP          `yaml:"isps,omitempty"`
	Registries    []Registry     `yaml:"registries,omitempty"`
	Zones         []Zone         `yaml:"zones,omitempty"`
	Servers       []Server       `yaml:"servers,omitempty"`
	InfraServices []InfraService `yaml:"infra_services,omitempty"`
	Services      []BizService   `yaml:"services,omitempty"`
	Domains       []Domain       `yaml:"domains,omitempty"`
	Mesh          *MeshConfig    `yaml:"mesh,omitempty"`
}

// MeshConfig mesh 组网全局设置（可选，来自 userdata/<env>/mesh.yaml）。
// 缺省（无该文件 / 未填 domain_zone）= mesh 域名相关自动行为整体关闭。
type MeshConfig struct {
	// DomainZone mesh 域名区域：hostname 以 .{节点}.v-{env}.{DomainZone} 结尾视为
	// mesh 域名（触发网关生成器 forwarded_for 自动打标、二级链路 HSTS 检查等行为）。
	DomainZone string `yaml:"domain_zone,omitempty"`
}

// domainZoneRegex 简化域名校验：小写字母/数字/点/连字符（如 example.net、mesh.internal）
var domainZoneRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$`)

// Validate 校验 mesh 设置（domain_zone 为空 = 关闭；非空须为合法域名且不超长）。
func (m *MeshConfig) Validate() error {
	if m == nil || m.DomainZone == "" {
		return nil
	}
	if len(m.DomainZone) > 253 || !domainZoneRegex.MatchString(m.DomainZone) {
		return fmt.Errorf("%w: mesh.domain_zone %q 不是合法域名", domain.ErrInvalidName, m.DomainZone)
	}
	return nil
}

// Validate 验证配置的完整性
func (c *Config) Validate() error {
	for i, s := range c.Secrets {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("secrets[%d]: %w", i, err)
		}
	}
	for i, isp := range c.ISPs {
		if err := isp.Validate(); err != nil {
			return fmt.Errorf("isps[%d]: %w", i, err)
		}
	}
	for i, r := range c.Registries {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("registries[%d]: %w", i, err)
		}
	}
	for i, z := range c.Zones {
		if err := z.Validate(); err != nil {
			return fmt.Errorf("zones[%d]: %w", i, err)
		}
	}
	for i, s := range c.Servers {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("servers[%d]: %w", i, err)
		}
	}
	for i, infra := range c.InfraServices {
		if err := infra.Validate(); err != nil {
			return fmt.Errorf("infra_services[%d]: %w", i, err)
		}
	}

	for i, s := range c.Services {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("services[%d]: %w", i, err)
		}
	}
	for i, d := range c.Domains {
		if err := d.Validate(); err != nil {
			return fmt.Errorf("domains[%d]: %w", i, err)
		}
	}
	if err := c.Mesh.Validate(); err != nil {
		return fmt.Errorf("mesh: %w", err)
	}
	return nil
}

// toMapPtr 将切片转换为以名称为键的指针映射（内部工具函数）
func toMapPtr[T any](items []T, getName func(T) string) map[string]*T {
	m := make(map[string]*T)
	for i := range items {
		m[getName(items[i])] = &items[i]
	}
	return m
}

// GetSecretsMap 获取密钥映射
func (c *Config) GetSecretsMap() map[string]string {
	m := make(map[string]string)
	for _, s := range c.Secrets {
		m[s.Name] = s.Value
	}
	return m
}

// GetISPMap 获取 ISP 映射
func (c *Config) GetISPMap() map[string]*ISP {
	return toMapPtr(c.ISPs, func(isp ISP) string { return isp.Name })
}

// GetZoneMap 获取区域映射
func (c *Config) GetZoneMap() map[string]*Zone {
	return toMapPtr(c.Zones, func(z Zone) string { return z.Name })
}

// GetInfraServiceMap 获取基础设施服务映射
func (c *Config) GetInfraServiceMap() map[string]*InfraService {
	return toMapPtr(c.InfraServices, func(s InfraService) string { return s.Name })
}

// GetServerMap 获取服务器映射
func (c *Config) GetServerMap() map[string]*Server {
	return toMapPtr(c.Servers, func(s Server) string { return s.Name })
}

// GetServiceMap 获取业务服务映射
func (c *Config) GetServiceMap() map[string]*BizService {
	return toMapPtr(c.Services, func(s BizService) string { return s.Name })
}

// GetRegistryMap 获取注册表映射
func (c *Config) GetRegistryMap() map[string]*Registry {
	return toMapPtr(c.Registries, func(r Registry) string { return r.Name })
}

// GetDomainMap 获取域名映射
func (c *Config) GetDomainMap() map[string]*Domain {
	return toMapPtr(c.Domains, func(d Domain) string { return d.Name })
}

// GetAllDNSRecords 获取所有 DNS 记录
func (c *Config) GetAllDNSRecords() []DNSRecord {
	var records []DNSRecord
	for _, d := range c.Domains {
		records = append(records, d.FlattenRecords()...)
	}
	return records
}

// GetEndpoints 获取所有对外暴露的应用端点
func (c *Config) GetEndpoints() []Endpoint {
	var endpoints []Endpoint
	for _, svc := range c.Services {
		if svc.Internal {
			continue
		}
		for _, route := range svc.Gateways {
			if !route.HasGateway() {
				continue
			}
			protocol := "https"
			if route.HTTP && !route.HTTPS {
				protocol = "http"
			}
			endpoints = append(endpoints, Endpoint{
				ServiceName: svc.Name,
				Hostname:    route.Hostname,
				Protocol:    protocol,
				Path:        route.Path,
				Server:      svc.Server,
			})
		}
	}
	sort.Slice(endpoints, func(i, j int) bool {
		if endpoints[i].Hostname != endpoints[j].Hostname {
			return endpoints[i].Hostname < endpoints[j].Hostname
		}
		return endpoints[i].ServiceName < endpoints[j].ServiceName
	})
	return endpoints
}

// ParsePort 解析端口号字符串
func ParsePort(s string) (int, error) {
	port, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%w: %s", domain.ErrInvalidPort, s)
	}
	if port <= 0 || port > MaxPortNumber {
		return 0, fmt.Errorf("%w: %d", domain.ErrInvalidPort, port)
	}
	return port, nil
}
