package entity

import (
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/lite-lake/infra-yamlops/internal/domain"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
	"gopkg.in/yaml.v3"
)

type InfraServiceType string

const (
	InfraServiceTypeGateway      InfraServiceType = "gateway"
	InfraServiceTypeTunnelServer InfraServiceType = "tunnel-server"
	InfraServiceTypeTunnelClient InfraServiceType = "tunnel-client"
	InfraServiceTypeEasyTier     InfraServiceType = "easytier"
	InfraServiceTypeUptimeKuma   InfraServiceType = "uptime-kuma"
)

type GatewayPorts struct {
	HTTP  int `yaml:"http"`
	HTTPS int `yaml:"https"`
}

func (p *GatewayPorts) Validate() error {
	if err := ValidatePort(p.HTTP); err != nil {
		return fmt.Errorf("http %w", err)
	}
	if err := ValidatePort(p.HTTPS); err != nil {
		return fmt.Errorf("https %w", err)
	}
	return nil
}

type GatewaySSLConfig struct {
	Mode     string `yaml:"mode"`
	Endpoint string `yaml:"endpoint,omitempty"`
	APIKey   string `yaml:"api_key,omitempty"`
}

func (s *GatewaySSLConfig) Validate() error {
	if s.Mode != "local" && s.Mode != "remote" {
		return fmt.Errorf("%w: ssl mode must be 'local' or 'remote'", domain.ErrInvalidType)
	}
	if s.Mode == "remote" && s.Endpoint == "" {
		return domain.RequiredField("endpoint for remote ssl mode")
	}
	return nil
}

type GatewayWAFConfig struct {
	Enabled      bool     `yaml:"enabled"`
	DefaultMode  string   `yaml:"default_mode,omitempty"` // block | detect | disabled
	Whitelist    []string `yaml:"whitelist,omitempty"`
	CRSProxyURL  string   `yaml:"crs_proxy_url,omitempty"`  // CRS 下载代理 URL，为空则不使用代理
	CRSMirrorURL string   `yaml:"crs_mirror_url,omitempty"` // CRS 规则镜像服务地址，为空则直连 GitHub
	// CRSFailurePolicy CRS 不可用时的失败策略（内部PRD §5.4/§5.7）：
	// degrade = 无 WAF 启动 + 高优告警；fail = WAF 构造失败进程退出（gate 侧现状默认）。
	// 空 = 不下发，gate 侧默认 fail（与现状一致）。
	CRSFailurePolicy string `yaml:"crs_failure_policy,omitempty"`
	// CRSPinnedSHA256 H2（内部PRD）CRS tarball 期望 sha256 pin（gate
	// waf.crs.download.pinned_sha256，最高优先、优先于镜像 pins 清单）。容忍可选
	// "sha256:" 前缀；格式（归一化后 64 位十六进制）在 Validate fail-fast，
	// 避免畸形 pin 到 gate 启动/刷新期才报错。
	CRSPinnedSHA256 string `yaml:"crs_pinned_sha256,omitempty"`
	// CRSMirrorAllowInsecureHTTP H2（内部PRD）镜像站明文 http 放行开关（gate
	// waf.crs.download.mirror_allow_insecure_http，内网明文镜像逃生口 D6）。
	// nil = false（仅 https，明文 http 镜像 gate 启动校验直接报错）；显式 true 由 gate 启动 Warn。
	CRSMirrorAllowInsecureHTTP *bool `yaml:"crs_mirror_allow_insecure_http,omitempty"`
}

func (w *GatewayWAFConfig) Validate() error {
	if w.DefaultMode != "" && w.DefaultMode != "block" && w.DefaultMode != "detect" && w.DefaultMode != "disabled" {
		return fmt.Errorf("waf.default_mode must be 'block', 'detect' or 'disabled', got '%s'", w.DefaultMode)
	}
	if w.CRSFailurePolicy != "" && w.CRSFailurePolicy != "degrade" && w.CRSFailurePolicy != "fail" {
		return fmt.Errorf("waf.crs_failure_policy must be 'degrade' or 'fail', got '%s'", w.CRSFailurePolicy)
	}
	if err := validatePinnedSHA256(w.CRSPinnedSHA256); err != nil {
		return err
	}
	for _, cidr := range w.Whitelist {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return fmt.Errorf("%w: %s", domain.ErrInvalidCIDR, cidr)
		}
	}
	return nil
}

// crsPinnedSHA256HexLen 归一化后期望的十六进制 sha256 摘要长度（与 infra-gate 校验口径一致）
const crsPinnedSHA256HexLen = 64

// validatePinnedSHA256 校验 crs_pinned_sha256 格式（与 infra-gate validatePinnedSHA256 同口径）：
// 空串 = 未配置放行；非空时去首尾空白、容忍可选 "sha256:" 前缀，归一化后必须是
// crsPinnedSHA256HexLen 位十六进制。畸形 pin 若放行会在 gate 启动/每次刷新完整性校验处
// 反复失败，yamlops validate 阶段 fail-fast 更早暴露
func validatePinnedSHA256(pin string) error {
	trimmed := strings.TrimSpace(pin)
	if trimmed == "" {
		return nil
	}
	normalized := trimmed
	if len(normalized) >= len("sha256:") && strings.EqualFold(normalized[:len("sha256:")], "sha256:") {
		normalized = strings.TrimSpace(normalized[len("sha256:"):])
	}
	if len(normalized) != crsPinnedSHA256HexLen {
		return fmt.Errorf("waf.crs_pinned_sha256 %q must be a %d-char hex sha256 (optional %q prefix allowed), got %d chars after normalization", pin, crsPinnedSHA256HexLen, "sha256:", len(normalized))
	}
	for _, r := range normalized {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return fmt.Errorf("waf.crs_pinned_sha256 %q must be a %d-char hex sha256 (optional %q prefix allowed)", pin, crsPinnedSHA256HexLen, "sha256:")
		}
	}
	return nil
}

// GatewayBlacklistConfig IP 黑名单配置（gate 全局段，与 WAF 平级）。
// Whitelist 条目支持单 IP 或 CIDR（与 waf.whitelist 仅 CIDR 不同，gate 侧两者都支持）。
type GatewayBlacklistConfig struct {
	Enabled     bool     `yaml:"enabled"`
	DefaultMode string   `yaml:"default_mode,omitempty"` // block | detect | disabled，空默认 block
	Whitelist   []string `yaml:"whitelist,omitempty"`    // 本地豁免 IP/CIDR
	SourceURL   string   `yaml:"source_url,omitempty"`   // 黑名单源 URL，为空则使用 gate 侧默认 USTC 源
	// AllowHTTPSource L6（内部PRD）明文 http 数据源放行开关（gate
	// blacklist.allow_http_source，内网明文 feed 逃生口 D6）。nil = false（仅 https，
	// 明文 http 源 gate 启动校验直接报错）。
	AllowHTTPSource *bool `yaml:"allow_http_source,omitempty"`
	// MaxEntries L6（内部PRD）防追加式投毒的增长上限基数（gate
	// blacklist.max_entries）：新 feed 去重条目数超过 max(2×上次成功条目数, max_entries)
	// 拒绝替换、保留旧表。0 = 不下发，gate 侧取缺省 200000；Validate 拒绝负值。
	MaxEntries int `yaml:"max_entries,omitempty"`
}

