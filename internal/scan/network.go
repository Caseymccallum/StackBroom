package scan

import (
	"fmt"
	"sort"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/model"
)

// maxAssociationWarnings bounds ownership notes so a machine with many
// protected processes cannot flood the report.
const maxAssociationWarnings = 12

// attachNetwork joins listener rows to the process snapshot by PID, then
// re-checks the process creation time to detect PID reuse. It never concludes
// that a process exited, crashed or failed to bind: an absent or unverifiable
// owner is reported as unknown or inferred.
func (s *Scanner) attachNetwork(snapshot *model.Snapshot, creationTimes map[int32]time.Time) {
	if len(snapshot.Network.Listeners) == 0 {
		return
	}

	byPID := make(map[int32]*model.Process, len(snapshot.Processes))
	for i := range snapshot.Processes {
		byPID[snapshot.Processes[i].PID] = &snapshot.Processes[i]
	}

	type identityResult struct {
		created time.Time
		ok      bool
	}
	verified := map[int32]identityResult{}
	var warnings []string
	inferredRows := 0

	for i := range snapshot.Network.Listeners {
		listener := &snapshot.Network.Listeners[i]
		if listener.PID <= 0 {
			listener.Ownership = model.Unknown
			warnings = append(warnings, fmt.Sprintf(
				"Listener on port %d has no usable owning PID; ownership is unknown.", listener.Port))
			continue
		}

		process, ok := byPID[listener.PID]
		if !ok {
			listener.Ownership = model.Unknown
			warnings = append(warnings, fmt.Sprintf(
				"Listener on port %d is owned by PID %d, which was not present in the process snapshot; no conclusion is drawn about why.",
				listener.Port, listener.PID))
			continue
		}

		// PID matching alone is not identity. Start with the snapshot's name
		// so the report is useful, then strengthen or weaken ownership.
		listener.ProcessName = process.Name
		if process.Project != nil {
			listener.ProjectName = process.Project.Name
		}

		expected, hasExpected := creationTimes[listener.PID]
		if !hasExpected {
			listener.Ownership = model.Inferred
			inferredRows++
			continue
		}

		current, checked := verified[listener.PID]
		if !checked {
			created, ok := s.opts.Identity(listener.PID)
			current = identityResult{created: created, ok: ok}
			verified[listener.PID] = current
		}
		if !current.ok {
			listener.Ownership = model.Inferred
			inferredRows++
			continue
		}
		if !expected.Equal(current.created) {
			// The PID now belongs to a different process. Do not attach the
			// listener to either process: the original owner is no longer
			// observable, and the new process has not been shown to own it.
			listener.Ownership = model.Unknown
			listener.ProcessName = ""
			listener.ProjectName = ""
			warnings = append(warnings, fmt.Sprintf(
				"PID %d changed identity between the process snapshot and the listener sample; ownership of port %d is unknown.",
				listener.PID, listener.Port))
			continue
		}
		listener.Ownership = model.Confirmed
	}

	attachListeningPorts(snapshot.Processes, snapshot.Network.Listeners)

	if inferredRows > 0 {
		warnings = append([]string{fmt.Sprintf(
			"Ownership is inferred for %d of %d listener rows: the process creation time was unavailable or could not be re-read. PID reuse was not ruled out for those rows.",
			inferredRows, len(snapshot.Network.Listeners))}, warnings...)
	}
	warnings = uniqueWarnings(warnings)
	if len(warnings) > maxAssociationWarnings {
		dropped := len(warnings) - maxAssociationWarnings
		warnings = warnings[:maxAssociationWarnings]
		warnings = append(warnings, fmt.Sprintf("%d additional listener ownership notes were omitted.", dropped))
	}
	snapshot.Network.Warnings = append(snapshot.Network.Warnings, warnings...)
	for _, warning := range warnings {
		snapshot.Warnings = append(snapshot.Warnings, "Network: "+warning)
	}
}

// attachListeningPorts records observed ports on the owning process. A port is
// attached only when ownership is confirmed or inferred; unknown ownership is
// left off the process entirely.
func attachListeningPorts(processes []model.Process, listeners []model.TCPListener) {
	byPID := make(map[int32]*model.Process, len(processes))
	for i := range processes {
		byPID[processes[i].PID] = &processes[i]
	}

	for _, listener := range listeners {
		if listener.Ownership == model.Unknown || listener.ProcessName == "" {
			continue
		}
		process, ok := byPID[listener.PID]
		if !ok {
			continue
		}
		process.ListeningPorts = appendUniquePort(process.ListeningPorts, listener.Port)
		process.PortOwnership = weakerOwnership(process.PortOwnership, listener.Ownership)
	}
	for i := range processes {
		sort.Ints(processes[i].ListeningPorts)
	}
}

func uniqueWarnings(warnings []string) []string {
	seen := make(map[string]bool, len(warnings))
	out := warnings[:0]
	for _, warning := range warnings {
		if seen[warning] {
			continue
		}
		seen[warning] = true
		out = append(out, warning)
	}
	return out
}

func appendUniquePort(ports []int, port int) []int {
	for _, existing := range ports {
		if existing == port {
			return ports
		}
	}
	return append(ports, port)
}

// weakerOwnership returns the least certain of two states. An empty state means
// no listener has been associated yet.
func weakerOwnership(current, next model.Confidence) model.Confidence {
	if current == "" {
		return next
	}
	rank := map[model.Confidence]int{
		model.Confirmed: 0,
		model.Inferred:  1,
		model.Unknown:   2,
	}
	if rank[next] > rank[current] {
		return next
	}
	return current
}
