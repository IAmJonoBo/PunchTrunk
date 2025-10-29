# Observability Spec

## Logs

- PunchTrunk emits phase summaries (fmt, lint, hotspots) to stdout. Each log line includes timestamp, phase, and result. CI retains full logs for 30 days.
- Sensitive data: scrub file contents and secrets; log only file paths and counts. Enable verbose mode (`--verbose`) only in secure environments.
- Correlate CI runs using GitHub run ID and commit SHA; include both in log headers.

## Metrics

- **SLI: Runtime p95** for lint+hotspots phases (target <10 minutes). Derived from GitHub Actions step duration.
- **SLI: SARIF success rate** (percentage of runs uploading valid SARIF). Target ≥99%.
- **SLI: Trunk failure rate** (runs ending with exit code ≠0 due to lint violations). Track trend but do not set SLO; used for coaching.
- Burn rate alerts trigger if runtime p95 exceeds target for two consecutive days or SARIF success drops below 97%.

## Traces / Profiles

- Full distributed tracing is not required. During performance investigations, enable Go CPU/mem profiles via `PPROF=1 make run` (optional instrumentation) and store artefacts in `/tmp/profiles` for short-term analysis.

## Dashboards & Alerts

- Dashboard lives in Grafana (`PunchTrunk :: CI Health`) aggregating GitHub Actions metrics via the GitHub exporter.
- Alerts route to `#punchtrunk-alerts` Slack channel with PagerDuty escalation for SEV1 runtime/SARIF failures.
- Maintainers review dashboards during weekly sync; anomalies become follow-up issues with `observability` label.

## Golden Signals

- Traffic: number of CI runs per day (`lint-and-hotspots` job count).
- Errors: count of non-zero exits broken down by phase.
- Saturation: runner minutes consumed versus allocation; monitor to plan caching optimisations.
- Latency: already covered by runtime SLI.
