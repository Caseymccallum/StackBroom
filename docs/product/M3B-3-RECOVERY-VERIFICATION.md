# M3B-3 Recovery Verification and No-Fallback Hardening

**Status:** architecture and deterministic tests only.

Execution remains disabled in ordinary builds and in tagged builds. The native
recovery provider, no-fallback preflight, final-file guard and durable receipt
store are deliberately unavailable.

No Shell deletion, restoration, app launch, screenshot, destructive test, merge,
tag or release occurred.

## What changed

### No-fallback preflight gate

The engine now requires a read-only `NoFallbackPreflight` result for every target
before the executor is called. A result is accepted only when all of these are
explicitly true:

- Recycle operation is supported for the volume/item.
- Recycle policy is known.
- Recycle capacity is known for the item.
- A provider identifier and observation/expiry window are present.

Missing, expired, unknown or unsupported evidence refuses the entire operation
with `permanent_fallback_unproven`. A Recycle flag or a successful API call is not
treated as proof.

The current provider is `unavailablePreflight`; it always refuses.

### Final-file guard

The engine now requires a `FinalFileGuard` for each target and holds it through
the executor. The guard is intended to bind the final file identity rather than a
path. If it is unavailable, cannot be acquired, or returns nil, execution is
refused with `final_file_guard_unavailable`.

The current native implementation is deliberately absent. The M3B-2 ancestor
guard mitigates directory replacement but does not resolve the final-file race.

### Recovery receipts

`ReceiptVault` stores backend-only recovery evidence separately from public
action history:

- 256-bit random opaque token;
- target ID and original file identity;
- provider identifier;
- observation and expiry times;
- one-time `Take`;
- no path, filename, content, command line or credential.

The only implementation is `MemoryReceiptVault`; it is not durable across a
restart. A durable encrypted receipt store remains blocked on a separate privacy
and integrity review.

### Durable receipt provider gate

The engine requires a `ReceiptProvider` to be available before execution and to
return a matching, unexpired receipt after recovery evidence is observed. A
missing provider or mismatched receipt leaves the item `removed_unverified` and
cannot produce a `completed` result.

The current provider is `unavailableReceiptProvider`; it refuses before COM.

### Availability reporting

`execution.Engine.Available()` now aggregates:

- compiled/configured executor availability;
- native recovery verifier;
- no-fallback preflight;
- final-file guard;
- durable receipt provider.

Preview availability uses this method, so a tagged build cannot advertise
execution when any safety provider is unresolved.

## Remaining blockers

1. Independent review of the M3B-2 COM sink and this engine integration.
2. A read-only native provider that captures/rebinds a durable Shell namespace
   identity without assuming a retained COM pointer survives restart.
3. Native Recycle Bin capacity/policy preflight that cannot silently fall back to
   permanent deletion.
4. Durable receipt storage with a reviewed privacy/integrity model.
5. A final-file identity guard compatible with Shell recycling, or continued
   refusal.
6. Collision-refusing, no-elevation restoration implementation and verification.
7. Owner-authorised disposable-file live test.

## Tests

- engine refuses when preflight is unavailable or returns unsupported/unknown
  evidence;
- engine refuses when final guard is unavailable;
- engine closes guards after completion;
- engine refuses a receipt provider failure and cannot report `completed`;
- receipt vault token is opaque, single-use and expiring;
- engine availability fails closed by default.

No test invokes COM, Shell deletion, Recycle Bin operations or restoration.
