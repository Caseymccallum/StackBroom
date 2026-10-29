# M3C — Handle-Bound Disposition Feasibility

**Status:** native prototype implemented behind the existing `cleanup_execution`
tag. It has not been run against a real user file. Ordinary builds contain a
refusing stub only.

## Decision

The first narrow mode is **permanent deletion of explicitly selected old crash
dumps**. This is not a replacement for Recycle Bin. It is a separate operation
with no recovery promise.

Recycling remains a separate operation and is not silently substituted. The
Recycle Bin path remains gated by the M3B findings: Windows does not document a
pre-execution no-fallback guarantee for it.

## Why permanent deletion can be more defensible here

`FileDispositionInfo` is a handle-based operation:

- the caller opens the exact file with `DELETE` access;
- the same handle is used for identity, attributes and the final disposition;
- the operation does not accept an arbitrary path at disposition time;
- the basic form does not use POSIX delete semantics;
- a successful API return still does not prove the directory entry is gone, so
  the prototype checks the path after closing the handle.

The prototype deliberately uses the basic `FileDispositionInfo` class **before**
considering `FileDispositionInfoEx` or POSIX semantics. POSIX deletion removes
the visible name when the delete handle closes but can leave file data accessible
through other open handles; that is not a simpler recovery story.

## Native sequence implemented

1. Require the `cleanup_execution` build tag and
   `STACKBROOM_ENABLE_PERMANENT_DELETE=1`.
2. Accept only a permanent-only execution context for `crash-dumps-old`.
3. Revalidate every selected target:
   - exact `.dmp` name and extension;
   - trusted `CrashDumps` root;
   - age and rule limits;
   - regular file, stable identity, single link;
   - no reparse point, cloud placeholder, sparse, hidden or system file;
   - no read-only attribute.
4. Hold the existing ancestor-directory guards.
5. Open the exact file with `DELETE | FILE_READ_ATTRIBUTES`, no sharing, and
   `FILE_FLAG_OPEN_REPARSE_POINT`.
6. Recheck identity, size, write time, link count and attributes on that handle.
7. Resolve the handle's final path and confirm it remains inside the approved
   root.
8. Mark deletion with `SetFileInformationByHandle(FileDispositionInfo)`.
9. Close the handle, then check the original path with `Lstat`.
10. Report `deleted` only when the path is absent and the executor reported a
    verified deletion; otherwise report `still_present`, `refused`, `changed` or
    `unknown`. A successful disposition API return alone is not `Verified`.

## What the APIs guarantee and what they do not

### Documented guarantees

- `FileDispositionInfo` requires a handle opened with `DELETE` access.
- Basic deletion is pending until all open handles are closed and the link count
  is zero. Opening with no sharing makes conflicting existing handles a refusal,
  not a forced delete.
- A read-only file requires a separate ignore-read-only flag in the extended
  class; this prototype refuses read-only files instead.
- A return value of `STATUS_CANNOT_DELETE` indicates the file is read-only or has
  an existing mapped view in the documented extended semantics.
- `FileDispositionInfoEx` supports POSIX semantics, which removes the visible
  name at close while other open handles may retain access to the data.

### Not guaranteed

- The API call alone does not prove the path is gone.
- Deletion can be pending while other handles exist. The prototype avoids this
  by refusing a non-exclusive open, but Windows and filter drivers can still
  produce conditions that require the post-check.
- Disk space reclamation is not proven by a successful delete call. The result
  records an allocation estimate and marks `SpaceReclaimedKnown=false`.
- The handle does not make an ancestor junction race impossible by itself. The
  existing ancestor guards narrow it; the final path check detects a mismatch.
- No elevation is requested. A process already elevated is not cryptographically
  prevented from acting with its existing token; a future hardening pass may
  refuse elevated processes explicitly.

### Not supported

- Recycle Bin or recovery through this path.
- Directories.
- Hard links.
- Reparse points and cloud placeholders.
- Hidden, system, read-only or sparse files.
- FAT, exFAT, network, removable or unknown filesystems. The prototype requires
  NTFS and refuses everything else.
- Forced deletion of a locked file.
- POSIX deletion semantics.
- Crash-safe journaling of a permanent deletion. A permanent action has no
  restoration record by design; the result and privacy-safe history are the
  evidence.

## Gate proof

- `executor_stub.go` has `//go:build !windows || !cleanup_execution` and
  `Available()` returns `ErrDisabled`.
- `executor_windows.go` has `//go:build windows && cleanup_execution`; even then
  the environment variable must be `1`.
- The ordinary app never constructs the permanent engine; `NewNativeExecutor`
  returns the refusing stub.
- No permanent-delete UI is exposed. The Cleaning screen continues to say
  cleaning is unavailable.

## First authorised disposable-file test

Not performed. A future owner-approved test must use a disposable NTFS fixture
directory, never a real crash dump:

1. Create a disposable `.dmp` fixture under a dedicated test root.
2. Record its identity, size and allocation.
3. Run the tagged prototype with the explicit environment gate.
4. Confirm the original path is absent.
5. Confirm no other test file changed.
6. Confirm the result says permanent, recovery unavailable and space estimate
   not proven.
7. Repeat with a locked handle, hard link, reparse point, junction, hidden file,
   system file, wrong extension, changed identity and expired preview; every
   case must refuse without touching the target.

## Sources

- [SetFileInformationByHandle](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-setfileinformationbyhandle)
- [FILE_DISPOSITION_INFO](https://learn.microsoft.com/en-us/windows/win32/api/winbase/ns-winbase-file_disposition_info)
- [FILE_DISPOSITION_INFORMATION_EX](https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/ntddk/ns-ntddk-_file_disposition_information_ex)
- [FileDispositionInformationEx protocol](https://learn.microsoft.com/en-us/openspecs/windows_protocols/ms-fscc/2e860264-018a-47b3-8555-565a13b35a45)
- [FILE_INFO_BY_HANDLE_CLASS](https://learn.microsoft.com/en-us/windows/win32/api/minwinbase/ne-minwinbase-file_info_by_handle_class)
- [GetFinalPathNameByHandle](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-getfinalpathnamebyhandlew)
