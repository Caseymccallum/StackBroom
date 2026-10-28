# M1: Process Control — Research and Design

## Windows APIs

### Process Identity Verification

- `OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION)` — minimal access for verification
- `GetProcessTimes` — native creation time for PID-reuse detection
- `QueryFullProcessImageName` — executable path for additional identity
- `CloseHandle` — mandatory cleanup

### Graceful Close

- `EnumWindows` + `GetWindowThreadProcessId` — find windows owned by target PID
- `PostMessage(hwnd, WM_CLOSE, 0, 0)` — request graceful shutdown
- `WaitForSingleObject(process, timeout)` — observe outcome
- Returns: success, timeout (ignored close), or failure

### Forced Termination

- `OpenProcess(PROCESS_TERMINATE)` — explicit termination right
- `TerminateProcess(process, exitCode)` — force kill
- Requires elevated privileges for some processes
- Returns: success, access denied, or failure

### Priority Control

- `OpenProcess(PROCESS_SET_INFORMATION)` — required for priority changes
- `GetPriorityClass` — readback verification
- `SetPriorityClass` — limited to supported classes:
  - `IDLE_PRIORITY_CLASS`
  - `BELOW_NORMAL_PRIORITY_CLASS`
  - `NORMAL_PRIORITY_CLASS`
  - `ABOVE_NORMAL_PRIORITY_CLASS`
  - `HIGH_PRIORITY_CLASS`
- **NOT exposed**: `REALTIME_PRIORITY_CLASS` (requires special privileges, can destabilize system)

### Protected Process Detection

- `IsProcessCritical` (Windows 8.1+) — detect critical system processes
- Check process name against known protected list (defence-in-depth)
- Check protection level via `NtQueryInformationProcess` (advanced)
- Refuse operations on:
  - System Idle Process (PID 0)
  - System process (PID 4)
  - smss.exe, csrss.exe, wininit.exe, winlogon.exe, lsass.exe, svchost.exe (critical)
  - Protected processes (PPL)
  - StackBroom itself and children

## Security Model

### Preview/Confirm/Execute Pattern

1. **Preview**: User selects process, requests action
2. **Backend validates**: PID exists, creation time matches, not protected
3. **Frontend shows**: Exact operation, risks, consequences
4. **User confirms**: Explicit approval
5. **Backend re-validates**: PID still matches (race protection)
6. **Execute**: Perform operation
7. **Verify**: Observe actual outcome
8. **Report**: Success/partial/failure with evidence

### Operation Identifiers

- Each preview generates a unique `operationId`
- Backend stores preview with expiry (60 seconds)
- Confirmation must reference valid `operationId`
- Frontend cannot substitute PID or action type
- Single-use: operation invalidated after execution

### Refusal Policy

Refuse if:
- PID does not exist
- Creation time mismatch (PID reuse)
- Process is protected/critical
- Process is StackBroom or child
- Insufficient privileges
- Operation expired
- Invalid operation ID

## Testing Strategy

### Unit Tests

- Mock process handles for deterministic testing
- Test refusal cases (protected, stale, invalid)
- Test priority readback
- Test handle cleanup

### Integration Tests

- Use disposable test processes (e.g., `notepad.exe`, `calc.exe`)
- Never terminate real user processes
- Verify actual process state after operations
- Test graceful close with responsive apps
- Test forced termination with unresponsive apps

### Fixture Processes

```go
// Start disposable process for testing
cmd := exec.Command("notepad.exe")
cmd.Start()
pid := cmd.Process.Pid
// ... test operations ...
// Cleanup: ensure process terminated
```

## Limitations

- Cannot terminate processes requiring elevation without admin rights
- Cannot modify protected processes (PPL)
- Priority changes do not guarantee CPU allocation
- Graceful close depends on application message loop
- Some processes ignore WM_CLOSE
- Windows may restart critical services automatically

## Next Steps

1. Implement `internal/processcontrol/` package
2. Add Wails bridge methods
3. Build process details drawer UI
4. Add confirmation dialogs
5. Write tests with disposable fixtures
6. Document in VALIDATION.md
