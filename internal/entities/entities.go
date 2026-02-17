package entities

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
)

var (
	ErrInvalidName     = errors.New("invalid name")
	ErrInvalidIP       = errors.New("invalid IP address")
	ErrInvalidPort     = errors.New("invalid port")
	ErrInvalidURL      = errors.New("invalid URL")
	ErrInvalidDomain   = errors.New("invalid domain")
	ErrInvalidTTL      = errors.New("invalid TTL")
	ErrEmptyValue      = errors.New("empty value")
	ErrMissingSecret   = errors.New("missing secret reference")
	ErrInvalidDuration = errors.New("invalid duration")
)

type SecretRef struct {
	Plain  string `yaml:"plain,omitempty"`
	Secret string `yaml:"secret,omitempty"`
}

func (s *SecretRef) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var plain string
	if err := unmarshal(&plain); err == nil {
		s.Plain = plain
		return nil
	}

	type alias SecretRef
	var ref alias
	if err := unmarshal(&ref); err != nil {
		return err
	}
	s.Plain = ref.Plain
	s.Secret = ref.Secret
	return nil
}

func (s *SecretRef) MarshalYAML() (interface{}, error) {
	if s.Secret != "" {
		return map[string]string{"secret": s.Secret}, nil
	}
	return s.Plain, nil
}

func (s *SecretRef) Resolve(secrets map[string]string) (string, error) {
	if s.Secret != "" {
		val, ok := secrets[s.Secret]
		if !ok {
			return "", fmt.Errorf("%w: %s", ErrMissingSecret, s.Secret)
		}
		return val, nil
	}
	return s.Plain, nil
}

func (s *SecretRef) Validate() error {
	if s.Plain == "" && s.Secret == "" {
		return ErrEmptyValue
	}
	return nil
}

type Secret struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

func (s *Secret) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidName)
	}
	return nil
}

type ISPService string

const (
	ISPServiceServer      ISPService = "server"
	ISPServiceDomain      ISPService = "domain"
	ISPServiceDNS         ISPService = "dns"
	ISPServiceCertificate ISPService = "certificate"
)

type ISP struct {
	Name        string               `yaml:"name"`
	Services    []ISPService         `yaml:"services"`
	Credentials map[string]SecretRef `yaml:"credentials"`
}

func (i *ISP) Validate() error {
	if i.Name == "" {
		return fmt.Errorf("%w: isp name is required", ErrInvalidName)
	}
	if len(i.Services) == 0 {
		return errors.New("at least one service is required")
	}
	if len(i.Credentials) == 0 {
		return errors.New("credentials are required")
	}
	for key, ref := range i.Credentials {
		if err := ref.Validate(); err != nil {
			return fmt.Errorf("credential %s: %w", key, err)
		}
	}
	return nil
}

func (i *ISP) HasService(service ISPService) bool {
	for _, s := range i.Services {
		if s == service {
			return true
		}
	}
	return false
}

type Zone struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
	ISP         string `yaml:"isp"`
	Region      string `yaml:"region"`
}

func (z *Zone) Validate() error {
	if z.Name == "" {
		return fmt.Errorf("%w: zone name is required", ErrInvalidName)
	}
	if z.ISP == "" {
		return errors.New("isp is required")
	}
	if z.Region == "" {
		return errors.New("region is required")
	}
	return nil
}

type GatewayPorts struct {
	HTTP  int `yaml:"http"`
	HTTPS int `yaml:"https"`
}

func (p *GatewayPorts) Validate() error {
	if p.HTTP <= 0 || p.HTTP > 65535 {
		return fmt.Errorf("%w: http port must be between 1 and 65535", ErrInvalidPort)
	}
	if p.HTTPS <= 0 || p.HTTPS > 65535 {
		return fmt.Errorf("%w: https port must be between 1 and 65535", ErrInvalidPort)
	}
	return nil
}

type GatewaySSLConfig struct {
	Mode     string `yaml:"mode"`
	Endpoint string `yaml:"endpoint,omitempty"`
}

