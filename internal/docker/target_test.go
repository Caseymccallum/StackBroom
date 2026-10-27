package docker

import (
	"context"
	"strings"
	"testing"

	"github.com/Caseymccallum/StackBroom/internal/run"
)

func TestClassifyEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		local    bool
		reason   string
	}{
		{"docker desktop pipe", "npipe:////./pipe/dockerDesktopLinuxEngine", true, "local named pipe"},
		{"classic pipe", "npipe:////./pipe/docker_engine", true, "local named pipe"},
		{"unix socket", "unix:///var/run/docker.sock", true, "local Unix socket"},
		{"file descriptor", "fd://3", true, "local file descriptor"},
		{"remote tcp", "tcp://192.0.2.10:2375", false, "network destination"},
		{"loopback tcp", "tcp://127.0.0.1:2375", false, "loopback"},
		{"localhost tcp", "tcp://localhost:2375", false, "localhost"},
		{"ssh host", "ssh://builder@example.com", false, "network destination"},
		{"https", "https://docker.example.com:2376", false, "network destination"},
		{"unknown scheme", "docker-container://builder", false, "network destination"},
		{"bare path", "C:\\docker\\npipe", false, "recognised transport scheme"},
		{"empty", "   ", false, "no Docker endpoint"},
		{"missing pipe path", "npipe://", false, "no path"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyEndpoint(tc.endpoint)
			if got.Local != tc.local {
				t.Errorf("Local = %v, want %v (%+v)", got.Local, tc.local, got)
			}
			if !strings.Contains(strings.ToLower(got.Reason), strings.ToLower(tc.reason)) {
				t.Errorf("Reason = %q, want it to contain %q", got.Reason, tc.reason)
			}
		})
	}
}

func TestDisplayEndpointRedactsCredentials(t *testing.T) {
	got := displayEndpoint("tcp://user:supersecret@example.com:2375")
	if strings.Contains(got, "supersecret") {
		t.Fatalf("credentials leaked in display endpoint: %q", got)
	}
	if !strings.Contains(got, "example.com") {
		t.Fatalf("host was removed: %q", got)
	}
}

func TestResolveTargetPrefersDockerContextOverHost(t *testing.T) {
	runner := newFakeRunner(map[string]run.Result{
		"context inspect prod": {Stdout: `[{"Name":"prod","Endpoints":{"docker":{"Host":"npipe:////./pipe/prod"}}}]`},
	})
	target := ResolveTarget(context.Background(), runner, "docker", envLookup(map[string]string{
		"DOCKER_CONTEXT": "prod",
		"DOCKER_HOST":    "tcp://192.0.2.10:2375",
	}))

	if !target.Local || target.Context != "prod" || target.Source != SourceContextEnv {
		t.Fatalf("target = %+v", target)
	}
	for _, call := range runner.calls {
		if strings.Contains(call, "inspect desktop-linux") {
			t.Errorf("DOCKER_HOST/active context was inspected despite DOCKER_CONTEXT: %s", call)
		}
	}
}

func TestResolveTargetUsesDockerHostWhenContextEnvUnset(t *testing.T) {
	runner := newFakeRunner(nil)
	target := ResolveTarget(context.Background(), runner, "docker", envLookup(map[string]string{
		"DOCKER_HOST": "tcp://192.0.2.10:2375",
	}))

	if target.Local || target.Source != SourceHostEnv {
		t.Fatalf("target = %+v", target)
	}
	if target.Display != "tcp://192.0.2.10:2375" {
		t.Errorf("display = %q", target.Display)
	}
	if target.Context != "desktop-linux" {
		t.Errorf("context should still be reported: %q", target.Context)
	}
}

func TestResolveTargetRejectsUnusableContextName(t *testing.T) {
	runner := newFakeRunner(nil)
	target := ResolveTarget(context.Background(), runner, "docker", envLookup(map[string]string{
		"DOCKER_CONTEXT": "--help",
	}))
	if target.Source != SourceContextEnv || target.Local {
		t.Fatalf("target = %+v", target)
	}
	if len(runner.calls) != 0 {
		t.Errorf("invalid context name should not be passed to Docker: %v", runner.calls)
	}
}

func TestResolveTargetInspectFailureIsUnknown(t *testing.T) {
	runner := newFakeRunner(map[string]run.Result{
		"context inspect missing": {ExitCode: 1, Stderr: "context \"missing\" does not exist"},
	})
	target := ResolveTarget(context.Background(), runner, "docker", envLookup(map[string]string{
		"DOCKER_CONTEXT": "missing",
	}))
	if target.Local || target.Source != SourceUnknown {
		t.Fatalf("target = %+v", target)
	}
	if !strings.Contains(target.Reason, "could not be inspected") {
		t.Errorf("reason = %q", target.Reason)
	}
}

func TestCollectRefusesRemoteEndpointByDefault(t *testing.T) {
	runner := newFakeRunner(nil)
	info := Inspector{
		Runner:    runner,
		LookupEnv: envLookup(map[string]string{"DOCKER_HOST": "tcp://192.0.2.10:2375"}),
	}.Collect(context.Background())

	if info.Status.Available {
		t.Fatal("remote endpoint must not be contacted by default")
	}
	if info.Status.EndpointLocal {
		t.Error("remote endpoint reported as local")
	}
	if !strings.Contains(info.Status.Reason, "--allow-remote-docker") {
		t.Errorf("reason should explain the opt-in: %q", info.Status.Reason)
	}
	if !strings.Contains(info.Status.Reason, "did not contact") {
		t.Errorf("reason should state no connection was made: %q", info.Status.Reason)
	}
	for _, call := range runner.calls {
		if strings.Contains(call, "version") || strings.Contains(call, " ps ") {
			t.Errorf("daemon was contacted: %s", call)
		}
	}
}

func TestCollectRemoteEndpointWithApproval(t *testing.T) {
	runner := newFakeRunner(map[string]run.Result{
		"version --format {{.Server.Version}}": {Stdout: "29.4.0\n"},
	})
	info := Inspector{
		Runner:      runner,
		AllowRemote: true,
		LookupEnv:   envLookup(map[string]string{"DOCKER_HOST": "tcp://192.0.2.10:2375"}),
	}.Collect(context.Background())

	if !info.Status.Available || !info.Status.RemoteApproved {
		t.Fatalf("status = %+v", info.Status)
	}
	if info.Status.EndpointLocal {
		t.Error("remote endpoint reported as local")
	}
	if len(info.Warnings) == 0 || !strings.Contains(info.Warnings[0], "read-only") {
		t.Errorf("warnings = %v", info.Warnings)
	}
}

func TestCollectLocalNamedPipeIsAllowed(t *testing.T) {
	runner := newFakeRunner(map[string]run.Result{
		"version --format {{.Server.Version}}": {Stdout: "29.4.0\n"},
	})
	info := Inspector{Runner: runner}.Collect(context.Background())

	if !info.Status.Available || !info.Status.EndpointLocal {
		t.Fatalf("status = %+v", info.Status)
	}
	if info.Status.RemoteApproved {
		t.Error("local endpoint should not be marked remote")
	}
	if !strings.Contains(info.Status.Endpoint, "npipe") {
		t.Errorf("endpoint = %q", info.Status.Endpoint)
	}
}
