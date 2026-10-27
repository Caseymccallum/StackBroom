# StackBroom: future authorised controls

This is a design contract for later development, not an implemented action API. The current four-method, privacy-filtered bridge remains unchanged.

## Where capabilities fit

| Capability | Entry and detail | Preview before confirmation | Recovery and result |
|---|---|---|---|
| Process management | Processes: select a row to open a details pane; keep routine inventory uncluttered. Actions belong in that pane, with an explicit advanced section. | Name, PID, creation time, affected app, unsaved-work risk and required permission. Refresh and revalidate identity immediately before action to prevent PID reuse errors. | Show the exact result; do not promise undo for termination. Offer reopening only if safely supported. Never auto-retry a destructive command. |
| Application and startup management | Add destinations only when functional. Distinguish installed apps from currently running processes. | Publisher, startup entry source, current value, intended change and sign-in impact; request elevation only for the specific operation. | Preserve the previous value and provide a narrowly scoped restore operation where supported. |
| Storage exploration and cleanup | Storage: retain capacity at the top; measured folder analysis below. Use explicit scan scope and cancellation. | Exact paths, exclusions, sizes, age, ownership, risk and whether recycle-bin recovery applies. Used space and Docker estimates must never become a guaranteed recoverable total. | Per-item outcomes, kept/skipped/failed states and recovery instructions. Never silently bypass locked files or permissions. |
| System health and security | Add a real destination when trustworthy observations exist; use source-specific status and timestamps. | Explain the source, uncertainty, exact Windows setting to change and security implications. No synthetic overall health score. | Read back the actual state after change. Retain the previous setting when restoration is possible. |
| BYOK adviser | Optional Settings setup and a contextual troubleshooting pane. Keep the normal app usable without AI. | Show provider, model, estimated cost where available, and the exact redacted payload before each transmission. User selects the evidence to send. | Provider errors remain separate from PC status. Key removal, secure OS credential storage and clear history controls need a separate threat model. AI output never grants action permission. |

## Shared action flow

1. Inspect: a details pane identifies one target and the observation time. Keep evidence and confidence visible.
2. Preview: a narrow backend API returns an operation ID, revalidated target identity, requested permission, exact effects and recovery limitations. A stale preview expires.
3. Confirm: describe the named action and target in plain language. Use a specific verb such as “Disable startup for …”. No preselected destructive bulk operations and no generic “Fix all”. Cancel receives equal keyboard access.
4. Execute: one explicit user confirmation authorises only the previewed operation. The backend rechecks scope and identity. There is no arbitrary shell or filesystem binding.
5. Report: show succeeded, skipped and failed outcomes individually; announce completion accessibly. Cancellation explains what has already happened. Automatic retries require an idempotent, reviewed operation design.
6. Recover: offer undo only when restoration is actually supported. Otherwise explain the limitation before confirmation and provide useful next steps afterwards.

## Implementation sequence for DeepSeek

Keep DTO mapping and privacy filtering at the service boundary. Start with read-only details and storage scan previews, each with bounded scope, cancellation, resource budgets and tests for access denial, missing/stale identities and unsupported data. Design capability-specific service methods and permissions before implementing any action. Review reparse points, hard links, exclusions and very large directory traversal for storage; review PID creation time and protected processes for process actions. Confirm Windows security observations against their original sources. Implement credential handling and provider transmission only in a separately authorised BYOK milestone.

Require keyboard operation, focus return, both themes, outcome announcements, real WebView2 verification and permission-denial tests for each functional milestone. A roadmap card must never substitute for a working destination or suggest that current controls perform an action.
