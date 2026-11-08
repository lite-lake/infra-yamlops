package gate

import (
	"strings"

	"github.com/lite-lake/infra-yamlops/internal/constants"
	"gopkg.in/yaml.v3"
)

// splitCommaSeparated 将逗号分隔的字符串拆分为切片，自动去除空白项
func splitCommaSeparated(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

const (
	defaultGZipLevel           = 6
	defaultGZipMinSize         = 0
	defaultArchiveDir          = "./applogs/archive"
	defaultMaxRetentionDays    = 30
	defaultCRSDownloadTimeout  = 60
	defaultCRSRetryCount       = 2 // CRS 下载轮数（内部PRD §5.3：gate 新版将 timeout 当总预算、retry_count 当轮数）
	defaultBlacklistSourceName = "ustc"
	defaultBlacklistSourceURL  = "https://blackip.ustc.edu.cn/list.php?txt"
)

type Generator struct{}

func NewGenerator() *Generator {
	return &Generator{}
}

type serverConfig struct {
	Port int `yaml:"port"`
	// TrustedProxies 可信代理网段（infra-gate server.trusted_proxies）。
	// omitempty：nil/空 = 未配置，不生成该键（保持存量输出逐字节不变）。
	// 仅内网 gate 应配置 mesh 网段；要求 infra-gate ≥ v1.0.18 对应的 gate 新版
	// （旧版 gate 配置后 allowed_ips 仍按解析 IP 评估，mesh-only 路由会 403）。
	TrustedProxies []string `yaml:"trusted_proxies,omitempty"`
	// StrictPathCheck H1（内部PRD）请求路径规范化强制逃生口（infra-gate
	// server.strict_path_check）。omitempty：nil = 未配置不生成该键（gate 默认开），
	// 显式 false 为逃生口（gate 启动 Warn）。
	StrictPathCheck *bool `yaml:"strict_path_check,omitempty"`
	// WebSocketInsecureSkip L3（内部PRD）全局 WSS 上游跳过证书校验（infra-gate
	// server.websocket_insecure_skip，Deprecated——gate 侧已迁移至 per-host
	// hosts[].web_socket_insecure_skip）。omitempty：nil = 未配置不生成该键（gate 默认 false）。
	WebSocketInsecureSkip *bool `yaml:"websocket_insecure_skip,omitempty"`
	GZipEnabled           bool  `yaml:"g_zip_enabled"`
	GZipLevel             int   `yaml:"g_zip_level"`
	GZipMinSize           int   `yaml:"g_zip_min_size"`
	HTTP2Enabled          bool  `yaml:"http2_enabled"`
}

type loggerConfig struct {
	Level            int    `yaml:"level"`
	EnableConsole    bool   `yaml:"enable_console"`
	EnableFile       bool   `yaml:"enable_file"`
	LogDir           string `yaml:"log_dir"`
	ArchiveDir       string `yaml:"archive_dir"`
	MaxRetentionDays int    `yaml:"max_retention_days"`
	EnableColor      bool   `yaml:"enable_color"`
}

type wafConfig struct {
	Enabled     bool   `yaml:"enabled"`
	DefaultMode string `yaml:"default_mode,omitempty"`
	Whitelist   struct {
		Enabled  bool     `yaml:"enabled"`
		IPRanges []string `yaml:"ip_ranges,omitempty"`
	} `yaml:"whitelist"`
	CRS struct {
		Enabled bool   `yaml:"enabled"`
		Version string `yaml:"version"`
		// FailurePolicy CRS 不可用时的失败策略（内部PRD §5.4）：degrade = 无 WAF 启动 +
		// 高优告警；fail = 进程退出（gate 侧现状默认）。omitempty：未配置不生成该键（存量输出
		// 逐字节不变）。版本门槛：旧版 gate 读到该键静默忽略——不报错但仍为 fail 行为，
		// 部署顺序须 gate 新镜像先行（同 auth/trusted_proxies 纪律）。
		FailurePolicy string               `yaml:"failure_policy,omitempty"`
		Download      wafCRSDownloadConfig `yaml:"download"`
	} `yaml:"crs"`
	Custom struct {
		Enabled   bool   `yaml:"enabled"`
		RulesFile string `yaml:"rules_file,omitempty"`
	} `yaml:"custom"`
	Debug struct {
		LogLevel int    `yaml:"log_level,omitempty"`
		LogFile  string `yaml:"log_file,omitempty"`
	} `yaml:"debug,omitempty"`
}

type wafCRSDownloadConfig struct {
	Timeout int `yaml:"timeout"`
	// RetryCount CRS 下载轮数，恒输出（默认 defaultCRSRetryCount=2，与 Timeout 恒输出风格一致；
	// 内部PRD §5.3：gate 新版将 timeout 当总预算、retry_count 当轮数）。
	// 版本门槛：旧版 gate 读到该键静默忽略（不报错，行为不变）。
	RetryCount int `yaml:"retry_count"`
	// PinnedSHA256 H2（内部PRD）CRS tarball 期望 sha256 pin（infra-gate
	// waf.crs.download.pinned_sha256，最高优先、优先于镜像 pins 清单）。
	// omitempty：空 = 未配置不生成该键（gate 侧回退镜像 pins 清单 / 金副本）。
	PinnedSHA256 string `yaml:"pinned_sha256,omitempty"`
	// MirrorAllowInsecureHTTP H2（内部PRD）镜像站明文 http 放行开关（infra-gate
	// waf.crs.download.mirror_allow_insecure_http，内网明文镜像逃生口 D6）。
	// omitempty：nil = 未配置不生成该键（gate 默认仅 https，明文 http 镜像启动校验直接报错）。
	MirrorAllowInsecureHTTP *bool             `yaml:"mirror_allow_insecure_http,omitempty"`
	Proxy                   wafCRSProxyConfig `yaml:"proxy"`
	MirrorURL               []string          `yaml:"mirror_url,omitempty"`
}

type wafCRSProxyConfig struct {
	Enabled  bool   `yaml:"enabled"`
	URL      string `yaml:"url,omitempty"`
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
}

type hostWAFConfig struct {
	DefaultMode       string `yaml:"default_mode,omitempty"`
	RequestBodyAccess *bool  `yaml:"request_body_access,omitempty"`
}

type blacklistSourceConfig struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
}

