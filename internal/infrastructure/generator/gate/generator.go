package gate

import (
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
	Enabled   bool `yaml:"enabled"`
	Whitelist struct {
		IPRanges []string `yaml:"ip_ranges"`
	} `yaml:"whitelist"`
	CRS struct {
		Enabled bool   `yaml:"enabled"`
		Version string `yaml:"version"`
	} `yaml:"crs"`
}

type sslConfig struct {
	Remote struct {
		Enabled           bool   `yaml:"enabled"`
		Endpoint          string `yaml:"endpoint"`
		AutoUpdate        bool   `yaml:"auto_update"`
		UpdateCheckWindow string `yaml:"update_check_window,omitempty"`
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
}

type gateConfig struct {
	Server serverConfig `yaml:"server"`
	Logger loggerConfig `yaml:"logger"`
	WAF    wafConfig    `yaml:"waf"`
	SSL    sslConfig    `yaml:"ssl"`
	Hosts  []hostConfig `yaml:"hosts"`
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
			LogDir:        "./applogs",
		},
		WAF: wafConfig{
			Enabled: cfg.WAFEnabled,
			Whitelist: struct {
				IPRanges []string `yaml:"ip_ranges"`
			}{
				IPRanges: cfg.Whitelist,
			},
			CRS: struct {
				Enabled bool   `yaml:"enabled"`
				Version string `yaml:"version"`
			}{
				Enabled: true,
				Version: "v4.19.0",
			},
		},
		SSL: sslConfig{
			Remote: struct {
				Enabled           bool   `yaml:"enabled"`
				Endpoint          string `yaml:"endpoint"`
				AutoUpdate        bool   `yaml:"auto_update"`
				UpdateCheckWindow string `yaml:"update_check_window,omitempty"`
			}{
				Enabled:           sslEnabled,
				Endpoint:          cfg.SSLEndpoint,
				AutoUpdate:        cfg.SSLAutoUpdate,
				UpdateCheckWindow: cfg.SSLUpdateCheckTime,
			},
		},
		Hosts: make([]hostConfig, 0, len(hosts)),
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
		}
		config.Hosts = append(config.Hosts, host)
	}

	data, err := yaml.Marshal(&config)
	if err != nil {
		return "", err
	}

	return string(data), nil
}
