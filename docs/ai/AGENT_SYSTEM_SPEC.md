# Agent System Specification

## Roles & responsibilities

- **Orchestrator:** PunchTrunk CLI sequences Trunk fmt/check, hotspot computation, and SARIF generation under a shared timeout context.
- **Worker agents (by capability):** Trunk CLI (formatting/linting per language), Git CLI (churn analytics). Both run as deterministic helpers with pinned versions.
- **Human‑in‑the‑loop moments:** Developers review Trunk annotations and SARIF before merging; platform maintainers approve config changes; security reviews SARIF uploads when findings touch sensitive areas.

## Tools & permissions

- **Trunk CLI:** Inputs: repository contents. Outputs: formatted files, lint diagnostics. Permissions: read/write workspace. Rate limits: local execution.
- **Git CLI:** Inputs: git history. Outputs: diff and churn stats. Permissions: read `.git` metadata. No external network required.
- **Filesystem:** Writes restricted to `bin/` for binaries and `reports/` for SARIF; all other paths read-only.

## Contracts

- **Inputs:** Command-line flags, `.trunk/trunk.yaml`, accessible git history. Requires Go 1.22 runtime when building locally.
- **Outputs:** SARIF 2.1 (`ruleId` = `hotspot`, level `note`), stdout/stderr logs, non-zero exits when any phase fails.
- **Error handling:** Transient failures (tool downloads) bubble to CI for retry; deterministic failures (lint errors) require contributor fixes. PunchTrunk preserves the first lint failure via `exitErr`.

## Safety & guardrails

- **Prompt hygiene:** See `docs/ai/COPILOT_USAGE.md` and `.github/copilot-instructions.md`; AI assistance documented in PRs.
- **Red‑teaming:** Periodic synthetic repos with seeded issues ensure hotspots and lint catch regressions.
- **Data boundaries:** Source code stays local/CI; SARIF uploaded only to GitHub Code Scanning; no PII allowed. Secrets detection enforced via Trunk.
- **Escalation:** Security contact in `docs/internal/policies/SECURITY_POLICY.md`; CI failure on SARIF upload triggers maintainer review.

## Evaluation

- **Offline:** Sample repos spanning languages validate formatter coverage and hotspot accuracy.
- **Online:** CI metrics (success rate, runtime), SARIF ingestion success, count of new lint violations.
- **Cadence:** Quarterly review; ad-hoc postmortems after incidents documented in `docs/internal/templates/Postmortem.md`.
