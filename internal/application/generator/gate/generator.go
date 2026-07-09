package gate

import (
	"github.com/lite-lake/infra-yamlops/internal/constants"
	"gopkg.in/yaml.v3"
)

type Generator struct{}

func NewGenerator() *Generator {
	return &Generator{}
}

type serverConfig struct {
	Port         int  `yaml:"port"`
	GZipEnabled  bool `yaml:"g_zip_enabled"`
	HTTP2Enabled bool `yaml:"http2_enabled"`
}

type loggerConfig struct {
	Level         int    `yaml:"level"`
	EnableConsole bool   `yaml:"enable_console"`
	EnableFile    bool   `yaml:"enable_file"`
	LogDir        string `yaml:"log_dir,omitempty"`
}

type wafConfig struct {
	Enabled     bool `yaml:"enabled"`
	DefaultMode string `yaml:"default_mode,omitempty"`
	Whitelist   struct {
		Enabled  bool     `yaml:"enabled"`
		IPRanges []string `yaml:"ip_ranges,omitempty"`
	} `yaml:"whitelist"`
	CRS struct {
		Enabled bool   `yaml:"enabled"`
		Version string `yaml:"version"`
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

type hostWAFConfig struct {
	DefaultMode string `yaml:"default_mode,omitempty"`
}

type sslConfig struct {
	Remote struct {
		Enabled           bool   `yaml:"enabled"`
		Endpoint          string `yaml:"endpoint"`
		AutoUpdate        bool   `yaml:"auto_update"`
		UpdateCheckWindow string `yaml:"update_check_window,omitempty"`
		APIKey            string `yaml:"api_key,omitempty"`
	} `yaml:"remote"`
}

type hostConfig struct {
	Name                string         `yaml:"name"`
	Port                int            `yaml:"port"`
	SSLPort             int            `yaml:"ssl_port,omitempty"`
	Backend             []string       `yaml:"backend"`
	HealthCheck         string         `yaml:"health_check,omitempty"`
	HealthCheckInterval string         `yaml:"health_check_interval,omitempty"`
	HealthCheckTimeout  string         `yaml:"health_check_timeout,omitempty"`
	HealthCheckEnabled  *bool          `yaml:"health_check_enabled,omitempty"`
	PreserveHostHeader  bool           `yaml:"preserve_host_header"`
	GZipEnabled         *bool          `yaml:"g_zip_enabled,omitempty"`
	OverrideHost        string         `yaml:"override_host,omitempty"`
	StripProxyHeaders   *bool          `yaml:"strip_proxy_headers,omitempty"`
	WAF                 *hostWAFConfig `yaml:"waf,omitempty"`
}

type gateConfig struct {
	Server       serverConfig       `yaml:"server"`
	Logger       loggerConfig       `yaml:"logger"`
	WAF          wafConfig          `yaml:"waf"`
	SSL          sslConfig          `yaml:"ssl"`
	Notification notificationConfig `yaml:"notification,omitempty"`
	Hosts        []hostConfig       `yaml:"hosts"`
}

type notificationConfig struct {
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url"`
	Timeout string `yaml:"timeout,omitempty"`
}

func (g *Generator) Generate(cfg *GatewayConfig, hosts []HostRoute) (string, error) {
	sslEnabled := cfg.SSLMode == "remote"

	config := gateConfig{
		Server: serverConfig{
			Port:         cfg.Port,
			GZipEnabled:  true,
			HTTP2Enabled: true,
		},
		Logger: loggerConfig{
			Level:         cfg.LogLevel,
			EnableConsole: true,
			EnableFile:    true,
			LogDir:        constants.DefaultLogDir,
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
				Enabled bool   `yaml:"enabled"`
				Version string `yaml:"version"`
			}{
				Enabled: cfg.WAFEnabled,
				Version: constants.DefaultCRSVersion,
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
			}{},
		},
		SSL: sslConfig{
			Remote: struct {
				Enabled           bool   `yaml:"enabled"`
				Endpoint          string `yaml:"endpoint"`
				AutoUpdate        bool   `yaml:"auto_update"`
				UpdateCheckWindow string `yaml:"update_check_window,omitempty"`
				APIKey            string `yaml:"api_key,omitempty"`
			}{
				Enabled:           sslEnabled,
				Endpoint:          cfg.SSLEndpoint,
				AutoUpdate:        true,
				UpdateCheckWindow: "00:00-00:59",
				APIKey:            cfg.SSLAPIKey,
			},
		},
		Hosts: make([]hostConfig, 0, len(hosts)),
	}

	// Map notification config if provided
	if cfg.Notification != nil {
		config.Notification = notificationConfig{
			Enabled: cfg.Notification.Enabled,
			URL:     cfg.Notification.URL,
			Timeout: cfg.Notification.Timeout,
		}
	}

	for _, h := range hosts {
		host := hostConfig{
			Name:                h.Name,
			Port:                h.Port,
			SSLPort:             h.SSLPort,
			Backend:             h.Backend,
			HealthCheck:         h.HealthCheck,
			HealthCheckInterval: h.HealthCheckInterval,
			HealthCheckTimeout:  h.HealthCheckTimeout,
			HealthCheckEnabled:  h.HealthCheckEnabled,
			PreserveHostHeader:  true,
			GZipEnabled:         h.GZipEnabled,
			OverrideHost:        h.OverrideHost,
			StripProxyHeaders:   h.StripProxyHeaders,
		}
		if h.WAFMode != "" {
			host.WAF = &hostWAFConfig{DefaultMode: h.WAFMode}
		}
		config.Hosts = append(config.Hosts, host)
	}

	data, err := yaml.Marshal(&config)
	if err != nil {
		return "", err
	}

	return string(data), nil
}
