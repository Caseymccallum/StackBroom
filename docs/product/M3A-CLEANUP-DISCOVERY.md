# M3A Cleanup Discovery — Implementation Status

**Status:** read-only discovery and preview only. No deletion, movement, renaming, quarantine or attribute mutation exists.

## Curated rules

### `user-temp-old` v1.0.0

- Scope: current user's `%TEMP%`.
- Owner: current Windows user.
- Eligibility: regular file, at least 14 days old, single hard link, not hidden/system, not reparse, not cloud placeholder.
- Exclusions: directories, links, placeholders, hidden/system content, newer files.
- Deletion mode for a future milestone: Recycle Bin where supported; recovery uncertain.
- Risk: applications may still own old temporary files; Recycle Bin support varies.
- Why defensible: narrow per-user temp scope plus long idle age and identity checks.

### `crash-dumps-old` v1.0.0

- Scope: `%LOCALAPPDATA%\CrashDumps`.
- Eligibility: `.dmp` regular files at least 7 days old, single link, not hidden/system, not reparse/placeholder.
- Risk: dumps are diagnostic evidence; users may need them for support.
- Deletion mode: Recycle Bin where supported; recovery uncertain.

No user-defined patterns, registry rules, browser profiles, credentials, documents, game saves, Docker volumes, WSL disks or cloud working data are allowed.

## Discovery service

- `ListCleanupRules()` returns the curated catalogue.
- `StartCleanupDiscovery(ruleID, maxCandidates)` starts one cancellable read-only scan.
- `GetCleanupDiscovery(id)` returns progress, bounded candidates, exclusion counts and coverage.
- `CancelCleanupDiscovery(id)` requests cancellation.
- Discovery reuses the hardened M2 scanner: hidden/system filters, reparse refusal, placeholder checks, hard-link awareness, path scope and coverage.
- Candidate IDs are discovery-scoped; arbitrary paths are never accepted.

## Preview service

- `CreateCleanupPreview(discoveryID, candidateIDs)` accepts only IDs from that discovery.
- `GetCleanupPreview(previewID)` returns an expiring 10-minute preview.
- Each selected candidate is revalidated with `storage.ValidateFileRecord`.
- Stale, inaccessible, reparse, placeholder or hard-linked targets are excluded with reasons.
- Preview includes file count, logical/allocated sizes, recoverable estimate or Unknown, future deletion mode, Recycle Bin limitations, side effects, risks and expiry.
- **No `ConfirmCleanup`, `DeleteFiles`, `MoveToRecycleBin` or generic mutation method exists.**

## Cleaning UI

The Cleaning section lists categories, owner/scope/evidence/risk, runs analysis, shows measured sizes and exclusions, supports individual selection and creates an exact preview. It explicitly says **Preview only — cleaning not yet available** and has no functional-looking Clean Now control.

## Tests

- Rule catalogue and no mutation methods (reflection).
- Temp age and hard-link exclusions.
- Eligible candidate preview.
- Crash-dump extension filtering.
- Preview expiry.
- Changed candidate revalidation.
- Frontend rule display, checkbox selection, preview request, recovery uncertainty and absence of destructive controls.
- M2 safety, pagination, privacy and treemap regression suites remain green.
