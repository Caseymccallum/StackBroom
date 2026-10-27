# StackBroom architecture

Status: v0.1.0 (read-only). This document describes what exists, not what is
planned. Plans are in [ROADMAP.md](ROADMAP.md).

## Design principles

1. **Observation before action.** v0.1 has no cleanup code path at all. Safety is
   structural, not a flag.
2. **Evidence over inference.** A project association needs a manifest on disk and
   a process path inside it. Age, memory use and "looks idle" are never treated as
   evidence that something is unnecessary.
3. **Unknown is a valid answer.** Missing measurements are reported as `Unknown`
   or `Unavailable`, never filled with an estimate.
4. **Small, testable components.** Every collector is an interface with a fake in
   tests. Parsers are pure functions over strings and bytes.
5. **Standard library only.** No third-party Go modules, no cgo, no embedded
   browser, no server, no telemetry.

## Data flow

```
                      +-----------------------+
                      |  cmd/stackbroom main  |
                      +-----------+-----------+
                                  |
                     internal/cli (commands, rendering)
                                  |
                     internal/scan (orchestration, defaults)
             +--------------------+--------------------+
             |                    |                    |
   internal/winproc       internal/docker       internal/wsl
   (processes, CIM)       (docker CLI)          (wsl.exe)
             |                    |                    |
             +--------------------+--------------------+
                                  |
                        internal/analysis
              (summary, duplicate suspects, recommendations)
                                  |
                          internal/model
                     (JSON contract, no behaviour)
```

Supporting packages: `internal/project` (association heuristics, used by `scan`),
`internal/run` (bounded command execution), `internal/wintext` (console encoding),
`internal/units` (byte formatting), `internal/version` (build stamp).

Dependency rule: `model` imports nothing from the project; collectors and analysis
never import `cli`; `cli` never imports collector internals.

## Packages and responsibilities

| Package | Responsibility | Notable constraints |
| --- | --- | --- |
| `model` | Data types and the JSON contract (`schemaVersion` 1) | Pure data; no I/O |
| `run` | Execute an external command with a hard timeout | Argument vector only; no shell |
| `winproc` | Enumerate processes, read memory and image paths, parse CIM command lines, classify runtimes and dev servers | Syscalls isolated in `*_windows.go`; parsers platform-neutral |
| `project` | Turn path evidence into a project association with a confidence level | Requires an on-disk manifest; ignores package caches |
| `docker` | Query the Docker CLI and parse containers, stats, disk usage and images | Read-only subcommands only; JSON-lines parsers with table fallbacks |
| `wsl` | List distributions and decode WSL console output | Only `wsl.exe --list --verbose` |
| `wintext` | Decode UTF-16LE (with or without BOM), UTF-16BE, UTF-8 and BOMs | Never silently drops data |
| `privacy` | Redact secrets and tokenise paths at the output boundary | Deep-copies the snapshot; never mutates inspection data |
| `analysis` | Aggregate totals, find duplicate suspects, build recommendations | Threshold-driven; no I/O |
| `scan` | Wire collectors, resolve projects, attach WSL memory, call analysis | Collectors injected for tests |
| `cli` | Parse flags, render text, encode JSON, exit codes | ASCII-only text output |

## Process collection

1. `CreateToolhelp32Snapshot(TH32CS_SNAPPROCESS)` enumerates PID, parent PID and
   executable name. The `PROCESSENTRY32W` buffer is decoded by a pure function, so
   its layout is covered by unit tests with synthetic bytes.
2. `OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION)` plus
   `QueryFullProcessImageNameW` obtains the executable path where permitted.
3. `GetProcessMemoryInfo` reads the working set. If Windows refuses access, the
   value stays unknown.
4. Optional enrichment runs one fixed PowerShell/CIM query:

   ```powershell
   $ErrorActionPreference='Stop'
   [Console]::OutputEncoding=[System.Text.Encoding]::UTF8
   Get-CimInstance -ClassName Win32_Process |
     Select-Object ProcessId,ParentProcessId,CommandLine,CreationDate,WorkingSetSize |
     ConvertTo-Json -Compress -Depth 2
   ```

   This supplies command lines, start times and a memory fallback for processes
   the API cannot open (notably `vmmem`). The script is a constant; observed data
   is never interpolated into it. If PowerShell is unavailable, the scan continues
   with roles and projects marked unknown and an explicit warning.
