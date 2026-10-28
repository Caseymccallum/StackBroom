package main

import (
	"sort"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/model"
	"github.com/Caseymccallum/StackBroom/internal/sysinfo"
)

func mapOverview(snapshot model.Snapshot, metrics sysinfo.Metrics, mode string, captured time.Time) Overview {
	hostname := metrics.Hostname
	if hostname == "" {
		hostname = snapshot.Host.Hostname
	}
	memoryUsed := 0.0
	if metrics.MemoryKnown && metrics.TotalMemoryBytes > 0 {
		available := metrics.AvailableMemoryBytes
		if available > metrics.TotalMemoryBytes {
			available = metrics.TotalMemoryBytes
		}
		memoryUsed = float64(metrics.TotalMemoryBytes-available) / float64(metrics.TotalMemoryBytes) * 100
	}
	top := make([]ProcessRow, 0, len(snapshot.Summary.TopProcesses))
	for _, process := range snapshot.Summary.TopProcesses {
		top = append(top, mapTopProcess(process))
	}
	return Overview{
		Hostname:             hostname,
		WindowsVersion:       metrics.WindowsVersion,
		BuildNumber:          metrics.BuildNumber,
		LogicalProcessors:    metrics.LogicalProcessors,
		TotalMemoryBytes:     metrics.TotalMemoryBytes,
		AvailableMemoryBytes: metrics.AvailableMemoryBytes,
		MemoryKnown:          metrics.MemoryKnown,
		MemoryUsedPercent:    memoryUsed,
		CPUPercent:           metrics.CPUPercent,
		CPUKnown:             metrics.CPUKnown,
		ProcessCount:         snapshot.Summary.TotalProcesses,
		DevProcessCount:      snapshot.Summary.DevProcessCount,
		DevServerCount:       snapshot.Summary.DevServerCount,
		TopProcesses:         top,
		Volumes:              mapVolumes(metrics.Volumes),
		CapturedAt:           captured,
		ScanMode:             mode,
	}
}

func mapVolumes(volumes []sysinfo.Volume) []VolumeRow {
	out := make([]VolumeRow, 0, len(volumes))
	for _, volume := range volumes {
		out = append(out, VolumeRow{
			Root:        volume.Root,
			Label:       volume.Label,
			FileSystem:  volume.FileSystem,
			TotalBytes:  volume.TotalBytes,
			FreeBytes:   volume.FreeBytes,
			UsedBytes:   volume.UsedBytes,
			UsedPercent: volume.UsedPercent,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Root < out[j].Root })
	return out
}

func mapProcesses(processes []model.Process, issueToken func(model.Process) string) []ProcessRow {
	out := make([]ProcessRow, 0, len(processes))
	for _, process := range processes {
		out = append(out, mapProcess(process, issueToken))
	}
	return out
}

func mapProcess(process model.Process, issueToken func(model.Process) string) ProcessRow {
	row := ProcessRow{
		PID:            process.PID,
		ParentPID:      process.ParentPID,
		Name:           process.Name,
		Runtime:        string(process.Runtime),
		Category:       process.Category,
		DevServerKind:  process.DevServerKind,
		MemoryBytes:    process.WorkingSetBytes,
		MemoryKnown:    process.MemoryKnown,
		StartTimeKnown: process.StartTimeKnown,
		RequestedPorts: append([]int(nil), process.RequestedPorts...),
		ListeningPorts: append([]int(nil), process.ListeningPorts...),
		PortOwnership:  string(process.PortOwnership),
		TargetToken:    issueToken(process),
	}
	if process.StartedAt != nil {
		row.StartedAt = process.StartedAt.Local().Format(time.RFC3339)
	}
	if process.Project != nil {
		row.ProjectName = process.Project.Name
		row.ProjectConfidence = string(process.Project.Confidence)
	}
	return row
}

func mapTopProcess(process model.TopProcess) ProcessRow {
	return ProcessRow{
		PID:         process.PID,
		Name:        process.Name,
		Runtime:     string(process.Runtime),
		Category:    process.Category,
		MemoryBytes: process.MemoryBytes,
		MemoryKnown: process.MemoryKnown,
		ProjectName: process.ProjectName,
	}
}
