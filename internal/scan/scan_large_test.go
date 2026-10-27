package scan

import (
	"context"
	"testing"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/model"
	"github.com/Caseymccallum/StackBroom/internal/winproc"
)

// TestRunHandlesLargeProcessInventory exercises orchestration with thousands of
// processes, including unresolved projects and unknown memory.
func TestRunHandlesLargeProcessInventory(t *testing.T) {
	const total = 5000
	procs := make([]model.Process, total)
	for i := range procs {
		category := model.CategoryOther
		if i%5 == 0 {
			category = model.CategoryDevServer
		}
		procs[i] = model.Process{
			PID:             int32(i + 1),
			Name:            "proc.exe",
			Category:        category,
			Runtime:         model.RuntimeNode,
			MemoryKnown:     i%2 == 0,
			WorkingSetBytes: uint64(i) * 512,
			RequestedPorts:  []int{3000 + i%100},
		}
	}

	scanner := New(Options{
		Network:   fakeNetwork{},
		Now:       func() time.Time { return time.Unix(0, 0) },
		Processes: fakeProcesses{result: winproc.Result{Processes: procs}},
		Docker:    fakeDocker{},
		WSL:       fakeWSL{},
	})
	snapshot := scanner.Run(context.Background())

	if len(snapshot.Processes) != total {
		t.Fatalf("processes = %d", len(snapshot.Processes))
	}
	if snapshot.Summary.TotalProcesses != total {
		t.Errorf("summary total = %d", snapshot.Summary.TotalProcesses)
	}
	if snapshot.Summary.DevServerCount != total/5 {
		t.Errorf("dev servers = %d, want %d", snapshot.Summary.DevServerCount, total/5)
	}
	if len(snapshot.Summary.TopProcesses) != 8 {
		t.Errorf("top processes = %d", len(snapshot.Summary.TopProcesses))
	}
	// Every dev server requests one of 100 ports, so duplicate suspects must be
	// produced without flooding the output.
	if len(snapshot.Duplicates) == 0 {
		t.Error("expected duplicate suspects for repeated ports")
	}
	if len(snapshot.Duplicates) > 100 {
		t.Errorf("duplicate groups = %d, want at most one per port", len(snapshot.Duplicates))
	}
}