5. Classification uses only the executable name and command line. Known dev-server
   invocations become `dev-server`; build/test/tooling invocations become
   `dev-tool`; other runtime processes stay `runtime`; anything else is `other`.
   A plain `node server.js` is **not** a dev server.
6. After project resolution, a `runtime` process with a **confirmed** project is
   promoted to `dev-tool` (it is development activity) but never to `dev-server`.

## Project association

* Candidate paths come from the executable, the first script argument and values
  of path-taking flags (`--prefix`, `--root`, `--dir`, `--app-dir`, ...).
* Positional arguments after the first are ignored: they are usually data (an MCP
  filesystem server is handed a directory to serve, not a project it belongs to).
* Relative paths are ignored because the process working directory is not
  observable through CIM.
* The nearest ancestor containing a manifest (`package.json`, `go.mod`,
  `pyproject.toml`, `requirements.txt`, `manage.py`, `Cargo.toml`,
  `composer.json`, `Gemfile`, `pom.xml`, `build.gradle`, `mix.exs`, `.git`)
  is the project root. Volume roots never qualify.
* `confirmed` needs: manifest present, referenced path exists, and the running
  script or executable inside the root. Everything else is `inferred`.
* Docker containers use the `com.docker.compose.project.working_dir` label
  instead; Docker is the authority on where a Compose project lives.

## Docker inspection

| Purpose | Command |
| --- | --- |
| Availability | `docker version --format {{.Server.Version}}` |
| Context | `docker context show` |
| Containers | `docker ps -a --no-trunc --format {{json .}}` |
| Usage | `docker stats --no-stream --format {{json .}}` (running containers only) |
| Storage | `docker system df --format {{json .}}`, falling back to `docker system df` |
| Images | `docker image ls --no-trunc --format {{json .}}` |

Parsers are JSON-lines based, tolerant of blank lines, and report malformed lines
as warnings instead of failing. Sizes are parsed from Docker's human-readable text
(SI and IEC units). Unparsable sizes stay unknown. A stopped daemon or missing CLI
ends Docker collection immediately with an actionable reason; StackBroom does not
attempt to start Docker Desktop.

### Endpoint safety

Before any daemon command, `internal/docker` resolves the endpoint without
contacting it. `DOCKER_CONTEXT` selects a context; a named context's endpoint
takes precedence over `DOCKER_HOST`, while the special `default` context derives
its endpoint from `DOCKER_HOST`. StackBroom reads the endpoint Docker reports with
`docker context show` and `docker context inspect` (local metadata only) and
classifies that effective target.

Only the context's explicit `Endpoints["docker"]` entry is used; other endpoint
types are never selected. Only canonical local named pipes
(`npipe:////./pipe/<name>`), absolute Unix sockets and numeric file descriptors are
treated as local. Host-qualified named pipes address another machine over SMB and
are rejected as unsupported, so they are never contacted even with
`--allow-remote-docker`; remote daemons use `tcp` or `ssh`. Malformed endpoints are
invalid, every network transport requires explicit approval, and invalid or
unresolved endpoints are refused even with that flag. The resolved endpoint,
validity, locality and approval state are reported in `docker.status`, so "Docker
unavailable" is never ambiguous between "daemon stopped", "remote endpoint
declined" and "endpoint could not be resolved".

Container sizes shown by `docker ps` are ignored; per-container disk usage is
reported as unmeasured rather than approximated.

## WSL inspection

`wsl.exe --list --verbose` only. Output is decoded with `wintext` because WSL
emits UTF-16LE **without a byte-order mark**. Rows are sliced using the header's
column positions (headers and states are localized, names may contain spaces) with
a right-to-left fallback. The `*` marker identifies the default distribution.

Memory is attributed only to the shared WSL2 utility VM (`vmmem` or `vmmemWSL`
working set) and labelled approximate. Per-distribution memory would require
running commands inside a distribution, which StackBroom deliberately does not do.

