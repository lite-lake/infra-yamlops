package entity

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lite-lake/infra-yamlops/internal/domain"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
)

type ServiceBase struct {
	Server   string   `yaml:"server"`
	Networks []string `yaml:"networks,omitempty"`
}

func (s *ServiceBase) GetServer() string {
	return s.Server
}

func (s *ServiceBase) GetNetworks() []string {
	return s.Networks
}

type ServiceHealthcheck struct {
	Path     string `yaml:"path"`
	Interval string `yaml:"interval"`
	Timeout  string `yaml:"timeout"`
}

func (h *ServiceHealthcheck) Validate() error {
	if h.Path == "" {
		return domain.RequiredField("healthcheck path")
	}
	if !strings.HasPrefix(h.Path, "/") {
		return fmt.Errorf("%w: healthcheck path must start with /", domain.ErrInvalidPath)
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
			return fmt.Errorf("%w: invalid volume format, expected source:target", domain.ErrInvalidFormat)
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
		return domain.RequiredField("volume source")
	}
	if v.Target == "" {
		return domain.RequiredField("volume target")
	}
	return nil
}

type ServicePort struct {
	Container int    `yaml:"container"`
	Host      int    `yaml:"host"`
	Protocol  string `yaml:"protocol,omitempty"`
}

func (p *ServicePort) Validate() error {
	if err := ValidatePort(p.Container); err != nil {
		return fmt.Errorf("container %w", err)
	}
	if err := ValidatePort(p.Host); err != nil {
		return fmt.Errorf("host %w", err)
	}
	if p.Protocol != "" && p.Protocol != "tcp" && p.Protocol != "udp" {
		return fmt.Errorf("%w: protocol must be 'tcp' or 'udp'", domain.ErrInvalidProtocol)
	}
	return nil
}

type RouteWAFConfig struct {
	Mode              string `yaml:"mode,omitempty"`                // 空=继承全局 default_mode；block | detect | disabled
	RequestBodyAccess *bool  `yaml:"request_body_access,omitempty"` // nil=继承全局；false=跳过请求体扫描（大体积二进制上传），仅检查头部/查询串
}

func (r *RouteWAFConfig) Validate() error {
	if r.Mode != "" && r.Mode != "block" && r.Mode != "detect" && r.Mode != "disabled" {
		return fmt.Errorf("waf.mode must be 'block', 'detect' or 'disabled', got '%s'", r.Mode)
	}
	return nil
}

// RouteAuthConfig 网关密码门配置（ServiceGatewayRoute.auth，内部PRD）。
// 启用后该网关 host 由 infra-gate 实施统一密码访问（单密码共享门，业务应用零改动）。
type RouteAuthConfig struct {
	// Password 访问密码；支持 secret 引用（推荐）或 plain 明文
	Password *valueobject.SecretRef `yaml:"password,omitempty"`
	// TTL 登录态有效期（Go duration 字符串），空 = 默认 24h
	TTL string `yaml:"ttl,omitempty"`
	// PublicPaths 免认证路径前缀（如 ["/healthz"]），空 = 全站拦截
	PublicPaths []string `yaml:"public_paths,omitempty"`
	// MaxFailures 登录失败锁定阈值（M3，内部PRD，gate auth.max_failures）：
	// 失败累计 N 次锁定来源 IP。0 = 不下发，gate 侧取缺省 5；Validate 拒绝负值。
	MaxFailures int `yaml:"max_failures,omitempty"`
	// LockDuration 登录失败锁定时长（M3，gate auth.lock_duration，Go duration 字符串）。
	// 空 = 不下发，gate 侧取缺省 15m；Validate 要求正 duration。
	LockDuration string `yaml:"lock_duration,omitempty"`
}

func (a *RouteAuthConfig) Validate() error {
	if a.Password == nil {
		return fmt.Errorf("%w: auth.password is required when auth is enabled", domain.ErrRequired)
	}
	if err := a.Password.Validate(); err != nil {
		return fmt.Errorf("auth.password: %w", err)
	}
	if a.TTL != "" {
		if _, err := time.ParseDuration(a.TTL); err != nil {
			return fmt.Errorf("auth.ttl '%s' is not a valid duration: %w", a.TTL, err)
		}
	}
	for _, p := range a.PublicPaths {
		if !strings.HasPrefix(p, "/") {
			return fmt.Errorf("auth.public_paths entry '%s' must start with /", p)
		}
	}
	if a.MaxFailures < 0 {
		return fmt.Errorf("auth.max_failures must be >= 0, got %d", a.MaxFailures)
	}
	if a.LockDuration != "" {
		d, err := time.ParseDuration(a.LockDuration)
		if err != nil {
			return fmt.Errorf("auth.lock_duration '%s' is not a valid duration: %w", a.LockDuration, err)
		}
		if d <= 0 {
			return fmt.Errorf("auth.lock_duration must be positive, got %s", a.LockDuration)
		}
	}
	return nil
}

