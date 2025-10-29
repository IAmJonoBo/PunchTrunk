# Quality Gates (CI must pass)

- **Trunk fmt/check:** Zero blocking errors; hold-the-line violations fail the build.
- **Hotspot SARIF:** File generated and passes schema validation. Upload step must succeed or job retries.
- **Tests:** `go test ./...` (when implemented) must pass with coverage reported; thresholds documented in `docs/testing-strategy.md`.
- **Security scans:** Secrets detection, `govulncheck`, and container scan (Trivy) report no high/critical issues before release.
- **Docs:** README, AGENTS, and affected docs updated; markdownlint clean (`trunk fmt`).

Fail the build if any gate fails. No “just this once”.
