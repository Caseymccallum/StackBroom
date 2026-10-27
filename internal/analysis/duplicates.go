package analysis

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/model"
	"github.com/Caseymccallum/StackBroom/internal/units"
)

// FindDuplicates looks for development servers that appear to serve the same
// purpose: the same runtime on the same port, or the same confirmed project
// running twice.
//
// Port groups use two independent kinds of evidence: ports requested on command
// lines and ports actually observed listening. A verified listener proves a
// socket was bound; it never proves that either process is redundant or safe to
// stop. An observed binding can be intentional (different addresses or address
// families), and environment variables or config files can change the effective
// port in ways StackBroom cannot see.
func FindDuplicates(procs []model.Process, network model.NetworkInfo) []model.DuplicateGroup {
	type portKey struct {
		runtime model.Runtime
		port    int
	}

	portGroups := map[portKey][]model.Process{}
	projectGroups := map[string][]model.Process{}
	projectRoots := map[string]string{}
	listenersByPort := map[int][]model.TCPListener{}
	processByPID := make(map[int32]model.Process, len(procs))

	for _, listener := range network.Listeners {
		listenersByPort[listener.Port] = append(listenersByPort[listener.Port], listener)
	}
	for _, p := range procs {
		processByPID[p.PID] = p
		if p.Category != model.CategoryDevServer {
			continue
		}
		for _, port := range p.RequestedPorts {
			if port > 0 {
				key := portKey{runtime: p.Runtime, port: port}
				portGroups[key] = append(portGroups[key], p)
			}
		}
		// A listener can exist without a recognisable port argument (for
		// example a port supplied by a config file). It still participates.
		for _, port := range p.ListeningPorts {
			if port > 0 {
				key := portKey{runtime: p.Runtime, port: port}
				portGroups[key] = append(portGroups[key], p)
			}
		}
		if p.Project != nil && p.Project.Confidence == model.Confirmed {
			key := strings.ToLower(p.Project.Root) + "|" + string(p.Runtime)
			projectGroups[key] = append(projectGroups[key], p)
			projectRoots[key] = p.Project.Root
		}
	}

	var out []model.DuplicateGroup
	seen := map[string]bool{}

	portKeys := make([]portKey, 0, len(portGroups))
	for k := range portGroups {
		portKeys = append(portKeys, k)
	}
	sort.Slice(portKeys, func(i, j int) bool {
		if portKeys[i].port != portKeys[j].port {
			return portKeys[i].port < portKeys[j].port
		}
		return portKeys[i].runtime < portKeys[j].runtime
	})

	for _, key := range portKeys {
		group := dedupeByPID(portGroups[key])
		if len(group) < 2 {
			continue
		}
		sig := pidSignature(group)
		if seen[sig] {
			continue
		}
		seen[sig] = true
		out = append(out, buildPortGroup(key.runtime, key.port, group,
			listenersByPort[key.port], processByPID, network))
	}

	projectKeys := make([]string, 0, len(projectGroups))
	for k := range projectGroups {
		projectKeys = append(projectKeys, k)
	}
	sort.Strings(projectKeys)

	for _, key := range projectKeys {
		group := dedupeByPID(projectGroups[key])
		if len(group) < 2 {
			continue
		}
		sig := pidSignature(group)
		if seen[sig] {
			continue
		}
		seen[sig] = true
		root := projectRoots[key]
		out = append(out, buildProjectGroup(
			fmt.Sprintf("%s in %s", string(group[0].Runtime), filepath.Base(root)),
			group[0].Runtime, root, group, network,
		))
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind == model.DuplicateByPort
		}
		return out[i].MemoryBytes > out[j].MemoryBytes
	})
	return out
}

func dedupeByPID(procs []model.Process) []model.Process {
	seen := map[int32]bool{}
	out := make([]model.Process, 0, len(procs))
	for _, p := range procs {
		if seen[p.PID] {
			continue
		}
		seen[p.PID] = true
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PID < out[j].PID })
	return out
}

func pidSignature(procs []model.Process) string {
	parts := make([]string, 0, len(procs))
	for _, p := range procs {
		parts = append(parts, fmt.Sprintf("%d", p.PID))
	}
	return strings.Join(parts, ",")
}

