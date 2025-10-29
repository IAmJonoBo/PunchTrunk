# CI Pipeline

For the authoritative walkthrough, see `../operations/ci.md`. This outline keeps the stage order visible at a glance and defers details to the runbook so we maintain one source of truth.

**Stages:** lint → typecheck → test → build → security scans → package → (optional) E2E → publish.

- Cache dependencies sensibly; kill flakiness fast.
- Parallelise where possible; keep feedback under 10 minutes.
- Generate SBOM; upload coverage & reports.
