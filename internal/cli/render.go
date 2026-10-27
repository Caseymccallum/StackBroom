package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Caseymccallum/StackBroom/internal/model"
	"github.com/Caseymccallum/StackBroom/internal/units"
)

// Output is deliberately ASCII-only so it survives Windows console code pages
// and PowerShell redirection unchanged.

func renderScan(w io.Writer, s model.Snapshot) {
	fmt.Fprintf(w, "%s %s - read-only scan\n", s.Tool.Name, s.Tool.Version)
	fmt.Fprintf(w, "Generated %s in %.1fs\n",
		s.GeneratedAt.Local().Format("2006-01-02 15:04:05"), float64(s.DurationMS)/1000)

	fmt.Fprintf(w, "Host %s/%s", s.Host.OS, s.Host.Arch)
	if s.Host.Hostname != "" {
		fmt.Fprintf(w, " (%s)", s.Host.Hostname)
	}
	if s.Host.MemoryKnown {
		available := s.Host.AvailableMemoryBytes
		if available > s.Host.TotalMemoryBytes {
			available = s.Host.TotalMemoryBytes
		}
		used := s.Host.TotalMemoryBytes - available
		fmt.Fprintf(w, " - memory %s used of %s, %s available",
			units.Bytes(used), units.Bytes(s.Host.TotalMemoryBytes), units.Bytes(available))
	} else {
		fmt.Fprint(w, " - memory unknown")
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w)

	renderResourceOverview(w, s)
	renderProcesses(w, s)
	renderTopMemory(w, s)
	renderDocker(w, s)
	renderWSL(w, s)
	renderDuplicates(w, s)
	renderRecommendationSummary(w, s)
	renderNotes(w, s)
}

func renderResourceOverview(w io.Writer, s model.Snapshot) {
	fmt.Fprintln(w, "RESOURCES")
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	sum := s.Summary

	devMemory := units.Bytes(sum.DevMemoryBytes) + " reported"
	if sum.DevMemoryKnown < sum.DevProcessCount {
		devMemory = units.Bytes(sum.DevMemoryBytes) + " reported (partial)"
	}
	devLine := devMemory
	if sum.DevProcessCount == 0 {
		devLine = "none identified"
	}
	fmt.Fprintf(tw, "  Development processes\t%d (%d servers)\t%s\n",
		sum.DevProcessCount, sum.DevServerCount, devLine)

	fmt.Fprintf(tw, "  Top memory consumers\t%d shown\t%s in listed processes\n",
		len(sum.TopProcesses), units.Bytes(sum.TopMemoryBytes))

	switch {
	case !s.Docker.Status.Available:
		fmt.Fprintf(tw, "  Docker\tunavailable\t%s\n", dashIfEmpty(s.Docker.Status.Reason))
	case !sum.DockerTotalKnown && sum.DockerSizeRowsTotal > 0:
		fmt.Fprintf(tw, "  Docker\tpartial storage report\t%s of %d categories known\n",
			units.Bytes(sum.DockerTotalBytes), sum.DockerSizeRowsKnown)
	case sum.DockerSizeRowsTotal == 0:
		fmt.Fprintf(tw, "  Docker\tavailable\tstorage usage unavailable\n")
	default:
		fmt.Fprintf(tw, "  Docker\t%s on disk\t%s reclaimable\n",
			units.Bytes(sum.DockerTotalBytes), units.Bytes(sum.DockerReclaimableBytes))
	}

	switch {
	case !s.WSL.Available:
		fmt.Fprintf(tw, "  WSL2\tunavailable\t%s\n", dashIfEmpty(s.WSL.Reason))
	case s.WSL.VMMemoryKnown:
		fmt.Fprintf(tw, "  WSL2 utility VM\t%s host-side (approximate)\t%d of %d distributions running\n",
			units.Bytes(s.WSL.VMMemoryBytes), s.Summary.WSLRunningDistributions, len(s.WSL.Distributions))
	default:
		fmt.Fprintf(tw, "  WSL2 utility VM\tmemory unknown\t%d of %d distributions running\n",
			s.Summary.WSLRunningDistributions, len(s.WSL.Distributions))
	}
	_ = tw.Flush()
	fmt.Fprintln(w)
}

func renderProcesses(w io.Writer, s model.Snapshot) {
	var dev []model.Process
	for _, p := range s.Processes {
		if p.IsDevelopment() {
			dev = append(dev, p)
		}
	}
	if len(dev) == 0 {
		return
	}

	fmt.Fprintf(w, "DEVELOPMENT PROCESSES (%d)\n", len(dev))
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "  PID\tRUNTIME\tCATEGORY\tPROJECT\tMEMORY\tAGE\tDETAIL")
	for _, p := range dev {
		fmt.Fprintf(tw, "  %d\t%s\t%s\t%s\t%s\t%s\t%s\n",
			p.PID,
			p.Runtime,
			p.Category,
			projectLabel(p.Project),
			processMemory(p),
			processAge(p, s.GeneratedAt),
			processDetail(p),
		)
	}
	_ = tw.Flush()
	fmt.Fprintln(w)
}

func renderTopMemory(w io.Writer, s model.Snapshot) {
	if len(s.Summary.TopProcesses) == 0 {
		return
	}
	fmt.Fprintln(w, "TOP MEMORY CONSUMERS")
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "  PID\tPROCESS\tMEMORY\tCATEGORY\tPROJECT")
	for _, tp := range s.Summary.TopProcesses {
		project := "-"
		if tp.ProjectName != "" {
			project = tp.ProjectName
		}
		fmt.Fprintf(tw, "  %d\t%s\t%s\t%s\t%s\n", tp.PID, sanitize(tp.Name), units.Bytes(tp.MemoryBytes), tp.Category, sanitize(project))
	}
	_ = tw.Flush()
	fmt.Fprintln(w)
}

func projectLabel(ref *model.ProjectRef) string {
	if ref == nil {
		return "unknown"
	}
	return fmt.Sprintf("%s (%s)", sanitize(ref.Name), ref.Confidence)
}

func processMemory(p model.Process) string {
	if !p.MemoryKnown {
		return "Unknown"
	}
	return units.Bytes(p.WorkingSetBytes)
}

func processAge(p model.Process, now time.Time) string {
	if !p.StartTimeKnown || p.StartedAt == nil {
		return "Unknown"
	}
	if age := ageText(*p.StartedAt, now); age != "" {
		return age
	}
	return "Unknown"
}

func processDetail(p model.Process) string {
	var parts []string
	if p.DevServerKind != "" {
		parts = append(parts, p.DevServerKind)
	}
	if len(p.Ports) > 0 {
		portParts := make([]string, 0, len(p.Ports))
		for _, port := range p.Ports {
			portParts = append(portParts, fmt.Sprintf("%d", port))
		}
		parts = append(parts, "port "+strings.Join(portParts, ","))
	}
	if p.ParentName != "" && p.ParentPID > 0 {
		parts = append(parts, "parent "+sanitize(p.ParentName))
	}
	if len(parts) == 0 {
		if p.CommandLineKnown {
			return "no recognisable role"
		}
		return "command line unavailable"
	}
	return strings.Join(parts, " | ")
}

func dashIfEmpty(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func ageText(started time.Time, now time.Time) string {
	d := now.Sub(started)
	if d < 0 {
		return ""
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	switch {
	case days >= 2:
		return fmt.Sprintf("%dd", days)
	case days == 1:
		return fmt.Sprintf("%dd %dh", days, hours)
	case hours >= 1:
		return fmt.Sprintf("%dh%02dm", hours, minutes)
	default:
		return fmt.Sprintf("%dm", minutes)
	}
}
