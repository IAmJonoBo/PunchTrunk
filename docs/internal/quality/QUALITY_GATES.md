# Quality Gates (CI must pass)

_This document defines the quality gates enforced at each stage of the PunchTrunk development and deployment pipeline._

## Core Quality Gates

### 1. Code Quality Gates

- **Trunk fmt/check:** Zero blocking errors; hold-the-line violations fail the build.
- **Linting:** All configured linters pass without warnings on changed files.
- **Compilation:** Binary builds successfully on all target platforms.
- **Code Review:** At least one maintainer approval required.

### 2. Testing Gates

- **Unit Tests:** `go test ./...` must pass with 100% success rate.
- **E2E Tests:** All E2E scenarios pass (happy path, error handling, multi-language).
- **Kitchen Sink Test:** Comprehensive end-to-end validation passes.
- **Integration Tests:** Real Trunk CLI integration works correctly.
- **Evaluation Suite:** `make eval-hotspots` passes, matching SARIF baseline fixtures.
- **Coverage:** Maintain >80% coverage for core logic; thresholds in `docs/testing-strategy.md`.

### 3. Security Gates

- **Secrets Detection:** No secrets committed to repository.
- **Vulnerability Scanning:** `govulncheck` reports no high/critical issues.
- **Offline Bundle Integrity:** Bundles rebuilt, checksums verified, and manifests updated.
- **CodeQL Scanning:** No new security vulnerabilities introduced.
- **Dependency Audit:** All dependencies reviewed and up-to-date.

### 4. Output Validation Gates

- **Hotspot SARIF:** File generated and passes SARIF 2.1.0 schema validation.
- **SARIF Upload:** Upload to GitHub Code Scanning succeeds.
- **JSON Validity:** All JSON outputs are valid and well-formed.
- **File Outputs:** All expected output files are generated.

### 5. Documentation Gates

- **README:** Updated if CLI flags or usage changed.
- **CHANGELOG:** Entry added for user-facing changes.
- **AGENTS.md:** Updated if architecture or workflows changed.
- **Related Docs:** All affected documentation updated.
- **Markdown Linting:** All docs pass markdownlint (`trunk fmt`).

### 6. Performance Gates

- **CI Duration:** Full pipeline completes in < 10 minutes p95.
- **Test Duration:** E2E tests complete in < 5 minutes.
- **Hotspot Computation:** Completes within timeout (< 2 minutes for typical repos).
- **Memory Usage:** Peak memory usage < 500MB for standard operations.

## Gate Enforcement

**Rule:** Fail the build if any gate fails. No "just this once".

### Required for PR Merge

- All code quality gates ✓
- All testing gates ✓
- All security gates ✓
- All output validation gates ✓
- Documentation gate (if applicable) ✓

### Required for Release

- All PR merge gates ✓
- Performance gates ✓
- Offline bundle integrity check ✓
- Multi-platform build validation ✓
- Release documentation ✓

## Quality Metrics

### Success Criteria

- Test success rate: >99%
- CI pipeline duration: <10 minutes (p95)
- Zero high/critical security vulnerabilities
- SARIF upload success rate: 100%
- No regressions in hotspot accuracy

### Continuous Monitoring

- CI job duration trends
- Test failure patterns
- Security scan results
- Performance metrics
- User-reported issues

## Bypass Procedures

**Emergency Bypass:** Only allowed for critical production incidents

- Requires two maintainer approvals
- Must document in incident ticket
- Complete bypassed checks within 24 hours
- Full post-incident review required

**Never Bypass:** Security gates cannot be bypassed under any circumstances

## References

For detailed information about each gate:

- [E2E Strategy](../delivery/E2E_STRATEGY.md) - Comprehensive E2E approach
- [Testing Strategy](../testing-strategy.md) - Test coverage and approach
- [CI Operations](../operations/ci.md) - CI implementation details
- [Security Policy](../policies/SECURITY_POLICY.md) - Security requirements
