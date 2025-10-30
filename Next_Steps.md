# Next Steps

## Tasks

- [ ] Triage trunk lint/security backlog (shellcheck SC2250, YAML quoting, golangci-lint errcheck/unused, markdownlint, osv) (Owner: assistant, Due: TBC)
- [ ] Integrate offline Semgrep config (`semgrep/offline-ci.yml`) into automation and document usage (Owner: assistant, Due: TBC)
- [ ] Capture updated QA summary once remaining gates are green (Owner: assistant, Due: TBC)

## Steps

- [ ] Review trunk check findings and scope remediation plan
- [ ] Wire `semgrep/offline-ci.yml` into Makefile/Trunk workflows and update docs
- [ ] Draft QA/verification summary covering new tooling and outstanding backlogs

## Deliverables

- [ ] QA summary including baseline failures and post-change verification

## Quality Gates

- [ ] Trunk lint/format (`trunk fmt`, `trunk check`) – failing: shellcheck SC2250 warnings, YAML quoting, golangci-lint errcheck/unused, markdownlint MD033/MD040, and osv-scanner Go stdlib CVEs remain
- [ ] Security scan (Semgrep offline config) – integrate into automation (manual `semgrep --config=semgrep/offline-ci.yml` currently passing)
- [ ] Documentation updates – pending once lint/security backlog addressed

## Links

- [ ] Pending (add PR link once available)

## Risks/Notes

- Read-only SARIF fallback now checks ancestor permissions; ensure release notes mention new warning message when redirecting outputs.
- Trunk check backlog remains across shellcheck, GitHub Actions hardening, golangci-lint errcheck/unused, markdownlint, and osv-scanner results—requires prioritisation before release.
- Offline Semgrep config lives at `semgrep/offline-ci.yml`; ensure QA automation and documentation reference the new invocation.
- `build-offline-bundle` BATS suite now green; continue monitoring when adjusting bundle contents.
