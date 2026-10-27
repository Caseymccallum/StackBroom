// Package docker performs read-only inspection of a local Docker installation
// through the Docker CLI.
//
// StackBroom never starts Docker Desktop, never prunes, and never passes
// observed container metadata back into a command. Every call is a fixed
// argument vector with a timeout.
package docker

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/Caseymccallum/StackBroom/internal/model"
)

// unitMultipliers maps Docker's size suffixes to byte counts. Docker prints SI
// units (kB, MB, GB) for most human-readable values and IEC units (KiB, MiB,
// GiB) for a few legacy paths, so both are supported.
var unitMultipliers = map[string]float64{
	"":    1,
	"b":   1,
	"k":   1e3,
	"kb":  1e3,
	"mb":  1e6,
	"gb":  1e9,
	"tb":  1e12,
	"pb":  1e15,
	"kib": 1024,
	"mib": 1 << 20,
	"gib": 1 << 30,
	"tib": 1 << 40,
	"pib": 1 << 50,
}

// ParseSize converts a Docker human-readable size such as "1.43GB", "630.1MB",
// "4.1kB" or "0B" into bytes. The boolean is false when the text is not a size,
// so callers can show "Unknown" instead of zero.
func ParseSize(s string) (uint64, bool) {
	t := strings.TrimSpace(s)
	if i := strings.IndexByte(t, '('); i >= 0 {
		t = strings.TrimSpace(t[:i])
	}
	if t == "" {
		return 0, false
	}

	i := 0
	for i < len(t) {
		c := t[i]
		if (c >= '0' && c <= '9') || c == '.' || c == ',' {
			i++
			continue
		}
		break
	}
	numPart := strings.ReplaceAll(t[:i], ",", "")
	if numPart == "" {
		return 0, false
	}
	value, err := strconv.ParseFloat(numPart, 64)
	if err != nil || value < 0 {
		return 0, false
	}
	unit := strings.ToLower(strings.TrimSpace(t[i:]))
	mult, ok := unitMultipliers[unit]
	if !ok {
		return 0, false
	}
	return uint64(value*mult + 0.5), true
}

// ParsePercent reads a percentage from a Docker string such as
// "12.41GB (79%)" or "42.68MB". It returns false when no percentage is present.
func ParsePercent(s string) (float64, bool) {
	t := strings.TrimSpace(s)
	open := strings.LastIndexByte(t, '(')
	closeIdx := strings.LastIndexByte(t, ')')
	if open >= 0 && closeIdx > open {
		t = t[open+1 : closeIdx]
	}
	t = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(t), "%"))
	value, err := strconv.ParseFloat(t, 64)
	if err != nil || value < 0 || value > 100 {
		return 0, false
	}
	return value, true
}

// splitColumns splits fixed-width table rows using the column positions of the
// header line. Docker's table output is stable in layout, and this keeps a
// multi-word type such as "Local Volumes" in one cell.
func splitColumns(header string, row string) []string {
	type span struct{ start, end int }
	var spans []span

	inField := false
	start := 0
	for i := 0; i <= len(header); i++ {
		isSpace := i == len(header) || header[i] == ' ' || header[i] == '\t'
		if !isSpace && !inField {
			inField = true
			start = i
		}
		if isSpace && inField {
			inField = false
			spans = append(spans, span{start: start, end: i})
		}
	}
	if len(spans) == 0 {
		return nil
	}

	cells := make([]string, 0, len(spans))
	for i, sp := range spans {
		end := len(row)
		if i+1 < len(spans) {
			end = spans[i+1].start
		}
		if sp.start >= len(row) {
			cells = append(cells, "")
			continue
		}
		if end > len(row) {
			end = len(row)
		}
		cells = append(cells, strings.TrimSpace(row[sp.start:end]))
	}
	return cells
}

// rawContainer matches the JSON objects emitted by
// `docker ps -a --format '{{json .}}'`.
type rawContainer struct {
	ID        string `json:"ID"`
	Names     string `json:"Names"`
	Image     string `json:"Image"`
	State     string `json:"State"`
	Status    string `json:"Status"`
	CreatedAt string `json:"CreatedAt"`
	Ports     string `json:"Ports"`
	Labels    string `json:"Labels"`
}

