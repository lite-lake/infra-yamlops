package usecase

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testRemoteDir = "/opt/yo/prod/api"
	testPullCmd   = "sudo docker compose -f " + testRemoteDir + "/docker-compose.yml pull"
	testUpCmd     = "sudo docker compose -f " + testRemoteDir + "/docker-compose.yml up -d --force-recreate --pull never"
)

// scriptedSSHClient 复用 mockSSHClient 的记录/上传能力，但按命令逐条决定返回。
// 用于区分 inspect / tag / pull 等命令的成功或失败。
type scriptedSSHClient struct {
	*mockSSHClient
	handler func(cmd string) (stdout, stderr string, err error)
}

func newScriptedSSHClient(handler func(cmd string) (stdout, stderr string, err error)) *scriptedSSHClient {
	return &scriptedSSHClient{mockSSHClient: &mockSSHClient{}, handler: handler}
}

func (c *scriptedSSHClient) Run(cmd string) (stdout, stderr string, err error) {
	c.commandsRun = append(c.commandsRun, cmd)
	if c.handler == nil {
		return "", "", nil
	}
	return c.handler(cmd)
}

// runningProdHandler 模拟容器当前运行 repo/api:prod，其他命令成功。
func runningProdHandler(cmd string) (string, string, error) {
	if strings.HasPrefix(cmd, "sudo docker inspect -f") {
		return "sha256:abc123|repo/api:prod", "", nil
	}
	return "", "", nil
}

func TestRetainPreviousImage_FirstDeployInspectFails(t *testing.T) {
	client := &mockSSHClient{runErr: errors.New("no such object")}
	result := &Result{}

	RetainPreviousImage(client, "yo-prod-api", "repo/api:prod", result)

	if len(client.commandsRun) != 1 {
		t.Fatalf("commands = %v, want only the inspect command", client.commandsRun)
	}
	if len(result.Warnings) != 0 {
		t.Errorf("Warnings = %v, want none on first deploy", result.Warnings)
	}
}

func TestRetainPreviousImage_TagsCurrentImage(t *testing.T) {
	client := &mockSSHClient{runStdout: "sha256:abc123|registry.example.com:5000/app/api:prod\n"}
	result := &Result{}

	RetainPreviousImage(client, "yo-prod-api", "registry.example.com:5000/app/api:prod", result)

	want := []string{
		"sudo docker inspect -f '{{.Image}}|{{.Config.Image}}' 'yo-prod-api' 2>/dev/null",
		"sudo docker tag 'sha256:abc123' 'registry.example.com:5000/app/api:prod-prev'",
	}
	if len(client.commandsRun) != len(want) {
		t.Fatalf("commands = %v, want %v", client.commandsRun, want)
	}
	for i := range want {
		if client.commandsRun[i] != want[i] {
			t.Errorf("commandsRun[%d] = %q, want %q", i, client.commandsRun[i], want[i])
		}
	}
	if len(result.Warnings) != 0 {
		t.Errorf("Warnings = %v, want none", result.Warnings)
	}
}

// 回归：running repo:prod + desired repo:prod-prev（回滚）绝不能 docker tag。
func TestRetainPreviousImage_SkipsWhenDesiredIsProdPrev(t *testing.T) {
	client := newScriptedSSHClient(runningProdHandler)

	RetainPreviousImage(client, "yo-prod-api", "repo/api:prod-prev", &Result{})

	for _, cmd := range client.commandsRun {
		if strings.HasPrefix(cmd, "sudo docker tag ") {
			t.Fatalf("rollback deploy must not tag; commands = %v", client.commandsRun)
		}
	}
	if len(client.commandsRun) != 0 {
		t.Errorf("commands = %v, want no remote commands at all", client.commandsRun)
	}
}

func TestRetainPreviousImage_SkipsWhenCurrentIsProdPrev(t *testing.T) {
	client := &mockSSHClient{runStdout: "sha256:abc123|registry.example.com/app/api:prod-prev"}

	RetainPreviousImage(client, "yo-prod-api", "repo/api:prod", &Result{})

	if len(client.commandsRun) != 1 {
		t.Fatalf("commands = %v, want only inspect (never overwrite prod-prev with itself)", client.commandsRun)
	}
}

// digest 形式也应被识别为回滚目标（Config.Image 可能带 @sha256）。
func TestRetainPreviousImage_SkipsWhenCurrentProdPrevHasDigest(t *testing.T) {
	client := &mockSSHClient{runStdout: "sha256:abc123|repo/api:prod-prev@sha256:deadbeef"}

	RetainPreviousImage(client, "yo-prod-api", "repo/api:prod", &Result{})

	if len(client.commandsRun) != 1 {
		t.Fatalf("commands = %v, want only inspect for digest-form self reference", client.commandsRun)
	}
}

func TestRetainPreviousImage_SkipsNoValueConfigImage(t *testing.T) {
	client := &mockSSHClient{runStdout: "sha256:abc123|<no value>"}

	RetainPreviousImage(client, "yo-prod-api", "repo/api:prod", &Result{})

	if len(client.commandsRun) != 1 {
		t.Fatalf("commands = %v, want only inspect when Config.Image is <no value>", client.commandsRun)
	}
}