type ServiceGatewayRoute struct {
	Hostname          string           `yaml:"hostname"`
	ContainerPort     int              `yaml:"container_port,omitempty"`
	Path              string           `yaml:"path,omitempty"`
	HTTP              bool             `yaml:"http,omitempty"`
	HTTPS             bool             `yaml:"https,omitempty"`
	GzipEnabled       *bool            `yaml:"gzip_enabled,omitempty"`
	OverrideHost      string           `yaml:"override_host,omitempty"`
	StripProxyHeaders *bool            `yaml:"strip_proxy_headers,omitempty"` // 是否剥离代理头部
	RobotsDenyAll     *bool            `yaml:"robots_deny_all,omitempty"`     // 强制覆盖 /robots.txt 拒绝所有爬虫，透传给 infra-gate
	AllowedIPs        []string         `yaml:"allowed_ips,omitempty"`         // 仅允许这些来源 IP/CIDR 访问该网关 host，空=nil=不限制
	WAF               *RouteWAFConfig  `yaml:"waf,omitempty"`                 // 虚拟主机级 WAF 配置
	BlacklistMode     string           `yaml:"blacklist_mode,omitempty"`      // 空=继承全局；block | detect | disabled
	Auth              *RouteAuthConfig `yaml:"auth,omitempty"`                // 密码门配置，透传给 infra-gate（内部PRD）
	// RedirectHTTPS 明文 HTTP 监听上收到请求时不代理，网关直接 301 → https://<原 host><RequestURI>
	// （内部总体方案 W3-2 / 决策 D8：逐条目跳转语义，不做 infra-gate 全局 80 端口行为变更）。
	// nil = false，透传给 infra-gate HostConfig.redirect_https
	RedirectHTTPS *bool `yaml:"redirect_https,omitempty"`
	// RedirectHost 规范域收敛：该 host（http 与 https 都算）请求 Host（剥端口后）≠ 本值时，
	// 网关 301 → https://<本值><RequestURI>，用于 www/别名域收敛到裸域/规范域。
	// 空 = 不启用；纯主机名（可含端口），Validate 禁止 "://" 与 "/"，透传给 infra-gate HostConfig.redirect_host
	RedirectHost string `yaml:"redirect_host,omitempty"`
	// HSTS 响应头 per-host 覆盖：nil = infra-gate 默认值（现状 max-age=31536000; includeSubDomains; preload）；
	// "off"（大小写不敏感）或 "" = 不下发；其他值 = 按字面值下发。用于双层网关链路只让边缘层下发一次，
	// 透传给 infra-gate HostConfig.hsts
	HSTS *string `yaml:"hsts,omitempty"`
	// WebSocketInsecureSkip L3（内部PRD）per-host WSS 上游跳过证书校验（gate
	// hosts[].web_socket_insecure_skip）。nil = 不下发，gate 侧继承全局
	// server.websocket_insecure_skip（兼容期，见 GatewayWebSocketInsecureSkip）；
	// per-host 显式配置优先。显式 true 由 gate 启动 Warn。
	WebSocketInsecureSkip *bool `yaml:"web_socket_insecure_skip,omitempty"`
}

type serviceGatewayRouteAlias ServiceGatewayRoute

