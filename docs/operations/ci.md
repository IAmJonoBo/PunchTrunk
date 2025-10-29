# CI Operations

_This is a How-to guide paired with a Reference summary._

## Workflow Summary

- Job: `lint-and-hotspots` in `.github/workflows/ci.yml`.
- Runner: `ubuntu-latest`, timeout 30 minutes.
- Fetch depth: `0` to ensure git history for hotspots.

## Step-by-Step

1. **Checkout**
   - Uses `actions/checkout@v4` with `fetch-depth: 0`.
   - Required so `git log --numstat` has churn data.
2. **Cache Trunk**
   - Stores `~/.cache/trunk` keyed on `.trunk/trunk.yaml` hash.
   - Speeds up formatter and linter startup on ephemeral runners.
3. **Trunk Check (annotations)**
   - `trunk-io/trunk-action@v1` with `arguments: check` handles inline annotations.
   - Respects `.trunk/trunk.yaml` and `.trunk/configs/*` overrides.
4. **Setup Go**
   - `actions/setup-go@v5` pins Go `1.22.x` for forward compatibility.
5. **Build Binary**
   - `go build -o bin/trunk-orchestrator ./cmd/trunk-orchestrator`.
   - Mirrors `make build` to catch compilation errors before runtime.
6. **Run Hotspots**
   - Executes `./bin/trunk-orchestrator --mode hotspots --base-branch=origin/<base>`.
   - For pull requests, `<base>` resolves to `github.event.pull_request.base.ref`.
7. **Upload SARIF**
   - `github/codeql-action/upload-sarif@v3` pushes `reports/hotspots.sarif` to Code Scanning.

## Adapting the Workflow

- **Disable hotspots**: change the orchestrator command to remove `hotspots` or set `--mode fmt,lint`.
- **Add extra modes**: update `--mode` and ensure README, Makefile, and docs stay in sync.
- **Alternate base branch**: override `--base-branch` when running on release branches.
- **Custom caches**: add additional `actions/cache` steps for Go build cache (`~/.cache/go-build`) if builds slow down.

## Failure Investigation

- Check Trunk Action logs first; it surfaces lint failures with inline annotations.
- If hotspots fail due to missing history, confirm `fetch-depth: 0` or widen the git fetch.
- SARIF upload errors often indicate invalid JSON; run `jq` locally or re-run hotspots to regenerate.