func buildPortGroup(runtime model.Runtime, port int, procs []model.Process,
	listeners []model.TCPListener, processByPID map[int32]model.Process,
	network model.NetworkInfo) model.DuplicateGroup {

	group := model.DuplicateGroup{
		Kind:       model.DuplicateByPort,
		Key:        fmt.Sprintf("%s on port %d", runtime, port),
		Runtime:    runtime,
		Port:       port,
		Confidence: portGroupConfidence(procs, listeners),
		Listeners:  listenerRefs(listeners),
	}

	participants := make(map[int32]bool, len(procs))
	var memory uint64
	known := 0
	for _, p := range procs {
		participants[p.PID] = true
		group.PIDs = append(group.PIDs, p.PID)
		if p.MemoryKnown {
			memory += p.WorkingSetBytes
			known++
		}

		requested := containsPort(p.RequestedPorts, port)
		rows := listenersForPID(listeners, p.PID)
		detail := fmt.Sprintf("PID %d (%s", p.PID, p.Name)
		if p.DevServerKind != "" {
			detail += ", " + p.DevServerKind
		}
		detail += ")"
		if requested {
			detail += fmt.Sprintf(" requests port %d", port)
		}
		if len(rows) > 0 {
			detail += "; observed listening on " + joinListenerRows(rows)
			if !requested {
				detail += " (no matching requested-port argument was found)"
			}
		} else if requested {
			detail += "; no listener owned by this PID was observed for this port"
		}
		if p.StartTimeKnown && p.StartedAt != nil {
			detail += ", running since " + p.StartedAt.Local().Format("2006-01-02 15:04")
		}
		if p.MemoryKnown {
			detail += ", " + units.Bytes(p.WorkingSetBytes) + " working set"
		}
		group.Evidence = append(group.Evidence, detail)
	}
	group.MemoryBytes = memory
	group.MemoryKnown = known

	// Rows owned by something other than a group participant matter: they can
	// explain why a requested port is not available.
	for _, pid := range listenerPIDs(listeners) {
		if participants[pid] {
			continue
		}
		if owner, ok := processByPID[pid]; ok {
			group.Evidence = append(group.Evidence, fmt.Sprintf(
				"Port %d is also held by PID %d (%s), which is not one of the %s development servers in this group.",
				port, pid, owner.Name, runtime))
		} else {
			group.Evidence = append(group.Evidence, fmt.Sprintf(
				"Port %d is also held by PID %d, which was not present in the process snapshot.",
				port, pid))
		}
	}

	noted := map[string]bool{}
	for _, listener := range listeners {
		switch listener.Ownership {
		case model.Inferred:
			key := fmt.Sprintf("%d|inferred", listener.PID)
			if noted[key] {
				continue
			}
			noted[key] = true
			group.Evidence = append(group.Evidence, fmt.Sprintf(
				"Ownership of the listener on %s is inferred: PID %d matched the snapshot but its creation time was not re-verified.",
				listenerText(listener), listener.PID))
		case model.Unknown:
			group.Evidence = append(group.Evidence, fmt.Sprintf(
				"Ownership of the listener on %s (PID %d) is unknown; it was not matched to a verified process identity.",
				listenerText(listener), listener.PID))
		}
	}

	switch {
	case len(listeners) == 0 && !network.Available:
		group.Evidence = append(group.Evidence,
			"TCP listener inspection was unavailable, so this group is based on requested ports only.")
	case len(listeners) == 0:
		group.Evidence = append(group.Evidence,
			fmt.Sprintf("No listener was observed on port %d at scan time.", port))
	case network.Available && !network.IPv6:
		group.Evidence = append(group.Evidence,
			"IPv6 listener data was unavailable; only IPv4 bindings were observed.")
	}
	group.Evidence = append(group.Evidence,
		"A listener sample is a point-in-time observation. It proves a socket was bound, not that a process is redundant.")
	return group
}

func buildProjectGroup(key string, runtime model.Runtime, root string, procs []model.Process,
	network model.NetworkInfo) model.DuplicateGroup {

	group := model.DuplicateGroup{
		Kind:        model.DuplicateByProject,
		Key:         key,
		Runtime:     runtime,
		ProjectRoot: root,
		Confidence:  model.Inferred,
	}

	var memory uint64
	known := 0
	for _, p := range procs {
		group.PIDs = append(group.PIDs, p.PID)
		if p.MemoryKnown {
			memory += p.WorkingSetBytes
			known++
		}
		detail := fmt.Sprintf("PID %d (%s", p.PID, p.Name)
		if p.DevServerKind != "" {
			detail += ", " + p.DevServerKind
		}
		detail += ")"
		if len(p.RequestedPorts) > 0 {
			detail += fmt.Sprintf(" requests port %s", joinPorts(p.RequestedPorts))
		}
		if len(p.ListeningPorts) > 0 {
			detail += fmt.Sprintf("; listener rows associated with this PID on port %s", joinPorts(p.ListeningPorts))
			ownership := p.PortOwnership
			if ownership == "" {
				ownership = model.Unknown
			}
			if ownership != model.Confirmed {
				detail += " (ownership " + string(ownership) + ")"
			}
		} else if len(p.RequestedPorts) > 0 {
			detail += "; no listener owned by this PID was observed"
		}
		if p.StartTimeKnown && p.StartedAt != nil {
			detail += ", running since " + p.StartedAt.Local().Format("2006-01-02 15:04")
		}
		if p.MemoryKnown {
			detail += ", " + units.Bytes(p.WorkingSetBytes) + " working set"
		}
		group.Evidence = append(group.Evidence, detail)
	}
	group.MemoryBytes = memory
	group.MemoryKnown = known
	if !network.Available {
		group.Evidence = append(group.Evidence,
			"TCP listener inspection was unavailable, so this group is based on project association only.")
	} else {
		group.Evidence = append(group.Evidence,
			"A listener sample is a point-in-time observation. It proves a socket was bound, not that a process is redundant.")
	}
	return group
}

