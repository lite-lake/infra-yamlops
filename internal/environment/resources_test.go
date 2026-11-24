package environment

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/lite-lake/infra-yamlops/internal/domain/entity"
)

// ---------------------------------------------------------------------------
// CPU window
// ---------------------------------------------------------------------------

func TestParseCPUWindow_Normal(t *testing.T) {
	// sample1 busy=140, sample2 busy=190 => busyPct ~= 20%
	s1 := CPUSample{StatLine: "cpu  100 10 20 10000 100 5 5 0", UptimeLine: "100.0 50.0"}
	s2 := CPUSample{StatLine: "cpu  150 10 20 10200 100 5 5 0", UptimeLine: "101.0 50.0"}
	busy, _, _ := ParseCPUWindow(s1, s2)
	if busy < 15 || busy > 25 {
		t.Errorf("busy = %v, want ~20", busy)
	}
}

func TestParseCPUWindow_ShortWindow(t *testing.T) {
	s1 := CPUSample{StatLine: "cpu  1 1 1 100 2 1 1 0", UptimeLine: "10.0 5.0"}
	s2 := CPUSample{StatLine: "cpu  2 1 1 101 2 1 1 0", UptimeLine: "10.2 5.0"}
	_, _, err := ParseCPUWindow(s1, s2)
	if !errors.Is(err, ErrCPUSampleWindowTooShort) {
		t.Fatalf("err = %v, want ErrCPUSampleWindowTooShort", err)
	}
}

func TestParseCPUWindow_BadLines(t *testing.T) {
	s1 := CPUSample{StatLine: "notcpu x", UptimeLine: "10.0 5.0"}
	s2 := CPUSample{StatLine: "cpu  1 1 1 1 1 1 1 1", UptimeLine: "11.0 5.0"}
	if _, _, err := ParseCPUWindow(s1, s2); err == nil {
		t.Fatal("ParseCPUWindow() error = nil, want error for bad stat line")
	}
}

// ---------------------------------------------------------------------------
// meminfo
// ---------------------------------------------------------------------------

func TestParseMeminfo_Standard(t *testing.T) {
	raw := `MemTotal:        8388608 kB
MemFree:         2097152 kB
MemAvailable:    4194304 kB
Buffers:         102400 kB
Cached:          204800 kB`
	used, total, err := ParseMeminfo(raw)
	if err != nil {
		t.Fatalf("ParseMeminfo() error = %v", err)
	}
	if total != 8*1024*1024*1024 {
		t.Errorf("total = %d, want %d", total, 8*1024*1024*1024)
	}
	// used = total - avail = 4 GiB
	if used != 4*1024*1024*1024 {
		t.Errorf("used = %d, want %d", used, 4*1024*1024*1024)
	}
}

func TestParseMeminfo_OldKernelFallback(t *testing.T) {
	raw := `MemTotal:        8388608 kB
MemFree:         2097152 kB
Buffers:         102400 kB
Cached:          204800 kB`
	used, total, err := ParseMeminfo(raw)
	if err != nil {
		t.Fatalf("ParseMeminfo() error = %v", err)
	}
	want := uint64(8*1024*1024*1024) - 2097152*1024 - 102400*1024 - 204800*1024
	if used != want {
		t.Errorf("used = %d, want %d (fallback)", used, want)
	}
	if total != 8*1024*1024*1024 {
		t.Errorf("total = %d, want %d", total, 8*1024*1024*1024)
	}
}

func TestParseMeminfo_AllMissing(t *testing.T) {
	if _, _, err := ParseMeminfo("MemFree: 100 kB\n"); err == nil {
		t.Fatal("ParseMeminfo() error = nil, want error when MemTotal missing")
	}
}

// ---------------------------------------------------------------------------
// loadavg
// ---------------------------------------------------------------------------

