# Changelog

All notable changes to StackBroom are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
semantic versioning once it reaches 1.0.

## [Unreleased] - M3C-2 native validation gate

No live deletion or restoration was performed. The native test harness is
compile-tagged and runtime-authorised; the live test is skipped without separate
owner approval.

### Added

- Audit record for the handle-bound permanent-delete implementation.
- Test-only `m3c2_fixture` harness with a generated NTFS fixture root, exact
  identity/content verification, sentinel files and a runtime authorisation
  guard.
- A gated single-file live test that remains skipped unless all three explicit
  authorisation conditions are present.

### Corrected

- The native executor now refuses an already-elevated process before opening a
  target.
- The native worker repeats target validation, `.dmp` and age checks instead of
  relying only on engine preflight.
- A deletion is counted only after the executor marks it `Verified` and the
  original path is absent.

## [Unreleased] - M3C cleanup UX and crash-dump prototype

Execution remains disabled in ordinary builds. No user file, Recycle Bin item or
native disposition operation was executed. Tests use disposable temporary
fixtures only; native tests open a fixture handle without marking it for deletion.

### Changed

- Cleaning now uses plain language: Find files, choose files, review, clean and
  results. Developer-oriented execution/review language was removed from the
  screen.
- The unavailable Clean action explains that cleaning is not available in this
  build instead of exposing internal gates.
- Preview expiry now shows time remaining rather than “Just now”.
- Recovery and space estimates show known values as “At least …” when some files
  could not be measured.
- Skipped-file reasons and search statuses are translated to plain language.
- Disclosure summaries use aligned, rotating icons and no duplicate native
  marker.

### Added

- Dedicated `internal/cleanup/permanent` prototype for `crash-dumps-old`, using
  an exclusive handle, basic `FileDispositionInfo`, identity/path rechecks and a
  post-close absence check. It is compiled out of ordinary builds and not wired
  into the UI or service.
- Permanent-only execution permission and result/history vocabulary; recovery is
  always reported unavailable and reclaimed space remains an estimate.
- Native tests that open and verify a disposable fixture without deleting it.
- Frontend tests for expiry, plain error/reason copy, individual selection,
  skipped reasons, exact selected targets and the recycle/permanent distinction.

## [Unreleased] - M3B-4 recovery feasibility decision

Execution remains disabled. No user file, Recycle Bin item or native Shell/COM
operation was deleted, recycled, moved, renamed, purged or restored. Tests use
disposable temporary fixtures only.

### Decided

- Windows Recycle Bin execution is **NO-GO** under the current recovery-first
  policy: `IFileOperation` cannot prove before execution that permanent fallback
  is impossible, and `PostDeleteItem` can report a fully deleted item after the
  fact.
- StackBroom-managed quarantine is **CONDITIONAL** for a separately gated
  feasibility prototype only. It does not reclaim disk space by moving files.
- The production posture remains **preview-only** (`Option C`).

### Added

- Feasibility and architecture decision record with official Windows references.
- Receipt lifecycle: non-consuming `Lookup`, single-owner `Reserve`, `Release`
  and final `Commit`, with expiry, replay and concurrency tests.
- Tests for incomplete no-fallback evidence (unsupported filesystem, unknown
  policy, unknown capacity, missing provider and expiry).
- Tests for invalid, future-dated, mismatched and unavailable restoration
  evidence.

### Corrected

- Receipt inspection no longer consumes the recovery authorisation.
- Recovery evidence is exposed only after a successful receipt path; the desktop
  mapping requires `ItemRecycled`.
- Receipt-provider tests now exercise the provider refusal path rather than an
  incomplete-evidence shortcut.

## [Unreleased] - M3B-3 recovery verification and no-fallback gates

Execution remains disabled. No native Shell, Recycle Bin or restoration operation was run.

### Added

- Read-only no-fallback preflight contract and fail-closed engine gate.
- Final-file guard contract required through executor lifetime, with guaranteed close.
- Backend-only opaque, single-use, expiring receipt vault separate from action history.
- Durable receipt-provider gate that prevents a completed result without matching stored evidence.
- Aggregate `Engine.Available()` check used by preview availability.
- Deterministic tests for unavailable/unsupported preflight, unavailable guards, guard cleanup and receipt failure.

## [Unreleased] - M3B-1 recovery-first execution architecture

Execution remains disabled in ordinary builds. No deletion has been tested.

### Added

- Native-only immutable execution context with exact file identities, rule version, trusted root, recycle-only permission, expiry and single atomic consumption.
- Fail-closed cleanup target validation for identity, hard links, reparse points, placeholders, hidden/system state, size/write time and scope.
- Trusted known-folder scope policy for `%TEMP%` and `%LOCALAPPDATA%`; environment overrides outside the trusted location are refused.
- Execution state machine, per-item outcomes, recovery-evidence gate and post-operation presence checks.
- Build-tagged Windows `IFileOperation` STA worker with Recycle Bin flags, COM lifecycle cleanup and no permanent-deletion mode.
- Backend feature gate: ordinary builds have no compiled executor and an environment variable cannot enable one.
- Privacy-filtered local action history with retention, clearing and no paths or file contents.
- Disabled future confirmation UI, explicit Select all eligible action and no automatic candidate selection.
- Four-finding security review and M3B threat model.