func (s *GatewaySSLConfig) Validate() error {
	if s.Mode != "local" && s.Mode != "remote" {
		return errors.New("ssl mode must be 'local' or 'remote'")
	}
	if s.Mode == "remote" && s.Endpoint == "" {
		return errors.New("endpoint is required for remote ssl mode")
	}
	return nil
}

type GatewayWAFConfig struct {
	Enabled   bool     `yaml:"enabled"`
	Whitelist []string `yaml:"whitelist,omitempty"`
}

func (w *GatewayWAFConfig) Validate() error {
	for _, cidr := range w.Whitelist {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return fmt.Errorf("invalid CIDR %s: %w", cidr, err)
		}
	}
	return nil
}

type GatewayConfig struct {
	Source string `yaml:"source"`
	Sync   bool   `yaml:"sync"`
}

type Gateway struct {
	Name     string           `yaml:"name"`
	Zone     string           `yaml:"zone"`
	Server   string           `yaml:"server"`
	Image    string           `yaml:"image"`
	Ports    GatewayPorts     `yaml:"ports"`
	Config   GatewayConfig    `yaml:"config"`
	SSL      GatewaySSLConfig `yaml:"ssl"`
	WAF      GatewayWAFConfig `yaml:"waf"`
	LogLevel int              `yaml:"log_level,omitempty"`
}

func (g *Gateway) Validate() error {
	if g.Name == "" {
		return fmt.Errorf("%w: gateway name is required", ErrInvalidName)
	}
	if g.Zone == "" {
		return errors.New("zone is required")
	}
	if g.Server == "" {
		return errors.New("server is required")
	}
	if g.Image == "" {
		return errors.New("image is required")
	}
	if err := g.Ports.Validate(); err != nil {
		return err
	}
	if err := g.SSL.Validate(); err != nil {
		return err
	}
	if err := g.WAF.Validate(); err != nil {
		return err
	}
	return nil
}

type ServerIP struct {
	Public  string `yaml:"public,omitempty"`
	Private string `yaml:"private,omitempty"`
}

func (i *ServerIP) Validate() error {
	if i.Public != "" && net.ParseIP(i.Public) == nil {
		return fmt.Errorf("%w: public IP %s", ErrInvalidIP, i.Public)
	}
	if i.Private != "" && net.ParseIP(i.Private) == nil {
		return fmt.Errorf("%w: private IP %s", ErrInvalidIP, i.Private)
	}
	return nil
}

type ServerSSH struct {
	Host     string    `yaml:"host"`
	Port     int       `yaml:"port"`
	User     string    `yaml:"user"`
	Password SecretRef `yaml:"password"`
}

func (s *ServerSSH) Validate() error {
	if s.Host == "" {
		return errors.New("ssh host is required")
	}
	if s.Port <= 0 || s.Port > 65535 {
		return fmt.Errorf("%w: ssh port must be between 1 and 65535", ErrInvalidPort)
	}
	if s.User == "" {
		return errors.New("ssh user is required")
	}
	if err := s.Password.Validate(); err != nil {
		return fmt.Errorf("ssh password: %w", err)
	}
	return nil
}

type ServerEnvironment struct {
	APTSource  string   `yaml:"apt_source,omitempty"`
	Registries []string `yaml:"registries,omitempty"`
}

type Server struct {
	Name        string            `yaml:"name"`
	Zone        string            `yaml:"zone"`
	ISP         string            `yaml:"isp"`
	OS          string            `yaml:"os"`
	IP          ServerIP          `yaml:"ip"`
	SSH         ServerSSH         `yaml:"ssh"`
	Environment ServerEnvironment `yaml:"environment,omitempty"`
}

func (s *Server) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("%w: server name is required", ErrInvalidName)
	}
	if s.Zone == "" {
		return errors.New("zone is required")
	}
	if s.ISP == "" {
		return errors.New("isp is required")
	}
	if err := s.IP.Validate(); err != nil {
		return err
	}
	if err := s.SSH.Validate(); err != nil {
		return err
	}
	return nil
}

