# Next Steps

## Tasks
- [x] Implement auto-download fallback in `scripts/build-offline-bundle.sh` when the default trunk binary is missing (Owner: assistant, Due: TBC)
- [x] Extend shell test coverage for offline bundle auto-download scenarios (Owner: assistant, Due: TBC)
- [x] Align offline bundle naming with normalized target OS/arch inputs (Owner: assistant, Due: TBC)

## Steps
- [x] Capture baseline QA results and identify existing failures
- [x] Design test cases for Linux and macOS auto-download flows
- [x] Update shell script implementation to track user-supplied binaries and emit clear logs
- [x] Normalize target overrides and default bundle naming, update docs/tests accordingly
- [ ] Validate changes via automated tests and document results

## Deliverables
- [x] Updated offline bundle build script reflecting auto-download logic and normalized naming defaults
- [x] New/updated shell tests covering missing-local-binary and naming flows
- [ ] QA summary including baseline failures and post-change verification

## Quality Gates
- [ ] Go tests (`go test ./...`) – failing: read-only SARIF fallback expectations remain (baseline issue)
- [ ] Trunk lint/format (`trunk fmt`, `trunk check`) – blocked: Trunk CLI absent in container
- [x] Go vet (`go vet ./...`)
- [ ] Security scan (Semgrep or equivalent) – not installed in environment
- [x] Build (`make build`)

## Links
- [ ] Pending (add PR link once available)

## Risks/Notes
- Baseline Go tests currently failing in `TestRunHotspotsRedirectsOnReadOnlyWorkspace` and `TestRunHotspotsUsesCustomTmpDirFallback`; need follow-up investigation if failures persist post-change.
- Trunk CLI absent locally, blocking lint/format until bootstrap or download strategy decided.
- Semgrep not installed in environment; evaluate installation vs. documenting limitation.
- BATS CLI unavailable locally; shell tests cannot be executed until dependency is installed.
- Normalized naming changes require release notes once bundled for distribution to inform downstream automation.
