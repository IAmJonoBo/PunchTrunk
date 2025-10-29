# Local Development Guide

_This is a How-to guide in the Diátaxis framework._

## Prerequisites

- Go 1.22 or later (`go version` should report ≥1.22; stay current with the latest Go 1.22 patch release to match CI).
- Trunk CLI installed (`trunk version`). Run `trunk init` the first time or after `trunk upgrade` bumps the pinned toolchain.
- Git with history for the working branch. For shallow clones fetch depth with `git fetch --deepen=1000` if hotspots need longer history.

## Bootstrap

```bash
make build
```

- Creates `bin/punchtrunk` using `CGO_ENABLED=0` for static output.
- Rebuild after flag or dependency changes in `cmd/punchtrunk/main.go`.

## Common Tasks

### Format and Lint

```bash
./bin/punchtrunk --mode fmt,lint --autofix=fmt --base-branch=origin/main
```

- Runs Trunk formatters, then linters without autofix.
- Set `--autofix=all` when you explicitly want Trunk to apply linter fixes.

### Hotspot Scan

```bash
make hotspots
```

- Executes hotspots only (`--mode hotspots`).
- Output saves to `reports/hotspots.sarif`. Inspect with `jq . reports/hotspots.sarif`.

### Full Flow

```bash
make run
```

- Builds if needed and runs fmt, lint, and hotspots.
- Uses defaults from `parseFlags` to keep behavior in sync with CI.

## Verifying Changes

- Run `trunk fmt` and `trunk check` directly if you need faster feedback on targeted files.
- Confirm SARIF diffs before committing to keep `reports/hotspots.sarif` readable.
- Use `git status` to ensure Trunk autofixes are staged, especially on ephemeral dev environments.

## Updating Tool Versions

- When Go releases a new stable version, update your local install, run `go env GOROOT` to confirm the toolchain, and adjust `go.mod` if language features change.
- Refresh Trunk plugins with `trunk upgrade --yes`, then commit `.trunk/trunk.yaml` and update `docs/trunk-config.md` to reflect the new versions.
- After any upgrade, run the full workflow (`make run`) to validate formatter and linter compatibility.

## Troubleshooting

- Missing Trunk cache: run `trunk clean`, then `trunk upgrade` to repopulate tool versions.
- Hotspots empty: ensure recent commits exist or extend history with `git fetch --deepen`.
- Timeout reached: pass a larger `--timeout` (seconds) or limit modes `--mode fmt,lint`.
