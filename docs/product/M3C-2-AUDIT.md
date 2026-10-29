# M3C-2/3 — Native Cleanup Audit and Live-Test Gate

**Status:** audit and test-harness preparation. No native deletion has been run.

**Baseline:** `46be670ec1bb18949d4eb3e6659e07a3e714c0ca`.

**Working branch:** `feat/m3c3-native-acceptance`.

## Independent source audit

This audit was performed against the actual source, not the M3C-1 design. It is
not a substitute for a second human security review.

### Confirmed blocking defects corrected

1. **Elevation was not rejected.** The native `Available()` method checked only
   the environment gate. It now reads the current process token's
   `TokenElevation`; an elevated process receives `ErrElevated` before any file
   open. No UAC request is made.
2. **The native worker trusted the engine's preflight.** `deleteOne` now repeats
   `storage.ValidateCleanupTarget`, the seven-day age check and the `.dmp`
   restriction before opening the file.
3. **A bare disposition success could be reported as success.** Results now
   carry a `Verified` bit. The engine requires both an absent original path and
   `Verified` from the native worker before counting a deletion.
4. **A dedicated harness had no way to prove it was test-only.** The harness is
   compiled only with `windows && m3c2_fixture`, generates its own temporary NTFS
   root, and requires runtime authorisation before creating anything.

### Audit notes that remain risks

- **Cancellation after `SetFileInformationByHandle` is irreversible.** The
  worker checks cancellation before the disposition, but if the mark succeeds
  it closes the handle to complete the deletion rather than leaving a
  delete-pending state.
- **Close failure is uncertain.** If `CloseHandle` fails, the worker reports
  `unknown`; it cannot safely assume whether deletion completed.
- **A filter driver or mapped section can still make deletion fail or remain
  pending.** The exclusive open and post-close path check are conservative, but
  they are not a kernel guarantee.
- **No elevation check exists in the legacy Recycle engine.** That engine is
  still disabled; the new permanent prototype is the only audited path.
- **An already-elevated process is refused, but the application does not change
  its manifest or token.** The refusal is an application-level safety gate.
- **No crash-safe journal exists for permanent deletion.** This is intentional:
  a permanent operation has no restoration record.
- **`SpaceReclaimedKnown` is always false.** The result is an allocation
  estimate, not a measured volume-free-space delta.
- **The harness directory is generated under the OS temp directory and holds a
  read-sharing directory handle.** It prevents writes/deletes through the
  directory while allowing read-only verification; it is not an OS sandbox.
- **The harness lock is released immediately before the authorised live
  disposition.** A parent-directory sharing handle must not decide whether the
  actual delete can proceed. The fixture was already verified and is a uniquely
  named owner-created temporary directory. The production ancestor guard still
  applies during the delete.
- **Lock interaction has been exercised without mutation.** A test holds the
  harness directory lock, applies the production ancestor guard, opens the
  target with DELETE access, verifies its exact handle identity and closes it
  without disposition. It passes.
- **No destructive junction, locked-file or renamed-path test has been run.**

### ABI and API review

- `CreateFile` requests `DELETE | FILE_READ_ATTRIBUTES`, no sharing, and
  `FILE_FLAG_OPEN_REPARSE_POINT`.
- `SetFileInformationByHandle` is called with `FileDispositionInfo` (4) and a
  one-byte `FILE_DISPOSITION_INFO` value. The handle has `DELETE` access.
- Basic `FileDispositionInfo` is used, not POSIX or
  `FileDispositionInfoEx`.
- Identity, size, write time, link count and attributes are checked on the same
  handle before disposition.
- `GetFinalPathNameByHandleW` is used to confirm the handle path remains under
  the trusted root.
- The original path is checked after close. A successful API return alone is
  never treated as confirmation.
- `GetTokenInformation(TokenElevation)` is used to reject an elevated process.

## Disposable fixture harness

Package: `internal/cleanup/permanent/harness`.

Compile tag: `windows && m3c2_fixture` only. It is not imported by the desktop
service or Wails bindings.

