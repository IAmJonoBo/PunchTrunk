# Deployment Pipeline

_This document describes the complete deployment pipeline for PunchTrunk from development to production._

## Pipeline Overview

```text
┌─────────────┐     ┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Local     │────▶│   PR CI     │────▶│   Main CI   │────▶│   Release   │
│   Dev       │     │   (E2E)     │     │   (Full)    │     │   Deploy    │
└─────────────┘     └─────────────┘     └─────────────┘     └─────────────┘
      │                    │                    │                    │
      │                    │                    │                    │
      ▼                    ▼                    ▼                    ▼
  make build          Unit Tests          Integration          Multi-platform
   make run            E2E Tests           Performance          Bundle publish
  go test             Quality Gates       Security scan        GitHub Release
```

## Stage 1: Local Development

### Developer Workflow

```bash
# 1. Make changes
vim cmd/punchtrunk/main.go

# 2. Format and lint
make fmt
trunk check

# 3. Build
make build

# 4. Test locally
go test -v ./...

# 5. Run full pipeline
make run

# 6. Commit
git add .
git commit -m "feat: add new feature"
git push
```

### Quality Checks

- Code formatted (trunk fmt)
- Linters pass (trunk check)
- Unit tests pass
- Binary builds successfully
- Manual validation of changes

### Local Exit Criteria

- All tests pass locally
- Code is properly formatted
- Self-review completed
- Ready to create PR

## Stage 2: Pull Request CI

### Workflow: `.github/workflows/ci.yml`

#### Steps

1. **Checkout** (fetch-depth: 0 for git history)
2. **Cache Trunk** (speeds up linter startup)
3. **Trunk Check** (inline annotations on PR)
4. **Setup Go** (1.22.x)
5. **Build Binary**
6. **Runner Preflight** (`scripts/prep-runner.sh` hydrates Trunk caches, runs `tool-health`, and publishes Markdown/JSON reports while recording warnings without failing the job)
7. **Run All Tests** (unit + E2E)
8. **Run Hotspots**
9. **Upload SARIF** (to Code Scanning)

### Quality Gates (Must Pass)

- ✓ All Trunk checks pass
- ✓ Binary compiles successfully
- ✓ All tests pass (unit + E2E)
- ✓ Kitchen sink test validates full pipeline
- ✓ SARIF generated and valid
- ✓ SARIF uploads successfully
- ✓ Code review approved

### Parallel Workflow: `.github/workflows/e2e.yml`

#### Jobs

1. **E2E Tests** (all E2E scenarios)
2. **Integration with Trunk** (real CLI integration via `scripts/run-quality-suite.sh`, which first runs `prep-runner.sh` and publishes the same preflight/tool-health artifacts we rely on in CI)
3. **Performance Check** (< 2 min for hotspots)
4. **Quality Gate Summary** (all gates passed)

### Pull Request Exit Criteria

- All CI checks pass
- E2E workflow completes successfully
- Code review approved by maintainer
- All conversations resolved
- Ready to merge

## Stage 3: Main Branch CI

### Workflow: Same as PR CI + Additional Checks

#### Additional Steps

1. **Full Test Suite** (no test filtering)
2. **Performance Benchmarks** (baseline tracking)
3. **Coverage Report** (upload to coverage service)
4. **Build Validation** (ensure main is healthy)

### Publish Quality Gates

- All PR gates plus:
- ✓ No test flakiness detected
- ✓ Performance within baseline
- ✓ Coverage maintained or improved

### Failure Response

- **Critical Failure**:
  - Block all PRs immediately
  - Page on-call engineer
  - Fix or revert within 1 hour
- **Non-Critical**:
  - Create issue for investigation
  - Track in daily standup
  - Fix in next PR

### Publish Exit Criteria

- Main branch is healthy
- All tests passing
- Performance stable
- Ready for release

## Stage 4: Release Pipeline

### Trigger

- Manual: Create release tag (vX.Y.Z)
- Or: Push tag matching `v*.*.*` pattern

### Workflow: `.github/workflows/release.yml` (future)

#### Phase 1: Pre-Release Validation

```bash
# 1. Validate version tag
- Check semantic versioning
- Verify CHANGELOG updated
- Validate release notes

# 2. Full test suite
- Run all tests on release branch
- Run E2E suite on clean environment
- Kitchen sink validation

# 3. Security scan
- govulncheck
- Offline bundle integrity check (checksum + manifest)
- Dependency audit
- punchtrunk --mode tool-health (run against each bundle via punchtrunk-airgap.env)
```

