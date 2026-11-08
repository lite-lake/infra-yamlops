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
