# Project Charter

**Working title:** PunchTrunk  
**Problem worth solving:** Teams need hermetic linting, safe autofix, and actionable hotspot insights without bespoke scripts.  
**Success criteria (measurable):** CI fmt+lint+hotspots runtime <10 minutes p95; ≥99% SARIF upload success; 100% target repos adopting PunchTrunk workflow.  
**Non-negotiables:** Pinned toolchain (Go 1.22.x, Trunk versions), no secrets/PII exposure, runs as nonroot, adheres to n00tropic security/privacy policies.

## Scope (what’s in / out)

- **In:** Go CLI orchestrator, Trunk integration, hotspot scoring, SARIF generation, GitHub Actions workflow, documentation, Docker packaging.
- **Out:** Hosting managed Trunk services, building custom linters, editing upstream Trunk code.

## Stakeholders & RACI

- **Product:** n00tropic platform PM (Responsible for roadmap alignment, Accountable for adoption goals).
- **Engineering:** PunchTrunk maintainers (Responsible for implementation and releases).
- **Design:** Developer experience writer (Consulted on docs tone and examples).
- **AI/ML:** AI enablement lead (Consulted on guardrails and evaluation strategy).
- **QA/Ops:** CI operations engineer (Responsible for workflow reliability; Informed on releases/incidents).

## Risks & Assumptions

- **Big rocks:** Maintaining Trunk cache efficiency on ephemeral runners; ensuring hotspot heuristics stay predictive; keeping documentation in sync across Diátaxis set.
- **Unknowns to spike:** Automated hotspot regression tests with fixtures; performance on large monorepos; SARIF validation automation in CI.

> Keep this punchy. If it can’t fit on two pages, you’re writing a novel.
