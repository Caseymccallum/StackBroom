# M3B Execution Plan — Separately Reviewable Deletion

**Status:** design only. M3A has no deletion, move, rename, quarantine or attribute mutation method.

## Authorisation gate

M3B must not ship until an independent security review and owner-authorised destructive tests on disposable fixtures are complete. The first mutation-enabled release defaults to recoverable, narrowly scoped operations and does not enable permanent deletion by default.

## COM and thread model

- `IFileOperation` is a Shell COM object. Initialize COM on the worker thread with `CoInitializeEx` using an apartment model reviewed against the Shell requirement; do not call from an arbitrary Wails callback thread.
- Run the operation on a dedicated worker with a clear lifecycle. Marshal progress and per-item results back to the service; do not hold UI threads.
- Create one `IFileOperation` per confirmed preview. Release COM interfaces and uninitialize the apartment on every path.
- Do not use a background service or shell command.

## Target identity

The preview contains exact internal file records. Immediately before execution, for each selected item:

1. Verify the rule ID and version still match.
2. Re-resolve the rule scope and ensure the path is inside it.
3. Re-open with `FILE_FLAG_OPEN_REPARSE_POINT` and verify reparse/placeholder attributes.
4. Verify volume serial + file ID, size, write time and link count.
5. Refuse if any ancestor is a reparse point or resolves outside the approved root.
6. Never accept a path or identity from the frontend at execution time; accept only the opaque preview ID and consume it once.

## Recycle Bin mode

- Request `FOF_ALLOWUNDO` for the first release.
- The preview must state that Recycle Bin support depends on volume, policy, size and type.
- A file too large for the Recycle Bin may be permanently deleted by Shell policy. M3B should detect known unsupported cases and refuse rather than silently downgrade; if the platform cannot determine support in advance, the preview must state the uncertainty.
- Permanent deletion is a separate opt-in rule mode requiring a second explicit confirmation and is not enabled by default.

## Per-item operation and outcomes

Track each item independently: `deleted`, `recycled`, `skipped`, `changed`, `access_denied`, `cancelled`, `failed`, `unknown`.

- `IFileOperation` return values are evidence, not proof. Verify target absence after the operation.
- For recycle mode, verify the item is no longer at the original path and that the operation did not report an error.
- If verification is unavailable, report `unknown`, never success.
- Cancellation between items stops new work; already-completed items remain reported individually.
- Partial failure is a first-class result and must not be presented as full success.

## Restoration evidence

Restoration is a separate explicit user action. Do not offer restore for permanent deletion. For recycle mode, use the Shell restore path and verify the original identity and location; a successful API call alone is not restoration evidence.

## Hard links, links and placeholders

- Multi-link files are excluded unless a rule explicitly documents the link semantics and the preview warns that deleting one name does not remove the content.
- Reparse points, junctions and cloud placeholders are refused, not followed or hydrated.
- Parent-directory substitution is rechecked immediately before each item; the current M2 design has a residual TOCTOU window between path resolution and opening the final handle and must be documented in the security review.

## Privilege and user authorisation

- Run at the user's normal integrity level; do not elevate to widen scope.
- Refuse protected/system locations.
- Require an explicit confirmation of the exact preview, with action label, counts, sizes, deletion mode, recovery uncertainty and consequences.
- No automatic cleanup, no “Fix All”, no remembered destructive consent.

## Verification and tests before enablement

- Rule scope escape, denied roots and extension mismatch.
- Junction/symlink substitution at final target and parent directory.
- Hard-link behaviour and warnings.
- Placeholder refusal and no hydration.
- Inaccessible file, read-only file, file in use and permission denial.
- Preview expiry, stale metadata and changed identity.
- COM cancellation, partial outcomes and process shutdown.
- Recycle Bin unavailable/too large/permanent path.
- Restoration success, missing item and false restore refusal.
- Per-item outcome reporting and post-operation verification.
- Privacy: no full paths or content in history.
- Owner-authorised destructive tests only on disposable fixtures.

## Explicit non-goals for the first M3B release

- Permanent deletion by default.
- Generic delete API.
- Registry cleaning.
- Docker/WSL volume deletion.
- Automatic or scheduled cleanup.
- Cleaning protected system content.
