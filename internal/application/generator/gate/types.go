package gate

// ForwardedForAppend host 级 forwarded_for 的取值：透传并追加 X-Forwarded-For。
// 由 yamlops 对 mesh 域名 host 自动打标（见 HostRoute.ForwardedFor），用户无需配置。
const ForwardedForAppend = "append"

type NotificationConfig struct {
	Enabled bool
	URL     string
	Timeout string
}

// BlacklistConfig IP 黑名单全局配置；nil = 未配置，不生成 gate 的 blacklist 段（存量配置输出保持不变）
type BlacklistConfig struct {
	Enabled     bool
	DefaultMode string
	Whitelist   []string
	SourceURL   string
	// AllowHTTPSource L6（内部PRD）明文 http 数据源放行开关（gate
	// blacklist.allow_http_source）。nil = 未配置，不生成该键（gate 默认仅 https）。
	AllowHTTPSource *bool
	// MaxEntries L6（内部PRD）防追加式投毒增长上限基数（gate
	// blacklist.max_entries）。0 = 未配置，不生成该键（gate 默认 200000）。
	MaxEntries int
}

// TrackerShipperConfig C2 网关通道 shipper 配置（infra-gate 根 Config.tracker_shipper 顶层段，
// 与 server:/hosts: 平级，内部PRD §7.1）。nil = 未配置，不生成该键（存量输出逐字节不变）。
type TrackerShipperConfig struct {
	// Enabled 总开关：false 时段仍生成（缺省形态），gate 侧不启动 shipper
	Enabled bool
	// Endpoint ingest 端点（mesh 域名 .../api/internal/gateway-ingest）
	Endpoint string
	// SharedSecret 共享密钥明文（{secret: name} 引用已由 generator 层在生成阶段解析，
	// 找不到 secret 直接报错不产出半成品，照 auth.password 先例）
	SharedSecret string
	// NodeName 上报节点名；未配置时由 generator 层自动注入容器命名 yo-{env}-{服务名}
	NodeName string
	// BufferSize 有界缓冲条数；0 = 未配置，generator 层填默认值
	BufferSize int
	// BatchSize 批量发送条数；0 = 未配置，generator 层填默认值
	BatchSize int
	// FlushInterval flush 周期（Go duration 字符串）；空 = 未配置，generator 层填默认值
	FlushInterval string
	// FailureSampleLogRatio 丢弃/失败采样日志比例；0 = 未配置，generator 层填默认值
	//（故无法显式配 0，确需极低采样可配极小正值）
	FailureSampleLogRatio float64
}

// tracker_shipper 缺省值（内部PRD §7.1）：段生成时未配置的字段由 yamlops 显式填入，
// 产物完整自描述，infra-gate 侧不依赖自身兜底。
const (
	DefaultTrackerShipperBufferSize            = 10000
	DefaultTrackerShipperBatchSize             = 500
	DefaultTrackerShipperFlushInterval         = "1s"
	DefaultTrackerShipperFailureSampleLogRatio = 0.01
)

