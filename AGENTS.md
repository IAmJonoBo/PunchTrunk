# PunchTrunk Agent Handbook

## Repo At A Glance

- Thin Go CLI that wraps `trunk fmt`, `trunk check`, and hotspot scoring so lint flows stay hermetic and fast.
- Single entrypoint at `cmd/punchtrunk/main.go`; all configuration flows through the `Config` struct and `parseFlags`.
- Outputs `reports/hotspots.sarif` (SARIF 2.1.0, rule `hotspot`, level `note`) for GitHub code scanning.
- Core configs live in `.trunk/trunk.yaml` with overrides in `.trunk/configs/`; docs that explain decisions sit under `docs/` (`overview.md`, `hotspots-methodology.md`, `testing-strategy.md`, etc.).

## Architecture & Behavior

- `runTrunkFmt`/`runTrunkCheck` shell out to Trunk; `cfg.Autofix` only adds `--fix` when the user opts in. The global `exitErr` carries non-zero lint exits—leave it wired so CI fails when linting does.
- `computeHotspots` calls `git diff --name-only <base>...HEAD` and `git log --numstat --since=90 days`; missing history or binary files should degrade gracefully (skip rather than crash).
- `roughComplexity` is the token-per-line heuristic; tweak weighting and scoring before rewriting the heuristic to preserve ranking stability.
- `writeSARIF` ensures `reports/` exists and produces file-level notes; keep schema constants intact so downstream uploads keep working.
- Build-time `Version` is injected with `-ldflags -X main.Version`; runtime logic should remain side-effect free to keep the distroless image lean.

## Developer Workflows

- `make build` (CGO disabled) → `bin/punchtrunk`; `make run` executes `fmt,lint,hotspots`; `make hotspots` mirrors CI; `make test` runs all Go tests.
- Local loop: `./bin/punchtrunk --mode fmt,lint,hotspots --base-branch=origin/main`; hotspots-only parity via `make hotspots`.
- CI (`.github/workflows/ci.yml`) checks out with `fetch-depth: 0`, caches `~/.cache/trunk`, builds with Go 1.25.x, runs `go test -v ./...`, executes hotspots, and uploads SARIF with `codeql-action`.
- Trunk CLI must be available; install via `trunk init` locally or rely on CI bootstrap. Update `.trunk/trunk.yaml` and `.trunk/configs/` together when toggling linters.

## Testing & Safety Nets

- `go test ./...` exercises unit tests (`main_test.go`) plus comprehensive E2E scenarios (`e2e_test.go`) that create temporary git repos across multiple languages—Git must be on PATH.
- Ensure `reports/hotspots.sarif` is writable; even empty results should produce a file so CI uploads succeed.
- Hotspot ranking truncates to top 500 entries; keep that cap unless dashboards change expectations.
- When adjusting git commands or churn windows, retain compatibility with shallow clones and nonexistent remotes.

## Distribution & Extensions

- Docker image builds with `docker build -t punchtrunk:local .` and runs as `nonroot` in `gcr.io/distroless/static:nonroot`; direct any runtime writes to `/tmp` or pre-created paths.
- `scripts/install.sh` downloads release artifacts (`punchtrunk-<os>-<arch>`), optionally verifies checksums, and sets up `/usr/local/bin`; keep filenames stable for installers.
- Optional Semgrep rules live in `semgrep/`; wire them into `.trunk/trunk.yaml` if you expand lint coverage.
- Release signing is expected via cosign; update install docs and Dockerfile comments if distribution changes.
