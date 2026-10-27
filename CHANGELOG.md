# Changelog

All notable changes to StackBroom are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
semantic versioning once it reaches 1.0.

## [Unreleased] - Desktop foundation

Read-only Windows desktop MVP. The CLI remains unchanged and independently
buildable; no cleanup capability was added.

### Added

- **Wails v3 desktop application** under `apps/desktop/`, with React, TypeScript
  and Tailwind CSS. The application calls the existing Go engine through four
  typed, read-only bridge methods.
- **Overview, Processes, Storage, Developer Tools and Settings** screens using
  real Windows observations. Session-only memory/CPU charts; no persisted
  monitoring history.
- **Read-only host metrics collector** (`internal/sysinfo`) for physical memory,
  interval CPU usage, Windows build and fixed-volume capacity. No mutation path.
- **Narrow, sanitized desktop DTOs**; command lines, executable paths, project
  roots and raw profile paths never cross the frontend boundary.
- **Desktop tests** for redaction, scan overlap/cancellation, 5,000-process
  inventories, unavailable Docker/WSL, frontend filtering, review-card safety,
  navigation and theme.
- **Desktop documentation** covering architecture, dependency licences, visual
  and accessibility review, validation evidence, future storage analysis and a
  design-only BYOK AI adviser.

### Security

- No kill/delete/prune/settings-write method, no shell execution, no remote
  Docker override, no production HTTP listener and no AI provider contact.
- The CLI's default privacy policy is applied at the desktop service boundary.

## [0.2.0] - 2026-10-27

Read-only TCP listener truth. No cleanup capability was added; the command
allowlist and Docker endpoint boundary are unchanged.

### Added

- **TCP listener discovery.** `GetExtendedTcpTable` is read for IPv4 and IPv6
  listener rows, including wildcard and specific addresses, port, owning PID and
  address family. No socket is created, no packet is sent and no elevation is
  requested.
- **`TCP LISTENERS` section.** The text report shows development-related sockets
  (owned by a recognised development process or holding a port one requested)
  with process, project and ownership. System services remain in `--json`.
- **Requested vs observed ports.** Processes now carry `requestedPorts` from
  command lines, `listeningPorts` from verified associations, and
  `portOwnership`.
- **PID-reuse detection.** Each process's native creation time is captured with
  `GetProcessTimes`, then re-read for listener PIDs after sampling. Ownership is
  `confirmed` only when both values match; otherwise `inferred` or `unknown`.
- **Evidence-led duplicate recommendations.** Groups distinguish two verified
  listeners, one listener plus a requester, no observed listener, a port held by
  an unrelated process, and uncertain ownership. Listener rows are included in
  `duplicateSuspects[].listeners`.
- **JSON schema v3.** `docs/schema/v3.json` describes `network`, listener rows,
  duplicate listener evidence and the renamed port fields. Version 2 remains
  published for existing consumers.

### Changed

- `processes[].ports` is renamed to `processes[].requestedPorts`. This is a
  breaking schema change with documented migration steps.
- Duplicate confidence no longer rises because two command lines mention the same
  port. It rises only for verified listener observations, and still describes the
  detected facts, not the safety of stopping anything.
- Recommendation wording no longer says "only one process can listen" or implies
  a failed bind. A missing listener is reported as "not observed", with the
  possible reasons kept open.
- Start times now prefer the native Windows creation time for every process; CIM
  remains a display fallback and never confirms listener ownership.

### Privacy

- Listener process/project names and warnings pass through the same redaction
  policy as process names, evidence and recommendations. No command lines are
  added to listener or duplicate evidence fields.

### Fixed during implementation

- The Windows TCP table can return a buffer much larger than `4 + count*rowSize`
  when the table shrinks between calls (8-248 bytes for IPv4, up to 1,744 for IPv6
  observed live). Parsing uses the declared count and derived ABI offsets, caps
  allocation and row counts, and ignores trailing allocation slack.
- Ownership warnings are summarised per scan instead of repeating once per IPv4
  and IPv6 row.

[0.2.0]: https://github.com/Caseymccallum/StackBroom/compare/v0.1.2...v0.2.0

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
