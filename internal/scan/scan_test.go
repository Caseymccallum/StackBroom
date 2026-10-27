package scan

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/model"
	"github.com/Caseymccallum/StackBroom/internal/winproc"
)

type fakeProcesses struct {
	result winproc.Result
	err    error
}

func (f fakeProcesses) Collect(ctx context.Context) (winproc.Result, error) {
	_ = ctx
	return f.result, f.err
}

type fakeDocker struct{ info model.DockerInfo }

func (f fakeDocker) Collect(ctx context.Context) model.DockerInfo {
	_ = ctx
	return f.info
}

type fakeWSL struct{ info model.WSLInfo }

func (f fakeWSL) Collect(ctx context.Context) model.WSLInfo {
	_ = ctx
	return f.info
}

type fakeResolver struct {
	refs map[string]*model.ProjectRef
}

func (f fakeResolver) Resolve(exePath, commandLine string) *model.ProjectRef {
	return f.refs[commandLine]
}

func TestRunBuildsSnapshot(t *testing.T) {
	fixed := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

	procs := []model.Process{
		{PID: 1, Name: "explorer.exe", Category: model.CategoryOther, Runtime: model.RuntimeUnknown},
		{PID: 10, ParentPID: 5, Name: "vmmem", Category: model.CategoryOther, MemoryKnown: true, WorkingSetBytes: 400 << 20},
		{
			PID: 20, ParentPID: 1, Name: "node.exe", Runtime: model.RuntimeNode,
			Category: model.CategoryRuntime, CommandLine: "node C:\\proj\\server.js",
			CommandLineKnown: true, MemoryKnown: true, WorkingSetBytes: 100 << 20,
		},
	}

	ref := &model.ProjectRef{
		Root: `C:\proj`, Name: "proj", Marker: "package.json",
		Confidence: model.Confirmed, Evidence: []string{"package.json found in C:\\proj"},
	}

	scanner := New(Options{
		Now: func() time.Time { return fixed },
		Processes: fakeProcesses{result: winproc.Result{
			Processes: procs,
			Source:    winproc.SourceCIM,
			Warnings:  []string{"example warning"},
		}},
		Docker: fakeDocker{info: model.DockerInfo{
			Status: model.DockerStatus{Available: true, ServerVersion: "29.4.0"},
			Containers: []model.DockerContainer{
				{Name: "old", State: "exited", Status: "Exited (0) 5 months ago", StoppedForDays: 150, StoppedForText: "5 months"},
			},
		}},
		WSL: fakeWSL{info: model.WSLInfo{
			Available:     true,
			Distributions: []model.WSLDistribution{{Name: "docker-desktop", State: "Running", Version: "2"}},
		}},
		Projects: fakeResolver{refs: map[string]*model.ProjectRef{
			"node C:\\proj\\server.js": ref,
		}},
	})

	snapshot := scanner.Run(context.Background())

	if !snapshot.ReadOnly {
		t.Error("snapshot must be marked read-only")
	}
	if snapshot.SchemaVersion != model.SchemaVersion {
		t.Errorf("schema version = %q", snapshot.SchemaVersion)
	}
	if snapshot.GeneratedAt != fixed || snapshot.DurationMS != 0 {
		t.Errorf("time = %s duration %d", snapshot.GeneratedAt, snapshot.DurationMS)
	}
	if len(snapshot.Processes) != 3 {
		t.Fatalf("processes = %d", len(snapshot.Processes))
	}

	if snapshot.Processes[2].ParentName != "explorer.exe" {
		t.Errorf("parent name = %q", snapshot.Processes[2].ParentName)
	}

	// A confirmed project promotes a plain runtime process to a dev tool, but
	// never to a dev server.
	got := snapshot.Processes[2]
	if got.Project == nil || got.Project.Name != "proj" {
		t.Fatalf("project association missing: %+v", got)
	}
	if got.Category != model.CategoryDevTool {
		t.Errorf("category = %q, want dev-tool", got.Category)
	}

	if !snapshot.WSL.VMMemoryKnown || snapshot.WSL.VMMemoryBytes != 400<<20 {
		t.Errorf("wsl vm memory = %d known=%v", snapshot.WSL.VMMemoryBytes, snapshot.WSL.VMMemoryKnown)
	}
	if snapshot.WSL.VMProcessName != "vmmem" || snapshot.WSL.VMNote == "" {
		t.Errorf("wsl vm metadata = %+v", snapshot.WSL)
	}

	if snapshot.Summary.DevProcessCount != 1 || snapshot.Summary.DevServerCount != 0 {
		t.Errorf("summary dev counts = %+v", snapshot.Summary)
	}
	if len(snapshot.Recommendations) == 0 {
		t.Error("expected the stale container to produce a recommendation")
	}
	if len(snapshot.Warnings) == 0 || !strings.Contains(snapshot.Warnings[0], "example warning") {
		t.Errorf("warnings = %v", snapshot.Warnings)
	}
}

func TestRunSurvivesProcessCollectorFailure(t *testing.T) {
	scanner := New(Options{
		Now:       func() time.Time { return time.Unix(0, 0) },
		Processes: fakeProcesses{err: errors.New("access denied")},
		Docker:    fakeDocker{},
		WSL:       fakeWSL{},
	})
	snapshot := scanner.Run(context.Background())

	if len(snapshot.Processes) != 0 {
		t.Errorf("processes = %+v", snapshot.Processes)
	}
	if !hasWarning(snapshot.Warnings, "Process inspection failed") {
		t.Errorf("warnings = %v", snapshot.Warnings)
	}
}

func hasWarning(values []string, substr string) bool {
	for _, v := range values {
		if strings.Contains(v, substr) {
			return true
		}
	}
	return false
}
