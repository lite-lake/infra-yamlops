package environment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lite-lake/infra-yamlops/internal/constants"
	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
	"github.com/lite-lake/infra-yamlops/internal/infrastructure/ssh"
)

// ---------------------------------------------------------------------------
// Remote segment markers (one marker = one command result block in the output)
// ---------------------------------------------------------------------------

const (
	segHostCPU1 = "HOST_CPU_1"
	segMeminfo  = "MEMINFO"
	segStats    = "DOCKER_STATS"
	segHostCPU2 = "HOST_CPU_2"
	segLoadavg  = "LOADAVG"
	segDisk     = "DISK"
	segPS       = "DOCKER_PS"
	segImages   = "IMAGES"
	segDirs     = "DIRS"
	segDockerDF = "DOCKER_DF"
)

const markerDone = "@@DONE"

// segMarkerNames is the allow-list of recognized markers (bare names, no "@@").
// A content line that merely begins with "@@" (collision sample) is treated as
// content, never as a boundary, unless its name is in this set.
var segMarkerNames = map[string]bool{
	segHostCPU1: true, segMeminfo: true, segStats: true, segHostCPU2: true,
	segLoadavg: true, segDisk: true, segPS: true, segImages: true, segDirs: true,
	segDockerDF: true,
}

// rcSegmentMap maps a segment key to its trailing RC marker name. A missing RC
// for a segment that carries one means the command never finished (truncation).
var rcSegmentMap = map[string]string{
	"disk":         "disk_rc",
	"docker_stats": "stats_rc",
	"docker_ps":    "ps_rc",
	"images":       "images_rc",
	"dirs":         "dirs_rc",
	"docker_df":    "docker_df_rc",
}

// SegmentState is the three-state truthfulness contract of a collected segment.
//
//	ok      - data complete
//	empty   - legitimate zero value (no containers / no dirs / CPU window too short)
//	failed  - marker missing / invalid content / command failed or timed out
type SegmentState int

const (
	SegOK SegmentState = iota
	SegEmpty
	SegFailed
)

func (s SegmentState) String() string {
	switch s {
	case SegOK:
		return "ok"
	case SegEmpty:
		return "empty"
	default:
		return "failed"
	}
}

// MarshalJSON serializes the segment state as a plain "ok"|"empty"|"failed".
func (s SegmentState) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// ---------------------------------------------------------------------------
// Data model (contract §5.1)
// ---------------------------------------------------------------------------

// HostResources is the host-level summary for one server. It is non-nil only
// when none of host_cpu/meminfo/loadavg is failed.
type HostResources struct {
	CPUCores      int      // nproc
	CPUUsagePct   *float64 // two /proc/stat deltas; nil = window too short or failed
	StealPct      *float64 // stolen CPU time (separate column to explain VM stalls)
	Load1_5_15    [3]float64
	MemTotalBytes uint64
	MemUsedBytes  uint64
	MemTotalHuman string
	MemUsedHuman  string
}

// DiskMount is a single real mount point. df -B1 provides block counts that are
// already in bytes, so no ×1024 is needed.
type DiskMount struct {
	MountPoint string
	TotalBytes uint64
	UsedBytes  uint64
	UsePercent float64
	TotalHuman string
	UsedHuman  string
}

// ServiceResourceInfo is the per-service resource detail for one managed service.
type ServiceResourceInfo struct {
	Name       string
	Status     string // running / exited / not_deployed / -
	CPUPercent string // verbatim docker stats CPUPerc, "-" if missing
	MemUsage   string // "128MiB / 4GiB"
	MemPercent string
	ImageSize  string
	RwSize     string
	DirSize    string

	MemUsedBytes  *uint64
	MemTotalBytes *uint64
	DirBytes      *uint64
}

// UnmanagedContainer is a running container that is not managed by yamlops.
type UnmanagedContainer struct {
	Names  string
	Image  string
	Status string
	SizeRw string
}

// ServerResourceReport is the complete report for one server.
type ServerResourceReport struct {
	ServerName string
	Zone       string
	Env        string
	ScannedAt  string
	Host       *HostResources
	Disks      []DiskMount
	Services   []ServiceResourceInfo
	Unmanaged  []UnmanagedContainer
	DockerDF   *DockerDiskUsage
	Segments   map[string]SegmentState
	Degraded   bool
	ScanError  string

	// unexported raw parsed pieces, consumed by BuildServiceReports and never
	// serialized.
	containers []containerPS
	stats      []containerStats
	images     []imageEntry
	dirs       map[string]*uint64
}

// ContainerRunning returns the number of running containers seen on the host.
func (r *ServerResourceReport) ContainerRunning() int {
	n := 0
	for _, c := range r.containers {
		if c.State == "running" {
			n++
		}
	}
	return n
}

// ContainerTotal returns the total number of containers (including stopped).
func (r *ServerResourceReport) ContainerTotal() int { return len(r.containers) }

// DirTotalBytes returns the sum of all managed service dirs, nil if none.
func (r *ServerResourceReport) DirTotalBytes() *uint64 {
	var total uint64
	found := false
	for _, b := range r.dirs {
		if b != nil && *b > 0 {
			total += *b
			found = true
		}
	}
	if !found {
		return nil
	}
	return &total
}

// UnmatchedImageCount returns the number of docker images that could not be
// attributed to any managed service (registry/digest normalization misses).
func (r *ServerResourceReport) UnmatchedImageCount() int {
	matched := 0
	for _, svc := range r.Services {
		if svc.ImageSize != "-" && svc.ImageSize != "" {
			matched++
		}
	}
	if len(r.images) <= matched {
		return 0
	}
	return len(r.images) - matched
}

// ---------------------------------------------------------------------------
// Inspector
// ---------------------------------------------------------------------------

