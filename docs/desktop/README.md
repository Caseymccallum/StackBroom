# StackBroom Desktop Documentation

- [Architecture](ARCHITECTURE.md)
- [Dependency licences](DEPENDENCIES.md)
- [Visual, responsive and accessibility review](DESIGN-REVIEW.md)
- [Validation evidence](VALIDATION.md)
- [Screenshots](screenshots/)
- [Optional BYOK AI adviser — design only](BYOK-AI.md)
- [M2 storage intelligence implementation](../product/M2-IMPLEMENTATION.md)

Inspection remains read-only. The only mutation capability is explicit, verified process control (graceful close, termination and non-real-time priority). It does not clean up files, remove containers or images, change startup/Windows settings, contact an AI provider or open a production network listener.

## Desktop UX refinement

The implementation and native validation following the frozen desktop foundation are documented in [UX-DELIVERY.md](UX-DELIVERY.md), with a [ranked audit](UX-AUDIT.md), [future controls contract](FUTURE-CONTROLS.md), and an expanded [screenshot gallery](screenshots/ux-polish/). The earlier validation document describes the foundation baseline; the newer report describes the UX refinement.