// portGroupConfidence is confirmed only when at least two participants have a
// listener whose PID and creation time were both verified. Everything else is
// inferred: a request on a command line never raises confidence by itself.
func portGroupConfidence(procs []model.Process, listeners []model.TCPListener) model.Confidence {
	participants := make(map[int32]bool, len(procs))
	for _, p := range procs {
		participants[p.PID] = true
	}
	confirmed := map[int32]bool{}
	observed := map[int32]bool{}
	uncertain := map[int32]bool{}
	for _, listener := range listeners {
		if !participants[listener.PID] {
			continue
		}
		observed[listener.PID] = true
		switch listener.Ownership {
		case model.Confirmed:
			confirmed[listener.PID] = true
		case model.Unknown:
			uncertain[listener.PID] = true
		}
	}
	if len(confirmed) >= 2 && len(uncertain) == 0 && len(confirmed) == len(observed) {
		return model.Confirmed
	}
	return model.Inferred
}

func joinPorts(ports []int) string {
	parts := make([]string, 0, len(ports))
	for _, p := range ports {
		parts = append(parts, fmt.Sprintf("%d", p))
	}
	return strings.Join(parts, ", ")
}

func listenerRefs(listeners []model.TCPListener) []model.ListenerRef {
	if len(listeners) == 0 {
		return nil
	}
	refs := make([]model.ListenerRef, 0, len(listeners))
	for _, listener := range listeners {
		refs = append(refs, model.ListenerRef{
			Family:    listener.Family,
			Address:   listener.Address,
			Port:      listener.Port,
			PID:       listener.PID,
			Ownership: listener.Ownership,
		})
	}
	sort.SliceStable(refs, func(i, j int) bool {
		if refs[i].PID != refs[j].PID {
			return refs[i].PID < refs[j].PID
		}
		if refs[i].Family != refs[j].Family {
			return refs[i].Family < refs[j].Family
		}
		return refs[i].Address < refs[j].Address
	})
	return refs
}

func listenerPIDs(listeners []model.TCPListener) []int32 {
	seen := map[int32]bool{}
	var pids []int32
	for _, listener := range listeners {
		if seen[listener.PID] {
			continue
		}
		seen[listener.PID] = true
		pids = append(pids, listener.PID)
	}
	sort.Slice(pids, func(i, j int) bool { return pids[i] < pids[j] })
	return pids
}

func listenersForPID(listeners []model.TCPListener, pid int32) []model.TCPListener {
	var out []model.TCPListener
	for _, listener := range listeners {
		if listener.PID == pid {
			out = append(out, listener)
		}
	}
	return out
}

func containsPort(ports []int, port int) bool {
	for _, candidate := range ports {
		if candidate == port {
			return true
		}
	}
	return false
}

func joinListenerRows(listeners []model.TCPListener) string {
	parts := make([]string, 0, len(listeners))
	for _, listener := range listeners {
		parts = append(parts, listenerText(listener))
	}
	return strings.Join(parts, ", ")
}

func listenerText(listener model.TCPListener) string {
	address := listener.Address
	if strings.Contains(address, ":") && !strings.HasPrefix(address, "[") {
		address = "[" + address + "]"
	}
	return fmt.Sprintf("%s:%d (%s)", address, listener.Port, listener.Family)
}

func ageString(started *time.Time, now time.Time) string {
	if started == nil {
		return ""
	}
	d := now.Sub(*started)
	if d < 0 {
		return ""
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	switch {
	case days >= 2:
		return fmt.Sprintf("%d days", days)
	case days == 1:
		return fmt.Sprintf("%d day %d h", days, hours)
	case hours >= 1:
		return fmt.Sprintf("%dh %02dm", hours, minutes)
	default:
		return fmt.Sprintf("%dm", minutes)
	}
}