Runtime gate: `STACKBROOM_M3C2_HARNESS_AUTHORIZED=1`.

`Prepare()`:

- creates a fresh generated root under the OS temporary directory;
- validates it with `storage.ValidateRoot`;
- requires the temporary base to be named `Temp`/`tmp` and the generated root to
  be named `StackBroom-M3C2-*`;
- rejects roots under known Documents, Desktop, Downloads, Pictures, Videos,
  Music or CrashDumps path segments;
- requires NTFS;
- creates one `m3c2-single.dmp` with deterministic contents and two sentinel
  files;
- records the exact path, Windows file identity, size, modification time,
  content SHA-256 and sentinel hashes;
- holds a directory handle that denies write/delete sharing;
- verifies the initial fixture state before returning.

No production cleanup rule is changed. No caller-supplied path is accepted by
the harness. The ordinary app cannot construct it because the package is absent
from ordinary compilation.

## First live test

Test: `TestM3C2SingleFileLiveDeletion` in
`internal/cleanup/permanent/live_m3c2_test.go`.

It is skipped unless **all three** gates are set:

```text
STACKBROOM_M3C2_HARNESS_AUTHORIZED=1
STACKBROOM_M3C2_LIVE_DELETE_AUTHORIZED=1
STACKBROOM_ENABLE_PERMANENT_DELETE=1
```

Command, only after separate owner approval:

```powershell
go test -tags "cleanup_execution m3c2_fixture" `
  ./internal/cleanup/permanent `
  -run '^TestM3C2SingleFileLiveDeletion$' -count=1
```

Expected evidence:

1. One disposable `.dmp` exists and has a recorded identity/content.
2. The result is `completed`, one item is `deleted`, and `Verified` is true.
3. The original path is absent.
4. Both sentinel files remain byte-for-byte unchanged.
5. Recovery is reported unavailable.
6. Space is an estimate only (`SpaceReclaimedKnown=false`).

The test does not run by default, does not use a real crash dump, and does not
modify a production rule.

## Subsequent refusal tests to prepare

- locked file: open with an incompatible share mode; no disposition call;
- hidden file; no disposition call;
- system file; no disposition call;
- hard-linked file; no disposition call;
- renamed/replaced file between preview and execution; no disposition call;
- junction-swapped ancestor; no disposition call;
- changed contents, size or timestamp; no disposition call;
- expired preview; no disposition call;
- duplicate confirmation; context refusal;
- elevated process; gate refusal.

Each must verify the target and sentinels are untouched.

## Test provenance

| Test area | Mock or real native API |
|---|---|
| Engine validation, expiry, partial/cancelled, false success | Mock executor and injected path state |
| Native gate and elevation helper | Real Windows token API; no file mutation |
| Harness root isolation and sensitive-location refusal | Real temporary directories; no disposition call |
| Harness lock + ancestor guard + DELETE open/verify | Real Windows sharing; no disposition call |
| Outside-fixture context refusal | Real fixture/context validation |
| Native handle open/identity/final-path verification | Real NTFS fixture; no disposition call |
| Native locked-file refusal | Real Windows sharing; no disposition call |
| Disposable live deletion | Real NTFS fixture and real disposition; skipped unless authorised |
| History privacy | Local file, no native API |
| Frontend Cleaning behaviour | Mocked cleanup hook and DOM |

## Decision

- **First single-file native acceptance test:** CONDITIONAL GO, only with the
  three explicit authorisation gates and after focused source review.
- **Consumer Cleaning integration:** **NO-GO** until the authorised live test
  passes, sentinels remain unchanged, outcome semantics are reviewed, and a
  separate UI/release approval is given.

## Next integration step

Only after independent review and a successful authorised disposable test:
wire one backend-owned, selected `crash-dumps-old` preview to the permanent
engine behind the existing gate, return the `Result.Summary` fields to a
truthful result view, and keep Recycle and permanent as separate explicit
operations. Do not add a second category or a general delete API.
