# M3B-4 Recovery Feasibility and Architectural Decision

**Status:** feasibility and architecture decision only. Cleanup execution remains
disabled. No user file, Recycle Bin item or native Shell/COM operation was
deleted, recycled, moved, renamed, purged or restored. Go tests create and remove
disposable temporary fixtures only.

**Baseline:** `feat/m3b-3-recovery-verification` at
`c6959fa4ff4b8f313a70fee4454555adf7084c16`.

**Branch:** `feat/m3b-4-recovery-feasibility`.

This document deliberately separates documented Windows guarantees, source
facts, observed behaviour and assumptions. A post-operation callback is not
evidence that permanent deletion was impossible beforehand.

## Decision summary

| Option | Decision | Meaning |
|---|---|---|
| A. Windows Recycle Bin | **NO-GO** | Windows does not expose a documented, pre-execution, no-fallback guarantee. The Shell can report a permanent deletion after the fact, which is too late. |
| B. StackBroom-managed quarantine | **CONDITIONAL** | A narrow same-volume, handle-bound, journaled quarantine may be technically defensible, but it is not proven and must not ship without a separate design, independent review and destructive testing. It does not reclaim disk space by moving files. |
| C. Remain preview-only | **GO now** | The only currently defensible production posture. Keep discovery and preview read-only while B is investigated. |

**Architectural recommendation:** keep StackBroom preview-only. Do not implement
Recycle Bin execution. Fund quarantine only as a separately gated feasibility
prototype (`M3C-1`), not as a production cleanup path.

## 1. Evidence standard

- **Guaranteed** means stated in current Microsoft API documentation or
  guaranteed by a documented API contract.
- **Source fact** means directly visible in this repository.
- **Observed** means a local observation, not a contractual guarantee.
- **Assumption** means an engineering hypothesis that still needs a test.

No independent security review has approved M3B-2 or M3B-3. This document is an
audit for review, not an approval.

## 2. Phase 0 audit

### 2.1 Fail-closed properties that are real

- The ordinary build compiles out the native executor. `execution.BuildEnabled()`
  is a compile-time `false` in `gate_stub.go`, and `ConfirmCleanup` checks it
  before the environment gate (`apps/desktop/service_cleanup.go`).
- `Engine.Available()` aggregates executor, verifier, preflight, final guard and
  receipt provider. `NewEngine` installs unavailable preflight, guard and receipt
  providers, so availability fails closed (`internal/cleanup/execution/engine.go`).
- `windowsExecutor.Execute` checks `NewRecoveryVerifier().Available()` before
  `executeWindowsSTA`; the current verifier always refuses. COM is therefore not
  activated even in a tagged build (`windows_executor.go`).
- The execution context is immutable, expiring, target-bound and atomically
  single-use (`context.go`).
- The native sink implements the documented 19-entry
  `IFileOperationProgressSink` vtable, uses an STA, checks the recycle flag,
  returns `E_ABORT` on cancellation, and never claims `recycled` merely because
  `PostDeleteItem` supplied a pointer (`windows_sink.go`).
- Cleanup target validation refuses missing identity, hard links, reparse points,
  placeholders, hidden/system/sparse files, metadata changes and scope escapes
  (`internal/storage/cleanup_validation.go`).

These properties support refusal. They do not prove that a Shell delete is
recoverable.

### 2.2 Findings

**F1 — The Shell contract is fallback-capable by design.** Microsoft documents
`FOF_ALLOWUNDO` as "Preserve undo information, if possible", and documents
`FOF_WANTNUKEWARNING` as a warning when an item is "being destroyed ... rather
than recycled." Most directly, `PostDeleteItem` says `psiNewlyCreated` is `NULL`
if the item "was fully deleted." A `NULL` callback is proof that permanent
deletion happened, not proof that it could not happen. Source: `windows_sink.go`
handles `newItem == nil` by recording `removed_unverified`, but the file is already
gone.

