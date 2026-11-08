package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lite-lake/infra-yamlops/internal/application/generator/compose"
	"github.com/lite-lake/infra-yamlops/internal/constants"
	"github.com/lite-lake/infra-yamlops/internal/domain"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"gopkg.in/yaml.v3"
)

const (
	uptimeKumaDefaultContainerPort = 3001 // Kuma 容器内端口
	uptimeKumaDefaultInterval      = "60s"
	uptimeKumaWebhookListenPort    = 37402 // agent 告警转换端点，仅内网
)

// generateUptimeKumaConfig 生成 uptime-kuma 的 targets.yaml（agent 消费）+ 主/agent 双容器 compose。
func (g *Generator) generateUptimeKumaConfig(serverDir string, infra *entity.InfraService, config *entity.Config) error {
	uk := infra.UptimeKumaConfig
	if uk == nil {
		return fmt.Errorf("uptime-kuma config is nil for %s", infra.Name)
	}

	secrets := config.GetSecretsMap()
	adminPassword, err := uk.Admin.Password.Resolve(secrets)
	if err != nil {
		return fmt.Errorf("failed to resolve admin password for %s: %w", infra.Name, err)
	}
	dbPassword, err := uk.DB.Password.Resolve(secrets)
	if err != nil {
		return fmt.Errorf("failed to resolve db password for %s: %w", infra.Name, err)
	}

	targetsContent, err := generateUptimeKumaTargets(uk, config.GetSecretsMap())
	if err != nil {
		return err
	}
	targetsFile := filepath.Join(serverDir, fmt.Sprintf("%s.kuma.targets.yaml", infra.Name))
	if err := os.WriteFile(targetsFile, []byte(targetsContent), constants.FilePermissionConfig); err != nil {
		return fmt.Errorf("failed to write uptime-kuma targets: %w", err)
	}

	envFileName := fmt.Sprintf("%s.env", infra.Name)
	envFile := filepath.Join(serverDir, envFileName)
	envContent := g.buildUptimeKumaEnvFile(infra, config, adminPassword, dbPassword)
	if err := os.WriteFile(envFile, []byte(envContent), constants.FilePermissionOwnerRW); err != nil {
		return fmt.Errorf("failed to write uptime-kuma env file %s: %w", envFile, err)
	}

	composeContent, err := g.generateUptimeKumaCompose(infra, config, envFileName)
	if err != nil {
		return err
	}
	composeFile := filepath.Join(serverDir, fmt.Sprintf("%s.compose.yaml", infra.Name))
	return os.WriteFile(composeFile, []byte(composeContent), constants.FilePermissionConfig)
}

// buildUptimeKumaEnvFile 收集主/agent 两个容器的全部 env 变量到同一个 {svc}.env（0600）。
// 密钥只在 env_file 中（DB 密钥 / ADMIN 密钥用不同变量名），compose yaml 不写任何明文密钥。
func (g *Generator) buildUptimeKumaEnvFile(infra *entity.InfraService, config *entity.Config, adminPassword, dbPassword string) string {
	uk := infra.UptimeKumaConfig

	reconcileInterval := uk.Sync.ReconcileInterval
	if reconcileInterval == "" {
		reconcileInterval = uptimeKumaDefaultInterval
	}

	envMap := map[string]string{
		// 主容器（Kuma）：外部 DB + WS 反向代理优化
		"UPTIME_KUMA_DB_TYPE":         uk.DB.Type,
		"UPTIME_KUMA_DB_HOSTNAME":     uk.DB.Hostname,
		"UPTIME_KUMA_DB_PORT":         fmt.Sprintf("%d", uk.DB.Port),
		"UPTIME_KUMA_DB_NAME":         uk.DB.Name,
		"UPTIME_KUMA_DB_USERNAME":     uk.DB.Username,
		"UPTIME_KUMA_DB_PASSWORD":     dbPassword,
		"UPTIME_KUMA_WS_ORIGIN_CHECK": "bypass",
		// agent 容器（对标同步）
		"KUMA_URL":                fmt.Sprintf("http://%s:%d", infra.Name, uptimeKumaDefaultContainerPort),
		"KUMA_ADMIN_USERNAME":     uk.Admin.Username,
		"KUMA_ADMIN_PASSWORD":     adminPassword,
		"KUMA_TARGETS_FILE":       "/targets.yaml",
		"RECONCILE_INTERVAL":      reconcileInterval,
		"WEBHOOK_LISTEN_PORT":     fmt.Sprintf("%d", uptimeKumaWebhookListenPort),
		"KUMA_NOTIFY_WEBHOOK_URL": fmt.Sprintf("http://%s-agent:%d/webhook/kuma", infra.Name, uptimeKumaWebhookListenPort),
	}
	// 留空不写 env（agent 自身默认 1h）；0s 原样透传表示关闭兜底重登。
	if uk.Sync.ReloginInterval != "" {
		envMap["KUMA_RELOGIN_INTERVAL"] = uk.Sync.ReloginInterval
	}

	serverMap := config.GetServerMap()
	if server, ok := serverMap[infra.Server]; ok {
		envMap["DEPLOY_ZONE_NAME"] = server.Zone
	}
	envMap["DEPLOY_ENV_NAME"] = g.env
	envMap["DEPLOY_SERVER_NAME"] = infra.Server
	envMap["DEPLOY_SERVICE_NAME"] = infra.Name

	keys := make([]string, 0, len(envMap))
	for k := range envMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(fmt.Sprintf("%s=%s\n", k, envMap[k]))
	}
	return sb.String()
}