func (r *ServiceGatewayRoute) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw struct {
		Hostname              string           `yaml:"hostname"`
		ContainerPort         *int             `yaml:"container_port"`
		Path                  string           `yaml:"path,omitempty"`
		HTTP                  bool             `yaml:"http,omitempty"`
		HTTPS                 bool             `yaml:"https,omitempty"`
		GzipEnabled           *bool            `yaml:"gzip_enabled,omitempty"`
		OverrideHost          string           `yaml:"override_host,omitempty"`
		StripProxyHeaders     *bool            `yaml:"strip_proxy_headers,omitempty"`
		RobotsDenyAll         *bool            `yaml:"robots_deny_all,omitempty"`
		AllowedIPs            []string         `yaml:"allowed_ips,omitempty"`
		WAF                   *RouteWAFConfig  `yaml:"waf,omitempty"`
		BlacklistMode         string           `yaml:"blacklist_mode,omitempty"`
		Auth                  *RouteAuthConfig `yaml:"auth,omitempty"`
		RedirectHTTPS         *bool            `yaml:"redirect_https,omitempty"`
		RedirectHost          string           `yaml:"redirect_host,omitempty"`
		HSTS                  *string          `yaml:"hsts,omitempty"`
		WebSocketInsecureSkip *bool            `yaml:"web_socket_insecure_skip,omitempty"`
	}
	if err := unmarshal(&raw); err != nil {
		return err
	}

	r.Hostname = raw.Hostname
	if raw.ContainerPort != nil {
		r.ContainerPort = *raw.ContainerPort
	} else {
		r.ContainerPort = 80
	}
	r.Path = raw.Path
	r.HTTP = raw.HTTP
	r.HTTPS = raw.HTTPS
	r.GzipEnabled = raw.GzipEnabled
	r.OverrideHost = raw.OverrideHost
	r.StripProxyHeaders = raw.StripProxyHeaders
	r.RobotsDenyAll = raw.RobotsDenyAll
	r.AllowedIPs = raw.AllowedIPs
	r.WAF = raw.WAF
	r.BlacklistMode = raw.BlacklistMode
	r.Auth = raw.Auth
	r.RedirectHTTPS = raw.RedirectHTTPS
	r.RedirectHost = raw.RedirectHost
	r.HSTS = raw.HSTS
	r.WebSocketInsecureSkip = raw.WebSocketInsecureSkip

	return nil
}

