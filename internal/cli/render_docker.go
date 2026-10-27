package cli

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/Caseymccallum/StackBroom/internal/model"
	"github.com/Caseymccallum/StackBroom/internal/units"
)

func renderDocker(w io.Writer, s model.Snapshot) {
	d := s.Docker
	fmt.Fprintln(w, "DOCKER")

	if !d.Status.Available {
		fmt.Fprintf(w, "  Status: unavailable - %s\n\n", sanitize(dashIfEmpty(d.Status.Reason)))
		return
	}

	status := "available"
	if d.Status.ServerVersion != "" {
		status += " (server " + sanitize(d.Status.ServerVersion)
		if d.Status.Context != "" {
			status += ", context " + sanitize(d.Status.Context)
		}
		status += ")"
	}
	fmt.Fprintf(w, "  Status: %s\n", status)
	fmt.Fprintf(w, "  Containers: %d running, %d stopped\n", d.RunningContainers(), d.StoppedContainers())

	if len(d.DiskUsage) > 0 {
		fmt.Fprintln(w, "  Storage (Docker's own report; total and reclaimable are different measures):")
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "    TYPE\tTOTAL\tACTIVE\tSIZE\tRECLAIMABLE")
		for _, row := range d.DiskUsage {
			fmt.Fprintf(tw, "    %s\t%d\t%d\t%s\t%s\n",
				sanitize(row.Type), row.Total, row.Active, sanitize(orUnknownText(row.SizeText)), sanitize(orUnknownText(row.ReclaimableText)))
		}
		_ = tw.Flush()
	}

	if running := runningContainers(d); len(running) > 0 {
		fmt.Fprintln(w, "  Running containers:")
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(tw, "    NAME\tIMAGE\tCPU\tMEMORY")
		for _, c := range running {
			cpu, mem := "Unknown", "Unknown"
			if c.StatsKnown {
				cpu, mem = orDash(c.CPUPercent), orDash(c.MemoryUsage)
			}
			fmt.Fprintf(tw, "    %s\t%s\t%s\t%s\n", sanitize(c.Name), sanitize(c.Image), sanitize(cpu), sanitize(mem))
		}
		_ = tw.Flush()
	}

	if stopped := stoppedContainers(d); len(stopped) > 0 {
		shown := stopped
		if len(shown) > 10 {
			shown = shown[:10]
		}
		fmt.Fprintln(w, "  Stopped containers:")
		for _, c := range shown {
			line := fmt.Sprintf("    %s - %s", sanitize(c.Name), sanitize(c.Status))
			if c.ComposeProject != "" {
				line += " (compose: " + sanitize(c.ComposeProject) + ")"
			}
			fmt.Fprintln(w, line)
		}
		if len(stopped) > len(shown) {
			fmt.Fprintf(w, "    ... and %d more\n", len(stopped)-len(shown))
		}
	}

	if len(d.Images) > 0 {
		shown := d.Images
		if len(shown) > 5 {
			shown = shown[:5]
		}
		fmt.Fprintln(w, "  Largest images:")
		for _, img := range shown {
			usage := ""
			if !img.InUse {
				usage = ", not referenced by a container"
			}
			fmt.Fprintf(w, "    %s:%s - %s%s\n", sanitize(img.Repository), sanitize(img.Tag), sanitize(orUnknownText(img.SizeText)), usage)
		}
		if d.ImagesTruncated {
			fmt.Fprintln(w, "    ... image list truncated")
		}
	}
	fmt.Fprintln(w)
}

func renderWSL(w io.Writer, s model.Snapshot) {
	info := s.WSL
	fmt.Fprintln(w, "WSL")

	if !info.Available {
		fmt.Fprintf(w, "  Status: unavailable - %s\n\n", sanitize(dashIfEmpty(info.Reason)))
		return
	}
	if len(info.Distributions) == 0 {
		fmt.Fprintln(w, "  No distributions installed.")
		fmt.Fprintln(w)
		return
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "  DISTRIBUTION\tSTATE\tVERSION\tDEFAULT")
	for _, d := range info.Distributions {
		def := ""
		if d.IsDefault {
			def = "yes"
		}
		fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n", sanitize(d.Name), sanitize(d.State), sanitize(d.Version), def)
	}
	_ = tw.Flush()

	if info.VMMemoryKnown {
		fmt.Fprintf(w, "  Host-side WSL2 VM memory: %s (approximate; %s covers all distributions)\n",
			units.Bytes(info.VMMemoryBytes), sanitize(info.VMProcessName))
	} else {
		fmt.Fprintln(w, "  Host-side WSL2 VM memory: Unknown (vmmem process not readable)")
	}
	fmt.Fprintln(w, "  Per-distribution memory: Unknown (StackBroom does not run commands inside distributions)")
	fmt.Fprintln(w)
}