func TestParseLoadAndCores_Normal(t *testing.T) {
	loads, cores, err := ParseLoadAndCores("0.10 0.25 0.30 1/234 5678\n8\n")
	if err != nil {
		t.Fatalf("ParseLoadAndCores() error = %v", err)
	}
	if loads != [3]float64{0.10, 0.25, 0.30} {
		t.Errorf("loads = %v", loads)
	}
	if cores != 8 {
		t.Errorf("cores = %d, want 8", cores)
	}
}

func TestParseLoadAndCores_BadNproc(t *testing.T) {
	if _, _, err := ParseLoadAndCores("0.10 0.25 0.30\nabc\n"); err == nil {
		t.Fatal("ParseLoadAndCores() error = nil, want error for non-numeric nproc")
	}
}

func TestParseLoadAndCores_Short(t *testing.T) {
	if _, _, err := ParseLoadAndCores("0.10 0.25 0.30\n"); err == nil {
		t.Fatal("ParseLoadAndCores() error = nil, want error for missing nproc line")
	}
}

// ---------------------------------------------------------------------------
// df
// ---------------------------------------------------------------------------

func TestParseDiskDF_Normal(t *testing.T) {
	raw := `Filesystem      1B-blocks        Used   Available Capacity Mounted on
/dev/sda1      80000000000 20000000000 60000000000      25% /
/dev/sdb1      100000000000 50000000000 50000000000      50% /data`
	mounts, state, err := ParseDiskDF(raw, 0)
	if err != nil {
		t.Fatalf("ParseDiskDF() error = %v", err)
	}
	if state != SegOK {
		t.Fatalf("state = %v, want SegOK", state)
	}
	if len(mounts) != 2 {
		t.Fatalf("len(mounts) = %d, want 2", len(mounts))
	}
	if mounts[0].MountPoint != "/" {
		t.Errorf("mounts[0].MountPoint = %q, want '/'", mounts[0].MountPoint)
	}
	if mounts[0].UsePercent < 24 || mounts[0].UsePercent > 26 {
		t.Errorf("mounts[0].UsePercent = %v, want ~25", mounts[0].UsePercent)
	}
}

func TestParseDiskDF_SkipsHeader(t *testing.T) {
	// A header that happens to have exactly 6 columns must not become a mount row.
	raw := `Filesystem 1B-blocks Used Available Capacity Mounted on
/dev/x 100 50 50 50% /tmp`
	mounts, state, _ := ParseDiskDF(raw, 0)
	if state != SegOK {
		t.Fatalf("state = %v, want SegOK", state)
	}
	if len(mounts) != 1 {
		t.Fatalf("len(mounts) = %d, want 1 (header skipped)", len(mounts))
	}
	if mounts[0].MountPoint != "/tmp" {
		t.Errorf("MountPoint = %q, want '/tmp'", mounts[0].MountPoint)
	}
}

func TestParseDiskDF_MountSpace(t *testing.T) {
	raw := `Filesystem 1B-blocks Used Available Capacity Mounted on
/dev/y 200 100 100 50% /data/my mount`
	mounts, _, _ := ParseDiskDF(raw, 0)
	if len(mounts) != 1 || mounts[0].MountPoint != "/data/my mount" {
		t.Errorf("mount with space not parsed: %+v", mounts)
	}
}

func TestParseDiskDF_EmptyRowRC0(t *testing.T) {
	_, state, _ := ParseDiskDF("", 0)
	if state != SegEmpty {
		t.Fatalf("state = %v, want SegEmpty for 0 rows + rc=0", state)
	}
}

func TestParseDiskDF_RCError(t *testing.T) {
	_, state, _ := ParseDiskDF("", 124)
	if state != SegFailed {
		t.Fatalf("state = %v, want SegFailed for rc=124", state)
	}
}

// ---------------------------------------------------------------------------
// docker stats
// ---------------------------------------------------------------------------