type GatewayConfig struct {
	Port           int
	LogLevel       int
	WAFEnabled     bool
	WAFDefaultMode string
	Whitelist      []string
	CRSProxyURL    string
	CRSMirrorURL   string
	// CRSFailurePolicy CRS 不可用时的失败策略（infra-gate waf.crs.failure_policy，内部PRD §5.4/§5.7）。
	// degrade = 无 WAF 启动 + 高优告警；fail = 进程退出（gate 侧现状默认）。空 = 不生成该键
	// （存量输出逐字节不变，gate 侧按默认 fail 处理）。
	// 版本门槛：旧版 gate 读到 failure_policy 会静默忽略——不报错但仍为 fail 行为，
	// 部署顺序须 gate 新镜像先行（含降级状态机）→ 更新 yamlops → 下发带 crs_failure_policy 的
	// userdata 配置重新生成部署（与 auth/trusted_proxies 的「gate 镜像先行」纪律一致）。
	CRSFailurePolicy string
	// CRSPinnedSHA256 H2（内部PRD）CRS tarball 期望 sha256 pin（gate
	// waf.crs.download.pinned_sha256，最高优先、优先于镜像 pins 清单）。
	// 空 = 不生成该键（gate 侧回退镜像 pins 清单 / 金副本）。格式由实体层 Validate fail-fast。
	CRSPinnedSHA256 string
	// CRSMirrorAllowInsecureHTTP H2（内部PRD）镜像站明文 http 放行开关（gate
	// waf.crs.download.mirror_allow_insecure_http，内网明文镜像逃生口 D6）。
	// nil = 不生成该键（gate 默认仅 https，明文 http 镜像启动校验直接报错）；显式 true 由 gate 启动 Warn。
	CRSMirrorAllowInsecureHTTP *bool
	Blacklist                  *BlacklistConfig
	// StrictPathCheck H1（内部PRD）请求路径规范化强制逃生口（gate
	// server.strict_path_check）。nil = 不生成该键（gate 默认开）；显式 false 为逃生口（gate 启动 Warn）。
	StrictPathCheck *bool
	// WebSocketInsecureSkip L3（内部PRD）全局 WSS 上游跳过证书校验（gate
	// server.websocket_insecure_skip，Deprecated——gate 侧已迁移至 per-host
	// hosts[].web_socket_insecure_skip，兼容期作 per-host nil 时的缺省值）。
	// nil = 不生成该键（gate 默认 false，恒校验证书）；显式 true 由 gate 启动 Warn。
	WebSocketInsecureSkip *bool
	// TrustedProxies gate 的可信代理网段（gate server.trusted_proxies）。
	// 仅内网 gate（mesh 二级链路的第二跳）应配置 mesh 网段（按环境规划的内网 mesh 网段），
	// 公网边缘 gate 必须保持为空（nil/空 = 不生成该键，存量输出不变）。
	// 版本门槛：要求 infra-gate 为 2026-09-09 之后构建的新镜像（commit a5b9349，gate 代码无自身版本号，以构建日期/提交为准）（旧版 gate 配置后 allowed_ips 仍按解析 IP 评估，
	// 会导致 mesh-only 路由全员 403），部署顺序 gate 镜像先行、配置后下（内部PRD §6.1）。
	TrustedProxies []string
	SSLMode        string
	SSLEndpoint    string
	SSLAPIKey      string
	Notification   *NotificationConfig
	// TrackerShipper C2 网关通道 shipper 段（infra-gate 根 Config.tracker_shipper，内部PRD §7.1）。
	// nil = 未配置，不生成该键（存量输出逐字节不变）。
	// 版本门槛（强制）：旧版 gate 读到该段静默忽略——不报错但 shipper 不工作。
	// 部署顺序：先发布含 shipper 的新版 infra-gate 镜像 → 更新 yamlops →
	// 下发带 tracker_shipper 的 userdata 配置重新生成部署（auth/trusted_proxies 的 gate 镜像先行纪律）。
	TrackerShipper *TrackerShipperConfig
}

