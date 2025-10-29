# ADR 0001: Trunk Orchestrator Core

## Status

Accepted

## Context

The repository needs a consistent way to run Trunk formatters and linters while publishing hotspot analytics. Teams want lightweight automation that works in ephemeral CI environments.

## Decision

- Implement a single Go CLI (`cmd/trunk-orchestrator/main.go`) that:
  - Parses execution flags and manages timeouts.
  - Invokes `trunk fmt` and `trunk check` instead of re-implementing lint logic.
  - Computes hotspots using git churn and complexity heuristics.
  - Emits SARIF 2.1 results to `reports/hotspots.sarif` for GitHub Code Scanning.
- Ship a distroless Docker image to run the CLI in CI with minimal surface area.
- Keep Trunk configuration self-contained under `.trunk/` so agents and contributors share the same toolchain.

## Consequences

- All linting and formatting behavior depends on Trunk; updates require editing `.trunk/trunk.yaml`.
- Hotspot scoring must remain tolerant of missing git history to serve ephemeral runners.
- The CLI binary becomes the integration point for additional modes; future features must preserve backwards compatibility for existing flags and SARIF schema.
