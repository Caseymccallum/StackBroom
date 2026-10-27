//go:build !windows

package winproc

import "context"

// collectProcesses reports that process inspection is unavailable rather than
// failing the whole scan. StackBroom is Windows-first; other platforms still get
// Docker and WSL reporting where those tools exist.
func collectProcesses(ctx context.Context, opts Options) (Result, error) {
	_ = ctx
	_ = opts
	return Result{
		Source: SourceOtherOS,
		Warnings: []string{
			"Process inspection is only implemented for Windows; no process data was collected.",
		},
	}, nil
}
