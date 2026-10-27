# StackBroom Desktop

Read-only Windows desktop MVP built with Wails v3.0.0, React 18 and TypeScript. It reuses the root StackBroom engine; the CLI remains independently buildable.

## Build

Requirements: Go 1.26+, Node/npm, Wails v3 CLI (`wails3`), WebView2 runtime.

```powershell
cd apps/desktop
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/build.ps1
```

The result is `bin/StackBroom.exe`. The script regenerates bindings, builds the frontend, runs Go tests/vet, generates Windows resources and builds the executable. The binary and temporary `.syso` are ignored by Git.

## Frontend checks

```powershell
cd apps/desktop/frontend
npm install
npm run dev
```

`npm run dev` serves the Vite UI for component work, but the desktop bridge is only available inside the Wails WebView. Use the production build for real scans and visual validation. The supported build path for this milestone is `scripts/build.ps1`.

## Tests

```powershell
cd apps/desktop
go test ./...

cd frontend
npm test
npx tsc --noEmit
```

## Screenshot gallery

With the built executable:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/gallery.ps1 -Theme dark
powershell -NoProfile -ExecutionPolicy Bypass -File scripts/gallery.ps1 -Theme light
```

Preview mode is read-only, forces a fixed theme, disables auto-refresh and uses a frameless window so exact 1920×1080 captures are possible on smaller displays. Production launches keep the native window frame. Screenshots default to `docs/desktop/screenshots/` relative to the current directory; pass `-OutDir` to override.

## Boundaries

- No kill, delete, prune, write or shell method.
- No remote-Docker override.
- No production HTTP listener.
- No AI provider or credential storage.
- Sanitized DTOs are the only data crossing into the webview.

See [docs/desktop/ARCHITECTURE.md](../../docs/desktop/ARCHITECTURE.md) and [docs/desktop/VALIDATION.md](../../docs/desktop/VALIDATION.md).
