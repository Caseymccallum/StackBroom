# M3B-2 safety hardening and handoff

Date: 28 October 2026. Base: `5398553caa8ec9d2362ee30a94ecdf673353a8cd`.
Branch: `feat/m3b-2-safety-hardening`.

## Status and evidence boundaries

Safety hardening implemented; live cleanup and restoration remain blocked.
This is an implementation and self-review, not independent security acceptance.
The previous conversation's M3B-1 attachment was treated as a self-report. Its
performance, binary hash, tests, and completion claims were not adopted as new
evidence. Current source, current remote refs, local SDK declarations, and the
checks below were inspected directly in this run.

GitHub's branches API and a direct remote-head query both confirmed the base
branch still pointed to the requested SHA before publication. A separate local
clone was used; the original checkout was left untouched.

No Shell deletion/restoration, destructive process test, app launch, screenshot,
fullscreen action, merge, tag, release, or binary publication occurred.
Existing deterministic tests use disposable files and fake executors; some
simulate disappearance by removing only their own fixture files. They do not
exercise Windows Recycle Bin behavior. Process-control tests use injected fakes.
Live inspection opt-ins were not enabled.

## Source audit and corrections

The original IFileOperation IID, CLSID, IShellItem IID, and numeric slots were
correct. An initial progress message incorrectly called the IID/slots defective;
this was corrected after reading the full SDK declaration. There is no ABI
correction claim for values that were already right.

| Finding | Change |
|---|---|
| Perform failure skipped aborted-status query | Always query after Perform, retain both outcomes, treat unreadable or unsupported query status as aborted/uncertain |
| HRESULT comparisons treated every nonzero value as failure | Use signed 32-bit HRESULT failure semantics; abort-query output requires documented S_OK |
| Queue acceptance was reported as completion | Count PostDeleteItem callbacks only; store actual per-item HRESULT; duplicate callbacks fail closed |
| No native callback cancellation | Implement all 19 IFileOperationProgressSink slots, cancellation at pre-delete/update/timer/start callbacks, reject unexpected operation types |
| Service ran with Background and no cancel endpoint | Ten-minute context, one active execution, CancelCleanupExecution, progress lookup, shutdown cancellation, bounded progress records |
| Display names could differ from actual path extension | Require basename consistency; recheck canonical path extension, age, depth, candidate limit and root |
| View slices escaped backend storage | Clone all preview slices on storage and retrieval; native Context remains copied and single-use |
| Executor error could be hidden by later evidence | Completion also requires no executor error; retain HRESULT and failure details |
| Recovery success accepted a bare boolean | Require nonempty provider kind/token and matching successful executor item as well as independent evidence |
| History strings were not actually privacy-filtered | Closed vocabularies on write and read, read-time retention, refuse to overwrite unreadable/malformed history |

COM calls now use typed interface/vtable pointers rather than uintptr-to-pointer
arithmetic. Callback objects are pinned, kept in a registry, implement
QueryInterface/AddRef/Release, and retain synthetic/native recycle item references
only within their apartment lifetime. The ordinary build excludes this worker
and sink. The native entry point additionally checks runtime opt-in and recovery
availability before COM activation, defending against a direct engine bypass.

The SDK contract test reads the installed header and compares the complete
IFileOperation method order and selected GUIDs with implementation values. Slots
include IUnknown: flags 5, DeleteItem 18, PerformOperations 21, aborted query 22.
Progress-sink method signatures follow the SDK declaration, with exact callback
arity, including the eight-argument PostNewItem entry.

## Ancestor mitigation and residual race

GuardCleanupAncestors opens every directory from the local drive root through the
target parent without write/delete sharing, rejects reparse/cloud attributes, and
holds the handles through Shell processing. Each target is revalidated after
guard acquisition and again in PreDeleteItem. Non-drive-local paths fail closed.

A non-mutating access test demonstrated that attribute-only directory handles
were insufficient. Adding FILE_LIST_DIRECTORY made a concurrent DELETE-access
open fail with ERROR_SHARING_VIOLATION. This test requests access only; it does not
rename or remove the guarded directory.

This mitigates ancestor replacement; it does not prove complete TOCTOU safety.
The final file is still bound by Shell using a path and cannot simply be held
without delete sharing while allowing Shell to recycle it. A final-file swap or
content change between the callback's validation and Shell's action remains an
unresolved blocker. Guards also do not prove safety against privileged actors,
all filesystems, or filter drivers. No adversarial live substitution test ran.

## Recovery evidence and restoration architecture

Microsoft documents PostDeleteItem's actual deletion HRESULT separately from
queue acceptance and identifies a non-null newly-created item as the recycled
Shell item. The sink retains that item with AddRef and releases it with its owner.
It never serializes the COM pointer or calls this durable recovery.

Current handling:
- Failure HRESULT: per-item failed/access-denied.
- S_OK with no recycled item: removed_unverified.
- S_OK with a recycled item: recycle_item_observed, still removed_unverified.
- Other success/status codes: unknown; not inferred to mean recycled.

The native RecoveryVerifier remains unavailable. Both service preview availability
and native entry point fail closed. Compile-time and runtime gates are preserved.
No restoration bridge or restoration mutation method exists.

RecycleReceipt and PrepareRestore provide a read-only architecture:
- receipt must bind target ID and original filesystem identity;
- backend provider and opaque token must exist, with valid observation/expiry;
- inspector must be available and freshly validate the receipt and destination;
- mismatches, cancellation, stale evidence, collisions and inspection uncertainty
  refuse planning;