// validBlacklistWhitelistEntry 校验 blacklist.whitelist 条目：单 IP 或 CIDR 均合法
func validBlacklistWhitelistEntry(entry string) bool {
	if net.ParseIP(entry) != nil {
		return true
	}
	if _, _, err := net.ParseCIDR(entry); err == nil {
		return true
	}
	return false
}

func (b *GatewayBlacklistConfig) Validate() error {
	if b.DefaultMode != "" && b.DefaultMode != "block" && b.DefaultMode != "detect" && b.DefaultMode != "disabled" {
		return fmt.Errorf("blacklist.default_mode must be 'block', 'detect' or 'disabled', got '%s'", b.DefaultMode)
	}
	for _, entry := range b.Whitelist {
		if !validBlacklistWhitelistEntry(entry) {
			return fmt.Errorf("%w: %s (expected IP or CIDR)", domain.ErrInvalidIP, entry)
		}
	}
	if b.SourceURL != "" {
		u, err := url.Parse(b.SourceURL)
		if err != nil || u.Scheme == "" || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return fmt.Errorf("%w: blacklist.source_url %q must be a valid http/https URL", domain.ErrInvalidURL, b.SourceURL)
		}
	}
	if b.MaxEntries < 0 {
		return fmt.Errorf("blacklist.max_entries must be >= 0, got %d", b.MaxEntries)
	}
	return nil
}

// validateGatewayTrustedProxies 校验 gate 的可信代理网段列表（server 级 trusted_proxies）：
// 逐条要求合法 CIDR 或裸 IP；禁止 0.0.0.0/0、::/0（防止误配成信任所有来源，
// 与 infra-gate 侧 Validate 的 fail-fast 行为对齐，内部PRD §6.1）。
func validateGatewayTrustedProxies(entries []string) error {
	for i, entry := range entries {
		if entry == "0.0.0.0/0" || entry == "::/0" {
			return fmt.Errorf("%w: trusted_proxies[%d] %q must not trust all clients (only internal mesh segments allowed)", domain.ErrInvalidCIDR, i, entry)
		}
		if _, _, err := net.ParseCIDR(entry); err != nil {
			if net.ParseIP(entry) == nil {
				return fmt.Errorf("%w: trusted_proxies[%d] %q (expected CIDR or IP)", domain.ErrInvalidCIDR, i, entry)
			}
		}
	}
	return nil
}

type GatewayNotification struct {
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url"`
	Timeout string `yaml:"timeout,omitempty"` // 默认 "5s"
}

func (n *GatewayNotification) Validate() error {
	if n.Enabled && n.URL == "" {
		return domain.RequiredField("notification url when enabled")
	}
	return nil
}

// GatewayTrackerShipperConfig C2 网关通道 shipper 配置（infra-gate 顶层 tracker_shipper 段，
// 与 server:/hosts: 平级，内部PRD §7.1）。逐节点启用访问日志实时外送，默认关。
// 未配置（nil 指针）时 gate 产物不生成该键（存量输出逐字节不变）。
type GatewayTrackerShipperConfig struct {
	// Enabled 总开关；false 时段仍可存在（缺省形态），gate 侧不启动 shipper
	Enabled bool `yaml:"enabled"`
	// Endpoint ingest 端点（mesh 域名，如 https://app-demo.node1.v-dev.mesh.example.net/api/internal/gateway-ingest）
	Endpoint string `yaml:"endpoint,omitempty"`
	// SharedSecret 共享密钥；支持 {secret: name} 引用（推荐）或 plain 明文（照 auth.password 先例）。
	// 注意不可加 omitempty：SecretRef 仅含非导出字段，yaml.v3 的 omitempty 会把它误判为零值丢弃。
	SharedSecret valueobject.SecretRef `yaml:"shared_secret"`
	// NodeName 上报节点名；未配置时由生成器自动注入容器命名 yo-{env}-{服务名}
	NodeName string `yaml:"node_name,omitempty"`
	// BufferSize 有界缓冲条数，0 = 未配置（生成时填默认 10000）
	BufferSize int `yaml:"buffer_size,omitempty"`
	// BatchSize 批量发送条数，0 = 未配置（生成时填默认 500）
	BatchSize int `yaml:"batch_size,omitempty"`
	// FlushInterval flush 周期（Go duration 字符串），空 = 未配置（生成时填默认 1s）
	FlushInterval string `yaml:"flush_interval,omitempty"`
	// FailureSampleLogRatio 丢弃/失败采样日志比例 [0,1]，0 = 未配置（生成时填默认 0.01，
	// 因此该字段无法显式配 0；确需极低采样可配极小正值如 0.0001）
	FailureSampleLogRatio float64 `yaml:"failure_sample_log_ratio,omitempty"`
}

func (t *GatewayTrackerShipperConfig) Validate() error {
	if t.FlushInterval != "" {
		d, err := time.ParseDuration(t.FlushInterval)
		if err != nil || d <= 0 {
			return fmt.Errorf("%w: tracker_shipper.flush_interval %q must be a valid positive duration (e.g. 1s, 500ms)", domain.ErrInvalidDuration, t.FlushInterval)
		}
	}
	if t.BufferSize < 0 {
		return fmt.Errorf("%w: tracker_shipper.buffer_size must be >= 0, got %d", domain.ErrInvalidFormat, t.BufferSize)
	}
	if t.BatchSize < 0 {
		return fmt.Errorf("%w: tracker_shipper.batch_size must be >= 0, got %d", domain.ErrInvalidFormat, t.BatchSize)
	}
	if t.FailureSampleLogRatio < 0 || t.FailureSampleLogRatio > 1 {
		return fmt.Errorf("%w: tracker_shipper.failure_sample_log_ratio must be within [0, 1], got %v", domain.ErrInvalidFormat, t.FailureSampleLogRatio)
	}
	if !t.Enabled {
		return nil
	}
	if t.Endpoint == "" {
		return domain.RequiredField("tracker_shipper endpoint when enabled")
	}
	if err := t.SharedSecret.Validate(); err != nil {
		return fmt.Errorf("tracker_shipper.shared_secret: %w", err)
	}
	// node_name 允许为空：未配置时由生成器自动注入 yo-{env}-{服务名}
	// （见 generator.buildTrackerShipperConfig，显式配置值优先），产物保证非空，
	// 实体层不作必填限制（避免 LoadAndValidate 先于生成执行时把自动注入变成死路径）。
	return nil
}

// TunnelServerPorts frps 端口配置
type TunnelServerPorts struct {
	BindPort      int `yaml:"bind_port"`       // 隧道控制端口，默认 30070
	VhostHTTPPort int `yaml:"vhost_http_port"` // HTTP vhost 端口，默认 80
}

