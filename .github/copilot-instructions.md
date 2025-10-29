# PunchTrunk Agent Guide

## Architecture

- CLI entrypoint `cmd/punchtrunk/main.go` orchestrates `trunk fmt`, `trunk check`, and hotspot scoring; keep logic self-contained and side-effect free.
- `computeHotspots` shells out to git (`git diff`, `git log --numstat`) and parses repo files for complexity; changes must preserve compatibility with shallow clones by guarding errors.
- SARIF emission happens via `writeSARIF`; results land in `reports/hotspots.sarif` and encode file-level `note` severities.
- Global `exitErr` carries the first `trunk check` failure so CI exits non-zero even if other modes succeed; do not reset it unless intentionally modifying exit policy.
- Shell utilities live in `scripts/` (currently `hotspots.sh` placeholder) and should mirror CLI behavior when expanded.

## Workflows

- Local build: `go build -o bin/punchtrunk ./cmd/punchtrunk` (or `make build`); Go 1.22+ per `go.mod`, CI stays on the latest 1.22 patch.
- Common runs: `./bin/punchtrunk --mode fmt,lint,hotspots --base-branch=origin/main`; for hotspots-only CI parity use `make hotspots`.
- Trunk toolchain is configured in `.trunk/trunk.yaml`; `trunk fmt` hits formatters only, `trunk check` drives linters with hold-the-line defaults.
- CI workflow `.github/workflows/ci.yml` checks out with `fetch-depth: 0`, caches Trunk, builds the binary, runs hotspots against the PR base, then uploads SARIF.
- Docker image builds via `docker build -t punchtrunk:local .`, producing a distroless runtime binary at `/app/punchtrunk`.

## Conventions

- Flag definitions live in `parseFlags`; whenever a flag changes, update README examples and ensure defaults align with orchestrator behavior.
- `computeHotspots` assumes text files; guard binary or missing files by skipping them rather than failing the run.
- `roughComplexity` uses token-per-line ratio; adjust scoring constants instead of rewriting heuristics when tuning results.
- Keep JSON/SARIF struct tags stable; downstream tooling expects SARIF 2.1 with rule id `hotspot` and level `note`.
- Trunk linter overrides live under `.trunk/configs/` (Markdownlint, ShellCheck, Yamllint); extend these files in-place to stay hermetic.

## Integration Points

- Semgrep rules in `semgrep/print-debug.yml` are optional; wire them into `.trunk/trunk.yaml` if enabling Semgrep autofix.
- Hotspot SARIF is meant for GitHub Code Scanning upload via the workflow; validate changes with `codeql-action/upload-sarif` expectations (a readable file at `reports/hotspots.sarif`).
- When expanding modes, ensure new behavior is reflected in README, Makefile targets, and CI `--mode` invocations to keep humans and automation aligned.
- Distroless runtime runs as `nonroot`; any new filesystem writes must target writable paths (e.g., `/tmp`) or occur before copying into the final image.
- Prefer `exec.CommandContext` with the shared timeout context when invoking external tools so the orchestrator cancels cleanly on shutdown.
