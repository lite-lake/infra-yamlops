package environment

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PruneFilter defines which Docker resources to prune.
type PruneFilter string

const (
	PruneFilterAll       PruneFilter = "all"
	PruneFilterImage     PruneFilter = "image"
	PruneFilterContainer PruneFilter = "container"
	PruneFilterVolume    PruneFilter = "volume"
	PruneFilterBuilder   PruneFilter = "builder"
)

// DockerDiskUsageEntry represents a single row from `docker system df --format '{{json .}}'`.
type DockerDiskUsageEntry struct {
	Type        string `json:"Type"`
	TotalCount  string `json:"TotalCount"`
	Active      string `json:"Active"`
	Reclaimable string `json:"Reclaimable"`
	Size        string `json:"Size"`
	TotalSize   string `json:"TotalSize"`
}

// DockerDiskUsage holds parsed disk usage data for a server.
type DockerDiskUsage struct {
	Entries []DockerDiskUsageEntry
}

// ReclaimableSize returns the reclaimable size string for the given resource type.
func (d DockerDiskUsage) ReclaimableSize(resourceType string) string {
	for _, e := range d.Entries {
		if strings.EqualFold(e.Type, resourceType) {
			return e.Reclaimable
		}
	}
	return "0B"
}

// TotalReclaimable returns a summary of all reclaimable space.
func (d DockerDiskUsage) TotalReclaimable() string {
	var parts []string
	for _, e := range d.Entries {
		if e.Reclaimable != "" && e.Reclaimable != "0B" {
			parts = append(parts, fmt.Sprintf("%s: %s", e.Type, e.Reclaimable))
		}
	}
	if len(parts) == 0 {
		return "0B"
	}
	return strings.Join(parts, ", ")
}

// DockerDiskUsage scans the server's Docker disk usage via `docker system df`.
func (s *Syncer) DockerDiskUsage() (*DockerDiskUsage, error) {
	stdout, stderr, err := s.client.Run("sudo docker system df --format '{{json .}}' 2>&1")
	if err != nil {
		return nil, fmt.Errorf("docker system df failed: %w, stderr: %s", err, stderr)
	}

	usage, err := parseDockerDiskUsage(stdout)
	if err != nil {
		return nil, fmt.Errorf("docker system df parse failed: %w, stderr: %s", err, stderr)
	}
	return usage, nil
}

// parseDockerDiskUsage parses the line-per-JSON output of `docker system df --format '{{json .}}'`.
func parseDockerDiskUsage(output string) (*DockerDiskUsage, error) {
	var entries []DockerDiskUsageEntry
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		var entry DockerDiskUsageEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("invalid docker system df entry %q: %w", line, err)
		}
		entries = append(entries, entry)
	}
	return &DockerDiskUsage{Entries: entries}, nil
}

// PruneDocker runs docker prune on the server for the specified filter.
func (s *Syncer) PruneDocker(filter PruneFilter) SyncResult {
	cmd := buildPruneCommand(filter)
	stdout, stderr, err := s.client.Run(cmd)
	if err != nil {
		return SyncResult{
			Name:    fmt.Sprintf("docker_prune_%s", filter),
			Success: false,
			Message: fmt.Sprintf("prune failed: %s", strings.TrimSpace(stderr)),
			Error:   err,
		}
	}
	return SyncResult{
		Name:    fmt.Sprintf("docker_prune_%s", filter),
		Success: true,
		Message: strings.TrimSpace(stdout),
	}
}

// buildPruneCommand constructs the docker prune command for the given filter.
// 注意：部署会保留上一版镜像为 :prod-prev 作为回滚目标，因此这里刻意不使用
// `-a`（image prune -a / system prune -a 会删除所有未使用镜像，包括带标签的
// 回滚镜像及其层）。没有替代回滚存储前，禁止重新引入 `-a`。
func buildPruneCommand(filter PruneFilter) string {
	switch filter {
	case PruneFilterImage:
		// 仅清理 dangling（无标签）镜像，带标签的 :prod-prev 保留。
		return "sudo docker image prune -f 2>&1"
	case PruneFilterContainer:
		return "sudo docker container prune -f 2>&1"
	case PruneFilterVolume:
		return "sudo docker volume prune -f 2>&1"
	case PruneFilterBuilder:
		return "sudo docker builder prune -af 2>&1"
	default:
		// 去掉 -a：带标签镜像（含 :prod-prev）及其层保留，仅清 dangling 与未使用资源。
		return "sudo docker system prune -f --volumes 2>&1"
	}
}

// ParsePruneFilter converts a string filter to PruneFilter.
func ParsePruneFilter(s string) PruneFilter {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "image":
		return PruneFilterImage
	case "container":
		return PruneFilterContainer
	case "volume":
		return PruneFilterVolume
	case "builder":
		return PruneFilterBuilder
	default:
		return PruneFilterAll
	}
}
