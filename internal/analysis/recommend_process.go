package analysis

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/model"
	"github.com/Caseymccallum/StackBroom/internal/units"
)

// wslRecommendations reports running distributions that are not part of the
// Docker Desktop plumbing. It never suggests which distribution is "unused":
// that cannot be known from outside the distribution.
func wslRecommendations(wslInfo model.WSLInfo) []model.Recommendation {
	if !wslInfo.Available {
		return nil
	}
	var recs []model.Recommendation
	for _, d := range wslInfo.Distributions {
		if !d.Running() {
			continue
		}
		if strings.HasPrefix(strings.ToLower(d.Name), "docker-desktop") {
			// Managed by Docker Desktop; not a developer decision.
			continue
		}

		rec := model.Recommendation{
			ID:         KindWSLRunning + "-" + strings.ToLower(d.Name),
			Kind:       KindWSLRunning,
			Title:      fmt.Sprintf("WSL distribution %q is running", d.Name),
			Detected:   fmt.Sprintf("wsl.exe --list --verbose reports %q with state %q (WSL version %s).", d.Name, d.State, d.Version),
			Risk:       model.RiskHigh,
			Confidence: model.Confirmed,
			WhyUnnecessary: "A running WSL2 distribution keeps the shared WSL utility VM alive, and that VM holds memory on the host " +
				"even while the distribution is idle. Whether the distribution itself is still needed cannot be determined from outside.",
			Evidence: []string{
				"State is reported by WSL itself and was not changed by StackBroom.",
				"StackBroom did not run any command inside the distribution, so per-distribution memory use is unknown.",
			},
			Review: []string{
				"Check for open terminals, VS Code Remote-WSL windows or other tools attached to this distribution.",
				"Confirm no long-running build, database or container workload depends on it.",
				"Save any work inside the distribution before considering a shutdown.",
			},
			RiskDetail: "High: terminating a WSL distribution stops every process inside it immediately, including databases, builds and " +
				"unsaved editor work. A restart may not restore in-memory state.",
			Subjects: []model.Subject{{Kind: "wsl-distribution", ID: d.Name, Detail: d.State}},
		}

		if wslInfo.VMMemoryKnown {
			rec.Resource = fmt.Sprintf("Host-side WSL utility VM working set: %s (approximate, shared across all running distributions).",
				units.Bytes(wslInfo.VMMemoryBytes))
			rec.ResourceBytes = wslInfo.VMMemoryBytes
			rec.Evidence = append(rec.Evidence,
				fmt.Sprintf("The %s process reports %s, which covers the whole WSL2 VM rather than this distribution.",
					wslInfo.VMProcessName, units.Bytes(wslInfo.VMMemoryBytes)))
			if wslInfo.VMNote != "" {
				rec.Evidence = append(rec.Evidence, wslInfo.VMNote)
			}
		} else {
			rec.Resource = "Memory use unknown: the host-side WSL VM memory could not be measured."
			rec.ResourceUnknown = true
		}
		recs = append(recs, rec)
	}
	return recs
}

// longRunningRecommendations offers project-confirmed development servers that
// have been running for longer than the threshold. Age is treated as a prompt,
// never as proof.
func longRunningRecommendations(procs []model.Process, now time.Time, opts Options) []model.Recommendation {
	type aged struct {
		process model.Process
		age     time.Duration
	}
	var candidates []aged

	for _, p := range procs {
		if p.Category != model.CategoryDevServer || !p.StartTimeKnown || p.StartedAt == nil {
			continue
		}
		if p.Project == nil || p.Project.Confidence != model.Confirmed {
			continue
		}
		age := now.Sub(*p.StartedAt)
		if age < opts.StaleProcessAfter {
			continue
		}
		candidates = append(candidates, aged{process: p, age: age})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].age > candidates[j].age
	})
	if len(candidates) > 5 {
		candidates = candidates[:5]
	}

	recs := make([]model.Recommendation, 0, len(candidates))
	for _, c := range candidates {
		p := c.process
		kind := p.DevServerKind
		if kind == "" {
			kind = string(p.Runtime) + " development server"
		}
		rec := model.Recommendation{
			ID:         fmt.Sprintf("%s-%d", KindLongRunningServer, p.PID),
			Kind:       KindLongRunningServer,
			Title:      fmt.Sprintf("%s for %s has been running for %s", kind, p.Project.Name, ageString(p.StartedAt, now)),
			Detected:   fmt.Sprintf("PID %d (%s) has been running since %s for the project at %s.", p.PID, p.Name, p.StartedAt.Local().Format("2006-01-02 15:04"), p.Project.Root),
			Risk:       model.RiskMedium,
			Confidence: model.Confirmed,
			WhyUnnecessary: "A development server started hours ago is often left behind after switching projects, while still holding " +
				"memory and a port. Age alone is not evidence that the server is unnecessary; this is a prompt to check.",
			Evidence: append(append([]string{}, p.Project.Evidence...),
				fmt.Sprintf("Process start time from Windows CIM: %s.", p.StartedAt.Local().Format(time.RFC3339))),
			Review: []string{
				"Check whether the terminal or editor task that started it is still open.",
				"Confirm nothing is using the server: a browser tab, a device on your network or a colleague may be connected.",
				"Restart it later if you still need it; the cost of stopping it is usually only a restart.",
			},
			RiskDetail: "Medium: stopping the server interrupts anything currently connected and loses in-memory state. Make sure the " +
				"server is not serving work you still need.",
			Subjects: []model.Subject{{Kind: "process", ID: fmt.Sprintf("%d", p.PID), Detail: p.CommandLine}},
		}
		if len(p.Ports) > 0 {
			rec.Evidence = append(rec.Evidence, "Command line requests port "+joinPorts(p.Ports)+".")
		}
		if p.MemoryKnown {
			rec.Resource = units.Bytes(p.WorkingSetBytes) + " working set (Windows measurement)."
			rec.ResourceBytes = p.WorkingSetBytes
		} else {
			rec.Resource = "Memory use unknown: Windows refused access to this process."
			rec.ResourceUnknown = true
		}
		recs = append(recs, rec)
	}
	return recs
}
