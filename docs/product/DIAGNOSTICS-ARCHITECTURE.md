# Diagnostics Architecture

## Goal

Explain observed behaviour without overclaiming. Diagnostics produce evidence, correlations and bounded explanations. They do not produce a generic "health score".

## Problem Detective flow

1. **Frame the symptom** — slow startup, high memory, high CPU, disk activity, crashes, battery drain.
2. **Collect a bounded observation window** — CPU, memory, disk, process and optional event evidence.
3. **Correlate** — identify candidate contributors with timestamps and confidence.
4. **Explain** — plain-language hypothesis plus the evidence that supports it.
5. **Offer safe next steps** — inspect, open details, or use an approved action.
6. **Verify** — re-measure after an action and state whether the symptom changed.

## Data sources

- Existing process/port/disk observation engine.
- `GetSystemTimes`, `GlobalMemoryStatusEx`, process I/O counters.
- Optional bounded ETW sessions for file/process/network context.
- Windows Event Log and reliability records for crash/hang explanations.
- Power/battery APIs.

ETW is opt-in, time-bounded, cancellable and never a permanent background service by default. A diagnostic session must stop when the user stops it.

## Evidence model

Each finding carries:
- What was observed.
- When and for how long.
- Which process/file/device is involved.
- Confidence and missing evidence.
- Whether the observation is historical or live.

## Non-goals

- Kernel drivers or packet capture in the first wave.
- Continuous background telemetry.
- Automatic remediation without explicit preview/confirmation.
- Malware verdicts from resource usage.
- Fabricated history when no samples exist.

## Performance limits

- CPU sampling must not itself consume meaningful CPU.
- Memory ceilings for retained samples.
- Bounded history (ring buffer) with retention settings.
- All collection cancellable and safe to run while the UI remains responsive.

## Privacy

Local by default. Command lines, executable paths and file paths are redacted before display and before any optional AI request. AI receives a previewed, user-approved subset only.
