# M3B-1 Cleanup Engine — Implementation Notes

**Status:** architecture implemented, execution disabled in ordinary builds, no live destructive test.

## Native execution boundary

`internal/cleanup/execution` owns the only mutation-shaped interface:

```text
Service ConfirmCleanup(preview ID)
  -> native-only execution.Context (exact FileRecords, single use, expiry)
  -> Engine (rule, root, eligibility, recovery gate, state machine)
  -> Executor (ordinary build: disabled; tagged Windows build: IFileOperation)
  -> post-operation presence + recovery evidence
  -> privacy-filtered history
```

The frontend never sends a path, a `FileRecord`, a root or a target identity. It receives only a preview ID and an execution result.

## Execution context

- Exact `storage.FileRecord` copies, including file ID, volume identity, size, write time, link count and root.
- Rule ID and version.
- Trusted root.
- Recycle-only permission.
- Expiry (same 10-minute preview window).
- Atomic `Consume(now)`; a second confirmation fails.
- No frontend-facing paths; execution results expose target IDs, states and reason codes only.

## Eligibility and fail-closed behaviour

Before the executor is called, the engine re-checks:

- rule exists at the previewed version;
- rule is recycle-only;
- application-close prerequisites cannot be verified, so rules with `RequiresClosed` refuse execution;
- trusted known-folder root still resolves to the context root;
- a recovery verifier is available;
- every target passes `storage.ValidateCleanupTarget`;
- extension and age still match the rule.

One invalid target aborts the whole operation before `IFileOperation`; no best-effort partial mutation is allowed at validation time.

`ValidateCleanupTarget` refuses missing file IDs, hard links, reparse points, placeholders, hidden/system files, non-regular files, size/write-time changes, identity changes and out-of-root paths.

## Windows COM worker

The Windows implementation is compiled only with:

```text
go build -tags cleanup_execution
```

It is therefore absent from ordinary production builds.

The worker:

- locks the OS thread and enters a single-threaded COM apartment;
- creates `IFileOperation` with `CLSID_FileOperation` / `IID_IFileOperation`;
- sets `FOF_ALLOWUNDO | FOFX_ADDUNDORECORD | FOFX_RECYCLEONDELETE` plus non-interactive error controls;
- queues one `IShellItem` per target using `SHCreateItemFromParsingName`;
- calls `PerformOperations` once;
- reads `GetAnyOperationsAborted`;
- releases every COM interface and uninitialises the apartment on all exits.

The vtable indices and GUIDs are source-reviewed but have not been exercised in a live operation. The tagged build is for compile/review only until authorised testing.

## Recovery semantics

The engine records five separate quantities:

1. logical bytes selected;
2. allocated bytes measured;
3. estimated immediate reclaim (preview estimate);
4. verified disk-space change (not yet implemented; unknown);
5. items actually present in a recoverable location (verification evidence).

A file disappearing from its original path is not success. If the verifier is unavailable or returns no evidence, the item is `removed_unverified` and the overall state cannot be `completed`.

The current native recovery verifier reports unavailable. Therefore even a tagged build refuses before calling the executor. A future `IFileOperationProgressSink` or Recycle Bin `$I` verifier is required before execution can be enabled.

## State machine

```text
previewed -> confirmed -> validating -> executing -> verifying
                                              |-> completed
                                              |-> partial
                                              |-> failed
                                              |-> cancelled
```

`completed` requires every selected item to be absent from its original path and accompanied by recovery evidence. API return codes alone are never success.

## Action history

`internal/history` is a local JSONL journal. It records timestamp, type, rule ID/version, state, counts, logical/allocated bytes, mode, reason code and recovery availability. It does not record paths, names, file contents, command lines or credentials. Retention defaults to 30 days / 500 records; `Clear` removes the journal.

No record is written for previews or for execution attempts that never occur.

## Feature gate

Two independent layers:

1. **Compile time:** `execution.BuildEnabled()` is `false` unless built with `cleanup_execution`.
2. **Runtime:** `STACKBROOM_ENABLE_CLEANUP_EXECUTION=1` is also required.

`ConfirmCleanup` checks both. A test sets the environment variable in an ordinary build and confirms the request is refused. Hiding the UI button is not a security control.

## Filesystem race limitations

`IFileOperation` receives a path-derived `IShellItem`; it does not use the validated metadata handle. Windows does not provide a portable handle-relative Shell delete here. The engine revalidates immediately before execution and refuses reparse ancestors, but a concurrent actor can still substitute an ancestor between validation and Shell processing. This is not described as race-safe.

First execution-enabled release must either:

- implement a handle-identity verification mechanism accepted by security review, or
- refuse targets whose ancestor chain cannot be proven stable.

## Authorisation gate

Real deletion remains disabled. Before enabling:

1. Independent security review.
2. Verified Recycle Bin evidence/restoration design.
3. Owner-authorised tests on deliberately created disposable files.
4. Confirmation that Recycle Bin policy cannot silently fall back to permanent deletion.
5. A separate build with an explicit execution tag and backend gate.
