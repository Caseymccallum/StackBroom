# Safety, security and limitations

StackBroom v0.2 is **read-only**. This document states the safety contract, how
it is enforced, what the measurements mean, and what a future cleanup feature
would require before it could exist.

## The v0.2 safety contract

StackBroom v0.2 must not, and cannot:

- kill, suspend or signal a process;
- stop, start, restart, remove or pause a Docker container;
- remove images, build cache, volumes or networks;
- run `docker prune` in any form;
- start Docker Desktop or any Docker service;
- start, stop, terminate, restart or export a WSL distribution;
- execute commands inside a WSL distribution;
- modify registry keys, services, environment variables or files;
- run anything automatically at startup, on a timer, or in the background.

The CLI has exactly two data commands: `scan` and `plan`. Both inspect and print.

## How the contract is enforced

1. **No cleanup code path exists.** There is no "apply" command, no confirmation
   prompt, no hidden flag. `plan --json` even carries `"executesCleanup": false`.
2. **A fixed command allowlist.** The only external commands StackBroom can run
   are listed below. Nothing is constructed from observed data.
3. **No shell.** `internal/run` uses `exec.Command` with an argument vector.
   Process names, paths, container labels and command lines are never
   concatenated into a command string, so shell metacharacters are inert.
4. **Read-only Windows APIs.** Process inspection uses `CreateToolhelp32Snapshot`,
   `OpenProcess` with `PROCESS_QUERY_LIMITED_INFORMATION`,
   `QueryFullProcessImageNameW`, `GetProcessTimes` and `GetProcessMemoryInfo`.
   Listener inspection uses `GetExtendedTcpTable` with
   `TCP_TABLE_OWNER_PID_LISTENER`. No socket is opened, no packet is sent, no
   firewall rule is read or changed, and no process-termination or
   thread-injection API is imported. No elevation is requested.
5. **Tests assert the property.** Docker and WSL collector tests record every
   invocation and fail if a mutating verb (`prune`, `rm`, `stop`, `kill`, `start`,
   `--terminate`, `--exec`) appears. Recommendation tests fail if any text implies
   an automatic action.
6. **Bounded output rendering.** Observed strings are sanitised for terminal
   control characters and truncated before printing, so a hostile process name
   cannot rewrite the terminal.
7. **Timeouts on everything.** No external command can hang the scan.

## External commands (complete list)

| Tool | Invocation | Purpose | When |
| --- | --- | --- | --- |
| Docker CLI | `docker version --format {{.Server.Version}}` | Availability probe | Unless `--skip-docker`, and only after the endpoint is approved |
| Docker CLI | `docker context show` | Read the active context name (local metadata only) | Before any daemon command |
| Docker CLI | `docker context inspect <name>` | Read the context's endpoint (local metadata only) | Before any daemon command |
| Docker CLI | `docker ps -a --no-trunc --format {{json .}}` | Containers | If Docker is available |
| Docker CLI | `docker stats --no-stream --format {{json .}}` | CPU/memory of **running** containers | Only when at least one container runs |
| Docker CLI | `docker system df --format {{json .}}` | Storage categories | If Docker is available |
| Docker CLI | `docker system df` | Storage fallback for older Docker | Only if the JSON form yields nothing |
| Docker CLI | `docker image ls --no-trunc --format {{json .}}` | Image inventory | If Docker is available |
| WSL | `wsl.exe --list --verbose` | Distributions and state | Unless `--skip-wsl` |
| PowerShell | `powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -Command <fixed script>` | Read command lines, start times and a memory fallback via CIM | Unless `--no-command-lines` |

Notes:

- `-ExecutionPolicy Bypass` applies to that single PowerShell process. It does not
  read or write machine or user execution policy.
- The PowerShell script is a compile-time constant:
  `Get-CimInstance -ClassName Win32_Process | Select-Object ... | ConvertTo-Json`.
  It contains no interpolation and no mutating verb.
