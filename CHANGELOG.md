# Changelog

All notable changes to StackBroom are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
semantic versioning once it reaches 1.0.

## [0.1.2] - 2026-10-27

Final Docker boundary hardening from independent review of v0.1.1. No new product
features; the read-only command allowlist is unchanged.

### Security

- **Deterministic context endpoint selection.** A Docker context's endpoint is now
  read from the explicit `Endpoints["docker"]` entry only. The previous code
  iterated the endpoint map and could select an unrelated local endpoint from a
  context whose Docker endpoint was remote. Missing, blank, capitalised or
  otherwise unexpected keys fail closed.
- **Named-pipe locality validation.** Only the canonical local form
  `npipe:////./pipe/<name>` is classified as local. Host-qualified UNC pipes
  (`npipe:////<host>/pipe/<name>`) address another machine over SMB and are
  rejected as unsupported, so they are never contacted even with
  `--allow-remote-docker`. Malformed, ambiguous, encoded, traversal, query-bearing
  and empty pipe forms are also invalid. Remote daemons require `tcp` or `ssh`.
- **Fail-closed remote approval.** `--allow-remote-docker` no longer rescues an
  unresolved or malformed endpoint. Approval requires a successfully resolved,
  valid endpoint; invalid endpoints are never contacted.
- `http://` and `https://` are rejected as unsupported Docker daemon transports
  rather than being offered for remote approval.
- Unix sockets must be absolute and non-empty; file descriptors must be numeric.

### Added

- Regression tests for multi-endpoint contexts in different JSON orders (50
  iterations), unrelated local endpoints beside a remote Docker endpoint, missing
  and blank Docker keys, local/remote/malformed pipe forms, and
  approval-with-invalid-endpoint cases.

### Changed

- Docker status now distinguishes "endpoint could not be resolved" from
  "remote endpoint refused" in its reason text.
- Safety and architecture documentation describe the canonical pipe form and the
  invalid-endpoint rule.

[0.1.2]: https://github.com/Caseymccallum/StackBroom/compare/v0.1.1...v0.1.2

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
