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
			Title:            "Old temporary files",
			Description:      "Files in your own temporary folder that haven't changed for two weeks.",
			Owner:            "Your Windows account",
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
			RequiresClosed:   []string{"Apps that may still be using old temporary files"},
			Risks: []string{
				"An app may still need an old temporary file.",
				"Windows may not be able to put every file in the Recycle Bin.",
				"An app that is using a temporary file could fail if it disappears.",
			},
			Evidence: []string{
				"Only files in your temporary folder are included.",
				"Folders are never selected.",
				"Files must be at least 14 days old.",
				"Linked, hidden, system and cloud-only files are skipped.",
			},
			WhySafe: "The search stays inside your temporary folder, skips special files and only includes files that have been left alone for two weeks.",
		},
		{
			ID:               "crash-dumps-old",
			Version:          "1.0.0",
			Title:            "Old crash dumps",
			Description:      "Crash dump files in your own CrashDumps folder that are at least a week old.",
			Owner:            "Your Windows account",
			Category:         "Crash information",
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
				"Crash dumps can help support staff investigate a crash.",
				"Windows may not be able to put every dump in the Recycle Bin.",
			},
			Evidence: []string{
				"Only .dmp files in your CrashDumps folder are included.",
				"Only files at least 7 days old are included.",
				"Folders, linked files, hidden files and cloud-only files are skipped.",
			},
			WhySafe: "The search is limited to .dmp files in your CrashDumps folder and skips folders and special files.",
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
	trusted, err := trustedRootForToken(r.ScopeToken)
	if err != nil {
		return "", fmt.Errorf("trusted cleanup scope is unavailable")
	}
	if r.ScopeSubdir != "" {
		trusted = filepath.Join(trusted, r.ScopeSubdir)
		base = filepath.Join(base, r.ScopeSubdir)
	}
	trusted = filepath.Clean(trusted)
	absolute, err := filepath.Abs(filepath.Clean(base))
	if err != nil || !strings.EqualFold(absolute, trusted) || isDeniedScope(trusted) {
		return "", fmt.Errorf("rule scope is not the trusted Windows location")
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