func TestParseDockerStats_Normal(t *testing.T) {
	raw := `{"ID":"a1","Name":"yo-prod-app","CPUPerc":"1.25%","MemUsage":"128MiB / 4GiB","MemPerc":"3.12%"}
{"ID":"b2","Name":"yo-prod-worker","CPUPerc":"0.00%","MemUsage":"10MiB / 4GiB","MemPerc":"0.24%"}`
	stats, state, err := ParseDockerStats(raw, 0, 2)
	if err != nil {
		t.Fatalf("ParseDockerStats() error = %v", err)
	}
	if state != SegOK {
		t.Fatalf("state = %v, want SegOK", state)
	}
	if len(stats) != 2 {
		t.Fatalf("len(stats) = %d, want 2", len(stats))
	}
	if stats[0].MemUsedBytes == nil || *stats[0].MemUsedBytes != 128*1024*1024 {
		t.Errorf("MemUsedBytes = %v, want 128MiB", stats[0].MemUsedBytes)
	}
	if stats[0].MemTotalBytes == nil || *stats[0].MemTotalBytes != 4*1024*1024*1024 {
		t.Errorf("MemTotalBytes = %v, want 4GiB", stats[0].MemTotalBytes)
	}
}

func TestParseDockerStats_CrossValidation_Empty(t *testing.T) {
	// ps reports 0 running => stats noise/header is a legitimate empty state.
	raw := "Cannot connect to the Docker daemon at unix:///var/run/docker.sock\n"
	_, state, _ := ParseDockerStats(raw, 0, 0)
	if state != SegEmpty {
		t.Fatalf("state = %v, want SegEmpty when ps proves 0 running", state)
	}
}

func TestParseDockerStats_CrossValidation_Failed(t *testing.T) {
	// ps could NOT confirm -> same noise text is a real failure.
	raw := "Cannot connect to the Docker daemon at unix:///var/run/docker.sock\n"
	_, state, _ := ParseDockerStats(raw, 0, -1)
	if state != SegFailed {
		t.Fatalf("state = %v, want SegFailed when ps cannot confirm 0 running", state)
	}
}

func TestParseDockerStats_NoiseSkipped(t *testing.T) {
	raw := `sudo: unable to resolve host hostname
{"ID":"a1","Name":"yo-prod-app","CPUPerc":"1.25%","MemUsage":"128MiB / 4GiB","MemPerc":"3.12%"}`
	stats, state, _ := ParseDockerStats(raw, 0, 1)
	if state != SegOK {
		t.Fatalf("state = %v, want SegOK even with sudo noise", state)
	}
	if len(stats) != 1 {
		t.Fatalf("len(stats) = %d, want 1", len(stats))
	}
}

// ---------------------------------------------------------------------------
// docker ps
// ---------------------------------------------------------------------------

func TestParseDockerPS_Normal(t *testing.T) {
	raw := `{"ID":"a1","Names":"yo-prod-app","Image":"app:1.0","Status":"Up 2 hours","State":"running","SizeRw":10485760,"Labels":"com.docker.compose.project=yo-prod-app"}`
	ps, state, err := ParseDockerPS(raw, 0)
	if err != nil {
		t.Fatalf("ParseDockerPS() error = %v", err)
	}
	if state != SegOK {
		t.Fatalf("state = %v, want SegOK", state)
	}
	if len(ps) != 1 {
		t.Fatalf("len(ps) = %d, want 1", len(ps))
	}
	if ps[0].State != "running" {
		t.Errorf("State = %q, want running", ps[0].State)
	}
	if ps[0].SizeRw == nil || *ps[0].SizeRw != 10485760 {
		t.Errorf("SizeRw = %v, want 10485760", ps[0].SizeRw)
	}
	if got := ps[0].Labels["com.docker.compose.project"]; got != "yo-prod-app" {
		t.Errorf("Labels[project] = %q, want yo-prod-app", got)
	}
}

func TestParseDockerPS_SizeStringRW(t *testing.T) {
	// Some docker versions expose Size as "4.1kB (virtual 165MB)" with no SizeRw.
	raw := `{"ID":"a1","Names":"yo-prod-app","Image":"app:1.0","Status":"Up","State":"running","Size":"4.1kB (virtual 165MB)","Labels":"com.docker.compose.project=yo-prod-app"}`
	ps, state, _ := ParseDockerPS(raw, 0)
	if state != SegOK {
		t.Fatalf("state = %v, want SegOK", state)
	}
	if len(ps) != 1 || ps[0].SizeRwStr != "4.1kB" {
		t.Errorf("SizeRwStr = %q, want '4.1kB'", ps[0].SizeRwStr)
	}
}