func (p *TunnelServerPorts) Validate() error {
	if err := ValidatePort(p.BindPort); err != nil {
		return fmt.Errorf("bind_port %w", err)
	}
	if err := ValidatePort(p.VhostHTTPPort); err != nil {
		return fmt.Errorf("vhost_http_port %w", err)
	}
	return nil
}

// TunnelRoute 声明一个隧道路由。
// yamlops 会在同服务器的 gate 配置中自动生成指向 frps vhost 的 host 条目。
type TunnelRoute struct {
	Hostname string `yaml:"hostname"`
}

func (r *TunnelRoute) Validate() error {
	if r.Hostname == "" {
		return domain.RequiredField("tunnel_routes hostname")
	}
	return nil
}

// TunnelServerConfig tunnel-server（frps）配置
type TunnelServerConfig struct {
	Ports        TunnelServerPorts     `yaml:"ports,omitempty"`
	AuthToken    valueobject.SecretRef `yaml:"auth_token"`
	TunnelRoutes []TunnelRoute         `yaml:"tunnel_routes,omitempty"`
}

func (c *TunnelServerConfig) Validate() error {
	if c.Ports.BindPort == 0 {
		c.Ports.BindPort = 30070
	}
	if c.Ports.VhostHTTPPort == 0 {
		c.Ports.VhostHTTPPort = 80
	}
	if err := c.Ports.Validate(); err != nil {
		return err
	}
	if err := c.AuthToken.Validate(); err != nil {
		return fmt.Errorf("auth_token: %w", err)
	}
	if len(c.TunnelRoutes) == 0 {
		return domain.RequiredField("tunnel_routes (at least one)")
	}
	seen := make(map[string]bool)
	for i, r := range c.TunnelRoutes {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("tunnel_routes[%d]: %w", i, err)
		}
		if seen[r.Hostname] {
			return fmt.Errorf("tunnel_routes[%d].hostname %q is duplicated", i, r.Hostname)
		}
		seen[r.Hostname] = true
	}
	return nil
}

// TunnelClientProxy 定义 frpc 的一个代理条目
type TunnelClientProxy struct {
	Name          string   `yaml:"name"`
	Type          string   `yaml:"type"`                     // "http" | "https" | "tcp"
	CustomDomains []string `yaml:"custom_domains,omitempty"` // HTTP/HTTPS 类型用
	LocalAddr     string   `yaml:"local_addr"`               // 如 "gateway-node1:80"
	RemotePort    int      `yaml:"remote_port,omitempty"`    // TCP 类型用
}

func (p *TunnelClientProxy) Validate() error {
	if p.Name == "" {
		return domain.RequiredField("proxies name")
	}
	if p.Type != "http" && p.Type != "https" && p.Type != "tcp" {
		return fmt.Errorf("%w: proxies type must be 'http', 'https' or 'tcp', got '%s'", domain.ErrInvalidType, p.Type)
	}
	if p.LocalAddr == "" {
		return domain.RequiredField("proxies local_addr")
	}
	if (p.Type == "http" || p.Type == "https") && len(p.CustomDomains) == 0 {
		return domain.RequiredField("proxies custom_domains for http/https type")
	}
	if p.Type == "tcp" && p.RemotePort == 0 {
		return domain.RequiredField("proxies remote_port for tcp type")
	}
	if p.Type == "tcp" {
		if err := ValidatePort(p.RemotePort); err != nil {
			return fmt.Errorf("remote_port %w", err)
		}
	}
	return nil
}

// TunnelClientConfig tunnel-client（frpc）配置
type TunnelClientConfig struct {
	ServerAddr string                `yaml:"server_addr"` // 如 "gate-node2.mesh.example.net:30070"
	AuthToken  valueobject.SecretRef `yaml:"auth_token"`
	Proxies    []TunnelClientProxy   `yaml:"proxies,omitempty"`
}

func (c *TunnelClientConfig) Validate() error {
	if c.ServerAddr == "" {
		return domain.RequiredField("server_addr")
	}
	if err := c.AuthToken.Validate(); err != nil {
		return fmt.Errorf("auth_token: %w", err)
	}
	if len(c.Proxies) == 0 {
		return domain.RequiredField("proxies (at least one)")
	}
	seen := make(map[string]bool)
	for i, p := range c.Proxies {
		if err := p.Validate(); err != nil {
			return fmt.Errorf("proxies[%d]: %w", i, err)
		}
		if seen[p.Name] {
			return fmt.Errorf("proxies[%d].name %q is duplicated", i, p.Name)
		}
		seen[p.Name] = true
	}
	return nil
}

// EasyTierFlags 对应 easytier-core 的 flags 表（除 listeners/peers/network_identity 外的布尔/字符串开关）。
// 生成 toml 时全部显式写出，避免依赖 easytier 默认值。
type EasyTierFlags struct {
	LatencyFirst          bool   `yaml:"latency_first,omitempty"`           // --latency-first
	PrivateMode           bool   `yaml:"private_mode,omitempty"`            // --private-mode
	RelayNetworkWhitelist string `yaml:"relay_network_whitelist,omitempty"` // --relay-network-whitelist，默认 "*"，必须显式写 "" 才表示"不转发其他网络"
	RelayAllPeerRPC       bool   `yaml:"relay_all_peer_rpc,omitempty"`      // --relay-all-peer-rpc
	NoListener            bool   `yaml:"no_listener,omitempty"`             // --no-listener（叶子节点）
}

// EasyTierConfig easytier 节点配置（L3 overlay mesh）
type EasyTierConfig struct {
	Role          string                `yaml:"role"`                // "shared" | "leaf"
	VirtualIP     string                `yaml:"virtual_ip"`          // "10.171.0.1/24"（-i）
	NetworkName   string                `yaml:"network_name"`        // "lite-mesh-dev"（-n）
	NetworkSecret valueobject.SecretRef `yaml:"network_secret"`      // -k
	Hostname      string                `yaml:"hostname,omitempty"`  // --hostname
	Listeners     []string              `yaml:"listeners,omitempty"` // -l，仅 shared 角色
	Peers         []string              `yaml:"peers,omitempty"`     // -p
	Flags         EasyTierFlags         `yaml:"flags,omitempty"`     // 其余开关
}

