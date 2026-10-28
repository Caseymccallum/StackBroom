# Action Security Architecture

M3 applies this model through the rule-bound design in [`M3-CLEANUP-SPEC.md`](M3-CLEANUP-SPEC.md); no deletion capability is implemented in M2.

## Scope

This document describes how StackBroom authorises system-changing actions. It applies to process control today and is the reusable pattern for startup, cleanup, file locks and application management.

## Trust boundary

```text
React webview (untrusted input)
        |
        | PID + opaque target token + intent
        v
Wails service bridge (validates, never trusts identity claims)
        |
        v
processcontrol package (verify, preview, execute, verify outcome)
        |
        v
Windows APIs
```

The webview is not a security boundary. It may be compromised or scripted. The service must remain safe even if every value it receives is hostile.

## Target identity

- A token is issued by the backend when a process snapshot is produced. It binds the PID to the native `FILETIME` creation time observed by the inspection engine and carries an expiry.
- Tokens are HMAC-SHA256 protected with a process-local random key. They are opaque to the frontend.
- The frontend never sends a timestamp and cannot choose a target. PID reuse after the snapshot causes verification to fail closed.
- Tokens expire after 15 minutes. An expired token requires a fresh scan.

## Preview / confirm lifecycle

1. The service verifies the token signature, PID binding, expiry and live creation time.
2. It re-checks protection, critical status and self-protection.
3. It stores an immutable preview with a 128-bit random operation ID and a 60-second expiry.
4. Confirmation atomically consumes the preview. A second confirmation is refused.
5. Execution re-verifies identity on the **same handle** used to perform the operation.
6. The service reports only observed outcomes: success, timeout, partial, refused or failure. Delivery of a request is never reported as success.

## Protection policy

Denied regardless of token:

- Critical processes reported by `IsProcessCritical`.
- Protected system names (defence in depth).
- StackBroom's own process.
- Real-time priority.
- Any inability to determine critical status (fail closed).

## Handle correctness

Operations open the process with the required rights plus `PROCESS_QUERY_LIMITED_INFORMATION | SYNCHRONIZE`, read creation time and critical status from that handle, then act. Separate handles are not used to prove the identity of a mutation.

## Privacy

- `exePath` is internal to identity checks and is never returned to the webview.
- The bridge returns only name, PID, protected/critical flags and action outcome.
- Tokens contain PID and creation time, not executable paths or command lines.

## History

Action history records operation type, target display name, timestamp, requested/observed values, outcome and refusal reason. It does not record executable paths, command lines or file contents. It is local and retention-controlled.

## Review requirements

Every action capability requires refusal-path tests, concurrent confirmation tests, stale-identity tests, protected-target tests and an explicit statement of what is irreversible.
