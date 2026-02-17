package plan

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/litelake/yamlops/internal/compose"
	"github.com/litelake/yamlops/internal/entities"
	"github.com/litelake/yamlops/internal/gate"
	"gopkg.in/yaml.v3"
)

type DeploymentState struct {
	Services   map[string]*entities.Service
	Gateways   map[string]*entities.Gateway
	Servers    map[string]*entities.Server
	Zones      map[string]*entities.Zone
	Domains    map[string]*entities.Domain
	Records    map[string]*entities.DNSRecord
	Certs      map[string]*entities.Certificate
	Registries map[string]*entities.Registry
	ISPs       map[string]*entities.ISP
}

type Planner struct {
	config     *entities.Config
	state      *DeploymentState
	composeGen *compose.Generator
	gateGen    *gate.Generator
	outputDir  string
	env        string
}

func NewPlanner(cfg *entities.Config, env string) *Planner {
	if env == "" {
		env = "dev"
	}
	state := &DeploymentState{
		Services:   make(map[string]*entities.Service),
		Gateways:   make(map[string]*entities.Gateway),
		Servers:    make(map[string]*entities.Server),
		Zones:      make(map[string]*entities.Zone),
		Domains:    make(map[string]*entities.Domain),
		Records:    make(map[string]*entities.DNSRecord),
		Certs:      make(map[string]*entities.Certificate),
		Registries: make(map[string]*entities.Registry),
		ISPs:       make(map[string]*entities.ISP),
	}
	return &Planner{
		config:     cfg,
		state:      state,
		composeGen: compose.NewGenerator(),
		gateGen:    gate.NewGenerator(),
		outputDir:  "deployments",
		env:        env,
	}
}

func NewPlannerWithState(cfg *entities.Config, state *DeploymentState, env string) *Planner {
	if env == "" {
		env = "dev"
	}
	if state == nil {
		state = &DeploymentState{
			Services:   make(map[string]*entities.Service),
			Gateways:   make(map[string]*entities.Gateway),
			Servers:    make(map[string]*entities.Server),
			Zones:      make(map[string]*entities.Zone),
			Domains:    make(map[string]*entities.Domain),
			Records:    make(map[string]*entities.DNSRecord),
			Certs:      make(map[string]*entities.Certificate),
			Registries: make(map[string]*entities.Registry),
			ISPs:       make(map[string]*entities.ISP),
		}
	}
	return &Planner{
		config:     cfg,
		state:      state,
		composeGen: compose.NewGenerator(),
		gateGen:    gate.NewGenerator(),
		outputDir:  "deployments",
		env:        env,
	}
}

func (p *Planner) SetOutputDir(dir string) {
	p.outputDir = dir
}

func (p *Planner) Plan(scope *Scope) (*Plan, error) {
	if scope == nil {
		scope = &Scope{}
	}

	plan := NewPlanWithScope(scope)

	p.planISPs(plan, scope)
	p.planZones(plan, scope)
	p.planDomains(plan, scope)
	p.planRecords(plan, scope)
	p.planCertificates(plan, scope)
	p.planRegistries(plan, scope)
	p.planServers(plan, scope)
	p.planGateways(plan, scope)
	p.planServices(plan, scope)

	return plan, nil
}

func (p *Planner) planISPs(plan *Plan, scope *Scope) {
	cfgMap := p.config.GetISPMap()

	for name, state := range p.state.ISPs {
		if _, exists := cfgMap[name]; !exists {
			if scope.Matches("", "", "", "") {
				plan.AddChange(&Change{
					Type:     ChangeTypeDelete,
					Entity:   "isp",
					Name:     name,
					OldState: state,
					NewState: nil,
					Actions:  []string{fmt.Sprintf("delete isp %s", name)},
				})
			}
		}
	}

	for name, cfg := range cfgMap {
		if state, exists := p.state.ISPs[name]; exists {
			if !p.ispEquals(state, cfg) {
				if scope.Matches("", "", "", "") {
					plan.AddChange(&Change{
						Type:     ChangeTypeUpdate,
						Entity:   "isp",
						Name:     name,
						OldState: state,
						NewState: cfg,
						Actions:  []string{fmt.Sprintf("update isp %s", name)},
					})
				}
			}
		} else {
			if scope.Matches("", "", "", "") {
				plan.AddChange(&Change{
					Type:     ChangeTypeCreate,
					Entity:   "isp",
					Name:     name,
					OldState: nil,
					NewState: cfg,
					Actions:  []string{fmt.Sprintf("create isp %s", name)},
				})
			}
		}
	}
}

