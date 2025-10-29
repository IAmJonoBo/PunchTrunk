# PunchTrunk Agent Guide

## Architecture & Responsibilities

- `cmd/punchtrunk/main.go` is the sole binary; it orchestrates `trunk fmt`, `trunk check`, and hotspot scoring while keeping side effects contained.
- CLI state flows through `Config`; flags are defined in `parseFlags`. Whenever the surface changes, align README examples, Makefile targets, CI args, and the install script.
- `runTrunkFmt` / `runTrunkCheck` shell out to Trunk. `cfg.Autofix` only adds `--fix` when requested, and `exitErr` propagates non-zero lint results to CI—do not clear it unless you mean to change exit policy.
- `computeHotspots` combines `git diff --name-only <base>...HEAD` with `git log --numstat --since=90 days`; guard shallow clones and binary files (missing history should degrade gracefully, not fail).
- `writeSARIF` emits SARIF 2.1.0 `note` results at `reports/hotspots.sarif` with rule id `hotspot`. Keep schema/levels stable so GitHub code scanning keeps ingesting uploads.
- Build-time `Version` comes from `-ldflags -X main.Version`; keep runtime logic side-effect free so the distroless image remains predictable.

## Workflows & Toolchain

- `make build` (CGO disabled) produces `bin/punchtrunk`; `make run` executes `fmt,lint,hotspots`; `make hotspots` mirrors the CI hotspot job; `make test` runs all Go tests.
- Typical local run: `./bin/punchtrunk --mode fmt,lint,hotspots --base-branch=origin/main`. For hotspots-only parity use `make hotspots` after a build.
- Trunk configuration lives in `.trunk/trunk.yaml`; extend linters there and mirror overrides under `.trunk/configs/` to stay hermetic.
- CI (`.github/workflows/ci.yml`) fetches full history, caches `~/.cache/trunk`, builds with Go 1.25.x, runs `go test -v ./...`, executes hotspots, then uploads `reports/hotspots.sarif` via `codeql-action`.
- Docker builds via `docker build -t punchtrunk:local .`; the runtime image is `gcr.io/distroless/static:nonroot`, so every write target must already exist or point to `/tmp`.

## Testing & Safety Checks

- `go test ./...` covers unit and E2E cases. `cmd/punchtrunk/e2e_test.go` spins up temporary git repos across multiple languages—Git must be on PATH and tests must tolerate fresh commits.
- Hotspot heuristics rely on `roughComplexity` (token-per-line). Tune weighting by adjusting scoring math instead of rewriting the heuristic to avoid destabilizing rankings.
- Always ensure `reports/hotspots.sarif` is writable and exists even if hotspot computation returns zero results; downstream uploads assume the file is present.
- When altering git invocations (base branch lookup, churn window), preserve compatibility with shallow clones and missing remotes; fall back rather than exiting fatally.

## Integrations & Extensions

- `scripts/install.sh` downloads release artifacts (`punchtrunk-<os>-<arch>`) and verifies optional checksums—keep asset names stable.
- Optional Semgrep rules live under `semgrep/`; enable them by wiring definitions into `.trunk/trunk.yaml` if you expand lint coverage.
- `docs/` hosts deeper decisions (`overview.md`, `hotspots-methodology.md`, `testing-strategy.md`, etc.). Update the relevant doc whenever behavior changes so operators and agents stay aligned.
- Distroless container and release assets are meant to be signed with cosign; see `Dockerfile` and `scripts/install.sh` if you adjust distribution.