// blacklistConfig gate 全局黑名单段。不生成 update_interval/timeout/ignore_private_ips：
// gate 侧默认 1h/60s/true 已满足需求。
type blacklistConfig struct {
	Enabled     bool                    `yaml:"enabled"`
	DefaultMode string                  `yaml:"default_mode,omitempty"`
	Sources     []blacklistSourceConfig `yaml:"sources"`
	Whitelist   []string                `yaml:"whitelist,omitempty"`
	// AllowHTTPSource L6（内部PRD）明文 http 数据源放行开关（infra-gate
	// blacklist.allow_http_source）。omitempty：nil = 未配置不生成该键（gate 默认仅 https，
	// 明文 http 源启动校验直接报错；内网明文 feed 逃生口显式 true）。
	AllowHTTPSource *bool `yaml:"allow_http_source,omitempty"`
	// MaxEntries L6（内部PRD）防追加式投毒增长上限基数（infra-gate
	// blacklist.max_entries）。omitempty：0 = 未配置不生成该键（gate 默认 200000）。
	MaxEntries int `yaml:"max_entries,omitempty"`
}

type hostBlacklistConfig struct {
	DefaultMode string `yaml:"default_mode,omitempty"`
}

// hostAuthConfig 密码门段（infra-gate HostConfig.auth，内部PRD）。
// 整段 omitempty + 实体层 password 必填：未启用 auth 的 host 不生成该键
// （存量输出逐字节不变），启用的 host password 恒非空。
type hostAuthConfig struct {
	Password    string   `yaml:"password"`
	TTL         string   `yaml:"ttl,omitempty"`
	PublicPaths []string `yaml:"public_paths,omitempty"`
	// MaxFailures 登录失败锁定阈值（M3，内部PRD，infra-gate auth.max_failures）。
	// omitempty：0 = 未配置不生成该键（gate 侧取缺省 5，与既有现值一致）。
	MaxFailures int `yaml:"max_failures,omitempty"`
	// LockDuration 登录失败锁定时长（M3，infra-gate auth.lock_duration）。
	// omitempty：空 = 未配置不生成该键（gate 侧取缺省 15m）。
	LockDuration string `yaml:"lock_duration,omitempty"`
}

type sslConfig struct {
	Remote struct {
		Enabled           bool     `yaml:"enabled"`
		Endpoint          []string `yaml:"endpoint,omitempty"`
		AutoUpdate        bool     `yaml:"auto_update"`
		UpdateCheckWindow string   `yaml:"update_check_window,omitempty"`
		APIKey            string   `yaml:"api_key,omitempty"`
	} `yaml:"remote"`
}