func (p *Planner) ispEquals(a, b *entities.ISP) bool {
	if a.Name != b.Name {
		return false
	}
	if len(a.Services) != len(b.Services) {
		return false
	}
	for i, s := range a.Services {
		if i >= len(b.Services) || s != b.Services[i] {
			return false
		}
	}
	return true
}

func (p *Planner) planZones(plan *Plan, scope *Scope) {
	cfgMap := p.config.GetZoneMap()

	for name, state := range p.state.Zones {
		if _, exists := cfgMap[name]; !exists {
			if scope.Matches(name, "", "", "") {
				plan.AddChange(&Change{
					Type:     ChangeTypeDelete,
					Entity:   "zone",
					Name:     name,
					OldState: state,
					NewState: nil,
					Actions:  []string{fmt.Sprintf("delete zone %s", name)},
				})
			}
		}
	}

	for name, cfg := range cfgMap {
		if state, exists := p.state.Zones[name]; exists {
			if !p.zoneEquals(state, cfg) {
				if scope.Matches(name, "", "", "") {
					plan.AddChange(&Change{
						Type:     ChangeTypeUpdate,
						Entity:   "zone",
						Name:     name,
						OldState: state,
						NewState: cfg,
						Actions:  []string{fmt.Sprintf("update zone %s", name)},
					})
				}
			}
		} else {
			if scope.Matches(name, "", "", "") {
				plan.AddChange(&Change{
					Type:     ChangeTypeCreate,
					Entity:   "zone",
					Name:     name,
					OldState: nil,
					NewState: cfg,
					Actions:  []string{fmt.Sprintf("create zone %s", name)},
				})
			}
		}
	}
}

func (p *Planner) zoneEquals(a, b *entities.Zone) bool {
	return a.Name == b.Name && a.ISP == b.ISP && a.Region == b.Region
}

func (p *Planner) planDomains(plan *Plan, scope *Scope) {
	cfgMap := p.config.GetDomainMap()

	for name, state := range p.state.Domains {
		if _, exists := cfgMap[name]; !exists {
			if scope.Matches("", "", "", name) {
				plan.AddChange(&Change{
					Type:     ChangeTypeDelete,
					Entity:   "domain",
					Name:     name,
					OldState: state,
					NewState: nil,
					Actions:  []string{fmt.Sprintf("delete domain %s", name)},
				})
			}
		}
	}

	for name, cfg := range cfgMap {
		if state, exists := p.state.Domains[name]; exists {
			if !p.domainEquals(state, cfg) {
				if scope.Matches("", "", "", name) {
					plan.AddChange(&Change{
						Type:     ChangeTypeUpdate,
						Entity:   "domain",
						Name:     name,
						OldState: state,
						NewState: cfg,
						Actions:  []string{fmt.Sprintf("update domain %s", name)},
					})
				}
			}
		} else {
			if scope.Matches("", "", "", name) {
				plan.AddChange(&Change{
					Type:     ChangeTypeCreate,
					Entity:   "domain",
					Name:     name,
					OldState: nil,
					NewState: cfg,
					Actions:  []string{fmt.Sprintf("create domain %s", name)},
				})
			}
		}
	}
}

func (p *Planner) domainEquals(a, b *entities.Domain) bool {
	return a.Name == b.Name && a.ISP == b.ISP && a.Parent == b.Parent && a.AutoRenew == b.AutoRenew
}

