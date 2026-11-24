package environment

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/ssh"
)

type Checker struct {
	client     *ssh.Client
	server     *entity.Server
	secrets    map[string]string
	registries map[string]*entity.Registry
}

func NewChecker(client *ssh.Client, server *entity.Server, registries []entity.Registry, secrets map[string]string) *Checker {
	regMap := make(map[string]*entity.Registry)
	for i := range registries {
		regMap[registries[i].Name] = &registries[i]
	}
	return &Checker{
		client:     client,
		server:     server,
		secrets:    secrets,
		registries: regMap,
	}
}

func (c *Checker) CheckAll() []CheckResult {
	var results []CheckResult

	results = append(results, CheckResult{
		Name:    "SSH Connection",
		Status:  CheckStatusOK,
		Message: "OK",
	})

	results = append(results, c.CheckSudo())
	results = append(results, c.CheckDocker())
	results = append(results, c.CheckDockerCompose())
	results = append(results, c.CheckAPTSource())
	results = append(results, c.CheckRegistries()...)

	return results
}

func (c *Checker) CheckSudo() CheckResult {
	_, _, err := c.client.Run("sudo -n true 2>&1")
	if err != nil {
		return CheckResult{
			Name:    "Sudo Passwordless",
			Status:  CheckStatusError,
			Message: "Requires password",
			Detail:  err.Error(),
		}
	}
	return CheckResult{
		Name:    "Sudo Passwordless",
		Status:  CheckStatusOK,
		Message: "OK",
	}
}

func (c *Checker) CheckDocker() CheckResult {
	stdout, _, err := c.client.Run("docker --version 2>/dev/null")
	if err != nil {
		return CheckResult{
			Name:    "Docker",
			Status:  CheckStatusError,
			Message: "Not installed",
		}
	}

	re := regexp.MustCompile(`Docker version (\d+\.\d+\.\d+)`)
	matches := re.FindStringSubmatch(stdout)
	if len(matches) > 1 {
		return CheckResult{
			Name:    "Docker",
			Status:  CheckStatusOK,
			Message: matches[1],
		}
	}

	return CheckResult{
		Name:    "Docker",
		Status:  CheckStatusOK,
		Message: strings.TrimSpace(stdout),
	}
}

func (c *Checker) CheckDockerCompose() CheckResult {
	stdout, _, err := c.client.Run("docker compose version 2>/dev/null")
	if err == nil {
		re := regexp.MustCompile(`Docker Compose version v?(\d+\.\d+\.\d+)`)
		matches := re.FindStringSubmatch(stdout)
		if len(matches) > 1 {
			return CheckResult{
				Name:    "Docker Compose",
				Status:  CheckStatusOK,
				Message: matches[1],
			}
		}
	}

	stdout, _, err = c.client.Run("docker-compose --version 2>/dev/null")
	if err != nil {
		return CheckResult{
			Name:    "Docker Compose",
			Status:  CheckStatusError,
			Message: "Not installed",
		}
	}

	re := regexp.MustCompile(`docker-compose version (\d+\.\d+\.\d+)`)
	matches := re.FindStringSubmatch(stdout)
	if len(matches) > 1 {
		return CheckResult{
			Name:    "Docker Compose",
			Status:  CheckStatusOK,
			Message: matches[1] + " (v1)",
		}
	}

	return CheckResult{
		Name:    "Docker Compose",
		Status:  CheckStatusOK,
		Message: strings.TrimSpace(stdout),
	}
}

func (c *Checker) CheckAPTSource() CheckResult {
	expected := c.server.Environment.APTSource
	if expected == "" {
		return CheckResult{
			Name:    "APT Source",
			Status:  CheckStatusOK,
			Message: "Not configured",
		}
	}

	stdout, _, err := c.client.Run("cat /etc/apt/sources.list 2>/dev/null; ls /etc/apt/sources.list.d/*.list 2>/dev/null | xargs cat 2>/dev/null")
	if err != nil {
		return CheckResult{
			Name:    "APT Source",
			Status:  CheckStatusError,
			Message: "Failed to read sources",
			Detail:  err.Error(),
		}
	}

	current := c.detectAPTSource(stdout)

	if current == expected {
		return CheckResult{
			Name:    "APT Source",
			Status:  CheckStatusOK,
			Message: current,
		}
	}

	return CheckResult{
		Name:    "APT Source",
		Status:  CheckStatusWarning,
		Message: fmt.Sprintf("current: %s, expected: %s", current, expected),
	}
}

func (c *Checker) CheckRegistries() []CheckResult {
	var results []CheckResult

	if len(c.server.Environment.Registries) == 0 {
		results = append(results, CheckResult{
			Name:    "Registries",
			Status:  CheckStatusOK,
			Message: "Not configured",
		})
		return results
	}

	for _, regName := range c.server.Environment.Registries {
		registry, ok := c.registries[regName]
		if !ok {
			results = append(results, CheckResult{
				Name:    fmt.Sprintf("Registry: %s", regName),
				Status:  CheckStatusError,
				Message: "Not found in config",
			})
			continue
		}

		if IsRegistryLoggedIn(c.client, registry, true) {
			results = append(results, CheckResult{
				Name:    fmt.Sprintf("Registry: %s", regName),
				Status:  CheckStatusOK,
				Message: fmt.Sprintf("Logged in to %s", registry.URL),
			})
		} else {
			results = append(results, CheckResult{
				Name:    fmt.Sprintf("Registry: %s", regName),
				Status:  CheckStatusWarning,
				Message: fmt.Sprintf("Not logged in to %s", registry.URL),
			})
		}
	}

	return results
}

func (c *Checker) detectAPTSource(content string) string {
	content = strings.ToLower(content)

	if strings.Contains(content, "mirrors.tuna.tsinghua.edu.cn") ||
		strings.Contains(content, "tuna.tsinghua.edu.cn") {
		return "tuna"
	}
	if strings.Contains(content, "mirrors.aliyun.com") {
		return "aliyun"
	}
	if strings.Contains(content, "mirrors.tencentyun.com") ||
		strings.Contains(content, "mirrors.cloud.tencent.com") {
		return "tencent"
	}
	if strings.Contains(content, "archive.ubuntu.com") ||
		strings.Contains(content, "security.ubuntu.com") {
		return "official"
	}

	return "unknown"
}

func FormatResults(serverName string, results []CheckResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s] Environment Check\n", serverName))

	for _, r := range results {
		icon := "✅"
		switch r.Status {
		case CheckStatusWarning:
			icon = "⚠️"
		case CheckStatusError:
			icon = "❌"
		}

		sb.WriteString(fmt.Sprintf("  %-20s %s %s\n", r.Name+":", icon, r.Message))
	}

	return sb.String()
}