**F2 — The no-fallback contracts are currently only refusal gates.**
`NoFallbackPreflight`, `FinalFileGuard` and `ReceiptProvider` have no native
implementation. `NewUnavailablePreflight`, `NewUnavailableFinalGuard` and
`NewUnavailableReceiptProvider` always fail, and `Engine.Available()` therefore
always fails for a production engine. This is the correct current state, but it
is not evidence that those guarantees can be obtained.

**F3 — Receipt capture is not connected to a receipt store.** `ReceiptProvider`
returns a `RecycleReceipt` with an `OpaqueToken`, and the engine checks that the
token is non-empty and unexpired. Nothing stores that receipt in
`MemoryReceiptVault`, and nothing later looks it up. A provider can therefore
return a syntactically valid token that has no durable recovery meaning. The
vault is currently used only by its unit test. This is a real gap between the
word "durable" and the implementation.

**F4 — The old `Take` operation consumed a receipt during inspection.** The
M3B-3 `MemoryReceiptVault.Take` deleted the entry before returning it. A caller
that merely displayed or inspected a receipt destroyed the only recovery
authorisation. M3B-4 replaces `Take` with side-effect-free `Lookup`, a single-owner
`Reserve`, `Release`, and final `Commit`; see section 6.

**F5 — Tests prove refusal paths and fake integration, not native behaviour.**
`TestNativeExecutorRefusesBeforeCOM` reaches the unavailable verifier and never
calls COM. `TestSinkCallbacksWithoutShellOperations` dispatches synthetic ABI
callbacks. `fakeExecutor` uses `os.Remove` on disposable test files. No test
executes `IFileOperation`, Recycle Bin restoration, a real handle-bound rename or
crash recovery. This is appropriate for the current prohibition, but it cannot be
presented as native verification.

**F6 — The final-file race is not closed.** `GuardCleanupAncestors` holds
directory handles without broad sharing and narrows ancestor substitution, but
the Shell still receives a path through `SHCreateItemFromParsingName`
(`windows_executor.go`). `IFileOperation` has no API for passing the already
validated file handle. A final-file guard that actually binds the file is not
implemented. The ancestor guard does not prove which final file the Shell acts
on.

**F7 — UI recovery state could outlive receipt failure.** The engine previously
copied `RecoveryEvidence` into an item before receipt capture. If capture failed,
the backend view could still expose `RecoveryAvailable: true` from transient
evidence. M3B-4 only exposes evidence on the successful `ItemRecycled` receipt
path, and the desktop mapping additionally requires `Status == ItemRecycled`.

**F8 — Native handle and sink lifetimes are bounded by target count, not a
dedicated cap.** `executeWindowsSTA` holds an ancestor guard for every target
until the operation returns, and retains a sink and (when supplied) a recycled
COM item per target. `Context` bounds the target count indirectly through the
rule, but there is no separate native resource budget. A future implementation
must cap targets and release per-target resources promptly.

**F9 — Receipts have no owner binding.** A receipt binds `TargetID` and
`OriginalFileID`; it does not bind the current user SID, the rule version, the
root or a process identity. The opaque token is a bearer capability. In a
same-user threat model this may be acceptable, but it is not the same as proving
ownership. A durable design must bind an owner and define token compromise.

**F10 — In-memory receipts cannot support restart recovery.** `MemoryReceiptVault`
is process-local. It does not persist across application restart, does not
survive a crash and does not zero memory on `Clear`. Durable receipt storage is
still blocked on separate privacy and integrity review.


## 3. Windows Recycle Bin feasibility

### 3.1 Questions and documented result