- If PowerShell is blocked by policy, the scan continues without command lines and
  says so in its warnings.
- StackBroom never invokes `docker run`, `docker start`, `docker desktop start`,
  `wsl --terminate`, `wsl --shutdown`, `taskkill`, `Stop-Process` or `sc`.

## Limits and resource bounds

| Bound | Value | Rationale |
| --- | --- | --- |
| Per-command timeout | 20 s default, configurable with `--timeout` | A wedged Docker daemon cannot hang the scan |
| Wait delay after kill | 2 s | Guarantees inherited pipes close |
| Image list | 40 largest images (reported as truncated) | Keeps output readable |
| Container list | All containers, but text output shows 10 stopped | Avoids a wall of output |
| Docker stats | `--no-stream` only | One measurement; never watches |
| Project search depth | 12 directory levels, never a volume root | Bounds filesystem work |
| Process list | All processes | Needed for accurate totals |

## Handling untrusted data

Process names, command lines, executable paths, Docker labels, container names,
image tags, distribution names and command output are all treated as untrusted
text.

- They are parsed, never executed.
- They are never used as command names, arguments to a shell, file paths for
  writing, or format strings.
- Text output replaces control characters and caps length (`sanitize`), preventing
  ANSI escape injection into a terminal.
- JSON output escapes control characters per the JSON standard; consumers should
  still treat strings as data.
- No files are written by StackBroom, so observed paths cannot cause file
  overwrites.
- StackBroom contacts no service of its own. Docker is reached over local IPC by
  default; with `--allow-remote-docker`, read-only commands go to the configured
  daemon over the network instead. Nothing is sent to StackBroom or a third party.

## Evidence rules (why nothing is called "unnecessary")

A recommendation is a prompt to look, not a verdict. The rules deliberately
reject weak signals:

- **Age is not evidence.** A development server running for 12 hours may be
  serving a long test session, a demo or another machine. Age only triggers a
  review prompt, and the text says so explicitly.
- **Memory use is not evidence.** A 900 MB process may be the most important one
  on the machine. Memory only affects ordering and how a suggestion is worded.
- **Idle-looking is not measurable here.** StackBroom does not read CPU time
  deltas, so it never claims a process is idle.
- **Requested port is not ownership.** Two command lines can request port 5173
  while only one holds it. Duplicate suspects are labelled `inferred` and the
  evidence says the listening socket was not checked.
- **A project match needs a manifest.** Package caches, `node_modules`, temp
  directories and Program Files are excluded because they contain manifests that
  are not the developer's project.

