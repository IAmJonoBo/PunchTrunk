# Privacy & Data Policy

## Data Classes

- **Source code:** primary data processed. Treated as confidential intellectual property.
- **Metadata:** commit SHAs, file paths, lint diagnostics, hotspot scores. Contains no personal data when repositories follow conventions (no secrets/PII).
- **User identifiers:** limited to GitHub usernames in CI logs when actions run; no direct storage beyond GitHub systems.

## Collection & Minimisation

- PunchTrunk operates locally or within CI. It reads repository files and generates SARIF findings. No network transmission occurs aside from SARIF upload to GitHub Code Scanning.
- Logs exclude file contents and redact potential secrets flagged by Trunk.

## Retention

- SARIF files remain in `reports/` until cleaned by repo maintainers; GitHub Code Scanning retains uploaded SARIF per GitHub’s default retention (90 days) unless manually removed.
- CI logs follow GitHub retention (default 90 days).
- No long-term storage of personal data by PunchTrunk.

## Data Subject Requests

- Because PunchTrunk does not persist personal data, data subject requests route to repository owners. If any personal data surfaces in SARIF or logs, maintainers must remove the artefact immediately and rotate affected credentials.

## Lawful Basis & Audit

- Usage is internal to n00tropic projects. Processing relies on legitimate interest in maintaining code quality.
- Audit trails: pull request history, CI logs, and SARIF uploads provide traceability. Maintainers review logs quarterly to ensure policies remain effective.
