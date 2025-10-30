# Overview

## Purpose

PunchTrunk bundles Trunk CLI operations and hotspot analysis inside a single Go binary. The orchestrator keeps formatter and linter execution hermetic while emitting SARIF so CI pipelines stay simple.

> Rebranding: older materials may reference `trunk-orchestrator`; the binary, release assets, and documentation now use the PunchTrunk name.
> Ownership: the project is authored and maintained by **n00tropic**; PunchTrunk is the tool, not the company.

## Architecture at a Glance

- `cmd/punchtrunk/main.go` parses flags, builds a timeout-scoped context, and runs Trunk commands (`trunk fmt`, `trunk check`). It now bootstraps prerequisites via `ensureEnvironment`, auto-installing the Trunk CLI into `~/.trunk/bin` (or `%USERPROFILE%\.trunk\bin`) when it is missing, normalising `--trunk-config-dir`, validating user-supplied binaries from `--trunk-binary`/`PUNCHTRUNK_TRUNK_BINARY`, and failing fast when `PUNCHTRUNK_AIRGAPPED=1` disallows downloads. Detected overlaps with other formatter/linter configs trigger informational guidance so operators can scope PunchTrunk with `--trunk-arg` filters when needed.
- Hotspot scoring combines git churn data with a token density heuristic before writing SARIF (`reports/hotspots.sarif`); on read-only workspaces the CLI redirects output to `/tmp/punchtrunk/reports/<filename>` and logs the new location for upload steps.
- CI (`.github/workflows/ci.yml`) restores cached Trunk tools, builds the binary, runs hotspots, and uploads SARIF.
- Trunk configuration lives in `.trunk/trunk.yaml` with overrides under `.trunk/configs/` to keep dependencies pinned.

## Why It Exists

- Fast linting on ephemeral runners without custom scripts.
- A single offline bundle that travels with Trunk, configs, and cached toolchains.
- Consistent hotspot reporting to surface risky files for reviewers.

## Key Design Principles

- **Hermetic tooling**: all linting goes through Trunk to avoid ad-hoc installations.
- **Graceful degradation**: hotspot logic tolerates shallow clones and missing history by retrying git queries with progressively shorter history windows and falling back to empty datasets when no commits exist.
- **Safe defaults**: formatters autofix by default; linters are warn-only unless `--autofix` expands scope.
- **Separation of concerns**: CLI orchestrates workflow steps; Trunk config expresses tool choices.

## Related Documents

- `docs/internal/workflows/local-dev.md` for day-to-day commands.
- `docs/hotspots-methodology.md` for scoring details.
- `docs/trunk-config.md` for linters and runtime pinning.
- `docs/internal/operations/ci.md` for pipeline integration.
- `docs/ai/` for agent guardrails, evaluation plans, and Copilot usage.
- `docs/development/` for coding standards and toolchain links.
- `docs/internal/delivery/` for release and CI summaries backed by the core guides above.
- `docs/internal/quality/` for QA checklists and quality gates.
- `docs/internal/templates/` for ADR, RFC, postmortem, and agent evaluation templates.

## Ongoing Maintenance

- Revisit `go.mod` and `.trunk/trunk.yaml` quarterly to align with supported Go and Trunk CLI versions.
- When bumping runtime or linter versions, update `docs/trunk-config.md` and `docs/internal/workflows/local-dev.md` to keep setup steps accurate.
- Capture new architectural decisions as ADRs under `docs/adr/` so changes stay discoverable for future contributors.