func (p *Planner) planRecords(plan *Plan, scope *Scope) {
	cfgRecords := p.config.DNSRecords
	recordKey := func(r *entities.DNSRecord) string {
		return fmt.Sprintf("%s:%s:%s", r.Domain, r.Type, r.Name)
	}

	stateMap := make(map[string]*entities.DNSRecord)
	for key, r := range p.state.Records {
		stateMap[key] = r
	}

	cfgMap := make(map[string]*entities.DNSRecord)
	for i := range cfgRecords {
		key := recordKey(&cfgRecords[i])
		cfgMap[key] = &cfgRecords[i]
	}

	for key, state := range stateMap {
		if _, exists := cfgMap[key]; !exists {
			if scope.Matches("", "", "", state.Domain) {
				plan.AddChange(&Change{
					Type:     ChangeTypeDelete,
					Entity:   "dns_record",
					Name:     key,
					OldState: state,
					NewState: nil,
					Actions:  []string{fmt.Sprintf("delete dns record %s", key)},
				})
			}
		}
	}

	for key, cfg := range cfgMap {
		if state, exists := stateMap[key]; exists {
			if !p.recordEquals(state, cfg) {
				if scope.Matches("", "", "", cfg.Domain) {
					plan.AddChange(&Change{
						Type:     ChangeTypeUpdate,
						Entity:   "dns_record",
						Name:     key,
						OldState: state,
						NewState: cfg,
						Actions:  []string{fmt.Sprintf("update dns record %s", key)},
					})
				}
			}
		} else {
			if scope.Matches("", "", "", cfg.Domain) {
				plan.AddChange(&Change{
					Type:     ChangeTypeCreate,
					Entity:   "dns_record",
					Name:     key,
					OldState: nil,
					NewState: cfg,
					Actions:  []string{fmt.Sprintf("create dns record %s", key)},
				})
			}
		}
	}
}

func (p *Planner) recordEquals(a, b *entities.DNSRecord) bool {
	return a.Domain == b.Domain && a.Type == b.Type && a.Name == b.Name && a.Value == b.Value && a.TTL == b.TTL
}

func (p *Planner) planCertificates(plan *Plan, scope *Scope) {
	cfgMap := p.config.GetCertificateMap()

	for name, state := range p.state.Certs {
		if _, exists := cfgMap[name]; !exists {
			if scope.Matches("", "", "", "") {
				plan.AddChange(&Change{
					Type:     ChangeTypeDelete,
					Entity:   "certificate",
					Name:     name,
					OldState: state,
					NewState: nil,
					Actions:  []string{fmt.Sprintf("delete certificate %s", name)},
				})
			}
		}
	}

	for name, cfg := range cfgMap {
		if state, exists := p.state.Certs[name]; exists {
			if !p.certificateEquals(state, cfg) {
				if scope.Matches("", "", "", "") {
					plan.AddChange(&Change{
						Type:     ChangeTypeUpdate,
						Entity:   "certificate",
						Name:     name,
						OldState: state,
						NewState: cfg,
						Actions:  []string{fmt.Sprintf("update certificate %s", name)},
					})
				}
			}
		} else {
			if scope.Matches("", "", "", "") {
				plan.AddChange(&Change{
					Type:     ChangeTypeCreate,
					Entity:   "certificate",
					Name:     name,
					OldState: nil,
					NewState: cfg,
					Actions:  []string{fmt.Sprintf("create certificate %s", name)},
				})
			}
		}
	}
}

func (p *Planner) certificateEquals(a, b *entities.Certificate) bool {
	if a.Name != b.Name || a.Provider != b.Provider || a.DNSProvider != b.DNSProvider {
		return false
	}
	if len(a.Domains) != len(b.Domains) {
		return false
	}
	for i, d := range a.Domains {
		if i >= len(b.Domains) || d != b.Domains[i] {
			return false
		}
	}
	return true
}

func (p *Planner) planRegistries(plan *Plan, scope *Scope) {
	cfgMap := p.config.GetRegistryMap()

	for name, state := range p.state.Registries {
		if _, exists := cfgMap[name]; !exists {
			if scope.Matches("", "", "", "") {
				plan.AddChange(&Change{
					Type:     ChangeTypeDelete,
					Entity:   "registry",
					Name:     name,
					OldState: state,
					NewState: nil,
					Actions:  []string{fmt.Sprintf("delete registry %s", name)},
				})
			}
		}
	}

	for name, cfg := range cfgMap {
		if state, exists := p.state.Registries[name]; exists {
			if !p.registryEquals(state, cfg) {
				if scope.Matches("", "", "", "") {
					plan.AddChange(&Change{
						Type:     ChangeTypeUpdate,
						Entity:   "registry",
						Name:     name,
						OldState: state,
						NewState: cfg,
						Actions:  []string{fmt.Sprintf("update registry %s", name)},
					})
				}
			}
		} else {
			if scope.Matches("", "", "", "") {
				plan.AddChange(&Change{
					Type:     ChangeTypeCreate,
					Entity:   "registry",
					Name:     name,
					OldState: nil,
					NewState: cfg,
					Actions:  []string{fmt.Sprintf("create registry %s", name)},
				})
			}
		}
	}
}

