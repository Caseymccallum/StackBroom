package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/Caseymccallum/StackBroom/internal/model"
)

func renderDuplicates(w io.Writer, s model.Snapshot) {
	if len(s.Duplicates) == 0 {
		return
	}
	fmt.Fprintf(w, "POTENTIAL DUPLICATES (%d)\n", len(s.Duplicates))
	for _, g := range s.Duplicates {
		fmt.Fprintf(w, "  %s - PIDs %s (%s confidence)\n", sanitize(g.Key), pidText(g.PIDs), g.Confidence)
		for _, line := range g.Evidence {
			fmt.Fprintf(w, "    %s\n", sanitize(line))
		}
	}
	fmt.Fprintln(w)
}

func renderRecommendationSummary(w io.Writer, s model.Snapshot) {
	fmt.Fprintln(w, "REVIEW OPPORTUNITIES")
	if len(s.Recommendations) == 0 {
		fmt.Fprintln(w, "  None of the current evidence thresholds was met.")
		fmt.Fprintln(w, "  That does not mean nothing is idle; it means nothing could be shown to be worth reviewing.")
		fmt.Fprintln(w)
		return
	}
	for i, rec := range s.Recommendations {
		fmt.Fprintf(w, "  %d. %s  [risk: %s, confidence: %s]\n", i+1, sanitize(rec.Title), rec.Risk, rec.Confidence)
	}
	fmt.Fprintln(w, "  Run `stackbroom plan` for the evidence and review steps behind each suggestion.")
	fmt.Fprintln(w, "  Nothing has been stopped, deleted or pruned.")
	fmt.Fprintln(w)
}

func renderNotes(w io.Writer, s model.Snapshot) {
	fmt.Fprintln(w, "NOTES AND LIMITATIONS")
	fmt.Fprintln(w, "  - StackBroom is read-only: no process, container, image, volume or WSL distribution was changed.")
	fmt.Fprintln(w, "  - Process memory is the Windows working set. Unknown means Windows refused access; it is never estimated.")
	fmt.Fprintln(w, "  - Docker totals and reclaimable sizes are separate Docker reports, not a promise that space will be freed.")
	fmt.Fprintln(w, "  - WSL memory is the host-side utility VM figure; per-distribution use is not measurable from outside.")
	fmt.Fprintln(w, "  - Project associations are evidence-based: \"confirmed\" needs a manifest on disk and a referenced path inside it.")
	if len(s.Warnings) > 0 {
		fmt.Fprintln(w, "  - Scan notes:")
		for _, warning := range dedupeStrings(s.Warnings) {
			fmt.Fprintf(w, "      %s\n", sanitize(warning))
		}
	}
	fmt.Fprintln(w)
}

func runningContainers(d model.DockerInfo) []model.DockerContainer {
	var out []model.DockerContainer
	for _, c := range d.Containers {
		if c.Running() {
			out = append(out, c)
		}
	}
	return out
}

func stoppedContainers(d model.DockerInfo) []model.DockerContainer {
	var out []model.DockerContainer
	for _, c := range d.Containers {
		if !c.Running() {
			out = append(out, c)
		}
	}
	return out
}

func pidText(pids []int32) string {
	parts := make([]string, 0, len(pids))
	for _, pid := range pids {
		parts = append(parts, fmt.Sprintf("%d", pid))
	}
	return strings.Join(parts, ", ")
}

func orUnknownText(s string) string {
	if strings.TrimSpace(s) == "" {
		return "Unknown"
	}
	return s
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

// sanitize makes an observed string safe to print in a fixed-width table.
//
// Process names, container labels and command lines are attacker-influenced
// text. Control characters could rewrite the terminal (ANSI escapes) or corrupt
// column alignment, so they are replaced, and very long values are truncated.
// JSON output is not sanitised: encoding/json escapes control characters and
// programmatic consumers should see the real value.
func sanitize(s string) string {
	if s == "" {
		return s
	}
	const maxRunes = 200
	var b strings.Builder
	count := 0
	for _, r := range s {
		if r == '\t' || r == '\n' || r == '\r' {
			r = ' '
		}
		if r < 0x20 || (r >= 0x7f && r <= 0x9f) {
			r = '?'
		}
		b.WriteRune(r)
		count++
		if count >= maxRunes {
			b.WriteString("...")
			break
		}
	}
	return b.String()
}

func dedupeStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		key := strings.TrimSpace(s)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, key)
	}
	return out
}
