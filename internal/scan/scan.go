// Package scan coordinates the read-only collectors and the analysis layer.
//
// Every dependency is an interface with a production default, which keeps the
// orchestration testable without touching live processes, Docker or WSL.
package scan

import (
	"context"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/analysis"
	"github.com/Caseymccallum/StackBroom/internal/docker"
	"github.com/Caseymccallum/StackBroom/internal/model"
	"github.com/Caseymccallum/StackBroom/internal/project"
	"github.com/Caseymccallum/StackBroom/internal/run"
	"github.com/Caseymccallum/StackBroom/internal/winproc"
	"github.com/Caseymccallum/StackBroom/internal/wsl"
)

// ProcessSource collects processes.
type ProcessSource interface {
	Collect(ctx context.Context) (winproc.Result, error)
}

// DockerSource collects Docker observations.
type DockerSource interface {
	Collect(ctx context.Context) model.DockerInfo
}

// WSLSource collects WSL observations.
type WSLSource interface {
	Collect(ctx context.Context) model.WSLInfo
}

// ProjectResolver associates a process with a project.
type ProjectResolver interface {
	Resolve(exePath, commandLine string) *model.ProjectRef
}

// Options configures a scan.
type Options struct {
	Runner             run.Runner
	Timeout            time.Duration
	EnrichCommandLines bool
	SkipDocker         bool
	SkipWSL            bool
	// AllowRemoteDocker permits inspecting a Docker daemon that is not on a
	// local IPC channel. It is false by default.
	AllowRemoteDocker bool
	TopN              int
	Recommendation    analysis.Options
	Now               func() time.Time

	// Injection points; nil means use the production implementation.
	Processes ProcessSource
	Docker    DockerSource
	WSL       WSLSource
	Projects  ProjectResolver
}

// Scanner runs one scan.
type Scanner struct {
	opts Options
}

// processCollector adapts winproc.Collect to ProcessSource.
type processCollector struct {
	options winproc.Options
}

func (p processCollector) Collect(ctx context.Context) (winproc.Result, error) {
	return winproc.Collect(ctx, p.options)
}

// New builds a Scanner, filling in production defaults.
func New(opts Options) *Scanner {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.TopN <= 0 {
		opts.TopN = analysis.DefaultTopN
	}
	if opts.Projects == nil {
		opts.Projects = project.Resolver{}
	}
	if opts.Processes == nil {
		opts.Processes = processCollector{
			options: winproc.Options{
				Runner:             opts.Runner,
				EnrichCommandLines: opts.EnrichCommandLines,
				Timeout:            opts.Timeout,
			},
		}
	}
	if opts.Docker == nil {
		opts.Docker = docker.Inspector{
			Runner:      opts.Runner,
			Timeout:     opts.Timeout,
			AllowRemote: opts.AllowRemoteDocker,
		}
	}
	if opts.WSL == nil {
		opts.WSL = wsl.Inspector{Runner: opts.Runner, Timeout: opts.Timeout}
	}
	return &Scanner{opts: opts}
}
