package winproc

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Caseymccallum/StackBroom/internal/model"
)

func classifyOf(exe, cmd string) model.Process {
	p := model.Process{Name: exe, CommandLine: cmd, CommandLineKnown: cmd != ""}
	Classify(&p)
	return p
}

type classifyCase struct {
	name         string
	exe          string
	cmd          string
	wantRuntime  model.Runtime
	wantCategory string
	wantKind     string
	wantPorts    []int
}

func TestClassify(t *testing.T) {
	tests := []classifyCase{
		{
			name:         "vite dev server",
			exe:          "node.exe",
			cmd:          `"C:\Program Files\nodejs\node.exe" C:\proj\node_modules\vite\bin\vite.js --port 5173`,
			wantRuntime:  model.RuntimeNode,
			wantCategory: model.CategoryDevServer,
			wantKind:     "vite",
			wantPorts:    []int{5173},
		},
		{
			name:         "npm run dev through npm-cli shim",
			exe:          "node.exe",
			cmd:          `"C:\Program Files\nodejs\node.exe" "C:\Program Files\nodejs\node_modules\npm\bin\npm-cli.js" run dev`,
			wantRuntime:  model.RuntimeNode,
			wantCategory: model.CategoryDevServer,
			wantKind:     "package script: dev",
		},
		{
			name:         "next dev with -p",
			exe:          "node.exe",
			cmd:          `node C:\proj\node_modules\next\dist\bin\next dev -p 3001`,
			wantRuntime:  model.RuntimeNode,
			wantCategory: model.CategoryDevServer,
			wantKind:     "next dev",
			wantPorts:    []int{3001},
		},
		{
			name:         "next start is a server but separate kind",
			exe:          "node.exe",
			cmd:          `node C:\proj\node_modules\next\dist\bin\next start`,
			wantRuntime:  model.RuntimeNode,
			wantCategory: model.CategoryDevServer,
			wantKind:     "next start",
		},
		{
			name:         "plain node script stays a runtime process",
			exe:          "node.exe",
			cmd:          `node "C:\proj\server.js"`,
			wantRuntime:  model.RuntimeNode,
			wantCategory: model.CategoryRuntime,
		},
		{
			name:         "node with vitest is tooling",
			exe:          "node.exe",
			cmd:          `node C:\proj\node_modules\vitest\vitest.mjs run`,
			wantRuntime:  model.RuntimeNode,
			wantCategory: model.CategoryDevTool,
		},
		{
			name:         "node type server is tooling",
			exe:          "node.exe",
			cmd:          `node C:\proj\node_modules\typescript\lib\tsserver.js`,
			wantRuntime:  model.RuntimeNode,
			wantCategory: model.CategoryDevTool,
		},
		{
			name:         "no command line is never a server",
			exe:          "node.exe",
			cmd:          "",
			wantRuntime:  model.RuntimeNode,
			wantCategory: model.CategoryRuntime,
		},
		{
			name:         "go run",
			exe:          "go.exe",
			cmd:          `go run .`,
			wantRuntime:  model.RuntimeGo,
			wantCategory: model.CategoryDevServer,
			wantKind:     "go run",
		},
		{
			name:         "go test is tooling",
			exe:          "go.exe",
			cmd:          `go test ./...`,
			wantRuntime:  model.RuntimeGo,
			wantCategory: model.CategoryDevTool,
		},
	}
	runClassifyCases(t, tests)
}

func TestClassifyOtherRuntimes(t *testing.T) {
	tests := []classifyCase{
		{
			name:         "uvicorn",
			exe:          "python.exe",
			cmd:          `python -m uvicorn app.main:app --port 8000`,
			wantRuntime:  model.RuntimePython,
			wantCategory: model.CategoryDevServer,
			wantKind:     "uvicorn",
			wantPorts:    []int{8000},
		},
		{
			name:         "django runserver",
			exe:          "python.exe",
			cmd:          `python manage.py runserver 8080`,
			wantRuntime:  model.RuntimePython,
			wantCategory: model.CategoryDevServer,
			wantKind:     "runserver",
		},
		{
			name:         "http.server module",
			exe:          "python.exe",
			cmd:          `python -m http.server 8000`,
			wantRuntime:  model.RuntimePython,
			wantCategory: model.CategoryDevServer,
			wantKind:     "http.server",
		},
		{
			name:         "pip install is tooling",
			exe:          "pip.exe",
			cmd:          `pip install requests`,
			wantRuntime:  model.RuntimePython,
			wantCategory: model.CategoryDevTool,
		},
		{
			name:         "dotnet watch",
			exe:          "dotnet.exe",
			cmd:          `dotnet watch run`,
			wantRuntime:  model.RuntimeDotnet,
			wantCategory: model.CategoryDevServer,
			wantKind:     "dotnet watch",
		},
		{
			name:         "msbuild is tooling",
			exe:          "MSBuild.exe",
			cmd:          `MSBuild.exe C:\proj\app.csproj`,
			wantRuntime:  model.RuntimeDotnet,
			wantCategory: model.CategoryDevTool,
		},
		{
			name:         "php artisan serve",
			exe:          "php.exe",
			cmd:          `php artisan serve --port=8081`,
			wantRuntime:  model.RuntimePHP,
			wantCategory: model.CategoryDevServer,
			wantKind:     "artisan serve",
			wantPorts:    []int{8081},
		},
		{
			name:         "cargo run",
			exe:          "cargo.exe",
			cmd:          `cargo run`,
			wantRuntime:  model.RuntimeRust,
			wantCategory: model.CategoryDevServer,
			wantKind:     "cargo run",
		},
		{
			name:         "cargo test is tooling",
			exe:          "cargo.exe",
			cmd:          `cargo test`,
			wantRuntime:  model.RuntimeRust,
			wantCategory: model.CategoryDevTool,
		},
		{
			name:         "deno task dev",
			exe:          "deno.exe",
			cmd:          `deno task dev`,
			wantRuntime:  model.RuntimeDeno,
			wantCategory: model.CategoryDevServer,
			wantKind:     "deno",
		},
		{
			name:         "bun run dev",
			exe:          "bun.exe",
			cmd:          `bun run dev`,
			wantRuntime:  model.RuntimeBun,
			wantCategory: model.CategoryDevServer,
			wantKind:     "package script: dev",
		},
		{
			name:         "unrelated executable",
			exe:          "chrome.exe",
			cmd:          `chrome.exe --type=renderer`,
			wantRuntime:  model.RuntimeUnknown,
			wantCategory: model.CategoryOther,
		},
	}
	runClassifyCases(t, tests)
}

