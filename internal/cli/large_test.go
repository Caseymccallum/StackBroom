package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/Caseymccallum/StackBroom/internal/analysis"
	"github.com/Caseymccallum/StackBroom/internal/model"
)

func TestScanTextHandlesLargeProcessTable(t *testing.T) {
	snapshot := fixtureSnapshot()
	const total = 2000
	procs := make([]model.Process, total)
	for i := range procs {
		procs[i] = model.Process{
			PID:             int32(i + 1),
			Name:            "node.exe",
			Runtime:         model.RuntimeNode,
			Category:        model.CategoryDevTool,
			WorkingSetBytes: uint64(i) * 1024,
			MemoryKnown:     true,
		}
	}
	snapshot.Processes = procs
	snapshot.Summary = analysis.Summarize(procs, snapshot.Docker, snapshot.WSL, 8)

	app, out, _ := newTestApp(snapshot)
	if code := app.Run(context.Background(), []string{"scan"}); code != ExitOK {
		t.Fatalf("exit code = %d", code)
	}
	text := out.String()
	if !strings.Contains(text, "DEVELOPMENT PROCESSES (2000)") {
		t.Errorf("large table was not rendered as expected:\n%s", text[:min(len(text), 500)])
	}
	if !strings.Contains(text, "TOP MEMORY CONSUMERS") {
		t.Error("top memory section missing from large render")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
