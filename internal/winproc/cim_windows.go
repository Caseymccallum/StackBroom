//go:build windows

package winproc

import (
	"context"
	"errors"

	"github.com/Caseymccallum/StackBroom/internal/run"
)

func queryCommandLines(ctx context.Context, opts Options) (map[int32]Enrichment, []string, error) {
	runner := opts.Runner
	if runner == nil {
		runner = run.Exec{Timeout: opts.Timeout}
	}

	res := runner.Run(ctx, "powershell.exe",
		"-NoProfile",
		"-NonInteractive",
		"-ExecutionPolicy", "Bypass",
		"-Command", cimScript,
	)
	if !res.OK() {
		return nil, nil, errors.New(res.FailureReason())
	}

	enrich, warnings, err := ParseCIMProcesses([]byte(res.Stdout))
	if err != nil {
		return nil, warnings, err
	}
	if len(enrich) == 0 {
		return enrich, warnings, errors.New("PowerShell returned no usable command lines")
	}
	return enrich, warnings, nil
}
