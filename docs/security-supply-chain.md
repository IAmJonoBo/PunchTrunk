# Security and Supply Chain

_This is an Explanation with actionable guidance._

## Binary Build

- `Dockerfile` uses a two-stage build: Go toolchain for compilation, distroless static runtime for execution.
- Output binary resides at `/app/trunk-orchestrator` and runs as the `nonroot` user.
- Writes must target writable paths (e.g., `/tmp`) when extending runtime behavior.

## Signing Guidance

- `make docker` builds the local image `trunk-orchestrator:local`.
- `make sign` is a placeholder; prefer `cosign sign --keyless trunk-orchestrator:local`.
- Publish guidance and signed image references in this file when release automation is added.

## Dependency Pinning

- Go dependencies are minimal; ensure `go.mod` stays on Go `1.20` while CI uses `1.22.x` to catch forward incompatibilities.
- Trunk plugins and linters are pinned in `.trunk/trunk.yaml`. Update pins only after testing locally and in a preview CI run.

## CI Secrets

- Current workflow does not require secrets. If you add authenticated registries or signing keys, store them in repository secrets and document the usage here.

## Supply Chain Risks and Mitigations

- **Unpinned Trunk runtime**: rely on `.trunk/trunk.yaml` commit to keep tool versions deterministic.
- **Distroless base updates**: monitor `gcr.io/distroless/static` release notes and rebuild the image when new CVEs publish.
- **SARIF ingestion**: ensure only trusted workflows upload SARIF. GitHub Code Scanning enforces repository permissions.
