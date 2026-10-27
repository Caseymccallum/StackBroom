// Package project associates observed processes and containers with local
// projects, using on-disk evidence rather than name guessing.
//
// The rules are deliberately strict:
//   - a project root must contain a recognised manifest (package.json, go.mod,
//     pyproject.toml, .git, ...);
//   - the process must reference a path inside that root;
//   - a confirmed association requires the referenced path to exist on disk.
//
// Anything weaker is reported as inferred, and a process with no path evidence
// gets no association at all.
package project

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Caseymccallum/StackBroom/internal/model"
	"github.com/Caseymccallum/StackBroom/internal/winproc"
)

// markerNames are checked in priority order; the first match is reported.
var markerNames = []string{
	"package.json",
	"go.mod",
	"pyproject.toml",
	"requirements.txt",
	"manage.py",
	"Cargo.toml",
	"composer.json",
	"Gemfile",
	"pom.xml",
	"build.gradle",
	"mix.exs",
	".git",
}

// DefaultMaxDepth bounds how far up the directory tree StackBroom will look for
// a project marker. It keeps a scan fast and stops a stray manifest in a user
// profile directory from claiming every process on the machine.
const DefaultMaxDepth = 12

// Resolver performs project association.
type Resolver struct {
	// MaxDepth overrides DefaultMaxDepth when positive.
	MaxDepth int

	// ignore overrides the ignored-location filter. Tests use it because
	// t.TempDir() lives under AppData\Local\Temp, which production code
	// deliberately excludes.
	ignore func(string) bool
}

func (r Resolver) isIgnored(dir string) bool {
	if r.ignore != nil {
		return r.ignore(dir)
	}
	return isIgnoredLocation(dir)
}

// Resolve returns the project a process appears to belong to, or nil when the
// evidence is not strong enough.
func (r Resolver) Resolve(exePath, commandLine string) *model.ProjectRef {
	depth := r.MaxDepth
	if depth <= 0 {
		depth = DefaultMaxDepth
	}

	candidates := candidatesFromCommandLine(commandLine)
	if exePath != "" {
		candidates = append(candidates, candidate{path: exePath, display: exePath, role: roleExe})
	}

	for _, cand := range candidates {
		dir := candidateDir(cand.path)
		if dir == "" {
			continue
		}
		if r.isIgnored(dir) {
			// Installed packages, caches and tool directories contain their own
			// manifests. Associating a process with them would be technically
			// true but misleading, so StackBroom reports no project instead.
			continue
		}
		root, marker, ok := findRoot(dir, depth)
		if !ok {
			continue
		}

		ref := &model.ProjectRef{
			Root:   root,
			Name:   projectName(root),
			Marker: marker,
		}
		ref.Evidence = append(ref.Evidence, candidateEvidence(cand))
		ref.Evidence = append(ref.Evidence, marker+" found in "+root)

		referencedExists := pathExists(cand.path)
		if referencedExists {
			ref.Evidence = append(ref.Evidence, "referenced path exists on disk")
		} else {
			ref.Evidence = append(ref.Evidence, "referenced path was not found on disk")
		}

		exeInside := exePath != "" && isInside(exePath, root)
		if exeInside {
			ref.Evidence = append(ref.Evidence, "executable is inside the project directory")
		}

		// Confirmed requires the running script or executable itself to sit
		// inside the project. A directory passed as an argument is weaker
		// evidence: an MCP filesystem server, for example, is handed a directory
		// to serve, not a project it belongs to.
		scriptInside := cand.role == roleScript && isInside(cand.path, root)
		if referencedExists && (exeInside || scriptInside) {
			ref.Confidence = model.Confirmed
		} else {
			ref.Confidence = model.Inferred
			if cand.role == roleFlag {
				ref.Evidence = append(ref.Evidence,
					"the project was identified from a flag value, not from the running script")
			}
		}
		return ref
	}
	return nil
}

// FromComposeWorkingDir builds an association from Docker's own Compose label.
// Docker is the authority on where a Compose project lives, so the association
// is confirmed when the directory still exists on disk.
func FromComposeWorkingDir(dir string) *model.ProjectRef {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil
	}
	clean := filepath.Clean(dir)
	ref := &model.ProjectRef{
		Root:   clean,
		Name:   projectName(clean),
		Marker: "com.docker.compose.project.working_dir",
	}
	if pathExists(clean) {
		ref.Confidence = model.Confirmed
		ref.Evidence = []string{"Docker Compose label reports the project directory " + clean}
	} else {
		ref.Confidence = model.Inferred
		ref.Evidence = []string{"Docker Compose label reports " + clean + ", which is not present on disk"}
	}
	return ref
}