func (c *EasyTierConfig) Validate() error {
	if c.Role != "shared" && c.Role != "leaf" {
		return fmt.Errorf("%w: easytier role must be 'shared' or 'leaf', got '%s'", domain.ErrInvalidType, c.Role)
	}
	if c.VirtualIP == "" {
		return domain.RequiredField("easytier virtual_ip")
	}
	ip, _, err := net.ParseCIDR(c.VirtualIP)
	if err != nil {
		return fmt.Errorf("%w: easytier virtual_ip '%s': %v", domain.ErrInvalidCIDR, c.VirtualIP, err)
	}
	if ip == nil || ip.To4() == nil {
		return fmt.Errorf("%w: easytier virtual_ip must be an IPv4 CIDR: %s", domain.ErrInvalidCIDR, c.VirtualIP)
	}
	if c.NetworkName == "" {
		return domain.RequiredField("easytier network_name")
	}
	if err := c.NetworkSecret.Validate(); err != nil {
		return fmt.Errorf("easytier network_secret: %w", err)
	}
	switch c.Role {
	case "shared":
		if len(c.Listeners) == 0 {
			return domain.RequiredField("easytier listeners (at least one) for shared role")
		}
		for i, l := range c.Listeners {
			if !validEasyTierListenerURI(l) {
				return fmt.Errorf("%w: easytier listeners[%d] '%s' (expected scheme://ip:port)", domain.ErrInvalidType, i, l)
			}
		}
		if c.Flags.NoListener {
			return fmt.Errorf("%w: easytier shared role cannot set no_listener", domain.ErrInvalidType)
		}
	case "leaf":
		if len(c.Listeners) > 0 {
			return fmt.Errorf("%w: easytier leaf role cannot set listeners", domain.ErrInvalidType)
		}
	}
	for i, p := range c.Peers {
		if !validEasyTierListenerURI(p) {
			return fmt.Errorf("%w: easytier peers[%d] '%s' (expected scheme://host:port)", domain.ErrInvalidType, i, p)
		}
	}
	return nil
}

// validEasyTierListenerURI 校验 easytier 的 URI 格式（scheme://host:port）
func validEasyTierListenerURI(s string) bool {
	if s == "" {
		return false
	}
	u, err := url.Parse(s)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}
	switch u.Scheme {
	case "tcp", "udp", "quic", "ring", "wg", "ws", "wss", "faketcp":
		return true
	}
	return false
}

// UptimeKumaAdminConfig Kuma 首个管理员账号。
// 首启时 agent 通过 needSetup/setup 完成无人值守建号。
type UptimeKumaAdminConfig struct {
	Username string                `yaml:"username"`
	Password valueobject.SecretRef `yaml:"password"`
}

func (c *UptimeKumaAdminConfig) Validate() error {
	if c.Username == "" {
		return domain.RequiredField("uptime-kuma admin username")
	}
	if err := c.Password.Validate(); err != nil {
		return fmt.Errorf("uptime-kuma admin password: %w", err)
	}
	return nil
}

// UptimeKumaDBConfig Kuma 外部数据库（MariaDB/MySQL）连接配置。
type UptimeKumaDBConfig struct {
	Type     string                `yaml:"type"`     // sqlite | mariadb | embedded-mariadb
	Hostname string                `yaml:"hostname"` // DB 主机
	Port     int                   `yaml:"port"`     // DB 端口
	Name     string                `yaml:"name"`     // 数据库名
	Username string                `yaml:"username"`
	Password valueobject.SecretRef `yaml:"password"`
}

func (d *UptimeKumaDBConfig) Validate() error {
	if d.Type == "" {
		return domain.RequiredField("uptime-kuma db type")
	}
	if d.Type != "sqlite" && d.Type != "mariadb" && d.Type != "embedded-mariadb" {
		return fmt.Errorf("%w: uptime-kuma db type must be 'sqlite', 'mariadb' or 'embedded-mariadb', got '%s'", domain.ErrInvalidType, d.Type)
	}
	if d.Hostname == "" {
		return domain.RequiredField("uptime-kuma db hostname")
	}
	if err := ValidatePort(d.Port); err != nil {
		return fmt.Errorf("uptime-kuma db port %w", err)
	}
	if d.Name == "" {
		return domain.RequiredField("uptime-kuma db name")
	}
	if d.Username == "" {
		return domain.RequiredField("uptime-kuma db username")
	}
	if err := d.Password.Validate(); err != nil {
		return fmt.Errorf("uptime-kuma db password: %w", err)
	}
	return nil
}

// UptimeKumaGatewayConfig 声明一个 Kuma 网关 host（enabled 时该域名经 gate 反代到 Kuma）。
type UptimeKumaGatewayConfig struct {
	Hostname      string `yaml:"hostname"`
	HTTPS         bool   `yaml:"https"`
	ContainerPort int    `yaml:"container_port"` // 默认 3001
}

func (g *UptimeKumaGatewayConfig) Validate() error {
	if g.Hostname == "" {
		return domain.RequiredField("uptime-kuma gateways hostname")
	}
	if g.ContainerPort != 0 {
		if err := ValidatePort(g.ContainerPort); err != nil {
			return fmt.Errorf("uptime-kuma gateways container_port %w", err)
		}
	}
	return nil
}

// UptimeKumaSyncConfig agent 对标容器配置。
type UptimeKumaSyncConfig struct {
	Image             string `yaml:"image"`                      // agent 镜像（明文）
	ReconcileInterval string `yaml:"reconcile_interval"`         // 对标周期，默认 60s
	ReloginInterval   string `yaml:"relogin_interval,omitempty"` // 主动重登兜底；空=agent 默认 1h，0s=关闭
}

func (c *UptimeKumaSyncConfig) Validate() error {
	if c.Image == "" {
		return domain.RequiredField("uptime-kuma sync.image")
	}
	if c.ReconcileInterval != "" {
		d, err := time.ParseDuration(c.ReconcileInterval)
		if err != nil || d <= 0 {
			return fmt.Errorf("%w: uptime-kuma sync.reconcile_interval %q must be a valid positive duration (e.g. 30s, 2m)", domain.ErrInvalidType, c.ReconcileInterval)
		}
	}
	// relogin_interval 允许空（agent 默认 1h）、0s（关闭兜底）或正 duration。
	if c.ReloginInterval != "" {
		d, err := time.ParseDuration(c.ReloginInterval)
		if err != nil || d < 0 {
			return fmt.Errorf("%w: uptime-kuma sync.relogin_interval %q must be a valid non-negative duration or 0s (e.g. 1h, 30m, 0s)", domain.ErrInvalidType, c.ReloginInterval)
		}
	}
	return nil
}

// UptimeKumaNotificationConfig 告警通知（mc 企微 webhook）。
type UptimeKumaNotificationConfig struct {
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url,omitempty"`
}

func (c *UptimeKumaNotificationConfig) Validate() error {
	if c.Enabled && c.URL == "" {
		return domain.RequiredField("uptime-kuma notification url when enabled")
	}
	return nil
}

// uptimeKumaMonitorTypes Kuma 支持的监控类型（socket.io payload 的 type 取值）。
var uptimeKumaMonitorTypes = map[string]bool{
	"http":            true,
	"http-browser":    true,
	"keyword":         true,
	"keyword-browser": true,
	"json-query":      true, // Kuma 1.23+ json-query 监控（jsonPath/expectedValue 走 Extra 透传）
	"port":            true,
	"ping":            true,
	"dns":             true,
	"push":            true,
	"group":           true,
}