| Requirement before execution | Documented finding | Verdict |
|---|---|---|
| Volume supports recycling | `SHQueryRecycleBin` returns only current size and item count for a drive. `SHQUERYRBINFO` has no capacity, policy, eligibility or support field. | Not provable from this API. |
| Item is eligible | Shell decides eligibility during the operation. `FOF_ALLOWUNDO` is conditional ("if possible"). `FOFX_RECYCLEONDELETE` expresses a request, not a preflight proof. | Not guaranteed. |
| Policy cannot permanently delete | `FOF_WANTNUKEWARNING` exists specifically because an item can be "destroyed ... rather than recycled." `PostDeleteItem` reports `NULL` when fully deleted. | Permanent fallback is explicitly contemplated. |
| Capacity/quota cannot force fallback | No public API reserves Recycle Bin capacity or returns a no-fallback capacity decision. Default capacity is derived from quota and can be changed by policy; `SHQueryRecycleBin` reports only current usage. | Not provable. |
| Exact source identity will be acted on | `IFileOperation` accepts an `IShellItem`; the native code creates it from a path with `SHCreateItemFromParsingName`. The validated handle is not passed. | Path-based race remains. |
| Durable recovery identity can be captured before deletion | `PostDeleteItem` receives an `IShellItem` pointer after the fact. A COM pointer is not a durable token and the documentation does not promise it survives restart or Recycle Bin changes. | Not available before execution. |
| Item can be restored safely | There is no documented `IFileOperation` restore method. Recycle Bin restoration is a Shell namespace/UI operation; `SHEmptyRecycleBin` only purges. | Not a supported recovery contract. |

### 3.2 Conclusion: Option A is NO-GO

The decisive point is ordering. StackBroom's policy requires proof **before** a
mutation that a permanent-delete fallback cannot occur. `IFileOperation` and the
Recycle Bin APIs do not provide that proof. They provide flags, best-effort
behaviour and a post-operation result that may say the item was fully deleted.
Recording that result as `removed_unverified` is honest, but it is not recovery.

`PostDeleteItem` must not be used as a preflight guarantee. The callback is
evidence after the irreversible event; it cannot make the event recoverable.

A Recycle Bin implementation would therefore remain **NO-GO** even if a native
verifier and receipt provider were written, unless Microsoft documents a new
pre-execution no-fallback contract that this project can rely on.


## 4. Handle-bound reversible quarantine

### 4.1 Mechanism under consideration

Option B would move a validated file into an application-managed recovery area
on the **same volume**, using a handle-bound rename rather than a Shell delete:

- open the exact source with `DELETE` access and
  `FILE_FLAG_OPEN_REPARSE_POINT`;
- validate identity, link count, attributes and metadata on that handle;
- open the quarantine destination directory and retain its handle;
- call `SetFileInformationByHandle(..., FileRenameInfo, ...)` with
  `FILE_RENAME_INFO.RootDirectory` set to the destination-directory handle and
  `ReplaceIfExists = FALSE`;
- record the intent and result in a durable journal;
- restore later with the reverse operation after collision and identity checks.

Microsoft documents that `FILE_RENAME_INFO.RootDirectory` can be a handle to the
directory relative to which a relative name is resolved. The same structure
documents that `ReplaceIfExists = FALSE` fails if the target exists. The kernel
rename documentation states that a file or directory can only be renamed within
a volume, and that a file cannot be renamed while it has open handles (apart from
a breakable batch oplock). NTFS keeps a file ID until deletion; FAT can change a
file ID on rename, and ReFS uses a 128-bit ID. Hard links are multiple directory
entries for one file and must be on the same volume.

### 4.2 What this would and would not prove

**Could be proven with a narrow prototype:**

- the source handle and destination-parent handle refer to the objects the code
  validated;
- the rename cannot overwrite an existing destination when `ReplaceIfExists` is
  false;
- the file remains on the same volume and keeps its content;
- a recorded file identity can be checked after the rename;
- a journal can distinguish "not moved", "moved" and "unknown" after interruption.

**Not guaranteed by the documented APIs:**

- that a rename and a journal update are one atomic transaction;
- that directory-entry metadata is durable across sudden power loss without a
  volume flush;
- that an arbitrary process cannot hold an incompatible handle and make the
  rename fail;
- that a destination directory can be created on every volume with acceptable
  ACLs and user-visible semantics;
- that quarantining reclaims disk space (it does not; it only changes location);
- that eventual purge is recoverable (it is not; purge must be a separately
  authorised irreversible operation).


