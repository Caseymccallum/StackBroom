# Capability Research

Feasibility and risk notes for the long-term roadmap. None of these capabilities is implied to be implemented yet.

| Capability | Windows support | Privacy | Performance | Primary risk | Dependency |
|---|---|---|---|---|---|
| Storage Intelligence (M2) | `FindFirstFileExW`, `GetFileInformationByHandleEx`, volume/file IDs | Paths stay in Go; only user-visible scopes and redacted aggregates cross the bridge | High on large trees; bounded workers and cancellation required | Reparse loops, hard-link double counting, cloud placeholder downloads | None |
| Cleanup Engine (M3) | `IFileOperation`, Recycle Bin semantics, per-item errors | Exact target previews required | Moderate | Permanent loss, scope escape, reparse tricks | M2 traversal and action history |
| Startup/App Management (M4) | Registry Run keys, Startup folders, MSIX/desktop inventory, scheduled tasks | App names/publishers only; no arbitrary command strings | Low | Disabling security or enterprise-managed entries | M1 action-security framework |
| Maintenance History (M5) | Local application storage | Must avoid sensitive paths; retention and deletion controls | Low | Misrepresenting irreversible actions | Introduced alongside M1/M3 |
| PC Timeline (M6) | Local snapshot diffs; reliability/event sources | Opt-in, local-only, redacted | Moderate | Fabricated causality | M5 history storage |
| Problem Detective (M7) | Bounded counters; optional ETW sessions | Local observations only | ETW can be expensive; never continuous by default | Overclaiming causes from resource percentages | Observation engine + history |
| Trust Explorer (M8) | Authenticode/WinTrust, file identity, startup/process evidence | Publisher/subject metadata only, not raw paths by default | Moderate (signature checks) | Simplistic trust scores | M4 inventory + action framework |
| Network Transparency (M9) | `GetExtendedTcpTable`, UDP table, owner PID | Remote addresses are user-visible; no packet capture | Low | Mislabeling connections as malicious | Existing Port Truth |
| Laptop Health (M10) | `GetSystemPowerStatus`, battery reports/WMI | Local only | Low | Inventing unavailable battery data | None |
| Reliability Centre (M11) | Event Log APIs / WMI reliability records | Event details can be sensitive; redact | Moderate | Treating every crash as malware/hardware | History storage |
| File Lock Detective (M12) | Restart Manager (`RmStartSession`, `RmRegisterResources`) | File paths sensitive; scope to user request | Low/Moderate | Claiming universal handle detection | M1 action framework for closes |
| Maintenance Packs (M13) | Curated rules per application | Strict allow-lists; no community scripts | Low | Unsafe cache classification | M3 rules engine |
| Security/Malware (M14) | Windows Security Center, Defender APIs; ClamAV/YARA-X research | File scanning is sensitive; explicit consent | High for scans | False positives, licensing, GPL redistribution | Separate security review |
| BYOK AI (M15) | Windows Credential Manager | Payload preview and per-request consent | Low (network latency) | Prompt injection, silent data export | Optional adapter and consent UI |
| Support Packs (M16) | Local export | Redaction and exact preview required | Low | Leaking private documents or command lines | M5 history + privacy review |

## Explicitly deferred

- Kernel drivers, packet capture, firewall interception.
- Universal kernel handle enumeration.
- Automatic quarantine/deletion.
- Real-time protection claims.
- Arbitrary user-provided cleanup scripts or registry cleaning.
