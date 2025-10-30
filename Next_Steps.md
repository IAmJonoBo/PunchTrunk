# Next Steps

## Tasks

- [ ] Triage trunk lint/security backlog (shellcheck SC2250, YAML quoting, golangci-lint errcheck/unused, markdownlint, osv) (Owner: assistant, Due: TBC) - Note: Requires external tooling (Trunk CLI, golangci-lint, osv-scanner) not available in current environment
- [x] Integrate offline Semgrep config (`semgrep/offline-ci.yml`) into automation and document usage (Owner: assistant, Completed: 2025-10-30)
- [x] Capture updated QA summary once remaining gates are green (Owner: assistant, Completed: 2025-10-30)

## Steps

- [ ] Review trunk check findings and scope remediation plan - Note: Requires Trunk CLI installation
- [x] Wire `semgrep/offline-ci.yml` into Makefile/Trunk workflows and update docs
- [x] Draft QA/verification summary covering new tooling and outstanding backlogs

## Deliverables

- [x] QA summary including baseline failures and post-change verification (see QA_SUMMARY.md)

## Quality Gates

- [ ] Trunk lint/format (`trunk fmt`, `trunk check`) – failing: shellcheck SC2250 warnings, YAML quoting, golangci-lint errcheck/unused, markdownlint MD033/MD040, and osv-scanner Go stdlib CVEs remain - Note: Requires Trunk CLI and additional tooling
- [x] Security scan (Semgrep offline config) – integrated into automation via Makefile and CI workflow; documented in README.md and INTEGRATION_GUIDE.md
- [x] Documentation updates – complete with security scanning integration documented

## Links

- [ ] Pending (add PR link once available)

## Risks/Notes

- Read-only SARIF fallback now checks ancestor permissions; ensure release notes mention new warning message when redirecting outputs.
- Trunk check backlog remains across shellcheck, GitHub Actions hardening, golangci-lint errcheck/unused, markdownlint, and osv-scanner results—requires prioritisation before release.
- Offline Semgrep config lives at `semgrep/offline-ci.yml`; ensure QA automation and documentation reference the new invocation.
- `build-offline-bundle` BATS suite now green; continue monitoring when adjusting bundle contents.