type hostConfig struct {
	Name                string   `yaml:"name"`
	Port                int      `yaml:"port"`
	SSLPort             int      `yaml:"ssl_port,omitempty"`
	Backend             []string `yaml:"backend"`
	HealthCheck         string   `yaml:"health_check,omitempty"`
	HealthCheckInterval string   `yaml:"health_check_interval,omitempty"`
	HealthCheckTimeout  string   `yaml:"health_check_timeout,omitempty"`
	HealthCheckEnabled  *bool    `yaml:"health_check_enabled,omitempty"`
	PreserveHostHeader  *bool    `yaml:"preserve_host_header,omitempty"`
	GZipEnabled         *bool    `yaml:"g_zip_enabled,omitempty"`
	OverrideHost        string   `yaml:"override_host,omitempty"`
	StripProxyHeaders   *bool    `yaml:"strip_proxy_headers,omitempty"`
	RobotsDenyAll       *bool    `yaml:"robots_deny_all,omitempty"`
	AllowedIPs          []string `yaml:"allowed_ips,omitempty"`
	// ForwardedFor mesh 域名 host 的 XFF 策略（infra-gate HostConfig.forwarded_for）。
	// omitempty：仅 mesh 域名 host 生成 `forwarded_for: append`（yamlops 自动打标），
	// 非 mesh host 不生成该键（保持存量输出不变，gate 对未标记 host 恒为边缘清理）。
	ForwardedFor string `yaml:"forwarded_for,omitempty"`
	// RedirectHTTPS 明文 HTTP 监听 301 → https（内部总体方案 W3-2 / 决策 D8 逐条目
	// 跳转语义）。omitempty：未配置不生成该键（存量输出逐字节不变）。
	// 版本门槛（强制）：旧版 infra-gate 静默忽略未知键——不报错但不生效，部署顺序 gate 镜像先行
	// → 更新 yamlops → 下发带该键的 userdata 配置（同 auth/trusted_proxies 纪律）。
	RedirectHTTPS *bool `yaml:"redirect_https,omitempty"`
	// RedirectHost 规范域收敛目标（www/别名域 → 裸域/规范域，http 与 https 都生效）。
	// omitempty：空 = 不启用，不生成该键。版本门槛同 redirect_https。
	RedirectHost string `yaml:"redirect_host,omitempty"`
	// HSTS 响应头 per-host 覆盖：nil = gate 默认值（现状）；"off"/"" = 不下发；其他 = 字面值。
	// omitempty：未配置不生成该键。版本门槛同 redirect_https。
	HSTS *string `yaml:"hsts,omitempty"`
	// WebSocketInsecureSkip L3（内部PRD）per-host WSS 上游跳过证书校验（infra-gate
	// hosts[].web_socket_insecure_skip）。omitempty：nil = 未配置不生成该键，gate 侧继承全局
	// server.websocket_insecure_skip（兼容期）；per-host 显式配置优先。
	WebSocketInsecureSkip *bool                `yaml:"web_socket_insecure_skip,omitempty"`
	WAF                   *hostWAFConfig       `yaml:"waf,omitempty"`
	Blacklist             *hostBlacklistConfig `yaml:"blacklist,omitempty"`
	Auth                  *hostAuthConfig      `yaml:"auth,omitempty"`
}

type gateConfig struct {
	Server         serverConfig          `yaml:"server"`
	Logger         loggerConfig          `yaml:"logger"`
	WAF            wafConfig             `yaml:"waf"`
	Blacklist      *blacklistConfig      `yaml:"blacklist,omitempty"`
	SSL            *sslConfig            `yaml:"ssl,omitempty"`
	Notification   *notificationConfig   `yaml:"notification,omitempty"`
	TrackerShipper *trackerShipperConfig `yaml:"tracker_shipper,omitempty"`
	Hosts          []hostConfig          `yaml:"hosts"`
}

type notificationConfig struct {
	Enabled bool     `yaml:"enabled"`
	URL     []string `yaml:"url,omitempty"`
	Timeout string   `yaml:"timeout,omitempty"`
}