type candidateRole int

const (
	// roleScript is the path the runtime was asked to execute, e.g. the first
	// path argument to node or python.
	roleScript candidateRole = iota
	// roleFlag is a path supplied as a flag value (--prefix, --root, ...).
	roleFlag
	// roleExe is the process's own executable.
	roleExe
)

type candidate struct {
	path    string
	display string
	role    candidateRole
}

// packageManagers are tools whose positional path arguments are usually data or
// package names, not the script being run. Their flag values are still treated
// as project pointers.
var packageManagers = map[string]bool{
	"npm": true, "npx": true, "pnpm": true, "yarn": true, "bunx": true,
	"corepack": true, "poetry": true, "pipenv": true, "docker": true,
}

// candidatesFromCommandLine extracts absolute Windows paths from a command
// line and records why each was considered.
//
// Relative paths are ignored because the process working directory is not
// observable through CIM; guessing it would create false associations.
// Positional path arguments after the first are also ignored: they are usually
// data directories (an MCP filesystem server, for example, is handed a folder
// to serve, not a folder it belongs to).
func candidatesFromCommandLine(commandLine string) []candidate {
	tokens := winproc.SplitCommandLine(commandLine)
	var out []candidate
	seen := map[string]bool{}

	add := func(raw string, role candidateRole) {
		p := normalisePath(raw)
		if p == "" {
			return
		}
		key := strings.ToLower(p)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, candidate{path: p, display: p, role: role})
	}

	viaPackageManager := runsPackageManager(tokens)
	firstPath := true
	expectFlagValue := false

	for i, raw := range tokens {
		tok := strings.TrimSpace(strings.Trim(raw, `"`))
		if tok == "" {
			continue
		}
		// Token zero is the executable when the command line starts with a
		// full path, e.g. "C:\Program Files\nodejs\node.exe".
		if i == 0 && looksLikeWindowsPath(tok) {
			continue
		}

		if looksLikeFlag(tok) {
			if idx := strings.Index(tok, "="); idx > 0 {
				if looksLikeWindowsPath(tok[idx+1:]) {
					add(tok[idx+1:], roleFlag)
				}
				expectFlagValue = false
				continue
			}
			expectFlagValue = flagTakesPathValue(tok)
			continue
		}

		if expectFlagValue {
			expectFlagValue = false
			if looksLikeWindowsPath(tok) {
				add(tok, roleFlag)
				continue
			}
		}

		if !looksLikeWindowsPath(tok) {
			continue
		}
		if firstPath {
			firstPath = false
			if viaPackageManager {
				// npx/npm/pnpm pass through to another tool; a positional path
				// here is an argument, not the running script.
				continue
			}
			add(tok, roleScript)
			continue
		}
		// Later positional paths are treated as data, not project evidence.
	}
	return out
}

// flagTakesPathValue lists flags whose following token is a project path.
func flagTakesPathValue(flag string) bool {
	switch strings.ToLower(flag) {
	case "--prefix", "-c", "--cwd", "--dir", "--root", "--project",
		"--app-dir", "--chdir", "--working-dir", "--working-directory":
		return true
	}
	return false
}

// runsPackageManager reports whether a command line invokes a package manager,
// either directly or through its CLI script (npm-cli.js, npx-cli.js, ...).
func runsPackageManager(tokens []string) bool {
	for i, raw := range tokens {
		if i >= 4 {
			break
		}
		base := baseName(raw)
		if packageManagers[base] {
			return true
		}
		if strings.Contains(base, "npm-cli") || strings.Contains(base, "npx-cli") ||
			strings.Contains(base, "pnpm-cli") || strings.Contains(base, "yarn-cli") {
			return true
		}
	}
	return false
}

