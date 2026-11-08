package usecase

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/lite-lake/infra-yamlops/internal/constants"
	domainerr "github.com/lite-lake/infra-yamlops/internal/domain"
	"github.com/lite-lake/infra-yamlops/internal/domain/contract"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/ssh"
)

// LogSource 标识一次日志查询读取的是当前容器还是历史归档。
type LogSource string

const (
	LogSourceCurrent  LogSource = "current"
	LogSourceArchived LogSource = "archived"
)

// LogOptions 描述一次容器日志查询的参数。
type LogOptions struct {
	Tail       int    // 末尾行数；<=0 表示全部
	Since      string // docker --since（相对时长或 RFC3339）
	Until      string // docker --until
	Timestamps bool   // 显示 docker 时间戳
	Grep       string // 服务端 grep 过滤（大小写不敏感）
	RunID      string // 非空表示读取指定归档（如 20260828T101530-abc123）
}

// ContainerStatus 是 docker inspect 采集到的容器状态摘要。
type ContainerStatus struct {
	Name         string
	ID           string
	State        string
	RestartCount int
	StartedAt    string
	FinishedAt   string
	Exists       bool
}

// LogEntry 是一次日志读取的结果。
type LogEntry struct {
	ServiceName string
	ServerName  string
	Container   string
	Source      LogSource
	ArchiveID   string // 仅 Source == archived 时有值
	Status      *ContainerStatus
	Content     string
}

// LogTarget 一个待查日志的服务（含服务器定位）。
type LogTarget struct {
	ServiceName string
	ServerName  string
}

// ContainerName 由服务名构造固定容器名 yo-{env}-{service}。
func ContainerName(env, serviceName string) string {
	return fmt.Sprintf(constants.ServiceNameFormat, env, serviceName)
}

// ServiceLogsDir 服务日志归档目录（该服务自己的 compose 目录下）。
func ServiceLogsDir(remoteDir string) string {
	return remoteDir + "/" + constants.ServiceLogsDirName
}

// BuildDockerLogsCommand 构造 docker logs 命令（不含 follow）。
func BuildDockerLogsCommand(container string, opts LogOptions) string {
	parts := []string{"sudo docker logs"}
	if opts.Tail > 0 {
		parts = append(parts, "--tail", fmt.Sprintf("%d", opts.Tail))
	}
	if opts.Since != "" {
		parts = append(parts, "--since", ssh.ShellEscape(opts.Since))
	}
	if opts.Until != "" {
		parts = append(parts, "--until", ssh.ShellEscape(opts.Until))
	}
	if opts.Timestamps {
		parts = append(parts, "--timestamps")
	}
	parts = append(parts, ssh.ShellEscape(container), "2>&1")
	cmd := strings.Join(parts, " ")
	if opts.Grep != "" {
		cmd = fmt.Sprintf("%s | grep -i -F %s || true", cmd, ssh.ShellEscape(opts.Grep))
	}
	return cmd
}

// BuildContainerInspectCommand 构造容器状态采集命令（一次拿到状态四元组）。
// 容器不存在时输出 "missing"。
func BuildContainerInspectCommand(container string) string {
	escaped := ssh.ShellEscape(container)
	return fmt.Sprintf(
		`sudo docker inspect -f '{{.Id}} {{.State.Status}} {{.RestartCount}} {{.State.StartedAt}} {{.State.FinishedAt}}' %s 2>/dev/null || echo missing`,
		escaped,
	)
}

// ParseContainerInspectOutput 解析 inspect 输出为 ContainerStatus。
func ParseContainerInspectOutput(output string) *ContainerStatus {
	line := strings.TrimSpace(output)
	if line == "" || line == "missing" {
		return &ContainerStatus{Exists: false}
	}
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return &ContainerStatus{Exists: false}
	}
	rc := 0
	fmt.Sscanf(fields[2], "%d", &rc)
	return &ContainerStatus{
		ID:           strings.TrimPrefix(fields[0], "sha256:"),
		State:        fields[1],
		RestartCount: rc,
		StartedAt:    fields[3],
		FinishedAt:   fields[4],
		Exists:       true,
	}
}

