# Release Strategy

## Distribution

- Free, open-source Windows desktop application.
- Primary artifact: signed Windows installer.
- Secondary artifact: portable executable for advanced users.
- CLI remains available and tested independently.
- No forced account, subscription or cloud dependency for core features.

## Build provenance

- Reproducible commands documented in `docs/desktop` and `README.md`.
- Every release records source SHA, build script, Go/Node/Wails versions and artifact SHA-256.
- Screenshots must be captured from the exact artifact described.
- A build is not published merely because it compiles.

## Signing

- Authenticode signing is required before public distribution.
- Signing certificate and timestamping are release-operator responsibilities, never hard-coded.
- Unsigned developer builds are labelled as such and not presented as releases.

## Updates

- Update checks must be explicit, privacy-respecting and optional.
- Signed update manifests; no silent privilege escalation.
- Rollback path documented.
- Never auto-update into a build with unreviewed action capabilities.

## Compatibility

- Windows 10/11 supported baseline (exact versions tested listed per release).
- WebView2 runtime requirement documented.
- x64 first; arm64 only after verification.

## Release gates

1. Unit, integration and frontend tests pass.
2. Refusal and privacy tests pass.
3. No unreviewed destructive capability is reachable.
4. Accessibility smoke test in both themes and both reference resolutions.
5. Owner approval for any live destructive validation.
6. Artifact hash recorded.
7. Changelog and known limitations updated.

## Governance

- Feature work lands on dedicated branches; no direct release from an unreviewed branch.
- Security-sensitive changes require a second review and negative-path tests.
- A bot or self-review is not independent validation.
- Do not merge, tag, sign or publish without explicit owner authorisation.
