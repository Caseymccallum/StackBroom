package scan

import (
	"context"
	"testing"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/model"
	"github.com/Caseymccallum/StackBroom/internal/winproc"
)

func TestRunRespectsSkipFlags(t *testing.T) {
	scanner := New(Options{
		Network:    fakeNetwork{},
		Now:        func() time.Time { return time.Unix(0, 0) },
		SkipDocker: true,
		SkipWSL:    true,
		Processes:  fakeProcesses{},
		Docker:     fakeDocker{info: model.DockerInfo{Status: model.DockerStatus{Available: true}}},
		WSL:        fakeWSL{info: model.WSLInfo{Available: true}},
	})
	snapshot := scanner.Run(context.Background())

	if snapshot.Docker.Status.Available {
		t.Error("docker should not have been collected")
	}
	if snapshot.WSL.Available {
		t.Error("wsl should not have been collected")
	}
	if !hasWarning(snapshot.Warnings, "Docker inspection was skipped") ||
		!hasWarning(snapshot.Warnings, "WSL inspection was skipped") {
		t.Errorf("warnings = %v", snapshot.Warnings)
	}
}

func TestRunDoesNotPromoteInferredProjects(t *testing.T) {
	scanner := New(Options{
		Network: fakeNetwork{},
		Now:     func() time.Time { return time.Unix(0, 0) },
		Processes: fakeProcesses{result: winproc.Result{Processes: []model.Process{{
			PID: 5, Name: "node.exe", Runtime: model.RuntimeNode, Category: model.CategoryRuntime,
			CommandLine: "node x", CommandLineKnown: true,
		}}}},
		Docker: fakeDocker{},
		WSL:    fakeWSL{},
		Projects: fakeResolver{refs: map[string]*model.ProjectRef{
			"node x": {Root: `C:\x`, Name: "x", Confidence: model.Inferred},
		}},
	})
	snapshot := scanner.Run(context.Background())
	if snapshot.Processes[0].Category != model.CategoryRuntime {
		t.Fatalf("inferred project changed the category to %q", snapshot.Processes[0].Category)
	}
}

func TestRunPropagatesCollectorWarnings(t *testing.T) {
	scanner := New(Options{
		Network: fakeNetwork{},
		Now:     func() time.Time { return time.Unix(0, 0) },
		Processes: fakeProcesses{result: winproc.Result{
			Warnings: []string{"Command lines could not be read."},
		}},
		Docker: fakeDocker{info: model.DockerInfo{
			Status:   model.DockerStatus{Available: false, Reason: "Docker CLI not found on PATH"},
			Warnings: []string{"Container list unavailable: timed out"},
		}},
		WSL: fakeWSL{info: model.WSLInfo{Available: false, Reason: "wsl.exe was not found"}},
	})
	snapshot := scanner.Run(context.Background())

	if !hasWarning(snapshot.Warnings, "Command lines could not be read") {
		t.Errorf("process warning lost: %v", snapshot.Warnings)
	}
	if !hasWarning(snapshot.Warnings, "Docker CLI not found") {
		t.Errorf("docker reason lost: %v", snapshot.Warnings)
	}
	if !hasWarning(snapshot.Warnings, "Container list unavailable") {
		t.Errorf("docker warning lost: %v", snapshot.Warnings)
	}
	if !hasWarning(snapshot.Warnings, "wsl.exe was not found") {
		t.Errorf("wsl reason lost: %v", snapshot.Warnings)
	}
}
