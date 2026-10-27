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
			reliable, uncertain := participantListeningPIDs(g)
			switch {
			case len(reliable) == 0 && len(uncertain) > 0:
				rec.Title = fmt.Sprintf("Listener rows reference port %d, but their ownership is uncertain", g.Port)
				rec.Detected = fmt.Sprintf(
					"%d %s development server processes are associated with port %d (PIDs %s). Listener rows exist, but none could be tied to a verified process identity.",
					len(g.PIDs), g.Runtime, g.Port, pidList(g.PIDs))
				rec.WhyUnnecessary = "The TCP table reported listeners on this port, but the owning process could not be verified. " +
					"StackBroom cannot say which development server is serving traffic, or whether either is redundant."
			case len(reliable) == 0 && len(g.Listeners) > 0:
				rec.Title = fmt.Sprintf("Two %s development servers request port %d; the port is held by another process", g.Runtime, g.Port)
				rec.Detected = fmt.Sprintf(
					"%d %s development server processes request port %d (PIDs %s); no listener owned by these PIDs was observed, but the port is bound by another process.",
					len(g.PIDs), g.Runtime, g.Port, pidList(g.PIDs))
				rec.WhyUnnecessary = "The port is bound, but not by either recognised development server in this group. " +
					"They may be configured for another port, may not have started, or may be unable to bind; StackBroom cannot distinguish those cases."
			case len(reliable) == 0:
				rec.Title = fmt.Sprintf("Two %s development servers request port %d; no listener was observed", g.Runtime, g.Port)
				rec.Detected = fmt.Sprintf(
					"%d %s development server processes request port %d (PIDs %s); no listener owned by these PIDs was observed.",
					len(g.PIDs), g.Runtime, g.Port, pidList(g.PIDs))
				rec.WhyUnnecessary = "A port on a command line is a request, not proof that the socket was bound. The effective port may come " +
					"from a config file or environment variable, and startup may be incomplete. StackBroom cannot tell which process, if any, will serve traffic."
			case len(reliable) == 1:
				rec.Title = fmt.Sprintf("Two %s development servers target port %d; only PID %d is observed listening",
					g.Runtime, g.Port, reliable[0])
				rec.Detected = fmt.Sprintf(
					"%d %s development server processes are associated with port %d (PIDs %s); only PID %d has an observed listener.",
					len(g.PIDs), g.Runtime, g.Port, pidList(g.PIDs), reliable[0])
				rec.WhyUnnecessary = "One process was observed bound to the port. The other requested it but no listener owned by its PID was observed. " +
					"It may not have started, may use another port, or may be unable to bind; StackBroom cannot distinguish those cases."
			default:
				rec.Title = fmt.Sprintf("Two %s development servers are observed listening on port %d", g.Runtime, g.Port)
				rec.Detected = fmt.Sprintf(
					"%d %s development server processes are associated with port %d (PIDs %s); %d participants have observed listeners.",
					len(g.PIDs), g.Runtime, g.Port, pidList(g.PIDs), len(reliable))
				rec.WhyUnnecessary = "Both sockets were bound at scan time. That can be intentional when the addresses or address families differ, " +
					"or it can be a leftover session. A binding proves the socket, not that either process is redundant."
			}
		} else {
			rec.Title = fmt.Sprintf("Two %s development servers appear to run for the same project", g.Runtime)
			rec.Detected = fmt.Sprintf("%d %s development server processes reference the project at %s (PIDs %s).",
				len(g.PIDs), g.Runtime, g.ProjectRoot, pidList(g.PIDs))
			rec.WhyUnnecessary = "Two servers for one project usually means an earlier session was not closed. " +
				"Listener evidence can show which sockets are bound, but it does not prove either server is redundant."
		}

		rec.Evidence = append(rec.Evidence, g.Evidence...)
		if g.Kind == model.DuplicateByPort {
			if len(g.Listeners) == 0 {
				rec.Evidence = append(rec.Evidence,
					fmt.Sprintf("No listener owned by these PIDs was observed on port %d.", g.Port))
			}
			if hasUnknownOwnership(g.Listeners) {
				rec.Evidence = append(rec.Evidence,
					"At least one listener's ownership could not be verified, so its PID association is uncertain.")
			}
			rec.Evidence = append(rec.Evidence,
				"Requested ports and observed listeners are separate evidence. A request can be overridden by environment variables or config files.",
				"Listener observations are a point-in-time TCP table sample; they prove a socket was bound, not that a process is safe to stop.")
		} else {
			rec.Evidence = append(rec.Evidence,
				"Project association and listener observation are independent evidence; neither proves redundancy.")
		}

		rec.Resource = memoryText(g.MemoryBytes, g.MemoryKnown, len(g.PIDs))
		rec.Review = []string{
			"Identify which terminal, editor task or service manager started each PID.",
			"Confirm which process is serving the project you are actively using.",
		}
		if g.Port > 0 {
			rec.Review = append(rec.Review,
				fmt.Sprintf("Check the listening process yourself, for example with `Get-NetTCPConnection -LocalPort %d` in PowerShell; StackBroom does not run it.", g.Port))
			rec.Review = append(rec.Review,
				"Compare the observed addresses; different local addresses or IPv4/IPv6 bindings may be intentional.")
		}
		rec.Review = append(rec.Review,
			"Close only the session you recognise as finished; do not stop a process solely because a socket is bound.")
		rec.RiskDetail = "Stopping the wrong process ends an active development session and may lose unsaved in-memory state. " +
			"Restarting the server is usually quick, but any state held only in that process is lost."

		for _, pid := range g.PIDs {
			rec.Subjects = append(rec.Subjects, model.Subject{Kind: "process", ID: fmt.Sprintf("%d", pid)})
		}
		for _, listener := range g.Listeners {
			rec.Subjects = append(rec.Subjects, model.Subject{
				Kind:   "listener",
				ID:     listenerRefText(listener),
				Detail: fmt.Sprintf("pid=%d family=%s ownership=%s", listener.PID, listener.Family, listener.Ownership),
			})
		}
		recs = append(recs, rec)
	}
	return recs
}

