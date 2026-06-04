package gate

type GatewayConfig struct {
	Port        int
	LogLevel    int
	WAFEnabled  bool
	Whitelist   []string
	SSLMode     string
	SSLEndpoint string
	SSLAPIKey   string
}

type HostRoute struct {
	Name                string
	Port                int
	SSLPort             int
	Backend             []string
	HealthCheck         string
	HealthCheckInterval string
	HealthCheckTimeout  string
	HealthCheckEnabled  *bool  // nil = 默认启用，false = 禁用
	PreserveHostHeader  bool
	GZipEnabled         *bool // 使用指针以支持"未设置"状态
	OverrideHost        string
}
