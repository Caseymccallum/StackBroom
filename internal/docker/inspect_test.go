package docker

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Caseymccallum/StackBroom/internal/run"
)

// fakeRunner records every Docker invocation and returns canned results.
type fakeRunner struct {
	results map[string]run.Result
	calls   []string
}

// localContextJSON is the shape `docker context inspect` returns for a local
// Docker Desktop context.
const localContextJSON = `[{"Name":"desktop-linux","Description":"Docker Desktop","Endpoints":{"docker":{"Host":"npipe:////./pipe/dockerDesktopLinuxEngine"}}}]`

func newFakeRunner(results map[string]run.Result) *fakeRunner {
	merged := map[string]run.Result{
		"context show":                  {Stdout: "desktop-linux\n"},
		"context inspect desktop-linux": {Stdout: localContextJSON},
	}
	for key, value := range results {
		merged[key] = value
	}
	return &fakeRunner{results: merged}
}

// envLookup returns a lookup function backed by a map, for tests.
func envLookup(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) run.Result {
	_ = ctx
	key := strings.Join(args, " ")
	f.calls = append(f.calls, name+" "+key)
	if r, ok := f.results[key]; ok {
		r.Command = name
		r.Args = args
		return r
	}
	return run.Result{
		Command:  name,
		Args:     args,
		ExitCode: 1,
		Err:      fmt.Errorf("unexpected command in test: %s", key),
	}
}

func happyPathRunner() *fakeRunner {
	return newFakeRunner(map[string]run.Result{
		"version --format {{.Server.Version}}": {Stdout: "29.4.0\n"},
		"context show":                         {Stdout: "desktop-linux\n"},
		"ps -a --no-trunc --format {{json .}}": {Stdout: `{"ID":"aaaabbbbccccdddd","Names":"web","Image":"nginx:latest","State":"running","Status":"Up 2 hours","Labels":"com.docker.compose.project=mysite"}
{"ID":"eeeeffff00001111","Names":"db","Image":"mysql:8.0","State":"exited","Status":"Exited (137) 5 months ago","Labels":""}
`},
		"stats --no-stream --format {{json .}}": {Stdout: `{"Container":"aaaabbbbcccc","Name":"web","CPUPerc":"0.50%","MemUsage":"12.5MiB / 1GiB","MemPerc":"1.22%"}
`},
		"system df --format {{json .}}": {Stdout: `{"Active":"5","Reclaimable":"12.41GB (79%)","Size":"15.64GB","TotalCount":"25","Type":"Images"}
{"Active":"0","Reclaimable":"30.09MB (100%)","Size":"30.09MB","TotalCount":"5","Type":"Containers"}
{"Active":"4","Reclaimable":"590.3MB (60%)","Size":"979MB","TotalCount":"18","Type":"Local Volumes"}
{"Active":"0","Reclaimable":"42.68MB","Size":"630.1MB","TotalCount":"21","Type":"Build Cache"}
`},
		"image ls --no-trunc --format {{json .}}": {Stdout: `{"ID":"sha256:1111","Repository":"nginx","Tag":"latest","Size":"187MB","CreatedSince":"2 weeks ago"}
{"ID":"sha256:2222","Repository":"mysql","Tag":"8.0","Size":"600MB","CreatedSince":"3 months ago"}
{"ID":"sha256:3333","Repository":"unused","Tag":"1.0","Size":"900MB","CreatedSince":"1 year ago"}
`},
	})
}

func TestCollectHappyPath(t *testing.T) {
	runner := happyPathRunner()
	info := Inspector{Runner: runner}.Collect(context.Background())

	if !info.Status.Available {
		t.Fatalf("docker should be available: %+v", info.Status)
	}
	if info.Status.ServerVersion != "29.4.0" || info.Status.Context != "desktop-linux" {
		t.Errorf("status = %+v", info.Status)
	}
	if info.RunningContainers() != 1 || info.StoppedContainers() != 1 {
		t.Errorf("container counts = %d running, %d stopped", info.RunningContainers(), info.StoppedContainers())
	}
	if !info.Containers[0].StatsKnown || info.Containers[0].MemoryUsage != "12.5MiB / 1GiB" {
		t.Errorf("running container stats were not merged: %+v", info.Containers[0])
	}
	if row, ok := info.DiskUsageOf("Images"); !ok || row.Reclaimable == nil {
		t.Errorf("images disk usage missing: %+v", row)
	}
	if len(info.Images) != 3 {
		t.Fatalf("images = %d, want 3", len(info.Images))
	}
	// Sorted largest first: unused 900MB, mysql 600MB, nginx 187MB.
	if info.Images[0].Repository != "unused" {
		t.Errorf("images not sorted by size: %+v", info.Images)
	}
	if info.Images[0].InUse {
		t.Errorf("unused image should not be marked in use: %+v", info.Images[0])
	}
	// Both nginx and mysql are referenced by containers (running and stopped),
	// so neither may be presented as unused.
	if !info.Images[1].InUse || !info.Images[2].InUse {
		t.Errorf("images used by containers were not marked in use: %+v", info.Images)
	}
}

func TestCollectIssuesOnlyReadOnlyCommands(t *testing.T) {
	runner := happyPathRunner()
	Inspector{Runner: runner}.Collect(context.Background())

	forbidden := []string{" prune", " rm ", " stop ", " kill ", " start ", " restart ", "volume rm", "image rm"}
	for _, call := range runner.calls {
		lower := " " + strings.ToLower(call) + " "
		for _, verb := range forbidden {
			if strings.Contains(lower, verb) {
				t.Fatalf("StackBroom issued a mutating Docker command: %s", call)
			}
		}
	}
	if len(runner.calls) == 0 {
		t.Fatal("expected Docker to be queried")
	}
}
