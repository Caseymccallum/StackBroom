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
		valid    bool
		reason   string
	}{
		// Canonical local channels.
		{"docker desktop pipe", "npipe:////./pipe/dockerDesktopLinuxEngine", true, true, "local named pipe"},
		{"classic pipe", "npipe:////./pipe/docker_engine", true, true, "local named pipe"},
		{"pipe case insensitive", "NPIPE:////./PIPE/docker_engine", true, true, "local named pipe"},
		{"unix socket", "unix:///var/run/docker.sock", true, true, "local Unix socket"},
		{"file descriptor", "fd://3", true, true, "local file descriptor"},

		// Well-formed network destinations: valid, never local.
		{"remote tcp", "tcp://192.0.2.10:2375", false, true, "network destination"},
		{"loopback tcp", "tcp://127.0.0.1:2375", false, true, "loopback"},
		{"localhost tcp", "tcp://localhost:2375", false, true, "localhost"},
		{"ipv6 loopback tcp", "tcp://[::1]:2375", false, true, "loopback"},
		{"ssh host", "ssh://builder@example.com", false, true, "network destination"},

		// Unsupported or malformed endpoints: invalid and never contacted.
		{"host-qualified pipe", "npipe:////fileserver/pipe/docker_engine", false, false, "not supported"},
		{"localhost pipe", "npipe:////localhost/pipe/docker_engine", false, false, "not supported"},
		{"https unsupported", "https://docker.example.com:2376", false, false, "not supported"},
		{"http unsupported", "http://docker.example.com:2375", false, false, "not supported"},
		{"unknown scheme", "docker-container://builder", false, false, "unsupported transport"},
		{"bare path", "C:\\docker\\npipe", false, false, "recognised transport scheme"},
		{"empty", "   ", false, false, "no Docker endpoint"},
		{"pipe no path", "npipe://", false, false, "no path"},
		{"pipe three slashes", "npipe:///./pipe/docker_engine", false, false, "form"},
		{"pipe two slashes", "npipe://server/pipe/docker_engine", false, false, "form"},
		{"pipe empty name", "npipe:////./pipe/", false, false, "name"},
		{"pipe nested name", "npipe:////./pipe/a/b", false, false, "name"},
		{"pipe parent traversal", "npipe:////./pipe/../x", false, false, "name"},
		{"pipe encoded name", "npipe:////./pipe/docker%5Fengine", false, false, "unsupported characters"},
		{"pipe query", "npipe:////./pipe/engine?x=1", false, false, "unsupported characters"},
		{"pipe fragment", "npipe:////./pipe/engine#x", false, false, "unsupported characters"},
		{"pipe space", "npipe:////./pipe/docker engine", false, false, "name"},
		{"remote pipe empty name", "npipe:////server/pipe/", false, false, "name"},
		{"tcp no host", "tcp://", false, false, "no host"},
		{"tcp credentials", "tcp://user:pass@example.com:2375", false, false, "credentials"},
		{"tcp query", "tcp://example.com:2375?x=1", false, false, "query"},
		{"unix relative", "unix://relative.sock", false, false, "not absolute"},
		{"unix empty", "unix://", false, false, "no path"},
		{"fd empty", "fd://", false, false, "no descriptor"},
		{"fd non numeric", "fd://abc", false, false, "not numeric"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ClassifyEndpoint(tc.endpoint)
			if got.Local != tc.local {
				t.Errorf("Local = %v, want %v (%+v)", got.Local, tc.local, got)
			}
			if got.Valid != tc.valid {
				t.Errorf("Valid = %v, want %v (%+v)", got.Valid, tc.valid, got)
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

func TestResolveTargetDefaultContextReflectsDockerHost(t *testing.T) {
	// Docker's "default" context derives its endpoint from DOCKER_HOST, so the
	// CLI reports the network endpoint even with DOCKER_CONTEXT=default.
	runner := newFakeRunner(map[string]run.Result{
		"context inspect default": {Stdout: `[{"Name":"default","Endpoints":{"docker":{"Host":"tcp://192.0.2.10:2375"}}}]`},
	})
	target := ResolveTarget(context.Background(), runner, "docker", envLookup(map[string]string{
		"DOCKER_CONTEXT": "default",
		"DOCKER_HOST":    "tcp://192.0.2.10:2375",
	}))

	if target.Local {
		t.Fatalf("default context endpoint should be classified as network: %+v", target)
	}
	if target.Display != "tcp://192.0.2.10:2375" {
		t.Errorf("display = %q", target.Display)
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

func TestInspectContextSelectsOnlyDockerEndpoint(t *testing.T) {
	// Only the "docker" key addresses the daemon. Map iteration order and the
	// presence of an unrelated local endpoint must never change the result.
	remoteDocker := `[{"Name":"mixed","Endpoints":{
		"local":{"Host":"npipe:////./pipe/docker_engine"},
		"docker":{"Host":"tcp://192.0.2.10:2375"},
		"ssh":{"Host":"ssh://builder@example.com"}}}]`
	localDocker := `[{"Name":"mixed","Endpoints":{
		"ssh":{"Host":"ssh://builder@example.com"},
		"other":{"Host":"tcp://192.0.2.10:2375"},
		"docker":{"Host":"npipe:////./pipe/docker_engine"}}}]`

	for i := 0; i < 50; i++ {
		runner := newFakeRunner(map[string]run.Result{
			"context inspect mixed": {Stdout: remoteDocker},
		})
		target := ResolveTarget(context.Background(), runner, "docker",
			envLookup(map[string]string{"DOCKER_CONTEXT": "mixed"}))
		if !target.Valid || target.Local {
			t.Fatalf("iteration %d: target = %+v, want valid remote endpoint", i, target)
		}
		if target.Display != "tcp://192.0.2.10:2375" {
			t.Fatalf("iteration %d: selected %q", i, target.Display)
		}
	}

	runner := newFakeRunner(map[string]run.Result{
		"context inspect mixed": {Stdout: localDocker},
	})
	target := ResolveTarget(context.Background(), runner, "docker",
		envLookup(map[string]string{"DOCKER_CONTEXT": "mixed"}))
	if !target.Valid || !target.Local {
		t.Fatalf("target = %+v, want local docker endpoint", target)
	}
	if !strings.Contains(target.Display, "./pipe/docker_engine") {
		t.Errorf("selected %q, want the local docker endpoint", target.Display)
	}
}

func TestInspectContextFailsClosedWithoutDockerEndpoint(t *testing.T) {
	tests := []struct {
		name string
		out  string
	}{
		{
			name: "missing docker key",
			out:  `[{"Name":"x","Endpoints":{"local":{"Host":"npipe:////./pipe/docker_engine"}}}]`,
		},
		{
			name: "capitalised key is not docker",
			out:  `[{"Name":"x","Endpoints":{"Docker":{"Host":"npipe:////./pipe/docker_engine"}}}]`,
		},
		{
			name: "blank host",
			out:  `[{"Name":"x","Endpoints":{"docker":{"Host":"   "}}}]`,
		},
		{
			name: "no endpoints",
			out:  `[{"Name":"x"}]`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runner := newFakeRunner(map[string]run.Result{
				"context inspect x": {Stdout: tc.out},
			})
			target := ResolveTarget(context.Background(), runner, "docker",
				envLookup(map[string]string{"DOCKER_CONTEXT": "x"}))
			if target.Valid || target.Local {
				t.Fatalf("target = %+v, want an unresolved endpoint", target)
			}
			if !strings.Contains(strings.ToLower(target.Reason), "docker") {
				t.Errorf("reason = %q, want it to mention the docker endpoint", target.Reason)
			}
		})
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

func TestRemoteApprovalDoesNotRescueInvalidEndpoint(t *testing.T) {
	endpoints := []string{
		"tcp://",
		"tcp://user:pass@example.com:2375",
		"https://example.com:2376",
		"npipe://",
		"npipe://server/pipe/docker_engine",
		"npipe:////fileserver/pipe/docker_engine",
		"unix://relative.sock",
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			runner := newFakeRunner(nil)
			info := Inspector{
				Runner:      runner,
				AllowRemote: true,
				LookupEnv:   envLookup(map[string]string{"DOCKER_HOST": endpoint}),
			}.Collect(context.Background())

			if info.Status.Available {
				t.Fatalf("invalid endpoint was contacted: %+v", info.Status)
			}
			if !strings.Contains(info.Status.Reason, "could not be resolved") {
				t.Errorf("reason = %q", info.Status.Reason)
			}
			for _, call := range runner.calls {
				if strings.Contains(call, "version") || strings.Contains(call, " ps ") {
					t.Errorf("daemon was contacted for an invalid endpoint: %s", call)
				}
			}
		})
	}
}

func TestRemoteApprovalDoesNotRescueUnresolvedContext(t *testing.T) {
	runner := newFakeRunner(map[string]run.Result{
		"context inspect missing": {ExitCode: 1, Stderr: "context not found"},
	})
	info := Inspector{
		Runner:      runner,
		AllowRemote: true,
		LookupEnv:   envLookup(map[string]string{"DOCKER_CONTEXT": "missing"}),
	}.Collect(context.Background())

	if info.Status.Available {
		t.Fatal("unresolved context must not be contacted with --allow-remote-docker")
	}
	for _, call := range runner.calls {
		if strings.Contains(call, "version") {
			t.Errorf("daemon was contacted despite an unresolved context: %s", call)
		}
	}
}

func TestHostQualifiedNamedPipeIsNeverContacted(t *testing.T) {
	endpoints := []string{
		"npipe:////fileserver/pipe/docker_engine",
		"npipe:////localhost/pipe/docker_engine",
		"npipe://server/pipe/docker_engine",
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			for _, allow := range []bool{false, true} {
				runner := newFakeRunner(nil)
				info := Inspector{
					Runner:      runner,
					AllowRemote: allow,
					LookupEnv:   envLookup(map[string]string{"DOCKER_HOST": endpoint}),
				}.Collect(context.Background())

				if info.Status.Available {
					t.Fatalf("allowRemote=%v: host-qualified pipe was contacted", allow)
				}
				if info.Status.EndpointLocal {
					t.Errorf("allowRemote=%v: host-qualified pipe classified as local", allow)
				}
				for _, call := range runner.calls {
					if strings.Contains(call, "version") || strings.Contains(call, " ps ") {
						t.Errorf("allowRemote=%v: daemon command issued: %s", allow, call)
					}
				}
			}
		})
	}
}

func TestTargetApprovedRequiresValidEndpoint(t *testing.T) {
	tests := []struct {
		name   string
		target Target
		allow  bool
		want   bool
	}{
		{"local valid", Target{Valid: true, Local: true}, false, true},
		{"remote valid without approval", Target{Valid: true}, false, false},
		{"remote valid with approval", Target{Valid: true}, true, true},
		{"invalid with approval", Target{}, true, false},
		{"invalid local with approval", Target{Local: true}, true, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.target.Approved(tc.allow); got != tc.want {
				t.Errorf("Approved(%v) = %v, want %v (%+v)", tc.allow, got, tc.want, tc.target)
			}
		})
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