func (p *Planner) registryEquals(a, b *entities.Registry) bool {
	return a.Name == b.Name && a.URL == b.URL
}

func (p *Planner) planServers(plan *Plan, scope *Scope) {
	cfgMap := p.config.GetServerMap()
	zoneMap := p.config.GetZoneMap()

	for name, state := range p.state.Servers {
		if _, exists := cfgMap[name]; !exists {
			zoneName := ""
			if state.Zone != "" {
				zoneName = state.Zone
			}
			if scope.Matches(zoneName, name, "", "") {
				plan.AddChange(&Change{
					Type:     ChangeTypeDelete,
					Entity:   "server",
					Name:     name,
					OldState: state,
					NewState: nil,
					Actions:  []string{fmt.Sprintf("delete server %s", name)},
				})
			}
		}
	}

	for name, cfg := range cfgMap {
		zoneName := ""
		if cfg.Zone != "" {
			if z, ok := zoneMap[cfg.Zone]; ok {
				zoneName = z.Name
			}
		}
		if state, exists := p.state.Servers[name]; exists {
			if !p.serverEquals(state, cfg) {
				if scope.Matches(zoneName, name, "", "") {
					plan.AddChange(&Change{
						Type:     ChangeTypeUpdate,
						Entity:   "server",
						Name:     name,
						OldState: state,
						NewState: cfg,
						Actions:  []string{fmt.Sprintf("update server %s", name)},
					})
				}
			}
		} else {
			if scope.Matches(zoneName, name, "", "") {
				plan.AddChange(&Change{
					Type:     ChangeTypeCreate,
					Entity:   "server",
					Name:     name,
					OldState: nil,
					NewState: cfg,
					Actions:  []string{fmt.Sprintf("create server %s", name)},
				})
			}
		}
	}
}

func (p *Planner) serverEquals(a, b *entities.Server) bool {
	return a.Name == b.Name && a.Zone == b.Zone && a.ISP == b.ISP &&
		a.IP.Public == b.IP.Public && a.IP.Private == b.IP.Private
}

func (p *Planner) planGateways(plan *Plan, scope *Scope) {
	cfgMap := p.config.GetGatewayMap()
	serverMap := p.config.GetServerMap()

	for name, state := range p.state.Gateways {
		if _, exists := cfgMap[name]; !exists {
			serverName := state.Server
			zoneName := ""
			if s, ok := serverMap[serverName]; ok {
				zoneName = s.Zone
			}
			if scope.Matches(zoneName, serverName, "", "") {
				plan.AddChange(&Change{
					Type:     ChangeTypeDelete,
					Entity:   "gateway",
					Name:     name,
					OldState: state,
					NewState: nil,
					Actions:  []string{fmt.Sprintf("delete gateway %s", name)},
				})
			}
		}
	}

	for name, cfg := range cfgMap {
		serverName := cfg.Server
		zoneName := ""
		if s, ok := serverMap[serverName]; ok {
			zoneName = s.Zone
		}
		if state, exists := p.state.Gateways[name]; exists {
			if !p.gatewayEquals(state, cfg) {
				if scope.Matches(zoneName, serverName, "", "") {
					plan.AddChange(&Change{
						Type:     ChangeTypeUpdate,
						Entity:   "gateway",
						Name:     name,
						OldState: state,
						NewState: cfg,
						Actions:  []string{fmt.Sprintf("update gateway %s", name)},
					})
				}
			}
		} else {
			if scope.Matches(zoneName, serverName, "", "") {
				plan.AddChange(&Change{
					Type:     ChangeTypeCreate,
					Entity:   "gateway",
					Name:     name,
					OldState: nil,
					NewState: cfg,
					Actions:  []string{fmt.Sprintf("create gateway %s", name)},
				})
			}
		}
	}
}

