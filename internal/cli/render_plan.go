package cli

import (
	"fmt"
	"io"

	"github.com/Caseymccallum/StackBroom/internal/model"
	"github.com/Caseymccallum/StackBroom/internal/units"
)

func renderPlan(w io.Writer, s model.Snapshot) {
	fmt.Fprintf(w, "%s %s - review suggestions\n", s.Tool.Name, s.Tool.Version)
	fmt.Fprintf(w, "Generated %s in %.1fs\n",
		s.GeneratedAt.Local().Format("2006-01-02 15:04:05"), float64(s.DurationMS)/1000)
	fmt.Fprintln(w, "Read-only: StackBroom inspected the system and will not act on anything below.")
	fmt.Fprintln(w)

	fmt.Fprintf(w, "Context: %d development processes (%d servers), %s reported; Docker uses %s with %s Docker-estimated reclaimable.\n",
		s.Summary.DevProcessCount,
		s.Summary.DevServerCount,
		units.Bytes(s.Summary.DevMemoryBytes),
		dockerTotalText(s),
		dockerReclaimText(s))
	fmt.Fprintln(w)

	if len(s.Recommendations) == 0 {
		fmt.Fprintln(w, "No suggestions met the evidence thresholds.")
		fmt.Fprintln(w, "That does not mean nothing is idle; it means nothing could be shown to be worth reviewing.")
		fmt.Fprintln(w)
	}

	for i, rec := range s.Recommendations {
		fmt.Fprintf(w, "%d. %s\n", i+1, sanitize(rec.Title))
		fmt.Fprintf(w, "   Risk: %s | Confidence: %s\n", rec.Risk, rec.Confidence)
		fmt.Fprintf(w, "   Detected: %s\n", sanitize(rec.Detected))
		fmt.Fprintf(w, "   Why it may be unnecessary: %s\n", sanitize(rec.WhyUnnecessary))

		fmt.Fprintln(w, "   Evidence:")
		for _, e := range rec.Evidence {
			fmt.Fprintf(w, "     - %s\n", sanitize(e))
		}

		fmt.Fprintf(w, "   Resource use: %s\n", sanitize(rec.Resource))

		fmt.Fprintln(w, "   Review before acting:")
		for _, r := range rec.Review {
			fmt.Fprintf(w, "     - %s\n", sanitize(r))
		}

		fmt.Fprintf(w, "   Risk of clean-up: %s\n", sanitize(rec.RiskDetail))
		if len(rec.Subjects) > 0 {
			fmt.Fprintf(w, "   Applies to: %s\n", sanitize(subjectText(rec.Subjects)))
		}
		fmt.Fprintln(w)
	}

	if s.Redaction != nil {
		fmt.Fprintln(w, "OUTPUT POLICY")
		fmt.Fprintf(w, "  Command lines: %s. Paths tokenised as %%USERPROFILE%%. Secrets masked. Mode: %s.\n",
			sanitize(s.Redaction.CommandLines), sanitize(s.Redaction.Mode))
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, "NO CLEANUP WAS PERFORMED")
	fmt.Fprintln(w, "  StackBroom has no implementation for stopping processes, removing containers or images,")
	fmt.Fprintln(w, "  pruning Docker, deleting volumes or shutting down WSL distributions. Any action is yours.")
	fmt.Fprintln(w)

	if len(s.Warnings) > 0 {
		fmt.Fprintln(w, "SCAN NOTES")
		for _, warning := range dedupeStrings(s.Warnings) {
			fmt.Fprintf(w, "  - %s\n", sanitize(warning))
		}
		fmt.Fprintln(w)
	}
}

func dockerTotalText(s model.Snapshot) string {
	if !s.Docker.Status.Available {
		return "unknown"
	}
	if s.Summary.DockerSizeRowsTotal == 0 {
		return "an unreported amount"
	}
	if !s.Summary.DockerTotalKnown {
		return units.Bytes(s.Summary.DockerTotalBytes) + " (partial)"
	}
	return units.Bytes(s.Summary.DockerTotalBytes)
}

func dockerReclaimText(s model.Snapshot) string {
	if !s.Docker.Status.Available || !s.Summary.DockerReclaimKnown {
		return "an unknown amount"
	}
	return units.Bytes(s.Summary.DockerReclaimableBytes)
}

func subjectText(subjects []model.Subject) string {
	parts := make([]string, 0, len(subjects))
	for _, sub := range subjects {
		text := sub.Kind + " " + sub.ID
		if sub.Detail != "" {
			text += " (" + sub.Detail + ")"
		}
		parts = append(parts, text)
	}
	return joinComma(parts)
}

func joinComma(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for _, p := range parts[1:] {
		out += ", " + p
	}
	return out
}
