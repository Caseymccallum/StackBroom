# StackBroom Product Vision

## What StackBroom is

StackBroom is a free, open-source Windows PC understanding, maintenance, security and control centre. It is built for ordinary users first and developers second, without giving up technical honesty.

It answers four questions:

1. What is happening on my PC?
2. Why is it happening?
3. What can I safely do about it?
4. What changed after I acted?

## Principles

- **Measure, don't guess.** Unknown values stay unknown. A high percentage is not a diagnosis.
- **Explicit control.** Every system-changing action is previewed, confirmed and verified. No silent "Fix All".
- **No forced account.** Core inspection and maintenance work offline and without a subscription.
- **No unsolicited telemetry.** No hidden analytics, scareware or health scores.
- **Privacy by default.** Command lines, executable paths, project roots and user paths do not cross into the webview unless a reviewed, explicit feature requires a redacted subset.
- **Reversible where claimed.** Never call termination or permanent deletion reversible.
- **Optional intelligence.** AI is an optional BYOK adviser, never an authority over system actions.

## Product architecture

```text
Observation Engine
    |
    v
Evidence & Diagnostics
    |
    +--> Desktop Interface
    +--> Local History (opt-in)
    +--> Optional AI Adviser

Action Security Layer
    |
    +--> Process Management
    +--> Startup Management
    +--> Storage Cleanup
    +--> Other Approved Capabilities
```

Inspectors only observe. The action security layer only performs capability-specific, user-authorised operations. The AI layer only advises. The frontend only displays and requests.

No general-purpose privileged shell or arbitrary file-operation bridge is permitted.

## Audiences

### Beginner

Plain-language status, safe defaults, explanations, no destructive controls on the main dashboard.

### Enthusiast / developer

Deeper evidence, process identity, listener ownership, Docker/WSL views, advanced process and startup controls behind progressive disclosure.

## Product shape

Long-term navigation: Overview, Performance, Processes, Storage, Cleaning, Applications, Startup, Security, PC Timeline, Problem Detective, Developer Tools, Settings.

Only destinations with working functionality are exposed. Empty placeholders are not shipped.

## Success

StackBroom is successful when a user can understand a real problem, make an informed decision and verify the outcome, without being frightened into an unnecessary cleanup or losing control of their machine.
