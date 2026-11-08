package service

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/lite-lake/infra-yamlops/internal/domain"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
)

type Validator struct {
	cfg *entity.Config

	secrets   map[string]string
	isps      map[string]*entity.ISP
	zones     map[string]*entity.Zone
	servers   map[string]*entity.Server
	domainMap map[string]*entity.Domain
}

func NewValidator(cfg *entity.Config) *Validator {
	if cfg == nil {
		return &Validator{cfg: nil}
	}
	return &Validator{
		cfg:       cfg,
		secrets:   cfg.GetSecretsMap(),
		isps:      cfg.GetISPMap(),
		zones:     cfg.GetZoneMap(),
		servers:   cfg.GetServerMap(),
		domainMap: cfg.GetDomainMap(),
	}
}

func (v *Validator) Validate() error {
	if v.cfg == nil {
		return domain.ErrConfigNotLoaded
	}

	if err := v.cfg.Validate(); err != nil {
		return err
	}

	if err := v.validateISPReferences(); err != nil {
		return err
	}

	if err := v.validateZoneReferences(); err != nil {
		return err
	}

	if err := v.validateInfraServiceReferences(); err != nil {
		return err
	}

	if err := v.validateServerReferences(); err != nil {
		return err
	}

	if err := v.validateServiceReferences(); err != nil {
		return err
	}

	if err := v.validateInfraSecretReferences(); err != nil {
		return err
	}

	if err := v.validateDomainReferences(); err != nil {
		return err
	}

	if err := v.validateDNSReferences(); err != nil {
		return err
	}

	if err := v.validatePortConflicts(); err != nil {
		return err
	}

	if err := v.validateDomainConflicts(); err != nil {
		return err
	}

	if err := v.validateHostnameConflicts(); err != nil {
		return err
	}

	if err := v.validateEasyTierMesh(); err != nil {
		return err
	}

	return nil
}

func (v *Validator) validateISPReferences() error {
	for _, isp := range v.cfg.ISPs {
		for _, ref := range isp.Credentials {
			if ref.Secret() != "" {
				if _, ok := v.secrets[ref.Secret()]; !ok {
					return fmt.Errorf("%w: secret '%s' referenced by isp '%s' does not exist", domain.ErrMissingReference, ref.Secret(), isp.Name)
				}
			}
		}
	}
	return nil
}

func (v *Validator) validateZoneReferences() error {
	for _, zone := range v.cfg.Zones {
		if zone.ISP != "" {
			if _, ok := v.isps[zone.ISP]; !ok {
				return fmt.Errorf("%w: isp '%s' referenced by zone '%s' does not exist", domain.ErrMissingReference, zone.ISP, zone.Name)
			}
		}
	}
	return nil
}

func (v *Validator) validateInfraServiceReferences() error {
	for _, infra := range v.cfg.InfraServices {
		if _, ok := v.servers[infra.Server]; !ok {
			return fmt.Errorf("%w: server '%s' referenced by infra_service '%s' does not exist", domain.ErrMissingReference, infra.Server, infra.Name)
		}
	}
	return nil
}

func (v *Validator) validateServerReferences() error {
	for _, server := range v.cfg.Servers {
		if _, ok := v.zones[server.Zone]; !ok {
			return fmt.Errorf("%w: zone '%s' referenced by server '%s' does not exist", domain.ErrMissingReference, server.Zone, server.Name)
		}
		if server.ISP != "" {
			if _, ok := v.isps[server.ISP]; !ok {
				return fmt.Errorf("%w: isp '%s' referenced by server '%s' does not exist", domain.ErrMissingReference, server.ISP, server.Name)
			}
		}
		if server.SSH.Password.Secret() != "" {
			if _, ok := v.secrets[server.SSH.Password.Secret()]; !ok {
				return fmt.Errorf("%w: secret '%s' referenced by server '%s' ssh password does not exist", domain.ErrMissingReference, server.SSH.Password.Secret(), server.Name)
			}
		}
	}
	return nil
}

