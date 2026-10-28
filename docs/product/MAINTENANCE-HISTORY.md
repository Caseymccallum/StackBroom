# Maintenance History Contract

**Status:** contract only. M3A does not persist cleanup history and does not create records for operations that did not occur.

## Event model

```text
HistoryEvent {
  id, schemaVersion, occurredAt,
  type: discovery | preview | confirmation | execution_attempt | verified_outcome | recovery,
  capability: cleanup | process | startup | application,
  ruleId, ruleVersion,
  actionSummary,
  targetCount,
  logicalBytes, allocatedBytes, bytesKnown,
  deletionMode: recycle | permanent | none,
  recoverability: available | unavailable | unknown,
  outcome: success | partial | refused | timeout | cancelled | failed | unknown,
  reasonCode,
  previewId,
  executionId,
  evidenceRef (opaque)
}
```

## Rules

- Discovery and preview events describe read-only observations.
- A confirmation event is recorded only after explicit user confirmation.
- An execution attempt is recorded only when execution is enabled and attempted.
- A verified outcome is recorded only after post-operation verification.
- Recovery availability is recorded only when it is actually known.
- No event is fabricated for UI previews or for a feature that is not implemented.
- Full private paths, file contents, command lines, credentials and tokens are not stored. Prefer relative display paths and a rule ID.
- History is local, bounded and user-clearable. Default retention is 30 days or 500 events; the exact value is shown in Settings.
- Clearing history is verified and irreversible.
- Export/support packs are redacted and previewed before writing.

## Future storage decision

Persistent storage is deferred until the M3B execution design and privacy review settle the database location, encryption expectations, retention controls and backup interaction. M3A emits no historical records.
