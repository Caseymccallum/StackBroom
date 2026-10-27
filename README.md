# StackBroom

**Know what's running. Reclaim what's idle.**

StackBroom is a lightweight, open-source, Windows-first resource inspector for
software developers. It shows what development processes, Docker and WSL2 are
using, and suggests things worth reviewing. It never cleans up anything itself.

- **Version:** 0.1.0 (read-only)
- **Platform:** Windows 10/11 (process inspection is Windows-only; Docker and WSL
  inspection degrade gracefully elsewhere)
- **Dependencies:** Go standard library only, plus the tools it inspects
- **Network:** none. No telemetry, no accounts, no cloud services.

> **Safety first.** StackBroom v0.1 has no cleanup capability. It cannot stop a
> process, remove a container or image, prune Docker, delete a volume or stop a
> WSL distribution. Read [docs/SAFETY.md](docs/SAFETY.md) before relying on it.

## What it does

`stackbroom scan` reports:

- **Development processes** -- Node.js, Go, Python, .NET, Java, Ruby, PHP, Deno,
  Bun, Elixir and Rust processes, with PID, runtime, role (dev server, dev tool
  or plain runtime), working-set memory, parent process, age and the recognized
  development-server kind (vite, next dev, uvicorn, `go run`, `npm run dev`, ...).
- **Project associations** -- which local project a process belongs to, marked
  `confirmed`, `inferred` or absent, with the evidence behind the claim.
- **Potential duplicate servers** -- processes of the same runtime requesting the
  same port, or repeated servers for one confirmed project. Always framed as
  "check this", never as "this is redundant".
- **Docker** -- running and stopped containers, Docker's own CPU/memory stats for
  running containers, Compose project metadata, disk usage per category, the
  reclaimable figures Docker reports, images and build cache.
- **WSL2** -- installed distributions, their state and version, and the host-side
  WSL utility VM memory when it can be read.
- **A resource overview** -- what is using RAM, what Docker occupies, and where a
  careful look might pay off.

`stackbroom plan` turns the observations into review suggestions. Every
suggestion states what was detected, why it might be unnecessary, the evidence,
the known or estimated resource use, what to check before acting, and the risk
of any cleanup.

## What it does not do

- It does not kill processes, stop or remove containers, remove images, prune
  anything, delete volumes, or start/stop WSL distributions.
- It does not start Docker Desktop (a stopped daemon is reported, not launched).
- It does not run commands inside WSL distributions.
- It does not estimate memory it could not read; it reports `Unknown`.
- It does not claim a process belongs to a project without on-disk evidence.

## Requirements

| Requirement | Notes |
| --- | --- |
| Windows 10 1809+ or Windows 11 | Process inspection uses Toolhelp32 and PSAPI |
| Go 1.22+ to build | Built and tested with Go 1.27.0 windows/amd64 |
| PowerShell 5.1+ | Optional; used only for the read-only CIM command-line query |
| Docker CLI 20.10+ | Optional; older versions fall back to table parsing |
| WSL2 | Optional |

No cgo, no external Go modules, no admin rights required. Some processes owned by
other users or protected by the OS show `Unknown` memory, which is expected.

## Build

```powershell
git clone https://github.com/Caseymccallum/StackBroom.git
cd StackBroom
go build -trimpath -ldflags "-s -w -X github.com/Caseymccallum/StackBroom/internal/version.Version=0.1.0" -o bin/stackbroom.exe ./cmd/stackbroom
```

Or use the validation script, which formats, vets, tests and builds:

```powershell
pwsh -File scripts/build.ps1 -Version 0.1.0
```

The result is a single self-contained `bin\stackbroom.exe` (roughly 5 MB). Copy
it anywhere on your `PATH`; it has no runtime files.


## Usage

```
stackbroom scan [flags]      Read-only overview of development resource use
stackbroom plan [flags]      Review suggestions; nothing is ever executed
stackbroom version           Print the installed version
stackbroom help              Show help
```

