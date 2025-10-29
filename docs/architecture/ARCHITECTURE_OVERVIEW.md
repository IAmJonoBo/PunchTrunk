# Architecture Overview

## Snapshot (C4)

- **Context**: Developers and CI pipelines invoke PunchTrunk to run Trunk formatters/linters and compute hotspots. Neighbouring systems are Git (local or GitHub-hosted) and GitHub Actions, with optional Docker registries when the binary ships in a container.
- **Containers**: `cmd/punchtrunk` Go binary (CLI) running on dev laptops or CI runners; Trunk CLI toolchain downloaded to `~/.cache/trunk`; git command-line tooling; optional distroless Docker image containing the compiled binary.
- **Components**: flag parsing and mode orchestration (`parseFlags`, `runModes`); Trunk integration (`runTrunkFmt`, `runTrunkCheck`); hotspot engine (`computeHotspots`, `gitChurn`, `roughComplexity`); SARIF writer (`writeSARIF`).
- **Trust boundaries**: shell boundary between PunchTrunk and external executables (Trunk, git); filesystem boundary limiting writes to `bin/` and `reports/`; CI boundary where secrets remain owned by GitHub Actions and are not exposed to PunchTrunk.

> C4 reference: <https://c4model.com> (use context → containers → components as needed).

## Key flows

- **Primary user journey**: Developer runs `make run`. PunchTrunk builds (if needed), executes `trunk fmt` and `trunk check` scoped by hold-the-line diff detection, computes hotspots, and writes a SARIF file for inspection.
- **Critical background job**: GitHub Actions workflow `lint-and-hotspots` checks out code with `fetch-depth: 0`, restores Trunk cache, builds the binary, runs PunchTrunk in hotspots-only mode with the PR base branch, and uploads SARIF via `github/codeql-action/upload-sarif@v3`.

## Quality attributes (and how we hit them)

- **Performance**: CI SLO is <10 minutes for fmt+lint+hotspots. Trunk cache keyed on `.trunk/trunk.yaml` prevents redownloads. Hotspot scoring truncates to top 500 files to bound runtime.
- **Availability/resilience**: `context.Context` with timeouts cancels child processes; hotspot logic tolerates missing history by warning and skipping files instead of failing the run; CLI exits non-zero when Trunk reports errors via shared `exitErr`.
- **Security**: Distroless container image reduces attack surface; no network calls beyond Trunk downloads; secrets scanning enabled in Trunk config; SARIF stored locally then uploaded through GitHub’s authenticated action.
- **Observability**: CLI logs summarize each phase; SARIF provides structured findings; CI workflow surfaces Trunk annotations inline; future enhancement backlog includes structured JSON logging for hotspots.

## Interfaces

- **External tools**: Trunk CLI (`trunk fmt`, `trunk check`) pinned in `.trunk/trunk.yaml`; Git CLI (`git diff`, `git log --numstat`) for churn metrics.
- **Outputs**: SARIF 2.1 file at `reports/hotspots.sarif`; stdout/stderr logs consumed by developers and CI; exit codes used by GitHub status checks.
- **Configuration**: command-line flags (`--mode`, `--autofix`, `--base-branch`, `--timeout`, `--sarif-out`); `.trunk/configs/*` override tool behaviour; GitHub workflow parameters set base branch.

## Open decisions

- Active ADRs live under `/docs/architecture/ADRs`. Upcoming candidates include extracting hotspot logic into a testable package and evaluating per-rule SARIF severity tuning.
