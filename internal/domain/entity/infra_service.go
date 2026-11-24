package entity

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/lite-lake/infra-yamlops/internal/domain"
	"github.com/lite-lake/infra-yamlops/internal/domain/valueobject"
)

type InfraServiceType string

const (
	InfraServiceTypeGateway  InfraServiceType = "gateway"
	InfraServiceTypeEasyTier InfraServiceType = "easytier"
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
	// AllowInsecureHTTP 明文 http 证书端点显式放行开关
	//（gate ssl.remote.allow_insecure_http，内网 docker 别名端点逃生口）。
	// nil = false：mode=remote 且 endpoint 含明文 http 时 Validate fail-fast
	//（对齐 gate 启动校验，避免部署成功后网关崩溃循环）。
	AllowInsecureHTTP *bool `yaml:"allow_insecure_http,omitempty"`
}

func (s *GatewaySSLConfig) Validate() error {
	if s.Mode != "local" && s.Mode != "remote" {
		return fmt.Errorf("%w: ssl mode must be 'local' or 'remote'", domain.ErrInvalidType)
	}
	if s.Mode == "remote" && s.Endpoint == "" {
		return domain.RequiredField("endpoint for remote ssl mode")
	}
	if s.Mode == "remote" {
		for _, raw := range strings.Split(s.Endpoint, ",") {
			u, err := url.Parse(strings.TrimSpace(raw))
			if err != nil || u.Scheme == "" {
				continue // 畸形端点交由 gate 运行期报错
			}
			if u.Scheme == "http" && (s.AllowInsecureHTTP == nil || !*s.AllowInsecureHTTP) {
				return fmt.Errorf("ssl.endpoint %q uses plaintext http; set ssl.allow_insecure_http: true to allow（F6.1 内网明文逃生口；对齐 gate 启动校验，避免远端崩溃循环）", strings.TrimSpace(raw))
			}
		}
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
	// CRSDegradeAcknowledged 降级双钥匙确认（gate
	// waf.crs.degrade_acknowledged）：crs_failure_policy=degrade 时必须显式 true，
	// 否则实体层 Validate fail-fast（对齐 gate 启动校验，避免部署成功后网关崩溃循环：
	// 旧配置缺确认键会导致容器反复重启）。
	// 非 degrade 或未配置时不生成该键（gate 侧取安全缺省）。
	CRSDegradeAcknowledged bool `yaml:"crs_degrade_acknowledged,omitempty"`
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
	if w.CRSFailurePolicy == "degrade" && !w.CRSDegradeAcknowledged {
		return fmt.Errorf("waf.crs_failure_policy=degrade 需同时显式 waf.crs_degrade_acknowledged: true（承认无 WAF 降级运行；对齐 gate F11 双钥匙启动校验）")
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

	EasyTierConfig *EasyTierConfig `yaml:"easytier,omitempty"`

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
	}
	return (*infraServiceAlias)(s), nil
}

func (s *InfraService) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("%w: infra service name is required", domain.ErrInvalidName)
	}
	if s.Type != InfraServiceTypeGateway &&
		s.Type != InfraServiceTypeEasyTier {
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
	case InfraServiceTypeEasyTier:
		if s.EasyTierConfig == nil {
			return domain.RequiredField("easytier config")
		}
		if err := s.EasyTierConfig.Validate(); err != nil {
			return err
		}
	}
	return nil
}
