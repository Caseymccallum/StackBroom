package privacy

import "github.com/Caseymccallum/StackBroom/internal/model"

// cloneSnapshot deep-copies a snapshot so redaction never mutates the data the
// collectors and analysis produced.
func cloneSnapshot(s model.Snapshot) model.Snapshot {
	out := s
	if s.Redaction != nil {
		info := *s.Redaction
		out.Redaction = &info
	}
	if s.Timings != nil {
		timings := *s.Timings
		out.Timings = &timings
	}

	out.Processes = make([]model.Process, len(s.Processes))
	for i, process := range s.Processes {
		out.Processes[i] = cloneProcess(process)
	}

	out.Network = cloneNetwork(s.Network)
	out.Docker = cloneDocker(s.Docker)
	out.WSL = cloneWSL(s.WSL)
	out.Summary = cloneSummary(s.Summary)

	out.Duplicates = make([]model.DuplicateGroup, len(s.Duplicates))
	for i, group := range s.Duplicates {
		clone := group
		clone.PIDs = append([]int32(nil), group.PIDs...)
		clone.Evidence = append([]string(nil), group.Evidence...)
		clone.Listeners = append([]model.ListenerRef(nil), group.Listeners...)
		out.Duplicates[i] = clone
	}

	out.Recommendations = make([]model.Recommendation, len(s.Recommendations))
	for i, rec := range s.Recommendations {
		clone := rec
		clone.Evidence = append([]string(nil), rec.Evidence...)
		clone.Review = append([]string(nil), rec.Review...)
		clone.Subjects = append([]model.Subject(nil), rec.Subjects...)
		out.Recommendations[i] = clone
	}

	out.Warnings = append([]string(nil), s.Warnings...)
	return out
}

func cloneProcess(p model.Process) model.Process {
	out := p
	if p.RequestedPorts != nil {
		out.RequestedPorts = append([]int(nil), p.RequestedPorts...)
	}
	if p.ListeningPorts != nil {
		out.ListeningPorts = append([]int(nil), p.ListeningPorts...)
	}
	if p.StartedAt != nil {
		started := *p.StartedAt
		out.StartedAt = &started
	}
	if p.Project != nil {
		ref := *p.Project
		ref.Evidence = append([]string(nil), p.Project.Evidence...)
		out.Project = &ref
	}
	return out
}

func cloneNetwork(n model.NetworkInfo) model.NetworkInfo {
	out := n
	out.Listeners = append([]model.TCPListener(nil), n.Listeners...)
	out.Warnings = append([]string(nil), n.Warnings...)
	return out
}

func cloneDocker(d model.DockerInfo) model.DockerInfo {
	out := d
	out.Containers = append([]model.DockerContainer(nil), d.Containers...)

	out.DiskUsage = make([]model.DockerDiskUsage, len(d.DiskUsage))
	for i, row := range d.DiskUsage {
		clone := row
		clone.Size = cloneUint64(row.Size)
		clone.Reclaimable = cloneUint64(row.Reclaimable)
		clone.ReclaimablePercent = cloneFloat64(row.ReclaimablePercent)
		out.DiskUsage[i] = clone
	}

	out.Images = make([]model.DockerImage, len(d.Images))
	for i, image := range d.Images {
		clone := image
		clone.Size = cloneUint64(image.Size)
		out.Images[i] = clone
	}

	out.Warnings = append([]string(nil), d.Warnings...)
	return out
}

func cloneWSL(w model.WSLInfo) model.WSLInfo {
	out := w
	out.Distributions = append([]model.WSLDistribution(nil), w.Distributions...)
	return out
}

func cloneSummary(s model.Summary) model.Summary {
	out := s
	out.TopProcesses = append([]model.TopProcess(nil), s.TopProcesses...)
	out.RuntimeTotals = append([]model.RuntimeTotal(nil), s.RuntimeTotals...)
	return out
}

func cloneUint64(value *uint64) *uint64 {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneFloat64(value *float64) *float64 {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}
