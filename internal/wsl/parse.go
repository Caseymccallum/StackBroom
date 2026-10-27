// Package wsl performs read-only inspection of Windows Subsystem for Linux.
//
// StackBroom only lists distributions. It never starts, stops, terminates or
// executes anything inside a distribution, and it does not attribute the WSL2
// utility VM's memory to individual distributions.
package wsl

import (
	"strings"

	"github.com/Caseymccallum/StackBroom/internal/model"
)

// ParseDistributionList reads decoded `wsl.exe --list --verbose` output.
//
// The columns are fixed-width, but headers and states are localised and
// distribution names may contain spaces. Rows are therefore sliced using the
// column positions of the header line, with a right-to-left fallback for
// headerless output (the last field is the version, the one before it the
// state). Messages and headers are ignored rather than reported as rows.
func ParseDistributionList(out string) ([]model.WSLDistribution, []string) {
	var (
		dists    []model.WSLDistribution
		warnings []string
	)

	lines := strings.Split(out, "\n")
	header, firstRow := findHeader(lines)

	for _, rawLine := range lines[firstRow:] {
		line := strings.TrimRight(strings.TrimSuffix(rawLine, "\r"), " \t\x00")
		if strings.TrimSpace(line) == "" {
			continue
		}
		dist, ok := parseDistributionLine(header, line)
		if !ok {
			continue
		}
		dists = append(dists, dist)
	}

	if len(dists) == 0 && strings.Contains(strings.ToLower(out), "no installed distributions") {
		warnings = append(warnings, "WSL is installed but reports no distributions")
	}
	return dists, warnings
}

// findHeader returns the header line and the index of the first data line. WSL
// always prints a header, but its words are localised, so it is recognised by
// shape: the last field is not a version number.
func findHeader(lines []string) (string, int) {
	for i, raw := range lines {
		line := strings.TrimRight(strings.TrimSuffix(raw, "\r"), " \t\x00")
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if isVersionNumber(fields[len(fields)-1]) {
			continue
		}
		return line, i + 1
	}
	return "", 0
}

func parseDistributionLine(header, line string) (model.WSLDistribution, bool) {
	trimmed := strings.TrimSpace(line)
	isDefault := strings.HasPrefix(trimmed, "*")

	if header != "" {
		cells := splitRowByHeader(header, line)
		if len(cells) >= 3 {
			version := strings.TrimSpace(cells[len(cells)-1])
			state := strings.TrimSpace(cells[len(cells)-2])
			name := strings.TrimSpace(strings.Join(cells[:len(cells)-2], " "))
			name = strings.TrimSpace(strings.TrimPrefix(name, "*"))
			if name != "" && isVersionNumber(version) {
				return model.WSLDistribution{
					Name:      name,
					State:     normaliseState(state),
					Version:   version,
					IsDefault: isDefault,
				}, true
			}
		}
	}

	// Fallback for headerless or misaligned output.
	body := strings.TrimSpace(strings.TrimPrefix(trimmed, "*"))
	fields := strings.Fields(body)
	if len(fields) < 3 {
		return model.WSLDistribution{}, false
	}
	version := fields[len(fields)-1]
	if !isVersionNumber(version) {
		return model.WSLDistribution{}, false
	}
	state := fields[len(fields)-2]
	name := strings.TrimSpace(strings.Join(fields[:len(fields)-2], " "))
	if name == "" {
		return model.WSLDistribution{}, false
	}
	return model.WSLDistribution{
		Name:      name,
		State:     normaliseState(state),
		Version:   version,
		IsDefault: isDefault,
	}, true
}

// splitRowByHeader slices a fixed-width row at the header's column starts.
func splitRowByHeader(header, row string) []string {
	starts := columnStarts(header)
	if len(starts) == 0 {
		return nil
	}
	cells := make([]string, 0, len(starts))
	for i, start := range starts {
		end := len(row)
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		if start >= len(row) {
			cells = append(cells, "")
			continue
		}
		if end > len(row) {
			end = len(row)
		}
		cells = append(cells, strings.TrimSpace(row[start:end]))
	}
	return cells
}

func columnStarts(header string) []int {
	var starts []int
	inField := false
	for i := 0; i <= len(header); i++ {
		isSpace := i == len(header) || header[i] == ' ' || header[i] == '\t'
		if !isSpace && !inField {
			inField = true
			starts = append(starts, i)
		}
		if isSpace {
			inField = false
		}
	}
	return starts
}

// isVersionNumber reports whether a token is a WSL version number (1 or 2, and
// future-proofed for 3).
func isVersionNumber(s string) bool {
	if len(s) != 1 {
		return false
	}
	return s[0] >= '1' && s[0] <= '9'
}

// normaliseState maps the English WSL states to a stable lowercase form while
// leaving unrecognised (for example localised) states untouched.
func normaliseState(s string) string {
	switch strings.ToLower(s) {
	case "running":
		return "Running"
	case "stopped":
		return "Stopped"
	case "installing":
		return "Installing"
	case "uninstalling":
		return "Uninstalling"
	case "converting":
		return "Converting"
	default:
		return s
	}
}