// FetchContainerLogs 读取当前容器日志（一次性，不支持 follow）。
func FetchContainerLogs(client contract.SSHClient, container string, opts LogOptions) (string, *ContainerStatus, error) {
	inspectOut, _, err := client.Run(BuildContainerInspectCommand(container))
	if err != nil {
		return "", nil, fmt.Errorf("%w: inspect %s: %w", domainerr.ErrContainerLogFailed, container, err)
	}
	status := ParseContainerInspectOutput(inspectOut)
	if !status.Exists {
		return "", status, fmt.Errorf("%w: %s", domainerr.ErrContainerNotFound, container)
	}

	logOut, _, err := client.Run(BuildDockerLogsCommand(container, opts))
	if err != nil {
		// docker logs 命令本身带 `2>&1` 且 grep 已 `|| true`，正常不应失败；
		// 失败通常是 docker daemon 异常，原样上抛。
		return "", status, fmt.Errorf("%w: docker logs %s: %w", domainerr.ErrContainerLogFailed, container, err)
	}
	return logOut, status, nil
}

// FetchArchivedLogs 读取一份历史归档日志。runID 形如 20260828T101530-abc123。
func FetchArchivedLogs(client contract.SSHClient, remoteDir, runID string) (string, error) {
	if !validArchiveID(runID) {
		return "", fmt.Errorf("%w: invalid run id %q", domainerr.ErrLogArchiveNotFound, runID)
	}
	archivePath := fmt.Sprintf("%s/%s.log", ServiceLogsDir(remoteDir), ssh.ShellEscape(runID))
	stdout, stderr, err := client.Run(fmt.Sprintf("sudo cat %s 2>/dev/null || echo __missing__", archivePath))
	if err != nil {
		return "", fmt.Errorf("%w: read %s: %w", domainerr.ErrLogArchiveNotFound, runID, err)
	}
	if strings.TrimSpace(stdout) == "__missing__" {
		return "", fmt.Errorf("%w: %s", domainerr.ErrLogArchiveNotFound, runID)
	}
	_ = stderr
	return stdout, nil
}

