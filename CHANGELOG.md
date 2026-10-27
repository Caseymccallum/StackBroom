# Changelog

All notable changes to StackBroom are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
semantic versioning once it reaches 1.0.

## [0.1.1] - 2026-10-27

### Security and privacy

- **Remote Docker endpoints are refused by default.** StackBroom resolves the
  daemon endpoint from `DOCKER_CONTEXT`, `DOCKER_HOST` or the active context,
  using local CLI metadata only, and contacts it only when it is a local IPC
  channel (`npipe`, `unix`, `fd`). Network endpoints, including loopback TCP,
  require the new `--allow-remote-docker` opt-in. Unknown endpoints are refused.
- **Command lines are omitted from output by default.** They remain available
  internally for classification and project association. A new `--diagnostics`
  flag re-enables them with secrets masked and home paths tokenised, and prints a
  sensitivity warning.
- The same redaction applies to project evidence, duplicate evidence,
  recommendation text, subject detail and warnings, so there is no bypass path.
- External command output is capped at 8 MiB per stream; truncation is reported
  instead of silently presenting partial data.
- The terminal renderer continues to neutralise control characters.

### Fixed

- Docker reclaimable completeness no longer follows size completeness. A report
  with a reclaimable figure but a missing size now counts as reclaim-complete,
  and row counts are tracked separately.
- Docker reclaimable and image-reclaim recommendation text now states explicitly
  that the figure is Docker's estimate, not guaranteed recovery.

### Added

- Versioned JSON schema (`docs/schema/v2.json`) with `schemaVersion: "2"`, a
  documented privacy contract, `redaction` metadata, Docker endpoint fields,
  independent reclaim row counts, per-stage `timings` and WSL warnings.
- Golden-file CLI tests (`internal/cli/testdata/*.golden.txt`).
- Large-inventory tests: 20,000 processes in analysis, 5,000 in orchestration,
  2,000 rendered rows.
- Regression tests for synthetic credentials, private paths, remote endpoints,
  Docker accounting edge cases and output truncation.
- Wrapper coverage for pnpm, Yarn, Corepack, Volta, fnm, nvm, `npx`, `dlx` and
  `exec`.
- `docs/JSON-SCHEMA.md`, `docs/PERFORMANCE.md` and
  `docs/V0.2-TCP-OWNERSHIP.md` (design only).

### Changed

- `schemaVersion` is now `"2"`. Consumers that expect
  `processes[].commandLine` must opt in with `--diagnostics` or treat the field
  as optional.

### Not changed

- No cleanup capability, no process control, no Docker mutation, no WSL state
  change. The safety contract is unchanged.

[0.1.1]: https://github.com/Caseymccallum/StackBroom/compare/v0.1.0...v0.1.1

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