// Inspector collects and parses server resource snapshots over a single SSH
// session using pure shell + JSON output parsing (no Docker SDK, no agent).
type Inspector struct {
	client *ssh.Client
	env    string
}

// NewInspector builds an Inspector bound to an established SSH client.
func NewInspector(client *ssh.Client, env string) *Inspector {
	return &Inspector{client: client, env: env}
}

// Inspect runs one collection pass and returns the full report.
//
// Connection-level errors (unreachable / auth failure) do NOT go through the
// segment mechanism; they are surfaced via ScanError so the CLI/TUI can render
// the server as failed rather than as idle.
func (i *Inspector) Inspect(ctx context.Context, cfg *entity.Config, srv *entity.Server, secrets map[string]string) (*ServerResourceReport, error) {
	report := &ServerResourceReport{
		ServerName: srv.Name,
		Zone:       srv.Zone,
		Env:        i.env,
		ScannedAt:  time.Now().UTC().Format(time.RFC3339),
		Segments:   make(map[string]SegmentState),
		Disks:      []DiskMount{},
		Services:   []ServiceResourceInfo{},
		Unmanaged:  []UnmanagedContainer{},
		dirs:       map[string]*uint64{},
	}

	cmd := BuildInspectCommand(i.env)
	output, err := collectStream(ctx, i.client, cmd)
	if err != nil {
		report.ScanError = fmt.Sprintf("collection failed: %v", err)
		report.Degraded = true
		return report, nil
	}

	raw := SplitSegments(output)

	if !raw.DoneSeen || len(output) == 0 {
		report.ScanError = "output truncated"
		// Any RC-bearing command that never reached its RC marker is failed.
		for key, rcKey := range rcSegmentMap {
			if _, ok := raw.RCs[rcKey]; !ok {
				report.Segments[key] = SegFailed
			}
		}
	}

	// helper to read rc for a segment; notPresent => command never finished
	rc := func(rcKey string) (int, bool) {
		v, ok := raw.RCs[rcKey]
		return v, ok
	}

	// ---- host_cpu ----
	hostCPUState := SegOK
	var cpuBusy, cpuSteal float64
	windowErr := error(nil)
	hostCPUState = SegOK
	if err := i.parseCPU(&raw, &cpuBusy, &cpuSteal, &hostCPUState); err != nil {
		windowErr = err
	}
	_ = windowErr

	// ---- meminfo ----
	memState := SegOK
	var memUsed, memTotal uint64
	memUsed, memTotal, err = ParseMeminfo(raw.Segments[segMeminfo])
	if err != nil {
		memState = SegFailed
	}

	// ---- loadavg ----
	loadState := SegOK
	var loads [3]float64
	var nproc int
	loads, nproc, err = ParseLoadAndCores(raw.Segments[segLoadavg])
	if err != nil {
		loadState = SegFailed
	}

	report.setHost(hostCPUState, memState, loadState, cpuBusy, cpuSteal, memUsed, memTotal, loads, nproc)
	report.Segments["host_cpu"] = hostCPUState
	report.Segments["meminfo"] = memState
	report.Segments["loadavg"] = loadState

	// ---- disk ----
	diskState := SegFailed
	if pre, ok := report.Segments["disk"]; ok && pre == SegFailed {
		diskState = SegFailed
	} else if dRC, present := rc("disk_rc"); present {
		report.Disks, diskState, _ = ParseDiskDF(raw.Segments[segDisk], dRC)
	}
	report.Segments["disk"] = diskState

	// ---- docker_ps ----
	psState := SegFailed
	psRunCount := -1
	if pre, ok := report.Segments["docker_ps"]; ok && pre == SegFailed {
		psState = SegFailed
	} else if pRC, present := rc("ps_rc"); present {
		report.containers, psState, _ = ParseDockerPS(raw.Segments[segPS], pRC)
		if psState == SegOK {
			psRunCount = 0
			for _, c := range report.containers {
				if c.State == "running" {
					psRunCount++
				}
			}
		}
	}
	report.Segments["docker_ps"] = psState

	// ---- docker_stats (cross-validated against ps) ----
	statsState := SegFailed
	if pre, ok := report.Segments["docker_stats"]; ok && pre == SegFailed {
		statsState = SegFailed
	} else if sRC, present := rc("stats_rc"); present {
		report.stats, statsState, _ = ParseDockerStats(raw.Segments[segStats], sRC, psRunCount)
	}
	report.Segments["docker_stats"] = statsState

	// ---- images ----
	imagesState := SegFailed
	if pre, ok := report.Segments["images"]; ok && pre == SegFailed {
		imagesState = SegFailed
	} else if iRC, present := rc("images_rc"); present {
		report.images, imagesState = ParseImages(raw.Segments[segImages], iRC)
	}
	report.Segments["images"] = imagesState

	// ---- dirs ----
	dirsState := SegFailed
	if pre, ok := report.Segments["dirs"]; ok && pre == SegFailed {
		dirsState = SegFailed
	} else if dRC, present := rc("dirs_rc"); present {
		report.dirs, dirsState = ParseDirSizes(raw.Segments[segDirs], dRC, i.env)
	}
	report.Segments["dirs"] = dirsState

	// ---- docker system df (DockerDiskUsage reuse) ----
	dfState := SegFailed
	if pre, ok := report.Segments["docker_df"]; ok && pre == SegFailed {
		dfState = SegFailed
	} else if fRC, present := rc("docker_df_rc"); present {
		if fRC == 0 {
			if usage, derr := parseDockerDiskUsage(raw.Segments[segDockerDF]); derr == nil {
				report.DockerDF = usage
				dfState = SegOK
			}
		}
	}
	report.Segments["docker_df"] = dfState

	// ---- degraded flag ----
	report.Degraded = report.ScanError != ""
	for _, st := range report.Segments {
		if st == SegFailed {
			report.Degraded = true
			break
		}
	}

	report.BuildServiceReports(cfg, i.env)
	return report, nil
}

