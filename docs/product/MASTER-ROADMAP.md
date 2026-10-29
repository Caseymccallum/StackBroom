# StackBroom Master Roadmap

This roadmap supersedes feature ordering in the legacy technical roadmap while preserving its history and CLI compatibility notes (see `docs/ROADMAP.md`).

## Waves

### Wave A — Core usefulness

| Milestone | Capability | Permission |
|---|---|---|
| M1 | Process details, graceful close, verified termination, limited priority | Explicit per action |
| M2 | Storage Intelligence: user-scoped analysis, treemap, large files, duplicate candidates | Read-only (implementation on `feat/m2-storage-intelligence`) |
| M3 | Curated cleanup engine with exact previews and recoverable operations where supported | M3A discovery/preview read-only; M3B-1/2/3 execution and recovery architecture built but disabled; M3B-4 concludes Recycle Bin NO-GO and keeps preview-only; M3C simplifies the Cleaning UX and adds a tagged, unwired handle-bound permanent-delete prototype; no live cleanup is authorised |
| M4 | Installed applications, startup inventory/management, advanced process controls | Explicit per action |

### Wave B — Distinctive product features

| Milestone | Capability |
|---|---|
| M5 | Maintenance History & Undo Centre |
| M6 | PC Timeline (opt-in local snapshots and change explanation) |
| M7 | Problem Detective (guided, evidence-based performance investigation) |
| M8 | Application Trust Explorer (signature, publisher, startup, process evidence) |

### Wave C — Deeper diagnostics

| Milestone | Capability |
|---|---|
| M9 | Network Transparency (connections, UDP scope, ownership, no packet capture initially) |
| M10 | Laptop Health & Power diagnostics |
| M11 | Reliability Centre (crashes, hangs, selected events) |
| M12 | File Lock Detective (Restart Manager, plain-language explanations) |

### Wave D — Advanced ecosystem

| Milestone | Capability |
|---|---|
| M13 | Specialist maintenance packs |
| M14 | Windows Security and optional ClamAV/YARA-X research |
| M15 | Optional BYOK AI adviser |
| M16 | Privacy-safe support packs |

## Ranking criteria

Capabilities are ordered by everyday value, feasibility, safety risk, performance cost, dependencies, offline operation and differentiation. Release numbers follow completed capabilities, not the milestone numbers.

## Foundational capabilities introduced early

- M1 action-history contract (small, local, privacy-filtered).
- Target-token and action-security primitives reusable by M3/M4/M12.
- Privacy redaction and evidence/uncertainty labels.
- Accessibility and light/dark design system.
- Local, opt-in history storage with retention controls before M6 needs it.

## Explicit non-goals for the next wave

- Registry cleaning.
- Automatic deletion of user documents, downloads, game saves, browser profiles, credentials or Docker volumes.
- Shell execution or arbitrary cleanup scripts.
- Real-time antivirus replacement claims.
- Silent cloud telemetry or forced accounts.
- Automatic "Fix All" controls.
- Fabricated health scores or historical data.

## Release gates

Each milestone requires its own acceptance gate, automated tests, refusal-path tests, privacy review and owner approval before live destructive testing. No milestone is complete because a research document exists.
