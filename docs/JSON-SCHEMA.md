# StackBroom JSON schema (version 3)

Machine-readable schema: [`docs/schema/v3.json`](schema/v3.json) (JSON Schema
draft 2020-12). `scan --json` emits a **snapshot**; `plan --json` emits a narrower
**plan document**. Version 2 remains at [`docs/schema/v2.json`](schema/v2.json)
for consumers of the previous contract.

## Version history

| Version | Released | Change |
| --- | --- | --- |
| 1 | v0.1.0 | Initial contract |
| 2 | v0.1.1 | `commandLine` omitted by default; `commandLineOmitted` flag; `redaction` metadata; Docker endpoint/`endpointLocal`/`remoteApproved`; independent reclaim row counts; per-stage `timings`; WSL warnings |
| 3 | v0.2.0 | `network` listener observations; `processes[].requestedPorts` replaces `ports`; `listeningPorts` and `portOwnership`; `duplicateSuspects[].listeners`; `summary.tcpListenerCount`; `timings.networkMs` |

Version 3 is **not** source-compatible with clients that read
`processes[].ports`. Migration steps:

1. Replace `processes[].ports` with `processes[].requestedPorts`. The meaning is
   unchanged: ports found in the command line.
2. Use `processes[].listeningPorts` for ports actually observed bound to that
   PID; do not treat `requestedPorts` as a listener list.
3. Treat `network.available == false` as "listener inspection unavailable", not
   as "no listeners". Check `ipv4` and `ipv6` independently.
4. Read `duplicateSuspects[].listeners` for socket-level evidence; it may include
   a PID outside the group if another process holds the port.
5. Version 2 clients that only read fields that still exist can continue to do
   so, but must not assume `schemaVersion == "2"`.

Version 2 is **not** source-compatible with version 1 clients that expect
`processes[].commandLine`. Consumers must check `redaction.commandLines` and must
treat `commandLine` as optional.

## Privacy semantics

Command lines are inspection data, not output. They can contain passwords, API
tokens, connection strings and private paths, so they are withheld by default.

| Field | Default | `--diagnostics` |
| --- | --- | --- |
| `processes[].commandLine` | absent | present, secrets masked, home path tokenised |
| `processes[].commandLineKnown` | present | present |
| `processes[].commandLineOmitted` | `true` when withheld | absent |
| `processes[].exePath` | present, home tokenised | same |
| `redaction.mode` | `default` | `diagnostics` |
| `redaction.commandLines` | `omitted` | `redacted` |

Masking rules (applied to command lines, evidence, warnings, recommendation text,
subject detail, listener process/project names and listener warnings):

- values after sensitive flags (`--password`, `--token`, `--api-key`, ...);
- sensitive `NAME=value` assignments (`API_TOKEN=...`, `DATABASE_URL=...`);
- URL userinfo passwords (`scheme://user:password@host`);
- bearer tokens;
- the current user's home directory (replaced with `%USERPROFILE%`) and other
  users' profile folders (replaced with `<user>`).

Masking is conservative by design. It cannot catch a secret that does not match a
known pattern (for example an opaque positional argument). Diagnostics output must
still be treated as sensitive.

## Snapshot fields

| Field | Type | Notes |
| --- | --- | --- |
| `schemaVersion` | string | `"3"` |
| `readOnly` | boolean | always `true` |
| `redaction` | object | present in all CLI output |
| `tool` | object | `name`, `version` |
| `generatedAt` | date-time | scan start |
| `durationMs` | integer | total scan time |
| `timings` | object | `processesMs`, `networkMs`, `dockerMs`, `wslMs`, `analysisMs` |
| `host` | object | OS, arch, hostname, physical memory |
| `processes` | array | see below |
| `network` | object | observed TCP listeners, see below |
| `docker` | object | see below |
| `wsl` | object | distributions, VM memory |
| `summary` | object | aggregates with known/unknown flags |
| `duplicateSuspects` | array | port and project groups |
| `recommendations` | array | six-part suggestions |
| `warnings` | array | partial failures and notes |

### Network

| Field | Type | Notes |
| --- | --- | --- |
| `available` | boolean | at least one address family was readable |
| `reason` | string | why inspection failed, or empty |
| `ipv4`, `ipv6` | boolean | each family independently; one can fail while the other succeeds |
| `sampledAt` | date-time | time the table was read |
| `listeners[]` | array | one row per listening socket, never collapsed by port |
| `warnings` | array | family failures, rejected tables, ownership limitations |

Listener rows:

| Field | Type | Notes |
| --- | --- | --- |
| `protocol` | string | `tcp` |
| `family` | string | `ipv4` or `ipv6` |
| `address` | string | local address, unbracketed; link-local IPv6 may include `%scope` |
| `port` | integer | 1-65535 |
| `pid` | integer | owning PID from Windows |
| `processName` | string | optional; associated process name |
| `projectName` | string | optional; associated project name |
| `wildcard` | boolean | `0.0.0.0` or `::` |
| `ownership` | string | `confirmed`, `inferred` or `unknown` |

