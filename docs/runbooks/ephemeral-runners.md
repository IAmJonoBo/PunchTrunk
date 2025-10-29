# Ephemeral Runner Runbook

_This is a How-to runbook._

## Objective

Restore productivity on short-lived CI or cloud workspaces where caches and git history may be absent.

## Quick Checks

1. **Git Depth**
   - Run `git rev-parse HEAD` to confirm repository integrity.
   - If history is shallow, execute `git fetch --deepen=1000` or `git fetch --unshallow`.
2. **Trunk Cache**
   - Verify `~/.cache/trunk` exists. If missing, run `trunk clean` then `trunk check --all` to warm linters.
3. **Go Toolchain**
   - Confirm `go version` aligns with CI expectations (`1.22.x`).

## Running the Orchestrator

```bash
./bin/punchtrunk --mode fmt,lint,hotspots --base-branch=origin/main --timeout=900
```

- Build the binary first with `make build` when caches are cold.
- Increase `--timeout` for large repos or slow network storage.
- Adjust `--base-branch` for release branches or forks (for example, `origin/release/v1`). Document temporary overrides in the pull request to aid reviewers.

## Handling Failures

- **Missing SARIF**: rerun hotspots mode; ensure `reports/` directory exists.
- **gitChurn errors**: fallback by skipping hotspots when `git log` is unavailable (`--mode fmt,lint`).
- **Trunk download issues**: set `TRUNK_DOWNLOAD_MIRROR` if a corporate mirror is required; document in `docs/CONVENTIONS.md`.

## Resetting the Environment

- Delete temporary artifacts with `rm -rf bin reports`.
- Run `trunk clean` to remove stale tool versions.
- Rebuild and rerun orchestrator to confirm a clean slate.
- After the incident, capture lessons learned in `docs/CONVENTIONS.md` or open an ADR if processes need structural changes.

## Escalation

- Capture logs from `trunk check` and orchestrator output.
- Attach `reports/hotspots.sarif` (even if empty) to issue templates for faster triage.
- Update `docs/operations/ci.md` if a systemic CI change is needed.