func TestRetainPreviousImage_MalformedInspectOutput(t *testing.T) {
	client := &mockSSHClient{runStdout: "garbage-without-pipe"}

	RetainPreviousImage(client, "yo-prod-api", "repo/api:prod", &Result{})

	if len(client.commandsRun) != 1 {
		t.Fatalf("commands = %v, want only inspect on malformed output", client.commandsRun)
	}
}

// failingTagRunner 让 inspect 成功、tag 失败，验证打标签失败只告警不致命。
type failingTagRunner struct {
	commands []string
}

func (r *failingTagRunner) Run(cmd string) (string, string, error) {
	r.commands = append(r.commands, cmd)
	if len(r.commands) == 1 {
		return "sha256:abc123|repo/api:prod", "", nil
	}
	return "", "permission denied", errors.New("exit status 1")
}

func (r *failingTagRunner) RunWithStdin(stdin string, cmd string) (string, string, error) {
	return r.Run(cmd)
}

func TestRetainPreviousImage_TagFailureWarns(t *testing.T) {
	client := &failingTagRunner{}
	result := &Result{}

	RetainPreviousImage(client, "yo-prod-api", "repo/api:prod", result)

	if len(client.commands) != 2 {
		t.Fatalf("commands = %v, want inspect + tag", client.commands)
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("Warnings = %v, want one warning", result.Warnings)
	}
}

func TestStripImageTag(t *testing.T) {
	tests := []struct {
		ref  string
		want string
	}{
		{ref: "registry.example.com:5000/app/api:prod", want: "registry.example.com:5000/app/api"},
		{ref: "registry.example.com:5000/app/api", want: "registry.example.com:5000/app/api"},
		{ref: "repo/api@sha256:deadbeef", want: "repo/api"},
		{ref: "repo/api:prod@sha256:deadbeef", want: "repo/api"},
		{ref: "repo/api:prod", want: "repo/api"},
		{ref: "repo", want: "repo"},
		{ref: "localhost:5000/repo:tag", want: "localhost:5000/repo"},
	}
	for _, tt := range tests {
		if got := stripImageTag(tt.ref); got != tt.want {
			t.Errorf("stripImageTag(%q) = %q, want %q", tt.ref, got, tt.want)
		}
	}
}

func TestIsProdPrevRef(t *testing.T) {
	tests := []struct {
		ref  string
		want bool
	}{
		{ref: "repo/api:prod-prev", want: true},
		{ref: "repo/api:prod-prev@sha256:deadbeef", want: true},
		{ref: "repo/api:prod", want: false},
		{ref: "repo/api", want: false},
		{ref: "", want: false},
	}
	for _, tt := range tests {
		if got := isProdPrevRef(tt.ref); got != tt.want {
			t.Errorf("isProdPrevRef(%q) = %v, want %v", tt.ref, got, tt.want)
		}
	}
}

func writeTestCompose(t *testing.T) string {
	t.Helper()
	composeFile := filepath.Join(t.TempDir(), "api.compose.yaml")
	if err := os.WriteFile(composeFile, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatalf("write compose file: %v", err)
	}
	return composeFile
}

func indexOfCommand(commands []string, want string) int {
	for i, cmd := range commands {
		if cmd == want {
			return i
		}
	}
	return -1
}

func TestDeployComposeFile_RetainsImageBeforePull(t *testing.T) {
	client := newScriptedSSHClient(runningProdHandler)
	result := &Result{}
	ok := DeployComposeFile(client, &DeployComposeConfig{
		RemoteDir:     testRemoteDir,
		ComposeFile:   writeTestCompose(t),
		ContainerName: "yo-prod-api",
		Env:           "prod",
		Image:         "repo/api:prod",
	}, result)
	if !ok {
		t.Fatalf("DeployComposeFile() = false, error = %v", result.Error)
	}

	inspectCmd := "sudo docker inspect -f '{{.Image}}|{{.Config.Image}}' 'yo-prod-api' 2>/dev/null"
	tagCmd := "sudo docker tag 'sha256:abc123' 'repo/api:prod-prev'"
	idxInspect := indexOfCommand(client.commandsRun, inspectCmd)
	idxTag := indexOfCommand(client.commandsRun, tagCmd)
	idxPull := indexOfCommand(client.commandsRun, testPullCmd)
	idxUp := indexOfCommand(client.commandsRun, testUpCmd)
	if idxInspect < 0 || idxTag < 0 || idxPull < 0 || idxUp < 0 {
		t.Fatalf("commands = %v, want inspect, tag, pull and up", client.commandsRun)
	}
	if !(idxInspect < idxTag && idxTag < idxPull && idxPull < idxUp) {
		t.Errorf("order inspect=%d tag=%d pull=%d up=%d, want inspect < tag < pull < up", idxInspect, idxTag, idxPull, idxUp)
	}
}