type ServiceHealthcheck struct {
	Path     string `yaml:"path"`
	Interval string `yaml:"interval"`
	Timeout  string `yaml:"timeout"`
}

func (h *ServiceHealthcheck) Validate() error {
	if h.Path == "" {
		return errors.New("healthcheck path is required")
	}
	if !strings.HasPrefix(h.Path, "/") {
		return errors.New("healthcheck path must start with /")
	}
	return nil
}

type ServiceResources struct {
	CPU    string `yaml:"cpu,omitempty"`
	Memory string `yaml:"memory,omitempty"`
}

type ServiceVolume struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
	Sync   bool   `yaml:"sync,omitempty"`
}

func (v *ServiceVolume) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var short string
	if err := unmarshal(&short); err == nil {
		parts := strings.SplitN(short, ":", 2)
		if len(parts) != 2 {
			return errors.New("invalid volume format, expected source:target")
		}
		v.Source = parts[0]
		v.Target = parts[1]
		return nil
	}

	type alias ServiceVolume
	var full alias
	if err := unmarshal(&full); err != nil {
		return err
	}
	v.Source = full.Source
	v.Target = full.Target
	v.Sync = full.Sync
	return nil
}

func (v *ServiceVolume) Validate() error {
	if v.Source == "" {
		return errors.New("volume source is required")
	}
	if v.Target == "" {
		return errors.New("volume target is required")
	}
	return nil
}

type ServiceGatewayConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Hostname string `yaml:"hostname,omitempty"`
	Path     string `yaml:"path,omitempty"`
	SSL      bool   `yaml:"ssl,omitempty"`
}

func (g *ServiceGatewayConfig) Validate() error {
	if !g.Enabled {
		return nil
	}
	if g.Hostname == "" {
		return errors.New("gateway hostname is required when enabled")
	}
	return nil
}

type Service struct {
	Name        string               `yaml:"name"`
	Server      string               `yaml:"server"`
	Image       string               `yaml:"image"`
	Port        int                  `yaml:"port"`
	Env         map[string]SecretRef `yaml:"env,omitempty"`
	Secrets     []string             `yaml:"secrets,omitempty"`
	Healthcheck *ServiceHealthcheck  `yaml:"healthcheck,omitempty"`
	Resources   ServiceResources     `yaml:"resources,omitempty"`
	Volumes     []ServiceVolume      `yaml:"volumes,omitempty"`
	Gateway     ServiceGatewayConfig `yaml:"gateway,omitempty"`
	Internal    bool                 `yaml:"internal,omitempty"`
}

func (s *Service) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("%w: service name is required", ErrInvalidName)
	}
	if s.Server == "" {
		return errors.New("server is required")
	}
	if s.Image == "" {
		return errors.New("image is required")
	}
	if s.Port <= 0 || s.Port > 65535 {
		return fmt.Errorf("%w: port must be between 1 and 65535", ErrInvalidPort)
	}
	if s.Healthcheck != nil {
		if err := s.Healthcheck.Validate(); err != nil {
			return err
		}
	}
	for i, vol := range s.Volumes {
		if err := vol.Validate(); err != nil {
			return fmt.Errorf("volume %d: %w", i, err)
		}
	}
	if err := s.Gateway.Validate(); err != nil {
		return err
	}
	return nil
}

type RegistryCredentials struct {
	Username SecretRef `yaml:"username"`
	Password SecretRef `yaml:"password"`
}

func (c *RegistryCredentials) Validate() error {
	if err := c.Username.Validate(); err != nil {
		return fmt.Errorf("username: %w", err)
	}
	if err := c.Password.Validate(); err != nil {
		return fmt.Errorf("password: %w", err)
	}
	return nil
}

type Registry struct {
	Name        string              `yaml:"name"`
	URL         string              `yaml:"url"`
	Credentials RegistryCredentials `yaml:"credentials"`
}

