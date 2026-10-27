# Storage Analysis — Future Design

**Status: not implemented.** The current Storage screen is a read-only fixed-volume capacity view. It does not traverse folders, detect duplicate files or claim that used space is safely reclaimable.

## Why this needs a separate design

A recursive scanner that looks harmless can:

- follow junctions, symlinks or mounted volumes into unexpected trees;
- cross into network shares, cloud placeholders or protected system folders;
- loop through reparse points;
- count hard-linked files twice;
- hit access-denied paths and mislead the user with incomplete totals;
- block the UI or saturate a disk with I/O;
- become a deletion authority by accident.

## Proposed stages

1. **Volume facts** (current): fixed-volume capacity/free space from `GetDiskFreeSpaceEx`.
2. **User-selected folder analysis**: explicit folder roots, read-only enumeration, bounded depth/file count, cancellation, progress and skipped-path reporting.
3. **Size attribution**: per-folder and per-extension aggregates, with measured file sizes only.
4. **Duplicate candidates**: same-size hashing with partial reads first, then full hashes; hard-link and reparse-point handling documented.
5. **Review-only findings**: evidence cards with path scope, measured bytes, confidence and risk.
6. **Approval-driven cleanup**: a separate, privileged milestone with explicit allow-lists, previews and no "Fix All".

## Safety requirements

- Reparse points: identify via `FILE_ATTRIBUTE_REPARSE_POINT`; do not follow by default. Show them as skipped/depth-limited.
- Junctions/symlinks: resolve only with explicit user intent; never traverse into a loop.
- Permissions: skip access-denied paths and report counts; never request elevation automatically.
- Exclusions: Windows, Program Files, user profile defaults, OneDrive/cloud placeholders and application data are opt-in zones, not silent targets.
- Hard links: track file IDs/volume serials so one physical file is counted once where possible.
- Alternate data streams and sparse files: document how sizes are measured (`EndOfFile` vs allocated size).
- Long paths: use `\\?\`-safe APIs and test >260-character paths.
- Filesystem changes: expect files to move/disappear; findings are snapshots, not guarantees.
- Cancellation: every traversal must be cancellable; no unbounded goroutine fan-out.
- Performance: bounded worker pool, I/O priority, progress sampling and no full-file reads for size-only views.
- Network/removable/cloud: excluded unless explicitly selected; never scan in the background.
- No deletion in the analysis milestone.
- No "reclaimable" label unless a future measured, reversible action is proven safe.

## Data model sketch

```text
StorageScan
  root, volume, startedAt, completedAt
  filesScanned, directoriesScanned, bytesMeasured
  skipped[] { pathKind, reason, count }
  findings[] { kind, scope, measuredBytes, confidence, risk, evidence }
```

## Open questions for the next milestone

- Should the first folder scan be single-root or multi-root?
- How should cloud placeholder files be represented?
- What retention policy applies to locally cached scan results?
- Can duplicate detection be useful without reading file contents?
- What is the acceptable cancellation latency on a slow or failing disk?
- Which findings are useful to ordinary users without inviting fear-based cleanup?