// parseCPU parses the two /proc/stat+uptime windows into busy/steal percentages.
func (i *Inspector) parseCPU(raw *RawOutput, busyPct, stealPct *float64, state *SegmentState) error {
	s1 := CPUSample{StatLine: raw.Segments[segHostCPU1], UptimeLine: cpuUptimeLine(raw.Segments[segHostCPU1])}
	s2 := CPUSample{StatLine: raw.Segments[segHostCPU2], UptimeLine: cpuUptimeLine(raw.Segments[segHostCPU2])}
	if strings.TrimSpace(raw.Segments[segHostCPU1]) == "" || strings.TrimSpace(raw.Segments[segHostCPU2]) == "" {
		*state = SegFailed
		return nil
	}
	b, st, err := ParseCPUWindow(s1, s2)
	if err != nil {
		if errors.Is(err, ErrCPUSampleWindowTooShort) {
			*state = SegEmpty
			return nil
		}
		*state = SegFailed
		return nil
	}
	*busyPct, *stealPct = b, st
	*state = SegOK
	return nil
}

func cpuUptimeLine(seg string) string {
	lines := strings.Split(seg, "\n")
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if t != "" && !strings.HasPrefix(t, "cpu") {
			if _, err := strconv.ParseFloat(strings.Fields(t)[0], 64); err == nil {
				return t
			}
		}
	}
	return ""
}

func (r *ServerResourceReport) setHost(cpuState, memState, loadState SegmentState,
	cpuBusy, cpuSteal float64, memUsed, memTotal uint64, loads [3]float64, nproc int) {

	if cpuState == SegFailed || memState == SegFailed || loadState == SegFailed {
		r.Host = nil
		return
	}
	h := &HostResources{
		CPUCores:      nproc,
		Load1_5_15:    loads,
		MemTotalBytes: memTotal,
		MemUsedBytes:  memUsed,
		MemTotalHuman: humanBytes(memTotal),
		MemUsedHuman:  humanBytes(memUsed),
	}
	if cpuState == SegOK {
		h.CPUUsagePct = &cpuBusy
		h.StealPct = &cpuSteal
	}
	r.Host = h
}

// ---------------------------------------------------------------------------
// BuildInspectCommand
// ---------------------------------------------------------------------------

// BuildInspectCommand assembles the single-session composite command. Every
// risky command is wrapped in timeout, each trailing RC marker distinguishes
// "failed/timed out" from "succeeded but empty", and @@DONE provides a global
// arrival check. Paths and env are injected from constants — never hardcoded.
func BuildInspectCommand(env string) string {
	dirGlob := fmt.Sprintf("%s/%s", constants.RemoteBaseDir, fmt.Sprintf(constants.ServiceNameFormat, env, "*"))
	return fmt.Sprintf(`( echo '@@%s'; grep '^cpu ' /proc/stat; cat /proc/uptime;
  echo '@@%s'; grep -E '^(MemTotal|MemAvailable|Buffers|Cached|MemFree)' /proc/meminfo;
  echo '@@%s'; timeout 10 sudo docker stats --no-stream --format '{{json .}}'; echo "stats_rc=$?";
  echo '@@%s'; grep '^cpu ' /proc/stat; cat /proc/uptime;
  echo '@@%s'; cat /proc/loadavg; nproc;
  echo '@@%s'; timeout 5 df -P -B1 -l -x tmpfs -x devtmpfs -x overlay -x squashfs; echo "disk_rc=$?";
  echo '@@%s'; timeout 10 sudo docker system df --format '{{json .}}'; echo "docker_df_rc=$?";
  echo '@@%s'; timeout 20 sudo docker ps -a -s --format '{{json .}}'; echo "ps_rc=$?";
  echo '@@%s'; timeout 10 sudo docker images --format '{{json .}}'; echo "images_rc=$?";
  echo '@@%s'; timeout 30 sudo du -sb %s/ ; echo "dirs_rc=$?";
  echo '@@DONE' ) 2>&1`,
		segHostCPU1, segMeminfo, segStats, segHostCPU2, segLoadavg, segDisk, segDockerDF, segPS, segImages, segDirs, dirGlob)
}

// ---------------------------------------------------------------------------
// SplitSegments
// ---------------------------------------------------------------------------

// RawOutput is the split form of one SSH stream: content per segment, RC codes,
// and whether the global @@DONE marker arrived.
type RawOutput struct {
	Segments map[string]string
	RCs      map[string]int
	DoneSeen bool
}

