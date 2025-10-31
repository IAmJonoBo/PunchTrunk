# QA Summary – October 31, 2025

## Baseline vs Post-Change Verification

| Check                | Command                                        | Status | Notes                                                                                  |
| -------------------- | ---------------------------------------------- | ------ | -------------------------------------------------------------------------------------- |
| Build                | `make build`                                   | ✅     | No regressions introduced by shell updates.                                            |
| Go unit + E2E        | `make test`                                    | ✅     | Full suite (unit, integration, BATS) completes in 14.5s.                               |
| Lint (hold-the-line) | `trunk fmt && trunk check`                     | ✅     | No new violations; existing backlog documented below.                                  |
| Security scan        | `make semgrep`                                 | ✅     | Uses bundled rules under `semgrep/`, writes `reports/semgrep.sarif` without TLS calls. |
| Hotspot SARIF        | `./bin/punchtrunk --mode hotspots` (via tests) | ✅     | Still redirects to temp when workspace is read-only.                                   |

## Lint & Security Backlog Triage

The following findings pre-date this change set and remain active. Counts come from `trunk check --all --filter <linter> --no-progress`.

- **shellcheck** (`scripts/build-offline-bundle.sh`): 15 warnings spanning SC2310/SC2312 (masked return values), SC2155 (combined declaration), SC2034 (unused), SC2269 (self-assignment). Fixing will require refactoring installer sections without regressing hydration behavior.
- **yamllint** (GitHub Actions + Semgrep rule config): 13 redundant quoting violations (`quoted-strings`) in `.github/workflows/e2e.yml`, `.github/workflows/release.yml`, and `semgrep/print-debug.yml`.
- **golangci-lint (errcheck/unused)** (`cmd/punchtrunk/main.go`): 7 issues – one unused helper (`tempDir`) and six unchecked `fmt.Fprint*` calls in diagnostic output blocks.
- **markdownlint** (`docs/internal/delivery/E2E_STRATEGY.md`, `docs/internal/templates/*.md`): 18 issues (MD040 missing info strings on fenced blocks, MD033 inline HTML in templates). Requires doc clean-up or lint suppressions.
- **osv-scanner** (`go.mod` stdlib 1.22.99): 15 Go standard-library CVEs outstanding; blocked on upstream security release (1.22.x backport).

## Semgrep Hardening

- Added `scripts/run-semgrep.sh` plus a `make semgrep` target to execute Semgrep with repository-local rules (`semgrep/`) and `--disable-version-check` to avoid TLS verification failures.
- Script accepts an optional `SEMGREP_BIN` override so air-gapped runners can point at a pre-installed binary (e.g., shipped inside offline bundles).
- Output persists to `reports/semgrep.sarif` for CI ingestion; directory already ignored via `.gitignore`.

## Outstanding Work

- Track remediation of each lint backlog bucket (shellcheck, yamllint, golangci-lint, markdownlint, osv-scanner) before release cut.
- Bundle semgrep binary (or wheel) in offline artifacts to pair with the new runner script.
- Update release notes to mention the new Semgrep workflow and the `reports/semgrep.sarif` artifact for security gates.