// validArchiveID 限制 runID 字符集（含大写：时间戳格式带 T/Z），防止路径穿越。
func validArchiveID(runID string) bool {
	if runID == "" || len(runID) > 64 {
		return false
	}
	for _, c := range runID {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

// ArchiveRun 一份归档的元信息。
type ArchiveRun struct {
	ArchiveID string
	FileName  string
}

// ListArchiveRuns 列出某服务的历史归档（按时间倒序，最新在前）。
func ListArchiveRuns(client contract.SSHClient, remoteDir string) ([]ArchiveRun, error) {
	logsDir := ServiceLogsDir(remoteDir)
	stdout, _, err := client.Run(fmt.Sprintf("sudo ls -1 %s 2>/dev/null || true", ssh.ShellEscape(logsDir)))
	if err != nil {
		return nil, fmt.Errorf("%w: list %s: %w", domainerr.ErrLogsArchiveFailed, logsDir, err)
	}
	var runs []ArchiveRun
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasSuffix(line, ".log") {
			continue
		}
		id := strings.TrimSuffix(line, ".log")
		if !validArchiveID(id) {
			continue
		}
		runs = append(runs, ArchiveRun{ArchiveID: id, FileName: line})
	}
	sort.Slice(runs, func(i, j int) bool { return runs[i].ArchiveID > runs[j].ArchiveID })
	return runs, nil
}

// ArchiveContainerLogs 在容器被销毁/重启前导出其全部日志到归档目录。
// 返回归档文件名；容器不存在时返回空串与 nil（无日志可归档不是错误）。
func ArchiveContainerLogs(client contract.SSHClient, remoteDir, container string, now time.Time) (string, error) {
	inspectOut, _, err := client.Run(BuildContainerInspectCommand(container))
	if err != nil {
		return "", fmt.Errorf("%w: inspect %s: %w", domainerr.ErrLogsArchiveFailed, container, err)
	}
	status := ParseContainerInspectOutput(inspectOut)
	if !status.Exists {
		return "", nil
	}

	logsDir := ServiceLogsDir(remoteDir)
	archiveID := fmt.Sprintf("%s-%s", now.UTC().Format(constants.LogArchiveTimeFormat), shortID(status.ID))
	archivePath := fmt.Sprintf("%s/%s.log", logsDir, archiveID)

	// mkdir + 原子导出：先写临时文件再 mv，避免归档一半的文件被当作完整日志。
	// 整条命令必须包在 sudo sh -c 里：`>` 重定向由远端 shell 执行，
	// 若在用户上下文展开会因 /data 归 root 而 Permission denied。
	cmd := fmt.Sprintf(
		`sudo sh -c 'mkdir -p %s && docker logs --timestamps %s > %s.tmp 2>&1 && mv %s.tmp %s'`,
		ssh.ShellEscape(logsDir),
		ssh.ShellEscape(container),
		archivePath, archivePath, archivePath,
	)
	if _, stderr, err := client.Run(cmd); err != nil {
		return "", fmt.Errorf("%w: %s: %w, stderr: %s", domainerr.ErrLogsArchiveFailed, container, err, stderr)
	}
	return archiveID, nil
}

// PruneArchiveRuns 按保留份数清理旧归档（保留最新 maxKeep 份）。
func PruneArchiveRuns(client contract.SSHClient, remoteDir string, maxKeep int) error {
	if maxKeep <= 0 {
		maxKeep = constants.MaxLogsArchiveCount
	}
	runs, err := ListArchiveRuns(client, remoteDir)
	if err != nil {
		return err
	}
	if len(runs) <= maxKeep {
		return nil
	}
	logsDir := ServiceLogsDir(remoteDir)
	for _, run := range runs[maxKeep:] {
		path := fmt.Sprintf("%s/%s.log", logsDir, ssh.ShellEscape(run.ArchiveID))
		client.Run(fmt.Sprintf("sudo rm -f %s", path))
	}
	return nil
}

// shortID 取容器 ID 前 12 位。
func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// FetchServiceLogsForServer 为 TUI 的逐服务扫描获取一份服务的日志视图
// （状态 + tail 内容 + 归档列表，单次 SSH 连接完成）。
type ServiceLogView struct {
	ServiceName string
	ServerName  string
	Container   string
	Status      *ContainerStatus
	Content     string
	Err         string
	Archives    []string // 归档 id，最新在前
}

func FetchServiceLogsForServer(client contract.SSHClient, env, serviceName, serverName, remoteDir string, opts LogOptions) *ServiceLogView {
	view := &ServiceLogView{
		ServiceName: serviceName,
		ServerName:  serverName,
		Container:   ContainerName(env, serviceName),
	}
	container := ContainerName(env, serviceName)

	inspectOut, _, err := client.Run(BuildContainerInspectCommand(container))
	if err != nil {
		view.Err = fmt.Sprintf("inspect failed: %v", err)
		return view
	}
	view.Status = ParseContainerInspectOutput(inspectOut)

	if view.Status.Exists {
		content, _, err := FetchContainerLogs(client, container, opts)
		if err != nil {
			view.Err = err.Error()
		} else {
			view.Content = strings.TrimRight(content, "\n")
		}
	}

	if runs, err := ListArchiveRuns(client, remoteDir); err == nil {
		for _, r := range runs {
			view.Archives = append(view.Archives, r.ArchiveID)
		}
	}
	return view
}

// CollectServiceLogs 是日志查询的核心入口：并发拉取多个服务的日志。
// connect 由调用方注入（负责解析服务器 SSH 配置并建连），remoteDirOf 给出服务的远端目录。
// 返回按输入顺序排列的 LogEntry（错误信息以 "ERROR: " 前缀写入 Content）。
func CollectServiceLogs(
	ctx context.Context,
	env string,
	connect func(serverName string) (contract.SSHClient, error),
	targets []LogTarget,
	remoteDirOf func(serviceName string) string,
	opts LogOptions,
	concurrency int,
) []LogEntry {
	if concurrency <= 0 {
		concurrency = 5
	}
	entries := make([]LogEntry, len(targets))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, t := range targets {
		sem <- struct{}{}
		wg.Add(1)
		go func(idx int, tgt LogTarget) {
			defer wg.Done()
			defer func() { <-sem }()

			entry := LogEntry{
				ServiceName: tgt.ServiceName,
				ServerName:  tgt.ServerName,
				Container:   "",
				Source:      LogSourceCurrent,
			}

			select {
			case <-ctx.Done():
				entry.Content = "ERROR: cancelled"
				entries[idx] = entry
				return
			default:
			}

			client, err := connect(tgt.ServerName)
			if err != nil {
				entry.Content = fmt.Sprintf("ERROR: connect failed: %v", err)
				entries[idx] = entry
				return
			}

			remoteDir := remoteDirOf(tgt.ServiceName)
			entry.Container = ContainerName(env, tgt.ServiceName)

			if opts.RunID != "" {
				content, err := FetchArchivedLogs(client, remoteDir, opts.RunID)
				if err != nil {
					entry.Content = fmt.Sprintf("ERROR: %v", err)
				} else {
					entry.Source = LogSourceArchived
					entry.ArchiveID = opts.RunID
					entry.Content = content
				}
				entries[idx] = entry
				return
			}

			content, status, err := FetchContainerLogs(client, entry.Container, opts)
			entry.Status = status
			if err != nil {
				entry.Content = fmt.Sprintf("ERROR: %v", err)
			} else {
				entry.Content = content
			}
			entries[idx] = entry
		}(i, t)
	}
	wg.Wait()
	return entries
}