// uptimeKumaTargetsFile targets.yaml 结构与 lite-polyglot agent 消费约定一致。
type uptimeKumaTargetsFile struct {
	Version      int                              `yaml:"version"`
	Notification *uptimeKumaTargetsNotif          `yaml:"notification,omitempty"`
	Monitors     []entity.UptimeKumaMonitorConfig `yaml:"monitors"`
}

type uptimeKumaTargetsNotif struct {
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url,omitempty"`
}

// generateUptimeKumaTargets 生成 {svc}.kuma.targets.yaml：
// monitors + notification camelCase 直通，每个 monitor 固定打 tags: [yamlops]。
// Extra 中的 {secret: name} 引用在生成 targets.yaml 文本这一步递归解析为明文
// （内部PRD V6，secrets 来源与 generateUptimeKumaCompose 的 config.GetSecretsMap() 一致）；
// 解析失败 fail-closed 直接报错，不产出半成品 targets。
func generateUptimeKumaTargets(uk *entity.UptimeKumaConfig, secrets map[string]string) (string, error) {
	monitors := make([]entity.UptimeKumaMonitorConfig, 0, len(uk.Monitors))
	for _, m := range uk.Monitors {
		m.Tags = []string{"yamlops"}
		if len(m.Extra) > 0 {
			resolved, err := resolveMonitorExtraSecrets(m.Extra, secrets)
			if err != nil {
				return "", fmt.Errorf("uptime-kuma monitor '%s': %w", m.Name, err)
			}
			m.Extra = resolved
		}
		monitors = append(monitors, m)
	}

	tf := uptimeKumaTargetsFile{
		Version: 1,
		Notification: &uptimeKumaTargetsNotif{
			Enabled: uk.Notification.Enabled,
			URL:     uk.Notification.URL,
		},
		Monitors: monitors,
	}

	data, err := yaml.Marshal(&tf)
	if err != nil {
		return "", fmt.Errorf("failed to marshal uptime-kuma targets: %w", err)
	}
	return string(data), nil
}

// resolveMonitorExtraSecrets 对单个 monitor 的 Extra 深遍历做 {secret: name} 解析，
// 返回替换后的新 map（不改写入参）。
func resolveMonitorExtraSecrets(extra map[string]any, secrets map[string]string) (map[string]any, error) {
	resolved := make(map[string]any, len(extra))
	for k, v := range extra {
		rv, err := resolveSecretValue(v, secrets)
		if err != nil {
			return nil, err
		}
		resolved[k] = rv
	}
	return resolved, nil
}

// resolveSecretValue 深遍历 yaml 解码后的任意值，把形如 {secret: name} 的节点替换为
// secrets map 中的解析值（内部PRD V6）。
// {secret: name} 形状定义：map[string]any 且恰好一个 key "secret"、其值为 string。
// 找不到对应 secret 时 fail-closed 返回 error（包装 domain.ErrMissingSecret，由调用方
// 补充 monitor 名）；其他任何形状（字符串/数字/布尔/普通 map/切片）原样透传，切片元素递归。
func resolveSecretValue(v any, secrets map[string]string) (any, error) {
	switch t := v.(type) {
	case map[string]any:
		if name, ok := secretRefName(t); ok {
			val, found := secrets[name]
			if !found {
				return nil, fmt.Errorf("%w: %s", domain.ErrMissingSecret, name)
			}
			return val, nil
		}
		out := make(map[string]any, len(t))
		for k, vv := range t {
			resolved, err := resolveSecretValue(vv, secrets)
			if err != nil {
				return nil, err
			}
			out[k] = resolved
		}
		return out, nil
	case []any:
		out := make([]any, len(t))
		for i, vv := range t {
			resolved, err := resolveSecretValue(vv, secrets)
			if err != nil {
				return nil, err
			}
			out[i] = resolved
		}
		return out, nil
	default:
		return v, nil
	}
}

// secretRefName 判断 m 是否恰好为 {secret: <string>} 形状；是则返回 secret 名。
// 多余 key、值非 string 均视为普通 map（透传语义）。
func secretRefName(m map[string]any) (string, bool) {
	if len(m) != 1 {
		return "", false
	}
	name, ok := m["secret"].(string)
	return name, ok
}

func (g *Generator) generateUptimeKumaCompose(infra *entity.InfraService, config *entity.Config, envFileName string) (string, error) {
	uk := infra.UptimeKumaConfig

	networks := infra.Networks
	if len(networks) == 0 {
		networks = []string{fmt.Sprintf("yamlops-%s", g.env)}
	}

	mainSvc := &compose.ComposeService{
		Name:     infra.Name,
		Image:    infra.Image,
		Networks: networks,
		EnvFiles: []string{envFileName},
		Volumes:  []string{"./data:/app/data"},
		HealthCheck: &compose.HealthCheck{
			Test:        []string{"CMD", "curl", "-fsS", "http://127.0.0.1:3001"},
			Interval:    "10s",
			Timeout:     "5s",
			Retries:     5,
			StartPeriod: "20s",
		},
		ExtraHosts: []string{constants.HostDockerGateway},
	}

	agentSvc := &compose.ComposeService{
		Name:       infra.Name + "-agent",
		Image:      uk.Sync.Image,
		Networks:   networks,
		EnvFiles:   []string{envFileName},
		Volumes:    []string{fmt.Sprintf("./%s.kuma.targets.yaml:/targets.yaml:ro", infra.Name)},
		DependsOn:  map[string]compose.DependsOnEntry{infra.Name: {Condition: "service_healthy"}},
		ExtraHosts: []string{constants.HostDockerGateway},
	}

	return g.composeGen.GenerateMulti([]*compose.ComposeService{mainSvc, agentSvc}, g.env)
}
