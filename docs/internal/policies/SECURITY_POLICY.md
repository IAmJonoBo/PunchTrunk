# Security Policy

## Scope

- PunchTrunk is a Go CLI packaged as binaries and offline bundles. It orchestrates local tooling and produces SARIF reports. All controls apply to source, CI workflows, release artefacts, and provisioning scripts.

## Baseline Controls

- Follow OWASP ASVS Level 2 controls relevant to CLI tooling: input validation, safe subprocess execution, and secure logging.
- Enforce least privilege: workflows run without elevated permissions; ephemeral runners execute as non-root users.
- Secrets management: never commit credentials. Use GitHub Actions secrets if integration requires tokens (none currently).

## Threat Model

- Maintained in `docs/internal/policies/THREAT_MODEL.md`. Review every release cycle or after major dependency changes.

## Secure Development Lifecycle

- Require Trunk linting and hotspot scans before merge.
- Run Semgrep security scans (`make security` or `semgrep --config=semgrep/offline-ci.yml`) on all PRs to detect common security issues including shell command injection, unsafe patterns, and debug code.
- Run vulnerability scans (`trivy fs dist/`, `go list -m all | govulncheck`) prior to releases; document results in release notes.
- Security-sensitive changes (Trunk config, setup scripts, SARIF writer) need two maintainer approvals.

## Reporting & Response

- Report suspected vulnerabilities to `security@n00tropic.example`. Maintainers acknowledge within 24 hours and coordinate fixes following incident response playbook (`docs/internal/ops/INCIDENT_RESPONSE.md`).
- Do not disclose publicly until a fix is available and users are notified via release notes.

## Hard Rules

- Never commit secrets or production data.
- Never exfiltrate source code outside the authorised environment.
- Immediately rotate credentials if a secret leak is suspected and document the action.
