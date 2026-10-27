package main

import (
	"context"
	"errors"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/model"
	"github.com/Caseymccallum/StackBroom/internal/sysinfo"
	"github.com/Caseymccallum/StackBroom/internal/version"
)

// ErrScanInProgress prevents a second refresh from stacking work on top of a
// running scan. The UI also disables refresh while busy, but the service is the
// authority.
var ErrScanInProgress = errors.New("a scan is already running")

const scanTimeout = 60 * time.Second

type activeScan struct {
	id     string
	cancel context.CancelFunc
}

// DesktopService is the only bridge exposed to the webview. Every method is
// read-only and returns DTOs rather than internal engine types.
type DesktopService struct {
	mu        sync.Mutex
	active    *activeScan
	sequence  atomic.Uint64
	scanFn    func(context.Context, string) model.Snapshot
	metricsFn func(context.Context) sysinfo.Metrics
	now       func() time.Time
}

func NewDesktopService() *DesktopService {
	return &DesktopService{
		scanFn:    runEngine,
		metricsFn: sysinfo.Collect,
		now:       time.Now,
	}
}

func (s *DesktopService) GetAppInfo() AppInfo {
	preview := os.Getenv("STACKBROOM_PREVIEW") == "1"
	previewTheme := ""
	if preview {
		previewTheme = os.Getenv("STACKBROOM_PREVIEW_THEME")
		if previewTheme != "light" {
			previewTheme = "dark"
		}
	}
	return AppInfo{
		Name:          version.Name,
		Version:       version.Version,
		SchemaVersion: model.SchemaVersion,
		Toolchain:     runtime.Version(),
		ReadOnly:      true,
		Privacy:       "Command lines, executable paths, project roots and user profile paths are withheld or tokenised before data reaches the interface.",
		Limitations: []string{
			"StackBroom is read-only. It cannot stop processes, remove containers or images, prune Docker, modify files or change Windows settings.",
			"Docker endpoints that are not local IPC channels are refused. The desktop shell has no remote-Docker override.",
			"CPU usage is a short interval sample from Windows system counters, not a continuous monitor.",
			"Listener ownership is confirmed only when the PID and its native creation time match before and after sampling; otherwise it is inferred or unknown.",
			"Docker and WSL inspection can take several seconds. The interface remains responsive and the scan can be cancelled.",
			"Storage shows volume capacity only. It does not claim that used space is safely reclaimable.",
			"No AI provider is contacted in this milestone.",
		},
		Preview:      preview,
		PreviewTheme: previewTheme,
	}
}

// QuickScan collects fast host data plus a process snapshot without command
// lines, Docker or WSL. It is the overview path.
func (s *DesktopService) QuickScan() (QuickResult, error) {
	ctx, id, err := s.begin()
	if err != nil {
		return QuickResult{}, err
	}
	defer s.finish(id)

	started := s.now()
	snapshot := sanitize(s.scanFn(ctx, "quick"))
	metrics := s.metricsFn(ctx)
	completed := s.now()
	status := scanStatus(ctx)
	warnings := mergeWarnings(metrics.Warnings, snapshot.Warnings, "quick")
	return QuickResult{
		ID:          id,
		Status:      status,
		Mode:        "quick",
		StartedAt:   started,
		CompletedAt: completed,
		DurationMS:  completed.Sub(started).Milliseconds(),
		Partial:     status != "ok" || len(warnings) > 0,
		Overview:    mapOverview(snapshot, metrics, "quick", completed),
		Warnings:    warnings,
	}, nil
}

// FullScan adds command-line classification, Docker, WSL and recommendation
// evidence. It is intentionally explicit rather than part of the fast timer.
func (s *DesktopService) FullScan() (FullResult, error) {
	ctx, id, err := s.begin()
	if err != nil {
		return FullResult{}, err
	}
	defer s.finish(id)

	started := s.now()
	snapshot := sanitize(s.scanFn(ctx, "full"))
	metrics := s.metricsFn(ctx)
	completed := s.now()
	status := scanStatus(ctx)
	warnings := mergeWarnings(metrics.Warnings, snapshot.Warnings, "full")
	return FullResult{
		ID:            id,
		Status:        status,
		Mode:          "full",
		StartedAt:     started,
		CompletedAt:   completed,
		DurationMS:    completed.Sub(started).Milliseconds(),
		Partial:       status != "ok" || len(warnings) > 0,
		Overview:      mapOverview(snapshot, metrics, "full", completed),
		Processes:     mapProcesses(snapshot.Processes),
		Developer:     mapDeveloper(snapshot),
		Opportunities: mapOpportunities(snapshot.Recommendations),
		Warnings:      warnings,
	}, nil
}
