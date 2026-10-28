# StackBroom Desktop — Architecture

## Decision

The desktop MVP is a separate Go module at `apps/desktop/`, built with **Wails v3.0.0**, React 18 and TypeScript. The root CLI remains a dependency-free Go module and is still independently buildable.

Wails v3.0.0 is the current stable v3 release available from the Go module proxy. It is used through the official v3 API (`application.New`, `app.Window.NewWithOptions`, `application.AssetFileServerFS`) and the official `wails3` CLI. Older v2 examples were not used.

## Layout

```text
apps/desktop/
├── main.go                    Wails application and embedded frontend assets
├── service.go                 inspection bridge: quick/full scans, app info, cancel
├── service_process.go         verified process actions: preview/confirm, token checks
├── service_runtime.go         scan lock, timeout, sanitisation, warnings
├── mapping.go                 summary/process DTO mapping
├── mapping_developer.go       Docker/WSL/listener/duplicate/opportunity mapping
├── models.go                  narrow bridge DTOs
├── developer.go               developer DTOs
├── build/                     Windows manifest, version info, icon
├── frontend/
│   ├── src/                   React application and design system
│   ├── bindings/              generated Wails TypeScript bindings
│   └── dist/                  embedded production assets
└── scripts/                   build, icon, screenshot helpers
```

The desktop module uses a local `replace` to the root module:

```text
require github.com/Caseymccallum/StackBroom v0.0.0
replace github.com/Caseymccallum/StackBroom => ../..
```

This allows the desktop bridge to import `internal/scan`, `internal/privacy`, `internal/analysis`, `internal/winproc` and `internal/sysinfo` directly. No inspection algorithm is reimplemented in TypeScript. The root module gains no GUI dependency.

## Bridge contract

Two services are registered with Wails.

`DesktopService` is the inspection bridge:

| Method | Purpose |
|---|---|
| `GetAppInfo()` | version, schema, toolchain, capability limitations |
| `QuickScan()` | fast local metrics and processes, no command lines/Docker/WSL |
| `FullScan()` | command-line classification, Docker, WSL, listener truth, recommendations |
| `CancelScan()` | cancels the active scan |

`ProcessControlService` is the only mutation bridge:

| Method | Purpose |
|---|---|
| `GetProcessDetails(pid, targetToken)` | safe name/PID/protection details; no executable path |
| `PreviewProcessAction(pid, action, priority, targetToken)` | verify target and issue a 60-second operation preview |
| `ConfirmProcessAction(operationID)` | atomically consume the preview and execute/verify |

Process actions are limited to graceful close, forced termination and non-real-time priority. A backend-issued HMAC target token binds PID and native creation time; the webview never sends a timestamp. Protected, critical and StackBroom processes are refused. There is no delete, prune, startup-write, settings-write or shell method.

The bridge returns purpose-built DTOs, not `model.Snapshot`. In particular, these never cross into the webview:

- command lines;
- executable paths;
- project roots;
- arbitrary environment values;
- diagnostics mode;
- raw home-directory paths (privacy policy tokenises paths before mapping).

`privacy.Apply(..., privacy.DefaultPolicy(privacy.ModeDefault))` runs at the **service boundary** for both quick and full scans, so any future scan source is redacted even if a caller injects a different scanner.

## Scan model

- **Quick** runs the process inventory without CIM command-line enrichment, skips Docker and WSL, samples CPU for 300 ms, reads memory and fixed-volume capacity, and returns a partial snapshot.
- **Full** reuses the existing engine with command-line enrichment and Docker/WSL inspection. It is explicit and cancellable.
- Only one scan may run at a time. The service returns `ErrScanInProgress`; the UI also disables refresh.
- Scans have a 60-second service deadline and the existing engine command timeouts.
- Automatic refresh is off by default and, when enabled, performs **quick** scans only. Docker/WSL are never polled on a timer.
- Session charts keep a maximum of 40 in-memory samples. Nothing is persisted or transmitted.

## Rendering and assets

- The production executable embeds `frontend/dist` with `//go:embed all:frontend/dist`.
- Wails serves the assets to its own WebView2 instance. The packaged app does not open an HTTP listener.
- WebView2 runtime is required (present on supported Windows 10/11 installations).
- The Windows manifest declares `asInvoker` and per-monitor-v2 DPI awareness.
- DevTools are disabled in production.

## Preview mode

Screenshot automation sets `STACKBROOM_PREVIEW=1`, `STACKBROOM_PREVIEW_THEME=dark|light` and `STACKBROOM_PREVIEW_SCREEN=overview|processes|storage|developer|settings`. In that mode only:

- the window is frameless so an exact 1920×1080 client area can be captured on a smaller display;
- auto-refresh is forced off;
- the requested theme and screen are applied without writing preferences.

Preview mode does not change what data is read, does not perform actions and is not used in normal production launches.

## Build

From `apps/desktop`:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/build.ps1
```

The script generates bindings, runs `npm ci` and the Vite production build, runs Go tests/vet, generates Windows resources, builds `bin/StackBroom.exe` with `-H windowsgui`, and removes the temporary `.syso`.

The CLI remains buildable with the existing root scripts and `go build ./...`.

## Toolchain

- Root CLI: `go 1.22`.
- Desktop module: `go 1.26.0`.
- Local baseline observed: **Go 1.27.0 windows/amd64**.
- Go 1.27.3 contains a security update. No system-wide toolchain was changed in this milestone. Release builds should use Go 1.27.3+ after owner validation.
- Node observed: v26.5.0; npm 11.17.0.