### Corrected

- Preview candidates are no longer selected automatically.
- Preview and execution now refuse missing file identity, hidden/system files, hard links and untrusted environment scopes.

## [Unreleased] - M3A cleanup discovery and preview

Read-only cleanup discovery. No file deletion, movement, renaming, quarantine or attribute mutation exists.

### Added

- Curated, versioned cleanup rules for old per-user temporary files and old crash dumps.
- Native discovery service using the hardened storage scanner with rule eligibility, exclusions, coverage and cancellation.
- Expiring cleanup previews with exact internally retained identities, measured sizes, uncertain recovery and Recycle Bin limitations.
- Cleaning section with category evidence, analysis progress, candidate selection, exclusions and a preview-only workflow.
- Maintenance-history contract and M3B execution plan.
- Tests for rule eligibility, hard-link/age/extension exclusions, preview expiry, changed targets, discovery service and the absence of mutation methods.

### Corrected

- Treemap recursive regions now divide by their own totals; geometry tests cover proportionality, zero sizes, large Other regions, 60+ nodes and overlap/bounds.
- Duplicate coverage now reports hash attempts and failures, and incomplete duplicate analysis is explicit.
- Search uses the matched dataset's measured total for treemap percentages.
- Residual filesystem race protections and remaining TOCTOU limits are documented.

## [Unreleased] - M2 storage hardening

### Changed

- Content hashing now opens each candidate with `FILE_FLAG_OPEN_REPARSE_POINT`, verifies reparse/placeholder attributes, size, write time and file identity on the handle, and reads from that verified handle.
- Files replaced by junctions or other reparse points before hashing are refused.
- Directory enumeration uses 256-name batches instead of materialising a complete `[]DirEntry`.
- Scan coverage now distinguishes exhaustive traversal, access denial, hidden/system/reparse exclusions, depth/entry limits, cancellation and timeout.
- Duplicate coverage now reports enumerated, eligible, policy-skipped, candidate-considered, candidate-cap, verified and changed counts.
- Child navigation uses cursor pages over all indexed children; full-directory search is server-side and payloads remain bounded.
- Treemap uses an area-proportional recursive layout with a visible-node budget and an explicit Other region.
- Bridge errors carry sanitised `errorCode`/`errorMessage` values; absolute paths other than the selected root are removed.

### Design

- Added `docs/product/M3-CLEANUP-SPEC.md` for versioned rules, exact previews, Recycle Bin/`IFileOperation`, cancellation, history, restoration, privilege and race-condition requirements. No deletion code exists.

## [Unreleased] - M2 storage intelligence

Read-only storage analysis for an explicitly selected folder. No cleanup or
deletion action exists.

### Added

- `internal/storage` read-only scanner with bounded depth/entries, cancellation,
  reparse-point protection, hard-link-aware totals, long-path/Unicode support,
  placeholder awareness, large-file and folder aggregation, extension totals
  and skipped-path reporting.
- Staged duplicate verification: size, bounded prefix comparison, complete
  SHA-256, post-hash metadata recheck and hard-link separation.
- `StorageService` bridge with opaque root/scan IDs, one active scan, bounded
  results, cancellation and scan-context expiry.
- Storage explorer UI with folder selection, progress, treemap, breadcrumb,
  search, accessible child table, largest folders/files, type summary, partial
  warnings and duplicate evidence.
- Storage scanner, bridge and UI tests.

### Limits

- M2 visual screenshots were not captured because the owner prohibited visual
  validation that would bring the app window to the front.
- Traversal is sequential; hashing is bounded-concurrent.
- No deletion, cleanup, rename, move or attribute mutation was added.

## [Unreleased] - M1 process-control hardening and product architecture

Security and correctness work on the process-control capability. No install,
merge, tag or release was performed.

### Changed

- Replaced client-supplied creation timestamps with backend-issued opaque
  HMAC-SHA256 target tokens bound to PID and native creation time. Tokens expire
  after 15 minutes; the webview never sends identity evidence.
- Operations now verify identity and critical status on the same handle used for
  mutation, with `SYNCHRONIZE` for exit verification.
- `GetProcessDetails` no longer exposes executable paths across the bridge.
- Confirmation is an accessible in-app dialog with focus trap, Escape handling,
  focus restoration and explicit irreversible-action warnings.
- Process rows expose keyboard-focusable detail buttons.
- Action outcomes now report verified exit/readback, previous/new priority and
  honest `refused`/`partial`/`timeout`/`failure` states.
- Root CLI module remains dependency-free; `golang.org/x/sys` was removed from
  processcontrol in favor of `syscall`.

### Added

- Deterministic native-action seam and tests for successful preview/execute,
  duplicate confirmation, concurrent consumption, stale identity, protected
  targets, invalid priority, close timeout, terminate verification and priority
  readback.
- Target-token tamper/binding/expiry tests.
- Product architecture documentation under `docs/product/` and an M2 Storage
  Intelligence brief.

### Known limitations

- No live destructive validation has run; `notepad.exe` testing awaits owner
  authorisation.
- Desktop screenshots predate the new confirmation UI.
- Independent review is still required.

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
