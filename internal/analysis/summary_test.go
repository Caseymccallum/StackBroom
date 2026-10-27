package analysis

import (
	"testing"

	"github.com/Caseymccallum/StackBroom/internal/model"
)

func devProc(pid int32, rt model.Runtime, category string, memory uint64, known bool) model.Process {
	return model.Process{
		PID:             pid,
		Runtime:         rt,
		Category:        category,
		WorkingSetBytes: memory,
		MemoryKnown:     known,
	}
}

func TestSummarize(t *testing.T) {
	procs := []model.Process{
		devProc(1, model.RuntimeNode, model.CategoryDevServer, 100, true),
		devProc(2, model.RuntimeNode, model.CategoryDevTool, 200, true),
		devProc(3, model.RuntimePython, model.CategoryDevTool, 0, false),
		devProc(4, model.RuntimeUnknown, model.CategoryOther, 500, true),
	}

	size := func(n uint64) *uint64 { return &n }
	pct := 79.0
	dockerInfo := model.DockerInfo{
		Status: model.DockerStatus{Available: true},
		DiskUsage: []model.DockerDiskUsage{
			{Type: "Images", Size: size(1000), Reclaimable: size(500), ReclaimablePercent: &pct},
			{Type: "Build Cache", Size: size(500)},
			{Type: "Local Volumes"},
		},
	}
	wslInfo := model.WSLInfo{
		Available:     true,
		Distributions: []model.WSLDistribution{{Name: "Ubuntu", State: "Running", Version: "2"}},
		VMMemoryBytes: 473 << 20,
		VMMemoryKnown: true,
	}

	got := Summarize(procs, dockerInfo, wslInfo, 2)

	if got.TotalProcesses != 4 {
		t.Errorf("total processes = %d", got.TotalProcesses)
	}
	if got.DevProcessCount != 3 || got.DevServerCount != 1 {
		t.Errorf("dev counts = %d/%d, want 3/1", got.DevProcessCount, got.DevServerCount)
	}
	if got.DevMemoryBytes != 300 || got.DevMemoryKnown != 2 {
		t.Errorf("dev memory = %d known %d, want 300/2", got.DevMemoryBytes, got.DevMemoryKnown)
	}
	if len(got.TopProcesses) != 2 {
		t.Fatalf("top processes = %d, want 2", len(got.TopProcesses))
	}
	if got.TopProcesses[0].PID != 4 || got.TopProcesses[1].PID != 2 {
		t.Errorf("top processes = %+v", got.TopProcesses)
	}
	if got.TopMemoryBytes != 700 {
		t.Errorf("top memory = %d, want 700", got.TopMemoryBytes)
	}
	if got.DockerTotalBytes != 1500 {
		t.Errorf("docker total = %d", got.DockerTotalBytes)
	}
	if got.DockerTotalKnown {
		t.Error("docker total should be marked incomplete when a row has no size")
	}
	if got.DockerSizeRowsKnown != 2 || got.DockerSizeRowsTotal != 3 {
		t.Errorf("docker row counts = %d/%d", got.DockerSizeRowsKnown, got.DockerSizeRowsTotal)
	}
	if got.DockerReclaimKnown {
		t.Error("reclaimable total should be incomplete when a row has no size")
	}
	if got.DockerReclaimRowsKnown != 1 || got.DockerReclaimRowsTotal != 3 {
		t.Errorf("reclaim row counts = %d/%d", got.DockerReclaimRowsKnown, got.DockerReclaimRowsTotal)
	}
	if got.DockerReclaimableBytes != 500 {
		t.Errorf("docker reclaimable = %d", got.DockerReclaimableBytes)
	}
	if got.WSLRunningDistributions != 1 || !got.WSLVMMemoryKnown || got.WSLVMMemoryBytes != 473<<20 {
		t.Errorf("wsl summary = %+v", got)
	}
	if len(got.RuntimeTotals) != 2 {
		t.Fatalf("runtime totals = %+v", got.RuntimeTotals)
	}
	if got.RuntimeTotals[0].Runtime != model.RuntimeNode || got.RuntimeTotals[0].ProcessCount != 2 {
		t.Errorf("runtime totals order = %+v", got.RuntimeTotals)
	}
}

func TestSummarizeEmpty(t *testing.T) {
	got := Summarize(nil, model.DockerInfo{}, model.WSLInfo{}, 0)
	if got.TotalProcesses != 0 || got.DevProcessCount != 0 {
		t.Errorf("empty summary = %+v", got)
	}
	if got.DockerTotalKnown || got.DockerReclaimKnown {
		t.Error("empty docker report should not claim known totals")
	}
	if got.TopProcesses == nil || got.RuntimeTotals == nil {
		t.Error("empty slices should be non-nil for stable JSON")
	}
}
