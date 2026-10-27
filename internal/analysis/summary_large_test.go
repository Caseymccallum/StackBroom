package analysis

import (
	"sort"
	"testing"

	"github.com/Caseymccallum/StackBroom/internal/model"
)

// TestSummarizeLargeInventory checks that aggregation stays correct and bounded
// with a realistic worst case (tens of thousands of processes).
func TestSummarizeLargeInventory(t *testing.T) {
	const total = 20000
	procs := make([]model.Process, total)

	wantDev := 0
	wantServers := 0
	var wantDevMemory uint64
	known := make([]uint64, 0, total)

	for i := range procs {
		pid := int32(i + 1)
		category := model.CategoryOther
		runtime := model.RuntimeUnknown
		switch {
		case i%10 == 0:
			category = model.CategoryDevServer
			runtime = model.RuntimeNode
			wantServers++
			wantDev++
		case i%7 == 0:
			category = model.CategoryDevTool
			runtime = model.RuntimePython
			wantDev++
		}
		memoryKnown := i%3 != 0
		memory := uint64(i) * 1024
		procs[i] = model.Process{
			PID:             pid,
			Name:            "proc.exe",
			Category:        category,
			Runtime:         runtime,
			WorkingSetBytes: memory,
			MemoryKnown:     memoryKnown,
		}
		if category == model.CategoryDevServer || category == model.CategoryDevTool {
			if memoryKnown {
				wantDevMemory += memory
			}
		}
		if memoryKnown {
			known = append(known, memory)
		}
	}

	got := Summarize(procs, model.DockerInfo{}, model.WSLInfo{}, 8)

	if got.TotalProcesses != total {
		t.Errorf("total = %d", got.TotalProcesses)
	}
	if got.DevProcessCount != wantDev || got.DevServerCount != wantServers {
		t.Errorf("dev counts = %d/%d, want %d/%d",
			got.DevProcessCount, got.DevServerCount, wantDev, wantServers)
	}
	if got.DevMemoryBytes != wantDevMemory {
		t.Errorf("dev memory = %d, want %d", got.DevMemoryBytes, wantDevMemory)
	}
	if len(got.TopProcesses) != 8 {
		t.Fatalf("top processes = %d, want 8", len(got.TopProcesses))
	}

	sort.Slice(known, func(i, j int) bool { return known[i] > known[j] })
	var wantTop uint64
	for i := 0; i < 8; i++ {
		wantTop += known[i]
	}
	if got.TopMemoryBytes != wantTop {
		t.Errorf("top memory = %d, want %d", got.TopMemoryBytes, wantTop)
	}
	for i := 1; i < len(got.TopProcesses); i++ {
		if got.TopProcesses[i-1].MemoryBytes < got.TopProcesses[i].MemoryBytes {
			t.Fatalf("top processes are not sorted: %+v", got.TopProcesses)
		}
	}
}

func BenchmarkSummarizeLargeInventory(b *testing.B) {
	const total = 20000
	procs := make([]model.Process, total)
	for i := range procs {
		procs[i] = model.Process{
			PID:             int32(i + 1),
			Name:            "proc.exe",
			Category:        model.CategoryOther,
			Runtime:         model.RuntimeUnknown,
			WorkingSetBytes: uint64(i) * 1024,
			MemoryKnown:     true,
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Summarize(procs, model.DockerInfo{}, model.WSLInfo{}, 8)
	}
}
