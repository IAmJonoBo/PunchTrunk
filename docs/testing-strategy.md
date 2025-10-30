# Testing Strategy

_This is a How-to and Reference hybrid._

## Current Coverage

### Unit Tests (`main_test.go`)

- `TestHotspotSmoke`: Validates hotspot scoring with deterministic git history
- `TestWriteSARIF`: Ensures SARIF generation produces valid JSON with correct structure
- Coverage: Core hotspot functions and SARIF writing

### E2E Tests (`e2e_test.go`)

- `TestE2EHappyPath`: Complete workflow validation (fmt → lint → hotspots → SARIF)
- `TestE2EChangedFiles`: Validates changed file prioritization in hotspot scoring
- `TestE2EAutofixModes`: Tests all autofix modes (none, fmt, lint, all)
- `TestE2EErrorHandling`: Validates graceful error handling (invalid branches, binary files)
- `TestE2EMultiLanguage`: Tests multi-language support (Go, Python, JavaScript, Markdown)
- `TestE2EKitchenSink`: **Comprehensive validation of all features end-to-end**

### Integration Tests (`.github/workflows/e2e.yml`)

- Full pipeline execution with actual Trunk CLI
- SARIF validation and upload to Code Scanning
- Performance benchmarks (< 2 minutes for hotspots)
- Memory usage validation (< 500MB peak)

## Test Pyramid

```text
         /\
        /  \  E2E & Kitchen Sink (comprehensive, slower)
       /----\
      /      \ Integration (with real Trunk CLI)
     /--------\
    /          \ Unit (fast, deterministic)
   /____________\
```

## Goals

- Validate flag parsing and mode orchestration without spawning external processes when possible.
- Simulate git history to test hotspot scoring deterministically.
- Keep SARIF structure stable; validate against schema in tests.
- Test realistic multi-language scenarios.
- Ensure graceful error handling and degradation.

## Test Execution

### Local Development

```bash
# Run all tests
go test -v ./...

# Run only unit tests
go test -v ./cmd/punchtrunk -run "Test[^E2E]"

# Run only E2E tests
go test -v ./cmd/punchtrunk -run "TestE2E"

# Run kitchen sink test
go test -v ./cmd/punchtrunk -run "TestE2EKitchenSink"
```

### CI Pipeline

- **PR CI** (`.github/workflows/ci.yml`): Unit tests + build + lint
- **E2E CI** (`.github/workflows/e2e.yml`): Full E2E suite + integration + performance
- **Quality Gates**: All tests must pass before merge

## Kitchen Sink Test

The `TestE2EKitchenSink` is our most comprehensive test, validating:

1. **Multi-language repository**: Go, Python, JavaScript, Markdown
2. **Realistic git history**: Multiple commits with varying churn
3. **All PunchTrunk modes**: fmt, lint, hotspots
4. **Hotspot computation**: Churn detection, complexity scoring, ranking
5. **SARIF generation**: Valid 2.1.0 format with correct metadata
6. **File coverage**: All changed files included in analysis
7. **Complexity calculations**: Valid metrics for all files
8. **Error resilience**: Handles missing files gracefully
9. **Multi-phase validation**: 10 distinct validation phases

This test serves as the **final quality gate** before release.

## Recommended Approach

1. Write unit tests for new utility functions in `main.go`.
2. Add E2E tests for new modes or significant feature changes.
3. Use temporary directories and `git init` for tests requiring git history.
4. Validate SARIF structure in tests to catch schema regressions.
5. Run full test suite before submitting PR.

## Manual Verification

- Run `make run` before merging to ensure fmt, lint, and hotspots succeed together.
- Check `reports/hotspots.sarif` with `jq .` to validate JSON structure after code changes.
- Leverage Trunk's hold-the-line defaults: confirm updated files pass via `trunk check`.
- Test with real repositories to validate multi-language support.

## Performance Targets

- Unit tests: < 5 seconds total
- E2E tests: < 2 minutes total
- Kitchen sink test: < 1 minute
- CI pipeline (full): < 10 minutes p95

## Coverage Goals

- Core logic (hotspot computation, SARIF writing): > 80%
- CLI flag parsing and orchestration: > 70%
- Error handling paths: > 60%

## Future Enhancements

- Extract utility functions into `internal/` packages with dedicated tests.
- Add golden SARIF fixtures for regression testing.
- Implement benchmark tests for performance tracking.
- Add coverage reporting to CI with automated badge updates.
- Create sample multi-language repositories for extended testing.
- Document testing playbooks in `docs/internal/runbooks/ephemeral-runners.md`.
