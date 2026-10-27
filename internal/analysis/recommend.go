package analysis

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/model"
	"github.com/Caseymccallum/StackBroom/internal/units"
)

// Recommendation kinds. These are stable identifiers for JSON consumers.
const (
	KindDuplicateServer   = "duplicate-dev-server"
	KindStoppedContainers = "stopped-containers"
	KindBuildCache        = "docker-build-cache"
	KindUnusedImages      = "docker-unused-images"
	KindWSLRunning        = "wsl-running-distribution"
	KindLongRunningServer = "long-running-dev-server"
)

// Options sets the thresholds used when deciding whether something is worth
// mentioning. Thresholds only control noise, never action: StackBroom does not
// act on any of them.
type Options struct {
	// StaleProcessAfter is how old a project-confirmed development server must
	// be before it is offered for review.
	StaleProcessAfter time.Duration
	// StaleContainerDays is the minimum reported age of a stopped container.
	StaleContainerDays int
	// BuildCacheWarnBytes is the reclaimable build-cache size worth mentioning.
	BuildCacheWarnBytes uint64
	// ImageReclaimWarnBytes is the reclaimable image size worth mentioning.
	ImageReclaimWarnBytes uint64
	// MaxContainerNames bounds how many container names are listed.
	MaxContainerNames int
}

// DefaultOptions returns conservative thresholds.
func DefaultOptions() Options {
	return Options{
		StaleProcessAfter:     8 * time.Hour,
		StaleContainerDays:    14,
		BuildCacheWarnBytes:   250 * 1024 * 1024,
		ImageReclaimWarnBytes: 1024 * 1024 * 1024,
		MaxContainerNames:     8,
	}
}

// Recommend produces read-only suggestions from collected observations.
//
// Confidence describes the reliability of the detected facts (Docker's own
// state, a supported project match, matching command-line ports). It is not a
// claim that the suggested clean-up is correct - that judgement stays with the
// user.
func Recommend(procs []model.Process, dockerInfo model.DockerInfo, wslInfo model.WSLInfo, dups []model.DuplicateGroup, now time.Time, opts Options) []model.Recommendation {
	defaults := DefaultOptions()
	if opts.MaxContainerNames <= 0 {
		opts.MaxContainerNames = defaults.MaxContainerNames
	}
	if opts.StaleContainerDays <= 0 {
		opts.StaleContainerDays = defaults.StaleContainerDays
	}
	if opts.BuildCacheWarnBytes == 0 {
		opts.BuildCacheWarnBytes = defaults.BuildCacheWarnBytes
	}
	if opts.ImageReclaimWarnBytes == 0 {
		opts.ImageReclaimWarnBytes = defaults.ImageReclaimWarnBytes
	}
	if opts.StaleProcessAfter <= 0 {
		opts.StaleProcessAfter = defaults.StaleProcessAfter
	}

	var recs []model.Recommendation
	recs = append(recs, duplicateRecommendations(dups)...)
	recs = append(recs, stoppedContainerRecommendation(dockerInfo, opts)...)
	recs = append(recs, buildCacheRecommendation(dockerInfo, opts)...)
	recs = append(recs, unusedImageRecommendation(dockerInfo, opts)...)
	recs = append(recs, wslRecommendations(wslInfo)...)
	recs = append(recs, longRunningRecommendations(procs, now, opts)...)
	sortRecommendations(recs)
	return recs
}

func sortRecommendations(recs []model.Recommendation) {
	priority := map[string]int{
		KindDuplicateServer:   0,
		KindStoppedContainers: 1,
		KindBuildCache:        2,
		KindUnusedImages:      3,
		KindWSLRunning:        4,
		KindLongRunningServer: 5,
	}
	sort.SliceStable(recs, func(i, j int) bool {
		pi, pj := priority[recs[i].Kind], priority[recs[j].Kind]
		if pi != pj {
			return pi < pj
		}
		return recs[i].ResourceBytes > recs[j].ResourceBytes
	})
}

func duplicateRecommendations(dups []model.DuplicateGroup) []model.Recommendation {
	recs := make([]model.Recommendation, 0, len(dups))
	for i, g := range dups {
		rec := model.Recommendation{
			ID:              fmt.Sprintf("%s-%d", KindDuplicateServer, i+1),
			Kind:            KindDuplicateServer,
			Confidence:      g.Confidence,
			Risk:            model.RiskMedium,
			ResourceBytes:   g.MemoryBytes,
			ResourceUnknown: g.MemoryKnown == 0,
		}

		if g.Kind == model.DuplicateByPort {
			rec.Title = fmt.Sprintf("Two %s development servers appear to request port %d", g.Runtime, g.Port)
			rec.Detected = fmt.Sprintf("%d %s development server processes each request port %d (PIDs %s).",
				len(g.PIDs), g.Runtime, g.Port, pidList(g.PIDs))
			rec.WhyUnnecessary = "Only one process can listen on a TCP port. If both are still running, one is likely " +
				"a leftover from an earlier session, or is failing to bind and retrying."
		} else {
			rec.Title = fmt.Sprintf("Two %s development servers appear to run for the same project", g.Runtime)
			rec.Detected = fmt.Sprintf("%d %s development server processes reference the project at %s (PIDs %s).",
				len(g.PIDs), g.Runtime, g.ProjectRoot, pidList(g.PIDs))
			rec.WhyUnnecessary = "Two servers for one project usually means an earlier session was not closed. " +
				"One of them may be doing nothing useful."
		}

		rec.Evidence = append(rec.Evidence, g.Evidence...)
		if g.Kind == model.DuplicateByPort {
			rec.Evidence = append(rec.Evidence,
				"Evidence is the requested port in each command line; StackBroom did not check which process owns the listening socket.",
				"Ports can also come from environment variables or config files, which are not visible to StackBroom.")
		}

		rec.Resource = memoryText(g.MemoryBytes, g.MemoryKnown, len(g.PIDs))
		rec.Review = []string{
			"Identify which terminal, editor task or service manager started each PID.",
			"Confirm which process is serving the project you are actively using.",
		}
		if g.Port > 0 {
			rec.Review = append(rec.Review, fmt.Sprintf(
				"Check the listening process yourself, for example with `Get-NetTCPConnection -LocalPort %d` in PowerShell; StackBroom does not run it.", g.Port))
		}
		rec.Review = append(rec.Review, "Close only the session you recognise as finished.")
		rec.RiskDetail = "Stopping the wrong process ends an active development session and may lose unsaved in-memory state. " +
			"Restarting the server is usually quick, but any state held only in that process is lost."

		for _, pid := range g.PIDs {
			rec.Subjects = append(rec.Subjects, model.Subject{Kind: "process", ID: fmt.Sprintf("%d", pid)})
		}
		recs = append(recs, rec)
	}
	return recs
}

func memoryText(bytes uint64, known, total int) string {
	switch {
	case known == 0:
		return "Memory use unknown: Windows refused access to these processes."
	case known < total:
		return fmt.Sprintf("%s working set reported by %d of %d processes; the rest is unknown.",
			units.Bytes(bytes), known, total)
	default:
		return units.Bytes(bytes) + " working set."
	}
}

func pidList(pids []int32) string {
	parts := make([]string, 0, len(pids))
	for _, pid := range pids {
		parts = append(parts, fmt.Sprintf("%d", pid))
	}
	return strings.Join(parts, ", ")
}