func (v *Validator) validateServiceReferences() error {
	for _, service := range v.cfg.Services {
		if _, ok := v.servers[service.Server]; !ok {
			return fmt.Errorf("%w: server '%s' referenced by service '%s' does not exist", domain.ErrMissingReference, service.Server, service.Name)
		}
		for _, secretName := range service.Secrets {
			if _, ok := v.secrets[secretName]; !ok {
				return fmt.Errorf("%w: secret '%s' referenced by service '%s' does not exist", domain.ErrMissingReference, secretName, service.Name)
			}
		}
	}
	return nil
}

// validateInfraSecretReferences 校验 uptime-kuma 的 admin.password / db.password SecretRef 必须存在。
func (v *Validator) validateInfraSecretReferences() error {
	for _, infra := range v.cfg.InfraServices {
		if infra.Type != entity.InfraServiceTypeUptimeKuma || infra.UptimeKumaConfig == nil {
			continue
		}
		refs := []*valueobject.SecretRef{
			&infra.UptimeKumaConfig.Admin.Password,
			&infra.UptimeKumaConfig.DB.Password,
		}
		for _, ref := range refs {
			if ref.Secret() == "" {
				continue
			}
			if _, ok := v.secrets[ref.Secret()]; !ok {
				return fmt.Errorf("%w: secret '%s' referenced by infra_service '%s' does not exist", domain.ErrMissingReference, ref.Secret(), infra.Name)
			}
		}
	}
	return nil
}

func (v *Validator) validateDomainReferences() error {
	for _, d := range v.cfg.Domains {
		if d.ISP != "" {
			if _, ok := v.isps[d.ISP]; !ok {
				return fmt.Errorf("%w: isp '%s' referenced by domain '%s' does not exist", domain.ErrMissingReference, d.ISP, d.Name)
			}
		}
		if d.DNSISP != "" {
			if _, ok := v.isps[d.DNSISP]; !ok {
				return fmt.Errorf("%w: dns_isp '%s' referenced by domain '%s' does not exist", domain.ErrMissingReference, d.DNSISP, d.Name)
			}
		}
		if d.Parent != "" {
			if _, ok := v.domainMap[d.Parent]; !ok {
				return fmt.Errorf("%w: parent domain '%s' referenced by domain '%s' does not exist", domain.ErrMissingReference, d.Parent, d.Name)
			}
		}
	}
	return nil
}

func (v *Validator) validateDNSReferences() error {
	for _, record := range v.cfg.GetAllDNSRecords() {
		if _, ok := v.domainMap[record.Domain]; !ok {
			return fmt.Errorf("%w: domain '%s' referenced by dns record does not exist", domain.ErrMissingReference, record.Domain)
		}
	}
	return nil
}