func (r *Registry) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("%w: registry name is required", ErrInvalidName)
	}
	if r.URL == "" {
		return errors.New("url is required")
	}
	if err := r.Credentials.Validate(); err != nil {
		return err
	}
	return nil
}

type Domain struct {
	Name      string `yaml:"name"`
	ISP       string `yaml:"isp"`
	Parent    string `yaml:"parent,omitempty"`
	AutoRenew bool   `yaml:"auto_renew,omitempty"`
}

var domainRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)

func (d *Domain) Validate() error {
	if d.Name == "" {
		return fmt.Errorf("%w: domain name is required", ErrInvalidDomain)
	}
	name := d.Name
	if strings.HasPrefix(name, "*.") {
		name = name[2:]
	}
	if !domainRegex.MatchString(name) {
		return fmt.Errorf("%w: invalid domain format %s", ErrInvalidDomain, d.Name)
	}
	if d.ISP == "" {
		return errors.New("isp is required")
	}
	return nil
}

type DNSRecordType string

const (
	DNSRecordTypeA     DNSRecordType = "A"
	DNSRecordTypeAAAA  DNSRecordType = "AAAA"
	DNSRecordTypeCNAME DNSRecordType = "CNAME"
	DNSRecordTypeMX    DNSRecordType = "MX"
	DNSRecordTypeTXT   DNSRecordType = "TXT"
	DNSRecordTypeNS    DNSRecordType = "NS"
	DNSRecordTypeSRV   DNSRecordType = "SRV"
)

type DNSRecord struct {
	Domain string        `yaml:"domain"`
	Type   DNSRecordType `yaml:"type"`
	Name   string        `yaml:"name"`
	Value  string        `yaml:"value"`
	TTL    int           `yaml:"ttl"`
}

func (r *DNSRecord) Validate() error {
	if r.Domain == "" {
		return errors.New("domain is required")
	}
	validTypes := map[DNSRecordType]bool{
		DNSRecordTypeA:     true,
		DNSRecordTypeAAAA:  true,
		DNSRecordTypeCNAME: true,
		DNSRecordTypeMX:    true,
		DNSRecordTypeTXT:   true,
		DNSRecordTypeNS:    true,
		DNSRecordTypeSRV:   true,
	}
	if !validTypes[r.Type] {
		return fmt.Errorf("invalid dns record type: %s", r.Type)
	}
	if r.Name == "" {
		return errors.New("name is required")
	}
	if r.Value == "" {
		return errors.New("value is required")
	}
	if r.TTL < 0 {
		return fmt.Errorf("%w: ttl must be non-negative", ErrInvalidTTL)
	}
	return nil
}

type CertificateProvider string

const (
	CertificateProviderLetsEncrypt CertificateProvider = "letsencrypt"
	CertificateProviderZeroSSL     CertificateProvider = "zerossl"
)

type Certificate struct {
	Name        string              `yaml:"name"`
	Domains     []string            `yaml:"domains"`
	Provider    CertificateProvider `yaml:"provider"`
	DNSProvider string              `yaml:"dns_provider"`
	AutoRenew   bool                `yaml:"auto_renew,omitempty"`
	RenewBefore string              `yaml:"renew_before,omitempty"`
}

func (c *Certificate) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("%w: certificate name is required", ErrInvalidName)
	}
	if len(c.Domains) == 0 {
		return errors.New("at least one domain is required")
	}
	for _, domain := range c.Domains {
		if domain == "" {
			return errors.New("domain cannot be empty")
		}
	}
	validProviders := map[CertificateProvider]bool{
		CertificateProviderLetsEncrypt: true,
		CertificateProviderZeroSSL:     true,
	}
	if !validProviders[c.Provider] {
		return fmt.Errorf("invalid certificate provider: %s", c.Provider)
	}
	if c.DNSProvider == "" {
		return errors.New("dns_provider is required")
	}
	return nil
}