// uptimeKumaMonitorTypesList 返回全部允许类型的排序列表（用于错误文案，与 map 保持一致）。
func uptimeKumaMonitorTypesList() string {
	types := make([]string, 0, len(uptimeKumaMonitorTypes))
	for t := range uptimeKumaMonitorTypes {
		types = append(types, t)
	}
	sort.Strings(types)
	return strings.Join(types, "/")
}

// UptimeKumaMonitorConfig 监控目标，yaml tag 与 Kuma socket payload 的 camelCase 字段保持一致，
// 除 tags（由 yamlops 固定打 [yamlops]）外的字段原样透传给 agent。
// 未知字段（如 packetSize / dns_resolve_type / method / headers / timeout / retryInterval 等）
// 收集到 Extra 透传，避免白名单静默丢弃高级字段。
type UptimeKumaMonitorConfig struct {
	Name                     string   `yaml:"name"`
	Type                     string   `yaml:"type"`
	URL                      string   `yaml:"url,omitempty"`
	Interval                 int      `yaml:"interval,omitempty"`
	AcceptedStatuscodes      []string `yaml:"accepted_statuscodes,omitempty"`
	MaxRedirects             int      `yaml:"maxredirects,omitempty"`
	ExpiryNotification       bool     `yaml:"expiryNotification,omitempty"`
	DomainExpiryNotification bool     `yaml:"domainExpiryNotification,omitempty"`
	Keyword                  string   `yaml:"keyword,omitempty"`
	Hostname                 string   `yaml:"hostname,omitempty"`
	Port                     int      `yaml:"port,omitempty"`
	Tags                     []string `yaml:"tags,omitempty"` // 生成 targets.yaml 时固定 [yamlops]
	// Extra 收集白名单外的 camelCase 字段，原样写入 kuma.targets.yaml 透传给 agent。
	Extra map[string]any `yaml:"-"`
}

// uptimeKumaMonitorKnownField 判断 key 是否为 UptimeKumaMonitorConfig 的已知字段（yaml tag）。
func uptimeKumaMonitorKnownField(key string) bool {
	switch key {
	case "name", "type", "url", "interval", "accepted_statuscodes", "maxredirects",
		"expiryNotification", "domainExpiryNotification", "keyword", "hostname", "port", "tags":
		return true
	}
	return false
}

// UnmarshalYAML 自定义解析：已知字段正常解析，未知 key 收集到 Extra（透传语义，不报错）。
func (m *UptimeKumaMonitorConfig) UnmarshalYAML(node *yaml.Node) error {
	type plain UptimeKumaMonitorConfig
	var known plain
	if err := node.Decode(&known); err != nil {
		return err
	}
	*m = UptimeKumaMonitorConfig(known)
	m.Extra = nil

	var raw map[string]any
	if err := node.Decode(&raw); err != nil {
		return err
	}
	for key, val := range raw {
		if !uptimeKumaMonitorKnownField(key) {
			if m.Extra == nil {
				m.Extra = make(map[string]any)
			}
			m.Extra[key] = val
		}
	}
	return nil
}

// MarshalYAML 输出已知字段 + Extra 合并（Extra 键覆盖同名字段）。
// 注意：必须用值接收者——monitors 以值类型出现在 []UptimeKumaMonitorConfig 中，
// yaml.v3 对值元素只会命中值接收者的 Marshaler。
func (m UptimeKumaMonitorConfig) MarshalYAML() (interface{}, error) {
	out := map[string]any{
		"name": m.Name,
		"type": m.Type,
	}
	if m.URL != "" {
		out["url"] = m.URL
	}
	if m.Interval != 0 {
		out["interval"] = m.Interval
	}
	if len(m.AcceptedStatuscodes) > 0 {
		out["accepted_statuscodes"] = m.AcceptedStatuscodes
	}
	if m.MaxRedirects != 0 {
		out["maxredirects"] = m.MaxRedirects
	}
	if m.ExpiryNotification {
		out["expiryNotification"] = m.ExpiryNotification
	}
	if m.DomainExpiryNotification {
		out["domainExpiryNotification"] = m.DomainExpiryNotification
	}
	if m.Keyword != "" {
		out["keyword"] = m.Keyword
	}
	if m.Hostname != "" {
		out["hostname"] = m.Hostname
	}
	if m.Port != 0 {
		out["port"] = m.Port
	}
	if len(m.Tags) > 0 {
		out["tags"] = m.Tags
	}
	for k, v := range m.Extra {
		out[k] = v
	}
	return out, nil
}

// yamlMarshalSafe 包装 yaml.Marshal：对不可序列化的值（如 chan/func）yaml.v3 会 panic，
// 这里统一转成 error 返回。
func yamlMarshalSafe(v any) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("cannot marshal: %v", r)
		}
	}()
	_, err = yaml.Marshal(v)
	return err
}

func (m *UptimeKumaMonitorConfig) Validate() error {
	if m.Name == "" {
		return domain.RequiredField("uptime-kuma monitors name")
	}
	if !uptimeKumaMonitorTypes[m.Type] {
		return fmt.Errorf("%w: uptime-kuma monitor type must be one of %s, got '%s'", domain.ErrInvalidType, uptimeKumaMonitorTypesList(), m.Type)
	}
	if len(m.Extra) > 0 {
		if err := yamlMarshalSafe(m.Extra); err != nil {
			return fmt.Errorf("%w: uptime-kuma monitor '%s' extra fields must be yaml-serializable: %v", domain.ErrInvalidType, m.Name, err)
		}
	}
	switch m.Type {
	case "http", "http-browser", "keyword", "keyword-browser":
		if m.URL == "" {
			return domain.RequiredField("uptime-kuma monitor url")
		}
		if !strings.HasPrefix(m.URL, "http://") && !strings.HasPrefix(m.URL, "https://") {
			return fmt.Errorf("%w: uptime-kuma monitor url must start with http:// or https://", domain.ErrInvalidURL)
		}
		u, err := url.Parse(m.URL)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("%w: uptime-kuma monitor url %q is invalid", domain.ErrInvalidURL, m.URL)
		}
	case "port":
		if m.Hostname == "" {
			return domain.RequiredField("uptime-kuma monitor hostname for port type")
		}
		if m.Port == 0 {
			return fmt.Errorf("uptime-kuma monitor port %w", domain.ErrInvalidPort)
		}
		if err := ValidatePort(m.Port); err != nil {
			return fmt.Errorf("uptime-kuma monitor port %w", err)
		}
	case "ping", "dns":
		if m.Hostname == "" {
			return domain.RequiredField("uptime-kuma monitor hostname for " + m.Type + " type")
		}
	}
	return nil
}

// UptimeKumaConfig uptime-kuma 统一监控平台配置（主容器 + agent 双容器）。
type UptimeKumaConfig struct {
	Admin        UptimeKumaAdminConfig        `yaml:"admin"`
	DB           UptimeKumaDBConfig           `yaml:"db"`
	Gateways     []UptimeKumaGatewayConfig    `yaml:"gateways,omitempty"`
	Sync         UptimeKumaSyncConfig         `yaml:"sync"`
	Notification UptimeKumaNotificationConfig `yaml:"notification,omitempty"`
	Monitors     []UptimeKumaMonitorConfig    `yaml:"monitors,omitempty"`
}