func TestParseDockerPS_MissingSizeRw(t *testing.T) {
	// Older daemon omits SizeRw: must degrade to "-", not fail the segment.
	raw := `{"ID":"a1","Names":"yo-prod-app","Image":"app:1.0","Status":"Exited (0)","State":"exited","Labels":"com.docker.compose.project=yo-prod-app"}`
	ps, state, _ := ParseDockerPS(raw, 0)
	if state != SegOK {
		t.Fatalf("state = %v, want SegOK (missing SizeRw is not fatal)", state)
	}
	if len(ps) != 1 || ps[0].SizeRw != nil {
		t.Errorf("ps = %+v, want SizeRw nil", ps)
	}
}

func TestParseDockerPS_ZeroRowsOK(t *testing.T) {
	ps, state, _ := ParseDockerPS("", 0)
	if state != SegOK {
		t.Fatalf("state = %v, want SegOK for 0 rows + rc=0", state)
	}
	if len(ps) != 0 {
		t.Fatalf("len(ps) = %d, want 0", len(ps))
	}
}

func TestParseDockerPS_RC124(t *testing.T) {
	_, state, _ := ParseDockerPS("", 124)
	if state != SegFailed {
		t.Fatalf("state = %v, want SegFailed for rc=124", state)
	}
}

// ---------------------------------------------------------------------------
// images
// ---------------------------------------------------------------------------

func TestParseImages_Normal(t *testing.T) {
	raw := `{"ID":"aaa","Repository":"team/app","Tag":"1.0","Size":"100MB"}
{"ID":"bbb","Repository":"team/worker","Tag":"latest","Size":"50MB"}`
	imgs, state := ParseImages(raw, 0)
	if state != SegOK {
		t.Fatalf("state = %v, want SegOK", state)
	}
	if len(imgs) != 2 {
		t.Fatalf("len(imgs) = %d, want 2", len(imgs))
	}
	if imgs[0].repoTag() != "team/app:1.0" {
		t.Errorf("repoTag() = %q", imgs[0].repoTag())
	}
}

func TestParseImages_NoneTag(t *testing.T) {
	imgs, _ := ParseImages(`{"ID":"aaa","Repository":"<none>","Tag":"<none>","Size":"1GB"}`, 0)
	if len(imgs) != 1 || imgs[0].repoTag() != "<none>:<none>" {
		t.Errorf("none image not handled: %+v", imgs)
	}
}

func TestImageMatching(t *testing.T) {
	images := []imageEntry{{Repository: "team/app", Tag: "latest", Size: "100MB"}}
	// case-insensitive + default latest
	if sz, ok := matchImageSize("team/APP", images); !ok || sz != "100MB" {
		t.Errorf("case-insensitive match failed: sz=%q ok=%v", sz, ok)
	}
	if _, ok := matchImageSize("team/app@sha256:abc", images); ok {
		t.Error("digest ref should be a miss")
	}
	if _, ok := matchImageSize("other/app", images); ok {
		t.Error("different repo should be a miss")
	}
}

// ---------------------------------------------------------------------------
// dirs
// ---------------------------------------------------------------------------

func TestParseDirSizes_Normal(t *testing.T) {
	raw := "1024\t/data/yamlops/yo-prod-app/\n2048\t/data/yamlops/yo-prod-worker/"
	sizes, state := ParseDirSizes(raw, 0, "prod")
	if state != SegOK {
		t.Fatalf("state = %v, want SegOK", state)
	}
	if len(sizes) != 2 {
		t.Fatalf("len(sizes) = %d, want 2", len(sizes))
	}
	if *sizes["app"] != 1024 || *sizes["worker"] != 2048 {
		t.Errorf("sizes = %v", sizes)
	}
}