// SplitSegments cuts the stream by recognized @@ markers, strips RC lines, and
// records the DONE arrival flag.
func SplitSegments(output string) RawOutput {
	raw := RawOutput{
		Segments: make(map[string]string),
		RCs:      make(map[string]int),
	}
	lines := strings.Split(output, "\n")
	cur := ""
	for _, line := range lines {
		line = strings.TrimSuffix(line, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == markerDone {
			raw.DoneSeen = true
			cur = ""
			continue
		}
		if strings.HasPrefix(trimmed, "@@") {
			name := trimmed[2:]
			if segMarkerNames[name] {
				cur = name
				continue
			}
			// Unrecognized @@ line: falls through as content when inside a segment
		}
		if cur == "" {
			continue // content before the first marker is discarded
		}
		if key, code, ok := parseRCLine(trimmed); ok {
			raw.RCs[key] = code
			continue
		}
		raw.Segments[cur] += line + "\n"
	}
	return raw
}

// parseRCLine matches a trailing `<key>_rc=<line>` line, e.g. "disk_rc=124".
// The returned key includes the "_rc" suffix (so report lookups use "disk_rc").
func parseRCLine(line string) (string, int, bool) {
	idx := strings.Index(line, "_rc=")
	if idx <= 0 {
		return "", 0, false
	}
	key := line[:idx+len("_rc")]
	val := line[idx+len("_rc="):]
	code, err := strconv.Atoi(strings.TrimSpace(val))
	if err != nil {
		return "", 0, false
	}
	for _, ch := range key {
		if !(ch >= 'a' && ch <= 'z') && !(ch >= 'A' && ch <= 'Z') && !(ch >= '0' && ch <= '9') && ch != '_' {
			return "", 0, false
		}
	}
	return strings.ToLower(key), code, true
}

// ---------------------------------------------------------------------------
// Parse pure-function family
// ---------------------------------------------------------------------------

// CPUSample carries one /proc/stat sample plus its accompanying /proc/uptime.
type CPUSample struct {
	StatLine   string
	UptimeLine string
}

// ErrCPUSampleWindowTooShort marks a sampling window narrower than 0.5s, where
// deltas would be noise (SegEmpty, not a failure).
var ErrCPUSampleWindowTooShort = errors.New("cpu sample window too short")

// ParseCPUWindow computes busy/steal percentages from two CPU samples.
func ParseCPUWindow(s1, s2 CPUSample) (busyPct, stealPct float64, err error) {
	up1, err1 := parseUptime(s1.UptimeLine)
	up2, err2 := parseUptime(s2.UptimeLine)
	if err1 != nil || err2 != nil {
		return 0, 0, fmt.Errorf("uptime parse failed: %v / %v", err1, err2)
	}
	if up2 < up1 || up2-up1 < 0.5 {
		return 0, 0, ErrCPUSampleWindowTooShort
	}
	c1, err := parseCPUStat(s1.StatLine)
	if err != nil {
		return 0, 0, fmt.Errorf("stat1 parse failed: %w", err)
	}
	c2, err := parseCPUStat(s2.StatLine)
	if err != nil {
		return 0, 0, fmt.Errorf("stat2 parse failed: %w", err)
	}
	totalDelta := c2.total - c1.total
	if totalDelta == 0 {
		return 0, 0, errors.New("cpu total delta is zero")
	}
	busyDelta := int64(c2.total-c2.idle-c2.iowait) - int64(c1.total-c1.idle-c1.iowait)
	stealDelta := int64(c2.steal) - int64(c1.steal)
	busyPct = float64(busyDelta) / float64(totalDelta) * 100
	stealPct = float64(stealDelta) / float64(totalDelta) * 100
	if busyPct < 0 {
		busyPct = 0
	}
	if stealPct < 0 {
		stealPct = 0
	}
	return busyPct, stealPct, nil
}

type cpuCounters struct {
	total, idle, iowait, steal uint64
}

func parseCPUStat(line string) (cpuCounters, error) {
	f := strings.Fields(strings.TrimSpace(line))
	if len(f) < 5 || f[0] != "cpu" {
		return cpuCounters{}, errors.New("invalid cpu stat line")
	}
	vars := make([]uint64, 9)
	for i := 1; i < len(f) && i <= 8; i++ {
		v, err := strconv.ParseUint(f[i], 10, 64)
		if err != nil {
			return cpuCounters{}, fmt.Errorf("invalid cpu field %q", f[i])
		}
		vars[i-1] = v
	}
	// user, nice, system, idle, iowait, irq, softirq, steal
	total := vars[0] + vars[1] + vars[2] + vars[3] + vars[4] + vars[5] + vars[6] + vars[7]
	return cpuCounters{total: total, idle: vars[3], iowait: vars[4], steal: vars[7]}, nil
}

func parseUptime(line string) (float64, error) {
	f := strings.Fields(strings.TrimSpace(line))
	if len(f) == 0 {
		return 0, errors.New("empty uptime line")
	}
	v, err := strconv.ParseFloat(f[0], 64)
	if err != nil {
		return 0, err
	}
	return v, nil
}

// ParseMeminfo parses /proc/meminfo into used/total bytes.
func ParseMeminfo(raw string) (used, total uint64, err error) {
	values := make(map[string]uint64)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		var key string
		switch {
		case strings.HasPrefix(line, "Mem"):
			key = strings.TrimSpace(line[:strings.Index(line, ":")])
		case strings.HasPrefix(line, "Buffers:"), strings.HasPrefix(line, "Cached:"):
			key = line[:strings.Index(line, ":")]
		default:
			continue
		}
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		valStr := strings.Fields(strings.TrimSpace(line[idx+1:]))
		if len(valStr) == 0 {
			continue
		}
		num, perr := strconv.ParseUint(valStr[0], 10, 64)
		if perr != nil {
			continue
		}
		if len(valStr) > 1 && strings.EqualFold(valStr[1], "kB") {
			num *= 1024
		}
		values[key] = num
	}
	total = values["MemTotal"]
	if total == 0 {
		return 0, 0, errors.New("meminfo missing MemTotal")
	}
	if avail, ok := values["MemAvailable"]; ok {
		used = total - avail
		if avail > total {
			used = 0
		}
		return used, total, nil
	}
	// old-kernel fallback: used = total - free - buffers - cached
	used = total - values["MemFree"] - values["Buffers"] - values["Cached"]
	if values["MemFree"] > total {
		used = 0
	}
	return used, total, nil
}

