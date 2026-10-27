# Desktop Dependencies and Licences

The desktop module is isolated from the root CLI. The root CLI has no third-party Go dependencies. The desktop application adds the following direct dependencies.

## Go

| Dependency | Version | Licence |
|---|---|---|
| Wails v3 | 3.0.0 | MIT |
| golang.org/x/sys (transitive) | 0.48.0 | BSD-3-Clause |
| github.com/go-ole/go-ole (transitive) | 1.3.0 | MIT |
| github.com/godbus/dbus/v5 (transitive) | 5.2.2 | BSD-2-Clause |
| github.com/coder/websocket (transitive) | 1.8.14 | MIT |
| github.com/adrg/xdg (transitive) | 0.5.3 | MIT |
| github.com/mattn/go-colorable / go-isatty | 0.1.x / 0.0.20 | MIT |

The desktop module still uses only the Go standard library for its Windows host metrics (`internal/sysinfo`).

## Frontend

| Dependency | Version | Licence |
|---|---|---|
| @wailsio/runtime | 3.0.0 | MIT |
| React / React DOM | 18.3.x | MIT |
| lucide-react | 1.62.x | ISC |
| Tailwind CSS / @tailwindcss/vite | 4.3.x | MIT |
| Vite | 8.2.x | MIT |
| TypeScript | 5.9.x | Apache-2.0 |
| Vitest | 5.0.x | MIT |
| jsdom | 26.x | MIT |
| Testing Library React | 16.3.x | MIT |
| jest-dom | 6.6.x | MIT |
| @types/node, @types/react, @types/react-dom | current | MIT |

## Build and runtime requirements

- Windows 10/11 x64
- WebView2 Evergreen Runtime
- Go 1.26+ (local build used Go 1.27.0)
- Node 20+ (local build used Node 26.5.0, npm 11.17.0)
- Wails v3 CLI 3.0.0 for binding/resource generation

No licence text from dependency projects is relicensed by StackBroom. The project itself remains under its existing licence.