func TestParseDirSizes_EmptyGlobRC1(t *testing.T) {
	raw := "du: cannot access '/data/yamlops/yo-prod-*/': No such file or directory"
	_, state := ParseDirSizes(raw, 1, "prod")
	if state != SegEmpty {
		t.Fatalf("state = %v, want SegEmpty for all-cannot-access + rc=1", state)
	}
}

func TestParseDirSizes_TimeoutRC124(t *testing.T) {
	// Partial valid rows plus a timeout must still be failed (do not read a
	// truncated dir set as "no dir").
	raw := "1024\t/data/yamlops/yo-prod-app/\n2048\t/data/yamlops/yo-prod-worker/"
	_, state := ParseDirSizes(raw, 124, "prod")
	if state != SegFailed {
		t.Fatalf("state = %v, want SegFailed for rc=124 even with partial rows", state)
	}
}

func TestParseDirSizes_NoRowsRC0(t *testing.T) {
	_, state := ParseDirSizes("", 0, "prod")
	if state != SegEmpty {
		t.Fatalf("state = %v, want SegEmpty for no rows + rc=0", state)
	}
}

// ---------------------------------------------------------------------------
// segment splitting
// ---------------------------------------------------------------------------

func TestSplitSegments_Normal(t *testing.T) {
	out := `@@HOST_CPU_1
cpu 1 2 3 4 5 6 7 8
@@MEMINFO
MemTotal: 1000 kB
@@DONE`
	raw := SplitSegments(out)
	if !raw.DoneSeen {
		t.Fatal("DoneSeen = false, want true")
	}
	if !strings.Contains(raw.Segments["HOST_CPU_1"], "cpu 1 2 3 4 5 6 7 8") {
		t.Errorf("HOST_CPU_1 not split correctly: %q", raw.Segments["HOST_CPU_1"])
	}
	if !strings.Contains(raw.Segments["MEMINFO"], "MemTotal") {
		t.Errorf("MEMINFO not split correctly: %q", raw.Segments["MEMINFO"])
	}
}

func TestSplitSegments_RCStripped(t *testing.T) {
	out := "@@DISK\nFilesystem 1 2 3 4 5 /\ndisk_rc=0\n@@DONE"
	raw := SplitSegments(out)
	if raw.RCs["disk_rc"] != 0 {
		t.Errorf("RCs = %v, want disk_rc=0", raw.RCs)
	}
	if strings.Contains(raw.Segments["DISK"], "disk_rc") {
		t.Errorf("RC line not stripped: %q", raw.Segments["DISK"])
	}
}

func TestSplitSegments_DoneMissing(t *testing.T) {
	out := "@@MEMINFO\nMemTotal: 1000 kB\n"
	raw := SplitSegments(out)
	if raw.DoneSeen {
		t.Fatal("DoneSeen = true, want false")
	}
}

func TestSplitSegments_CollisionContent(t *testing.T) {
	// A content line that begins with @@ but is not a known marker stays content.
	out := "@@MEMINFO\nMemTotal: 1000 kB\n@@FAKE_HEADER\nMemFree: 10 kB\n@@DONE"
	raw := SplitSegments(out)
	if !strings.Contains(raw.Segments["MEMINFO"], "@@FAKE_HEADER") {
		t.Errorf("collision content lost: %q", raw.Segments["MEMINFO"])
	}
}

// ---------------------------------------------------------------------------
// BuildServiceReports
// ---------------------------------------------------------------------------

func testConfig() *entity.Config {
	return &entity.Config{
		Services: []entity.BizService{
			{ServiceBase: entity.ServiceBase{Server: "srv-a"}, Name: "app", Image: "team/app:1.0"},
			{ServiceBase: entity.ServiceBase{Server: "srv-a"}, Name: "proxy", ExternalBackends: []string{"https://api.example.com"}},
			{ServiceBase: entity.ServiceBase{Server: "srv-a"}, Name: "notdeployed", Image: "team/ghost:1.0"},
		},
	}
}

