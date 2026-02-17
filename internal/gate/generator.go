package gate

import (
	"gopkg.in/yaml.v3"
)

type GatewayConfig struct {
	Port        int
	LogLevel    int
	WAFEnabled  bool
	Whitelist   []string
	SSLMode     string
	SSLEndpoint string
}

type HostRoute struct {
	Name        string
	Port        int
	SSLPort     int
	Backend     []string
	HealthCheck string
}

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
	Level         int  `yaml:"level"`
	EnableConsole bool `yaml:"enable_console"`
}

type wafConfig struct {
	Enabled   bool `yaml:"enabled"`
	Whitelist struct {
		IPRanges []string `yaml:"ip_ranges"`
	} `yaml:"whitelist"`
}

type sslConfig struct {
	Remote struct {
		Enabled  bool   `yaml:"enabled"`
		Endpoint string `yaml:"endpoint"`
	} `yaml:"remote"`
}

type hostConfig struct {
	Name    string   `yaml:"name"`
	Port    int      `yaml:"port"`
	SSLPort int      `yaml:"ssl_port"`
	Backend []string `yaml:"backend"`
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
		},
		WAF: wafConfig{
			Enabled: cfg.WAFEnabled,
			Whitelist: struct {
				IPRanges []string `yaml:"ip_ranges"`
			}{
				IPRanges: cfg.Whitelist,
			},
		},
		SSL: sslConfig{
			Remote: struct {
				Enabled  bool   `yaml:"enabled"`
				Endpoint string `yaml:"endpoint"`
			}{
				Enabled:  sslEnabled,
				Endpoint: cfg.SSLEndpoint,
			},
		},
		Hosts: make([]hostConfig, 0, len(hosts)),
	}

	for _, h := range hosts {
		host := hostConfig{
			Name:    h.Name,
			Port:    h.Port,
			SSLPort: h.SSLPort,
			Backend: h.Backend,
		}
		config.Hosts = append(config.Hosts, host)
	}

	data, err := yaml.Marshal(&config)
	if err != nil {
		return "", err
	}

	return string(data), nil
}