func (v *Validator) validatePortConflicts() error {
	serverPorts := make(map[string]map[int]string)

	for _, infra := range v.cfg.InfraServices {
		key := infra.Server
		if serverPorts[key] == nil {
			serverPorts[key] = make(map[int]string)
		}
		if infra.GatewayPorts != nil {
			if infra.GatewayPorts.HTTP > 0 {
				if existing, ok := serverPorts[key][infra.GatewayPorts.HTTP]; ok {
					return fmt.Errorf("%w: gateway http port %d on server '%s' is used by both '%s' and '%s'", domain.ErrPortConflict, infra.GatewayPorts.HTTP, infra.Server, existing, infra.Name)
				}
				serverPorts[key][infra.GatewayPorts.HTTP] = infra.Name
			}
			if infra.GatewayPorts.HTTPS > 0 {
				if existing, ok := serverPorts[key][infra.GatewayPorts.HTTPS]; ok {
					return fmt.Errorf("%w: gateway https port %d on server '%s' is used by both '%s' and '%s'", domain.ErrPortConflict, infra.GatewayPorts.HTTPS, infra.Server, existing, infra.Name)
				}
				serverPorts[key][infra.GatewayPorts.HTTPS] = infra.Name
			}
		}
		if infra.Type == entity.InfraServiceTypeTunnelServer && infra.TunnelServerConfig != nil {
			bindPort := infra.TunnelServerConfig.Ports.BindPort
			if bindPort == 0 {
				bindPort = 30070
			}
			if bindPort > 0 {
				if existing, ok := serverPorts[key][bindPort]; ok {
					return fmt.Errorf("%w: tunnel-server bind_port %d on server '%s' is used by both '%s' and '%s'", domain.ErrPortConflict, bindPort, infra.Server, existing, infra.Name)
				}
				serverPorts[key][bindPort] = infra.Name
			}
		}
		if infra.Type == entity.InfraServiceTypeEasyTier && infra.EasyTierConfig != nil {
			// host 网络模式下 easytier 监听器直接绑定宿主端口，纳入冲突检测（RPC 127.0.0.1 回环端口除外）
			for _, listener := range infra.EasyTierConfig.Listeners {
				port := easyTierListenerPort(listener)
				if port <= 0 {
					continue
				}
				if existing, ok := serverPorts[key][port]; ok {
					return fmt.Errorf("%w: easytier listener port %d on server '%s' is used by both '%s' and '%s'", domain.ErrPortConflict, port, infra.Server, existing, infra.Name)
				}
				serverPorts[key][port] = infra.Name
			}
		}
		for _, port := range infra.Ports {
			if existing, ok := serverPorts[key][port.Host]; ok {
				return fmt.Errorf("%w: host port %d on server '%s' is used by both '%s' and '%s'", domain.ErrPortConflict, port.Host, infra.Server, existing, infra.Name)
			}
			serverPorts[key][port.Host] = infra.Name
		}
	}

	for _, service := range v.cfg.Services {
		key := service.Server
		if serverPorts[key] == nil {
			serverPorts[key] = make(map[int]string)
		}
		for _, port := range service.Ports {
			if existing, ok := serverPorts[key][port.Host]; ok {
				return fmt.Errorf("%w: host port %d on server '%s' is used by both '%s' and '%s'", domain.ErrPortConflict, port.Host, service.Server, existing, service.Name)
			}
			serverPorts[key][port.Host] = service.Name
		}
	}

	return nil
}

func (v *Validator) validateDomainConflicts() error {
	domainNames := make(map[string]string)
	for _, d := range v.cfg.Domains {
		if existing, ok := domainNames[d.Name]; ok {
			return fmt.Errorf("%w: domain '%s' is defined multiple times (first: '%s')", domain.ErrDomainConflict, d.Name, existing)
		}
		domainNames[d.Name] = d.Name
	}

	dnsKeys := make(map[string]bool)
	for _, record := range v.cfg.GetAllDNSRecords() {
		key := fmt.Sprintf("%s:%s:%s:%s", record.Domain, record.Type, record.Name, record.Value)
		if dnsKeys[key] {
			return fmt.Errorf("%w: dns record '%s' is defined multiple times (type: %s, name: %s, value: %s)", domain.ErrDNSSubdomainConflict, record.Domain, record.Type, record.Name, record.Value)
		}
		dnsKeys[key] = true
	}

	return nil
}

