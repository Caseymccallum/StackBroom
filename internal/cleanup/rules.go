package cleanup

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Rule is a curated, versioned cleanup definition. Rules never contain an
// arbitrary path supplied by the UI.
type Rule struct {
	ID               string
	Version          string
	Title            string
	Description      string
	Owner            string
	Category         string
	ScopeToken       string
	ScopeSubdir      string
	ScopeDisplay     string
	SupportedWindows string
	MinAgeDays       int
	MaxDepth         int
	MaxEntries       int
	MaxCandidates    int
	Extensions       []string
	DeletionMode     string
	Recoverable      string
	RequiresClosed   []string
	Risks            []string
	Evidence         []string
	WhySafe          string
}

func catalog() []Rule {
	return []Rule{
		{
			ID:               "user-temp-old",
			Version:          "1.0.0",
			Title:            "Old per-user temporary files",
			Description:      "Regular files in the current user's temporary folder that have not changed for at least 14 days.",
			Owner:            "Current Windows user",
			Category:         "Temporary files",
			ScopeToken:       "%TEMP%",
			ScopeDisplay:     "%TEMP%",
			SupportedWindows: "Windows 10 1809+ / Windows 11",
			MinAgeDays:       14,
			MaxDepth:         4,
			MaxEntries:       50000,
			MaxCandidates:    500,
			DeletionMode:     "recycle",
			Recoverable:      "uncertain",
			RequiresClosed:   []string{"Applications that still own older temporary files"},
			Risks: []string{
				"Some applications keep long-lived temporary data that is still needed.",
				"Recycle Bin support for temporary files depends on volume, policy and file size.",
				"Open applications may recreate or fail on missing temporary files.",
			},
			Evidence: []string{
				"Resolved root is the current user's %TEMP% folder.",
				"Only regular files are considered; directories are never selected.",
				"Files must be at least 14 days old, single-link, non-reparse and non-placeholder.",
				"Hidden and system files are excluded.",
			},
			WhySafe: "The rule is limited to the current user's temporary directory, requires a long idle age and excludes links, reparse points, placeholders and system content.",
		},
		{
			ID:               "crash-dumps-old",
			Version:          "1.0.0",
			Title:            "Old application crash dumps",
			Description:      "Windows Error Reporting .dmp files in the current user's CrashDumps folder that are at least 7 days old.",
			Owner:            "Current Windows user",
			Category:         "Diagnostic artifacts",
			ScopeToken:       "%LOCALAPPDATA%",
			ScopeSubdir:      "CrashDumps",
			ScopeDisplay:     "%LOCALAPPDATA%\\CrashDumps",
			SupportedWindows: "Windows 10 1809+ / Windows 11",
			MinAgeDays:       7,
			MaxDepth:         1,
			MaxEntries:       10000,
			MaxCandidates:    200,
			Extensions:       []string{"dmp"},
			DeletionMode:     "recycle",
			Recoverable:      "uncertain",
			RequiresClosed:   nil,
			Risks: []string{
				"Crash dumps are diagnostic evidence and may be needed for a support investigation.",
				"Recycle Bin support is not guaranteed for every dump file.",
			},
			Evidence: []string{
				"Resolved root is %LOCALAPPDATA%\\CrashDumps.",
				"Only .dmp regular files are eligible.",
				"Files must be at least 7 days old and have a single link.",
				"Hidden and system files are excluded.",
			},
			WhySafe: "The rule targets a known diagnostic artifact location, requires an explicit .dmp extension and an age threshold, and never selects directories or user documents.",
		},
	}
}

// Catalog returns a stable copy of the curated rules.
func Catalog() []Rule {
	rules := catalog()
	sort.Slice(rules, func(i, j int) bool { return rules[i].ID < rules[j].ID })
	return rules
}

func RuleByID(id string) (Rule, bool) {
	for _, rule := range catalog() {
		if rule.ID == id {
			return rule, true
		}
	}
	return Rule{}, false
}

func expandWindowsEnv(value string) string {
	for {
		start := strings.Index(value, "%")
		if start < 0 {
			return value
		}
		end := strings.Index(value[start+1:], "%")
		if end < 0 {
			return value
		}
		end += start + 1
		name := value[start+1 : end]
		replacement, exists := os.LookupEnv(name)
		if !exists || replacement == "" {
			return value
		}
		value = value[:start] + replacement + value[end+1:]
	}
}

func (r Rule) ResolveRoot() (string, error) {
	base := expandWindowsEnv(r.ScopeToken)
	if base == "" || base == r.ScopeToken || strings.Contains(base, "%") {
		return "", fmt.Errorf("rule scope environment is unavailable")
	}
	if r.ScopeSubdir != "" {
		base = filepath.Join(base, r.ScopeSubdir)
	}
	absolute, err := filepath.Abs(filepath.Clean(base))
	if err != nil {
		return "", fmt.Errorf("rule scope is not a valid path")
	}
	return absolute, nil
}

func (r Rule) AllowsExtension(name string) bool {
	if len(r.Extensions) == 0 {
		return true
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	for _, allowed := range r.Extensions {
		if ext == strings.ToLower(strings.TrimPrefix(allowed, ".")) {
			return true
		}
	}
	return false
}