// ParseLoadAndCores parses the two-line LOADAVG segment: /proc/loadavg then nproc.
func ParseLoadAndCores(loadSeg string) (loads [3]float64, cores int, err error) {
	var lines []string
	for _, l := range strings.Split(loadSeg, "\n") {
		if t := strings.TrimSpace(l); t != "" {
			lines = append(lines, t)
		}
	}
	if len(lines) < 2 {
		return loads, 0, errors.New("loadavg segment requires 2 lines")
	}
	lf := strings.Fields(lines[0])
	if len(lf) < 3 {
		return loads, 0, errors.New("invalid loadavg line")
	}
	for i := 0; i < 3; i++ {
		v, err := strconv.ParseFloat(lf[i], 64)
		if err != nil {
			return loads, 0, fmt.Errorf("invalid load field %q", lf[i])
		}
		loads[i] = v
	}
	cores, err = strconv.Atoi(strings.TrimSpace(lines[1]))
	if err != nil {
		return loads, 0, fmt.Errorf("invalid nproc %q", lines[1])
	}
	return loads, cores, nil
}

// ParseDiskDF parses `df -P -B1` output into mount points (bytes).
func ParseDiskDF(raw string, rc int) ([]DiskMount, SegmentState, error) {
	var mounts []DiskMount
	invalid := false
	rows := 0
	for _, line := range strings.Split(raw, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if isSudoNoise(t) {
			continue
		}
		f := strings.Fields(t)
		if len(f) == 0 {
			continue
		}
		if f[0] == "Filesystem" {
			continue // header; skip regardless of column count
		}
		if len(f) < 5 {
			invalid = true
			continue
		}
		total, err1 := strconv.ParseUint(f[1], 10, 64)
		used, err2 := strconv.ParseUint(f[2], 10, 64)
		if err1 != nil || err2 != nil {
			// a real data row with undecodable columns => suspicious
			invalid = true
			continue
		}
		mountPoint := strings.Join(f[5:], " ") // tolerate mount points with spaces
		if mountPoint == "" && len(f) >= 6 {
			mountPoint = f[5]
		}
		pct := 0.0
		if total > 0 {
			pct = float64(used) / float64(total) * 100
		}
		mounts = append(mounts, DiskMount{
			MountPoint: mountPoint,
			TotalBytes: total,
			UsedBytes:  used,
			UsePercent: pct,
			TotalHuman: humanBytes(total),
			UsedHuman:  humanBytes(used),
		})
		rows++
	}
	if rc != 0 {
		return nil, SegFailed, nil
	}
	if invalid {
		return nil, SegFailed, nil
	}
	if rows == 0 {
		return nil, SegEmpty, nil
	}
	return mounts, SegOK, nil
}

type containerStats struct {
	Name          string
	CPUPercent    string
	MemUsage      string
	MemPercent    string
	MemUsedBytes  *uint64
	MemTotalBytes *uint64
}

func (c containerStats) cpuValue() float64 {
	v, err := strconv.ParseFloat(strings.TrimSuffix(c.CPUPercent, "%"), 64)
	if err != nil {
		return 0
	}
	return v
}

// ParseDockerStats parses `docker stats --no-stream --format '{{json .}}'`.
// psRunningCount is the running count from an OK docker_ps segment, or -1 if
// docker_ps could not confirm.
func ParseDockerStats(raw string, rc int, psRunningCount int) ([]containerStats, SegmentState, error) {
	if psRunningCount == 0 {
		// PS says no running containers: the stats noise/header is expected.
		return nil, SegEmpty, nil
	}
	if rc != 0 {
		return nil, SegFailed, nil
	}
	var stats []containerStats
	invalid := false
	for _, line := range strings.Split(raw, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if isSudoNoise(t) {
			continue
		}
		var rawStat struct {
			ID       string `json:"ID"`
			Name     string `json:"Name"`
			CPUPerc  string `json:"CPUPerc"`
			MemUsage string `json:"MemUsage"`
			MemPerc  string `json:"MemPerc"`
		}
		if err := json.Unmarshal([]byte(t), &rawStat); err != nil || rawStat.Name == "" && rawStat.ID == "" {
			invalid = true
			continue
		}
		used, total := parseMemUsage(rawStat.MemUsage)
		stats = append(stats, containerStats{
			Name:          rawStat.Name,
			CPUPercent:    rawStat.CPUPerc,
			MemUsage:      rawStat.MemUsage,
			MemPercent:    rawStat.MemPerc,
			MemUsedBytes:  used,
			MemTotalBytes: total,
		})
	}
	if invalid {
		return nil, SegFailed, nil
	}
	if len(stats) == 0 {
		// No containers resolved but PS could not confirm zero running.
		return nil, SegFailed, nil
	}
	return stats, SegOK, nil
}

type containerPS struct {
	ID        string
	Names     string
	Image     string
	Status    string
	State     string
	SizeRw    *int64
	SizeRwStr string
	Labels    map[string]string
}

func (c containerPS) rw() (int64, bool) {
	if c.SizeRw == nil || *c.SizeRw <= 0 {
		return 0, false
	}
	return *c.SizeRw, true
}

// ParseDockerPS parses `docker ps -a -s --format '{{json .}}'`. 0 rows + rc=0 is
// a legitimate zero-container state (SegOK), not an error. A missing SizeRw
// (older daemon) degrades to "-" rather than failing the segment.
func ParseDockerPS(raw string, rc int) ([]containerPS, SegmentState, error) {
	if rc != 0 {
		return nil, SegFailed, nil
	}
	var out []containerPS
	invalid := false
	for _, line := range strings.Split(raw, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if isSudoNoise(t) {
			continue
		}
		var c struct {
			ID     string `json:"ID"`
			Names  string `json:"Names"`
			Image  string `json:"Image"`
			Status string `json:"Status"`
			State  string `json:"State"`
			SizeRw *int64 `json:"SizeRw"`
			Size   string `json:"Size"`
			// docker ps emits Labels as a comma-separated "k=v,k=v" STRING, not
			// a JSON object; parse it into a map by hand.
			Labels string `json:"Labels"`
		}
		if err := json.Unmarshal([]byte(t), &c); err != nil {
			invalid = true
			continue
		}
		out = append(out, containerPS{
			ID:        c.ID,
			Names:     c.Names,
			Image:     c.Image,
			Status:    c.Status,
			State:     c.State,
			SizeRw:    c.SizeRw,
			SizeRwStr: rwSizePrefix(c.Size),
			Labels:    parseLabelsString(c.Labels),
		})
	}
	if invalid {
		return nil, SegFailed, nil
	}
	return out, SegOK, nil
}

