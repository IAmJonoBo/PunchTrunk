# Local Development Guide

_This is a How-to guide in the Diátaxis framework._

## Prerequisites

- Go 1.22 or later (`go version` should report ≥1.22; stay current with the latest Go 1.22 patch release to match CI).
- Trunk CLI optional: PunchTrunk auto-installs the pinned CLI into `~/.trunk/bin` if it is missing. On air-gapped runners export `PUNCHTRUNK_AIRGAPPED=1` and point at an existing binary with `--trunk-binary` or `PUNCHTRUNK_TRUNK_BINARY` so the download step is skipped.
- Git with history for the working branch. For shallow clones fetch depth with `git fetch --deepen=1000` if hotspots need longer history.
- Python tooling via `uv`: run `uv venv` once, then `uv pip sync requirements.lock` to install the `sarif` CLI used by `make eval-hotspots`.

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
- Point to an existing Trunk stack with `--trunk-config-dir=/path/to/.trunk` and forward repeatable filters such as `--trunk-arg=--filter=tool:eslint` when local tooling already handles certain languages.

### Hotspot Scan

```bash
make hotspots
```

- Executes hotspots only (`--mode hotspots`).
- Output saves to `reports/hotspots.sarif` (falls back to `/tmp/punchtrunk/reports/<file>` if the repo checkout is read-only). Inspect with `jq . reports/hotspots.sarif`.

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
- If PunchTrunk reports limited git history, fetch more commits (`git fetch --deepen=1000` or clone with `--depth=0`) so churn scoring has meaningful data.
- When you see PunchTrunk warn about overlapping formatter or linter configs (Prettier, Black, ESLint, etc.), either disable the duplicate tool in your Trunk config or scope PunchTrunk with `--trunk-arg` to avoid double execution.

## Updating Tool Versions

- When Go releases a new stable version, update your local install, run `go env GOROOT` to confirm the toolchain, and adjust `go.mod` if language features change.
- Refresh Trunk plugins with `trunk upgrade --yes`, then commit `.trunk/trunk.yaml` and update `docs/trunk-config.md` to reflect the new versions.
- After any upgrade, run the full workflow (`make run`) to validate formatter and linter compatibility.

## Troubleshooting

- Missing Trunk cache: run `trunk clean`, then `trunk upgrade` to repopulate tool versions.
- Hotspots empty: ensure recent commits exist or extend history with `git fetch --deepen`.
- Timeout reached: pass a larger `--timeout` (seconds) or limit modes `--mode fmt,lint`.
- Read-only workspace: PunchTrunk writes hotspot SARIF to `/tmp/punchtrunk/reports`; check the log entry for the exact path and update CI upload steps accordingly.
