# Desktop Validation Evidence

## Build result

- Executable: `apps/desktop/bin/StackBroom.exe`
- Size: **11,600,896 bytes** (about 11.6 MB)
- SHA-256: `EFB67684CE92B41BFD855F93A3E0B44823BBD7AC8F5F3126343D57C7B7B7D8DE`
- Source branch: `feat/m3b-4-recovery-feasibility`; version `0.2.0`.
- Build tags: `production` (no `cleanup_execution`); `-H windowsgui`; trimmed with `-w -s`.
- Wails v3.0.0, React 18, TypeScript, Tailwind CSS v4.
- Local toolchain: Go 1.27.0 windows/amd64, Node v26.5.0, npm 11.17.0.

The binary is intentionally not committed. The build script reproduces it from the source tree.

## Test evidence

### Root CLI

- Existing Go tests, vet and build gate remain green.
- Desktop code is a nested Go module, so the root CLI is not linked to Wails or the desktop frontend.
- Schema v3 and existing CLI JSON output are unchanged.

### Desktop Go

`go test ./... -count=1` from `apps/desktop` passes:

- narrow DTO mapping and redaction of command lines, profile paths and a recognised secret assignment;
- quick/full mode dispatch;
- concurrent scan rejection and cancellation;
- 5,000-process inventory mapping;
- timeout/cancel status classification;
- controlled-build app metadata (no false read-only claim);
- opaque target tokens emitted per process row;
- preview/confirm execution once, duplicate refusal and protected/stale target refusal;
- no `exePath` crossing the bridge;
- priority readback evidence;
- curated cleanup rules, gated `ConfirmCleanup`, no `DeleteFiles`/`MoveToRecycleBin`/`CleanupFiles`, discovery-to-preview flow and preview expiry;
- cleanup rule age, extension, hard-link, trusted-scope and changed-target exclusions;
- execution context single use, expiry, rule-version refusal, recovery gate, partial/aborted results, cancellation and access-denial outcomes;
- local history retention, clearing and privacy filtering;
- no-fallback preflight refusal, final-file guard refusal/cleanup and receipt-provider failure;
- M3B-4 receipt lifecycle: non-consuming lookup, single-owner reservation, release, final commit, expiry and replay refusal;
- M3B-4 incomplete no-fallback evidence and invalid restoration evidence refusal;
- opaque receipt token, target binding and privacy;
- aggregate engine availability fails closed by default;
- backend feature gate remains disabled even when `STACKBROOM_ENABLE_CLEANUP_EXECUTION=1` in an ordinary build.

The opt-in execution sources compile with `go test -tags cleanup_execution ./internal/cleanup/execution` and `go test -tags cleanup_execution ./... -run '^$'` from `apps/desktop`. This is a compile/review check only; no COM operation was called.

M3B-4 root tests, root vet, tagged cleanup/storage/history tests and tagged vet all passed. The M3B-4 decision is recorded in `docs/product/M3B-4-RECOVERY-FEASIBILITY.md`: Recycle Bin execution is NO-GO, managed quarantine is CONDITIONAL for a separate feasibility prototype, and the production posture remains preview-only. The M3B-4 receipt lifecycle tests pass; `go test -race` was not run because the local toolchain requires cgo, which is disabled.

The root `internal/processcontrol` package also passes deterministic tests for graceful-close delivery/timeout/refusal, termination exit verification, priority readback/Real-time refusal, 16-way concurrent preview consumption, token tamper/PID binding/expiry and same-handle target requirements.

`internal/storage` passes read-only traversal tests for aggregation, large files, extension totals, duplicate candidates, hash-attempt/failure coverage, full-hash confirmation, duplicate candidate caps, hard-link accounting, hidden-file exclusion, junction non-traversal, junction-root refusal, files replaced by junctions before hashing, parent-directory junction substitution, offline-placeholder refusal, Unicode/deep names, cancellation, entry limits and changed files. The bridge tests cover authorised roots, opaque IDs, bounded cursor pages, full-directory search and matched totals, root-path privacy, sanitised error paths, concurrent-scan refusal and cancellation.

`internal/cleanup` passes curated-rule, age, extension, hard-link, preview revalidation and no-mutation tests.

Measured on a disposable 20,000-file/200-folder NTFS fixture: scan **2.259 s**, post-scan heap delta **14,199 KB**, sampled peak heap delta **21,254 KB**, total allocation delta **44,346 KB**, cancellation latency **sub-millisecond**. This is one local observation, not a general performance guarantee.

The opt-in live sysinfo test also passed on this machine: Windows 10 build 19045, a valid CPU interval sample and two fixed volumes. No live process termination, priority change or graceful close has been performed.

### Frontend

`npm test` passes **41 tests / 13 files**:

- byte/percent/port formatting and unknown values;
- process search, category filter, sort order and non-mutation;
- review card showing evidence without any action button;
- sidebar active state and Developer Tools visibility;
- listener search, ownership filter and pagination;
- App load, navigation, theme change and keyboard shortcut with a mocked bridge;
- process details confirmation, cancel, Escape, backend-error honesty, no `window.confirm`;
- keyboard-focusable process detail buttons and drawer closure when a refreshed inventory no longer contains the PID;
- Storage explorer root-selection empty state, measured evidence rendering, accessible child rows, duplicate evidence, coverage limits, cursor pagination and partial-result labelling;
- area-proportional treemap budgets, Other-region accounting, recursive region proportions, zero/disproportionate sizes and non-overlap/bounds;
- no cleanup/delete action in the Storage UI;
- Cleaning catalogue, preview-only language, empty default selection, explicit Select all eligible/Clear selection, preview request, unknown recovery and a disabled Confirm cleanup control with no Clean Now/delete action.

`npx tsc --noEmit` passes under strict TypeScript settings.

### Visual capture

The existing 28 screenshots cover the M1 UI at exact 1366x768 and 1920x1080 client sizes, dark and light. **M2 Storage and M3A Cleaning screenshots were not captured in this task** because the owner is using another fullscreen application and explicitly prohibited visual validation that would bring the StackBroom window to the front. Both UIs are covered by component tests only; a visual recapture at both sizes and themes is an outstanding gate.

## Performance observations

- Network stage: 0–1 ms in v0.2 measurements.
- Full scan: approximately **2.2–8.0 s** in desktop observations; Docker responsiveness dominates.
- Quick scan: local metrics/process sampling only; no Docker or WSL.
- CPU sample: 300 ms interval from Windows system counters.
- Desktop app process working set: **~32 MB**.
- StackBroom plus its seven WebView2 child processes at idle after scans: **~367 MB** working set, **~221 MB** private. This is the WebView2 runtime footprint, not the Go engine alone.
- Process table paginates 50 rows; search is deferred; backend mapping is tested with 5,000 processes.
- Session history is in-memory, capped at 40 samples, and is not written to disk.

## Security and privacy verification

- The frontend receives only sanitized DTOs. A JSON serialisation test asserts that command lines, home paths, project roots and a recognised secret do not cross the boundary.
- `privacy.Apply` runs at the service boundary for both quick and full scans.
- Process control is the only enabled mutation capability. It is limited to graceful close, forced termination and five non-real-time priority classes, requires a backend target token and explicit in-app confirmation, and refuses protected, critical and StackBroom processes.
- Cleanup execution is compiled out of ordinary builds (`cleanup_execution` tag absent). The gated `ConfirmCleanup` method refuses before any native path, including when its environment variable is set. No file deletion, recycle, move, rename, purge or restoration has been tested or enabled. M3B-4 adds no native mutation path.
- `GetProcessDetails` no longer returns `exePath`; paths remain internal to identity checks.
- No shell execution, arbitrary filesystem write API or unrestricted system API is exposed to the frontend.
- Docker remote endpoints are refused by the existing engine; the desktop shell has no override.
- Production assets are embedded and served to the app's own WebView2; a production launch check found **0 listening sockets** across the StackBroom process and its seven WebView2 children.
- No AI provider is contacted; no credential field or storage exists.
- No telemetry, account or cloud dependency.
- Monitoring history is not persisted by default.
- The Windows manifest runs `asInvoker`.

## Known limitations and gaps

- No live destructive process-control test has run. `notepad.exe` validation awaits explicit owner authorisation; protected/refusal behaviour is covered by deterministic fakes.
- M1 has not received independent review. A prior self-authored "security audit" was retracted and replaced by `docs/product/M1-SECURITY-AUDIT.md`.
- No live duplicate pair from two real already-running dev servers was validated in this milestone. A read-only CLI scan at 2026-10-27 20:13 found six development processes (one dev server), 38 listeners and zero development-related/associated listeners. No process was started or stopped. Fixture evidence remains, and the limitation is reported rather than hidden.
- No explicitly approved remote `tcp`/`ssh` Docker endpoint was contacted.
- TCP only; UDP is not inspected.
- No Windows ARM64 or 32-bit build was produced.
- GitHub Actions was unavailable; all checks were local.
- Go 1.27.3 was not installed and no system-wide tooling was changed. Release builds should use Go 1.27.3+ after owner validation.
- The Go race detector was not runnable locally because the installed environment has no CGO compiler; overlap/cancellation is covered by a functional concurrency test instead.
- Screen-reader/axe/Narrator validation is not yet performed.
- The storage analyzer is read-only and requires a user-selected folder. The Cleaning section is discovery/preview only in ordinary builds; its disabled confirmation control performs nothing.
- The tagged Windows `IFileOperation` worker has never been executed. Their COM vtable indices, recycle flags and resource cleanup are source-reviewed only. The native recovery verifier is unavailable, so a tagged build refuses before calling the executor.
- Recycle Bin evidence/restoration and permanent-fallback behaviour remain unverified. No live deletion is authorised.
- M3B-3 no-fallback preflight, final-file guard and durable receipt provider are unavailable; the engine therefore refuses execution even in a tagged build.
- Path-based `IFileOperation` has a residual ancestor-substitution window and is not described as race-safe.
- M2/M3A visual screenshots and screen-reader/axe/Narrator validation remain outstanding because visual validation was prohibited during this task.
- Storage traversal is sequential; only duplicate hashing uses bounded workers.
- The BYOK AI adviser is design-only.
- The app is a desktop MVP, not a tagged or published release.