func (v *Validator) validateHostnameConflicts() error {
	// 业务服务 hostname 以 server:hostname 为冲突键：同一 hostname 可部署在多个服务器
	// （多区域/智能 DNS 分流），但同一服务器上不能重复占用（gate 配置会生成重复 host 条目）。
	// 隧道路由 hostname 全局唯一（frps 的 custom_domains 匹配依赖全局唯一），并与业务 hostname 互斥。
	serverScoped := make(map[string]string)
	allBizHosts := make(map[string]string)
	tunnelHosts := make(map[string]string)
	for _, service := range v.cfg.Services {
		for _, route := range service.Gateways {
			if route.HasGateway() && route.Hostname != "" {
				key := fmt.Sprintf("%s:%s", service.Server, route.Hostname)
				if existing, ok := serverScoped[key]; ok {
					return fmt.Errorf("%w: hostname '%s' is used by both services '%s' and '%s' on server '%s'", domain.ErrHostnameConflict, route.Hostname, existing, service.Name, service.Server)
				}
				serverScoped[key] = service.Name
				if existing, ok := tunnelHosts[route.Hostname]; ok {
					return fmt.Errorf("%w: hostname '%s' is used by both service '%s' and tunnel '%s'", domain.ErrHostnameConflict, route.Hostname, service.Name, existing)
				}
				allBizHosts[route.Hostname] = service.Name
			}
		}
	}
	for _, infra := range v.cfg.InfraServices {
		if infra.Type == entity.InfraServiceTypeUptimeKuma && infra.UptimeKumaConfig != nil {
			for _, route := range infra.UptimeKumaConfig.Gateways {
				if route.Hostname == "" {
					continue
				}
				// 仿业务服务 hostname 规则：按 server 作用域（可跨服务器重复，同服务器唯一），
				// 且与隧道路由全局互斥。
				key := fmt.Sprintf("%s:%s", infra.Server, route.Hostname)
				if existing, ok := serverScoped[key]; ok {
					return fmt.Errorf("%w: hostname '%s' is used by both services '%s' and '%s' on server '%s'", domain.ErrHostnameConflict, route.Hostname, existing, infra.Name, infra.Server)
				}
				serverScoped[key] = infra.Name
				if existing, ok := tunnelHosts[route.Hostname]; ok {
					return fmt.Errorf("%w: hostname '%s' is used by both service '%s' and tunnel '%s'", domain.ErrHostnameConflict, route.Hostname, infra.Name, existing)
				}
				allBizHosts[route.Hostname] = infra.Name
			}
		}
		if infra.Type != entity.InfraServiceTypeTunnelServer || infra.TunnelServerConfig == nil {
			continue
		}
		for _, route := range infra.TunnelServerConfig.TunnelRoutes {
			if route.Hostname == "" {
				continue
			}
			if existing, ok := tunnelHosts[route.Hostname]; ok {
				return fmt.Errorf("%w: hostname '%s' is used by both tunnels '%s' and '%s'", domain.ErrHostnameConflict, route.Hostname, existing, infra.Name)
			}
			if existing, ok := allBizHosts[route.Hostname]; ok {
				return fmt.Errorf("%w: hostname '%s' is used by both service '%s' and tunnel '%s'", domain.ErrHostnameConflict, route.Hostname, existing, infra.Name)
			}
			tunnelHosts[route.Hostname] = infra.Name
		}
	}
	return nil
}

// easyTierListenerPort 解析 easytier listener URI（scheme://ip:port）的端口，非法返回 0
func easyTierListenerPort(listener string) int {
	u, err := url.Parse(listener)
	if err != nil {
		return 0
	}
	p, err := strconv.Atoi(u.Port())
	if err != nil || p <= 0 {
		return 0
	}
	return p
}

// validateEasyTierMesh 校验同一环境内 easytier 节点的 mesh 一致性：
// virtual_ip 唯一、network_name 一致、network_secret 解析后一致。
func (v *Validator) validateEasyTierMesh() error {
	ips := make(map[string]string)
	networkName := ""
	var networkSecret string
	haveSecret := false

	for _, infra := range v.cfg.InfraServices {
		if infra.Type != entity.InfraServiceTypeEasyTier || infra.EasyTierConfig == nil {
			continue
		}
		et := infra.EasyTierConfig

		if existing, ok := ips[et.VirtualIP]; ok {
			return fmt.Errorf("%w: easytier virtual_ip '%s' is used by both '%s' and '%s'", domain.ErrIPConflict, et.VirtualIP, existing, infra.Name)
		}
		ips[et.VirtualIP] = infra.Name

		if networkName == "" {
			networkName = et.NetworkName
		} else if et.NetworkName != networkName {
			return fmt.Errorf("%w: easytier network_name '%s' on '%s' differs from mesh network_name '%s'", domain.ErrNetworkConflict, et.NetworkName, infra.Name, networkName)
		}

		secret, err := et.NetworkSecret.Resolve(v.secrets)
		if err != nil {
			return fmt.Errorf("easytier '%s' network_secret: %w", infra.Name, err)
		}
		if haveSecret {
			if secret != networkSecret {
				return fmt.Errorf("%w: easytier network_secret on '%s' differs from mesh network_secret", domain.ErrNetworkConflict, infra.Name)
			}
		} else {
			networkSecret = secret
			haveSecret = true
		}
	}
	return nil
}
