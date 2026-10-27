package winproc

import (
	"context"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/model"
	"github.com/Caseymccallum/StackBroom/internal/run"
)

// Sources of process command lines, reported so the CLI can be explicit about
// how much it actually knows.
const (
	SourceCIM      = "cim"
	SourceNone     = "none"
	SourceDisabled = "disabled"
	SourceFailed   = "failed"
	SourceOtherOS  = "unsupported-platform"
)

// Options controls process collection.
type Options struct {
	// Runner executes the PowerShell CIM query. Tests inject a fake.
	Runner run.Runner
	// EnrichCommandLines requests command lines and start times. Without them
	// StackBroom can identify runtimes but not roles or projects.
	EnrichCommandLines bool
	// Timeout bounds the CIM query. Zero uses run.DefaultTimeout.
	Timeout time.Duration
}

// Result is the outcome of process collection. An empty process list with a
// warning is a valid result on non-Windows platforms.
type Result struct {
	Processes   []model.Process
	Source      string
	SourceError string
	Warnings    []string
}

// Collect enumerates processes and classifies the ones it recognises.
func Collect(ctx context.Context, opts Options) (Result, error) {
	return collectProcesses(ctx, opts)
}