type Config struct {
	Secrets      []Secret      `yaml:"secrets,omitempty"`
	ISPs         []ISP         `yaml:"isps,omitempty"`
	Zones        []Zone        `yaml:"zones,omitempty"`
	Gateways     []Gateway     `yaml:"gateways,omitempty"`
	Servers      []Server      `yaml:"servers,omitempty"`
	Services     []Service     `yaml:"services,omitempty"`
	Registries   []Registry    `yaml:"registries,omitempty"`
	Domains      []Domain      `yaml:"domains,omitempty"`
	DNSRecords   []DNSRecord   `yaml:"records,omitempty"`
	Certificates []Certificate `yaml:"certificates,omitempty"`
}

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
	for i, z := range c.Zones {
		if err := z.Validate(); err != nil {
			return fmt.Errorf("zones[%d]: %w", i, err)
		}
	}
	for i, g := range c.Gateways {
		if err := g.Validate(); err != nil {
			return fmt.Errorf("gateways[%d]: %w", i, err)
		}
	}
	for i, s := range c.Servers {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("servers[%d]: %w", i, err)
		}
	}
	for i, s := range c.Services {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("services[%d]: %w", i, err)
		}
	}
	for i, r := range c.Registries {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("registries[%d]: %w", i, err)
		}
	}
	for i, d := range c.Domains {
		if err := d.Validate(); err != nil {
			return fmt.Errorf("domains[%d]: %w", i, err)
		}
	}
	for i, r := range c.DNSRecords {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("records[%d]: %w", i, err)
		}
	}
	for i, cert := range c.Certificates {
		if err := cert.Validate(); err != nil {
			return fmt.Errorf("certificates[%d]: %w", i, err)
		}
	}
	return nil
}

func (c *Config) GetSecretsMap() map[string]string {
	m := make(map[string]string)
	for _, s := range c.Secrets {
		m[s.Name] = s.Value
	}
	return m
}

func (c *Config) GetISPMap() map[string]*ISP {
	m := make(map[string]*ISP)
	for i := range c.ISPs {
		m[c.ISPs[i].Name] = &c.ISPs[i]
	}
	return m
}

func (c *Config) GetZoneMap() map[string]*Zone {
	m := make(map[string]*Zone)
	for i := range c.Zones {
		m[c.Zones[i].Name] = &c.Zones[i]
	}
	return m
}

func (c *Config) GetGatewayMap() map[string]*Gateway {
	m := make(map[string]*Gateway)
	for i := range c.Gateways {
		m[c.Gateways[i].Name] = &c.Gateways[i]
	}
	return m
}

func (c *Config) GetServerMap() map[string]*Server {
	m := make(map[string]*Server)
	for i := range c.Servers {
		m[c.Servers[i].Name] = &c.Servers[i]
	}
	return m
}

func (c *Config) GetServiceMap() map[string]*Service {
	m := make(map[string]*Service)
	for i := range c.Services {
		m[c.Services[i].Name] = &c.Services[i]
	}
	return m
}

func (c *Config) GetRegistryMap() map[string]*Registry {
	m := make(map[string]*Registry)
	for i := range c.Registries {
		m[c.Registries[i].Name] = &c.Registries[i]
	}
	return m
}

func (c *Config) GetDomainMap() map[string]*Domain {
	m := make(map[string]*Domain)
	for i := range c.Domains {
		m[c.Domains[i].Name] = &c.Domains[i]
	}
	return m
}

func (c *Config) GetCertificateMap() map[string]*Certificate {
	m := make(map[string]*Certificate)
	for i := range c.Certificates {
		m[c.Certificates[i].Name] = &c.Certificates[i]
	}
	return m
}

func ParsePort(s string) (int, error) {
	port, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%w: %s", ErrInvalidPort, s)
	}
	if port <= 0 || port > 65535 {
		return 0, fmt.Errorf("%w: %d", ErrInvalidPort, port)
	}
	return port, nil
}