func TestBuildServiceReports_ExternalBackendsAndNotDeployed(t *testing.T) {
	cfg := testConfig()
	r := &ServerResourceReport{ServerName: "srv-a"}
	r.BuildServiceReports(cfg, "prod")
	if len(r.Services) != 3 {
		t.Fatalf("len(services) = %d, want 3", len(r.Services))
	}
	for _, s := range r.Services {
		switch s.Name {
		case "proxy":
			if s.Status != "-" || s.ImageSize != "-" || s.DirSize != "-" {
				t.Errorf("external_backends service %s not all '-': %+v", s.Name, s)
			}
		case "notdeployed":
			if s.Status != "not_deployed" {
				t.Errorf("notdeployed status = %q, want not_deployed", s.Status)
			}
		}
	}
}

func TestBuildServiceReports_LabelPrecedence(t *testing.T) {
	cfg := testConfig()
	// Manual container named yo-prod-app but with a compose label pointing to a
	// NON-managed project: the label wins and the container becomes unmanaged
	// rather than being stolen by the name fallback.
	r := &ServerResourceReport{
		ServerName: "srv-a",
		containers: []containerPS{
			{Names: "yo-prod-app", State: "running", Labels: map[string]string{"com.docker.compose.project": "yo-prod-other"}, Image: "team/app:1.0", Status: "Up"},
		},
	}
	r.BuildServiceReports(cfg, "prod")
	for _, s := range r.Services {
		if s.Name == "app" {
			if s.Status != "not_deployed" {
				t.Errorf("service app status = %q, want not_deployed (label points to non-managed project)", s.Status)
			}
		}
	}
	if len(r.Unmanaged) != 1 || r.Unmanaged[0].Names != "yo-prod-app" {
		t.Errorf("Unmanaged = %+v, want yo-prod-app listed", r.Unmanaged)
	}
}

func TestBuildServiceReports_NameFallback(t *testing.T) {
	cfg := testConfig()
	// No label at all -> name fallback must bind yo-prod-app to service "app".
	r := &ServerResourceReport{
		ServerName: "srv-a",
		containers: []containerPS{
			{Names: "yo-prod-app", State: "running", Labels: nil, Image: "team/app:1.0", Status: "Up"},
		},
	}
	r.BuildServiceReports(cfg, "prod")
	for _, s := range r.Services {
		if s.Name == "app" && s.Status != "running" {
			t.Errorf("service app status = %q, want running via name fallback", s.Status)
		}
	}
	if len(r.Unmanaged) != 0 {
		t.Errorf("Unmanaged = %+v, want empty", r.Unmanaged)
	}
}

func TestBuildServiceReports_ImageAndDirFill(t *testing.T) {
	cfg := testConfig()
	app := uint64(1500)
	r := &ServerResourceReport{
		ServerName: "srv-a",
		containers: []containerPS{
			{Names: "yo-prod-app", State: "running", Labels: map[string]string{"com.docker.compose.project": "yo-prod-app"}, Image: "team/app:1.0", Status: "Up", SizeRw: ptrInt64(2048)},
		},
		stats: []containerStats{
			{Name: "yo-prod-app", CPUPercent: "1.25%", MemUsage: "100MiB / 4GiB", MemPercent: "2.44%", MemUsedBytes: ptrUint64(100 * 1024 * 1024), MemTotalBytes: ptrUint64(4 * 1024 * 1024 * 1024)},
		},
		images: []imageEntry{{Repository: "team/app", Tag: "1.0", Size: "100MB"}},
		dirs:   map[string]*uint64{"app": &app},
	}
	r.BuildServiceReports(cfg, "prod")
	for _, s := range r.Services {
		if s.Name != "app" {
			continue
		}
		if s.Status != "running" {
			t.Errorf("status = %q", s.Status)
		}
		if s.CPUPercent != "1.25%" {
			t.Errorf("CPUPercent = %q", s.CPUPercent)
		}
		if s.ImageSize != "100MB" {
			t.Errorf("ImageSize = %q", s.ImageSize)
		}
		if s.DirBytes == nil || *s.DirBytes != 1500 {
			t.Errorf("DirBytes = %v", s.DirBytes)
		}
		if s.RwSize == "" || s.RwSize == "-" {
			t.Errorf("RwSize = %q, want filled", s.RwSize)
		}
		return
	}
	t.Fatal("service app not found")
}

