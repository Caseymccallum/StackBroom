# Optional BYOK AI Adviser — Design Only

**Status: not implemented.** This milestone makes no AI request, asks for no API key and stores no credential. The UI says so explicitly.

## Product rule

The adviser is optional. Normal PC health, process, storage, Docker, WSL and listener features must work without an account, API key or network connection.

## Provider-neutral interface

A future adapter interface should be small enough to support OpenAI-compatible endpoints, Anthropic, Google and local OpenAI-compatible servers without leaking provider-specific policy into the analysis engine.

```text
Provider
  id, displayName, baseURL, defaultModel
  models() -> []ModelDescriptor
  complete(ctx, request) -> Response
  validateCredential(ctx) -> error

Request
  systemInstructions
  redactedContext (bounded JSON)
  question
  timeout, maxOutputTokens

Response
  text, citations[], usage, finishReason
```

No provider SDK is required. A plain HTTPS client with strict timeouts and response-size limits is sufficient. Provider selection, endpoint and model remain user choices.

## Credential storage

- Store keys in **Windows Credential Manager** (generic credentials scoped to StackBroom and provider id) via the Win32 Credential APIs.
- Never write keys to JSON, localStorage, logs, crash output, screenshots or the Go engine snapshot.
- The frontend receives only `configured: true/false` and a masked suffix, never the key.
- The privileged credential read occurs only in the Go process at request time.
- A "Remove credential" action deletes the credential and clears in-memory copies.
- No key is required for local-only providers.

## Redaction and consent

Before any outbound request:

1. Build a **bounded, structured context** from observations the user selected.
2. Apply the existing privacy policy: no command lines, executable paths, project roots, raw profile paths, secrets or diagnostics.
3. Show a **preview** of the exact JSON/text payload.
4. Show destination host, provider, model, approximate size and timeout.
5. Require an explicit **Send once** confirmation. Consent is per request, not a silent standing permission.
6. Never send automatically on launch, refresh, error or schedule.
7. Keep a local, user-visible request log with timestamp, provider, model, byte count and outcome; no full prompt history by default.

The payload should default to the smallest useful evidence, for example CPU/memory summaries, process names/PIDs and Docker/WSL observations. Raw filesystem trees and credentials are never included.

## Cost and time controls

- Configurable provider, model, endpoint, timeout, max output tokens and a per-request cost ceiling.
- Display the provider's reported usage and an estimated cost where pricing metadata is available.
- Refuse requests above the configured ceiling or when the timeout/response-size limit is exceeded.
- No background retries that could multiply spend.
- A local-only provider can be configured with zero cost.

## Separation from system authority

The AI has **no** shell, process-control, file-deletion, Docker-prune, credential or settings-write capability. It returns text and evidence references only.

A privileged action, if ever added in a later milestone, must be:

- a separately designed, narrowly scoped operation;
- approved by the user through the ordinary non-AI UI;
- re-validated against fresh observations;
- never invoked because a model suggested it.

## Answer quality

Responses should be evidence-based:

- cite the observation fields or recommendation IDs used;
- state uncertainty and missing data;
- distinguish measured values from model inference;
- avoid inventing process names, paths, ports or cleanup outcomes;
- refuse to claim that space is safely reclaimable without a measurement;
- explain when the available context is insufficient.

## Future settings surface

- Adviser enabled/disabled (default off)
- Provider and endpoint
- Model
- Credential status / set / remove
- Timeout and token ceiling
- Cost ceiling
- Payload preview and consent
- Local request log and clear action
- "Test connection" that sends no system data

## Explicit non-goals for this milestone

- No provider contacted.
- No API key field.
- No credential storage.
- No paid call.
- No AI-generated cleanup instructions presented as actions.
- No "AI health score".
