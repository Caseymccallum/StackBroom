# M3C — Limited Cleanup Path

**Status:** product/engineering plan. Ordinary cleanup execution remains disabled.
No destructive test or deletion-capable build is authorised by this document.

## Product principle

StackBroom protects users from unintended actions, not from informed decisions.
The Cleaning screen should use ordinary language, keep the user in control and
explain a warning only when it applies.

The intended flow is:

**Find files → Choose files → Review → Clean → Results**

Until the native path is reviewed and tested, the Clean step stays disabled and
says so plainly.

## What M3B-4 established

- Windows Recycle Bin APIs do not provide a documented pre-execution guarantee
  that permanent fallback is impossible.
- `PostDeleteItem` can report that an item was fully deleted, but only after the
  event.
- A path-based Shell operation does not bind the already validated final file.
- The current no-fallback preflight, final-file guard and receipt provider are
  refusal gates, not proof.
- Managed quarantine can preserve the file with a handle-bound same-volume move,
  but does not reclaim disk space and needs a durable journal.

## Shortest technically defensible route

### M3C-1 — Read-only capability and journal harness

- Add read-only volume/filesystem checks: fixed volume, NTFS/ReFS, volume GUID,
  free space and file identity.
- Add a pure quarantine journal state machine with interruption points.
- Refuse reparse points, placeholders, hard links, directories, cloud files and
  unknown filesystems.
- Do not call `IFileOperation`, `SetFileInformationByHandle(FileRenameInfo)` or
  any other mutation API.

### M3C-2 — One owner-approved decision experiment

Use a disposable volume and a disposable file. Determine, with no user data:

1. What `FOFX_RECYCLEONDELETE` actually does for a file that is too large for the
   Recycle Bin or blocked by policy.
2. Whether `PostDeleteItem` ever returns a null `psiNewlyCreated` when the caller
   requested recycling.
3. Whether the per-volume capacity can be read without undocumented registry
   assumptions.
4. Whether `SetFileInformationByHandle` with a destination-directory handle
   moves the exact open file and preserves its identity.

If any tested path can permanently delete a file while the caller requested
recycling, **Recycle Bin cannot be the default cleanup action**. The result is a
design decision, not a bug to hide.

### M3C-3 — One category, one action

Start with **Old crash dumps** only, because the category is narrow, has a clear
purpose and the user can understand its value.

Recommended first action:

- **Move to Recycle Bin** only if M3C-2 proves a no-fallback path for the
  supported configuration.
- Otherwise use a **StackBroom-managed quarantine** and label it honestly:
  files leave their original folder, remain recoverable, and still use disk
  space until the user empties the quarantine.
- Never add a permanent-delete option to this milestone.
- Never act on unselected files, unrelated roots or protected paths.
- Revalidate every selected file at confirmation; a changed file is refused, not
  silently skipped.

### M3C-4 — Advanced permanent deletion

Only after independent review and owner-authorised disposable tests. It must be
a separate, explicit advanced action with an unambiguous warning. It must not be
the default, and it must never be substituted silently for recycling.

## Current blockers

- No documented no-fallback guarantee for Recycle Bin.
- Final-file identity is not bound to the Shell operation.
- No reviewed native recycle/restore or quarantine implementation.
- No owner authorisation for destructive tests.
- No durable receipt/journal storage approved.
- No UI/accessibility capture while the owner's fullscreen restriction applies.

## Acceptance criteria for a first working action

- The user explicitly selects each file or uses an explicit Select all.
- The category is one vetted rule; no arbitrary path input exists.
- The action is one of: reviewed recycle (if proven) or reviewed quarantine.
- A changed, missing, linked, reparse, placeholder or protected file is refused.
- The result is per-file and accurate, including partial/cancelled states.
- Unselected files are unchanged and outside the operation.
- The UI says whether space was reclaimed or only moved.
- Ordinary builds remain incapable of cleanup until the review gate is cleared.
