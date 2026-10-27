# StackBroom desktop UX delivery

Date: 27 October 2026. Branch: `feat/desktop-ux-polish`.

Initial SHA: `f5a84af005eaee8a4e0c2731cf81e82bb4071c42`.
Final implementation SHA: `908cc7e9c15813aeaa9fc652de509e42125ced1a` (validation/report commits follow it; the final branch SHA is supplied in the delivery message).

## Outcome and audit

The scrolling defect was reproduced and fixed in the real Windows Wails/WebView2 application. Its implicit outer grid row took the main pane's intrinsic height, making the intended scroll pane as tall as its content. The body then clipped everything below the window. The repair constrains the outer row and permits the main grid item to shrink; it does not remove the fixed sidebar or rely on body scrolling. [Measured before/fix experiment](scroll-baseline.json).

The complete ranked findings and exact component scope are in [UX-AUDIT.md](UX-AUDIT.md). All five destinations received implemented improvements, including compact Overview hierarchy, labelled compact navigation, reversible process sorting, keyboard table access, truthful missing chart samples, current Storage observations, Developer section links, complete expandable source notes, clearer Settings and corrected light-theme contrast. Below-fold review additionally fixed long recommendation badges forcing content outside clipped panels.

## Before and after

The original 20-image gallery is preserved in `screenshots/`. This milestone adds a native baseline capture and 80 final native WebView screenshots (top and bottom for five screens, two themes, four size targets).

| Evidence | Before | After |
|---|---|---|
| Overview, dark, 1366×768 | [Original](screenshots/overview-dark-1366x768.png) | [Top](screenshots/ux-polish/overview-dark-1366x768.png), [bottom](screenshots/ux-polish/overview-dark-1366x768-bottom.png) |
| Processes, dark, 1366×768 | [Original](screenshots/processes-dark-1366x768.png) | [Top](screenshots/ux-polish/processes-dark-1366x768.png), [bottom](screenshots/ux-polish/processes-dark-1366x768-bottom.png) |
| Storage, light, 1366×768 | [Original](screenshots/storage-light-1366x768.png) | [Top](screenshots/ux-polish/storage-light-1366x768.png), [bottom](screenshots/ux-polish/storage-light-1366x768-bottom.png) |
| Developer Tools, dark, 1920×1080 | [Original](screenshots/developer-dark-1920x1080.png) | [Top](screenshots/ux-polish/developer-dark-1920x1080.png), [bottom](screenshots/ux-polish/developer-dark-1920x1080-bottom.png) |
| Settings, light, 1366×768 | [Original](screenshots/settings-light-1366x768.png) | [Top](screenshots/ux-polish/settings-light-1366x768.png), [bottom](screenshots/ux-polish/settings-light-1366x768-bottom.png) |

Before and after samples contain real, changing observations and were not captured at the same moment. They are not performance comparisons.

## Native interaction evidence

This was not a browser mock. The shipping application was built and launched. Because Windows computer-use screenshot capture repeatedly timed out, a separate local diagnostic executable was built from the same main source and embedded frontend, changing only Windows WebView profile/debug options. The shipping `main.go`, engine and four-method bridge are unchanged. The diagnostic endpoint was observed bound to `127.0.0.1:9333`. It is not a production feature and its binary is not committed or published.

[ux-validation.json](ux-validation.json) records 40 cases: all five screens in dark and light at 1280×720, 1366×768, 1600×900 and 1920×1080 physical client targets on the host's 125% DPI setting. WebView2 client dimensions were changed through CDP; actual CSS dimensions and device pixel ratio are recorded, including fractional-DPI rounding. This verifies native WebView client layout, not a physical drag of the outer Windows frame.

All 40 final cases passed. One Storage case was rechecked after the native viewport changed size during the original key sequence. Two initial-launch process/developer captures were replaced after their full scan completed, and the harness now waits for readiness. Original attempts and successful rechecks are retained in the JSON. Assertions cover a viewport-bounded shell, zero unexpected horizontal page overflow, zero overflow inside cards, wheel-induced outer scroll movement, Page Down movement and Ctrl+End reaching the measured bottom. Pages that fit have zero range and need no scrolling. Tables retain their own bounded horizontal/vertical scroll owners. Early test attempts were corrected to place outer-pane wheel input in the content padding rather than inside the process table, and to wait for keyboard scroll settling; final evidence records completed inputs.

[ux-interactions.json](ux-interactions.json) records:

- Twenty keyboard tab steps on each screen; all 100 sampled focus positions intersect the viewport.
- Process search empty results, pagination to page 2, and keyboard/wheel movement to the bottom of the process table.
- Developer section-link navigation preserving the destination, expandable observation notes, reduced-motion preferences and a CDP minimize/restore cycle with usable layout afterwards.
- Five 200% scaling simulations using a 640×360 CSS viewport for a 1280×720 target: page bottoms remain reachable and page overflow remains zero. This is emulation, not a changed Windows scaling setting.

