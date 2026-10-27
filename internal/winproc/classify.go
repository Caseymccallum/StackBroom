package winproc

import (
	"strconv"
	"strings"

	"github.com/Caseymccallum/StackBroom/internal/model"
)

// runtimeByExe maps normalised executable names to a development runtime.
//
// Only names that unambiguously belong to a development toolchain are listed.
// A process that is not in this table is reported as "unknown"/"other"; it is
// never guessed into a runtime because of its memory profile or age.
var runtimeByExe = map[string]model.Runtime{
	"node":     model.RuntimeNode,
	"npm":      model.RuntimeNode,
	"npx":      model.RuntimeNode,
	"pnpm":     model.RuntimeNode,
	"yarn":     model.RuntimeNode,
	"corepack": model.RuntimeNode,
	// Version managers run Node tooling through their own executable.
	"volta":  model.RuntimeNode,
	"fnm":    model.RuntimeNode,
	"nvm":    model.RuntimeNode,
	"nodist": model.RuntimeNode,
	"nvs":    model.RuntimeNode,

	"go":    model.RuntimeGo,
	"gofmt": model.RuntimeGo,
	"gopls": model.RuntimeGo,
	"dlv":   model.RuntimeGo,
	"air":   model.RuntimeGo,

	"python":    model.RuntimePython,
	"python3":   model.RuntimePython,
	"pythonw":   model.RuntimePython,
	"py":        model.RuntimePython,
	"pip":       model.RuntimePython,
	"pipenv":    model.RuntimePython,
	"poetry":    model.RuntimePython,
	"pytest":    model.RuntimePython,
	"uvicorn":   model.RuntimePython,
	"gunicorn":  model.RuntimePython,
	"flask":     model.RuntimePython,
	"celery":    model.RuntimePython,
	"streamlit": model.RuntimePython,
	"jupyter":   model.RuntimePython,

	"dotnet":  model.RuntimeDotnet,
	"msbuild": model.RuntimeDotnet,

	"java":   model.RuntimeJava,
	"javaw":  model.RuntimeJava,
	"mvn":    model.RuntimeJava,
	"gradle": model.RuntimeJava,

	"ruby":   model.RuntimeRuby,
	"rails":  model.RuntimeRuby,
	"puma":   model.RuntimeRuby,
	"bundle": model.RuntimeRuby,
	"rake":   model.RuntimeRuby,

	"php":      model.RuntimePHP,
	"composer": model.RuntimePHP,
	"artisan":  model.RuntimePHP,

	"deno": model.RuntimeDeno,
	"bun":  model.RuntimeBun,
	"bunx": model.RuntimeBun,

	"elixir":   model.RuntimeElixir,
	"mix":      model.RuntimeElixir,
	"beam.smp": model.RuntimeElixir,

	"cargo": model.RuntimeRust,
	"rustc": model.RuntimeRust,
}

// Classify fills Runtime, Category, DevServerKind and RequestedPorts for a
// process. Requested ports come from the command line, not the TCP table.
//
// The classifier only uses two facts: the executable name and, when available,
// the command line. It never infers a role from memory use or age.
func Classify(p *model.Process) {
	exe := normaliseExe(p.Name)
	rt, ok := runtimeByExe[exe]
	if !ok {
		p.Runtime = model.RuntimeUnknown
		p.Category = model.CategoryOther
		p.DevServerKind = ""
		p.RequestedPorts = nil
		return
	}
	p.Runtime = rt

	args := SplitCommandLine(p.CommandLine)
	names := make([]string, 0, len(args))
	for _, a := range args {
		names = append(names, argName(a))
	}

	if kind := detectDevServer(names, p.CommandLine); kind != "" {
		p.Category = model.CategoryDevServer
		p.DevServerKind = kind
		p.RequestedPorts = extractPorts(args)
		return
	}
	if isDevTool(exe, names) {
		p.Category = model.CategoryDevTool
		return
	}
	p.Category = model.CategoryRuntime
}

func normaliseExe(name string) string {
	n := NormaliseName(name)
	for _, suffix := range []string{".cmd", ".bat", ".com"} {
		n = strings.TrimSuffix(n, suffix)
	}
	return n
}

// argName normalises one command-line token for name matching.
func argName(tok string) string {
	n := normaliseExe(tokenBase(tok))
	for _, suffix := range []string{".js", ".mjs", ".cjs"} {
		n = strings.TrimSuffix(n, suffix)
	}
	return n
}

func has(names []string, want ...string) bool {
	for _, n := range names {
		for _, w := range want {
			if n == w {
				return true
			}
		}
	}
	return false
}

func hasPair(names []string, first, second string) bool {
	for i := 0; i+1 < len(names); i++ {
		if names[i] == first && names[i+1] == second {
			return true
		}
	}
	return false
}

