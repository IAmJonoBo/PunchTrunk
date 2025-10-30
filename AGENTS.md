# PunchTrunk Agent Handbook

## Repo At A Glance

- Thin Go CLI that wraps `trunk fmt`, `trunk check`, and hotspot scoring so lint flows stay hermetic and fast.
- Single entrypoint at `cmd/punchtrunk/main.go`; all configuration flows through the `Config` struct and `parseFlags`.
- Outputs `reports/hotspots.sarif` (SARIF 2.1.0, rule `hotspot`, level `note`) for GitHub code scanning.
- Core configs live in `.trunk/trunk.yaml` with overrides in `.trunk/configs/`; docs that explain decisions sit under `docs/` (`overview.md`, `hotspots-methodology.md`, `testing-strategy.md`, etc.).

## Architecture & Behavior

- `runTrunkFmt`/`runTrunkCheck` shell out to Trunk; `cfg.Autofix` only adds `--fix` when the user opts in. The global `exitErr` carries non-zero lint exits—leave it wired so CI fails when linting does. `cfg.TrunkArgs` forwards each `--trunk-arg` flag to Trunk, `cfg.TrunkConfigDir` lets PunchTrunk target alternate Trunk config roots, and `cfg.TrunkBinary`/`PUNCHTRUNK_TRUNK_BINARY` allow pre-installed binaries to be reused (required when `PUNCHTRUNK_AIRGAPPED=1`).
- `computeHotspots` calls `git diff --name-only <base>...HEAD` and `git log --numstat --since=90 days`; missing history or binary files should degrade gracefully (skip rather than crash).
- `roughComplexity` is the token-per-line heuristic; tweak weighting and scoring before rewriting the heuristic to preserve ranking stability.
- `writeSARIF` ensures `reports/` exists and produces file-level notes; keep schema constants intact so downstream uploads keep working.
- Build-time `Version` is injected with `-ldflags -X main.Version`; runtime logic should remain side-effect free to keep release bundles predictable.
- `ensureEnvironment` now handles bootstrap duties: it verifies `git` exists, auto-installs Trunk (via the official installer script) into `~/.trunk/bin` when missing, normalises any `--trunk-config-dir`, validates explicit binaries when they are provided, respects `PUNCHTRUNK_AIRGAPPED` by skipping downloads, auto-discovers `.trunk/trunk.yaml` when no override is supplied, captures bundle metadata, assigns `TRUNK_CACHE_DIR`, and stores the resolved binary path on `Config.TrunkPath` so subprocesses always hit the right executable. `maybeWarnCompetingTools` scans for common formatter/linter configs (Prettier, Black, ESLint, etc.) and logs guidance so agents can scope PunchTrunk when overlap exists.
- `tool-health` is a read-only mode that inspects the detected Trunk CLI version plus cache hydration for pinned plugins, runtimes, and linters. It emits JSON, returns non-zero on mismatches, and is ideal for verifying offline bundles before production rollout.

## Developer Workflows

- `make build` (CGO disabled) → `bin/punchtrunk`; `make run` executes `fmt,lint,hotspots`; `make hotspots` mirrors CI; `make test` runs all Go tests.
- Local loop: `./bin/punchtrunk --mode fmt,lint,hotspots --base-branch=origin/main`; pass `--trunk-config-dir` when reusing an existing Trunk stack and add repeatable `--trunk-arg` flags (e.g. `--trunk-arg=--filter=tool:eslint`) to avoid running conflicting toolchains. Hotspots-only parity via `make hotspots`.
- CI (`.github/workflows/ci.yml`) checks out with `fetch-depth: 0`, caches `~/.cache/trunk`, builds with Go 1.25.x, runs `go test -v ./...`, executes hotspots, and uploads SARIF with `codeql-action`.
- Trunk CLI must be available; install via `trunk init` locally or rely on CI bootstrap. Update `.trunk/trunk.yaml` and `.trunk/configs/` together when toggling linters.
- Agents on new runners can rely on PunchTrunk itself to install Trunk automatically—no manual pre-flight commands required. Subsequent runs reuse the cached installation under the user’s home directory.

## Testing & Safety Nets

- `go test ./...` exercises unit tests (`main_test.go`) plus comprehensive E2E scenarios (`e2e_test.go`) that create temporary git repos across multiple languages—Git must be on PATH.
- Ensure `reports/hotspots.sarif` is writable; even empty results should produce a file so CI uploads succeed. On read-only workspaces PunchTrunk falls back to `/tmp/punchtrunk/reports/<filename>` and emits a log line—surface that path to upload steps when necessary.
- Hotspot ranking truncates to top 500 entries; keep that cap unless dashboards change expectations.
- When adjusting git commands or churn windows, retain compatibility with shallow clones and nonexistent remotes.

## Distribution & Extensions

- `scripts/build-offline-bundle.sh` produces archives (`punchtrunk-offline-<os>-<arch>.tar.gz`) that embed PunchTrunk, a pinned Trunk CLI, `.trunk` configs, cached toolchains, and helper env scripts; it now hydrates caches via `trunk install --ci` by default, captures the CLI version + config checksum in `manifest.json`, and supports `--skip-hydrate` when you need to package an empty cache. Use `scripts/setup-airgap.*` to install bundles on runners.
- `scripts/install.sh` downloads release artifacts (`punchtrunk-<os>-<arch>`), optionally verifies checksums, and sets up `/usr/local/bin`; keep filenames stable for installers and bundle manifests.
- Optional Semgrep rules live in `semgrep/`; wire them into `.trunk/trunk.yaml` if you expand lint coverage.
- Release signing is expected via cosign; update install docs and bundle manifests if distribution changes.
