package docker

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/run"
)

func TestCollectDaemonUnreachableStopsImmediately(t *testing.T) {
	runner := newFakeRunner(map[string]run.Result{
		"version --format {{.Server.Version}}": {
			ExitCode: 1,
			Stderr:   "Cannot connect to the Docker daemon at npipe:////./pipe/dockerDesktopLinuxEngine. Is the docker daemon running?",
		},
	})
	info := Inspector{Runner: runner}.Collect(context.Background())

	if info.Status.Available {
		t.Fatal("docker should be unavailable")
	}
	if !strings.Contains(info.Status.Reason, "not reachable") {
		t.Errorf("reason = %q", info.Status.Reason)
	}
	if !strings.Contains(strings.ToLower(info.Status.Reason), "did not start") {
		t.Errorf("reason should state Docker was not started: %q", info.Status.Reason)
	}
	if len(runner.calls) != 1 {
		t.Errorf("expected a single probe, got %d calls: %v", len(runner.calls), runner.calls)
	}
}

func TestCollectMissingCLI(t *testing.T) {
	runner := newFakeRunner(map[string]run.Result{
		"version --format {{.Server.Version}}": {Missing: true},
	})
	info := Inspector{Runner: runner}.Collect(context.Background())
	if info.Status.Available || !strings.Contains(info.Status.Reason, "not found on PATH") {
		t.Fatalf("status = %+v", info.Status)
	}
}

func TestCollectTimeout(t *testing.T) {
	runner := newFakeRunner(map[string]run.Result{
		"version --format {{.Server.Version}}": {TimedOut: true, Timeout: 5 * time.Second},
	})
	info := Inspector{Runner: runner}.Collect(context.Background())
	if info.Status.Available || !strings.Contains(info.Status.Reason, "did not respond within") {
		t.Fatalf("status = %+v", info.Status)
	}
}

func TestCollectMalformedOutputIsSurvivable(t *testing.T) {
	runner := newFakeRunner(map[string]run.Result{
		"version --format {{.Server.Version}}":    {Stdout: "29.4.0"},
		"ps -a --no-trunc --format {{json .}}":    {Stdout: "not json\n{\"ID\":\"abc\",\"Names\":\"web\",\"State\":\"running\"}\n"},
		"system df --format {{json .}}":           {Stdout: "also not json\n"},
		"system df":                               {Stdout: "TYPE  TOTAL  ACTIVE  SIZE  RECLAIMABLE\n"},
		"image ls --no-trunc --format {{json .}}": {Stdout: "{\"Repository\":\"nginx\",\"Tag\":\"latest\",\"Size\":\"187MB\"}\n"},
		"stats --no-stream --format {{json .}}":   {ExitCode: 1, Stderr: "no such container"},
	})

	info := Inspector{Runner: runner}.Collect(context.Background())
	if !info.Status.Available {
		t.Fatal("docker should still be reported available")
	}
	if len(info.Containers) != 1 {
		t.Errorf("containers = %d, want 1 valid container", len(info.Containers))
	}
	if info.Containers[0].StatsKnown {
		t.Error("stats should be unknown when docker stats fails")
	}
	if len(info.Warnings) < 3 {
		t.Errorf("expected warnings for the failures, got %v", info.Warnings)
	}
}

func TestCollectSkipsStatsWhenNothingIsRunning(t *testing.T) {
	runner := newFakeRunner(map[string]run.Result{
		"version --format {{.Server.Version}}": {Stdout: "29.4.0"},
		"ps -a --no-trunc --format {{json .}}": {Stdout: `{"ID":"abc","Names":"db","State":"exited","Status":"Exited (0) 2 days ago"}` + "\n"},
	})
	Inspector{Runner: runner}.Collect(context.Background())

	for _, call := range runner.calls {
		if strings.Contains(call, "stats") {
			t.Fatalf("docker stats should not run with no running containers: %s", call)
		}
	}
}