func (p *Planner) gatewayEquals(a, b *entities.Gateway) bool {
	return a.Name == b.Name && a.Zone == b.Zone && a.Server == b.Server &&
		a.Image == b.Image && a.Ports.HTTP == b.Ports.HTTP && a.Ports.HTTPS == b.Ports.HTTPS
}

func (p *Planner) planServices(plan *Plan, scope *Scope) {
	cfgMap := p.config.GetServiceMap()
	serverMap := p.config.GetServerMap()

	for name, state := range p.state.Services {
		if _, exists := cfgMap[name]; !exists {
			serverName := state.Server
			zoneName := ""
			if s, ok := serverMap[serverName]; ok {
				zoneName = s.Zone
			}
			if scope.Matches(zoneName, serverName, name, "") {
				plan.AddChange(&Change{
					Type:     ChangeTypeDelete,
					Entity:   "service",
					Name:     name,
					OldState: state,
					NewState: nil,
					Actions:  []string{fmt.Sprintf("delete service %s", name)},
				})
			}
		}
	}

	for name, cfg := range cfgMap {
		serverName := cfg.Server
		zoneName := ""
		if s, ok := serverMap[serverName]; ok {
			zoneName = s.Zone
		}
		if state, exists := p.state.Services[name]; exists {
			if !p.serviceEquals(state, cfg) {
				if scope.Matches(zoneName, serverName, name, "") {
					plan.AddChange(&Change{
						Type:     ChangeTypeUpdate,
						Entity:   "service",
						Name:     name,
						OldState: state,
						NewState: cfg,
						Actions:  []string{fmt.Sprintf("update service %s", name)},
					})
				}
			}
		} else {
			if scope.Matches(zoneName, serverName, name, "") {
				plan.AddChange(&Change{
					Type:     ChangeTypeCreate,
					Entity:   "service",
					Name:     name,
					OldState: nil,
					NewState: cfg,
					Actions:  []string{fmt.Sprintf("create service %s", name)},
				})
			}
		}
	}
}

