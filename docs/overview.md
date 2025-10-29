# Overview

## Purpose

PunchTrunk bundles Trunk CLI operations and hotspot analysis inside a single Go binary. The orchestrator keeps formatter and linter execution hermetic while emitting SARIF so CI pipelines stay simple.

## Architecture at a Glance

- `cmd/trunk-orchestrator/main.go` parses flags, builds a timeout-scoped context, and runs Trunk commands (`trunk fmt`, `trunk check`).
- Hotspot scoring combines git churn data with a token density heuristic before writing SARIF (`reports/hotspots.sarif`).
- CI (`.github/workflows/ci.yml`) restores cached Trunk tools, builds the binary, runs hotspots, and uploads SARIF.
- Trunk configuration lives in `.trunk/trunk.yaml` with overrides under `.trunk/configs/` to keep dependencies pinned.

## Why It Exists

- Fast linting on ephemeral runners without custom scripts.
- A single binary that can ship in a distroless container.
- Consistent hotspot reporting to surface risky files for reviewers.

## Key Design Principles

- **Hermetic tooling**: all linting goes through Trunk to avoid ad-hoc installations.
- **Graceful degradation**: hotspot logic tolerates shallow clones and missing history.
- **Safe defaults**: formatters autofix by default; linters are warn-only unless `--autofix` expands scope.
- **Separation of concerns**: CLI orchestrates workflow steps; Trunk config expresses tool choices.

## Related Documents

- `docs/workflows/local-dev.md` for day-to-day commands.
- `docs/hotspots-methodology.md` for scoring details.
- `docs/trunk-config.md` for linters and runtime pinning.
- `docs/operations/ci.md` for pipeline integration.