### 4.3 Required design gates for Option B

1. **Scope:** fixed NTFS/ReFS volumes only; regular files only; one link; no
   reparse point, placeholder, sparse file, hidden/system file or directory.
2. **Same-volume destination:** compare volume GUID paths, not drive-letter
   strings. Refuse if the application cannot obtain a per-user, correctly ACLed
   quarantine directory on that volume.
3. **Handle identity:** record and recheck volume serial plus NTFS 64-bit or ReFS
   128-bit file ID on the open handle. Refuse unsupported filesystems.
4. **No overwrite:** opaque random destination name, `ReplaceIfExists = FALSE`,
   plus a collision preflight. Never replace on collision.
5. **Journal:** write and flush an intent record before the rename; write and
   flush a result record after it. Startup recovery must reconcile by identity,
   not only by path. An uncertain state must refuse automatic action.
6. **Restoration:** refuse existing destination, missing parent, changed
   identity, changed content and elevation. Verify identity and content after
   restore. Do not auto-delete a colliding file.
7. **Privacy:** encrypted, authenticated journal; opaque quarantine names;
   restrictive ACL; no path in action history; explicit retention and clearing.
8. **Storage honesty:** show quarantined bytes as retained local storage, not as
   reclaimed space. A purge is a separate irreversible feature.
9. **Testing:** disposable fixtures only, with owner authorisation; include
   locked files, hard links, junctions, placeholders, cancellation, power-loss
   simulation and restart recovery.

### 4.4 Verdict

Option B is **CONDITIONAL**, not GO. It is the only candidate that could satisfy
a recovery-first policy, but the evidence needed for production does not yet
exist. Moving files into quarantine is not cleanup by itself, and a journal is
not automatically crash-consistent just because it is written to disk.


## 5. Architecture comparison

| Dimension | A. Recycle Bin | B. Managed quarantine | C. Preview-only |
|---|---|---|---|
| Target binding | Path/`IShellItem`; no validated handle | Source handle and destination-parent handle can bind the rename | Not applicable; no mutation |
| Recovery identity | Post-operation Shell item pointer; no documented durable token | File ID plus journal record can be verified before and after | Not applicable |
| Permanent fallback | Explicitly possible; `PostDeleteItem` can report `NULL` | No permanent-delete API in the move path | None |
| Capacity | No preflight reservation; quota/policy can force fallback | Same-volume move does not free space; no capacity fallback | None |
| Crash consistency | Shell-owned; not app-journaled | App must journal; rename and journal are not one transaction | None |
| Restoration | No documented API; Shell namespace/UI only | Reverse handle-bound rename, collision refusal and content check possible | None |
| Compatibility | Broad Shell support; difficult to constrain | Narrow: fixed NTFS/ReFS, same volume, single-link regular files | Broadest; read-only |
| User-visible storage | Recycle Bin usage grows | Quarantine usage grows; no reclaim until purge | No change |
| Maintenance | Shell/COM/policy complexity | Journal, encryption, ACL, recovery, retention and purge complexity | Lowest |
| Evidence today | Documented fallback behaviour; no no-fallback guarantee | API primitives documented; no end-to-end proof | Already implemented and tested |
| Recommendation | **NO-GO** | **CONDITIONAL** (R&D only) | **GO now** |

### 5.1 Recommendation

The production decision is **Option C**. It preserves the trust property that
StackBroom never claims a deletion is recoverable when it cannot prove it.
Option A cannot meet that property. Option B might, but only after a separate
prototype proves handle binding, journal recovery and collision-safe restoration;
until then it is a research direction, not a product capability.

This is not a preference for inaction. It is the only conclusion supported by the
current evidence.


## 6. Recovery receipt lifecycle

### 6.1 M3B-3 model audit