func (p *Planner) serviceEquals(a, b *entities.Service) bool {
	if a.Name != b.Name || a.Server != b.Server || a.Image != b.Image || a.Port != b.Port {
		return false
	}
	if len(a.Env) != len(b.Env) {
		return false
	}
	for k, v := range a.Env {
		if bv, ok := b.Env[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

func (p *Planner) GenerateDeployments() error {
	if err := os.MkdirAll(p.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	if err := p.generateServiceComposes(); err != nil {
		return err
	}

	if err := p.generateGatewayConfigs(); err != nil {
		return err
	}

	return nil
}

func (p *Planner) generateServiceComposes() error {
	serverServices := make(map[string][]*entities.Service)
	for i := range p.config.Services {
		svc := &p.config.Services[i]
		serverServices[svc.Server] = append(serverServices[svc.Server], svc)
	}

	for serverName, services := range serverServices {
		serverDir := filepath.Join(p.outputDir, serverName)
		if err := os.MkdirAll(serverDir, 0755); err != nil {
			return fmt.Errorf("failed to create server directory %s: %w", serverDir, err)
		}

		for _, svc := range services {
			if err := p.generateServiceCompose(serverDir, svc); err != nil {
				return err
			}
		}
	}

	return nil
}

func (p *Planner) generateServiceCompose(serverDir string, svc *entities.Service) error {
	ports := []string{}
	if svc.Port > 0 {
		ports = append(ports, fmt.Sprintf("%d:%d", svc.Port, svc.Port))
	}

	volumes := []string{}
	for _, v := range svc.Volumes {
		volumes = append(volumes, fmt.Sprintf("%s:%s", v.Source, v.Target))
	}

	var healthCheck *compose.HealthCheck
	if svc.Healthcheck != nil {
		healthCheck = &compose.HealthCheck{
			Test:     []string{"CMD", "curl", "-f", svc.Healthcheck.Path},
			Interval: svc.Healthcheck.Interval,
			Timeout:  svc.Healthcheck.Timeout,
			Retries:  3,
		}
	}

	var resources *compose.Resources
	if svc.Resources.CPU != "" || svc.Resources.Memory != "" {
		resources = &compose.Resources{
			Limits: &compose.ResourceLimits{
				Cpus:   svc.Resources.CPU,
				Memory: svc.Resources.Memory,
			},
		}
	}

	secrets := p.config.GetSecretsMap()
	envMap := make(map[string]string)
	for k, ref := range svc.Env {
		val, err := ref.Resolve(secrets)
		if err != nil {
			return fmt.Errorf("failed to resolve env %s for service %s: %w", k, svc.Name, err)
		}
		envMap[k] = val
	}
	for _, secretName := range svc.Secrets {
		if val, ok := secrets[secretName]; ok {
			envKey := strings.ToUpper(secretName)
			envMap[envKey] = val
		}
	}

	composeSvc := &compose.ComposeService{
		Name:        svc.Name,
		Image:       svc.Image,
		Ports:       ports,
		Environment: envMap,
		Volumes:     volumes,
		HealthCheck: healthCheck,
		Resources:   resources,
		Internal:    svc.Internal,
	}

	content, err := p.composeGen.Generate(composeSvc, p.env)
	if err != nil {
		return fmt.Errorf("failed to generate compose for service %s: %w", svc.Name, err)
	}

	composeFile := filepath.Join(serverDir, fmt.Sprintf("%s.compose.yaml", svc.Name))
	if err := os.WriteFile(composeFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write compose file %s: %w", composeFile, err)
	}

	return nil
}

func (p *Planner) generateGatewayConfigs() error {
	gatewayServers := make(map[string][]*entities.Gateway)
	for i := range p.config.Gateways {
		gw := &p.config.Gateways[i]
		gatewayServers[gw.Server] = append(gatewayServers[gw.Server], gw)
	}

	for serverName, gateways := range gatewayServers {
		serverDir := filepath.Join(p.outputDir, serverName)
		if err := os.MkdirAll(serverDir, 0755); err != nil {
			return fmt.Errorf("failed to create server directory %s: %w", serverDir, err)
		}

		for _, gw := range gateways {
			if err := p.generateGatewayConfig(serverDir, gw); err != nil {
				return err
			}
		}
	}

	return nil
}

func (p *Planner) generateGatewayConfig(serverDir string, gw *entities.Gateway) error {
	serverMap := p.config.GetServerMap()
	var hosts []gate.HostRoute
	for _, svc := range p.config.Services {
		if svc.Server == gw.Server && svc.Gateway.Enabled {
			var backendIP string
			if server, ok := serverMap[svc.Server]; ok && server.IP.Private != "" {
				backendIP = server.IP.Private
			} else {
				backendIP = "127.0.0.1"
			}
			backend := fmt.Sprintf("http://%s:%d", backendIP, svc.Port)
			hostname := svc.Gateway.Hostname
			if hostname == "" {
				hostname = svc.Name
			}

			healthPath := "/health"
			if svc.Healthcheck != nil {
				healthPath = svc.Healthcheck.Path
			}

			sslPort := 0
			if svc.Gateway.SSL {
				sslPort = gw.Ports.HTTPS
			}

			healthInterval := "30s"
			healthTimeout := "10s"
			if svc.Healthcheck != nil {
				if svc.Healthcheck.Interval != "" {
					healthInterval = svc.Healthcheck.Interval
				}
				if svc.Healthcheck.Timeout != "" {
					healthTimeout = svc.Healthcheck.Timeout
				}
			}

			hosts = append(hosts, gate.HostRoute{
				Name:                hostname,
				Port:                gw.Ports.HTTP,
				SSLPort:             sslPort,
				Backend:             []string{backend},
				HealthCheck:         healthPath,
				HealthCheckInterval: healthInterval,
				HealthCheckTimeout:  healthTimeout,
			})
		}
	}

	gatewayConfig := &gate.GatewayConfig{
		Port:               gw.Ports.HTTP,
		LogLevel:           gw.LogLevel,
		WAFEnabled:         gw.WAF.Enabled,
		Whitelist:          gw.WAF.Whitelist,
		SSLMode:            gw.SSL.Mode,
		SSLEndpoint:        gw.SSL.Endpoint,
		SSLAutoUpdate:      true,
		SSLUpdateCheckTime: "00:00-00:59",
	}

	content, err := p.gateGen.Generate(gatewayConfig, hosts)
	if err != nil {
		return fmt.Errorf("failed to generate gateway config for %s: %w", gw.Name, err)
	}

	configFile := filepath.Join(serverDir, fmt.Sprintf("%s.gate.yaml", gw.Name))
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write gateway config file %s: %w", configFile, err)
	}

	return nil
}

func (p *Planner) GetConfig() *entities.Config {
	return p.config
}

func (p *Planner) GetState() *DeploymentState {
	return p.state
}

func (p *Planner) SetState(state *DeploymentState) {
	p.state = state
}

func (p *Planner) LoadStateFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read state file: %w", err)
	}

	var cfg entities.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse state file: %w", err)
	}

	p.state = &DeploymentState{
		Services:   make(map[string]*entities.Service),
		Gateways:   make(map[string]*entities.Gateway),
		Servers:    make(map[string]*entities.Server),
		Zones:      make(map[string]*entities.Zone),
		Domains:    make(map[string]*entities.Domain),
		Records:    make(map[string]*entities.DNSRecord),
		Certs:      make(map[string]*entities.Certificate),
		Registries: make(map[string]*entities.Registry),
		ISPs:       make(map[string]*entities.ISP),
	}

	for i := range cfg.Services {
		p.state.Services[cfg.Services[i].Name] = &cfg.Services[i]
	}
	for i := range cfg.Gateways {
		p.state.Gateways[cfg.Gateways[i].Name] = &cfg.Gateways[i]
	}
	for i := range cfg.Servers {
		p.state.Servers[cfg.Servers[i].Name] = &cfg.Servers[i]
	}
	for i := range cfg.Zones {
		p.state.Zones[cfg.Zones[i].Name] = &cfg.Zones[i]
	}
	for i := range cfg.Domains {
		p.state.Domains[cfg.Domains[i].Name] = &cfg.Domains[i]
	}
	for i := range cfg.DNSRecords {
		key := fmt.Sprintf("%s:%s:%s", cfg.DNSRecords[i].Domain, cfg.DNSRecords[i].Type, cfg.DNSRecords[i].Name)
		p.state.Records[key] = &cfg.DNSRecords[i]
	}
	for i := range cfg.Certificates {
		p.state.Certs[cfg.Certificates[i].Name] = &cfg.Certificates[i]
	}
	for i := range cfg.Registries {
		p.state.Registries[cfg.Registries[i].Name] = &cfg.Registries[i]
	}
	for i := range cfg.ISPs {
		p.state.ISPs[cfg.ISPs[i].Name] = &cfg.ISPs[i]
	}

	return nil
}