func (c *UptimeKumaConfig) Validate() error {
	if err := c.Admin.Validate(); err != nil {
		return err
	}
	if err := c.DB.Validate(); err != nil {
		return err
	}
	if len(c.Gateways) == 0 {
		return domain.RequiredField("uptime-kuma gateways (at least one)")
	}
	for i, gw := range c.Gateways {
		if err := gw.Validate(); err != nil {
			return fmt.Errorf("uptime-kuma gateways[%d]: %w", i, err)
		}
	}
	if err := c.Sync.Validate(); err != nil {
		return err
	}
	if err := c.Notification.Validate(); err != nil {
		return err
	}
	seen := make(map[string]bool)
	for i, m := range c.Monitors {
		if err := m.Validate(); err != nil {
			return fmt.Errorf("uptime-kuma monitors[%d]: %w", i, err)
		}
		if seen[m.Name] {
			return fmt.Errorf("uptime-kuma monitors[%d].name %q is duplicated", i, m.Name)
		}
		seen[m.Name] = true
	}
	return nil
}

type InfraService struct {
	ServiceBase
	Name  string           `yaml:"name"`
	Type  InfraServiceType `yaml:"type"`
	Image string           `yaml:"image"`

	Secrets []string                         `yaml:"secrets,omitempty"`
	Env     map[string]valueobject.SecretRef `yaml:"env,omitempty"`

	GatewayPorts          *GatewayPorts           `yaml:"ports,omitempty"`
	GatewaySSL            *GatewaySSLConfig       `yaml:"ssl,omitempty"`
	GatewayWAF            *GatewayWAFConfig       `yaml:"waf,omitempty"`
	GatewayBlacklist      *GatewayBlacklistConfig `yaml:"blacklist,omitempty"`
	GatewayTrustedProxies []string                `yaml:"trusted_proxies,omitempty"` // gate 可信代理网段，仅内网 gate 配置 mesh 网段
	// GatewayStrictPathCheck H1（内部PRD）请求路径规范化强制逃生口（gate
	// server.strict_path_check）。nil = 不下发，gate 侧默认开；显式 false 为逃生口（gate 启动 Warn）。
	GatewayStrictPathCheck *bool `yaml:"strict_path_check,omitempty"`
	// GatewayWebSocketInsecureSkip L3（内部PRD）全局 WSS 上游跳过证书校验（gate
	// server.websocket_insecure_skip，Deprecated——gate 侧已迁移至 per-host
	// hosts[].web_socket_insecure_skip，兼容期作 per-host nil 时的缺省值；非零值 gate 启动 Warn）。
	// nil = 不下发（gate 默认 false，恒校验证书）。
	GatewayWebSocketInsecureSkip *bool                        `yaml:"websocket_insecure_skip,omitempty"`
	GatewayLogLevel              int                          `yaml:"log_level,omitempty"`
	GatewayNotification          *GatewayNotification         `yaml:"notification,omitempty"`
	GatewayTrackerShipper        *GatewayTrackerShipperConfig `yaml:"tracker_shipper,omitempty"` // C2 网关通道 shipper（内部PRD），透传给 infra-gate 顶层 tracker_shipper 段

	TunnelServerConfig *TunnelServerConfig `yaml:"tunnel_server,omitempty"`
	TunnelClientConfig *TunnelClientConfig `yaml:"tunnel_client,omitempty"`

	EasyTierConfig *EasyTierConfig `yaml:"easytier,omitempty"`

	UptimeKumaConfig *UptimeKumaConfig `yaml:"uptime_kuma,omitempty"`

	Resources ServiceResources `yaml:"resources,omitempty"`

	Ports []ServicePort `yaml:"extra_ports,omitempty"`
}

type infraServiceAlias InfraService

func (s *InfraService) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw struct {
		Name       string                           `yaml:"name"`
		Type       InfraServiceType                 `yaml:"type"`
		Server     string                           `yaml:"server"`
		Image      string                           `yaml:"image"`
		Networks   []string                         `yaml:"networks"`
		Secrets    []string                         `yaml:"secrets"`
		Env        map[string]valueobject.SecretRef `yaml:"env"`
		Resources  ServiceResources                 `yaml:"resources"`
		ExtraPorts []ServicePort                    `yaml:"extra_ports"`
	}
	if err := unmarshal(&raw); err != nil {
		return err
	}

	s.Name = raw.Name
	s.Type = raw.Type
	s.ServiceBase.Server = raw.Server
	s.ServiceBase.Networks = raw.Networks
	s.Image = raw.Image
	s.Secrets = raw.Secrets
	s.Env = raw.Env
	s.Resources = raw.Resources
	s.Ports = raw.ExtraPorts

	switch s.Type {
	case InfraServiceTypeGateway:
		var gw struct {
			Ports                 *GatewayPorts                `yaml:"ports"`
			SSL                   *GatewaySSLConfig            `yaml:"ssl"`
			WAF                   *GatewayWAFConfig            `yaml:"waf"`
			Blacklist             *GatewayBlacklistConfig      `yaml:"blacklist"`
			TrustedProxies        []string                     `yaml:"trusted_proxies"`
			StrictPathCheck       *bool                        `yaml:"strict_path_check"`
			WebSocketInsecureSkip *bool                        `yaml:"websocket_insecure_skip"`
			LogLevel              int                          `yaml:"log_level"`
			Notification          *GatewayNotification         `yaml:"notification"`
			TrackerShipper        *GatewayTrackerShipperConfig `yaml:"tracker_shipper"`
		}
		if err := unmarshal(&gw); err != nil {
			return err
		}
		s.GatewayPorts = gw.Ports
		s.GatewaySSL = gw.SSL
		s.GatewayWAF = gw.WAF
		s.GatewayBlacklist = gw.Blacklist
		s.GatewayTrustedProxies = gw.TrustedProxies
		s.GatewayStrictPathCheck = gw.StrictPathCheck
		s.GatewayWebSocketInsecureSkip = gw.WebSocketInsecureSkip
		s.GatewayLogLevel = gw.LogLevel
		s.GatewayNotification = gw.Notification
		s.GatewayTrackerShipper = gw.TrackerShipper

	case InfraServiceTypeTunnelServer:
		var ts struct {
			Ports        TunnelServerPorts     `yaml:"ports"`
			AuthToken    valueobject.SecretRef `yaml:"auth_token"`
			TunnelRoutes []TunnelRoute         `yaml:"tunnel_routes"`
		}
		if err := unmarshal(&ts); err != nil {
			return err
		}
		s.TunnelServerConfig = &TunnelServerConfig{
			Ports:        ts.Ports,
			AuthToken:    ts.AuthToken,
			TunnelRoutes: ts.TunnelRoutes,
		}

	case InfraServiceTypeTunnelClient:
		var tc struct {
			ServerAddr string                `yaml:"server_addr"`
			AuthToken  valueobject.SecretRef `yaml:"auth_token"`
			Proxies    []TunnelClientProxy   `yaml:"proxies"`
		}
		if err := unmarshal(&tc); err != nil {
			return err
		}
		s.TunnelClientConfig = &TunnelClientConfig{
			ServerAddr: tc.ServerAddr,
			AuthToken:  tc.AuthToken,
			Proxies:    tc.Proxies,
		}

	case InfraServiceTypeEasyTier:
		var et struct {
			Role          string                `yaml:"role"`
			VirtualIP     string                `yaml:"virtual_ip"`
			NetworkName   string                `yaml:"network_name"`
			NetworkSecret valueobject.SecretRef `yaml:"network_secret"`
			Hostname      string                `yaml:"hostname"`
			Listeners     []string              `yaml:"listeners"`
			Peers         []string              `yaml:"peers"`
			Flags         EasyTierFlags         `yaml:"flags"`
		}
		if err := unmarshal(&et); err != nil {
			return err
		}
		s.EasyTierConfig = &EasyTierConfig{
			Role:          et.Role,
			VirtualIP:     et.VirtualIP,
			NetworkName:   et.NetworkName,
			NetworkSecret: et.NetworkSecret,
			Hostname:      et.Hostname,
			Listeners:     et.Listeners,
			Peers:         et.Peers,
			Flags:         et.Flags,
		}

	case InfraServiceTypeUptimeKuma:
		var uk struct {
			Admin        UptimeKumaAdminConfig        `yaml:"admin"`
			DB           UptimeKumaDBConfig           `yaml:"db"`
			Gateways     []UptimeKumaGatewayConfig    `yaml:"gateways"`
			Sync         UptimeKumaSyncConfig         `yaml:"sync"`
			Notification UptimeKumaNotificationConfig `yaml:"notification"`
			Monitors     []UptimeKumaMonitorConfig    `yaml:"monitors"`
		}
		if err := unmarshal(&uk); err != nil {
			return err
		}
		s.UptimeKumaConfig = &UptimeKumaConfig{
			Admin:        uk.Admin,
			DB:           uk.DB,
			Gateways:     uk.Gateways,
			Sync:         uk.Sync,
			Notification: uk.Notification,
			Monitors:     uk.Monitors,
		}
	}

	return nil
}

