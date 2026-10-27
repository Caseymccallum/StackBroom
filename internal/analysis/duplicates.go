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
// purpose: either the same runtime requesting the same port, or the same
// confirmed project running twice.
//
// A group is a prompt to look, not proof of redundancy. StackBroom does not
// check which process actually owns the listening socket, and environment
// variables or config files can change the effective port.
func FindDuplicates(procs []model.Process) []model.DuplicateGroup {
	type portKey struct {
		runtime model.Runtime
		port    int
	}

	portGroups := map[portKey][]model.Process{}
	projectGroups := map[string][]model.Process{}
	projectRoots := map[string]string{}

	for _, p := range procs {
		if p.Category != model.CategoryDevServer {
			continue
		}
		for _, port := range p.Ports {
			key := portKey{runtime: p.Runtime, port: port}
			portGroups[key] = append(portGroups[key], p)
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
		out = append(out, buildGroup(
			model.DuplicateByPort,
			fmt.Sprintf("%s on port %d", key.runtime, key.port),
			key.runtime, key.port, "", group,
		))
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
		out = append(out, buildGroup(
			model.DuplicateByProject,
			fmt.Sprintf("%s in %s", string(group[0].Runtime), filepath.Base(root)),
			group[0].Runtime, 0, root, group,
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

func buildGroup(kind, key string, runtime model.Runtime, port int, projectRoot string, procs []model.Process) model.DuplicateGroup {
	group := model.DuplicateGroup{
		Kind:        kind,
		Key:         key,
		Runtime:     runtime,
		Port:        port,
		ProjectRoot: projectRoot,
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
		if len(p.Ports) > 0 {
			detail += fmt.Sprintf(" requests port %s", joinPorts(p.Ports))
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
	return group
}

func joinPorts(ports []int) string {
	parts := make([]string, 0, len(ports))
	for _, p := range ports {
		parts = append(parts, fmt.Sprintf("%d", p))
	}
	return strings.Join(parts, ", ")
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