| Property | Finding |
|---|---|
| Cryptographic randomness | 32 bytes from `crypto/rand`, rendered as 64 hex characters. Adequate for an opaque 256-bit token. |
| Identity binding | Binds `TargetID` and `OriginalFileID`; no user SID, rule version, root, provider key or process identity. |
| Expiration | `ObservedAt`/`ExpiresAt` are checked, but the provider chooses the interval; there is no maximum lifetime. |
| Replay prevention | The old `Take` deleted on read; it prevented replay but also consumed on inspection. The new lifecycle makes reservation the replay barrier. |
| Ownership | None. The token is a bearer capability inside the process. |
| Concurrency | The in-memory vault is mutex-protected. The old API had no reservation; the new API grants one live reservation. |
| In-memory lifetime | Entries live until `Take`/`Commit`/`Clear`; `Clear` drops references but does not zero memory. |
| Persistence | None. The vault is not durable and is not used by the engine. |
| Restart recovery | Impossible with the current store. |
| Privacy | The receipt itself contains no path, filename or content, but `OriginalFileID` includes the volume serial and file index and remains sensitive backend data. |

The critical lifecycle bug is fixed: **inspection no longer consumes the
authorisation**. The larger architectural gap remains: `ReceiptProvider` and
`MemoryReceiptVault` are not connected, so the current "durable" gate is only a
shape check.

### 6.2 Implemented in-memory state machine

```text
Store -> available
Lookup(available/reserved) -> same state, read-only
Reserve(available) -> reserved (random reservation ID, bounded lease)
Reserve(reserved, live) -> refused
Reserve(expired lease) -> new reservation
Release(current reservation) -> available
Commit(current reservation) -> consumed (entry removed)
Expiry -> Lookup/Reserve refuse; Commit removes and returns an error
```

`Lookup` is safe for display and evidence inspection. `Reserve` is the only
operation that grants restoration permission. A restoration executor must hold
the reservation through preflight, rename and post-restore verification, then
`Commit`. Cancellation, collision or failed verification must `Release`, never
`Commit`. A process crash loses the in-memory reservation; a durable store must
represent the same states explicitly.

### 6.3 Durable lifecycle required before persistence

A durable store must add, at minimum:

- **Owner binding:** current user SID and a per-user key; refuse a receipt from
  another profile.
- **Authenticated encryption:** encrypt paths/identity and authenticate the
  whole record; a token alone must not decrypt the journal.
- **Token handling:** store a hash of the bearer token, not the raw token; return
  the raw token once to the backend that needs it. Do not log it.
- **State columns:** `available`, `reserved`, `restoring`, `consumed`, `expired`,
  `quarantined` (for Option B), with reservation ID and lease expiry.
- **Atomic transitions:** compare-and-swap on state and reservation ID; no
  read-modify-write race between two restore attempts.
- **Recovery:** on startup, reconcile `restoring` records by file identity and
  destination collision, never by path alone; unknown state refuses.
- **Retention:** explicit expiry, user clearing and secure deletion; no silent
  extension.
- **Integrity:** reject truncated, replayed, reordered or downgraded records.
- **Privacy:** no path in public history or UI; path only inside encrypted
  backend evidence when restoration needs it.
- **Testability:** clock injection, crash-point injection and concurrent
  reservation tests.


## 7. Prototypes and required experiments

### 7.1 Implemented in M3B-4

- Side-effect-free receipt lookup, single-owner reservation, release, final
  commit, expiry refusal and reservation replacement: unit tests in
  `internal/cleanup/execution/receipt_vault_test.go`.
- Recovery evidence is exposed only after a successful receipt path; the desktop
  mapping requires `ItemRecycled`.
- No native prototype, no COM call, no file rename, no Recycle Bin operation and
  no restoration was added or executed.

### 7.2 Read-only experiments for M3C-1

These can be performed without mutating user files:

1. Enumerate fixed volumes with `GetLogicalDrives`, `GetDriveTypeW`,
   `GetVolumeNameForVolumeMountPointW` and `GetVolumeInformationW`.
2. Open a disposable fixture with `FILE_READ_ATTRIBUTES` only and compare
   `GetFileInformationByHandle` with `GetFileInformationByHandleEx(FileIdInfo)`.
3. Check whether a proposed quarantine directory is on the same volume GUID as
   the source.