// ParseContainerList reads one JSON object per line. Blank lines are ignored;
// malformed lines are skipped with a warning so one odd container cannot hide
// the rest of the list.
func ParseContainerList(out string) ([]model.DockerContainer, []string) {
	var (
		containers []model.DockerContainer
		warnings   []string
	)
	for i, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" {
			continue
		}
		var raw rawContainer
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			warnings = append(warnings, "Docker container line "+strconv.Itoa(i+1)+" was not valid JSON and was skipped")
			continue
		}
		labels := parseLabels(raw.Labels)
		c := model.DockerContainer{
			ID:                strings.TrimPrefix(raw.ID, "sha256:"),
			Name:              raw.Names,
			Image:             raw.Image,
			State:             strings.ToLower(raw.State),
			Status:            raw.Status,
			CreatedAt:         raw.CreatedAt,
			Ports:             raw.Ports,
			ComposeProject:    labels["com.docker.compose.project"],
			ComposeService:    labels["com.docker.compose.service"],
			ComposeWorkingDir: labels["com.docker.compose.project.working_dir"],
		}
		if !c.Running() {
			if text, days, ok := parseAgo(raw.Status); ok {
				c.StoppedForText = text
				c.StoppedForDays = days
			}
		}
		containers = append(containers, c)
	}
	return containers, warnings
}

// parseLabels splits Docker's comma-separated label string. Label values may
// contain an "=" (for example Windows paths), so only the first separator is
// significant.
func parseLabels(s string) map[string]string {
	labels := map[string]string{}
	for _, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			continue
		}
		labels[parts[0]] = parts[1]
	}
	return labels
}

// parseAgo extracts the age phrase from Docker's status text, e.g.
// "Exited (0) 23 hours ago" -> ("23 hours", 0, true).
//
// Docker reports coarse, rounded ages. The returned day count is an
// approximation used only to decide whether a container is worth mentioning.
func parseAgo(status string) (string, int, bool) {
	fields := strings.Fields(strings.TrimSpace(status))
	for i, f := range fields {
		if !strings.EqualFold(f, "ago") || i < 2 {
			continue
		}
		num, err := strconv.Atoi(fields[i-2])
		if err != nil || num < 0 {
			continue
		}
		unit := strings.ToLower(fields[i-1])
		hours := 0.0
		switch {
		case strings.HasPrefix(unit, "second"):
			hours = float64(num) / 3600
		case strings.HasPrefix(unit, "minute"):
			hours = float64(num) / 60
		case strings.HasPrefix(unit, "hour"):
			hours = float64(num)
		case strings.HasPrefix(unit, "day"):
			hours = float64(num) * 24
		case strings.HasPrefix(unit, "week"):
			hours = float64(num) * 24 * 7
		case strings.HasPrefix(unit, "month"):
			hours = float64(num) * 24 * 30
		case strings.HasPrefix(unit, "year"):
			hours = float64(num) * 24 * 365
		default:
			continue
		}
		return fields[i-2] + " " + fields[i-1], int(hours / 24), true
	}
	return "", 0, false
}

// rawDiskUsage matches `docker system df --format '{{json .}}'`. Docker has
// emitted both numbers and strings for the count fields across versions, so the
// raw JSON is kept and converted defensively.
type rawDiskUsage struct {
	Type        string          `json:"Type"`
	TotalCount  json.RawMessage `json:"TotalCount"`
	Active      json.RawMessage `json:"Active"`
	Size        string          `json:"Size"`
	Reclaimable string          `json:"Reclaimable"`
}

// ParseDiskUsageJSON reads the JSON-lines output of `docker system df`.
func ParseDiskUsageJSON(out string) ([]model.DockerDiskUsage, []string) {
	var (
		rows     []model.DockerDiskUsage
		warnings []string
	)
	for i, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" {
			continue
		}
		var raw rawDiskUsage
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			warnings = append(warnings, "Docker disk usage line "+strconv.Itoa(i+1)+" was not valid JSON and was skipped")
			continue
		}
		if raw.Type == "" {
			continue
		}
		row := model.DockerDiskUsage{
			Type:            raw.Type,
			Total:           rawInt(raw.TotalCount),
			Active:          rawInt(raw.Active),
			SizeText:        strings.TrimSpace(raw.Size),
			ReclaimableText: strings.TrimSpace(raw.Reclaimable),
		}
		if b, ok := ParseSize(row.SizeText); ok {
			row.Size = &b
		}
		if b, ok := ParseSize(row.ReclaimableText); ok {
			row.Reclaimable = &b
		}
		if p, ok := ParsePercent(row.ReclaimableText); ok {
			row.ReclaimablePercent = &p
		}
		rows = append(rows, row)
	}
	return rows, warnings
}