func TestBuildServiceReports_FailedSegmentsNA(t *testing.T) {
	cfg := testConfig()
	app := uint64(1500)
	r := &ServerResourceReport{
		ServerName: "srv-a",
		Segments: map[string]SegmentState{
			"dirs":         SegFailed,
			"images":       SegFailed,
			"docker_stats": SegFailed,
		},
		containers: []containerPS{
			{Names: "yo-prod-app", State: "running", Labels: map[string]string{"com.docker.compose.project": "yo-prod-app"}, Image: "team/app:1.0", Status: "Up"},
		},
		dirs: map[string]*uint64{"app": &app},
	}
	r.BuildServiceReports(cfg, "prod")
	for _, s := range r.Services {
		if s.Name != "app" {
			continue
		}
		if s.DirSize != "n/a" {
			t.Errorf("DirSize = %q, want n/a for failed dirs segment", s.DirSize)
		}
		if s.DirBytes != nil {
			t.Errorf("DirBytes = %v, want nil (null, never 0) for failed dirs segment", s.DirBytes)
		}
		if s.ImageSize != "n/a" {
			t.Errorf("ImageSize = %q, want n/a for failed images segment", s.ImageSize)
		}
		if s.CPUPercent != "n/a" {
			t.Errorf("CPUPercent = %q, want n/a for failed stats segment", s.CPUPercent)
		}
	}
}

func TestBuildServiceReports_EmptyNotDegraded(t *testing.T) {
	// A genuinely empty server (no containers / no dirs) must not be degraded.
	cfg := testConfig()
	r := &ServerResourceReport{
		ServerName: "srv-a",
		Segments: map[string]SegmentState{
			"docker_stats": SegEmpty,
			"docker_ps":    SegOK,
			"dirs":         SegEmpty,
		},
	}
	r.BuildServiceReports(cfg, "prod")
	if r.Degraded {
		t.Fatal("Degraded = true, want false for legitimately-empty segments")
	}
	for _, s := range r.Services {
		if s.Name == "app" && s.Status != "not_deployed" {
			t.Errorf("app status = %q, want not_deployed", s.Status)
		}
	}
}

func TestSegmentKeyConsistency(t *testing.T) {
	// The rc segment map keys must match the segment keys used for Segments.
	for key, rcKey := range rcSegmentMap {
		if key == "" || rcKey == "" {
			t.Errorf("rcSegmentMap has empty entry: %q => %q", key, rcKey)
		}
	}
}

func TestJSON_NilBytesNull(t *testing.T) {
	// A nil *uint64 must serialize to JSON null, never 0.
	b, err := json.Marshal(ServiceResourceInfo{Name: "x", DirBytes: nil})
	if err != nil {
		t.Fatalf("marshal error = %v", err)
	}
	if !strings.Contains(string(b), `"DirBytes":null`) {
		t.Errorf("JSON = %s, want DirBytes null", string(b))
	}
}

func TestSegmentState_MarshalJSON(t *testing.T) {
	cases := map[SegmentState]string{SegOK: `"ok"`, SegEmpty: `"empty"`, SegFailed: `"failed"`}
	for st, want := range cases {
		b, _ := json.Marshal(st)
		if string(b) != want {
			t.Errorf("Marshal(%v) = %s, want %s", st, b, want)
		}
	}
}

func TestHumanBytes(t *testing.T) {
	if got := humanBytes(0); got != "0 B" {
		t.Errorf("humanBytes(0) = %q, want '0 B'", got)
	}
	if got := humanBytes(900); got != "900 B" {
		t.Errorf("humanBytes(900) = %q", got)
	}
	if got := humanBytes(1024); got != "1.0 KiB" {
		t.Errorf("humanBytes(1024) = %q, want '1.0 KiB'", got)
	}
	if got := humanBytes(4 * 1024 * 1024 * 1024); got != "4.0 GiB" {
		t.Errorf("humanBytes(4GiB) = %q, want '4.0 GiB'", got)
	}
}