type imageEntry struct {
	ID         string
	Repository string
	Tag        string
	Size       string
}

func (e imageEntry) repoTag() string {
	r, t := e.Repository, e.Tag
	if r == "<none>" && t == "<none>" {
		return "<none>:<none>"
	}
	if r == "" {
		return ""
	}
	if t == "" {
		t = "latest"
	}
	return r + ":" + t
}

// ParseImages parses `docker images --format '{{json .}}'`. 0 rows + rc=0 is OK.
func ParseImages(raw string, rc int) ([]imageEntry, SegmentState) {
	if rc != 0 {
		return nil, SegFailed
	}
	var out []imageEntry
	invalid := false
	for _, line := range strings.Split(raw, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if isSudoNoise(t) {
			continue
		}
		var img struct {
			ID         string `json:"ID"`
			Repository string `json:"Repository"`
			Tag        string `json:"Tag"`
			Size       string `json:"Size"`
		}
		if err := json.Unmarshal([]byte(t), &img); err != nil {
			invalid = true
			continue
		}
		out = append(out, imageEntry{ID: img.ID, Repository: img.Repository, Tag: img.Tag, Size: img.Size})
	}
	if invalid {
		return nil, SegFailed
	}
	return out, SegOK
}

// ParseDirSizes parses `du -sb <base>/yo-<env>-*/` output into service-name ->
// raw bytes. A fully-empty glob (rc=1 + all "cannot access") is a legitimate
// empty dirs state; rc=124 (timeout) is always a failure.
func ParseDirSizes(raw string, rc int, env string) (map[string]*uint64, SegmentState) {
	prefix := fmt.Sprintf("%s/%s", constants.RemoteBaseDir, fmt.Sprintf(constants.ServiceNameFormat, env, ""))
	sizes := make(map[string]*uint64)
	errorLines := 0
	dataLines := 0
	invalid := false

	for _, line := range strings.Split(raw, "\n") {
		t := strings.TrimSpace(line)
		if t == "" {
			continue
		}
		if isSudoNoise(t) {
			continue
		}
		if isCannotAccess(t) {
			errorLines++
			continue
		}
		parts := strings.SplitN(t, "\t", 2)
		if len(parts) == 2 {
			if v, err := strconv.ParseUint(strings.TrimSpace(parts[0]), 10, 64); err == nil {
				path := strings.TrimSuffix(strings.TrimSpace(parts[1]), "/")
				name := strings.TrimPrefix(path, prefix)
				if name != path && name != "" {
					val := v
					sizes[name] = &val
					dataLines++
					continue
				}
			}
		}
		invalid = true
	}

	if rc == 124 {
		return sizes, SegFailed
	}
	if invalid {
		return sizes, SegFailed
	}
	if dataLines > 0 && rc == 0 {
		return sizes, SegOK
	}
	if dataLines == 0 && errorLines > 0 && rc == 1 {
		return sizes, SegEmpty
	}
	if dataLines == 0 && errorLines == 0 && rc == 0 {
		return sizes, SegEmpty
	}
	return sizes, SegFailed
}

// ---------------------------------------------------------------------------
// BuildServiceReports
// ---------------------------------------------------------------------------

