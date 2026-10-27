# Changelog

All notable changes to StackBroom are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
semantic versioning once it reaches 1.0.

## [0.1.0] - 2026-10-27

### Added

- Read-only Windows process discovery (Toolhelp32 + PSAPI) with working-set
  memory, parent process, executable path and start time.
- Command-line enrichment through a fixed PowerShell/CIM query, with a memory
  fallback for processes the direct API cannot open.
- Runtime classification for Node, Go, Python, .NET, Java, Ruby, PHP, Deno, Bun,
  Elixir and Rust, and dev-server recognition for vite, next, nuxt, Angular,
  react-scripts, webpack, nodemon, tsx, astro, svelte-kit, expo, npm/pnpm/yarn
  scripts, uvicorn, gunicorn, Django, Flask, FastAPI, `http.server`, streamlit,
  `go run`, air, dotnet watch/run, artisan serve, rails, puma, deno, bun, mix and
  cargo.
- Evidence-based project association with `confirmed` / `inferred` / none,
  excluding package caches, `node_modules`, temp and installation directories.
- Optional Docker inspection: containers, Compose metadata, stats for running
  containers, storage categories, images, JSON-lines parsing with table fallback.
- Optional WSL2 inspection: distributions, state, version, default marker and
  approximate host-side VM memory, including UTF-16LE output without a BOM.
- Resource summary, duplicate suspects (requested port and confirmed project) and
  six-part recommendations with explicit risk and confidence.
- CLI: `scan`, `plan`, `version`, `help`, `--json`, `--top`, `--timeout`,
  `--skip-docker`, `--skip-wsl`, `--no-command-lines`.
- Terminal-safe rendering: control characters in observed names are neutralised.
- Unit tests with fixtures and fake runners; opt-in read-only live tests.
- Documentation: README, architecture, safety and limitations, roadmap, and a
  local validation script.

### Security

- No cleanup capability. No shell invocation. Fixed command allowlist. Every
  external command is bounded by a timeout.

[0.1.0]: https://github.com/Caseymccallum/StackBroom/releases/tag/v0.1.0
