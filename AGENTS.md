# PunchTrunk Agent Handbook

## Repo At A Glance

- Purpose: thin Go CLI that wraps `trunk fmt`, `trunk check`, and hotspot scoring so linters stay fast and hermetic.
- Ownership: maintained by **n00tropic**; PunchTrunk refers to the CLI binary.
- Primary entrypoint: `cmd/punchtrunk/main.go` (single binary, no subpackages).
- Outputs: writes hotspot SARIF to `reports/hotspots.sarif` for GitHub Code Scanning.
- Core configs: `.trunk/trunk.yaml` plus overrides in `.trunk/configs/`; Make targets in `Makefile` mirror common flows.
- Rebranded from `trunk-orchestrator`; older scripts may still reference the former name.

## Required Tooling

- Go 1.22+ for local builds; CI pins 1.22.x (see `.github/workflows/ci.yml`).
- Trunk CLI (install via `trunk init` or let CI download). Holds formatter/linter versions via `.trunk/trunk.yaml`.
- Git available for hotspot churn analysis (`git diff`, `git log --numstat`). Works on shallow clones but guard missing history.
- Optional: Docker (builds `punchtrunk:local`), `cosign` for signing.

## Build & Run Pipeline

- `make build` → `bin/punchtrunk`; equivalent raw command: `CGO_ENABLED=0 go build -o bin/punchtrunk ./cmd/punchtrunk`.
- `make run` executes all modes locally (`fmt,lint,hotspots`) using defaults from `parseFlags`.
- `make hotspots` matches CI hotspot run. CI invokes `./bin/punchtrunk --mode hotspots --base-branch=origin/<base>`.
- Docker image: `docker build -t punchtrunk:local .` → distroless runtime at `/app/punchtrunk`, runs as `nonroot`.

## Flag Surface (from `parseFlags`)

- `--mode=fmt,lint,hotspots` controls phase selection.
- `--autofix=fmt` is default; `--autofix=none` for strict lint, `--autofix=all` passes `--fix` to Trunk.
- `--base-branch=origin/main` scopes hold-the-line diff detection.
- `--sarif-out=reports/hotspots.sarif` path must remain writable in distroless image.
- `--timeout` (seconds) and `--max-procs` tune orchestration concurrency.

## Hotspot Implementation Notes

- `computeHotspots` combines churn (`git log --numstat`) with `roughComplexity` token/line ratios.
- Handles binaries by treating `-` counts as single churn; skip unreadable or missing files instead of failing.
- Scores are ranked and truncated to top 500 before SARIF emission.
- SARIF writer (`writeSARIF`) produces rule id `hotspot`, level `note`; downstream workflows expect this schema (2.1.0).

## CI & Ephemeral Runner Guidance

- GitHub Action `lint-and-hotspots` restores Trunk cache (`~/.cache/trunk`) keyed off `.trunk/trunk.yaml`.
- Always checkout with `fetch-depth: 0` so churn history exists; on ephemeral runners missing history, guard git calls and degrade gracefully.
- Hotspot SARIF uploaded via `github/codeql-action/upload-sarif@v3`; ensure file exists even on failure (empty SARIF acceptable).
- When adding new modes, update CI arguments, README examples, and `Makefile` so automation and docs stay aligned.

## Extending Safely

- Modify flag handling inside `parseFlags`; keep defaults in sync with README & this guide.
- Add new Trunk linters/formatters in `.trunk/trunk.yaml` and mirror overrides under `.trunk/configs/` to stay self-contained.
- For new external tool invocations, use `exec.CommandContext` with the orchestrator-wide context so cancellations propagate.
- Keep Dockerfile distroless-friendly: any runtime writes must target `/tmp` or be completed in the build stage.

## Quick Checklist For Handovers

- Ensure `bin/punchtrunk` rebuild with `make build` before committing CLI changes.
- Run `./bin/punchtrunk --mode fmt,lint` locally so Trunk fixes format before pushing.
- Confirm `reports/hotspots.sarif` is regenerated and valid JSON (use `jq . reports/hotspots.sarif`).
- Update `README.md`, `.github/copilot-instructions.md`, and this file when modes, flags, or workflows shift.
- Review supporting docs under `docs/` (overview, workflows, CI ops, security) to keep guidance current with any dependency upgrades.
