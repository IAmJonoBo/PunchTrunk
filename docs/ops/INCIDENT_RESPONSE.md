# Incident Response

## Severity Matrix

- **SEV1 (blocking CI / release):** PunchTrunk prevents merges or corrupts outputs. Response time: 15 minutes. Announce in `#punchtrunk-alerts` and pin status message.
- **SEV2 (degraded functionality):** Hotspots missing or Trunk annotations delayed but workarounds exist. Response time: 1 hour. Communicate in Slack and create tracking issue.
- **SEV3 (minor regression):** Cosmetic or low-risk bugs. Triage within one business day.

## On-Call & Handoff

- Maintainer rotation provides primary and secondary responders. Schedule lives in `docs/ops/oncall.md` (to be created if process changes).
- Primary triages alerts, secondary handles comms and logistics if incident exceeds 2 hours.
- Use shared incident doc (template: `docs/templates/Postmortem.md`) to capture timeline in real time.

## Triage Steps

1. Confirm alert validity (check CI logs, GitHub status, SARIF uploads).
2. Identify blast radius (affected repos, branches, releases).
3. Apply mitigations (revert workflow, pin prior binary, disable hotspots mode) while preserving artefacts for analysis.
4. Communicate status updates every 30 minutes for SEV1/2 incidents.

## Communication

- Internal: `#punchtrunk-alerts` Slack channel, maintainer email alias, issue tracker label `incident`.
- External (if hosted offering impacted): coordinate with n00tropic comms team before public statements.
- Templates for updates stored under `docs/templates/Postmortem.md` (timeline section) and release notes for customer-facing summaries.

## Postmortem

- Hold a blameless review within five business days. Required attendees: incident responders, affected developers, security rep if data risk occurred.
- Document root causes, contributing factors, and corrective actions using `docs/templates/Postmortem.md`.
- Track action items in issue tracker with owners and due dates; review status in weekly maintainer sync until resolved.