- RestorePlan fields are private and expose no execution operation.

Required native provider work before any enablement:
1. Capture a durable Shell namespace identity on the STA, bind it to original
   volume/file identity and metadata, and re-enumerate/rebind the same item in the
   current user's Recycle Bin. Do not assume a retained pointer survives restart.
2. Prove that original absence plus callback result refers to this operation's
   exact item. Resolve ambiguous, changed, emptied, and foreign-user entries by
   refusal. Do not rely on undocumented $I/$R parsing or localized verb strings.
3. Establish a no-permanent-fallback policy for volume, size/quota, Shell policy,
   and unsupported items BEFORE deletion. The observed post-delete item is too
   late to protect a permanently deleted file. Recycle flags and
   TSF_DELETE_RECYCLE_IF_POSSIBLE alone are not treated as proof.
4. Store recovery receipts separately from public history, with an explicitly
   reviewed local privacy/integrity model. No persistence format was implemented.
5. Implement collision-refusing, no-elevation restore under held destination
   guards; atomically consume plans and revalidate immediately before action.
   Verify restored identity/content. Never overwrite or auto-rename.
6. Resolve the final-file race or continue refusing execution.

The provider interface and fake inspector tests are architecture, not verified
Windows recovery. No claim of reversible cleanup, restore success, or acceptable
native cancellation latency is made.

## Validation observed in this run

Environment: Windows amd64, Go 1.27.0, Wails 3.0.0, Windows SDK 10.0.26100.0.
SDK ShObjIdl_core.h SHA-256:
`1C6C0D74400D6DCB5B17FB2156A3CE8DF420CD9271E0A23230D661790406A03F`.

| Check | Result |
|---|---|
| Root go test ./... | Passed; live opt-ins disabled |
| Root go vet ./... | Passed |
| Tagged cleanup/execution, storage and history tests | Passed |
| Tagged vet for changed backend packages | Passed |
| Installed-SDK contract and synthetic callback ABI tests | Passed |
| Non-mutating Windows ancestor access-sharing test | Passed |
| Desktop go test ./... and go vet ./... | Passed |
| Tagged desktop go test -run '^$' ./... | Passed compilation; no tests run |
| Windows 386 tagged execution test executable | Cross-compiled; not launched |
| Regenerated Wails TypeScript bindings | Passed generation |
| Frontend TypeScript check | Passed |
| Frontend Vitest | 41 tests in 13 files passed |
| Frontend production build | Passed |
| Ordinary Windows desktop production build | Passed; not launched |
| git diff --check | Passed |

Ordinary local build tags were production only, confirmed using go version -m.
The binary was built before commit with dirty-tree metadata and is not a release.
SHA-256: `3A58AD47C98B35B21701AAD49A9AE99AA0DE87E4DB655400A22451578A3454C9`.

Transient failures were corrected and retested: default Go cache access denial
(use workspace GOCACHE), missing test error constant, insufficient attribute-only
ancestor sharing, and tagged vet's raw pointer warnings. The final tagged vet
passes after typed COM pointers and typed QueryInterface arguments.

Neither runtime Shell behavior nor an independent reviewer has validated the new
native sink. Callback tests invoke synthetic functions, never IFileOperation.
No race-detector run, UI/screen-reader validation, native latency measurement,
or remote CI result is claimed.

## Next handoff

1. Obtain an independent source/security review of the COM sink, refcount/pinning,
   engine result reconciliation, ancestor guards and restoration design.
2. Implement and independently verify the read-only native recovery provider,
   receipt privacy model, no-fallback preflight and final-file race defense.
3. Add deterministic tests for all new provider/refusal cases before any native
   operation. Keep ordinary builds disabled and keep native Available() refusing
   while any requirement is unresolved.
4. Only after separate explicit owner permission, plan a single disposable-file
   Shell recycle/restore test with identity/content checks and reviewed policy.
   This handoff grants no live-operation or UI permission.

Reproduce backend checks with a writable GOCACHE. For SDK validation set
STACKBROOM_TEST_SDK_HEADER to the installed um/ShObjIdl_core.h, then run
go test -tags cleanup_execution ./internal/cleanup/... ./internal/storage ./internal/history.
Never enable STACKBROOM_ENABLE_CLEANUP_EXECUTION to run these tests.

## Primary sources consulted

- [Microsoft Windows SDK declarations](https://github.com/microsoft/win32metadata/blob/main/generation/WinSDK/RecompiledIdlHeaders/um/ShObjIdl_core.h)
- [Aborted-query contract](https://learn.microsoft.com/en-us/windows/win32/api/shobjidl_core/nf-shobjidl_core-ifileoperation-getanyoperationsaborted)
- [Per-item result and recycled item](https://learn.microsoft.com/en-us/windows/win32/api/shobjidl_core/nf-shobjidl_core-ifileoperationprogresssink-postdeleteitem)
- [Pre-delete callback cancellation](https://learn.microsoft.com/en-us/windows/win32/api/shobjidl_core/nf-shobjidl_core-ifileoperationprogresssink-predeleteitem)
- [Operation flags](https://learn.microsoft.com/en-us/windows/win32/api/shobjidl_core/nf-shobjidl_core-ifileoperation-setoperationflags)

SDK declarations and documented callback semantics are externally sourced facts.
The safety design, mitigation effectiveness outside the tested sharing-access
case, and proposed restoration architecture are implementation conclusions and
remain subject to independent review.
