package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/Caseymccallum/StackBroom/internal/model"
)

// maxListenerRows keeps a machine with many development sockets readable. The
// full table is always present in --json.
const maxListenerRows = 20

func renderNetwork(w io.Writer, s model.Snapshot) {
	if !s.Network.Available {
		fmt.Fprintln(w, "TCP LISTENERS")
		fmt.Fprintf(w, "  Unavailable: %s\n", dashIfEmpty(s.Network.Reason))
		fmt.Fprintln(w)
		return
	}

	rows := relevantListeners(s)
	total := s.Summary.TCPListenerCount
	if total == 0 {
		fmt.Fprintln(w, "TCP LISTENERS")
		fmt.Fprintln(w, "  No TCP listeners were observed.")
		fmt.Fprintln(w)
		return
	}

	fmt.Fprintf(w, "TCP LISTENERS (%d observed, %d development-related)\n", total, len(rows))
	if len(rows) == 0 {
		fmt.Fprintln(w, "  None are owned by recognised development processes or hold a port one requested.")
		if total == 1 {
			fmt.Fprintln(w, "  1 other TCP listener was observed; use --json for the full list.")
		} else {
			fmt.Fprintf(w, "  %d other TCP listeners were observed; use --json for the full list.\n", total)
		}
		fmt.Fprintln(w)
		return
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "  PORT\tADDRESS\tPID\tPROCESS\tPROJECT\tOWNERSHIP")
	shown := len(rows)
	if shown > maxListenerRows {
		shown = maxListenerRows
	}
	for _, listener := range rows[:shown] {
		process := listener.ProcessName
		if process == "" {
			process = "unknown"
		}
		project := listener.ProjectName
		if project == "" {
			project = "-"
		}
		ownership := string(listener.Ownership)
		if ownership == "" {
			ownership = string(model.Unknown)
		}
		fmt.Fprintf(tw, "  %d\t%s\t%d\t%s\t%s\t%s\n",
			listener.Port, listenerAddress(listener), listener.PID,
			sanitize(process), sanitize(project), ownership)
	}
	_ = tw.Flush()
	if len(rows) > shown {
		fmt.Fprintf(w, "  ... %d more development-related listeners; use --json for the full list.\n", len(rows)-shown)
	}
	if !s.Network.IPv4 {
		fmt.Fprintln(w, "  IPv4 listener data was unavailable.")
	}
	if !s.Network.IPv6 {
		fmt.Fprintln(w, "  IPv6 listener data was unavailable.")
	}
	fmt.Fprintln(w, "  Listener rows are a point-in-time Windows TCP table sample, not proof that a process is redundant.")
	fmt.Fprintln(w)
}

// relevantListeners selects rows a developer can act on: sockets owned by a
// recognised development process, or sockets holding a port one of those
// processes requested. System services are left to --json.
func relevantListeners(s model.Snapshot) []model.TCPListener {
	devPIDs := map[int32]bool{}
	requestedPorts := map[int]bool{}
	for _, p := range s.Processes {
		if !p.IsDevelopment() {
			continue
		}
		devPIDs[p.PID] = true
		for _, port := range p.RequestedPorts {
			requestedPorts[port] = true
		}
	}

	var rows []model.TCPListener
	for _, listener := range s.Network.Listeners {
		if devPIDs[listener.PID] || requestedPorts[listener.Port] {
			rows = append(rows, listener)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Port != rows[j].Port {
			return rows[i].Port < rows[j].Port
		}
		if rows[i].Family != rows[j].Family {
			return rows[i].Family < rows[j].Family
		}
		if rows[i].Address != rows[j].Address {
			return rows[i].Address < rows[j].Address
		}
		return rows[i].PID < rows[j].PID
	})
	return rows
}

func listenerAddress(listener model.TCPListener) string {
	address := listener.Address
	if strings.Contains(address, ":") && !strings.HasPrefix(address, "[") {
		address = "[" + address + "]"
	}
	return sanitize(address)
}

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
	fmt.Fprintln(w, "  - Process memory is the Windows working set. Summed working sets can count shared pages more than once,")
	fmt.Fprintln(w, "    so they are not unique memory; Unknown is never estimated.")
	fmt.Fprintln(w, "  - Docker totals and reclaimable sizes are separate Docker reports. Reclaimable is Docker's estimate of what")
	fmt.Fprintln(w, "    clean-up could free, not space already freed and not a guarantee.")
	fmt.Fprintln(w, "  - WSL memory is the host-side utility VM figure; per-distribution use is not measurable from outside.")
	fmt.Fprintln(w, "  - Requested ports come from command lines; observed listeners come from the Windows TCP table. Neither proves that")
	fmt.Fprintln(w, "    a process is redundant, and a request is not proof that a socket was bound.")
	fmt.Fprintln(w, "  - Listener ownership is \"confirmed\" only when the PID's native creation time matched before and after sampling;")
	fmt.Fprintln(w, "    otherwise it is \"inferred\" or \"unknown\". System services are omitted from the text table but present in --json.")
	fmt.Fprintln(w, "  - Project associations are evidence-based: \"confirmed\" needs a manifest on disk and a referenced path inside it.")
	if s.Redaction != nil {
		fmt.Fprintf(w, "  - Output policy: mode=%s; command lines %s; paths tokenised as %%USERPROFILE%%; secrets masked.\n",
			sanitize(s.Redaction.Mode), sanitize(s.Redaction.CommandLines))
	}
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