func baseName(tok string) string {
	t := strings.ToLower(strings.Trim(strings.TrimSpace(tok), `"`))
	t = strings.ReplaceAll(t, "/", `\`)
	if idx := strings.LastIndex(t, `\`); idx >= 0 {
		t = t[idx+1:]
	}
	for _, suffix := range []string{".exe", ".cmd", ".bat", ".com"} {
		t = strings.TrimSuffix(t, suffix)
	}
	return t
}

func candidateEvidence(c candidate) string {
	switch c.role {
	case roleScript:
		return "process runs the script " + c.display
	case roleFlag:
		return "process passes " + c.display + " as a project path flag"
	default:
		return "executable path is " + c.display
	}
}

func looksLikeFlag(s string) bool {
	return strings.HasPrefix(s, "-")
}

func looksLikeWindowsPath(s string) bool {
	s = strings.TrimSpace(strings.Trim(s, `"`))
	if s == "" {
		return false
	}
	if strings.HasPrefix(s, `\\`) {
		return true
	}
	if len(s) >= 3 && isDriveLetter(s[0]) && s[1] == ':' && (s[2] == '\\' || s[2] == '/') {
		return true
	}
	return false
}

func isDriveLetter(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}

// normalisePath returns a cleaned absolute Windows path, or "" when the token is
// not usable as a local path.
func normalisePath(raw string) string {
	raw = strings.TrimSpace(strings.Trim(raw, `"`))
	if raw == "" || strings.HasPrefix(raw, `\\?\`) {
		return ""
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return ""
	}
	// POSIX paths such as /app/server.js refer to container or WSL filesystems.
	if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
		return ""
	}
	if !looksLikeWindowsPath(raw) {
		return ""
	}
	p := strings.ReplaceAll(raw, "/", `\`)
	return filepath.Clean(p)
}

// candidateDir decides which directory to start the marker search from.
func candidateDir(path string) string {
	if path == "" {
		return ""
	}
	if info, err := os.Stat(path); err == nil {
		if info.IsDir() {
			return path
		}
		return filepath.Dir(path)
	}
	base := filepath.Base(path)
	if strings.Contains(base, ".") {
		return filepath.Dir(path)
	}
	return path
}

// findRoot walks upwards until a directory containing a project marker is found.
// Volume roots are not accepted as project roots.
func findRoot(dir string, maxDepth int) (root string, marker string, ok bool) {
	cur := filepath.Clean(dir)
	for i := 0; i < maxDepth; i++ {
		if parent := filepath.Dir(cur); parent == cur {
			// Reached a volume root; never treat it as a project.
			return "", "", false
		}
		if m, found := markerIn(cur); found {
			return cur, m, true
		}
		cur = filepath.Dir(cur)
	}
	return "", "", false
}

func markerIn(dir string) (string, bool) {
	for _, name := range markerNames {
		if pathExists(filepath.Join(dir, name)) {
			return name, true
		}
	}
	return "", false
}

// isIgnoredLocation reports whether a directory belongs to an installed package,
// a package-manager cache or a tool installation rather than a developer's own
// project.
func isIgnoredLocation(dir string) bool {
	clean := filepath.Clean(dir)
	lower := strings.ToLower(clean)
	if !strings.HasSuffix(lower, `\`) {
		lower += `\`
	}

	for _, frag := range []string{
		`\node_modules\`,
		`\_npx\`,
		`\npm-cache\`,
		`\pnpm-store\`,
		`\yarn\cache\`,
		`\.vscode\extensions\`,
		`\appdata\local\temp\`,
		`\appdata\local\programs\`,
	} {
		if strings.Contains(lower, frag) {
			return true
		}
	}

	// Installation directories are matched relative to their volume so that a
	// user project that happens to be called "windows" is not excluded.
	volume := strings.ToLower(filepath.VolumeName(clean)) + `\`
	rest := strings.TrimPrefix(lower, volume)
	for _, prefix := range []string{`windows\`, `program files\`, `program files (x86)\`} {
		if strings.HasPrefix(rest, prefix) {
			return true
		}
	}
	return false
}

func pathExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func projectName(root string) string {
	clean := filepath.Clean(root)
	base := filepath.Base(clean)
	if base == "." || base == `\` || base == "/" {
		return clean
	}
	return base
}

// isInside reports whether child is root or below it, case-insensitively.
func isInside(child, root string) bool {
	c := strings.ToLower(filepath.Clean(child))
	r := strings.ToLower(filepath.Clean(root))
	if c == r {
		return true
	}
	return strings.HasPrefix(c, strings.TrimSuffix(r, `\`)+`\`)
}