| Flag | Applies to | Meaning |
| --- | --- | --- |
| `--json` | scan, plan | Emit structured JSON (stable `schemaVersion`) |
| `--top N` | scan | How many memory-heavy processes to list (default 8) |
| `--timeout DURATION` | scan, plan | Limit for **each** external command (default `20s`) |
| `--skip-docker` | scan, plan | Do not query Docker at all |
| `--skip-wsl` | scan, plan | Do not query WSL at all |
| `--no-command-lines` | scan, plan | Skip the PowerShell CIM lookup (faster, but roles and projects cannot be identified) |

Exit codes: `0` success, `1` runtime error, `2` usage error. Docker or WSL being
unavailable is **not** an error: the scan still succeeds and explains what it
could not read.

### Example: a real scan

Genuine output from a Windows 10 development machine on 27 October 2026 (Docker
Desktop 29.4.0 running, one WSL2 distribution), with some rows and project names
shortened for readability:

```
stackbroom 0.1.0 - read-only scan
Generated 2026-10-27 16:07:22 in 1.5s
Host windows/amd64 (DESKTOP-KDCMDNS) - memory 16.52 GB used of 23.78 GB, 7.26 GB available

RESOURCES
  Development processes  4 (0 servers)                   280 MB reported
  Top memory consumers   8 shown                         4.76 GB in listed processes
  Docker                 16.09 GB on disk                12.18 GB reclaimable
  WSL2 utility VM        462 MB host-side (approximate)  1 of 1 distributions running

DEVELOPMENT PROCESSES (4)
  PID    RUNTIME  CATEGORY  PROJECT                MEMORY  AGE  DETAIL
  7532   node     dev-tool  Proxima (confirmed)    59 MB   28m  parent Code.exe
  16576  node     dev-tool  agent-tools (confirmed) 100 MB 28m  parent cmd.exe
  ...

DOCKER
  Status: available (server 29.4.0, context desktop-linux)
  Containers: 0 running, 5 stopped
  Storage (Docker's own report; total and reclaimable are different measures):
    TYPE           TOTAL  ACTIVE  SIZE     RECLAIMABLE
    Images         25     5       15.64GB  12.41GB (79%)
    Containers     5      0       30.09MB  30.09MB (100%)
    Local Volumes  18     4       979MB    590.3MB (60%)
    Build Cache    21     0       630.1MB  42.68MB

WSL
  DISTRIBUTION    STATE    VERSION  DEFAULT
  docker-desktop  Running  2        yes
  Host-side WSL2 VM memory: 462 MB (approximate; vmmem covers all distributions)
  Per-distribution memory: Unknown (StackBroom does not run commands inside distributions)

REVIEW OPPORTUNITIES
  1. 2 containers have been stopped for at least 14 days  [risk: medium, confidence: confirmed]
  2. Docker reports 12.41GB (79%) of image storage as reclaimable  [risk: medium, confidence: confirmed]
  Run `stackbroom plan` for the evidence and review steps behind each suggestion.
  Nothing has been stopped, deleted or pruned.
```

### Example: a plan entry

```
1. 2 containers have been stopped for at least 14 days
   Risk: medium | Confidence: confirmed
   Detected: Docker reports 2 containers stopped for at least 14 days; the oldest reports 5 months.
   Why it may be unnecessary: Stopped containers keep their writable layer on disk and keep a
   service definition in the environment. ...
   Evidence:
     - beautiful_napier - Exited (255) 5 months ago
     - decisionops-db-1 - Exited (255) 5 months ago (compose project decisionops)
     - Ages come from Docker's rounded status text, not from timestamps.
   Resource use: Docker reports 30.09MB used by container writable layers across all 5 containers;
   per-container sizes were not measured.
   Review before acting:
     - Check whether you still need logs, test data or the writable layer inside these containers.
     - Confirm whether the project's Compose file still defines them; ...
     - Note that named volumes are separate from containers and are not included in the container size.
   Risk of clean-up: Removing a container deletes its writable layer and any anonymous volumes. ...
   Applies to: container beautiful_napier (...), container decisionops-db-1 (...)

NO CLEANUP WAS PERFORMED
```

Machine-readable output is available with `scan --json` and `plan --json`. Both
include `"readOnly": true`; the plan document also includes
`"executesCleanup": false`.