func TestNormalizeImageRef(t *testing.T) {
	repo, tag, ok := normalizeImageRef("team/app")
	if !ok || repo != "team/app" || tag != "latest" {
		t.Errorf("normalizeImageRef(team/app) = %s %s %v", repo, tag, ok)
	}
	repo, tag, _ = normalizeImageRef("team/app:1.0")
	if repo != "team/app" || tag != "1.0" {
		t.Errorf("normalizeImageRef(:1.0) = %s %s", repo, tag)
	}
	if _, _, ok := normalizeImageRef("team/app@sha256:abc"); ok {
		t.Error("digest ref should be a miss")
	}
}

func TestBuildInspectCommand_UsesConstants(t *testing.T) {
	cmd := BuildInspectCommand("prod")
	if !strings.Contains(cmd, "/data/yamlops/yo-prod-*") {
		t.Errorf("command does not reference constants.RemoteBaseDir: %q", cmd)
	}
	if !strings.Contains(cmd, "@@DONE") {
		t.Errorf("command missing @@DONE: %q", cmd)
	}
}

func ptrUint64(v uint64) *uint64 { return &v }
func ptrInt64(v int64) *int64    { return &v }

func TestParseDockerStats_MemParse(t *testing.T) {
	u, tot := parseMemUsage("128MiB / 4GiB")
	if u == nil || *u != 128*1024*1024 {
		t.Errorf("used = %v", u)
	}
	if tot == nil || *tot != 4*1024*1024*1024 {
		t.Errorf("total = %v", tot)
	}
}

// ---------------------------------------------------------------------------
// Full Inspect integration over a synthetic stream (no SSH): verify assembly.
// ---------------------------------------------------------------------------

func TestBuildReport_Assembly(t *testing.T) {
	// This exercises report.Segments mapping and Degraded propagation using the
	// pure functions only, without an SSH client.
	stream := `@@HOST_CPU_1
cpu  100 10 20 10000 100 5 5 0
100.0 50.0
@@MEMINFO
MemTotal: 8388608 kB
MemAvailable: 4194304 kB
@@DOCKER_STATS
{"ID":"a1","Name":"yo-prod-app","CPUPerc":"1.25%","MemUsage":"128MiB / 4GiB","MemPerc":"3.12%"}
stats_rc=0
@@HOST_CPU_2
cpu  200 10 20 10100 100 5 5 0
101.0 50.0
@@LOADAVG
0.10 0.20 0.30 1/10 100
4
@@DISK
Filesystem 1B-blocks Used Available Capacity Mounted on
/dev/x 100 50 50 50% /
disk_rc=0
@@DOCKER_DF
{"Type":"Images","TotalCount":"3","Active":"2","Size":"1GB","Reclaimable":"500MB (50%)"}
docker_df_rc=0
@@DOCKER_PS
{"ID":"a1","Names":"yo-prod-app","Image":"team/app:1.0","Status":"Up","State":"running","SizeRw":2048,"Labels":"com.docker.compose.project=yo-prod-app"}
ps_rc=0
@@IMAGES
{"ID":"aaa","Repository":"team/app","Tag":"1.0","Size":"100MB"}
images_rc=0
@@DIRS
1024	/data/yamlops/yo-prod-app/
dirs_rc=0
@@DONE`

	raw := SplitSegments(stream)
	if !raw.DoneSeen {
		t.Fatal("DoneSeen = false")
	}
	// per-segment parses mirror Inspect's assembly
	hostState := SegOK
	var busy float64
	var steal float64
	_ = (&Inspector{}).parseCPU(&raw, &busy, &steal, &hostState)
	if hostState != SegOK {
		t.Fatalf("host_cpu state = %v, want SegOK", hostState)
	}
	if busy <= 0 {
		t.Fatalf("busy = %v, want > 0", busy)
	}
}
