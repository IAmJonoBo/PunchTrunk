# Project Conventions

_This is a Reference document._

## Code Style

- Go source uses `gofmt`. Do not commit manual formatting tweaks.
- Keep orchestration logic in `cmd/punchtrunk/main.go`; add helper packages only when testing demands it.
- Prefer `exec.CommandContext` with shared context to respect timeouts.

## Commit Messages

- Use present tense summary lines (e.g., `Add SARIF severity guard`).
- Mention related docs updates when changing flags or workflows.
- Reference issue numbers with `Fixes #123` when applicable.

## Documentation Standards

- Follow Diátaxis: classify new docs as Tutorial, How-to, Explanation, or Reference.
- Use Google developer style: short sentences, active voice, descriptive headings.
- Keep markdownlint happy: surround headings, lists, and fences with blank lines.

## Tooling Expectations

- Run `make build` before committing orchestrator changes.
- Run `trunk fmt` and `trunk check` locally; CI treats warnings as build failures.
- Regenerate `reports/hotspots.sarif` when hotspot logic or dependencies change.

## Version Tracking

- Document Go or Trunk upgrades in the pull request description and update `docs/trunk-config.md` and `docs/internal/workflows/local-dev.md` accordingly.
- When Docker base images change, note the new digest in `docs/security-supply-chain.md` and release notes.
- Keep ADRs synchronized with implementation choices so future contributors understand why versions changed.

## Environment Variables

- `TRUNK_DOWNLOAD_MIRROR`: set when corporate mirrors host Trunk artifacts.
- `GIT_PAGER`: set to `cat` in CI if git commands need non-interactive output.
- Document new environment variables in this section and in relevant runbooks.