func (p *Planner) SaveStateToFile(path string) error {
	cfg := &entities.Config{
		Services:     make([]entities.Service, 0, len(p.state.Services)),
		Gateways:     make([]entities.Gateway, 0, len(p.state.Gateways)),
		Servers:      make([]entities.Server, 0, len(p.state.Servers)),
		Zones:        make([]entities.Zone, 0, len(p.state.Zones)),
		Domains:      make([]entities.Domain, 0, len(p.state.Domains)),
		DNSRecords:   make([]entities.DNSRecord, 0, len(p.state.Records)),
		Certificates: make([]entities.Certificate, 0, len(p.state.Certs)),
		Registries:   make([]entities.Registry, 0, len(p.state.Registries)),
		ISPs:         make([]entities.ISP, 0, len(p.state.ISPs)),
	}

	for _, svc := range p.state.Services {
		cfg.Services = append(cfg.Services, *svc)
	}
	for _, gw := range p.state.Gateways {
		cfg.Gateways = append(cfg.Gateways, *gw)
	}
	for _, srv := range p.state.Servers {
		cfg.Servers = append(cfg.Servers, *srv)
	}
	for _, z := range p.state.Zones {
		cfg.Zones = append(cfg.Zones, *z)
	}
	for _, d := range p.state.Domains {
		cfg.Domains = append(cfg.Domains, *d)
	}
	for _, r := range p.state.Records {
		cfg.DNSRecords = append(cfg.DNSRecords, *r)
	}
	for _, c := range p.state.Certs {
		cfg.Certificates = append(cfg.Certificates, *c)
	}
	for _, r := range p.state.Registries {
		cfg.Registries = append(cfg.Registries, *r)
	}
	for _, isp := range p.state.ISPs {
		cfg.ISPs = append(cfg.ISPs, *isp)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}
