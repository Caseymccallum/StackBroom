# M1 Security Review — Current Status

**Status:** implementation hardened in source and unit-tested; **independent review and live destructive validation are not complete.**
**Branch:** `feat/m1-hardening-product-roadmap`
**Previous reviewed SHA:** `653ab3a` (corrective work has since continued on `feat/m1-process-control`).

This document replaces an earlier self-authored report that incorrectly described itself as an "independent" audit and claimed M1 was ready for acceptance.

## Findings addressed in source

### 1. Duplicate execution

`ConfirmProcessAction` atomically consumes the preview before execution. A second confirmation receives an error. Concurrent consumers are tested (16 goroutines, exactly one succeeds).

### 2. Client-supplied identity

The previous API accepted a creation-time string from the webview. Mapping emitted seconds while native `FILETIME` has 100 ns precision, so valid actions could fail with a false PID-reuse error.

Current design:
- The inspection snapshot issues an opaque HMAC-SHA256 target token binding PID and native creation time.
- Tokens expire after 15 minutes and are bound to the PID.
- The frontend never sends a timestamp.
- Preview re-verifies token signature, expiry, live creation time, protection, critical status and self-protection.

### 3. Fail-open critical check

`IsProcessCritical` errors now fail closed. `openVerifiedProcess` also fails closed when critical status cannot be determined.

### 4. Separate handles / identity race

`openVerifiedProcess` opens the operation handle with the required rights plus `PROCESS_QUERY_LIMITED_INFORMATION | SYNCHRONIZE`, then reads creation time and critical status from that same handle. Mutations are performed on the verified handle.

### 5. False success

- Graceful close distinguishes "not delivered", "delivered but ignored", "verified exit", and "exit could not be verified".
- Opening a new handle is no longer treated as proof of exit.
- Termination success requires `WaitForSingleObject` to report exit.
- Priority changes require readback and report the observed value.
- Failures are returned as `refused`, `partial`, `timeout` or `failure`; no fabricated success.

### 6. Executable path exposure

`GetProcessDetails` no longer returns `exePath`; the path stays internal to identity checks. Tokens contain PID and creation time only.

### 7. Confirmation and accessibility

- `window.confirm` removed.
- In-app confirmation names the target, action and consequence.
- Dialog has `role="dialog"`, `aria-modal`, labelled title/description, Escape, focus trap and focus restoration.
- Duplicate submits are blocked while a request is in flight.
- Server `outcome`/`details`/observed values are displayed verbatim.
- Process table row details are keyboard-focusable buttons.

## Remaining limitations requiring independent review

- No live destructive test has been run. `notepad.exe` validation awaits owner authorisation.
- Protected/critical behaviour has been verified through deterministic fakes, not through live system processes.
- `IsProcessCritical` requires a supported Windows version; unsupported hosts fail closed.
- Session ID is not part of the target token.
- The desktop screenshots in `docs/desktop/screenshots` predate the new confirmation UI and must be recaptured for any release claim.
- A prior self-review is not independent validation.

## Verification commands

```powershell
go test ./... -count=1
cd apps/desktop
go test ./... -count=1
cd frontend
npx tsc --noEmit
npm test
```

## Next gate

1. Independent code review of `internal/processcontrol` and `service_process.go`.
2. Owner-authorised live test with a disposable `notepad.exe`.
3. Accessibility and screenshot recapture.
4. Only then update the M1 acceptance statement.