// trackerShipperConfig C2 网关通道 shipper 段（infra-gate 根 Config.tracker_shipper，内部PRD §7.1）。
// 整段 omitempty：nil = 未配置，不生成该键（存量输出逐字节不变）。
// 段内字段一律显式输出：缺省值由 generator 层（generator/gateway.go 的 buildTrackerShipperConfig）
// 填入，产物完整自描述。
// 版本门槛（强制）：旧版 gate 读到该段静默忽略——不报错但 shipper 不工作。
// 部署顺序：先发布含 shipper 的新版 infra-gate 镜像 → 更新 yamlops → 下发带 tracker_shipper 的
// userdata 配置重新生成部署（与 auth/trusted_proxies 的「gate 镜像先行」纪律一致）。
type trackerShipperConfig struct {
	Enabled               bool    `yaml:"enabled"`
	Endpoint              string  `yaml:"endpoint"`
	SharedSecret          string  `yaml:"shared_secret"`
	NodeName              string  `yaml:"node_name"`
	BufferSize            int     `yaml:"buffer_size"`
	BatchSize             int     `yaml:"batch_size"`
	FlushInterval         string  `yaml:"flush_interval"`
	FailureSampleLogRatio float64 `yaml:"failure_sample_log_ratio"`
}

func (g *Generator) Generate(cfg *GatewayConfig, hosts []HostRoute) (string, error) {
	sslEnabled := cfg.SSLMode == "remote"

	config := gateConfig{
		Server: serverConfig{
			Port:                  cfg.Port,
			TrustedProxies:        cfg.TrustedProxies, // nil/空 = 未配置，omitempty 不生成该键
			StrictPathCheck:       cfg.StrictPathCheck,
			WebSocketInsecureSkip: cfg.WebSocketInsecureSkip,
			GZipEnabled:           true,
			GZipLevel:             defaultGZipLevel,
			GZipMinSize:           defaultGZipMinSize,
			HTTP2Enabled:          true,
		},
		Logger: loggerConfig{
			Level:            cfg.LogLevel,
			EnableConsole:    true,
			EnableFile:       true,
			LogDir:           constants.DefaultLogDir,
			ArchiveDir:       defaultArchiveDir,
			MaxRetentionDays: defaultMaxRetentionDays,
			EnableColor:      true,
		},
		WAF: wafConfig{
			Enabled:     cfg.WAFEnabled,
			DefaultMode: cfg.WAFDefaultMode,
			Whitelist: struct {
				Enabled  bool     `yaml:"enabled"`
				IPRanges []string `yaml:"ip_ranges,omitempty"`
			}{
				Enabled:  cfg.WAFEnabled && len(cfg.Whitelist) > 0,
				IPRanges: cfg.Whitelist,
			},
			CRS: struct {
				Enabled       bool                 `yaml:"enabled"`
				Version       string               `yaml:"version"`
				FailurePolicy string               `yaml:"failure_policy,omitempty"`
				Download      wafCRSDownloadConfig `yaml:"download"`
			}{
				Enabled:       cfg.WAFEnabled,
				Version:       constants.DefaultCRSVersion,
				FailurePolicy: cfg.CRSFailurePolicy, // 空 = 未配置，omitempty 不生成该键
				Download: wafCRSDownloadConfig{
					Timeout:                 defaultCRSDownloadTimeout,
					RetryCount:              defaultCRSRetryCount,
					PinnedSHA256:            cfg.CRSPinnedSHA256,            // 空 = 未配置，omitempty 不生成该键
					MirrorAllowInsecureHTTP: cfg.CRSMirrorAllowInsecureHTTP, // nil = 未配置，omitempty 不生成该键
					Proxy: wafCRSProxyConfig{
						Enabled: cfg.CRSProxyURL != "",
						URL:     cfg.CRSProxyURL,
					},
					MirrorURL: splitCommaSeparated(cfg.CRSMirrorURL),
				},
			},
			Custom: struct {
				Enabled   bool   `yaml:"enabled"`
				RulesFile string `yaml:"rules_file,omitempty"`
			}{
				Enabled: false,
			},
			Debug: struct {
				LogLevel int    `yaml:"log_level,omitempty"`
				LogFile  string `yaml:"log_file,omitempty"`
			}{
				LogLevel: 0,
				LogFile:  "/dev/stdout",
			},
		},
		Hosts: make([]hostConfig, 0, len(hosts)),
	}

	if sslEnabled {
		config.SSL = &sslConfig{
			Remote: struct {
				Enabled           bool     `yaml:"enabled"`
				Endpoint          []string `yaml:"endpoint,omitempty"`
				AutoUpdate        bool     `yaml:"auto_update"`
				UpdateCheckWindow string   `yaml:"update_check_window,omitempty"`
				APIKey            string   `yaml:"api_key,omitempty"`
			}{
				Enabled:           true,
				Endpoint:          splitCommaSeparated(cfg.SSLEndpoint),
				AutoUpdate:        true,
				UpdateCheckWindow: "00:00-00:59",
				APIKey:            cfg.SSLAPIKey,
			},
		}
	}

	if cfg.Notification != nil {
		config.Notification = &notificationConfig{
			Enabled: cfg.Notification.Enabled,
			URL:     splitCommaSeparated(cfg.Notification.URL),
			Timeout: cfg.Notification.Timeout,
		}
	}

	// tracker_shipper 顶层段（内部PRD §7.1）：nil = 未配置，不生成该键
	// （保持存量输出逐字节不变）；非 nil 时整段完整显式输出
	if cfg.TrackerShipper != nil {
		config.TrackerShipper = &trackerShipperConfig{
			Enabled:               cfg.TrackerShipper.Enabled,
			Endpoint:              cfg.TrackerShipper.Endpoint,
			SharedSecret:          cfg.TrackerShipper.SharedSecret,
			NodeName:              cfg.TrackerShipper.NodeName,
			BufferSize:            cfg.TrackerShipper.BufferSize,
			BatchSize:             cfg.TrackerShipper.BatchSize,
			FlushInterval:         cfg.TrackerShipper.FlushInterval,
			FailureSampleLogRatio: cfg.TrackerShipper.FailureSampleLogRatio,
		}
	}

	// Blacklist 全局段：nil = 未配置，不生成（保持存量输出不变）
	if cfg.Blacklist != nil {
		sourceURL := cfg.Blacklist.SourceURL
		if sourceURL == "" {
			sourceURL = defaultBlacklistSourceURL
		}
		config.Blacklist = &blacklistConfig{
			Enabled:     cfg.Blacklist.Enabled,
			DefaultMode: cfg.Blacklist.DefaultMode,
			Sources: []blacklistSourceConfig{
				{Name: defaultBlacklistSourceName, URL: sourceURL},
			},
			Whitelist:       cfg.Blacklist.Whitelist,
			AllowHTTPSource: cfg.Blacklist.AllowHTTPSource, // nil = 未配置，omitempty 不生成该键
			MaxEntries:      cfg.Blacklist.MaxEntries,      // 0 = 未配置，omitempty 不生成该键
		}
	}

	for _, h := range hosts {
		preserveHost := h.PreserveHostHeader
		host := hostConfig{
			Name:                  h.Name,
			Port:                  h.Port,
			SSLPort:               h.SSLPort,
			Backend:               h.Backend,
			HealthCheck:           h.HealthCheck,
			HealthCheckInterval:   h.HealthCheckInterval,
			HealthCheckTimeout:    h.HealthCheckTimeout,
			HealthCheckEnabled:    h.HealthCheckEnabled,
			PreserveHostHeader:    &preserveHost,
			GZipEnabled:           h.GZipEnabled,
			OverrideHost:          h.OverrideHost,
			StripProxyHeaders:     h.StripProxyHeaders,
			RobotsDenyAll:         h.RobotsDenyAll,
			AllowedIPs:            h.AllowedIPs,
			ForwardedFor:          h.ForwardedFor, // 空 = 非 mesh host，omitempty 不生成该键
			RedirectHTTPS:         h.RedirectHTTPS,
			RedirectHost:          h.RedirectHost,
			HSTS:                  h.HSTS,
			WebSocketInsecureSkip: h.WebSocketInsecureSkip, // nil = 未配置，omitempty 不生成该键（gate 继承全局）
		}
		if h.WAFMode != "" || h.WAFRequestBodyAccess != nil {
			host.WAF = &hostWAFConfig{
				DefaultMode:       h.WAFMode,
				RequestBodyAccess: h.WAFRequestBodyAccess,
			}
		}
		if h.BlacklistMode != "" {
			host.Blacklist = &hostBlacklistConfig{
				DefaultMode: h.BlacklistMode,
			}
		}
		if h.AuthPassword != "" {
			// 密码门段：password 必填（实体 Validate 已保证），空 = 不启用不生成
			host.Auth = &hostAuthConfig{
				Password:     h.AuthPassword,
				TTL:          h.AuthTTL,
				PublicPaths:  h.AuthPublicPaths,
				MaxFailures:  h.AuthMaxFailures,  // 0 = 未配置，omitempty 不生成该键
				LockDuration: h.AuthLockDuration, // 空 = 未配置，omitempty 不生成该键
			}
		}
		config.Hosts = append(config.Hosts, host)
	}

	data, err := yaml.Marshal(&config)
	if err != nil {
		return "", err
	}

	return string(data), nil
}
