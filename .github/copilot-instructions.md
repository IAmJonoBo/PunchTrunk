# PunchTrunk Agent Guide

## Architecture & Responsibilities

- `cmd/punchtrunk/main.go` is the sole binary; it orchestrates `trunk fmt`, `trunk check`, and hotspot scoring while keeping side effects contained.
- CLI state flows through `Config`; flags are defined in `parseFlags`. Whenever the surface changes, align README examples, Makefile targets, CI args, and the install script.
- `runTrunkFmt` / `runTrunkCheck` shell out to Trunk. `cfg.Autofix` only adds `--fix` when requested, and `exitErr` propagates non-zero lint results to CI—do not clear it unless you mean to change exit policy. Repeated `--trunk-arg` flags are forwarded verbatim, and `--trunk-config-dir` overrides discovery so PunchTrunk can coexist with repos that already ship Trunk configs.
- `computeHotspots` combines `git diff --name-only <base>...HEAD` with `git log --numstat --since=90 days`; guard shallow clones and binary files (missing history should degrade gracefully, not fail).
- `writeSARIF` emits SARIF 2.1.0 `note` results at `reports/hotspots.sarif` with rule id `hotspot`. Keep schema/levels stable so GitHub code scanning keeps ingesting uploads.
- Build-time `Version` comes from `-ldflags -X main.Version`; keep runtime logic side-effect free so release bundles remain predictable.
- `ensureEnvironment` resolves prerequisites before any mode runs: it checks for Git and auto-installs Trunk into `~/.trunk/bin` (or the Windows equivalent) when missing, normalises `--trunk-config-dir` (auto-discovering `.trunk/trunk.yaml` when unset), validates any explicit `--trunk-binary`/`PUNCHTRUNK_TRUNK_BINARY` path (or fails fast when `PUNCHTRUNK_AIRGAPPED` forbids downloads), loads bundle manifests when present, saves the resolved config/cache directories (exporting `TRUNK_CACHE_DIR`), and records the binary path in `Config.TrunkPath` for all subprocesses. `maybeWarnCompetingTools` inspects well-known formatter/linter configs and nudges users to scope PunchTrunk when overlap is detected.
- `tool-health` emits a JSON report comparing the detected Trunk CLI version to `.trunk/trunk.yaml` and verifying cached plugins/runtimes/linters; it returns non-zero on mismatch or missing cache entries so automation can gate deployments.

## Workflows & Toolchain

- `make build` (CGO disabled) produces `bin/punchtrunk`; `make run` executes `fmt,lint,hotspots`; `make hotspots` mirrors the CI hotspot job; `make test` runs all Go tests.
- Typical local run: `./bin/punchtrunk --mode fmt,lint,hotspots --base-branch=origin/main`. Pin to an existing Trunk setup with `--trunk-config-dir=/path/to/.trunk` and forward filters (e.g. `--trunk-arg=--filter=tool:eslint`) when another formatter/linter already covers the same files. For hotspots-only parity use `make hotspots` after a build.
- Trunk configuration lives in `.trunk/trunk.yaml`; extend linters there and mirror overrides under `.trunk/configs/` to stay hermetic.
- CI (`.github/workflows/ci.yml`) fetches full history, caches `~/.cache/trunk`, builds with Go 1.25.x, runs `go test -v ./...`, executes hotspots, then uploads `reports/hotspots.sarif` via `codeql-action`.
- Offline bundles come from `scripts/build-offline-bundle.sh`; the script hydrates caches via `trunk fmt --fetch` / `trunk check --fetch`, captures manifest metadata (CLI version, trunk config checksum, hydration status), and supports `--skip-hydrate` when you intentionally package an empty cache. `scripts/setup-airgap.*` installs bundles and emits reusable env helpers so runners can source `punchtrunk-airgap.env`/`.ps1`.
- Agents running on fresh machines need no manual Trunk setup—`ensureEnvironment` will download the installer script from `https://get.trunk.io`, execute it non-interactively, and reuse the cached binary on subsequent runs.

## Testing & Safety Checks

- `go test ./...` covers unit and E2E cases. `cmd/punchtrunk/e2e_test.go` spins up temporary git repos across multiple languages—Git must be on PATH and tests must tolerate fresh commits.
- Hotspot heuristics rely on `roughComplexity` (token-per-line). Tune weighting by adjusting scoring math instead of rewriting the heuristic to avoid destabilizing rankings.
- Always ensure `reports/hotspots.sarif` is writable and exists even if hotspot computation returns zero results; when the workspace is read-only PunchTrunk automatically writes to `/tmp/punchtrunk/reports/<filename>` and logs the redirect so uploads can locate the file.
- When altering git invocations (base branch lookup, churn window), preserve compatibility with shallow clones and missing remotes; fall back rather than exiting fatally.

## Integrations & Extensions

- `scripts/install.sh` downloads release artifacts (`punchtrunk-<os>-<arch>`) and verifies optional checksums—keep asset names stable.
- Optional Semgrep rules live under `semgrep/`; enable them by wiring definitions into `.trunk/trunk.yaml` if you expand lint coverage.
- `docs/` hosts deeper decisions (`overview.md`, `hotspots-methodology.md`, `testing-strategy.md`, etc.). Update the relevant doc whenever behavior changes so operators and agents stay aligned.
- Offline bundles and release assets are meant to be signed with cosign; see `scripts/build-offline-bundle.sh` and `scripts/install.sh` if you adjust distribution.