4. Check destination collisions with `GetFileAttributesW`/`Lstat` without
   creating or renaming anything.
5. Run the journal state machine with a fake filesystem that can fail before,
   between and after the rename steps.
6. Run receipt lifecycle tests against disposable synthetic records.

### 7.3 Experiments that require separate owner approval

These are destructive or potentially destructive and are **not authorised by
this brief**:

- one disposable file recycled through `IFileOperation`, including an observed
  `PostDeleteItem` `NULL` case if one can be induced safely;
- one disposable file moved into a quarantine directory and restored;
- forced interruption between journal intent and rename;
- restoration with an existing destination, changed identity and changed content;
- locked-file, hard-link, junction and placeholder refusal under real handles;
- any Recycle Bin capacity/policy experiment that could permanently delete a
  non-disposable file.

Each experiment needs a written fixture list, a disposable volume or directory,
an abort condition, a reviewer and an owner authorisation separate from this
document.

## 8. Performance, reliability and UI review

- The engine executes targets sequentially and has a 10-minute service timeout.
  The native worker is STA-bound and cannot be made concurrent without a separate
  COM apartment design.
- `CleanupService` permits one running execution and rejects a second; progress
  updates take a short service mutex. No deadlock was found in the source path.
- `ConfirmCleanup` deletes the preview before execution and keeps the progress
  map bounded to roughly 20 entries. A failed or cancelled execution still
  records a terminal state.
- The native worker's per-target ancestor handles and sink objects scale with
  target count. M3C-1 must add a target cap and release per-target resources
  before another target starts.
- A receipt lookup is O(1) under a mutex; reservation adds one random 32-byte
  generation. No unbounded queue or goroutine was added.
- The desktop UI was not launched and no screenshot or screen-reader check was
  performed because the owner's fullscreen restriction remains in effect.
  M1/M2/M3A behavior was not intentionally changed.


## 9. Verification

All commands were run from the repository root unless stated otherwise. No
`STACKBROOM_ENABLE_CLEANUP_EXECUTION=1` was used. No executable was launched.

| Check | Result |
|---|---|
| Root `go test ./... -count=1` | Passed |
| Root `go vet ./...` | Passed |
| Tagged `go test -tags cleanup_execution ./internal/cleanup/... ./internal/storage ./internal/history -count=1` | Passed |
| Tagged `go vet` for the same packages | Passed |
| Desktop `go test ./... -count=1` | Passed |
| Desktop `go vet ./...` | Passed |
| Desktop `go test -tags cleanup_execution ./... -run '^$'` | Passed compilation; no tests executed |
| Windows amd64 tagged test compilation | Passed |
| Windows 386 tagged test compilation | Passed |
| Frontend `npx tsc --noEmit` | Passed |
| Frontend `npm test` | 41 tests in 13 files passed |
| Frontend `npm run build` | Passed |
| Ordinary Windows build | Passed; `production` tag, no `cleanup_execution` |
| `go test -race` | Not run: the local Go toolchain requires cgo, which is disabled for this build |

Ordinary build evidence:

- `apps/desktop/bin/StackBroom.exe`
- **11,600,896 bytes**
- SHA-256 `EFB67684CE92B41BFD855F93A3E0B44823BBD7AC8F5F3126343D57C7B7B7D8DE`
- `go version -m` reports `build -tags=production`, `GOOS=windows`,
  `GOARCH=amd64`; no `cleanup_execution` tag.
- The build script completed with exit code 0; the executable was not launched.

The test additions cover:

- lookup does not consume or reserve;
- a live reservation excludes a second owner;
- release restores availability without consuming;
- commit removes the receipt permanently;
- expired leases can be replaced; expired receipts cannot be reserved;
- incomplete preflight (unsupported filesystem, unknown policy, unknown
  capacity, missing provider, expiry) refuses before the executor;
- unavailable inspector, missing token, future observation, target mismatch and
  identity mismatch refuse without calling the inspector;
- receipt failure cannot expose recovery evidence.