// ParseDiskUsageTable reads the classic fixed-width `docker system df` table
// that older Docker versions produce.
func ParseDiskUsageTable(out string) ([]model.DockerDiskUsage, []string) {
	var (
		rows     []model.DockerDiskUsage
		warnings []string
	)
	lines := strings.Split(out, "\n")

	headerIdx := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		upper := strings.ToUpper(line)
		if strings.Contains(upper, "TYPE") && strings.Contains(upper, "RECLAIMABLE") {
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 {
		return nil, []string{"Docker disk usage table did not contain the expected header"}
	}

	header := strings.TrimRight(lines[headerIdx], "\r")
	for _, line := range lines[headerIdx+1:] {
		line = strings.TrimRight(strings.TrimSuffix(line, "\r"), " ")
		if strings.TrimSpace(line) == "" {
			continue
		}
		cells := splitColumns(header, line)
		if len(cells) < 5 {
			warnings = append(warnings, "Docker disk usage row could not be parsed: "+strings.TrimSpace(line))
			continue
		}
		row := model.DockerDiskUsage{
			Type:            cells[0],
			Total:           atoiSafe(cells[1]),
			Active:          atoiSafe(cells[2]),
			SizeText:        cells[3],
			ReclaimableText: cells[4],
		}
		if b, ok := ParseSize(row.SizeText); ok {
			row.Size = &b
		}
		if b, ok := ParseSize(row.ReclaimableText); ok {
			row.Reclaimable = &b
		}
		if p, ok := ParsePercent(row.ReclaimableText); ok {
			row.ReclaimablePercent = &p
		}
		rows = append(rows, row)
	}
	return rows, warnings
}

// rawImage matches `docker image ls --format '{{json .}}'`.
type rawImage struct {
	ID           string `json:"ID"`
	Repository   string `json:"Repository"`
	Tag          string `json:"Tag"`
	Size         string `json:"Size"`
	CreatedSince string `json:"CreatedSince"`
}

// ParseImageList reads the JSON-lines output of `docker image ls`.
func ParseImageList(out string) ([]model.DockerImage, []string) {
	var (
		images   []model.DockerImage
		warnings []string
	)
	for i, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" {
			continue
		}
		var raw rawImage
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			warnings = append(warnings, "Docker image line "+strconv.Itoa(i+1)+" was not valid JSON and was skipped")
			continue
		}
		img := model.DockerImage{
			ID:         strings.TrimPrefix(raw.ID, "sha256:"),
			Repository: raw.Repository,
			Tag:        raw.Tag,
			SizeText:   strings.TrimSpace(raw.Size),
			CreatedFor: raw.CreatedSince,
		}
		if b, ok := ParseSize(img.SizeText); ok {
			img.Size = &b
		}
		images = append(images, img)
	}
	return images, warnings
}

// rawStats matches `docker stats --no-stream --format '{{json .}}'`.
type rawStats struct {
	Container string `json:"Container"`
	Name      string `json:"Name"`
	CPUPerc   string `json:"CPUPerc"`
	MemUsage  string `json:"MemUsage"`
	MemPerc   string `json:"MemPerc"`
}

// ContainerStats is Docker's own live measurement for one running container.
type ContainerStats struct {
	CPUPercent  string
	MemoryUsage string
	MemoryPerc  string
}

// ParseStats reads the JSON-lines output of `docker stats --no-stream`.
// The result is keyed by both the short container ID and the container name so
// callers can match either.
func ParseStats(out string) (map[string]ContainerStats, []string) {
	stats := map[string]ContainerStats{}
	var warnings []string

	for i, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" {
			continue
		}
		var raw rawStats
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			warnings = append(warnings, "Docker stats line "+strconv.Itoa(i+1)+" was not valid JSON and was skipped")
			continue
		}
		if raw.Container == "" && raw.Name == "" {
			continue
		}
		s := ContainerStats{
			CPUPercent:  strings.TrimSpace(raw.CPUPerc),
			MemoryUsage: strings.TrimSpace(raw.MemUsage),
			MemoryPerc:  strings.TrimSpace(raw.MemPerc),
		}
		if raw.Container != "" {
			stats[raw.Container] = s
			stats[ShortID(raw.Container)] = s
		}
		if raw.Name != "" {
			stats[raw.Name] = s
		}
	}
	return stats, warnings
}

// ShortID returns the conventional 12-character form of a container or image
// ID, accepting values with or without the "sha256:" prefix.
func ShortID(id string) string {
	trimmed := strings.TrimPrefix(strings.TrimSpace(id), "sha256:")
	if len(trimmed) > 12 {
		return trimmed[:12]
	}
	return trimmed
}

func rawInt(raw json.RawMessage) int {
	if len(raw) == 0 {
		return 0
	}
	t := strings.Trim(strings.TrimSpace(string(raw)), `"`)
	n, err := strconv.Atoi(t)
	if err != nil {
		return 0
	}
	return n
}

func atoiSafe(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}
