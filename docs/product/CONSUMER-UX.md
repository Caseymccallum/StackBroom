# Consumer UX Architecture

## Structure

Primary navigation is grouped by intent, not by Windows subsystem:

1. **Home** — Overview, PC Timeline.
2. **Improve** — Performance, Startup, Cleaning.
3. **Understand** — Processes, Storage, Network, Problem Detective.
4. **Protect** — Security, Applications.
5. **Advanced** — Developer Tools.
6. **Settings**.

A destination is shown only when it has working, tested functionality. The current five-screen desktop shell remains the baseline until a milestone is complete.

## Progressive disclosure

- Default view answers "is this normal?" in plain language.
- Evidence and raw values are one level deeper.
- Technical controls (priority, termination, startup disabling) are never on the dashboard.
- Risk is expressed as consequences, not scores.

## Language

- "We observed", "this may indicate", "not enough evidence" instead of certainty.
- Never use "malware", "junk" or "optimise" without evidence and a defined action.
- Unknown is displayed as unknown, not zero.
- Every irreversible action says exactly what will be lost.

## Colour

Colour is supplementary. Status always includes text and an icon. No red/green-only encoding. Contrast meets WCAG 2.1 AA in both themes.

## Confidence and uncertainty

Every recommendation has:
- Observation.
- Confidence (high/medium/low/unknown).
- Why it matters.
- Safe next action.

Low confidence is not rendered as a warning.

## Empty, loading, error and partial states

- Empty states explain what will appear and why it is empty.
- Loading states name the work in progress and allow cancellation where safe.
- Errors show the backend's real message and a recovery action.
- Partial data is labeled partial; it is never padded with invented values.

## Accessibility

- All interactive controls are reachable by keyboard.
- Dialogs have a labelled `role="dialog"`, `aria-modal`, focus trap, Escape handling and focus restoration.
- Table row details are buttons, not click-only `<tr>` elements.
- Live action results use `role="status"` / `aria-live`.
- Focus is visible; motion respects reduced-motion preferences.

## Destructive confirmations

A confirmation must state target, action, consequence and reversibility. Confirmation is an in-app step, never `window.confirm`. Buttons are disabled while a request is in flight and duplicate submissions are refused.

## Beginner / advanced

Beginner mode uses plain names, hides raw counters and offers only safe actions. Advanced mode exposes evidence, process identity, priority classes and developer tooling. Switching modes never changes what is measured, only how it is explained.
