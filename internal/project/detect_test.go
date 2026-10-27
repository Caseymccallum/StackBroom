package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Caseymccallum/StackBroom/internal/model"
)

// testResolver disables the ignored-location filter because t.TempDir() lives
// under AppData\Local\Temp, which production code deliberately excludes.
func testResolver() Resolver {
	return Resolver{ignore: func(string) bool { return false }}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func samePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func TestResolveConfirmsScriptInsideProject(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "package.json"), `{"name":"demo"}`)
	script := filepath.Join(root, "src", "server.js")
	writeFile(t, script, "console.log('hi')")

	ref := testResolver().Resolve(`C:\Program Files\nodejs\node.exe`, `node "`+script+`"`)
	if ref == nil {
		t.Fatal("expected a project association")
	}
	if ref.Confidence != model.Confirmed {
		t.Errorf("confidence = %q, want confirmed", ref.Confidence)
	}
	if !samePath(ref.Root, root) {
		t.Errorf("root = %q, want %q", ref.Root, root)
	}
	if ref.Marker != "package.json" {
		t.Errorf("marker = %q, want package.json", ref.Marker)
	}
	if len(ref.Evidence) == 0 {
		t.Error("expected evidence to be recorded")
	}
}

func TestResolveFindsNearestProjectInMonorepo(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/root")
	web := filepath.Join(root, "apps", "web")
	writeFile(t, filepath.Join(web, "package.json"), `{"name":"web"}`)
	script := filepath.Join(web, "src", "main.ts")
	writeFile(t, script, "export {}")

	ref := testResolver().Resolve(`C:\Program Files\nodejs\node.exe`, `node "`+script+`"`)
	if ref == nil {
		t.Fatal("expected a project association")
	}
	if !samePath(ref.Root, web) {
		t.Errorf("root = %q, want nearest project %q", ref.Root, web)
	}
}

func TestResolveDegradesToInferredWhenScriptIsMissing(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "package.json"), `{"name":"demo"}`)
	missing := filepath.Join(root, "src", "removed.js")

	ref := testResolver().Resolve("", `node "`+missing+`"`)
	if ref == nil {
		t.Fatal("expected an inferred association")
	}
	if ref.Confidence != model.Inferred {
		t.Errorf("confidence = %q, want inferred", ref.Confidence)
	}
}

func TestResolveIgnoresRelativeAndUnknownPaths(t *testing.T) {
	tests := []struct {
		name string
		exe  string
		cmd  string
	}{
		{"relative script", `C:\Program Files\nodejs\node.exe`, `node server.js`},
		{"posix path", "/usr/bin/node", `node /app/server.js`},
		{"no paths at all", "node", "node -e 1"},
		{"url argument", "node", `node fetch.js https://example.com/app.js`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if ref := testResolver().Resolve(tc.exe, tc.cmd); ref != nil {
				t.Fatalf("unexpected association: %+v", ref)
			}
		})
	}
}

func TestResolveIgnoresPackageCachesAndDataArguments(t *testing.T) {
	tmp := t.TempDir()
	pkg := filepath.Join(tmp, "npm", "node_modules", "@modelcontextprotocol", "server-filesystem")
	writeFile(t, filepath.Join(pkg, "package.json"), `{"name":"server-filesystem"}`)
	script := filepath.Join(pkg, "dist", "index.js")
	writeFile(t, script, "")

	// The data directory handed to the filesystem server is not the project the
	// process belongs to.
	data := filepath.Join(tmp, "agent-workspace")
	if err := os.MkdirAll(data, 0o755); err != nil {
		t.Fatal(err)
	}

	resolver := Resolver{ignore: func(p string) bool {
		return strings.Contains(strings.ToLower(p), "node_modules")
	}}
	cmd := `node "` + script + `" "` + data + `"`
	if ref := resolver.Resolve(`C:\Program Files\nodejs\node.exe`, cmd); ref != nil {
		t.Fatalf("unexpected association with package cache: %+v", ref)
	}
}

func TestResolveTreatsFlagPathAsInferred(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "package.json"), `{"name":"demo"}`)

	ref := testResolver().Resolve(`C:\Program Files\nodejs\npm.cmd`, `npm --prefix "`+root+`" run dev`)
	if ref == nil {
		t.Fatal("expected an association from the --prefix flag")
	}
	if ref.Confidence != model.Inferred {
		t.Errorf("confidence = %q, want inferred for flag evidence", ref.Confidence)
	}
	if !strings.Contains(strings.Join(ref.Evidence, " "), "flag") {
		t.Errorf("evidence does not mention the flag: %v", ref.Evidence)
	}
}

func TestResolveIgnoresDirectoriesWithoutManifest(t *testing.T) {
	root := t.TempDir()
	script := filepath.Join(root, "server.js")
	writeFile(t, script, "")

	if ref := testResolver().Resolve("node", `node "`+script+`"`); ref != nil {
		t.Fatalf("unexpected association without a manifest: %+v", ref)
	}
}

func TestFromComposeWorkingDir(t *testing.T) {
	existing := t.TempDir()
	ref := FromComposeWorkingDir(existing)
	if ref == nil || ref.Confidence != model.Confirmed {
		t.Fatalf("existing compose dir: %+v", ref)
	}

	missing := filepath.Join(existing, "gone")
	ref = FromComposeWorkingDir(missing)
	if ref == nil || ref.Confidence != model.Inferred {
		t.Fatalf("missing compose dir: %+v", ref)
	}

	if FromComposeWorkingDir("   ") != nil {
		t.Fatal("empty compose dir should produce no association")
	}
}

func TestIsIgnoredLocation(t *testing.T) {
	ignored := []string{
		`C:\Windows\System32`,
		`C:\Program Files\nodejs`,
		`C:\Program Files (x86)\Steam`,
		`C:\Users\Casey\AppData\Roaming\npm\node_modules\pkg`,
		`C:\Users\Casey\AppData\Local\Temp\build`,
		`C:\Users\Casey\AppData\Local\Programs\Microsoft VS Code`,
		`D:\code\_npx\abc`,
		`C:\node_modules\foo`,
	}
	for _, path := range ignored {
		if !isIgnoredLocation(path) {
			t.Errorf("isIgnoredLocation(%q) = false, want true", path)
		}
	}

	allowed := []string{
		`C:\Users\Casey\Web Apps\StackBroom`,
		`C:\Users\Casey\Web Apps\project\windows`,
		`C:\Users\Casey\source\repos\App`,
		`D:\dev\index`,
	}
	for _, path := range allowed {
		if isIgnoredLocation(path) {
			t.Errorf("isIgnoredLocation(%q) = true, want false", path)
		}
	}
}
