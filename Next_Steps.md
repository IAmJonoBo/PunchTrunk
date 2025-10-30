# Next Steps

## Tasks

- [x] Implement auto-download fallback in `scripts/build-offline-bundle.sh` when the default trunk binary is missing (Owner: assistant, Due: TBC)
- [x] Extend shell test coverage for offline bundle auto-download scenarios (Owner: assistant, Due: TBC)
- [x] Align offline bundle naming with normalized target OS/arch inputs (Owner: assistant, Due: TBC)
- [ ] Triage trunk lint/security backlog (shellcheck, YAML quoting, golangci-lint errcheck, markdownlint, osv) (Owner: assistant, Due: TBC)
- [ ] Resolve Semgrep SSL failure by pinning an offline/locally cached rule set (Owner: assistant, Due: TBC)
- [ ] Repair `build-offline-bundle` BATS default bundle naming regression (Owner: assistant, Due: TBC)

## Steps

- [x] Capture baseline QA results and identify existing failures
- [x] Design test cases for Linux and macOS auto-download flows
- [x] Update shell script implementation to track user-supplied binaries and emit clear logs
- [x] Normalize target overrides and default bundle naming, update docs/tests accordingly
- [x] Validate changes via automated tests and document results
- [x] Install Trunk CLI, Semgrep, and BATS tooling locally for full gate coverage
- [ ] Review trunk check findings and scope remediation plan
- [ ] Establish Semgrep execution that succeeds in air-gapped/strict SSL environments
- [ ] Restore BATS suite to green for bundle naming scenario

## Deliverables

- [x] Updated offline bundle build script reflecting auto-download logic and normalized naming defaults
- [x] New/updated shell tests covering missing-local-binary and naming flows
- [ ] QA summary including baseline failures and post-change verification

## Quality Gates

- [x] Go tests (`go test ./...`) – passing at 70.9% coverage after read-only fallback detection fix
- [ ] Trunk lint/format (`trunk fmt`, `trunk check`) – failing: widespread shellcheck SC2250/SC2312 warnings, GitHub Actions permission/quoting issues (checkov/yamllint), golangci-lint unused/errcheck findings, markdownlint MD033/MD040, and osv-scanner Go stdlib CVEs
- [x] Go vet (`go vet ./...`)
- [ ] Security scan (Semgrep or equivalent) – failing: `semgrep --config=auto` terminated with SSL certificate verification error
- [x] Build (`make build`)
- [ ] Shell tests (`bats scripts/tests`) – failing: `default bundle name honours normalized target overrides` still red (trunk.exe assertion)

## Links

- [ ] Pending (add PR link once available)

## Risks/Notes

- Read-only SARIF fallback now checks ancestor permissions; ensure release notes mention new warning message when redirecting outputs.
- Trunk check exposes sizeable backlog across shellcheck, GitHub Actions hardening, golangci-lint errcheck/unused, markdownlint, and osv-scanner results—requires prioritisation before release.
- Semgrep registry access fails TLS verification under current runner; need offline bundle or CA pinning to unblock security gates.
- `build-offline-bundle` BATS suite still fails on Windows bundle expectation (missing `trunk.exe` in archive) and must be corrected prior to release packaging.
- Normalised naming change still requires comms in release notes for downstream automation consumers.
