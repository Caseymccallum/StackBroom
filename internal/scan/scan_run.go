package scan

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/Caseymccallum/StackBroom/internal/analysis"
	"github.com/Caseymccallum/StackBroom/internal/model"
	"github.com/Caseymccallum/StackBroom/internal/version"
	"github.com/Caseymccallum/StackBroom/internal/winproc"
)

// Run performs the scan. It never returns an error: individual collector
// failures become warnings so a partial picture is still delivered.
func (s *Scanner) Run(ctx context.Context) model.Snapshot {
	started := s.opts.Now()

	snapshot := model.Snapshot{
		SchemaVersion:   model.SchemaVersion,
		ReadOnly:        true,
		Tool:            model.Tool{Name: version.Name, Version: version.Version},
		GeneratedAt:     started,
		Processes:       []model.Process{},
		Network:         model.NetworkInfo{Listeners: []model.TCPListener{}},
		Duplicates:      []model.DuplicateGroup{},
		Recommendations: []model.Recommendation{},
		Warnings:        []string{},
	}

	snapshot.Host = hostInfo()

	timings := model.Timings{}
	stageStart := s.opts.Now()

	processResult, err := s.opts.Processes.Collect(ctx)
	if err != nil {
		snapshot.Warnings = append(snapshot.Warnings,
			"Process inspection failed: "+err.Error()+". No process data is shown.")
	} else {
		snapshot.Processes = processResult.Processes
		snapshot.Warnings = append(snapshot.Warnings, processResult.Warnings...)
	}
	s.fillParentNames(snapshot.Processes)
	s.associateProjects(snapshot.Processes)
	timings.ProcessesMS = s.opts.Now().Sub(stageStart).Milliseconds()

	// Listeners are sampled after the process snapshot so PID ownership can be
	// checked against a creation time captured before the socket read.
	stageStart = s.opts.Now()
	snapshot.Network = s.opts.Network.Collect(ctx)
	if snapshot.Network.Listeners == nil {
		snapshot.Network.Listeners = []model.TCPListener{}
	}
	if !snapshot.Network.Available && snapshot.Network.Reason != "" {
		snapshot.Warnings = append(snapshot.Warnings,
			"Network: "+strings.TrimSuffix(strings.TrimSpace(snapshot.Network.Reason), ".")+".")
	}
	for _, warning := range snapshot.Network.Warnings {
		snapshot.Warnings = append(snapshot.Warnings, "Network: "+warning)
	}
	s.attachNetwork(&snapshot, processResult.CreationTimes)
	timings.NetworkMS = s.opts.Now().Sub(stageStart).Milliseconds()

	stageStart = s.opts.Now()
	if s.opts.SkipDocker {
		snapshot.Warnings = append(snapshot.Warnings, "Docker inspection was skipped (--skip-docker).")
		snapshot.Docker = emptyDocker("inspection skipped")
	} else {
		snapshot.Docker = s.opts.Docker.Collect(ctx)
		if !snapshot.Docker.Status.Available {
			snapshot.Warnings = append(snapshot.Warnings, "Docker: "+snapshot.Docker.Status.Reason+".")
		}
		for _, w := range snapshot.Docker.Warnings {
			snapshot.Warnings = append(snapshot.Warnings, "Docker: "+w+".")
		}
	}
	timings.DockerMS = s.opts.Now().Sub(stageStart).Milliseconds()

	stageStart = s.opts.Now()
	if s.opts.SkipWSL {
		snapshot.Warnings = append(snapshot.Warnings, "WSL inspection was skipped (--skip-wsl).")
		snapshot.WSL = model.WSLInfo{
			Distributions: []model.WSLDistribution{},
			Reason:        "inspection skipped",
		}
	} else {
		snapshot.WSL = s.opts.WSL.Collect(ctx)
		if !snapshot.WSL.Available && snapshot.WSL.Reason != "" {
			snapshot.Warnings = append(snapshot.Warnings, "WSL: "+snapshot.WSL.Reason+".")
		}
		for _, w := range snapshot.WSL.Warnings {
			snapshot.Warnings = append(snapshot.Warnings, "WSL: "+w+".")
		}
	}
	s.attachWSLMemory(&snapshot)
	timings.WSLMS = s.opts.Now().Sub(stageStart).Milliseconds()

	stageStart = s.opts.Now()
	snapshot.Summary = analysis.Summarize(snapshot.Processes, snapshot.Docker, snapshot.WSL, s.opts.TopN)
	snapshot.Summary.TCPListenerCount = len(snapshot.Network.Listeners)
	snapshot.Duplicates = analysis.FindDuplicates(snapshot.Processes, snapshot.Network)
	snapshot.Recommendations = analysis.Recommend(
		snapshot.Processes, snapshot.Docker, snapshot.WSL, snapshot.Duplicates, started, s.opts.Recommendation)
	timings.AnalysisMS = s.opts.Now().Sub(stageStart).Milliseconds()
	snapshot.Timings = &timings

	snapshot.DurationMS = s.opts.Now().Sub(started).Milliseconds()
	if snapshot.DurationMS < 0 {
		snapshot.DurationMS = 0
	}
	return snapshot
}