// BuildServiceReports maps collected raw pieces onto configured services for
// this server, classifying ownership by compose label (preferred) then name.
func (r *ServerResourceReport) BuildServiceReports(cfg *entity.Config, env string) {
	if cfg == nil {
		return
	}

	type svcDef struct {
		name  string
		image string
	}
	var defs []svcDef
	managed := make(map[string]bool)
	for i := range cfg.Services {
		s := &cfg.Services[i]
		if s.Server != r.ServerName {
			continue
		}
		defs = append(defs, svcDef{name: s.Name, image: s.Image})
		managed[s.Name] = true
	}
	for i := range cfg.InfraServices {
		s := &cfg.InfraServices[i]
		if s.Server != r.ServerName {
			continue
		}
		defs = append(defs, svcDef{name: s.Name, image: s.Image})
		managed[s.Name] = true
	}

	labelToName := make(map[string]string)
	for name := range managed {
		labelToName[fmt.Sprintf(constants.ServiceNameFormat, env, name)] = name
	}

	statsByName := make(map[string][]containerStats)
	for _, st := range r.stats {
		n := normalizeName(st.Name)
		statsByName[n] = append(statsByName[n], st)
	}
	psByName := make(map[string][]containerPS)
	for _, c := range r.containers {
		n := normalizeName(c.Names)
		psByName[n] = append(psByName[n], c)
	}

	// group container names by managed service
	groups := make(map[string][]string)
	unmanagedSeen := make(map[string]bool)
	for _, c := range r.containers {
		matchedName := r.matchManagedName(c, env, managed, labelToName)
		if matchedName != "" {
			groups[matchedName] = append(groups[matchedName], normalizeName(c.Names))
			continue
		}
		if c.State != "running" {
			continue
		}
		nm := normalizeName(c.Names)
		if unmanagedSeen[nm] {
			continue
		}
		unmanagedSeen[nm] = true
		r.Unmanaged = append(r.Unmanaged, UnmanagedContainer{
			Names:  c.Names,
			Image:  c.Image,
			Status: c.Status,
			SizeRw: rwHuman(c.SizeRw),
		})
	}
	sort.Slice(r.Unmanaged, func(i, j int) bool { return r.Unmanaged[i].Names < r.Unmanaged[j].Names })

	for _, def := range defs {
		info := ServiceResourceInfo{
			Name:       def.name,
			Status:     "not_deployed",
			CPUPercent: "-",
			MemUsage:   "-",
			MemPercent: "-",
			ImageSize:  "-",
			RwSize:     "-",
			DirSize:    "-",
		}
		if def.image == "" {
			// external_backends: no container/image/dir => all "-"
			info.Status = "-"
			r.Services = append(r.Services, info)
			continue
		}

		names := groups[def.name]
		if len(names) == 0 {
			info.Status = "not_deployed"
		} else {
			status, cpuSum, usedSum, totalSum, rwSum, rwStr, statsPresent, anyRunning := r.aggregateService(names, statsByName, psByName)
			info.Status = status
			if statsPresent {
				info.CPUPercent = formatPct(cpuSum)
				if totalSum > 0 {
					info.MemUsage = fmt.Sprintf("%s / %s", humanBytes(usedSum), humanBytes(totalSum))
					info.MemPercent = fmt.Sprintf("%.2f%%", float64(usedSum)/float64(totalSum)*100)
					u, t := usedSum, totalSum
					info.MemUsedBytes = &u
					info.MemTotalBytes = &t
				}
			}
			if anyRunning {
				if rwStr != "" {
					info.RwSize = rwStr
				} else if rwSum > 0 {
					info.RwSize = humanBytes(uint64(rwSum))
				} else if len(names) == 1 {
					if b, ok := psByName[names[0]]; ok && len(b) > 0 {
						info.RwSize = rwHumanStat(b[0])
					}
				}
			}
		}

		if sz, ok := matchImageSize(def.image, r.images); ok {
			info.ImageSize = sz
		}
		if b, ok := r.dirs[def.name]; ok && b != nil {
			info.DirBytes = b
			info.DirSize = humanBytes(*b)
		}
		r.applyFailedSegmentOverrides(&info)
		r.Services = append(r.Services, info)
	}
	sort.Slice(r.Services, func(i, j int) bool { return r.Services[i].Name < r.Services[j].Name })
}

func (r *ServerResourceReport) matchManagedName(c containerPS, env string, managed map[string]bool, labelToName map[string]string) string {
	if project, ok := c.Labels["com.docker.compose.project"]; ok {
		if name, ok := labelToName[strings.TrimPrefix(project, "/")]; ok {
			return name
		}
		// Has a compose project label but not a managed one: never fall back to
		// the name prefix for a project already labelled.
		return ""
	}
	nm := normalizeName(c.Names)
	prefix := fmt.Sprintf("yo-%s-", env)
	if strings.HasPrefix(nm, prefix) {
		remainder := strings.TrimPrefix(nm, prefix)
		if managed[remainder] {
			return remainder
		}
	}
	return ""
}

func (r *ServerResourceReport) aggregateService(names []string, statsByName map[string][]containerStats, psByName map[string][]containerPS) (status string, cpuSum float64, usedSum, totalSum uint64, rwSum int64, rwStr string, statsPresent bool, anyRunning bool) {
	anyExited := false
	for _, nm := range names {
		for _, st := range statsByName[nm] {
			statsPresent = true
			cpuSum += st.cpuValue()
			if st.MemUsedBytes != nil {
				usedSum += *st.MemUsedBytes
			}
			if st.MemTotalBytes != nil {
				totalSum += *st.MemTotalBytes
			}
		}
		for _, c := range psByName[nm] {
			if c.State == "running" {
				anyRunning = true
			} else if c.State == "exited" || c.State == "dead" {
				anyExited = true
			}
			if b, ok := c.rw(); ok {
				rwSum += b
			}
			if rwStr == "" && c.SizeRwStr != "" {
				rwStr = c.SizeRwStr
			}
		}
	}
	switch {
	case anyRunning:
		status = "running"
	case anyExited:
		status = "exited"
	case len(names) > 0:
		status = "not_deployed"
	default:
		status = "not_deployed"
	}
	return
}

// applyFailedSegmentOverrides marks per-service columns as n/a when the backing
// segment failed, so a collection failure is never rendered as a legitimate
// empty value ("-"). Services that carry no local resource (external_backends)
// always keep "-"; not_deployed services keep "-" too.
func (r *ServerResourceReport) applyFailedSegmentOverrides(info *ServiceResourceInfo) {
	if info.Status != "running" && info.Status != "exited" {
		return
	}
	if r.Segments["docker_stats"] == SegFailed {
		info.CPUPercent, info.MemUsage, info.MemPercent = "n/a", "n/a", "n/a"
	}
	if r.Segments["docker_ps"] == SegFailed {
		info.RwSize = "n/a"
	}
	if r.Segments["images"] == SegFailed {
		info.ImageSize = "n/a"
	}
	if r.Segments["dirs"] == SegFailed {
		info.DirSize = "n/a"
		info.DirBytes = nil // never 0; failed collection means "unknown"
	}
}

// normalizeImageRef decomposes a repo[:tag] reference and normalizes a missing
// tag to ":latest". A digest reference returns ok=false and is treated as a miss.
func normalizeImageRef(ref string) (repo, tag string, ok bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.Contains(ref, "@") {
		return "", "", false
	}
	lastColon := strings.LastIndex(ref, ":")
	lastSlash := strings.LastIndex(ref, "/")
	if lastColon > lastSlash {
		repo = ref[:lastColon]
		tag = ref[lastColon+1:]
	} else {
		repo = ref
		tag = "latest"
	}
	if repo == "" {
		return "", "", false
	}
	if tag == "" {
		tag = "latest"
	}
	if repo == "<none>" && tag == "<none>" {
		return "", "", false
	}
	return repo, tag, true
}