No test invoked COM, Shell deletion, Recycle Bin, file rename or restoration.
The fake executor removes only disposable temporary fixtures created by tests;
no user path is used.


## 10. Exact next implementation milestone

**M3C-1 — Quarantine feasibility harness (read-only/simulated).**

Scope:

1. Add a read-only Windows capability probe for fixed-volume identity and
   filesystem family using `GetVolumeNameForVolumeMountPointW`,
   `GetVolumeInformationW` and `GetFileInformationByHandleEx(FileIdInfo)`.
2. Add a pure quarantine journal state machine with explicit `planned`,
   `intent_durable`, `renamed`, `verified`, `restore_intent`, `restored` and
   `unknown` states, plus fault injection for each transition.
3. Add same-volume and destination-collision checks that do not mutate files.
4. Refuse FAT/exFAT/network/removable/unknown filesystems, reparse points,
   placeholders, hard links, directories and missing identity.
5. Keep the production executor and `ConfirmCleanup` disabled. Do not add a
   native rename call in M3C-1.
6. Obtain independent review of the M3B-4 decision and the M3C-1 design before
   any handle-bound rename experiment.

Only after M3C-1 review and separate owner authorisation may M3C-2 implement a
single-file handle-bound quarantine/restore experiment on disposable fixtures.
M3C-2 is not authorised by this brief.

## Primary sources

- [IFileOperation](https://learn.microsoft.com/en-us/windows/win32/api/shobjidl_core/nn-shobjidl_core-ifileoperation)
  — STA requirement and Shell item model.
- [IFileOperation::SetOperationFlags](https://learn.microsoft.com/en-us/windows/win32/api/shobjidl_core/nf-shobjidl_core-ifileoperation-setoperationflags)
  — `FOF_ALLOWUNDO` ("if possible"), `FOF_WANTNUKEWARNING` ("destroyed ... rather
  than recycled"), `FOFX_RECYCLEONDELETE`.
- [IFileOperationProgressSink::PostDeleteItem](https://learn.microsoft.com/en-us/windows/win32/api/shobjidl_core/nf-shobjidl_core-ifileoperationprogresssink-postdeleteitem)
  — `psiNewlyCreated` is `NULL` if the item was fully deleted.
- [SHQueryRecycleBin](https://learn.microsoft.com/en-us/windows/win32/api/shellapi/nf-shellapi-shqueryrecyclebinw)
  and [SHQUERYRBINFO](https://learn.microsoft.com/en-us/windows/win32/api/shellapi/ns-shellapi-shqueryrbinfo)
  — current size and item count only.
- [SHEmptyRecycleBin](https://learn.microsoft.com/en-us/windows/win32/api/shellapi/nf-shellapi-shemptyrecyclebinw)
  — purge operation; no restore.
- [SetFileInformationByHandle](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-setfileinformationbyhandle)
  and [FILE_RENAME_INFO](https://learn.microsoft.com/en-us/windows/win32/api/winbase/ns-winbase-file_rename_info)
  — handle-bound rename, `RootDirectory`, `ReplaceIfExists`.
- [FILE_RENAME_INFORMATION (kernel)](https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/ntifs/ns-ntifs-_file_rename_information)
  — same-volume rule, open-handle restrictions, relative destination handle.
- [BY_HANDLE_FILE_INFORMATION](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/ns-fileapi-by_handle_file_information)
  — NTFS file ID stability, FAT caveat, ReFS 128-bit note.
- [FILE_ID_INFO](https://learn.microsoft.com/en-us/windows/win32/api/winbase/ns-winbase-file_id_info)
  — 128-bit file identity.
- [CreateHardLink](https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-createhardlinkw)
  — hard links and same-volume constraint.
- [GetVolumeNameForVolumeMountPoint](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-getvolumenameforvolumemountpointw)
  — volume GUID comparison.
- [Default Recycle Bin size](https://devblogs.microsoft.com/oldnewthing/20140701-00/?p=603)
  — Microsoft blog; quota-based default, explicitly non-contractual.
