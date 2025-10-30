# Next Steps Log

- 2025-10-30: Created baseline tracking with tasks for offline bundle auto-download work, recorded initial QA results (Go tests failing; lint/security pending).
- 2025-10-30: Added automated download tests for Linux/macOS flows, implemented trunk auto-download fallback, noted missing dependencies (BATS, Trunk CLI, Semgrep) blocking full QA.
- 2025-10-30: Normalized offline bundle target naming, expanded shell coverage for default naming, and documented QA gaps (Go tests still failing on SARIF redirect expectations; lint/security blocked by missing tools).
- 2025-10-30: Implemented ancestor permission detection for SARIF fallbacks, installed Trunk CLI/Semgrep/BATS locally, reran QA (go test/go vet/build now green) while capturing outstanding failures in trunk check, semgrep SSL, and BATS bundle naming.
- 2025-10-30: Resolved shellcheck SC2312 in installer/airgap scripts, hardened offline bundle copying (BATS suite now green), introduced offline Semgrep config (`semgrep/offline-ci.yml`), and recorded refreshed QA results (go test, go vet, make build, bats, semgrep). Outstanding backlog remains for trunk lint formatting, yamllint, golangci-lint, markdownlint, and osv-scanner.