type HostRoute struct {
	Name                string
	Port                int
	SSLPort             int
	Backend             []string
	HealthCheck         string
	HealthCheckInterval string
	HealthCheckTimeout  string
	HealthCheckEnabled  *bool // nil = 默认启用，false = 禁用
	PreserveHostHeader  bool
	GZipEnabled         *bool // 使用指针以支持"未设置"状态
	// OverrideHost 覆盖发给上游的 Host 头。用户显式配置的 ServiceGatewayRoute.override_host
	// 优先；external_backends 服务未显式配置时由生成器自动填充 external_backends URL 的 host
	// （v1.2，见 gateway.go 的 buildHostRoute / externalBackendsOverrideHost）。
	OverrideHost         string
	StripProxyHeaders    *bool    // 是否剥离代理头部，nil = 默认不剥离
	RobotsDenyAll        *bool    // 强制覆盖 /robots.txt 拒绝所有爬虫，nil = 不覆盖
	AllowedIPs           []string // 仅允许这些来源 IP/CIDR 访问该 host，nil = 不限制
	WAFMode              string   // 空 = 继承全局 default_mode
	WAFRequestBodyAccess *bool    // nil = 继承全局，false = 跳过请求体扫描
	BlacklistMode        string   // 空 = 继承全局 default_mode
	// ForwardedFor mesh 域名 host 的 XFF 策略（gate HostConfig.forwarded_for）。
	// 当前唯一取值 ForwardedForAppend（透传并追加 XFF）：yamlops 生成时对 hostname 匹配
	// mesh 域名后缀 .{节点}.v-{env}.{zone}（zone 见 mesh.yaml 的 mesh.domain_zone）的 host 自动打标，用户无需在 services_biz.yaml
	// 配置。非 mesh host 保持为空（omitempty 不生成该键），gate 对未标记 host 恒为边缘清理。
	// 版本门槛：要求 infra-gate 为 2026-09-09 之后含 forwarded_for 支持的镜像；旧镜像忽略该键、
	// 行为不变（无顺序约束）。gate 侧 fail-safe：标记 host 的直连对端不在 server.trusted_proxies
	// 网段时自动回退边缘清理，因此在任何 gate 上打标都安全（内部PRD v1.1）。
	ForwardedFor string
	// AuthPassword 密码门密码（secret 引用已在 buildHostRoute 解析为明文）。
	// 空 = 该 host 不启用密码门（不生成 auth 段，存量输出逐字节不变）。
	// 版本门槛（强制）：旧版 gate 读到 auth 段静默忽略（不报错但不拦截），
	// 部署顺序 gate 镜像先行 → 更新 yamlops → 下发带 auth 的 userdata 配置（内部PRD §11）。
	AuthPassword string
	// AuthTTL 登录态有效期（Go duration 字符串），空 = gate 侧默认 24h
	AuthTTL string
	// AuthPublicPaths 免认证路径前缀，nil = 无豁免
	AuthPublicPaths []string
	// AuthMaxFailures 登录失败锁定阈值（M3，内部PRD，gate auth.max_failures）。
	// 0 = 不生成该键（gate 侧取缺省 5，与既有现值一致）。
	AuthMaxFailures int
	// AuthLockDuration 登录失败锁定时长（M3，gate auth.lock_duration，Go duration 字符串）。
	// 空 = 不生成该键（gate 侧取缺省 15m）。
	AuthLockDuration string
	// RedirectHTTPS 明文 HTTP 监听上收到请求时不代理，infra-gate 直接
	// 301 → https://<原 host><原始 RequestURI>（内部总体方案 W3-2 / 决策 D8：
	// 逐条目跳转语义，不做 infra-gate 全局 80 端口行为变更）。
	// nil = false（不生成该键，存量输出逐字节不变）。
	// 版本门槛（强制）：旧版 infra-gate 对未知 host yaml 键静默忽略——下发不报错但不生效，
	// 部署顺序必须 gate 镜像先行 → 更新 yamlops → 下发带 redirect_https 的 userdata 配置
	// 重新生成部署（与 auth/trusted_proxies 的「gate 镜像先行」纪律一致）。
	RedirectHTTPS *bool
	// RedirectHost 规范域收敛：该 host（http 与 https 都算）请求 Host（剥端口后）≠ 本值时，
	// infra-gate 301 → https://<本值><原始 RequestURI>，用于 www/别名域收敛到裸域/规范域。
	// 空 = 不启用（不生成该键）。版本门槛同 redirect_https（旧 gate 静默忽略不生效）。
	RedirectHost string
	// HSTS 响应头 per-host 覆盖：nil = gate 默认 max-age=31536000; includeSubDomains; preload
	// （现状）；"off"（大小写不敏感）或 "" = 不下发；其他值 = 按字面值下发。用于双层网关链路
	// 只让边缘层下发一次。nil 不生成该键。版本门槛同 redirect_https（旧 gate 静默忽略不生效）。
	HSTS *string
	// WebSocketInsecureSkip L3（内部PRD）per-host WSS 上游跳过证书校验（gate
	// hosts[].web_socket_insecure_skip）。nil = 不生成该键，gate 侧继承全局
	// server.websocket_insecure_skip（兼容期）；per-host 显式配置优先。
	// 版本门槛：需相应安全修复版本的 infra-gate 镜像（16 网关均已就位，无顺序约束）。
	WebSocketInsecureSkip *bool
}
