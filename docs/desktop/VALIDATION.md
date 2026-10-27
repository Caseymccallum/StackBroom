# Desktop Validation Evidence

## Build result

- Executable: `apps/desktop/bin/StackBroom.exe`
- Size: **11,183,616 bytes** (about 11.2 MB)
- SHA-256: `45879BAF92DC8C476C7512169E989DA881B0EEE9AC20A2FED061EE27141BFDC0`
- Build tags: `production`; `-H windowsgui`; trimmed with `-w -s`.
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
- read-only app metadata.

The opt-in live sysinfo test also passed on this machine: Windows 10 build 19045, a valid CPU interval sample and two fixed volumes.

### Frontend

`npm test` passes **15 tests / 6 files**:

- byte/percent/port formatting and unknown values;
- process search, category filter, sort order and non-mutation;
- review card showing evidence without any action button;
- sidebar active state and Developer Tools visibility;
- listener search, ownership filter and pagination;
- App load, navigation, theme change and keyboard shortcut with a mocked bridge.

`npx tsc --noEmit` passes under strict TypeScript settings.

### Visual capture

20 screenshots at exact 1366×768 and 1920×1080 client sizes, dark and light, in `docs/desktop/screenshots/`. Captures came from the real executable, not a static mock-up.

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
- No bridge method can kill a process, remove a Docker object, prune, delete a file or change Windows settings.
- No shell execution, arbitrary filesystem write API or unrestricted system API is exposed to the frontend.
- Docker remote endpoints are refused by the existing engine; the desktop shell has no override.
- Production assets are embedded and served to the app's own WebView2; a production launch check found **0 listening sockets** across the StackBroom process and its seven WebView2 children.
- No AI provider is contacted; no credential field or storage exists.
- No telemetry, account or cloud dependency.
- Monitoring history is not persisted by default.
- The Windows manifest runs `asInvoker`.

## Known limitations and gaps

- No live duplicate pair from two real already-running dev servers was validated in this milestone. A read-only CLI scan at 2026-10-27 20:13 found six development processes (one dev server), 38 listeners and zero development-related/associated listeners. No process was started or stopped. Fixture evidence remains, and the limitation is reported rather than hidden.
- No explicitly approved remote `tcp`/`ssh` Docker endpoint was contacted.
- TCP only; UDP is not inspected.
- No Windows ARM64 or 32-bit build was produced.
- GitHub Actions was unavailable; all checks were local.
- Go 1.27.3 was not installed and no system-wide tooling was changed. Release builds should use Go 1.27.3+ after owner validation.
- The Go race detector was not runnable locally because the installed environment has no CGO compiler; overlap/cancellation is covered by a functional concurrency test instead.
- Screen-reader/axe/Narrator validation is not yet performed.
- The full-folder storage analyzer and duplicate-file engine are design-only.
- The BYOK AI adviser is design-only.
- The app is a desktop MVP, not a tagged or published release.