`ownership` is `confirmed` only when the PID's native creation time matched
before and after listener sampling. `unknown` means the PID was absent from the
snapshot or changed identity; it does **not** mean the process exited or crashed.

### Process

| Field | Type | Notes |
| --- | --- | --- |
| `pid`, `parentPid` | integer | |
| `name`, `parentName`, `exePath` | string | paths tokenised |
| `commandLine` | string | **optional** |
| `commandLineKnown` | boolean | whether a command line was observed |
| `commandLineOmitted` | boolean | withheld by policy |
| `workingSetBytes` | integer | resident working set |
| `memoryKnown` | boolean | `false` means Unknown, never zero-filled |
| `startedAt` | date-time | optional |
| `startTimeKnown` | boolean | |
| `runtime` | string | `node`, `go`, `python`, ... |
| `category` | string | `dev-server`, `dev-tool`, `runtime`, `other` |
| `devServerKind` | string | e.g. `vite`, `uvicorn` |
| `requestedPorts` | integer array | ports found on the command line (renamed from `ports`) |
| `listeningPorts` | integer array | ports actually observed listening for this PID |
| `portOwnership` | string | weakest listener ownership for this process; absent when none |
| `project` | object | `root`, `name`, `marker`, `confidence`, `evidence` |

### Docker

| Field | Type | Notes |
| --- | --- | --- |
| `status.available` | boolean | daemon reachable |
| `status.reason` | string | why it is unavailable, or why an endpoint was refused |
| `status.context` | string | active context name |
| `status.endpoint` | string | resolved endpoint, credentials removed |
| `status.endpointLocal` | boolean | true only for npipe/unix/fd |
| `status.remoteApproved` | boolean | network endpoint used with `--allow-remote-docker` |
| `containers[]` | array | state, status, Compose metadata, one-shot stats |
| `diskUsage[]` | array | Docker's own `size` / `reclaimable` figures |
| `images[]` | array | repository, tag, size, `inUse` |
| `warnings[]` | array | parse failures and truncated output |

A Docker endpoint that is not a local IPC channel is refused unless
`--allow-remote-docker` is passed, so a snapshot can show Docker as unavailable
while the daemon is healthy on another machine.

### Summary accounting

| Field | Meaning |
| --- | --- |
| `devMemoryBytes` | sum of working sets; shared pages may be counted more than once |
| `dockerTotalBytes` | sum of the size figures Docker reported |
| `dockerTotalKnown` | every disk-usage row had a size |
| `dockerReclaimableBytes` | sum of the reclaimable figures Docker reported |
| `dockerReclaimableKnown` | every row had a reclaimable figure (independent of size) |
| `dockerSizeRowsKnown/Total` | size completeness |
| `dockerReclaimRowsKnown/Total` | reclaimable completeness |
| `tcpListenerCount` | every observed listening TCP row, including system services |

`dockerReclaimableBytes` is Docker's estimate of what clean-up could free. It is
not a promise of recovered space and must not be presented as one.

### Recommendation

Every recommendation contains:

`id`, `kind`, `title`, `detected`, `whyItMightBeUnnecessary`, `evidence[]`,
`resource`, `resourceBytes`, `resourceUnknown`, `reviewBeforeActing[]`, `risk`,
`riskDetail`, `confidence`, `subjects[]`.

`risk` is `low`, `medium` or `high`; `confidence` is `confirmed`, `inferred` or
`unknown`. Neither value authorises an action: StackBroom has no cleanup path.

Duplicate groups may also carry `listeners[]` with `family`, `address`, `port`,
`pid` and `ownership`. Confidence is `confirmed` only when at least two
participants have listeners verified against process creation time. Mentioning
the same port on two command lines never raises confidence.

## Plan document

```json
{
  "schemaVersion": "3",
  "readOnly": true,
  "redaction": {
    "mode": "default",
    "commandLines": "omitted",
    "paths": "user-profile-tokenised",
    "secrets": "masked"
  },
  "executesCleanup": false,
  "tool": { "name": "stackbroom", "version": "0.2.0" },
  "generatedAt": "2026-10-27T12:00:00Z",
  "summary": { "...": "same Summary object" },
  "recommendations": [],
  "warnings": []
}
```

## Compatibility rules

- Adding a field is a minor change and does not bump `schemaVersion`.
- Removing, renaming or changing the meaning of a field bumps `schemaVersion`.
- Consumers must ignore unknown fields.
- Consumers must treat `commandLine` as optional and check
  `redaction.commandLines`.
- Consumers must treat missing `network.listeners` rows as unknown, never as
  evidence that a process exited.
- `executesCleanup` is always `false`; any change would be a new contract version
  and a safety review.

## Validation

`internal/model/schema_test.go` checks that the schema is valid JSON, that every
`$ref` resolves, that `schemaVersion` matches the Go constant, that the version-2
schema remains available, and that every required top-level key is actually
emitted by `model.Snapshot`.

