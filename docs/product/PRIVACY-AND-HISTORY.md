# Privacy and History

## Defaults

- No account.
- No telemetry endpoint.
- No cloud dependency for inspection, cleanup or diagnostics.
- Local-only history, disabled until the user opts in where the feature requires retention.

## Data that never leaves the machine by default

- Command lines.
- Executable paths and user profile paths.
- File names and directory trees.
- Browser history, credentials, documents.
- Network payloads.

## What the desktop bridge may expose

Only the minimum DTO needed by the screen:
- Process name, PID, parent PID, memory and start-time status.
- Opacity-preserving target tokens.
- Redacted project/container names where already reviewed.
- Action outcomes and observed values.

The bridge does not expose `exePath`, raw command lines, user names or environment variables.

## History model

A history record contains:
- Timestamp.
- Capability and action type.
- Target display name and PID (PID may be omitted after retention).
- Requested and observed value.
- Outcome and refusal reason.
- Whether the action was reversible (and what "reversible" means).

History is append-only from the product's perspective and stored locally. Authentication secrets, file contents and command lines are never stored.

## Retention

- Default retention is bounded (for example 30 days or 500 events), displayed in Settings.
- The user can clear history at any time.
- Clearing history is verified and irreversible.
- Exported support packs are previewed and redacted before writing.

## BYOK AI

- Disabled by default.
- Requires the user's own API key stored in Windows Credential Manager.
- Shows the exact payload before sending.
- Sends only the evidence needed for the question.
- Never performs actions; suggestions return to the normal preview/confirm path.
- No automatic retry that could duplicate data.

## Operator access

There is no remote operator or support backdoor. Debug logs are local, bounded and opt-in.
