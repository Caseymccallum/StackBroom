# M3 Cleanup Engine — Design and Security Specification

**Status:** design and M3B-1 architecture only. No deletion API is enabled in ordinary builds, and no destructive test has run.

## 1. Scope and principles

M3 adds curated, reversible-where-possible cleanup of user-selected disposable data. It does not add a generic filesystem-delete API, a registry cleaner, a silent “Fix All”, or autonomous cleanup.

Principles:

- User selects a category and sees exact targets before anything changes.
- Discovery, preview, confirmation and execution are separate states.
- Every deletion has a rule ID, reason, resolved identity, size and reversibility statement.
- Partial failure is reported per item; a partial result is never described as complete.
- Unknown, inaccessible or changed targets are refused, not guessed.

## 2. Versioned curated rules

A rule is a signed/versioned data record:

```text
Rule {
  id, schemaVersion, ruleVersion,
  category, title, description,
  allowedRoots, deniedRoots,
  match (extension/known cache path/process owner),
  maxDepth, maxAge,
  deletionMode: recycle|permanent,
  reversible: true|false,
  explanation, evidence,
  enabledByDefault
}
```

Rules ship with the application. There is no user-supplied script, arbitrary glob or registry cleaning. A rule update requires a security review and cannot silently widen its scope.

## 3. Approved target categories

Initial candidates:

- User temporary files created by known applications.
- Verified application caches with a documented owner and regeneration path.
- Installer remnants explicitly selected by the user.

Never automatic:

- Documents, Downloads, Desktop, Pictures, Videos, Music, game saves.
- Browser profiles, credentials, certificates, SSH keys.
- Windows, Program Files, ProgramData, Recovery, boot/system volumes.
- Docker volumes, WSL virtual disks, cloud-synced content.
- Junction/symlink targets outside the approved root.

## 4. Target identity and scope

For each candidate retain:

- Canonical path after normalisation.
- Volume serial number and file ID.
- File size, allocated size, link count and reparse/placeholder attributes.
- Rule ID and rule version.
- Confirmation timestamp.

Before deletion:

1. Re-open with `FILE_FLAG_OPEN_REPARSE_POINT` and no content access.
2. Verify the path still resolves inside the approved root.
3. Verify volume/file identity and metadata match the preview.
4. Refuse if the target is a reparse point, placeholder, directory outside the approved category, or changed since preview.
5. Refuse if the file has multiple hard links unless the rule explicitly permits it and the preview states the consequence.

## 5. Preview-before-confirm lifecycle

```text
select category
  -> discover candidates (read-only)
  -> validate rule scope and identity
  -> create immutable cleanup preview
  -> display exact targets, sizes, rule IDs, mode, reversibility
  -> explicit confirmation
  -> consume preview once
  -> revalidate every target
  -> execute per item
  -> verify outcome per item
  -> record minimal history
```

The preview expires (for example 10 minutes). Expired previews require a new discovery and confirmation. There is no automatic confirmation and no “remember my choice” for destructive actions.

## 6. Recoverable deletion and Recycle Bin limits

- Preferred mode is Recycle Bin through `IFileOperation` with `FOF_ALLOWUNDO`.
- Recycle Bin availability depends on volume, file size, policy and file type; the preview must not promise recovery when unavailable.
- Files too large for the Recycle Bin, network paths, some removable media and protected locations may be permanently deleted.
- Permanent deletion requires a second explicit confirmation naming exact targets and stating that recovery is not promised.
- The UI must state whether the selected rule uses recycle or permanent mode before confirmation.

## 7. `IFileOperation` integration

- Use one COM `IFileOperation` session per confirmed preview.
- Set `FOF_ALLOWUNDO` for recycle mode, `FOF_NOCONFIRMATION` only after StackBroom's own confirmation, and `FOFX_EARLYFAILURE` where appropriate.
- Do not use shell execution, `cmd /c del`, or a background service.
- Track the operation's HRESULT per item where COM exposes it; otherwise verify filesystem state directly.
- Never report success from the API return alone: verify target absence and, for recycle mode, that the item left the expected location.
- Cancellation is requested through COM and checked between items; already-completed items are reported individually.
- Clean up COM objects and handles on all paths.

## 8. Cancellation and partial outcomes

Outcomes per item: `deleted`, `recycled`, `skipped`, `changed`, `access_denied`, `cancelled`, `failed`, `unknown`.

A group outcome is complete only when every selected item is verified. Otherwise it is partial with counts and per-item details. A cancelled operation never leaves a background worker running.

## 9. Minimal action history

Store locally, with retention controls:

- Timestamp, rule ID/version, category.
- Target display name and relative path (not full private path by default).
- Size, deletion mode, reversibility.
- Outcome and refusal/failure code.
- Preview and confirmation identifiers (opaque, not credentials).

Do not store file contents, command lines, credentials or unrelated paths. History can be cleared and exported only after redaction.

## 10. Restoration verification

For Recycle Bin operations, attempt restoration through the Shell only when the user explicitly requests it and the item is still present. Verify restored identity and location; do not claim restoration from a successful API call alone. A permanent deletion cannot be restored and must not offer a false restore control.

## 11. Privilege boundaries

- Run at the user's normal integrity level.
- Do not request elevation merely to delete more.
- Refuse targets requiring administrator rights rather than escalating silently.
- Never expose a general delete method to the webview; actions are rule-bound and preview-bound.
- Keep the UI unprivileged; the Go service performs only validated rule operations.

## 12. Race conditions and filesystem changes

Between preview and execution the service revalidates path scope, reparse state, placeholder state, identity, size, link count and rule version. If any check fails, refuse that item. A path must not be followed after a reparse-point swap. Directory traversal for cleanup is bounded and does not follow junctions or mounted volumes.

## 13. Required tests before any M3 release

- Rule scope escape, denied root and category mismatch.
- Junction/symlink swap between preview and execution.
- Hard-link deletion semantics and warning.
- Cloud placeholder hydration refusal.
- Recycle Bin unavailable/too-large/permanent path.
- Access denied, file in use, read-only and network/removable volume.
- Cancellation during multi-item operation and per-item partial reporting.
- Identity change after preview and after confirmation.
- Restoration success, missing item and false-restore refusal.
- Privacy: no unapproved absolute paths in bridge/errors/history.
- Owner-authorised destructive tests on disposable fixtures only.

Only after these gates and an independent review may M3 be described as ready.