// participantListeningPIDs splits group participants into those with at least
// one associated listener row (confirmed or inferred) and those whose only rows
// have unknown ownership. Rows owned by non-participants never count as one of
// the development servers.
func participantListeningPIDs(g model.DuplicateGroup) (reliable, uncertain []int32) {
	participants := make(map[int32]bool, len(g.PIDs))
	for _, pid := range g.PIDs {
		participants[pid] = true
	}
	reliableSet := map[int32]bool{}
	uncertainSet := map[int32]bool{}
	for _, listener := range g.Listeners {
		if !participants[listener.PID] {
			continue
		}
		if listener.Ownership == model.Unknown || listener.Ownership == "" {
			uncertainSet[listener.PID] = true
		} else {
			reliableSet[listener.PID] = true
		}
	}
	for pid := range reliableSet {
		reliable = append(reliable, pid)
	}
	for pid := range uncertainSet {
		if reliableSet[pid] {
			continue
		}
		uncertain = append(uncertain, pid)
	}
	sort.Slice(reliable, func(i, j int) bool { return reliable[i] < reliable[j] })
	sort.Slice(uncertain, func(i, j int) bool { return uncertain[i] < uncertain[j] })
	return reliable, uncertain
}

func hasUnknownOwnership(listeners []model.ListenerRef) bool {
	for _, listener := range listeners {
		if listener.Ownership == model.Unknown || listener.Ownership == "" {
			return true
		}
	}
	return false
}

func listenerRefText(listener model.ListenerRef) string {
	address := listener.Address
	if strings.Contains(address, ":") && !strings.HasPrefix(address, "[") {
		address = "[" + address + "]"
	}
	return fmt.Sprintf("%s:%d", address, listener.Port)
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