## How confidence works

| Level | Meaning for a project association |
| --- | --- |
| `confirmed` | A project manifest exists on disk, the running script or executable is inside that project, and the referenced path exists |
| `inferred` | A manifest exists and a path argument points into it, but the running script is elsewhere or the path is missing from disk |
| *(none)* | No defensible evidence; the process is listed without a project |

Paths inside `node_modules`, npm/pnpm/yarn caches, `_npx`, AppData temp/programs,
Windows and Program Files are deliberately excluded: a package's own
`package.json` is not the user's project.

## Measurement accuracy

| Measure | Source | Accuracy |
| --- | --- | --- |
| Process working set | `GetProcessMemoryInfo` (PSAPI), CIM fallback | Good. Working set is resident memory, not private bytes |
| Process age | CIM `CreationDate` | Good when CIM is available |
| Command line | CIM via PowerShell | Good for the current user; may be unavailable for protected processes |
| Dev-server role | Command-line patterns | Deterministic but pattern-based; unrecognized servers stay `runtime` |
| Requested port | Command-line flags | Explicit flags only; environment/config ports are unknown |
| Port ownership | *not measured* | StackBroom does not inspect listening sockets in v0.1 |
| Docker sizes / reclaimable | `docker system df` | Docker's own estimate; total and reclaimable are different measures |
| Container CPU / memory | `docker stats --no-stream` | Docker's own one-shot sample |
| Per-container disk size | *not measured* | Unknown |
| WSL distribution state | `wsl.exe --list --verbose` | Good |
| WSL VM memory | `vmmem` working set | Approximate and host-side only; shared by all distributions |
| Per-distribution memory | *not measured* | Unknown by design |

## Test and development commands

```powershell
gofmt -l .                # should print nothing
go vet ./...
go test ./...             # unit tests only; no live processes, Docker or WSL

# Opt-in, read-only live inspection of the current machine:
$env:STACKBROOM_LIVE = "1"; go test ./internal/winproc -run TestLive -v
```

Ordinary tests use deterministic fixtures and injected fake runners. They never
start, stop or modify anything. The live tests are read-only and skipped unless
`STACKBROOM_LIVE=1`.

## Repository layout

```
cmd/stackbroom/      Program entry point
internal/model/      Shared JSON/data types (no behaviour)
internal/winproc/    Windows process enumeration and classification
internal/project/    Project association heuristics
internal/docker/     Docker CLI inspection and parsers
internal/wsl/        WSL inspection and parsers
internal/wintext/    UTF-16/UTF-8 console decoding
internal/analysis/   Summaries, duplicate suspects, recommendations
internal/scan/       Orchestration of collectors + analysis
internal/cli/        Commands, text rendering, JSON output
internal/run/        Bounded external-command runner
internal/units/      Byte formatting
internal/version/    Build stamp
docs/                Architecture, safety, roadmap
scripts/             Build/validation helper
```

Further reading: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md),
[docs/SAFETY.md](docs/SAFETY.md), [docs/ROADMAP.md](docs/ROADMAP.md).

## Privacy

StackBroom makes no network calls. It reads local process metadata and queries
local Docker and WSL services. `scan --json` output contains process command
lines and local paths, so treat a saved report as sensitive.

## Known limitations (v0.1)

- Port-based duplicate detection uses requested ports from command lines, not the
  actual listening socket. Two processes may request the same port while only one
  is bound.
- Servers started without a recognisable command line (for example through a
  wrapper script) stay classified as `runtime`.
- WSL per-distribution memory and Docker per-container sizes are not measured.
- Localized WSL state names are preserved verbatim; only English states are
  normalised, so a non-English state may not be counted as "Running".
- Docker Desktop must already be running for Docker data to appear.

## Roadmap

See [docs/ROADMAP.md](docs/ROADMAP.md). The next milestone is verifying which
process actually owns a listening port before calling anything a duplicate.

## Licence

`LICENSE` contains a **proposed MIT licence**, included for the owner's approval.
It is not final until the repository owner confirms it.