func (r *ServiceGatewayRoute) Validate() error {
	if r.Hostname == "" {
		return domain.RequiredField("gateway hostname")
	}
	if r.WAF != nil {
		if err := r.WAF.Validate(); err != nil {
			return err
		}
	}
	if r.BlacklistMode != "" && r.BlacklistMode != "block" && r.BlacklistMode != "detect" && r.BlacklistMode != "disabled" {
		return fmt.Errorf("blacklist_mode must be 'block', 'detect' or 'disabled', got '%s'", r.BlacklistMode)
	}
	// redirect_host 纯主机名校验（内部总体方案 W3-2 / D8）：infra-gate 用它构造
	// "https://<redirect_host><RequestURI>" 跳转地址，含 "://" 或 "/" 意味着配置成了 URL/路径
	// 而非主机名，fail-fast 拒绝（合法形态：纯主机名，可含端口，如 example.com[:8443]）
	if r.RedirectHost != "" {
		if strings.Contains(r.RedirectHost, "://") || strings.Contains(r.RedirectHost, "/") {
			return fmt.Errorf("%w: redirect_host must be a bare hostname (optionally with port), got '%s'", domain.ErrInvalidDomain, r.RedirectHost)
		}
	}
	if r.Auth != nil {
		if err := r.Auth.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (r *ServiceGatewayRoute) HasGateway() bool {
	return r.HTTP || r.HTTPS
}

type BizService struct {
	ServiceBase
	Name             string                           `yaml:"name"`
	Image            string                           `yaml:"image,omitempty"`
	ExternalBackends []string                         `yaml:"external_backends,omitempty"`
	Registry         string                           `yaml:"registry,omitempty"`
	Ports            []ServicePort                    `yaml:"ports,omitempty"`
	Env              map[string]valueobject.SecretRef `yaml:"env,omitempty"`
	Secrets          []string                         `yaml:"secrets,omitempty"`
	Healthcheck      *ServiceHealthcheck              `yaml:"healthcheck,omitempty"`
	Resources        ServiceResources                 `yaml:"resources,omitempty"`
	Volumes          []ServiceVolume                  `yaml:"volumes,omitempty"`
	Gateways         []ServiceGatewayRoute            `yaml:"gateways,omitempty"`
	Internal         bool                             `yaml:"internal,omitempty"`
}

type bizServiceAlias BizService

func (s *BizService) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw struct {
		Name             string                           `yaml:"name"`
		Server           string                           `yaml:"server"`
		Networks         []string                         `yaml:"networks,omitempty"`
		Image            string                           `yaml:"image,omitempty"`
		ExternalBackends []string                         `yaml:"external_backends,omitempty"`
		Registry         string                           `yaml:"registry,omitempty"`
		Ports            []ServicePort                    `yaml:"ports,omitempty"`
		Env              map[string]valueobject.SecretRef `yaml:"env,omitempty"`
		Secrets          []string                         `yaml:"secrets,omitempty"`
		Healthcheck      *ServiceHealthcheck              `yaml:"healthcheck,omitempty"`
		Resources        ServiceResources                 `yaml:"resources,omitempty"`
		Volumes          []ServiceVolume                  `yaml:"volumes,omitempty"`
		Gateways         []ServiceGatewayRoute            `yaml:"gateways,omitempty"`
		Internal         bool                             `yaml:"internal,omitempty"`
	}
	if err := unmarshal(&raw); err != nil {
		return err
	}

	s.Name = raw.Name
	s.ServiceBase.Server = raw.Server
	s.ServiceBase.Networks = raw.Networks
	s.Image = raw.Image
	s.ExternalBackends = raw.ExternalBackends
	s.Registry = raw.Registry
	s.Ports = raw.Ports
	s.Env = raw.Env
	s.Secrets = raw.Secrets
	s.Healthcheck = raw.Healthcheck
	s.Resources = raw.Resources
	s.Volumes = raw.Volumes
	s.Gateways = raw.Gateways
	s.Internal = raw.Internal

	return nil
}

func (s *BizService) MarshalYAML() (interface{}, error) {
	return struct {
		Name             string                           `yaml:"name"`
		Server           string                           `yaml:"server"`
		Networks         []string                         `yaml:"networks,omitempty"`
		Image            string                           `yaml:"image,omitempty"`
		ExternalBackends []string                         `yaml:"external_backends,omitempty"`
		Registry         string                           `yaml:"registry,omitempty"`
		Ports            []ServicePort                    `yaml:"ports,omitempty"`
		Env              map[string]valueobject.SecretRef `yaml:"env,omitempty"`
		Secrets          []string                         `yaml:"secrets,omitempty"`
		Healthcheck      *ServiceHealthcheck              `yaml:"healthcheck,omitempty"`
		Resources        ServiceResources                 `yaml:"resources,omitempty"`
		Volumes          []ServiceVolume                  `yaml:"volumes,omitempty"`
		Gateways         []ServiceGatewayRoute            `yaml:"gateways,omitempty"`
		Internal         bool                             `yaml:"internal,omitempty"`
	}{
		Name:             s.Name,
		Server:           s.ServiceBase.Server,
		Networks:         s.ServiceBase.Networks,
		Image:            s.Image,
		ExternalBackends: s.ExternalBackends,
		Registry:         s.Registry,
		Ports:            s.Ports,
		Env:              s.Env,
		Secrets:          s.Secrets,
		Healthcheck:      s.Healthcheck,
		Resources:        s.Resources,
		Volumes:          s.Volumes,
		Gateways:         s.Gateways,
		Internal:         s.Internal,
	}, nil
}

func (s *BizService) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("%w: service name is required", domain.ErrInvalidName)
	}
	if s.Server == "" {
		return domain.RequiredField("server")
	}
	if s.Image == "" && len(s.ExternalBackends) == 0 {
		return fmt.Errorf("%w: either image or external_backends is required", domain.ErrRequired)
	}
	if s.Image != "" && len(s.ExternalBackends) > 0 {
		return fmt.Errorf("%w: cannot specify both image and external_backends", domain.ErrInvalidFormat)
	}
	for i, port := range s.Ports {
		if err := port.Validate(); err != nil {
			return fmt.Errorf("port %d: %w", i, err)
		}
	}
	envKeys := make([]string, 0, len(s.Env))
	for key := range s.Env {
		envKeys = append(envKeys, key)
	}
	sort.Strings(envKeys)
	for _, key := range envKeys {
		ref := s.Env[key]
		if err := ref.Validate(); err != nil {
			return fmt.Errorf("env[%s]: %w", key, err)
		}
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
	for i, gw := range s.Gateways {
		if err := gw.Validate(); err != nil {
			return fmt.Errorf("gateway %d: %w", i, err)
		}
	}
	return nil
}
