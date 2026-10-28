# M3B Security Review and Threat Model

**Review type:** source-verifiable self-review. This is not an independent security review and cannot authorise live deletion.

No separate review artifact was present in the repository at the starting SHA. The four findings below were reproduced directly from the M3A source and are retained as the review baseline.

## Four findings

### F1 — Automatic selection of every candidate

**Finding:** `Cleaning.tsx` selected every candidate automatically when discovery completed. A user could build a preview containing all files without a deliberate per-candidate decision.

**Evidence:** the `useEffect` that populated `selectedIDs` from every candidate.

**Resolution:** selection now starts empty. The user must select each row or explicitly use **Select all eligible**. Per-item exclusions are never added by the bulk action because it only reads the bounded eligible candidate list.

**Tests:** `Cleaning.test.tsx` asserts an unchecked candidate and a disabled preview action before selection.

### F2 — Preview view was the only stored object

**Finding:** `cleanupPreviewState` stored only `CleanupPreviewView`. There was no separate native execution context, leaving a future executor able to reconstruct targets from bridge-facing fields.

**Evidence:** `service_cleanup.go` created a view, stored the view and an expiry, and had no execution object.

**Resolution:** `internal/cleanup/execution.Context` stores exact native `storage.FileRecord` identities, rule ID/version, trusted root, selected target IDs, permission scope and expiry. It is immutable after construction and consumed atomically with `Consume(now)`. The service stores it beside, not inside, the view. The frontend cannot construct or mutate it.

**Tests:** engine tests exercise single consumption and frontend arbitrary-path rejection.

### F3 — Environment-expanded roots were trusted

**Finding:** `Rule.ResolveRoot` expanded `%TEMP%` and `%LOCALAPPDATA%` from the process environment and checked only that the result was an absolute path. A process or launcher that changed those variables could redirect discovery and preview outside the intended user scope.

**Evidence:** `expandWindowsEnv` followed by `filepath.Abs`; no known-folder comparison.

**Resolution:** each rule token is resolved against a Windows known folder (`FOLDERID_LocalAppData`) and the environment-expanded path must exactly equal the trusted resolved scope. Denied system roots are refused. A test sets `TEMP` to a different directory and expects `ResolveRoot` to fail.

**Tests:** `TestEnvironmentOverrideOutsideTrustedScopeIsRefused`.

### F4 — Eligibility was incomplete and identity could be uncertain

**Evidence:** M3A preview used `storage.ValidateFileRecord`, which permitted an empty file ID and did not recheck hidden/system status. Age plus a few metadata checks was treated as sufficient.

**Resolution:** `storage.ValidateCleanupTarget` fails closed on missing identity, non-regular files, hard links, reparse points, placeholders, hidden/system files, size/write-time changes and identity changes. Preview and execution re-check rule version, trusted root, extension and age. An uncertain identity can never become an execution target.

**Tests:** `TestValidateCleanupTargetFailsClosed`, `TestValidateCleanupTargetRejectsHiddenFile`, execution changed-target tests.

## Threat model

| Threat | Current control | Residual |
|---|---|---|
| Hostile webview submits a path | Execution accepts only an opaque preview ID; native context is backend-owned | Path injection is blocked; the webview remains untrusted |
| Env override redirects scope | Known-folder equality and denied-root policy | A compromised process environment before app start still cannot make arbitrary paths trusted |
| Target replaced after preview | `ValidateCleanupTarget` immediately before executor; fail closed; no partial execution | Path-based validation and Shell path operation leave a TOCTOU window |
| Parent directory replaced by junction | Reparse-ancestor and resolved-path checks | Cannot prove the Shell used the validated handle |
| Hard link | Refused at preview and execution | None accepted |
| Cloud placeholder | Refused; no hydration | None accepted |
| Recovery not established | Engine requires `RecoveryVerifier.Available`; unavailable verifier refuses before execution | Native verifier is not implemented, so real execution cannot proceed |
| Silent permanent fallback | `FOF_ALLOWUNDO` + `FOFX_RECYCLEONDELETE`; verifier gate | Shell behaviour for oversized/unsupported files is not live-tested; execution remains disabled |
| Duplicate confirmation | Single-use native context; service checks expiry | Concurrent calls race to the engine, but only one `Consume` succeeds |
| Partial failure misreported | Per-item post-operation presence and evidence; completed requires every item verified | Actual COM result has not been live-tested |
| Privilege escalation | No elevation requested; normal user integrity; protected roots denied | Shell policy and ACL behaviour need owner-authorised tests |
| History leaks paths | History records counts, rule IDs, reason codes and state only | File name/path omission reduces forensic detail by design |

## Disabled-capability proof

- Ordinary builds use `//go:build !cleanup_execution`; `execution.BuildEnabled()` is a compile-time `false`.
- The Windows `IFileOperation` source file is compiled only with `cleanup_execution` and `windows` tags.
- `ConfirmCleanup` checks `execution.BuildEnabled()` first and then the exact backend environment gate `STACKBROOM_ENABLE_CLEANUP_EXECUTION=1`.
- A test sets the environment variable to `1` in an ordinary build and confirms execution is still refused.
- The native recovery verifier currently reports unavailable; even a tagged build refuses before calling the executor.
- No permanent-deletion mode, generic delete method, `DeleteFiles`, `MoveToRecycleBin` or `CleanupFiles` exists.

## Authorisation still required

1. Independent security review of the engine and COM worker.
2. Owner-authorised live test on deliberately created disposable files only.
3. Verification of Recycle Bin evidence and restoration, including oversized/unsupported files.
4. Confirmation that the target environment is Windows 10 1809+/Windows 11 and that known-folder resolution behaves as expected.
5. Separate approval before compiling a build with `-tags cleanup_execution` and setting the environment gate.