func matchImageSize(configImage string, images []imageEntry) (string, bool) {
	crepo, ctag, ok := normalizeImageRef(configImage)
	if !ok {
		return "", false
	}
	for _, img := range images {
		irepo, itag, ok := normalizeImageRef(img.repoTag())
		if !ok {
			continue
		}
		if strings.EqualFold(crepo, irepo) && strings.EqualFold(ctag, itag) {
			return img.Size, true
		}
	}
	return "", false
}

// ---------------------------------------------------------------------------
// small helpers
// ---------------------------------------------------------------------------

func normalizeName(s string) string {
	s = strings.TrimPrefix(strings.TrimSpace(s), "/")
	if idx := strings.Index(s, ","); idx >= 0 {
		s = s[:idx]
	}
	return s
}

func rwHuman(size *int64) string {
	if size == nil || *size <= 0 {
		return "-"
	}
	return humanBytes(uint64(*size))
}

// rwHumanStat returns the best available RW-size string for a container: the
// verbatim writable prefix from ps Size (e.g. "4.1kB"), else -.
func rwHumanStat(c containerPS) string {
	if c.SizeRwStr != "" {
		return c.SizeRwStr
	}
	if c.SizeRw != nil && *c.SizeRw > 0 {
		return humanBytes(uint64(*c.SizeRw))
	}
	return "-"
}

// rwSizePrefix extracts the writable-layer prefix from docker ps Size, e.g.
// "4.1kB (virtual 165MB)" -> "4.1kB".
func rwSizePrefix(size string) string {
	size = strings.TrimSpace(size)
	if size == "" {
		return ""
	}
	if idx := strings.Index(size, " (virtual"); idx >= 0 {
		return strings.TrimSpace(size[:idx])
	}
	if idx := strings.Index(size, " ("); idx >= 0 {
		return strings.TrimSpace(size[:idx])
	}
	return size
}

// parseLabelsString parses docker's comma-separated "k=v,k=v" label string.
func parseLabelsString(s string) map[string]string {
	m := make(map[string]string)
	for _, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 {
			continue
		}
		m[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
	}
	return m
}

func parseMemUsage(memUsage string) (*uint64, *uint64) {
	parts := strings.SplitN(strings.TrimSpace(memUsage), "/", 2)
	if len(parts) != 2 {
		return nil, nil
	}
	u, ok := parseSize(strings.TrimSpace(parts[0]))
	if !ok {
		return nil, nil
	}
	t, ok := parseSize(strings.TrimSpace(parts[1]))
	if !ok {
		return nil, nil
	}
	return u, t
}

func parseSize(s string) (*uint64, bool) {
	i := 0
	for i < len(s) && (s[i] >= '0' && s[i] <= '9' || s[i] == '.') {
		i++
	}
	if i == 0 {
		return nil, false
	}
	num, err := strconv.ParseFloat(s[:i], 64)
	if err != nil {
		return nil, false
	}
	unit := strings.ToLower(strings.TrimSpace(s[i:]))
	var mult float64
	switch unit {
	case "", "b":
		mult = 1
	case "k", "kb", "kib":
		mult = 1 << 10
	case "m", "mb", "mib":
		mult = 1 << 20
	case "g", "gb", "gib":
		mult = 1 << 30
	case "t", "tb", "tib":
		mult = 1 << 40
	case "p", "pb", "pib":
		mult = 1 << 50
	default:
		return nil, false
	}
	v := uint64(num * mult)
	return &v, true
}

func formatPct(v float64) string {
	return fmt.Sprintf("%.2f%%", v)
}

func isCannotAccess(line string) bool {
	l := strings.ToLower(line)
	return strings.Contains(l, "cannot access") || strings.Contains(l, "no such file") ||
		strings.Contains(l, "no such file or directory")
}

func isSudoNoise(line string) bool {
	l := strings.ToLower(strings.TrimSpace(line))
	if strings.HasPrefix(l, "sudo:") || strings.HasPrefix(l, "[sudo]") {
		return true
	}
	if strings.Contains(l, "unable to resolve host") && strings.Contains(l, "sudo") {
		return true
	}
	if strings.Contains(l, "password for") && strings.Contains(l, "sudo") {
		return true
	}
	return false
}

// humanBytes formats bytes with a 1024 base and one decimal place.
func humanBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

// FormatBytes exposes humanBytes for consumers (CLI/JSON display).
func FormatBytes(n uint64) string { return humanBytes(n) }

// collectStream runs a command via StreamRun (context-aware, execution-time
// cancel) and folds the stdout chunks into a single string. Since BuildInspectCommand
// merges stderr with `2>&1` on the remote side, the SSH stderr channel is simply
// drained to avoid any potential blocking.
func collectStream(ctx context.Context, client *ssh.Client, cmd string) (string, error) {
	stdoutCh := make(chan string, 128)
	stderrCh := make(chan string, 128)
	errCh := make(chan error, 1)
	go func() { errCh <- client.StreamRun(ctx, cmd, stdoutCh, stderrCh) }()

	var out strings.Builder
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case err := <-errCh:
			for chunk := range stdoutCh {
				out.WriteString(chunk)
			}
			if err != nil {
				return out.String(), err
			}
			return out.String(), nil
		case chunk, ok := <-stdoutCh:
			if ok {
				out.WriteString(chunk)
			}
		case <-stderrCh:
			// drained/ignored
		}
	}
}