func (s *InfraService) MarshalYAML() (interface{}, error) {
	switch s.Type {
	case InfraServiceTypeGateway:
		return struct {
			Name                  string                           `yaml:"name"`
			Type                  InfraServiceType                 `yaml:"type"`
			Server                string                           `yaml:"server"`
			Image                 string                           `yaml:"image"`
			Secrets               []string                         `yaml:"secrets,omitempty"`
			Env                   map[string]valueobject.SecretRef `yaml:"env,omitempty"`
			Ports                 *GatewayPorts                    `yaml:"ports,omitempty"`
			SSL                   *GatewaySSLConfig                `yaml:"ssl,omitempty"`
			WAF                   *GatewayWAFConfig                `yaml:"waf,omitempty"`
			Blacklist             *GatewayBlacklistConfig          `yaml:"blacklist,omitempty"`
			TrustedProxies        []string                         `yaml:"trusted_proxies,omitempty"`
			StrictPathCheck       *bool                            `yaml:"strict_path_check,omitempty"`
			WebSocketInsecureSkip *bool                            `yaml:"websocket_insecure_skip,omitempty"`
			LogLevel              int                              `yaml:"log_level,omitempty"`
			Notification          *GatewayNotification             `yaml:"notification,omitempty"`
			TrackerShipper        *GatewayTrackerShipperConfig     `yaml:"tracker_shipper,omitempty"`
			Resources             ServiceResources                 `yaml:"resources,omitempty"`
			Networks              []string                         `yaml:"networks,omitempty"`
			ExtraPorts            []ServicePort                    `yaml:"extra_ports,omitempty"`
		}{
			Name:                  s.Name,
			Type:                  s.Type,
			Server:                s.ServiceBase.Server,
			Image:                 s.Image,
			Secrets:               s.Secrets,
			Env:                   s.Env,
			Ports:                 s.GatewayPorts,
			SSL:                   s.GatewaySSL,
			WAF:                   s.GatewayWAF,
			Blacklist:             s.GatewayBlacklist,
			TrustedProxies:        s.GatewayTrustedProxies,
			StrictPathCheck:       s.GatewayStrictPathCheck,
			WebSocketInsecureSkip: s.GatewayWebSocketInsecureSkip,
			LogLevel:              s.GatewayLogLevel,
			Notification:          s.GatewayNotification,
			TrackerShipper:        s.GatewayTrackerShipper,
			Resources:             s.Resources,
			Networks:              s.ServiceBase.Networks,
			ExtraPorts:            s.Ports,
		}, nil
	case InfraServiceTypeTunnelServer:
		if s.TunnelServerConfig == nil {
			return (*infraServiceAlias)(s), nil
		}
		return struct {
			Name         string                           `yaml:"name"`
			Type         InfraServiceType                 `yaml:"type"`
			Server       string                           `yaml:"server"`
			Image        string                           `yaml:"image"`
			Secrets      []string                         `yaml:"secrets,omitempty"`
			Env          map[string]valueobject.SecretRef `yaml:"env,omitempty"`
			Ports        TunnelServerPorts                `yaml:"ports,omitempty"`
			AuthToken    valueobject.SecretRef            `yaml:"auth_token"`
			TunnelRoutes []TunnelRoute                    `yaml:"tunnel_routes,omitempty"`
			Networks     []string                         `yaml:"networks,omitempty"`
			ExtraPorts   []ServicePort                    `yaml:"extra_ports,omitempty"`
		}{
			Name:         s.Name,
			Type:         s.Type,
			Server:       s.ServiceBase.Server,
			Image:        s.Image,
			Secrets:      s.Secrets,
			Env:          s.Env,
			Ports:        s.TunnelServerConfig.Ports,
			AuthToken:    s.TunnelServerConfig.AuthToken,
			TunnelRoutes: s.TunnelServerConfig.TunnelRoutes,
			Networks:     s.ServiceBase.Networks,
			ExtraPorts:   s.Ports,
		}, nil
	case InfraServiceTypeTunnelClient:
		if s.TunnelClientConfig == nil {
			return (*infraServiceAlias)(s), nil
		}
		return struct {
			Name       string                           `yaml:"name"`
			Type       InfraServiceType                 `yaml:"type"`
			Server     string                           `yaml:"server"`
			Image      string                           `yaml:"image"`
			Secrets    []string                         `yaml:"secrets,omitempty"`
			Env        map[string]valueobject.SecretRef `yaml:"env,omitempty"`
			ServerAddr string                           `yaml:"server_addr"`
			AuthToken  valueobject.SecretRef            `yaml:"auth_token"`
			Proxies    []TunnelClientProxy              `yaml:"proxies,omitempty"`
			Networks   []string                         `yaml:"networks,omitempty"`
			ExtraPorts []ServicePort                    `yaml:"extra_ports,omitempty"`
		}{
			Name:       s.Name,
			Type:       s.Type,
			Server:     s.ServiceBase.Server,
			Image:      s.Image,
			Secrets:    s.Secrets,
			Env:        s.Env,
			ServerAddr: s.TunnelClientConfig.ServerAddr,
			AuthToken:  s.TunnelClientConfig.AuthToken,
			Proxies:    s.TunnelClientConfig.Proxies,
			Networks:   s.ServiceBase.Networks,
			ExtraPorts: s.Ports,
		}, nil
	case InfraServiceTypeEasyTier:
		if s.EasyTierConfig == nil {
			return (*infraServiceAlias)(s), nil
		}
		return struct {
			Name          string                           `yaml:"name"`
			Type          InfraServiceType                 `yaml:"type"`
			Server        string                           `yaml:"server"`
			Image         string                           `yaml:"image"`
			Secrets       []string                         `yaml:"secrets,omitempty"`
			Env           map[string]valueobject.SecretRef `yaml:"env,omitempty"`
			Role          string                           `yaml:"role"`
			VirtualIP     string                           `yaml:"virtual_ip"`
			NetworkName   string                           `yaml:"network_name"`
			NetworkSecret valueobject.SecretRef            `yaml:"network_secret"`
			Hostname      string                           `yaml:"hostname,omitempty"`
			Listeners     []string                         `yaml:"listeners,omitempty"`
			Peers         []string                         `yaml:"peers,omitempty"`
			Flags         EasyTierFlags                    `yaml:"flags,omitempty"`
			Networks      []string                         `yaml:"networks,omitempty"`
			ExtraPorts    []ServicePort                    `yaml:"extra_ports,omitempty"`
		}{
			Name:          s.Name,
			Type:          s.Type,
			Server:        s.ServiceBase.Server,
			Image:         s.Image,
			Secrets:       s.Secrets,
			Env:           s.Env,
			Role:          s.EasyTierConfig.Role,
			VirtualIP:     s.EasyTierConfig.VirtualIP,
			NetworkName:   s.EasyTierConfig.NetworkName,
			NetworkSecret: s.EasyTierConfig.NetworkSecret,
			Hostname:      s.EasyTierConfig.Hostname,
			Listeners:     s.EasyTierConfig.Listeners,
			Peers:         s.EasyTierConfig.Peers,
			Flags:         s.EasyTierConfig.Flags,
			Networks:      s.ServiceBase.Networks,
			ExtraPorts:    s.Ports,
		}, nil
	case InfraServiceTypeUptimeKuma:
		if s.UptimeKumaConfig == nil {
			return (*infraServiceAlias)(s), nil
		}
		return struct {
			Name         string                           `yaml:"name"`
			Type         InfraServiceType                 `yaml:"type"`
			Server       string                           `yaml:"server"`
			Image        string                           `yaml:"image"`
			Secrets      []string                         `yaml:"secrets,omitempty"`
			Env          map[string]valueobject.SecretRef `yaml:"env,omitempty"`
			Admin        UptimeKumaAdminConfig            `yaml:"admin"`
			DB           UptimeKumaDBConfig               `yaml:"db"`
			Gateways     []UptimeKumaGatewayConfig        `yaml:"gateways,omitempty"`
			Sync         UptimeKumaSyncConfig             `yaml:"sync"`
			Notification UptimeKumaNotificationConfig     `yaml:"notification,omitempty"`
			Monitors     []UptimeKumaMonitorConfig        `yaml:"monitors,omitempty"`
			Networks     []string                         `yaml:"networks,omitempty"`
			ExtraPorts   []ServicePort                    `yaml:"extra_ports,omitempty"`
		}{
			Name:         s.Name,
			Type:         s.Type,
			Server:       s.ServiceBase.Server,
			Image:        s.Image,
			Secrets:      s.Secrets,
			Env:          s.Env,
			Admin:        s.UptimeKumaConfig.Admin,
			DB:           s.UptimeKumaConfig.DB,
			Gateways:     s.UptimeKumaConfig.Gateways,
			Sync:         s.UptimeKumaConfig.Sync,
			Notification: s.UptimeKumaConfig.Notification,
			Monitors:     s.UptimeKumaConfig.Monitors,
			Networks:     s.ServiceBase.Networks,
			ExtraPorts:   s.Ports,
		}, nil
	}
	return (*infraServiceAlias)(s), nil
}