func TestDeployComposeFile_RollbackSkipsPullWhenLocalImageExists(t *testing.T) {
	client := newScriptedSSHClient(nil) // 所有命令成功 → 本地存在 :prod-prev
	result := &Result{}
	ok := DeployComposeFile(client, &DeployComposeConfig{
		RemoteDir:   testRemoteDir,
		ComposeFile: writeTestCompose(t),
		Env:         "prod",
		Image:       "repo/api:prod-prev",
	}, result)
	if !ok {
		t.Fatalf("DeployComposeFile() = false, error = %v", result.Error)
	}

	inspectImageCmd := "sudo docker image inspect 'repo/api:prod-prev' >/dev/null 2>&1"
	if indexOfCommand(client.commandsRun, inspectImageCmd) != 0 {
		t.Fatalf("commands = %v, want image inspect first", client.commandsRun)
	}
	if indexOfCommand(client.commandsRun, testPullCmd) >= 0 {
		t.Errorf("commands = %v, must skip compose pull when retained image exists", client.commandsRun)
	}
	if indexOfCommand(client.commandsRun, testUpCmd) < 0 {
		t.Errorf("commands = %v, want up after skipping pull", client.commandsRun)
	}
}

func TestDeployComposeFile_RollbackPullsWhenLocalImageMissing(t *testing.T) {
	client := newScriptedSSHClient(func(cmd string) (string, string, error) {
		if strings.HasPrefix(cmd, "sudo docker image inspect ") {
			return "", "", errors.New("no such image")
		}
		return "", "", nil
	})
	result := &Result{}
	ok := DeployComposeFile(client, &DeployComposeConfig{
		RemoteDir:   testRemoteDir,
		ComposeFile: writeTestCompose(t),
		Env:         "prod",
		Image:       "repo/api:prod-prev",
	}, result)
	if !ok {
		t.Fatalf("DeployComposeFile() = false, error = %v", result.Error)
	}

	inspectImageCmd := "sudo docker image inspect 'repo/api:prod-prev' >/dev/null 2>&1"
	idxInspect := indexOfCommand(client.commandsRun, inspectImageCmd)
	idxPull := indexOfCommand(client.commandsRun, testPullCmd)
	idxUp := indexOfCommand(client.commandsRun, testUpCmd)
	if idxInspect < 0 || idxPull < 0 || idxUp < 0 {
		t.Fatalf("commands = %v, want image inspect, pull and up", client.commandsRun)
	}
	if !(idxInspect < idxPull && idxPull < idxUp) {
		t.Errorf("order inspect=%d pull=%d up=%d, want inspect < pull < up", idxInspect, idxPull, idxUp)
	}
}

func TestDeployComposeFile_TagFailureStillPullsAndUps(t *testing.T) {
	client := newScriptedSSHClient(func(cmd string) (string, string, error) {
		switch {
		case strings.HasPrefix(cmd, "sudo docker inspect -f"):
			return "sha256:abc123|repo/api:prod", "", nil
		case strings.HasPrefix(cmd, "sudo docker tag "):
			return "", "permission denied", errors.New("exit status 1")
		default:
			return "", "", nil
		}
	})
	result := &Result{}
	ok := DeployComposeFile(client, &DeployComposeConfig{
		RemoteDir:     testRemoteDir,
		ComposeFile:   writeTestCompose(t),
		ContainerName: "yo-prod-api",
		Env:           "prod",
		Image:         "repo/api:prod",
	}, result)
	if !ok {
		t.Fatalf("DeployComposeFile() = false, want tag failure to be non-fatal; error = %v", result.Error)
	}
	if len(result.Warnings) != 1 {
		t.Errorf("Warnings = %v, want one tag warning", result.Warnings)
	}
	if indexOfCommand(client.commandsRun, testPullCmd) < 0 || indexOfCommand(client.commandsRun, testUpCmd) < 0 {
		t.Errorf("commands = %v, want pull and up after tag failure", client.commandsRun)
	}
}

func TestDeployComposeFile_DoesNotRetainOutsideProd(t *testing.T) {
	client := newScriptedSSHClient(runningProdHandler)
	result := &Result{}
	ok := DeployComposeFile(client, &DeployComposeConfig{
		RemoteDir:     testRemoteDir,
		ComposeFile:   writeTestCompose(t),
		ContainerName: "yo-dev-api",
		Env:           "dev",
		Image:         "repo/api:dev",
	}, result)
	if !ok {
		t.Fatalf("DeployComposeFile() = false, error = %v", result.Error)
	}
	for _, cmd := range client.commandsRun {
		if strings.HasPrefix(cmd, "sudo docker tag ") {
			t.Fatalf("non-prod deploy must not tag prod-prev; commands = %v", client.commandsRun)
		}
		if strings.HasPrefix(cmd, "sudo docker inspect -f") {
			t.Fatalf("non-prod deploy must not inspect container; commands = %v", client.commandsRun)
		}
	}
	if indexOfCommand(client.commandsRun, testPullCmd) < 0 {
		t.Errorf("commands = %v, want pull on non-prod deploy", client.commandsRun)
	}
}
