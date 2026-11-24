package compose

type HealthCheck struct {
	Test        []string `yaml:"test,omitempty"`
	Interval    string   `yaml:"interval,omitempty"`
	Timeout     string   `yaml:"timeout,omitempty"`
	Retries     int      `yaml:"retries,omitempty"`
	StartPeriod string   `yaml:"start_period,omitempty"`
}

type ResourceLimits struct {
	Cpus   string `yaml:"cpus,omitempty"`
	Memory string `yaml:"memory,omitempty"`
}

type Resources struct {
	Limits       *ResourceLimits `yaml:"limits,omitempty"`
	Reservations *ResourceLimits `yaml:"reservations,omitempty"`
}

type Deploy struct {
	Resources *Resources `yaml:"resources,omitempty"`
}

type NetworkConfig struct {
	Aliases []string `yaml:"aliases,omitempty"`
}

type Ulimit struct {
	Soft int `yaml:"soft"`
	Hard int `yaml:"hard"`
}

// DependsOnEntry describe a dependency condition (e.g. service_healthy).
type DependsOnEntry struct {
	Condition string `yaml:"condition,omitempty"`
}

type Service struct {
	Image         string                    `yaml:"image"`
	ContainerName string                    `yaml:"container_name,omitempty"`
	NetworkMode   string                    `yaml:"network_mode,omitempty"`
	Privileged    bool                      `yaml:"privileged,omitempty"`
	CapAdd        []string                  `yaml:"cap_add,omitempty"`
	Devices       []string                  `yaml:"devices,omitempty"`
	Ulimits       map[string]Ulimit         `yaml:"ulimits,omitempty"`
	Command       []string                  `yaml:"command,omitempty"`
	Ports         []string                  `yaml:"ports,omitempty"`
	Environment   map[string]string         `yaml:"environment,omitempty"`
	EnvFiles      []string                  `yaml:"env_file,omitempty"`
	Volumes       []string                  `yaml:"volumes,omitempty"`
	HealthCheck   *HealthCheck              `yaml:"healthcheck,omitempty"`
	Deploy        *Deploy                   `yaml:"deploy,omitempty"`
	Networks      map[string]*NetworkConfig `yaml:"networks,omitempty"`
	DependsOn     map[string]DependsOnEntry `yaml:"depends_on,omitempty"`
	Restart       string                    `yaml:"restart,omitempty"`
	ExtraHosts    []string                  `yaml:"extra_hosts,omitempty"`
	Logging       *LogRotation              `yaml:"logging,omitempty"`
}

type ExternalNetwork struct {
	External bool `yaml:"external"`
}

// LogRotation json-file 日志轮转配置（默认写入生成的 compose）。
type LogRotation struct {
	Driver  string            `yaml:"driver"`
	Options map[string]string `yaml:"options"`
}

type ComposeFile struct {
	Version  string                      `yaml:"version"`
	Services map[string]Service          `yaml:"services"`
	Networks map[string]*ExternalNetwork `yaml:"networks,omitempty"`
}

type ComposeService struct {
	Name        string
	Image       string
	NetworkMode string
	Privileged  bool
	CapAdd      []string
	Devices     []string
	Ulimits     map[string]Ulimit
	Command     []string
	Ports       []string
	Environment map[string]string
	EnvFiles    []string
	Volumes     []string
	HealthCheck *HealthCheck
	Resources   *Resources
	Internal    bool
	Networks    []string
	DependsOn   map[string]DependsOnEntry
	ExtraHosts  []string
	Restart     string
}
