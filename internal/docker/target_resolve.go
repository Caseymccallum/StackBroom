package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Caseymccallum/StackBroom/internal/run"
)

// ResolveTarget determines which daemon the Docker CLI would contact, using the
// documented precedence, without contacting any daemon.
//
// Precedence (Docker CLI reference, verified 27 Oct 2026):
//  1. DOCKER_CONTEXT selects a context. For a named context, its endpoint takes
//     precedence over DOCKER_HOST.
//  2. The special "default" context derives its endpoint from DOCKER_HOST, so
//     `docker context inspect default` reports DOCKER_HOST.
//  3. Otherwise the active context (set with `docker context use`) applies.
//
// StackBroom inspects the selected context and classifies the endpoint Docker
// reports, so it follows the CLI's effective target even when these rules
// interact.
func ResolveTarget(ctx context.Context, runner run.Runner, binary string, lookup func(string) (string, bool)) Target {
	if runner == nil {
		runner = run.Exec{}
	}
	if strings.TrimSpace(binary) == "" {
		binary = "docker"
	}
	if lookup == nil {
		lookup = os.LookupEnv
	}

	contextEnv, hasContextEnv := lookup("DOCKER_CONTEXT")
	hostEnv, hasHostEnv := lookup("DOCKER_HOST")

	if hasContextEnv && strings.TrimSpace(contextEnv) != "" {
		name := strings.TrimSpace(contextEnv)
		if strings.HasPrefix(name, "-") || strings.ContainsAny(name, "\r\n\t") {
			return Target{
				Context: name,
				Source:  SourceContextEnv,
				Reason:  "DOCKER_CONTEXT does not name a usable context",
			}
		}
		return inspectContext(ctx, runner, binary, name, SourceContextEnv)
	}

	if hasHostEnv && strings.TrimSpace(hostEnv) != "" {
		raw := strings.TrimSpace(hostEnv)
		class := ClassifyEndpoint(raw)
		return Target{
			Endpoint: raw,
			Display:  displayEndpoint(raw),
			Context:  activeContextName(ctx, runner, binary),
			Source:   SourceHostEnv,
			Local:    class.Local,
			Valid:    class.Valid,
			Reason:   class.Reason,
		}
	}

	name := activeContextName(ctx, runner, binary)
	if name == "" {
		return Target{
			Source: SourceUnknown,
			Reason: "the active Docker context could not be determined; no daemon was contacted",
		}
	}
	return inspectContext(ctx, runner, binary, name, SourceActiveCtx)
}

func activeContextName(ctx context.Context, runner run.Runner, binary string) string {
	res := runner.Run(ctx, binary, "context", "show")
	if !res.OK() {
		return ""
	}
	return strings.TrimSpace(res.Stdout)
}

type contextInspectEntry struct {
	Name      string `json:"Name"`
	Endpoints map[string]struct {
		Host string `json:"Host"`
	} `json:"Endpoints"`
}

func inspectContext(ctx context.Context, runner run.Runner, binary, name, source string) Target {
	target := Target{Context: name, Source: source}
	if strings.TrimSpace(name) == "" {
		target.Source = SourceUnknown
		target.Reason = "the Docker context name is empty"
		return target
	}

	res := runner.Run(ctx, binary, "context", "inspect", name)
	switch {
	case res.Missing:
		target.Source = SourceUnknown
		target.Reason = "Docker CLI not found on PATH"
		return target
	case res.TimedOut:
		target.Source = SourceUnknown
		target.Reason = "docker context inspect did not respond within " + res.Timeout.String()
		return target
	case !res.OK():
		target.Source = SourceUnknown
		target.Reason = "the Docker context " + name + " could not be inspected: " + res.FailureReason()
		return target
	}

	entries, err := parseContextInspect(res.Stdout)
	if err != nil {
		target.Source = SourceUnknown
		target.Reason = err.Error()
		return target
	}
	if len(entries) == 0 {
		target.Source = SourceUnknown
		target.Reason = "the Docker context " + name + " returned no endpoint"
		return target
	}

	endpoint, ok := entries[0].Endpoints[dockerEndpointKey]
	if !ok {
		target.Source = SourceUnknown
		target.Reason = "the Docker context " + name + " has no \"docker\" endpoint; no daemon was contacted"
		return target
	}
	host := strings.TrimSpace(endpoint.Host)
	if host == "" {
		target.Source = SourceUnknown
		target.Reason = "the Docker context " + name + " has a blank \"docker\" endpoint; no daemon was contacted"
		return target
	}

	class := ClassifyEndpoint(host)
	target.Endpoint = host
	target.Display = displayEndpoint(host)
	target.Local = class.Local
	target.Valid = class.Valid
	target.Reason = class.Reason
	return target
}

// parseContextInspect accepts both the array form Docker emits and a single
// object, so a future CLI change does not silently make every endpoint unknown.
func parseContextInspect(out string) ([]contextInspectEntry, error) {
	text := strings.TrimSpace(out)
	if text == "" {
		return nil, fmt.Errorf("docker context inspect returned no data")
	}
	var entries []contextInspectEntry
	if strings.HasPrefix(text, "[") {
		if err := json.Unmarshal([]byte(text), &entries); err != nil {
			return nil, fmt.Errorf("docker context inspect returned invalid JSON")
		}
		return entries, nil
	}
	var one contextInspectEntry
	if err := json.Unmarshal([]byte(text), &one); err != nil {
		return nil, fmt.Errorf("docker context inspect returned invalid JSON")
	}
	return []contextInspectEntry{one}, nil
}