#### Phase 2: Build Artifacts

```bash
# 1. Multi-platform binaries
- linux-amd64
- linux-arm64
- darwin-amd64 (Intel Mac)
- darwin-arm64 (Apple Silicon)
- windows-amd64

# 2. Offline bundles
- linux-amd64 bundle
- linux-arm64 bundle
- darwin-arm64 bundle
- windows-amd64 bundle
- Scan with Trivy
- Generate SBOM
- Review manifest hydration_status / warnings (rerun builder or inspect logs if caches were not fully prefetched)

# 3. Checksums
- Generate SHA256 checksums
- Create checksums.txt
```

#### Phase 3: Sign Artifacts

```bash
# 1. Sign binaries (optional)
- Use cosign with keyless OIDC
- Generate signatures

# 2. Verify bundle checksums
- Regenerate SHA-256 sums for each artifact
- Compare against committed manifest before publish
```

#### Phase 4: Publish

```bash
# 1. GitHub Release
- Create release with notes
- Upload all binaries
- Upload checksums
- Upload signatures

# 2. Update documentation
- Update README with new version
- Update installation instructions
- Deploy docs to GitHub Pages (if applicable)
```

### Quality Gates

- ✓ All pre-release validations pass
- ✓ Builds succeed on all platforms
- ✓ Offline bundle integrity validated
- ✓ Signatures generated successfully
- ✓ Release notes complete
- ✓ Artifacts uploaded successfully

### Monitoring Exit Criteria

- Release published on GitHub
- Offline bundles available
- Documentation updated
- Release announcement sent
- Monitoring configured

## Stage 5: Post-Release Monitoring

### Monitoring Period: 24-48 hours

#### Metrics to Watch

1. **CI Success Rate**
   - Monitor repos using new version
   - Track success/failure rates
   - Alert on regression

2. **SARIF Upload Success**
   - Ensure uploads working
   - Monitor for schema errors
   - Track upload latency

3. **Performance**
   - Hotspot computation time
   - Memory usage
   - CI pipeline duration

4. **Error Rates**
   - Application errors
   - Git operation failures
   - Trunk CLI integration issues

5. **User Feedback**
   - GitHub issues filed
   - Discussion forum activity
   - Direct user reports

### Response Procedures

#### Critical Issue (P0)

- Immediate rollback or hotfix
- Page on-call team
- Communication to users
- Post-incident review

#### Major Issue (P1)

- Hotfix release within 24 hours
- Update release notes
- Communication to users

#### Minor Issue (P2)

- Fix in next release
- Track in backlog
- Document workaround if needed

### Exit Criteria

- 24 hours with no critical issues
- Success rate within expected range
- No performance regressions
- User feedback positive or neutral

## Rollback Procedures

### Scenario 1: Pre-Release (Release Job Failed)

```bash
# 1. Fix issue
- Address build/test failure
- Re-run validation

# 2. Or delay release
- Update release notes with new date
- Communicate to stakeholders
```

### Scenario 2: Post-Release (Critical Bug Found)

```bash
# Option A: Hotfix
1. Create hotfix branch from release tag
2. Fix critical issue
3. Fast-track through CI
4. Release vX.Y.Z+1

# Option B: Rollback
1. Revert problematic commits on main
2. Re-release previous version
3. Update GitHub release notes
4. Communicate rollback to users

# Option C: Workaround
1. Document workaround in release notes
2. Update documentation
3. Fix in next planned release
```

### Rollback Communication

- Update GitHub release notes
- Post in GitHub Discussions
- Update documentation
- Notify dependent projects

## Continuous Improvement

### Metrics to Track

- Pipeline duration (trend over time)
- Test success rate
- Time to merge (PR created → merged)
- Release frequency
- Rollback frequency

### Regular Reviews

- Weekly: CI health check
- Monthly: Pipeline performance review
- Quarterly: Full pipeline assessment
- Annually: Strategy review

### Optimization Opportunities

- Parallelize independent jobs
- Optimize test execution
- Improve cache utilization
- Reduce build times
- Automate manual steps

## References

- [E2E Strategy](E2E_STRATEGY.md) - Testing approach
- [CI Pipeline](CI_PIPELINE.md) - CI configuration details
- [Quality Gates](../quality/QUALITY_GATES.md) - Gate definitions
- [Release Process](RELEASE_PROCESS.md) - Release procedures
- [CI Operations](../operations/ci.md) - CI maintenance