func emptyDocker(reason string) model.DockerInfo {
	return model.DockerInfo{
		Status:     model.DockerStatus{Available: false, Reason: reason},
		Containers: []model.DockerContainer{},
		DiskUsage:  []model.DockerDiskUsage{},
		Images:     []model.DockerImage{},
	}
}

func hostInfo() model.Host {
	hostname, _ := os.Hostname()
	host := model.Host{OS: runtime.GOOS, Arch: runtime.GOARCH, Hostname: hostname}
	if total, available, ok := winproc.HostMemory(); ok {
		host.TotalMemoryBytes = total
		host.AvailableMemoryBytes = available
		host.MemoryKnown = true
	}
	return host
}

// associateProjects applies the project heuristics and promotes confirmed
// runtime processes to development tools. Promotion never creates a "dev
// server": server status requires an explicit server command line.
func (s *Scanner) associateProjects(procs []model.Process) {
	for i := range procs {
		p := &procs[i]
		if p.Category == model.CategoryOther || !p.CommandLineKnown {
			continue
		}
		ref := s.opts.Projects.Resolve(p.ExePath, p.CommandLine)
		if ref == nil {
			continue
		}
		p.Project = ref
		if p.Category == model.CategoryRuntime && ref.Confidence == model.Confirmed {
			p.Category = model.CategoryDevTool
		}
	}
}

func (s *Scanner) fillParentNames(procs []model.Process) {
	names := make(map[int32]string, len(procs))
	for _, p := range procs {
		names[p.PID] = p.Name
	}
	for i := range procs {
		if procs[i].ParentName == "" {
			procs[i].ParentName = names[procs[i].ParentPID]
		}
	}
}

// attachWSLMemory reads the host-side WSL2 utility VM working set from the
// process list. It is labelled approximate because vmmem accounts for the whole
// VM and its figure does not map cleanly onto individual distributions.
func (s *Scanner) attachWSLMemory(snapshot *model.Snapshot) {
	var (
		total uint64
		found bool
		name  string
	)
	for _, p := range snapshot.Processes {
		base := strings.ToLower(winproc.NormaliseName(p.Name))
		if base != "vmmem" && base != "vmmemwsl" {
			continue
		}
		name = p.Name
		if !p.MemoryKnown {
			continue
		}
		total += p.WorkingSetBytes
		found = true
	}
	if !found {
		return
	}
	snapshot.WSL.VMMemoryBytes = total
	snapshot.WSL.VMMemoryKnown = true
	snapshot.WSL.VMProcessName = name
	snapshot.WSL.VMNote = fmt.Sprintf(
		"%s is the shared WSL2 utility VM; its working set covers every running distribution and may not match the sum of guest usage.",
		name)
}
