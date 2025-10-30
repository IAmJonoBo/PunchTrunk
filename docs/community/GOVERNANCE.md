# Governance

## Stewardship

- Maintainers: **n00tropic** platform engineering team (rotating pair). They own roadmap curation, dependency upgrades, and final say on releases.
- Contributors with sustained activity (>3 merged PRs in a quarter) may be invited to join maintainer rotation.

## Decision-Making

- Everyday changes follow lazy consensus in pull requests. Silence for 48 business hours implies approval.
- Significant technical changes require ADRs under `docs/architecture/ADRs` and at least two maintainer approvals.
- Disagreements escalate to the platform engineering manager, who facilitates resolution within three business days.

## Release Authority

- Release managers: maintainer on rotation finalises tags, GitHub releases, and Docker pushes. A backup maintainer reviews artefacts before publication.
- Emergency fixes: any maintainer may cut a patch release after notifying the team in `#punchtrunk-alerts` and documenting the rationale in the release notes.

## Security Contact

- Report vulnerabilities to `security@n00tropic.example`. A maintainer acknowledges within 24 hours and coordinates disclosure following `docs/internal/policies/SECURITY_POLICY.md`.

## Community Conduct

- PunchTrunk adopts the Contributor Covenant (see root `CONTRIBUTING.md`). Maintainers enforce the code of conduct and may revoke contributor permissions for repeated violations.