// detectDevServer recognises command lines that start a long-running local
// development server. Anything not explicitly listed returns "".
func detectDevServer(names []string, raw string) string {
	switch {
	// Node ecosystem.
	case has(names, "vite"):
		return "vite"
	case hasPair(names, "next", "dev"):
		return "next dev"
	case hasPair(names, "next", "start"):
		return "next start"
	case hasPair(names, "nuxt", "dev"):
		return "nuxt dev"
	case hasPair(names, "ng", "serve"):
		return "Angular CLI (ng serve)"
	case hasPair(names, "react-scripts", "start"):
		return "react-scripts start"
	case has(names, "webpack-dev-server") || hasPair(names, "webpack", "serve"):
		return "webpack dev server"
	case has(names, "nodemon", "ts-node-dev"):
		return "file watcher (nodemon/ts-node-dev)"
	case has(names, "tsx") && has(names, "watch"):
		return "tsx watch"
	case has(names, "astro") && has(names, "dev"):
		return "astro dev"
	case has(names, "svelte-kit") && has(names, "dev"):
		return "svelte-kit dev"
	case has(names, "expo") && has(names, "start"):
		return "expo start"
	case packageScriptDev(names) != "":
		return packageScriptDev(names)

	// Python.
	case has(names, "uvicorn"):
		return "uvicorn"
	case has(names, "gunicorn"):
		return "gunicorn"
	case has(names, "django-admin") && has(names, "runserver"):
		return "django-admin runserver"
	case has(names, "manage.py") && has(names, "runserver"):
		return "Django manage.py runserver"
	case has(names, "flask") && has(names, "run"):
		return "Flask development server"
	case has(names, "fastapi") && has(names, "dev"):
		return "fastapi dev"
	case has(names, "http.server"):
		return "python -m http.server"
	case has(names, "streamlit") && has(names, "run"):
		return "streamlit run"

	// Go.
	case hasPair(names, "go", "run"):
		return "go run"
	case has(names, "air"):
		return "air (Go live reload)"

	// .NET.
	case hasPair(names, "dotnet", "watch"):
		return "dotnet watch"
	case hasPair(names, "dotnet", "run"):
		return "dotnet run"

	// PHP.
	case hasPair(names, "artisan", "serve"):
		return "php artisan serve"
	case hasPair(names, "php", "-s"):
		return "php built-in server"

	// Ruby.
	case hasPair(names, "rails", "server") || hasPair(names, "rails", "s"):
		return "rails server"
	case has(names, "puma"):
		return "puma"

	// Deno, Bun, Elixir, Java, Rust.
	case has(names, "deno") && (has(names, "dev") || has(names, "--watch")):
		return "deno dev/watch"
	case has(names, "bun") && has(names, "dev"):
		return "bun dev"
	case hasPair(names, "mix", "phx.server"):
		return "mix phx.server"
	case has(names, "spring-boot:run", "bootrun"):
		return "Spring Boot run"
	case hasPair(names, "cargo", "run") || hasPair(names, "cargo", "watch"):
		return "cargo run/watch"
	}

	// Some servers are only identifiable from a combined form, e.g.
	// `python -m http.server 8000` where the module token carries a suffix.
	if strings.Contains(strings.ToLower(raw), "http.server") {
		return "python -m http.server"
	}
	return ""
}

// packageScriptDev recognises `npm run dev`, `pnpm start`, `yarn dev:api`, etc.
func packageScriptDev(names []string) string {
	for i, n := range names {
		if !isPackageManagerToken(n) {
			continue
		}
		rest := names[i+1:]
		if len(rest) >= 2 && rest[0] == "run" {
			if script := rest[1]; script == "dev" || strings.HasPrefix(script, "dev:") || script == "start" {
				return "package script: " + script
			}
		}
		if len(rest) >= 1 {
			if script := rest[0]; script == "dev" || strings.HasPrefix(script, "dev:") || script == "start" {
				return "package script: " + script
			}
		}
	}
	return ""
}

// isPackageManagerToken recognises package managers directly and through the
// CLI shim scripts Windows actually runs (npm-cli.js, npx-cli.js, ...).
func isPackageManagerToken(name string) bool {
	switch strings.TrimSuffix(name, "-cli") {
	case "npm", "npx", "pnpm", "yarn", "bun", "bunx", "corepack":
		return true
	}
	return false
}

// isDevTool recognises build, test, packaging and language-tooling invocations.
// These are development processes, but explicitly not servers.
func isDevTool(exe string, names []string) bool {
	switch exe {
	case "gopls", "gofmt", "dlv", "msbuild", "rustc",
		"npm", "npx", "pnpm", "yarn", "bunx", "corepack",
		"pip", "pipenv", "poetry", "composer", "bundle",
		"mvn", "gradle", "rake":
		return true
	}

	toolTokens := []string{
		"tsc", "eslint", "prettier", "jest", "vitest", "mocha", "ava",
		"playwright", "cypress", "tsserver",
		"pytest", "tox", "ruff", "mypy", "black", "pip", "poetry",
		"phpunit", "rspec", "rubocop",
	}
	if has(names, toolTokens...) {
		return true
	}
	for _, n := range names {
		if isPackageManagerToken(n) {
			return true
		}
	}

	switch exe {
	case "go":
		return has(names, "build", "test", "vet", "mod", "get", "generate", "install", "fmt", "clean")
	case "dotnet":
		return has(names, "build", "test", "publish", "restore", "pack", "clean")
	case "cargo":
		return has(names, "build", "test", "clippy", "fmt", "check", "bench", "doc")
	case "mix":
		return has(names, "test", "compile", "deps.get", "ecto.migrate")
	}
	return false
}

// extractPorts reads explicit port flags from a development server's command
// line. Ports configured through environment variables or config files are not
// visible here and are therefore left unknown.
func extractPorts(tokens []string) []int {
	var ports []int
	add := func(raw string) {
		raw = strings.TrimSpace(strings.Trim(raw, `"`))
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 65535 {
			return
		}
		for _, existing := range ports {
			if existing == n {
				return
			}
		}
		if len(ports) < 8 {
			ports = append(ports, n)
		}
	}

	flags := map[string]bool{
		"--port": true, "-p": true, "--listen-port": true, "--http-port": true,
	}
	for i, tok := range tokens {
		lower := strings.ToLower(strings.Trim(tok, `"`))
		if flags[lower] {
			if i+1 < len(tokens) {
				add(tokens[i+1])
			}
			continue
		}
		for _, prefix := range []string{"--port=", "-p=", "--listen-port=", "--http-port="} {
			if strings.HasPrefix(lower, prefix) {
				add(lower[len(prefix):])
			}
		}
	}
	return ports
}