func (s *InfraService) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("%w: infra service name is required", domain.ErrInvalidName)
	}
	if s.Type != InfraServiceTypeGateway &&
		s.Type != InfraServiceTypeTunnelServer &&
		s.Type != InfraServiceTypeTunnelClient &&
		s.Type != InfraServiceTypeEasyTier &&
		s.Type != InfraServiceTypeUptimeKuma {
		return fmt.Errorf("%w: %s", domain.ErrInvalidType, s.Type)
	}
	if s.Server == "" {
		return domain.RequiredField("server")
	}
	if s.Image == "" {
		return domain.RequiredField("image")
	}
	switch s.Type {
	case InfraServiceTypeGateway:
		if s.GatewayPorts != nil {
			if err := s.GatewayPorts.Validate(); err != nil {
				return err
			}
		}
		if s.GatewayNotification != nil {
			if err := s.GatewayNotification.Validate(); err != nil {
				return err
			}
		}
		if s.GatewayWAF != nil {
			if err := s.GatewayWAF.Validate(); err != nil {
				return err
			}
		}
		if s.GatewayBlacklist != nil {
			if err := s.GatewayBlacklist.Validate(); err != nil {
				return err
			}
		}
		if s.GatewayTrackerShipper != nil {
			if err := s.GatewayTrackerShipper.Validate(); err != nil {
				return err
			}
		}
		if err := validateGatewayTrustedProxies(s.GatewayTrustedProxies); err != nil {
			return err
		}
	case InfraServiceTypeTunnelServer:
		if s.TunnelServerConfig == nil {
			return domain.RequiredField("tunnel_server config")
		}
		if err := s.TunnelServerConfig.Validate(); err != nil {
			return err
		}
	case InfraServiceTypeTunnelClient:
		if s.TunnelClientConfig == nil {
			return domain.RequiredField("tunnel_client config")
		}
		if err := s.TunnelClientConfig.Validate(); err != nil {
			return err
		}
	case InfraServiceTypeEasyTier:
		if s.EasyTierConfig == nil {
			return domain.RequiredField("easytier config")
		}
		if err := s.EasyTierConfig.Validate(); err != nil {
			return err
		}
	case InfraServiceTypeUptimeKuma:
		if s.UptimeKumaConfig == nil {
			return domain.RequiredField("uptime_kuma config")
		}
		if err := s.UptimeKumaConfig.Validate(); err != nil {
			return err
		}
	}
	return nil
}
