# M2 Storage Intelligence — Implementation Status

**Branch:** `feat/m2-storage-intelligence`  
**Baseline:** `4fdee331a7a371e634b4a74a877f4e5fa483b420`  
**Status:** read-only implementation with unit/UI tests; visual validation intentionally not run because the owner is using another fullscreen application.

## Implemented

### Native engine

- `internal/storage` scans one explicitly selected root.
- Cancellable context checks throughout traversal and hashing.
- Bounded depth (default 64, maximum 256) and entries (default 250,000, hard ceiling 2,000,000).
- Junction, symlink and mount-point detection via reparse attributes; never followed.
- Long-path support through `\\?\` / `\\?\UNC\` paths for Win32 metadata calls.
- Hidden and system items excluded by default, with explicit options to include them.
- File metadata: logical size, compressed/allocated size when Windows exposes it, hard-link count, volume/file ID, placeholder attributes and modification time.
- Hard-link-aware totals: each unique file ID is counted once; additional names remain visible and labelled.
- Sparse/offline/recall-on-open/recall-on-data attributes identify cloud placeholders. Placeholder content is not opened or hashed.
- Access-denied and skipped paths are counted and sampled; partial results are explicit.
- Directory aggregation, largest folders/files, extension totals and deterministic ordering.
- Read-only: no create, write, rename, move, delete, attribute change or shell execution.

### Duplicate verification

1. Group eligible files by logical size (minimum 4 KiB by default).
2. Exclude placeholders, reparse entries and files above the hash limit.
3. Compare a bounded 64 KiB prefix.
4. Hash complete bytes with SHA-256 using bounded workers.
5. Re-stat size, modification time, placeholder/reparse state and file ID after hashing.
6. Collapse identical file IDs so hard-link names are not called independent copies.
7. Return only groups with at least two independently allocated files, including hash, size, relative paths, link counts and a link-semantics note.
8. No deletion or cleanup method exists.

### Desktop bridge

`StorageService` exposes:

| Method | Purpose |
|---|---|
| `SelectStorageRoot()` | native folder picker, validated root, opaque grant ID |
| `StartStorageScan(rootID, options)` | starts one cancellable scan, returns scan ID |
| `GetStorageScan(scanID)` | status/progress/bounded result |
| `CancelStorageScan(scanID)` | requests cancellation |
| `GetStorageNode(scanID, nodeID)` | bounded children for a scan-scoped node |
| `GetStorageRoot(rootID)` | display metadata for the authorised root |

- Root IDs and scan IDs are random 128-bit values.
- Raw paths stay in Go. The only absolute path returned is the root the user explicitly selected.
- Result payloads cap nodes at 200, largest folders/files at 50 and skipped samples at 200.
- Root grants expire after 24 hours; scan result contexts expire after 1 hour.
- A second concurrent scan is refused.
- A cancelled scan's workers stop before the result is published.

### Desktop UI

- Existing fixed-volume metrics retained.
- Folder picker, hidden/system options, start and cancel controls.
- Measured progress only: phase, item count, bytes observed and skipped count. No fabricated percentage.
- Breadcrumb navigation, search/filter, interactive one-level treemap with drill-down.
- Accessible child table as the primary keyboard/screen-reader alternative.
- Largest folders, largest files, extension summary, partial warnings and verified duplicate evidence.
- Placeholder, reparse, hard-link and truncated states are labelled.
- No cleanup or deletion buttons.

## Safety evidence

- Storage root must pass `storage.ValidateRoot`; reparse roots are rejected.
- Traversal never follows a reparse point, including junctions. Windows junction tests pass.
- Hard-link test verifies one physical 4,096-byte file is counted once.
- Hidden-file test verifies default exclusion and explicit inclusion.
- Symlink creation is skipped on this host because it requires a privilege; junction coverage runs instead.
- Duplicate test with identical 64 KiB prefixes but different content confirms full-hash verification is required.
- Changed-file test confirms stale metadata is rejected.
- Cancellation and entry-limit tests return `partial` with reasons.
- Bridge test verifies an absolute root path is absent from the serialized scan view and unauthorised IDs are refused.

## Performance observation

Synthetic disposable fixture: 50 folders × 100 files (5,000 files, 1,000 bytes each), local Windows NTFS temp directory.

- Scan: **1.190 s**
- Heap allocation delta after GC: **4,300 KB**
- Total allocation delta: **11,192 KB**
- Fixture creation and test overhead excluded from the scan timer; whole test observed 8.96 s.

These are observations on this host, not a general performance guarantee. No hundreds-of-thousands-entry benchmark was run, and no real user profile or Windows system directory was scanned.

## Known limitations

- Directory traversal is sequential. Content hashing uses bounded workers; traversal concurrency is not yet parallel.
- Access-denied behavior is implemented and counted, but a deterministic Windows ACL denial test was not added.
- Cloud-placeholder detection is attribute-based; no live OneDrive placeholder fixture was available.
- Sparse-file allocation is reported only when `GetCompressedFileSizeW` succeeds.
- Duplicate candidates are capped at 4,000 files per scan.
- Full result contexts expire after one hour; very large scans are capped at 30 minutes.
- No screenshots or visual validation were produced because launching/interacting with the app would take focus from the owner's fullscreen task.
- No M2 performance claim is made for 100k+ entries or slow network volumes.

## M3 Cleanup Engine readiness

**Not ready for deletion.** M2 now provides discovery, bounded evidence, reparse protection, hard-link awareness and duplicate verification. M3 still needs:

1. Curated rule definitions with exact ownership and effect.
2. A preview object containing exact resolved targets, sizes, rule IDs and reversibility.
3. Atomic confirmation separate from discovery.
4. `IFileOperation` / Recycle Bin semantics with per-item verification.
5. Per-item partial-failure reporting and a cancellation-safe state machine.
6. Action history with privacy-filtered evidence.
7. Dedicated ACL, path-race, junction-escape and hard-link deletion tests.
8. Owner authorisation before any destructive test.

M2 deliberately contains no deletion, cleanup, rename, move or attribute mutation.

