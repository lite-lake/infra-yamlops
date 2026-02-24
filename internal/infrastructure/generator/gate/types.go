package gate

type GatewayConfig struct {
	Port               int
	LogLevel           int
	WAFEnabled         bool
	Whitelist          []string
	SSLMode            string
	SSLEndpoint        string
	SSLAutoUpdate      bool
	SSLUpdateCheckTime string
}

type HostRoute struct {
	Name                string
	Port                int
	SSLPort             int
	Backend             []string
	HealthCheck         string
	HealthCheckInterval string
	HealthCheckTimeout  string
}