## Privacy boundary

Inspection needs full command lines; output does not. `internal/privacy` applies a
policy to a deep copy of the snapshot at the CLI boundary, after analysis:

- default: command line removed, `commandLineOmitted` set, home paths tokenised,
  secrets masked;
- `--diagnostics`: command line retained with secrets masked and paths tokenised,
  plus a sensitivity warning.

Because the same pass rewrites project evidence, duplicate evidence,
recommendation text, subject detail and warnings, there is no second channel that
can leak a raw command line. JSON contains `redaction` metadata so a consumer can
tell "omitted" from "not observed". See [JSON-SCHEMA.md](JSON-SCHEMA.md).

## JSON contract

`scan --json` emits a `Snapshot`; `plan --json` emits a `PlanDocument`. Both carry
`schemaVersion: "2"` and `readOnly: true`. Version 2 omits command lines by
default, adds `redaction` metadata, Docker endpoint fields, independent reclaim row
counts, per-stage `timings` and WSL warnings. The plan document also carries
`executesCleanup: false`.

Fields that could not be measured carry an explicit `known` boolean next to the
value (for example `memoryKnown`, `dockerTotalKnown`), and partial Docker totals
report how many categories were readable. The machine-readable schema lives in
`docs/schema/v2.json` and is checked by `internal/model/schema_test.go`.

## Testing strategy

* Parsers are tested with deterministic fixtures: captured Docker JSON, table
  output, WSL UTF-16 bytes, CIM JSON and synthetic `PROCESSENTRY32W` buffers.
* Collectors are tested with fake `run.Runner` implementations that record every
  invocation. Tests assert that no mutating command (`prune`, `rm`, `stop`,
  `kill`, `start`) is ever issued.
* Analysis is tested with constructed models, including unknown memory, partial
  Docker reports and empty inputs.
* CLI tests inject a fixed snapshot, so rendering is deterministic and no live
  process, container or distribution is touched. `testdata/*.golden.txt` lock the
  exact text layout.
* Large-inventory tests cover 20,000 processes in analysis, 5,000 in orchestration
  and 2,000 rendered rows.
* Privacy tests use synthetic passwords, tokens, database URLs and profile paths,
  and assert that neither JSON nor text can leak them.
* Output truncation is tested directly and through the runner.
* The JSON schema is validated against the Go model (required keys, `$ref`
  resolution, version constant).
* Live tests are opt-in (`STACKBROOM_LIVE=1`) and read-only.

## Key decisions and trade-offs

| Decision | Why | Cost |
| --- | --- | --- |
| Stdlib only | No supply-chain or licence surface; trivial builds | Manual Win32 bindings |
| PowerShell/CIM for command lines | Reading the PEB would need `PROCESS_VM_READ` and fragile 32/64-bit handling | ~1.5 s per scan; PowerShell dependency |
| Toolhelp32 + PSAPI for memory | Fast, no elevation, works for most processes | Some processes report `Unknown` |
| No socket inspection in v0.1 | Keeps the milestone small and avoids another syscall surface | Duplicate detection uses requested ports, not listening ports |
| ASCII output | Survives Windows code pages and PowerShell redirection | No colour or box drawing |
| Compose label for container projects | Docker's own metadata is stronger than path heuristics | Requires Compose-labelled containers |
| Refuse non-local Docker endpoints by default | A configured remote daemon must never be queried silently | Loopback TCP users must pass `--allow-remote-docker` |
| Redact at the presentation boundary | Inspection keeps full detail; every output path shares one policy | A second output path must call the same policy |
| Cap captured output at 8 MiB per stream | Bounds memory against a misbehaving tool | Truncation must be reported, not hidden |

## Extension points

* New runtime or dev-server pattern: extend `runtimeByExe` / `detectDevServer`.
* New collector: implement an interface in `scan` and add a model section.
* New recommendation: add a builder in `analysis` returning the full six-field
  contract, plus tests asserting every field is populated.
* New output format: keep it out of the collectors; render from `model.Snapshot`.

