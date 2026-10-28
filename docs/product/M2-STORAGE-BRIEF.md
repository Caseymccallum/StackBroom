# M2 Storage Intelligence — Implementation Brief

## Objective

Give users an evidence-based understanding of where storage is used, what is large or duplicated, and what is safe to inspect — without deleting or moving anything. M2 is strictly read-only.

## Questions and decisions

### 1. Which Windows APIs should be used?

- Enumeration: `FindFirstFileExW` with `FindExInfoBasic` and `FIND_FIRST_EX_LARGE_FETCH`.
- File identity and metadata: `CreateFileW` with `FILE_FLAG_BACKUP_SEMANTICS` and `FILE_FLAG_OPEN_REPARSE_POINT`; `GetFileInformationByHandleEx` (`FileIdInfo`, `FileStandardInfo`, `FileAttributeTagInfo`).
- Volume identity: volume serial number plus file ID.
- Allocation size vs logical size: `FileStandardInfo`.
- Do not shell out to `dir`, PowerShell or WMI for traversal.

### 2. What are the traversal bounds?

- One scan at a time; a new scan cancels the previous one through `context.Context`.
- Bounded worker pool (default min(8, NumCPU)).
- Default maximum depth 64; deeper trees are summarized as "too deep" rather than followed indefinitely.
- Maximum entries per scan configurable with a hard ceiling (for example 2,000,000); exceeding it stops with a partial result, not fabricated totals.
- Scope is an explicit user-selected root. Whole-drive scans require a warning and a longer timeout.

### 3. How are junctions and symlinks handled?

- Every directory entry is checked for `FILE_ATTRIBUTE_REPARSE_POINT` and the reparse tag.
- Traversal does not follow junctions or symlinks by default. They are shown as links and excluded from size totals to avoid loops and double counting.
- A resolved target outside the approved root is always refused.
- `\\?\` paths are normalized internally.

### 4. How are hard links handled?

- `FileStandardInfo.NumberOfLinks` is recorded.
- A file with multiple links is not counted twice in a tree total when its file ID has already been seen.
- Duplicate candidates with link count > 1 are labeled; they are never presented as free space that can be reclaimed without explaining link semantics.

### 5. How are cloud placeholders handled?

- Offline, recall-on-open and sparse attributes are recorded.
- Placeholder content is never opened for hashing or preview.
- A "measure metadata only" mode is the default for cloud roots.
- Hydration-required operations are refused in M2.

### 6. How is cancellation handled?

- Every stage accepts `context.Context`.
- Directory enumeration, hashing and aggregation check cancellation at bounded intervals.
- Partial results carry an explicit `partial: true` and the reason. A cancelled scan is never presented as complete.
- Worker goroutines exit before the scan returns.

### 7. How is progress reported?

- Progress events include phase (`enumerating`, `aggregating`, `hashing`), items processed, bytes observed, elapsed time and partial/error counts.
- Events are throttled (for example 10/s) and carry a monotonic scan ID.
- The UI never estimates completion from a fake percentage; it shows measured counts.

### 8. How are large directories handled?

- Do not load an entire directory's children into the webview.
- Aggregate per directory; return top-N children with "more" totals.
- Cap the number of nodes crossing the bridge (for example 2,000) and stream aggregates rather than raw entries.
- Sort deterministically and report truncation.

### 9. What are treemap limits?

- Render at most a bounded number of nodes (for example 1,500 visible rectangles).
- Aggregate smaller children into an "Other" node with exact totals.
- Minimum rectangle size below which a node is not drawn but remains in the accessible list.
- No animation of large treemaps; respect reduced motion.
- Provide an equivalent table view for screen readers and low-end GPUs.

### 10. How are duplicates verified?

- Group by size first.
- Hash candidates with a fast hash (for example xxHash/SHA-256 prefix) using bounded buffers.
- Verify final groups with full SHA-256 (or a documented cryptographic hash) before reporting them as duplicates.
- Never trust file name, extension or size alone.
- Exclude system files and placeholders; report inaccessible files separately.
- M2 only reports candidates; it does not delete.

### 11. How are paths handled across the bridge?

- Raw absolute paths stay in the Go process.
- The bridge returns user-selected display paths only for the active scan, already privacy-filtered by `privacy.Apply`, or an opaque node ID plus a display label.
- No command lines, environment variables or unrelated user paths are included.
- A "reveal in Explorer" action is a service-side operation using the node ID, not an arbitrary path from the webview.
- Tests assert that a synthetic `C:\Users\Private` path cannot appear in serialized DTOs unless explicitly selected and allowed.

### 12. What accessibility requirements apply?

- Treemap has a keyboard-navigable table/list alternative.
- Every node exposes name, size, percentage, type and child count as text.
- Focus order follows visual hierarchy; breadcrumbs are buttons.
- Cancellation and rescan are keyboard reachable.
- Progress uses `role="status"` with polite announcements, not rapid live-region spam.
- Contrast AA in both themes; no colour-only encoding.

### 13. How are tests mutation-free?

- Tests use temporary directories created by the test and removed afterwards.
- Fixtures include: nested tree, hard-linked file, directory junction/symlink (created only where permitted and skipped otherwise), sparse file, zero-byte file, inaccessible directory, long path, duplicate groups.
- No test scans the real user profile, `C:\Windows` or any production volume.
- Cancellation test uses a context cancelled after a bounded number of entries.
- Hashing tests use known content and deterministic hashes.

### 14. What is the DTO shape?

```text
StorageScanRequest { rootId, includeHidden, includeSystem, maxDepth, maxEntries, measurePlaceholders }
StorageScanProgress { scanId, phase, items, bytes, partial, errors, elapsedMs }
StorageNode { id, parentId, label, kind, logicalBytes, allocatedBytes, fileCount, linkCount, isPlaceholder, isReparse, truncated }
StorageFinding { kind: large_file | duplicate_group | placeholder | inaccessible, nodeIds, confidence, evidence }
StorageScanResult { scanId, rootLabel, totals, nodes[], findings[], partial, cancellationReason }
```

The bridge never accepts a raw filesystem path for a mutation in M2; there are no mutations.

### 15. What are the acceptance criteria?

- Read-only: no create, write, delete, rename, move or attribute change.
- Cancellation returns within a bounded time and reports partial data.
- No junction/symlink loops or double counting.
- Duplicate groups are hash-verified.
- No sensitive absolute paths cross the bridge by default.
- Treemap bounded and keyboard accessible.
- Deterministic tests on disposable fixtures.
- Baseline performance documented for a synthetic tree and a user-selected real folder, with hardware and elapsed time.
- Owner approval before any M3 delete capability is designed as implemented.

## Out of scope

Deletion, cleanup rules, disk defragmentation, partition management, cloud downloads and backup.

