# Cleanup Safety Specification

## Default rule

StackBroom does not delete anything in M2. M2 discovers and measures. M3 introduces a curated, preview-based cleanup engine.

## Allowed roots

- The current user's temporary folder.
- Explicitly selected user folders, with scope shown before scanning.
- Application caches only through a reviewed rule with a known owner and documented effect.

Never scan or clean the whole drive by default.

## Denied roots

- Windows, Program Files, ProgramData, Recovery, boot/system partitions.
- User documents, desktop, downloads, pictures, videos, music, game saves and OneDrive-synced content unless the user explicitly selects a specific item and the rule allows it.
- Browser profiles, credentials, password stores, certificate stores and SSH keys.
- Docker volumes, WSL virtual disks and `%APPDATA%` data that is not a verified disposable cache.
- Junction/reparse targets outside the approved root.

## Reparse points

Every directory entry is checked with `FILE_ATTRIBUTE_REPARSE_POINT` and the reparse tag. Tree traversal does not cross junctions or symlinks by default. A path whose resolved target escapes the approved root is refused.

## Hard links

Files with a link count greater than one are not automatically classified as duplicates for cleanup. Deleting one name may be harmless or may be the user's only intentional link; the preview states the link count.

## Cloud placeholders

Files with offline/recall attributes are measured but not downloaded. Any operation that would hydrate a placeholder is refused until the user selects a "measure without download" path.

## Deletion semantics

- Preferred: Recycle Bin via `IFileOperation` where the volume and file type support it.
- Permanent deletion requires a second explicit confirmation naming the exact paths/items.
- Never claim deletion is recoverable when it is not.
- Partial failure is reported per item; success requires every selected item to be verified gone.
- A cleanup preview lists exact resolved paths, sizes, reasons, rule IDs and what will remain.

## Safety tests

- Junction escape, symlink escape, hard-link duplicate, cloud placeholder, permission denied, file locked, path race, partial deletion and cancellation.
- Tests operate on disposable fixtures only and must not mutate the user's real profile.
