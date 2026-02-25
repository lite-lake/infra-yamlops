package environment

import (
	"encoding/json"
	"strings"

	"github.com/litelake/yamlops/internal/domain/entity"
)

type SSHTaskRunner interface {
	Run(cmd string) (string, string, error)
}

func IsRegistryLoggedIn(client SSHTaskRunner, registry *entity.Registry, useSudo bool) bool {
	var dockerInfoCmd, configJSONCmd string
	if useSudo {
		dockerInfoCmd = "sudo docker info 2>/dev/null | grep -i username || true"
		configJSONCmd = "sudo cat /root/.docker/config.json 2>/dev/null || true"
	} else {
		dockerInfoCmd = "sudo docker info 2>/dev/null | grep -i username || true"
		configJSONCmd = "cat ~/.docker/config.json 2>/dev/null || true"
	}

	dockerInfo, _, _ := client.Run(dockerInfoCmd)
	configJSON, _, _ := client.Run(configJSONCmd)

	return IsRegistryLoggedInWithData(registry, dockerInfo, configJSON)
}

func IsRegistryLoggedInWithData(r *entity.Registry, dockerInfo, configJSON string) bool {
	if strings.Contains(strings.ToLower(dockerInfo), strings.ToLower(r.URL)) {
		return true
	}

	type dockerConfig struct {
		Auths map[string]struct {
			Auth string `json:"auth"`
		} `json:"auths"`
	}

	var cfg dockerConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err == nil {
		for host, auth := range cfg.Auths {
			if strings.Contains(host, r.URL) && auth.Auth != "" {
				return true
			}
		}
	}

	return false
}
