package wsl

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/model"
	"github.com/Caseymccallum/StackBroom/internal/run"
	"github.com/Caseymccallum/StackBroom/internal/wintext"
)

// Inspector collects WSL information.
type Inspector struct {
	// Runner executes commands. Tests inject a fake.
	Runner run.Runner
	// Timeout bounds the command. Zero uses run.DefaultTimeout.
	Timeout time.Duration
	// Binary is the WSL executable; empty means "wsl.exe".
	Binary string
}

// Collect lists distributions. It never starts or stops anything, so the
// reported state is the state before StackBroom ran.
func (i Inspector) Collect(ctx context.Context) model.WSLInfo {
	binary := i.Binary
	if binary == "" {
		binary = "wsl.exe"
	}
	runner := i.Runner
	if runner == nil {
		runner = run.Exec{Timeout: i.Timeout}
	}

	info := model.WSLInfo{Distributions: []model.WSLDistribution{}}

	res := runner.Run(ctx, binary, "--list", "--verbose")
	stdout := wintext.DecodeConsole([]byte(res.Stdout))
	stderr := wintext.DecodeConsole([]byte(res.Stderr))

	if res.TimedOut {
		info.Reason = "wsl.exe did not respond within " + res.Timeout.String()
		return info
	}
	if res.Missing {
		info.Reason = "wsl.exe was not found; WSL is not installed on this machine"
		return info
	}

	if !res.OK() {
		combined := strings.ToLower(stdout + " " + stderr)
		switch {
		case strings.Contains(combined, "no installed distributions"):
			info.Available = true
			info.Reason = "WSL is installed but has no distributions"
			return info
		case strings.Contains(combined, "not installed") ||
			strings.Contains(combined, "optional component") ||
			strings.Contains(combined, "is not enabled"):
			info.Reason = firstNonEmptyLine(stderr, stdout, "WSL is not available on this machine")
			return info
		case strings.Contains(combined, "requires an update") || strings.Contains(combined, "wsl 2 requires"):
			info.Reason = firstNonEmptyLine(stderr, stdout, "WSL needs an update before it can be listed")
			return info
		default:
			info.Reason = firstNonEmptyLine(stderr, stdout, "wsl.exe exited with code "+strconv.Itoa(res.ExitCode))
			return info
		}
	}

	info.Available = true
	dists, warnings := ParseDistributionList(stdout)
	info.Distributions = dists
	if len(warnings) > 0 {
		info.Reason = warnings[0]
	}
	return info
}

func firstNonEmptyLine(sources ...string) string {
	for _, src := range sources {
		for _, line := range strings.Split(src, "\n") {
			line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
			if line != "" {
				return line
			}
		}
	}
	return ""
}
