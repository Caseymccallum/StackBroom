// Package cli implements StackBroom's command-line interface.
//
// The CLI is intentionally small: two read-only commands, one version command
// and a JSON mode. There is no interactive UI and no command that changes the
// system.
package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"runtime"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/model"
	"github.com/Caseymccallum/StackBroom/internal/run"
	"github.com/Caseymccallum/StackBroom/internal/scan"
	"github.com/Caseymccallum/StackBroom/internal/version"
)

// Exit codes.
const (
	ExitOK    = 0
	ExitError = 1
	ExitUsage = 2
)

const usageText = `StackBroom - know what's running, reclaim what's idle.

Usage:
  stackbroom scan [flags]     Read-only overview of development resource use
  stackbroom plan [flags]     Review suggestions; nothing is ever executed
  stackbroom version          Print the installed version
  stackbroom help             Show this help

Flags for scan and plan:
  --json                 Emit structured JSON instead of text
  --top N                Memory-heavy processes to list (default 8)
  --timeout DURATION     Limit for each external command (default 20s)
  --skip-docker          Do not query Docker at all
  --skip-wsl             Do not query WSL at all
  --no-command-lines     Skip the PowerShell command-line lookup (faster,
                         but roles and projects cannot be identified)

StackBroom is read-only. It never stops processes, removes containers or
images, prunes Docker, stops WSL distributions or changes configuration.
`

// App is the CLI application. Output writers and the scanner are injectable so
// the presentation can be tested without touching the host.
type App struct {
	Out  io.Writer
	Err  io.Writer
	Scan func(ctx context.Context, opts scan.Options) model.Snapshot
	// BaseOptions are the scan defaults, overridable by flags.
	BaseOptions scan.Options
}

// New returns an App writing to out and errOut.
func New(out, errOut io.Writer) *App {
	return &App{Out: out, Err: errOut}
}

type commonFlags struct {
	jsonOut        bool
	timeout        time.Duration
	skipDocker     bool
	skipWSL        bool
	noCommandLines bool
}

func addCommonFlags(fs *flag.FlagSet, c *commonFlags) {
	fs.BoolVar(&c.jsonOut, "json", false, "emit structured JSON")
	fs.DurationVar(&c.timeout, "timeout", run.DefaultTimeout, "limit for each external command")
	fs.BoolVar(&c.skipDocker, "skip-docker", false, "do not query Docker")
	fs.BoolVar(&c.skipWSL, "skip-wsl", false, "do not query WSL")
	fs.BoolVar(&c.noCommandLines, "no-command-lines", false, "skip the PowerShell command-line lookup")
}

func (a *App) scanOptions(c *commonFlags, top int) scan.Options {
	opts := a.BaseOptions
	opts.Timeout = c.timeout
	opts.EnrichCommandLines = !c.noCommandLines
	opts.SkipDocker = c.skipDocker
	opts.SkipWSL = c.skipWSL
	if top > 0 {
		opts.TopN = top
	}
	return opts
}

func (a *App) run(ctx context.Context, opts scan.Options) model.Snapshot {
	if a.Scan != nil {
		return a.Scan(ctx, opts)
	}
	return scan.New(opts).Run(ctx)
}

// Run executes one command and returns a process exit code.
func (a *App) Run(ctx context.Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprint(a.Out, usageText)
		return ExitUsage
	}

	switch args[0] {
	case "help", "-h", "--help", "-help":
		fmt.Fprint(a.Out, usageText)
		return ExitOK
	case "version", "-version", "--version":
		fmt.Fprintf(a.Out, "%s %s (%s/%s, %s)\n", version.Name, version.Version, runtime.GOOS, runtime.GOARCH, runtime.Version())
		return ExitOK
	case "scan":
		return a.runScan(ctx, args[1:])
	case "plan":
		return a.runPlan(ctx, args[1:])
	default:
		fmt.Fprintf(a.Err, "stackbroom: unknown command %q\n\n%s", args[0], usageText)
		return ExitUsage
	}
}

func (a *App) runScan(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	var common commonFlags
	addCommonFlags(fs, &common)
	top := fs.Int("top", 0, "memory-heavy processes to list")
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}

	snapshot := a.run(ctx, a.scanOptions(&common, *top))

	if common.jsonOut {
		if err := writeJSON(a.Out, snapshot); err != nil {
			fmt.Fprintf(a.Err, "stackbroom: could not write JSON: %v\n", err)
			return ExitError
		}
		return ExitOK
	}
	renderScan(a.Out, snapshot)
	return ExitOK
}

func (a *App) runPlan(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	var common commonFlags
	addCommonFlags(fs, &common)
	if err := fs.Parse(args); err != nil {
		return ExitUsage
	}

	snapshot := a.run(ctx, a.scanOptions(&common, 0))

	if common.jsonOut {
		doc := model.PlanDocument{
			SchemaVersion:   model.SchemaVersion,
			ReadOnly:        true,
			Executes:        false,
			Tool:            snapshot.Tool,
			GeneratedAt:     snapshot.GeneratedAt,
			Summary:         snapshot.Summary,
			Recommendations: snapshot.Recommendations,
			Warnings:        snapshot.Warnings,
		}
		if err := writeJSON(a.Out, doc); err != nil {
			fmt.Fprintf(a.Err, "stackbroom: could not write JSON: %v\n", err)
			return ExitError
		}
		return ExitOK
	}
	renderPlan(a.Out, snapshot)
	return ExitOK
}

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}