func runClassifyCases(t *testing.T, tests []classifyCase) {
	t.Helper()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyOf(tc.exe, tc.cmd)
			if got.Runtime != tc.wantRuntime {
				t.Errorf("runtime = %q, want %q", got.Runtime, tc.wantRuntime)
			}
			if got.Category != tc.wantCategory {
				t.Errorf("category = %q, want %q", got.Category, tc.wantCategory)
			}
			if tc.wantKind != "" && !strings.Contains(got.DevServerKind, tc.wantKind) {
				t.Errorf("devServerKind = %q, want it to contain %q", got.DevServerKind, tc.wantKind)
			}
			if tc.wantKind == "" && got.DevServerKind != "" {
				t.Errorf("devServerKind = %q, want empty", got.DevServerKind)
			}
			if !reflect.DeepEqual(got.Ports, tc.wantPorts) {
				t.Errorf("ports = %v, want %v", got.Ports, tc.wantPorts)
			}
		})
	}
}

func TestExtractPortsIgnoresOutOfRangeValues(t *testing.T) {
	ports := extractPorts([]string{"node", "server.js", "--port", "99999", "-p", "3000", "--port=abc"})
	want := []int{3000}
	if !reflect.DeepEqual(ports, want) {
		t.Fatalf("extractPorts = %v, want %v", ports, want)
	}
}

func TestClassifyPackageManagerWrappers(t *testing.T) {
	tests := []classifyCase{
		{
			name:         "pnpm cjs shim",
			exe:          "node.exe",
			cmd:          `node C:\proj\node_modules\pnpm\bin\pnpm.cjs run dev -- --port 3000`,
			wantRuntime:  model.RuntimeNode,
			wantCategory: model.CategoryDevServer,
			wantKind:     "package script: dev",
			wantPorts:    []int{3000},
		},
		{
			name:         "yarn js shim",
			exe:          "node.exe",
			cmd:          `node C:\proj\node_modules\yarn\bin\yarn.js dev`,
			wantRuntime:  model.RuntimeNode,
			wantCategory: model.CategoryDevServer,
			wantKind:     "package script: dev",
		},
		{
			name:         "corepack pnpm dev",
			exe:          "node.exe",
			cmd:          `node "C:\Program Files\nodejs\node_modules\corepack\dist\corepack.js" pnpm dev`,
			wantRuntime:  model.RuntimeNode,
			wantCategory: model.CategoryDevServer,
			wantKind:     "package script: dev",
		},
		{
			name:         "volta run npm run dev",
			exe:          "volta.exe",
			cmd:          `volta run npm run dev`,
			wantRuntime:  model.RuntimeNode,
			wantCategory: model.CategoryDevServer,
			wantKind:     "package script: dev",
		},
		{
			name:         "fnm exec npm run dev",
			exe:          "fnm.exe",
			cmd:          `fnm exec --using=20 npm run dev`,
			wantRuntime:  model.RuntimeNode,
			wantCategory: model.CategoryDevServer,
			wantKind:     "package script: dev",
		},
		{
			name:         "npm exec vite",
			exe:          "node.exe",
			cmd:          `node C:\Program Files\nodejs\node_modules\npm\bin\npm-cli.js exec vite`,
			wantRuntime:  model.RuntimeNode,
			wantCategory: model.CategoryDevServer,
			wantKind:     "vite",
		},
		{
			name:         "npx yes vite",
			exe:          "node.exe",
			cmd:          `node C:\Program Files\nodejs\node_modules\npm\bin\npx-cli.js --yes vite`,
			wantRuntime:  model.RuntimeNode,
			wantCategory: model.CategoryDevServer,
			wantKind:     "vite",
		},
		{
			name:         "pnpm dlx vite",
			exe:          "pnpm.exe",
			cmd:          `pnpm dlx vite`,
			wantRuntime:  model.RuntimeNode,
			wantCategory: model.CategoryDevServer,
			wantKind:     "vite",
		},
		{
			name:         "npm run dev:server",
			exe:          "node.exe",
			cmd:          `node C:\Program Files\nodejs\node_modules\npm\bin\npm-cli.js run dev:server`,
			wantRuntime:  model.RuntimeNode,
			wantCategory: model.CategoryDevServer,
			wantKind:     "package script: dev:server",
		},
		{
			name:         "yarn start",
			exe:          "yarn.exe",
			cmd:          `yarn start`,
			wantRuntime:  model.RuntimeNode,
			wantCategory: model.CategoryDevServer,
			wantKind:     "package script: start",
		},
		{
			name:         "pnpm install is tooling",
			exe:          "pnpm.exe",
			cmd:          `pnpm install`,
			wantRuntime:  model.RuntimeNode,
			wantCategory: model.CategoryDevTool,
		},
	}
	runClassifyCases(t, tests)
}
