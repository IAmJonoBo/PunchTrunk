# PunchTrunk Documentation Index

This index follows the [Diátaxis framework](https://diataxis.fr/), grouping developer-facing and operator-facing content into tutorials, how-to guides, explanations, and reference materials. Internal planning and quality gates now live under `docs/internal/` and are intentionally left out of this index.

## Tutorials

- _Coming soon_: end-to-end walkthroughs for new contributors and platform operators.

## How-to Guides

- [Integration Guide](INTEGRATION_GUIDE.md) — wire PunchTrunk into CI/CD systems, ephemeral runners, and agents.
- [Releasing PunchTrunk](releasing.md) — publish binaries and offline bundles.
- [Development Standards](development/REPO_CONVENTIONS.md) — day-to-day workflow expectations for contributors.

## Explanation

- [Product Overview](overview.md) — context, goals, and scope of the orchestrator.
- [Hotspots Methodology](hotspots-methodology.md) — how churn × complexity scoring works.
- [Testing Strategy](testing-strategy.md) — layers of automated assurance.
- [Security & Supply Chain](security-supply-chain.md) — threat model and mitigations.
- [Architecture Docs](architecture/) — macro and component-level design references.
- [AI System Specs](ai/) — agent evaluation and guardrail design notes.
- [Architecture Decision Records](adr/) — design rationale and history.

## Reference

- [CLI & Tooling Conventions](CONVENTIONS.md) — naming, formatting, and repository norms.
- [Trunk Configuration](trunk-config.md) — extending linters and formatters.
- [SARIF Schema Notes](SARIF_SCHEMA.md) — data contract for hotspot exports.
- [Community Guidelines](community/GOVERNANCE.md) — governance, contribution, and conduct policies.

## Internal Materials

Internal planning, quality gates, operational runbooks, and templates reside under [`docs/internal/`](internal/). These artifacts remain available for collaborators but are omitted from the public-facing index to avoid clutter and to preserve their specialised focus.