The skip-link navigation regression, unknown chart gaps and reversible sorting are also covered by frontend tests. Windows accessibility text exposed the navigation and table semantics. Native computer-use key injection did not provide reliable focus evidence after restoring the window, and coordinate input reported unavailable geometry. Therefore OS-level keyboard routing is not certified by that helper; the successful interaction tests injected real key/wheel events into the running native WebView via CDP.

## Accessibility results and limits

[ux-accessibility.json](ux-accessibility.json): axe-core 4.14.0 reports zero WCAG 2 A/AA and WCAG 2.1 AA violations across all five screens in both themes. The checker was loaded from an existing cached package in an ignored validation directory; no runtime dependency was added. An initial light-theme subtitle contrast failure (4.34:1) was repaired before the final audit.

This is not a claim of full accessibility compliance. Narrator, physical touchpad input, manual scrollbar dragging, OS-level keyboard routing, physical DPI changes, Windows text-size preferences, and every possible table row/content combination remain manual acceptance gates. Reduced motion, focus visibility, named scroll regions, captions, status messages, source-note expansion and both themes were checked within the native WebView. Existing icon/title ellipses in some Docker/project evidence remain available through the table and title text; a dedicated inspect-details pane is future work.

## Automated and Windows build results

- Baseline frontend: 15 tests passed; strict TypeScript and production build passed. Initial sandbox-only Vite `spawn EPERM` was resolved by running the normal build with authorised escalation.
- Final frontend: **19 tests / 7 files passed**; strict TypeScript and Vite production build passed.
- Root CLI: `go test ./...`, `go vet ./...`, `go build ./...` passed. A sandbox Go cache warning was resolved on the authorised rerun.
- Desktop Go tests and vet passed through the repository Windows build script. Generated bindings still expose one service and four methods.
- Final production Windows executable: `apps/desktop/bin/StackBroom.exe`, **11,190,272 bytes**, SHA-256 `A4D8A154769B618CC3AA58944FE3ED7B7D52178C18DD2F3D462CAA1F9F480B42`.
- Final CSS repairs were repackaged with the same production tags, Windows resources, trimpath and GUI linker settings after the complete build-script pass. The binary is ignored and is not published.
- Go race-detector, alternate architectures and CI remain the limitations already documented in [VALIDATION.md](VALIDATION.md).

## Performance observations

[ux-performance.json](ux-performance.json) retains the individual process measurements. The initial baseline shipping instance plus six WebView2 children measured **481.6 MiB working set**. A separate no-debug observation using an isolated WebView profile measured **482.8 MiB combined**, **226.3 MiB private**, across seven processes. It uses the completed UI before the last badge-only CSS repair. The isolated profile prevents earlier hidden launches sharing WebView2 processes and contaminating attribution. A direct shipping-process-only sample is explicitly marked incomplete in the JSON.

These are single observations under heavy unrelated system load and different profile conditions. They neither prove a leak nor demonstrate a measurable memory improvement. The prior approximately 367 MB benchmark was lower; a repeatable idle/scan/navigation soak with controlled host load remains useful. Diagnostic memory grew from about 506 to 653 MiB during screenshot/axe testing; that instrumented workload must not be compared directly with idle shipping measurements. No process was killed to create artificial performance results.

Production JavaScript is approximately **235.06 kB / 72.40 kB gzip**, versus 232.36 / 71.84 at baseline. CSS is approximately 31.30 / 7.56 versus 27.60 / 6.81. No dependency was added. Forty in-memory history samples, deferred filtering and 50-row pagination remain. Heavy backdrop blur and competing card shadows were removed, but no CPU improvement is claimed without a controlled measurement.

## Reproduction and next functional work

Run the standard `scripts/build.ps1` from `apps/desktop`, then `scripts/build-ux-validation.ps1` to create the separate ignored diagnostic executable. Launch it explicitly. Supply `PLAYWRIGHT_MODULE` pointing to an installed Playwright package and run `scripts/validate-ux.cjs` and `scripts/validate-interactions.cjs` from the repository root. For `scripts/validate-a11y.cjs`, also supply `AXE_CORE_SCRIPT` pointing to a local `axe.min.js`. Close the diagnostic application normally after testing. Never distribute its binary or ship its debug options. These scripts add no product service, telemetry or app dependency.

[Future controls contract](FUTURE-CONTROLS.md) gives DeepSeek the entry points, target identity, permission/risk previews, individual confirmations, outcome feedback, recovery constraints and safe implementation sequence for processes, startup, storage, security and optional BYOK AI. The next milestone should start with read-only target details and bounded storage previews, then review capability-specific APIs before implementing any mutation. Complete the remaining physical-device/Narrator checks alongside that work.

No cleanup, termination, pruning, WSL mutation, Windows configuration change, AI integration, unrestricted binding, merge, tag or binary publication was introduced.