Confidence in a recommendation describes the reliability of the *detected facts*
(Docker's own state, a supported project match, matching command-line flags). It
never asserts that cleanup is correct.

## Risk levels

| Level | Meaning | Examples |
| --- | --- | --- |
| Low | Data-safe; the cost is time or bandwidth | Reclaiming Docker build cache |
| Medium | Reversible with effort; may interrupt work or need re-downloading | Removing unused images; stopping a duplicate server; removing a stopped container |
| High | Destroys live state or is hard to undo | Terminating a running WSL distribution; any future volume deletion |

Risk is derived from the operation, not from how much space it frees.

## Measurement caveats

| Figure | Caveat |
| --- | --- |
| Process working set | Resident memory, not private bytes. Shared pages are counted per process, so a sum over processes can exceed physical RAM |
| Process age | Requires CIM. If unavailable, age is `Unknown` |
| Docker "SIZE" vs "RECLAIMABLE" | Two different Docker reports. Reclaimable is an estimate of what pruning could free, not space already freed |
| Docker image sizes | Rounded by Docker; identical layers shared between images are counted in each image's size |
| Container CPU / memory | A single `docker stats` sample, not an average |
| WSL VM memory | `vmmem` working set covers the whole utility VM and can double-count memory also attributed to guest processes. It is not per-distribution |
| Per-distribution memory | Not measured. Would require executing commands inside the distribution |
| Disk space | StackBroom does not stat the Docker data directory; it reports Docker's own accounting |

## Failure modes and behaviour

| Situation | Behaviour |
| --- | --- |
| Docker CLI missing | Docker section says unavailable with "not found on PATH"; scan succeeds |
| Docker daemon stopped | Says the daemon is not reachable and that StackBroom did not start it; scan succeeds |
| Docker permission denied | Reports the access error; no retry with elevation |
| Docker output malformed | Skips the offending line, adds a warning, keeps the rest |
| `docker stats` fails | Container list still shown; usage marked Unknown |
| WSL not installed | WSL section says unavailable; scan succeeds |
| WSL has no distributions | Reported as available with an empty list |
| PowerShell blocked | Command lines, ages and roles unavailable; explicit warning; scan succeeds |
| Process protected by Windows | Memory `Unknown`; process still listed |
| CIM date in an unknown format | Age `Unknown` rather than a guess |
| Command times out | That section degrades; the scan completes |

## Privacy

- No telemetry, analytics, update checks or crash reporting.
- No network access by default. With `--allow-remote-docker`, read-only Docker
  commands go to the configured daemon over the network; nothing is sent to
  StackBroom or any third party.
- Reports contain local paths and, in diagnostics mode, command lines. Treat saved
  JSON as sensitive: it can reveal usernames, project names and arguments.
- StackBroom writes no files unless you redirect its output yourself.

## What a future cleanup feature would require

Cleanup is **not** part of v0.2 and must not be added without a separate safety
review. A future implementation should, at minimum:

1. Present a specific action with its exact consequence and the evidence.
2. Require explicit per-action approval (no blanket "clean everything").
3. Never target volumes, bind mounts or user files in the first iteration.
4. Prefer reversible actions and document how to recover.
5. Verify identity before acting (PID + start time + executable, to avoid PID
   reuse; container ID + name).
6. Re-check state immediately before acting, and abort if it changed.
7. Log what was done, when, and the observed before/after state, locally.
8. Refuse to act on protected or system processes, or when running without the
   expected privileges.
9. Add tests that prove unapproved targets are never touched.
10. Keep the default configuration read-only.

## Reporting a safety issue

If you find a way for StackBroom to change system state, treat it as a security
bug: do not publish exploit details before the maintainer has responded. Include
the command line, the observed effect and the version (`stackbroom version`).


## Remote Docker endpoints

Docker's CLI can point at another machine through a context or environment
variable. StackBroom must not contact a remote daemon just because the CLI is
configured that way.

**Resolution order** (Docker CLI reference, checked 27 October 2026):

1. `DOCKER_CONTEXT` selects a context. For a named context, its endpoint takes
   precedence over `DOCKER_HOST`.
2. The special `default` context derives its endpoint from `DOCKER_HOST`, so
   `docker context inspect default` reports that value.
3. Otherwise the active context (`docker context use`) applies.

StackBroom reads the context name and endpoint with `docker context show` and
`docker context inspect`, which read local metadata and do not contact a daemon.
It classifies whatever endpoint Docker reports, so the refusal follows the CLI's
effective target even when these rules interact.

Endpoint classification:

| Endpoint | Classification | Default behaviour |
| --- | --- | --- |
| `npipe:////./pipe/<name>` | local named pipe (canonical Docker form) | allowed |
| `npipe:////<host>/pipe/<name>` | UNC pipe to another machine | **invalid; never contacted** |
| any other `npipe:` form | malformed or ambiguous | **invalid; never contacted** |
| `unix:///absolute/path` | local Unix socket | allowed |
| `unix:` with a relative or empty path | malformed | **invalid; never contacted** |
| `fd://<number>` | local file descriptor | allowed |
| `tcp://...` (including 127.0.0.1 and localhost) | network | refused; requires `--allow-remote-docker` |
| `ssh://...` | network | refused; requires `--allow-remote-docker` |
| `http://`, `https://` | not a Docker daemon transport | **invalid; never contacted** |
| unparseable, empty, missing host, unsupported scheme | unknown | **invalid; never contacted** |

Only the canonical local pipe form is classified as local. Host-qualified UNC
pipes address another machine over SMB and are rejected outright: StackBroom does
not support remote named pipes, so they are never contacted, even with
`--allow-remote-docker`. A malformed pipe is also never contacted. Remote daemons
remain available through `tcp://` or `ssh://` with explicit approval, but that
approval requires a successfully resolved, valid endpoint.

Loopback TCP is refused because a local port can be an SSH tunnel to a remote
daemon. Refusal is explicit: the Docker section reports the endpoint, the reason
and `--allow-remote-docker`; no daemon command is issued.

`--allow-remote-docker` permits the same read-only commands against a network
endpoint, adds a warning to the output and sets `status.remoteApproved`. It does
not enable any mutating command, because none exist.

## What TCP listener evidence proves

A listener row proves that, at the sample moment, Windows had a TCP socket in the
LISTEN state with that local address, port and owning PID. It does **not** prove:

- that the owning process is a development server (that comes from process
  classification);
- that a process requesting the same port in its command line is listening;
- that either process is redundant, idle or safe to stop;
- why a requested port has no listener. It may be overridden by config or an
  environment variable, may not have started yet, or may be unable to bind.

Ownership levels:

| Level | Meaning |
| --- | --- |
| `confirmed` | The PID was in the process snapshot and its native creation time matched when re-read after the TCP sample. |
| `inferred` | The PID matched by number, but a creation time was unavailable or could not be re-read. PID reuse was not ruled out. |
| `unknown` | The PID was absent from the snapshot, had no usable PID, or changed identity. The listener is reported without a process/project association. |

An `unknown` or `inferred` owner never means the process exited, crashed or
failed. Missing listeners are described as "not observed", not as failures.

Wildcard addresses (`0.0.0.0`, `::`) are local bindings on all interfaces, not
remote endpoints. IPv4 and IPv6 tables are independent; one can fail while the
other succeeds, and the result reports each family separately. UDP is not
inspected. No remote host is queried.

The process snapshot and the TCP table are sampled at different moments. A
listener can disappear or appear between them, and a PID can be reused. StackBroom
keeps both timestamps and the race limitation explicit rather than asserting a
causal story.

## Output privacy and redaction

Command lines are inspection input, not output. They routinely contain tokens,
passwords, connection strings, usernames and private paths.

| Mode | Command lines | Paths | Secrets |
| --- | --- | --- | --- |
| Default (no flag) | omitted; `commandLineOmitted: true` | home replaced with `%USERPROFILE%`, other profiles with `<user>` | masked |
| `--diagnostics` | present, secrets masked, paths tokenised | same | masked |

The same redaction is applied to project evidence, duplicate evidence,
recommendation text, subject detail, listener process/project names and scan
warnings — not just the process table — because evidence is assembled from
observed strings. Listener rows never include command lines. Local socket
addresses and ports are reported as observations; they are not secrets, but they
can reveal local network layout, so JSON output should still be handled as
sensitive. Diagnostics prints a
warning to stderr and adds one to `warnings`.

Masking covers sensitive flag values, sensitive `NAME=value` assignments, URL
userinfo passwords and bearer tokens. It cannot recognise an arbitrary secret
passed positionally, so diagnostics output must still be treated as sensitive.

Text output additionally replaces control characters, so a hostile process name
cannot inject ANSI escapes.

## Bounded command output

Each external command retains at most 8 MiB of stdout and 8 MiB of stderr. Output
beyond that is drained and discarded, and `Result.Truncated` is set. Docker and WSL
collectors turn truncation into a warning rather than presenting a partial list as
complete. This bounds memory even if a tool or daemon misbehaves.

