# StackBroom roadmap

The guiding rule: a dependable read-only tool is more valuable than a
feature-rich one that is unsafe. Each milestone must be independently useful and
must not weaken the safety contract.

## v0.1.0 -- read-only discovery (delivered)

- Windows process discovery: PID, parent, executable, working set, age, role.
- Runtime and dev-server classification for Node, Go, Python, .NET, Java, Ruby,
  PHP, Deno, Bun, Elixir and Rust.
- Evidence-based project association with `confirmed` / `inferred` / none.
- Docker inspection: containers, Compose metadata, stats for running containers,
  storage categories, images.
- WSL2 inspection: distributions, state, version, host-side VM memory.
- Resource summary, duplicate suspects, six-part recommendations.
- `scan`, `plan`, `version`, text and JSON output, graceful degradation.
- Unit tests with fixtures and fake runners; opt-in read-only live tests.

**Not in scope, by design:** any form of cleanup.

## v0.1.x -- hardening (next, small)

- Publish the JSON schema as a versioned document with examples.
- Add a `--max-processes` safety valve for machines with very large process lists.
- Normalise localised WSL states (or document a `--raw` mode).
- Broaden classification fixtures from real-world manager wrappers (pnpm, Volta,
  fnm, `nvm-windows`).
- Add golden-file tests for text rendering to catch accidental layout changes.
- Add GitHub Actions once the owner's billing restriction is lifted; until then
  `scripts/build.ps1` is the release gate.

## v0.2 -- port truth

**Goal:** stop guessing which development server owns a port.

- Read the TCP listener table through `GetExtendedTcpTable` (IPv4 and IPv6) and
  map listening ports to PIDs, read-only.
- Report requested vs actually-bound ports per process.
- Upgrade duplicate confidence when two processes cannot both be bound, and say
  which PID holds the port.
- Distinguish "server failed to bind and exited" from "server is running".

**Acceptance:** duplicate recommendations cite the listening PID and the
requested-port evidence separately; no new mutating capability.

## v0.3 -- activity without assumptions

- Sample CPU time (`GetProcessTimes`) over a short window to show recent activity
  as a measured fact, never as an "idle" verdict.
- Per-project rollups: processes + containers + declared compose project.
- Optional local config file (`.stackbroom.json`) for ignored paths, ignored
  ports and extra project markers. Read-only; no cleanup settings.

**Acceptance:** activity is shown with its sampling window and unit; no
recommendation says a process is "idle".

## v0.4 -- history and comparison

- Optional local snapshot history (JSON files under the user's profile), opt-in,
  no network.
- `stackbroom compare <before> <after>` to show what changed.
- Retention controls and a documented schema migration policy.

**Acceptance:** no telemetry; history can be disabled entirely; deleting history
is a file operation the user performs.

## v0.5 -- approval-driven cleanup (requires a separate safety review)

Only after v0.2-v0.4 prove the data is trustworthy.

- One action at a time, with an explicit confirmation that names the target and
  the consequence.
- Identity checks that survive PID reuse (PID + start time + executable path).
- Initial scope limited to build-cache pruning and removing clearly unused
  images; **no volumes, no bind mounts, no user files**.
- Dry-run output that is identical to the real action's plan.
- Local audit log of what was done.

**Gate:** a written threat model, a test suite proving unapproved targets are
never touched, and owner sign-off.

## v0.6 -- optional native interface

- A small native Windows front end over the same engine (no Electron).
- Read-only views first; any cleanup action reuses the v0.5 approval flow.

## v1.0 criteria

- Reliable on Windows 10 and 11 with and without Docker Desktop/WSL.
- No known false "confirmed" project associations in the fixture corpus.
- Documented JSON schema with migration notes.
- Reproducible builds and a CI pipeline.
- Safety documentation reviewed alongside every release.
