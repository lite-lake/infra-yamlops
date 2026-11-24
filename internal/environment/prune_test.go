package environment

import (
	"strings"
	"testing"
)

func TestParseDockerDiskUsage(t *testing.T) {
	input := `{"Active":"33","Reclaimable":"22.21GB (61%)","Size":"36.08GB","TotalCount":"176","Type":"Images"}
{"Active":"34","Reclaimable":"0B (0%)","Size":"29.86MB","TotalCount":"34","Type":"Containers"}
{"Active":"0","Reclaimable":"0B","Size":"0B","TotalCount":"0","Type":"Local Volumes"}
{"Active":"0","Reclaimable":"0B","Size":"0B","TotalCount":"0","Type":"Build Cache"}`

	usage, err := parseDockerDiskUsage(input)
	if err != nil {
		t.Fatalf("parseDockerDiskUsage() error = %v", err)
	}
	if len(usage.Entries) != 4 {
		t.Fatalf("entries len = %d, want 4", len(usage.Entries))
	}
	if got := usage.ReclaimableSize("Images"); got != "22.21GB (61%)" {
		t.Errorf("ReclaimableSize(Images) = %q, want %q", got, "22.21GB (61%)")
	}
	if got := usage.ReclaimableSize("Build Cache"); got != "0B" {
		t.Errorf("ReclaimableSize(Build Cache) = %q, want 0B", got)
	}
	if got := usage.TotalReclaimable(); !strings.Contains(got, "Images: 22.21GB (61%)") {
		t.Errorf("TotalReclaimable() = %q, want it to contain Images reclaimable", got)
	}
}

func TestParseDockerDiskUsage_Empty(t *testing.T) {
	usage, err := parseDockerDiskUsage("")
	if err != nil {
		t.Fatalf("parseDockerDiskUsage() error = %v", err)
	}
	if len(usage.Entries) != 0 {
		t.Fatalf("entries len = %d, want 0", len(usage.Entries))
	}
	if got := usage.TotalReclaimable(); got != "0B" {
		t.Errorf("TotalReclaimable() = %q, want 0B", got)
	}
}

func TestParseDockerDiskUsage_Invalid(t *testing.T) {
	_, err := parseDockerDiskUsage("not-json")
	if err == nil {
		t.Fatal("parseDockerDiskUsage() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "invalid docker system df entry") {
		t.Errorf("parseDockerDiskUsage() error = %v, want invalid entry error", err)
	}
}

func TestBuildPruneCommand(t *testing.T) {
	tests := []struct {
		name   string
		filter PruneFilter
		want   string
	}{
		{name: "image", filter: PruneFilterImage, want: "sudo docker image prune -f 2>&1"},
		{name: "container", filter: PruneFilterContainer, want: "sudo docker container prune -f 2>&1"},
		{name: "volume", filter: PruneFilterVolume, want: "sudo docker volume prune -f 2>&1"},
		{name: "builder", filter: PruneFilterBuilder, want: "sudo docker builder prune -af 2>&1"},
		{name: "all", filter: PruneFilterAll, want: "sudo docker system prune -f --volumes 2>&1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildPruneCommand(tt.filter)
			if got != tt.want {
				t.Errorf("buildPruneCommand(%q) = %q, want %q", tt.filter, got, tt.want)
			}
			// 带标签的 :prod-prev 是唯一回滚目标，image/system prune 绝不能带 -a。
			if tt.filter == PruneFilterImage || tt.filter == PruneFilterAll {
				if strings.Contains(got, " -a") {
					t.Errorf("buildPruneCommand(%q) = %q must not contain -a (would delete tagged rollback images)", tt.filter, got)
				}
			}
		})
	}
}
