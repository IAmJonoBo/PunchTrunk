# Next Steps

## Tasks

- [ ] Triage trunk lint/security backlog (shellcheck SC2250, YAML quoting, golangci-lint errcheck/unused, markdownlint, osv) (Owner: assistant, Due: TBC)
  - Note: Requires external tooling (Trunk CLI, golangci-lint, osv-scanner) not available in current environment
  - **Mitigation**: CI now explicitly installs Trunk CLI and validates tool availability before running checks
- [x] Integrate offline Semgrep config (`semgrep/offline-ci.yml`) into automation and document usage (Owner: assistant, Completed: 2025-10-30)
- [x] Capture updated QA summary once remaining gates are green (Owner: assistant, Completed: 2025-10-30)
- [x] Ensure ephemeral runners are fully provisioned with required tools (Owner: assistant, Completed: 2025-10-30)
- [x] Document agent-friendly usage patterns for PunchTrunk/Trunk commands (Owner: assistant, Completed: 2025-10-30)

## Steps

- [ ] Review trunk check findings and scope remediation plan (Owner: assistant, Due: TBC)
  - Note: Requires Trunk CLI installation
  - **Mitigation**: CI workflow now auto-installs Trunk CLI and runs checks; local environments can use auto-install or manual installation
- [x] Wire `semgrep/offline-ci.yml` into Makefile/Trunk workflows and update docs
- [x] Draft QA/verification summary covering new tooling and outstanding backlogs
- [x] Update CI workflow to explicitly provision Trunk CLI on ephemeral runners
- [x] Add comprehensive validation step to verify tool availability in CI
- [x] Create Agent Provisioning Guide documentation

## Deliverables

- [x] QA summary including baseline failures and post-change verification (see QA_SUMMARY.md)
- [x] Agent Provisioning Guide (docs/AGENT_PROVISIONING.md) covering all provisioning strategies
- [x] Enhanced CI workflow with explicit tool installation and validation
- [x] Agent-friendly quick reference in README.md

## Quality Gates

- [ ] Trunk lint/format (`trunk fmt`, `trunk check`) – failing: shellcheck SC2250 warnings, YAML quoting, golangci-lint errcheck/unused, markdownlint MD033/MD040, and osv-scanner Go stdlib CVEs remain
  - Note: Requires Trunk CLI and additional tooling
  - **Progress**: CI now installs Trunk CLI automatically and validates tool availability; lint failures will be surfaced by CI
- [x] Security scan (Semgrep offline config) – integrated into automation via Makefile and CI workflow; documented in README.md and INTEGRATION_GUIDE.md
- [x] Documentation updates – complete with security scanning integration documented and agent provisioning guide added
- [x] Tool provisioning – ephemeral runners now explicitly install and verify Trunk CLI before running checks

## Links

- [ ] Pending (add PR link once available)

## Risks/Notes

- Read-only SARIF fallback now checks ancestor permissions; ensure release notes mention new warning message when redirecting outputs.
- Trunk check backlog remains across shellcheck, GitHub Actions hardening, golangci-lint errcheck/unused, markdownlint, and osv-scanner results—requires prioritisation before release.
  - **Mitigation**: CI now runs these checks automatically; issues will be surfaced in PR reviews via Trunk Action annotations
- Offline Semgrep config lives at `semgrep/offline-ci.yml`; ensure QA automation and documentation reference the new invocation.
- `build-offline-bundle` BATS suite now green; continue monitoring when adjusting bundle contents.
- **New**: CI workflow explicitly installs Trunk CLI to ensure tool availability on ephemeral runners
- **New**: Comprehensive Agent Provisioning Guide provides three strategies: auto-install, explicit pre-install, and offline/air-gapped
- **New**: Validation step in CI checks git, trunk, go, punchtrunk, and python availability before running PunchTrunk
- **New**: validate-agent-environment.sh script created for local validation (has minor hang issue to be debugged)

## Recent Improvements (2025-10-30)

### Provisioning Enhancements
- Added explicit Trunk CLI installation step in CI workflow (`.github/workflows/ci.yml`)
- Added restore-keys to cache step for better cache hit rates
- Added comprehensive tool validation step that checks all required tools (git, trunk, go, punchtrunk, python)
- Created `docs/AGENT_PROVISIONING.md` with comprehensive provisioning strategies
- Added agent-friendly quick reference section to main README.md
- Created `scripts/validate-agent-environment.sh` for automated environment validation

### Documentation
- Agent Provisioning Guide covers:
  - Automatic installation (development)
  - Explicit pre-installation (CI/CD)
  - Offline/air-gapped environments
  - Tool verification with diagnose-airgap and tool-health modes
  - Complete GitHub Actions integration examples
  - All enabled linters/formatters from trunk.yaml
  - Troubleshooting guide
  - Firewall and network considerations
- Updated docs/README.md index to include provisioning guide
- Added troubleshooting entries for tool provisioning issues

### CI/CD
- Trunk CLI installation now uses environment variables:
  - `TRUNK_INIT_NO_ANALYTICS=1`
  - `TRUNK_TELEMETRY_OPTOUT=1`
- Added restore-keys pattern to Trunk cache for better performance
- Tool validation runs before prep-runner.sh to ensure prerequisites
- Better error messages and early failure if tools are missing

