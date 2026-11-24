package usecase

import (
	"strings"
	"testing"
	"time"
)

func TestContainerName(t *testing.T) {
	tests := []struct {
		env, service, want string
	}{
		{"prod", "api-server", "yo-prod-api-server"},
		{"dev", "web", "yo-dev-web"},
	}
	for _, tt := range tests {
		if got := ContainerName(tt.env, tt.service); got != tt.want {
			t.Errorf("ContainerName(%q, %q) = %q, want %q", tt.env, tt.service, got, tt.want)
		}
	}
}

func TestBuildDockerLogsCommand(t *testing.T) {
	tests := []struct {
		name    string
		opts    LogOptions
		want    []string
		notWant []string
	}{
		{
			name: "default tail",
			opts: LogOptions{Tail: 200},
			want: []string{"sudo docker logs", "--tail", "200", "'yo-prod-api'"},
		},
		{
			name:    "no tail = full logs",
			opts:    LogOptions{},
			want:    []string{"sudo docker logs", "'yo-prod-api'"},
			notWant: []string{"--tail"},
		},
		{
			name: "since/until/timestamps",
			opts: LogOptions{Tail: 10, Since: "1h", Until: "30m", Timestamps: true},
			want: []string{"--since", "'1h'", "--until", "'30m'", "--timestamps"},
		},
		{
			name: "grep appends server-side filter",
			opts: LogOptions{Tail: 10, Grep: "error"},
			want: []string{"| grep -i -F 'error'"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := BuildDockerLogsCommand("yo-prod-api", tt.opts)
			for _, w := range tt.want {
				if !strings.Contains(cmd, w) {
					t.Errorf("cmd %q missing %q", cmd, w)
				}
			}
			for _, nw := range tt.notWant {
				if strings.Contains(cmd, nw) {
					t.Errorf("cmd %q should not contain %q", cmd, nw)
				}
			}
		})
	}
}

func TestBuildContainerInspectCommand(t *testing.T) {
	cmd := BuildContainerInspectCommand("yo-prod-api")
	if !strings.Contains(cmd, "docker inspect") {
		t.Errorf("cmd missing docker inspect: %q", cmd)
	}
	if !strings.Contains(cmd, "|| echo missing") {
		t.Errorf("cmd must degrade to 'missing' when container absent: %q", cmd)
	}
}

func TestParseContainerInspectOutput(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		wantExist bool
		wantState string
		wantRC    int
	}{
		{"missing", "missing\n", false, "", 0},
		{"empty", "", false, "", 0},
		{
			name:      "running container",
			output:    "abc123def456 running 3 2026-08-28T09:12:04.000000000Z 0001-01-01T00:00:00Z\n",
			wantExist: true,
			wantState: "running",
			wantRC:    3,
		},
		{
			name:      "exited container",
			output:    "abc123def456 exited 0 2026-08-28T09:00:00Z 2026-08-28T09:05:00Z\n",
			wantExist: true,
			wantState: "exited",
			wantRC:    0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := ParseContainerInspectOutput(tt.output)
			if st.Exists != tt.wantExist {
				t.Errorf("Exists = %v, want %v", st.Exists, tt.wantExist)
			}
			if st.State != tt.wantState {
				t.Errorf("State = %q, want %q", st.State, tt.wantState)
			}
			if st.RestartCount != tt.wantRC {
				t.Errorf("RestartCount = %d, want %d", st.RestartCount, tt.wantRC)
			}
		})
	}
}

func TestValidArchiveID(t *testing.T) {
	valid := []string{"20260828T101530-abc123def456", "20260101T000000Z-aabbcc", "run_1"}
	invalid := []string{"", "../etc/passwd", "with space", "a;rm -rf /", "a/b", strings.Repeat("x", 65)}
	for _, id := range valid {
		if !validArchiveID(id) {
			t.Errorf("validArchiveID(%q) = false, want true", id)
		}
	}
	for _, id := range invalid {
		if validArchiveID(id) {
			t.Errorf("validArchiveID(%q) = true, want false", id)
		}
	}
}

func TestServiceLogsDir(t *testing.T) {
	remoteDir := "/data/yamlops/yo-prod-api"
	want := "/data/yamlops/yo-prod-api/logs"
	if got := ServiceLogsDir(remoteDir); got != want {
		t.Errorf("ServiceLogsDir = %q, want %q", got, want)
	}
}

func TestShortID(t *testing.T) {
	if got := shortID("abcdefghijklmnopqrstuvwxyz"); got != "abcdefghijkl" {
		t.Errorf("shortID long = %q", got)
	}
	if got := shortID("abc"); got != "abc" {
		t.Errorf("shortID short = %q", got)
	}
}

func TestArchiveIDFormat(t *testing.T) {
	// 归档 id = UTC 时间戳 + 容器短 ID，可按字典序排序 = 按时间排序
	now := time.Date(2026, 8, 28, 10, 15, 30, 0, time.UTC)
	id := now.UTC().Format("20060102T150405Z") + "-" + shortID("abcdef1234567890")
	if id != "20260828T101530Z-abcdef123456" {
		t.Errorf("archive id = %q", id)
	}
	if !validArchiveID(id) {
		t.Errorf("generated archive id %q should be valid", id)
	}
}
