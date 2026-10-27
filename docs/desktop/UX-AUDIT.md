# Desktop UX implementation audit

Baseline: `f5a84af005eaee8a4e0c2731cf81e82bb4071c42`, clean `feat/desktop-foundation`.
Delivery branch: `feat/desktop-ux-polish`.

## Findings and disposition

| Severity | Finding | Implemented response |
|---|---|---|
| P0 | The implicit shell grid row grows to the content's intrinsic height, putting the lower page outside the viewport. | Explicit `minmax(0, 1fr)` shell row and `min-height: 0` on the main grid item. Sidebar independently scrolls in short windows. |
| P1 | Keyboard focus lands on a heading outside the scroll owner; skip-link hash can navigate to Overview. | Focus the labelled main pane after navigation; make it a keyboard tab stop. Skip link focuses it without changing destination. Ignore in-page section hashes in app navigation. |
| P1 | Unknown CPU samples become zeroes, and missing memory can enter chart history as a number. | Nullable history, broken chart paths at missing samples, measured-count labels, no invented latest value. Unknown meters have no numeric ARIA value. |
| P1 | Storage chooses quick data even when a full observation is newer. | Select by observation timestamp, matching Overview. |
| P1 | Small light-theme subtitle text fails contrast at 4.34:1. | Darker muted text token, verified again with axe. Light primary actions use white text on dark teal. |
| P1 | Many source notes are reduced to one warning, with a misleading suggestion that extra notes appear in Settings. | One expandable, deduplicated source-note summary with all notes available on Overview and Developer Tools. |
| P2 | Large hero and stacked header push essential resource readings down. | Compact host card and header, four resource cards at 1000+ CSS pixels, reduced empty-state height. |
| P2 | Icon-only compact navigation is hard for beginners to learn. | Retain visible destination names on the compact rail and explicit accessible names. |
| P2 | Table scrollers lack keyboard entry, names are truncated and sorting arrows imply the wrong direction. | Focusable named table regions, wrapped process names, bounded table height, truthful arrows/ARIA sort and reversible sorting with unknown observations last. |
| P2 | Developer page has a long evidence sequence without section navigation. | Real in-page links for Docker/WSL, listeners, process evidence and reviews. Confidence and source details retained. |
| P2 | Storage copy exposes implementation concerns instead of explaining the available function. | Plain-language capacity boundary and clearly labelled planned exploration. No fake controls. |
| P2 | Settings contains release-engineering prose in ordinary preferences. | Useful keyboard shortcuts and a clearly labelled roadmap; retain actual toolchain in About and engineering caveats in documentation. |
| P2 | Controls are small, card shadows compete, and wrapping is fragile. | 40-pixel primary controls, restrained flat navy surfaces, wrapping headers/settings and stable scrollbar gutters. |

## Scope

Changed App navigation/focus, Sidebar, shared cards/progress, Sparkline, process filtering/table, listener and Docker table wrappers, Overview, Processes, Storage, Developer Tools, Settings and shell/responsive styles. No Go engine, CLI or shipping bridge changes; no new runtime dependency. Session history remains capped at 40, deferred search and 50-row pagination remain. No timers or chart animation were added. Existing reduced-motion overrides still apply.

## Scroll diagnosis

`scroll-baseline.json` records the actual native WebView2 experiment. At a 1106×683 CSS viewport the unfixed content pane is 2199 pixels high with zero scroll range; a real wheel event leaves `scrollTop=0`. The shell's implicit row measures about 2355 pixels. Adding only the explicit grid row and main minimum-height repair bounds the pane to 528 pixels and the same wheel input changes `scrollTop` to 1200. Body overflow hiding alone is not the cause, and removing it would surrender the fixed navigation layout.

The baseline screenshot is `screenshots/ux-before-native.png`. The original 20-image gallery remains available. New top and bottom screenshots are under `screenshots/ux-polish/`; `ux-validation.json` contains the measured input results and actual viewport dimensions. See `UX-DELIVERY.md` for final results and limitations.
