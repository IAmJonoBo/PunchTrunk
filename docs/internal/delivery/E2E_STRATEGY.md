# End-to-End Development and Deployment Strategy

_This document defines the comprehensive E2E development and deployment strategy for PunchTrunk, including quality gates, testing approach, and deployment pipeline._

## Overview

PunchTrunk's E2E strategy ensures reliable, repeatable deployments with minimal risk through automated testing, quality gates, and progressive rollout patterns.

## Quality Gates

### Pre-Commit Gates

- Local linting via `trunk fmt` and `trunk check`
- Unit tests pass (`go test ./...`)
- Binary builds successfully (`make build`)
- Developer self-review completed

### PR Gates (Required for Merge)

1. **Code Quality**
   - All Trunk checks pass (hold-the-line)
   - No new linter violations
   - Code review approved by maintainer
2. **Testing**
   - Unit tests pass (100% of existing tests)
   - E2E tests pass (all scenarios)
   - Kitchen sink test validates full pipeline
3. **Security**
   - CodeQL scanning passes
   - No high/critical security vulnerabilities
   - SARIF validation successful
   - Dependency vulnerabilities checked
4. **Documentation**
   - README updated if CLI flags changed
   - CHANGELOG entry added
   - Related docs updated

### Release Gates

1. **Build Validation**
   - Binary builds on all target platforms
   - Offline bundle builds successfully
   - Bundle checksums generated and verified
2. **E2E Validation**
   - Full E2E suite passes on release branch
   - Performance benchmarks within SLO (10 min p95)
   - Integration tests with sample repositories pass
3. **Deployment Readiness**
   - Release notes prepared
   - Rollback plan documented
   - Monitoring configured
   - On-call team notified

## E2E Testing Strategy

### Test Levels

#### 1. Unit Tests (`*_test.go`)

- Fast, deterministic, isolated
- Mock external dependencies (git, trunk)
- Cover individual functions and edge cases
- Target: >80% code coverage for core logic

#### 2. Integration Tests

- Test interactions between components
- Use real git repositories in temp directories
- Validate SARIF generation and structure
- Verify Trunk CLI integration

#### 3. E2E Tests (`e2e_test.go`)

- Test complete workflows end-to-end
- Simulate real-world usage scenarios
- Validate all modes: fmt, lint, hotspots
- Include failure scenarios and recovery

#### 4. Kitchen Sink Test

- Ultimate validation test
- Exercises all features simultaneously
- Validates entire pipeline: fmt → lint → hotspots → SARIF
- Tests with diverse code samples (multiple languages, complexity levels)
- Verifies quality gates enforcement

### E2E Test Scenarios

1. **Happy Path Flow**
   - Clean repository → format → lint → compute hotspots → generate SARIF
   - Validates: successful execution, correct output, expected exit codes

2. **Changed Files Flow**
   - Repository with changes → detect changed files → analyze hotspots → prioritize changed files
   - Validates: change detection, hotspot scoring boost, SARIF accuracy

3. **Autofix Scenarios**
   - Test all autofix modes: none, fmt, lint, all
   - Validates: proper flag handling, Trunk integration, file modifications

4. **Error Handling**
   - Missing git history (shallow clone)
   - Binary files in repository
   - Invalid base branch
   - Trunk check failures
   - Validates: graceful degradation, error messages, exit codes

5. **Performance Validation**
   - Large repository with extensive history
   - Validates: completes within timeout, memory usage reasonable

6. **Multi-Language Support**
   - Repository with Go, Python, JavaScript, Markdown
   - Validates: Trunk handles all languages, hotspots computed correctly

## Deployment Pipeline

### Stage 1: Local Development

```
Developer workstation
├── make fmt          # Format code
├── make lint         # Run linters
├── go test ./...     # Unit tests
└── make run          # Full local run
```

### Stage 2: Pull Request CI

```
GitHub Actions (ubuntu-latest)
├── Checkout (fetch-depth: 0)
├── Cache Trunk tools
├── Trunk Check (inline annotations)
├── Setup Go 1.22.x
├── Build binary
├── Run unit tests
├── Run E2E tests
├── Run hotspots
├── Upload SARIF
└── Validate quality gates
```

### Stage 3: Pre-Release Validation

```
Release branch CI
├── All PR gates pass
├── Build multi-platform binaries
│   ├── linux-amd64
│   ├── linux-arm64
│   ├── darwin-amd64
│   └── darwin-arm64
├── Build offline bundle
├── Verify bundle checksums
├── Full E2E suite on sample repos
├── Performance benchmarks
└── Generate SBOM
```

### Stage 4: Release

```text
GitHub Release
├── Create release tag (vX.Y.Z)
├── Generate release notes
├── Upload binaries and offline bundles
├── Sign artifacts (cosign)
└── Update documentation
```

### Stage 5: Post-Release Validation

```text
Monitoring and validation
├── Monitor GitHub Code Scanning uploads
├── Track CI success rates
├── Measure performance metrics
├── Collect user feedback
└── Monitor error rates
```

## Deployment Patterns

### Feature Flags

- Use `--mode` flag to enable/disable features
- Default to safe modes (fmt,lint) for conservative rollout
- Progressive enablement of hotspots mode

### Rollout Strategy

1. **Alpha**: Internal n00tropic repos (1-2 repos, 1 week)
2. **Beta**: Selected partner repos (5-10 repos, 2 weeks)
3. **GA**: Public release with documentation

### Rollback Plan

- Revert workflow file to previous version
- Pin previous binary version in CI
- Document rollback in incident log
- Root cause analysis and fix

## Quality Metrics

### Success Criteria

- CI pipeline completes in <10 minutes (p95)
- Test success rate >99%
- Zero high/critical security vulnerabilities
- SARIF uploads 100% successful
- No regressions in hotspot accuracy

### Monitoring

- CI job duration trends
- Test failure patterns
- SARIF validation errors
- Hotspot score distributions
- User-reported issues

## Compliance and Security

### Security Requirements

- Offline bundle reproducibility with pinned Trunk CLI
- Run as non-root user where applicable (CI, ephemeral runners)
- No secrets in logs or SARIF
- Regular dependency updates
- Vulnerability scanning in CI

### Audit Trail

- All CI runs logged
- Git commits signed
- Release artifacts signed with cosign
- SARIF files retained per security policy

## Maintenance and Evolution

### Regular Tasks

- Quarterly: Review and update dependencies
- Monthly: Review CI metrics and optimize
- Weekly: Monitor test stability
- Daily: Review PR quality gate failures

### Continuous Improvement

- Add new test scenarios based on bugs found
- Optimize E2E test runtime
- Expand multi-language coverage
- Improve error messages and diagnostics

## References

- [Testing Strategy](../testing-strategy.md) - Detailed test approach
- [CI Pipeline](CI_PIPELINE.md) - CI implementation details
- [Release Process](RELEASE_PROCESS.md) - Release procedures
- [CI Operations](../operations/ci.md) - CI maintenance guide
