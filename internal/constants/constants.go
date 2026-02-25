package constants

import "time"

const (
	RemoteBaseDir       = "/data/yamlops"
	ServiceDirPattern   = "yo-%s-%s"
	TempFilePattern     = "yamlops-*.yml"
	RemoteTempFileFmt   = "/tmp/yamlops-%d"
	ServicePrefixFormat = "yo-%s-%s"
)

const (
	DefaultHTTPPort       = 80
	DefaultHTTPSPort      = 443
	DefaultHealthInterval = "30s"
	DefaultHealthTimeout  = "10s"
	DefaultCRSVersion     = "v4.19.0"
)

const (
	ComposeVersion       = "3.8"
	DefaultRestartPolicy = "unless-stopped"
	DefaultHealthRetries = 3
)

const (
	GatewayConfigPath = "./gateway.yml:/app/configs/server.yml:ro"
	GatewayCachePath  = "./cache:/app/cache"
	SSLDataPath       = "./ssl-data:/app/data"
	SSLConfigPath     = "./ssl-config:/app/configs:ro"
	DefaultLogDir     = "./applogs"
)

const (
	HostDockerInternal = "host.docker.internal"
	HostDockerGateway  = "host.docker.internal:host-gateway"
)

const (
	DefaultSSHTimeoutSec           = 30
	DefaultSSHRetryAttempts        = 3
	DefaultSSHRetryInitialDelaySec = 1
	DefaultSSHRetryMaxDelaySec     = 30

	DefaultRetryMaxAttempts    = 3
	DefaultRetryInitialDelayMs = 100
	DefaultRetryMaxDelaySec    = 30
	DefaultRetryMultiplier     = 2.0

	DefaultDNSRecordTTL           = 600
	DefaultDNSRetryAttempts       = 3
	DefaultDNSRetryInitialDelayMs = 500
	DefaultDNSRetryMaxDelayMs     = 30000

	MaxPortNumber = 65535

	FilePermissionOwnerRW = 0600
	FilePermissionConfig  = 0644
	DirPermissionOwner    = 0700
	DirPermissionStandard = 0755
	DefaultRemoteFilePerm = "644"
	DefaultRemoteDirPerm  = "755"

	SSHStreamBufferSize = 1024
)

var (
	DefaultSSHTimeout           = DefaultSSHTimeoutSec * time.Second
	DefaultSSHRetryInitialDelay = DefaultSSHRetryInitialDelaySec * time.Second
	DefaultSSHRetryMaxDelay     = DefaultSSHRetryMaxDelaySec * time.Second
	DefaultRetryInitialDelay    = DefaultRetryInitialDelayMs * time.Millisecond
	DefaultRetryMaxDelay        = DefaultRetryMaxDelaySec * time.Second
	DefaultDNSRetryInitialDelay = DefaultDNSRetryInitialDelayMs * time.Millisecond
	DefaultDNSRetryMaxDelay     = DefaultDNSRetryMaxDelayMs * time.Millisecond
)
