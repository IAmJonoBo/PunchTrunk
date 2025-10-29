# Testing Strategy

_This is a How-to and Reference hybrid._

## Current Coverage

- There are no Go unit tests yet; logic lives in `cmd/punchtrunk/main.go`.
- Hotspot functions (`computeHotspots`, `gitChurn`, `roughComplexity`) are candidates for extracted packages if tests are added.

## Goals

- Validate flag parsing and mode orchestration without spawning external processes when possible.
- Simulate git history to test hotspot scoring deterministically.
- Keep SARIF structure stable; consider snapshot tests on generated JSON.

## Recommended Approach

1. Extract utility functions into an internal package when they gain tests.
2. Use temporary directories and `git init` for integration-style tests.
3. Stub external commands (`exec.CommandContext`) via helper functions if deterministic output is required.
4. Add `go test ./...` to CI once tests exist; document the change here and in `.github/workflows/ci.yml`.

## Manual Verification

- Run `make run` before merging to ensure fmt, lint, and hotspots succeed together.
- Check `reports/hotspots.sarif` with `jq .` to validate JSON structure after code changes.
- Leverage Trunk's hold-the-line defaults: confirm updated files pass via `trunk check`.

## Future Enhancements

- Provide sample repositories for smoke-testing against different languages.
- Add golden SARIF fixtures to guard against schema regressions.
- Document testing playbooks in `docs/runbooks/ephemeral-runners.md` once automated coverage improves.
- Introduce automated coverage reporting in CI when tests land, and record the process in `docs/operations/ci.md`.
