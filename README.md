# PunchTrunk (polyglot, ephemeral-friendly)

A lightweight CLI + CI setup that:

- Runs **Trunk** across your repo for linting and formatting
- Applies **safe autofixes** (formatters by default; linters optional)
- Surfaces **hotspots** (git churn × complexity) to guide attention
- Emits a **SARIF** file for hotspots (uploadable to GitHub code scanning)
- Integrates with **Trunk Action** for inline PR annotations
- Works out-of-the-box on **ephemeral runners** with caching

> Status: starter kit. Designed to be hermetic, fast, and agent-friendly.
> Rebranding note: formerly `trunk-orchestrator`, now published as **PunchTrunk** to reflect the broader workflow focus.
> Ownership: **n00tropic** maintains the software; PunchTrunk is the name of the orchestrator binary.

---

## Quick start

1. **Install Trunk CLI** locally (or let CI do it):
   - [Installation guide](https://docs.trunk.io/code-quality/setup-and-installation/initialize-trunk)
2. **Initialise** Trunk in your repo (first time only):

   ```bash
   trunk init
   ```

3. **Run the orchestrator** (local dev):

   ```bash
   go run ./cmd/punchtrunk --mode fmt,lint --autofix=fmt --base-branch=origin/main
   ```

   Or after building:

   ```bash
   ./bin/punchtrunk --mode fmt,lint --autofix=fmt --base-branch=origin/main
   ```

4. **CI on GitHub Actions**:
   - Copy `.github/workflows/ci.yml` to your repo.
   - On PRs, you’ll get inline annotations from Trunk Action.
   - Hotspots SARIF uploads to **Code Scanning** (Security tab).

---

## What you get

- **Hold-the-line** by default (changed files only), configurable base branch in `.trunk/trunk.yaml`.
- **Autofix**: by default only formatters are applied; linter autofix can be enabled with `--autofix=lint`.
- **Hotspots**: file-level ranking computed from recent git churn and simple complexity (token count); exported at `reports/hotspots.sarif`.
- **CI**: distroless container build, GitHub Actions workflow, cache examples for ephemeral runners, optional Reviewdog step for inline comments.
- **Polyglot**: Trunk drives the right tools per language; you can add linters via `.trunk/trunk.yaml`.

---

## Requirements

- Go 1.22+ to build the CLI
- Trunk CLI available in PATH on dev machines; CI job installs & caches it
- Git available (the hotspot analysis shells out to git)

---

## CLI usage

```text
PunchTrunk [flags]

Flags:
  --mode=fmt,lint,hotspots   Which phases to run (default: fmt,lint,hotspots)
  --autofix=none|fmt|lint|all  Which fixes to apply (default: fmt)
  --base-branch=<git ref>    Base for change detection (default: origin/main)
  --max-procs=<n>            Parallelism cap (default: logical CPUs)
  --timeout=<seconds>        Overall wall-clock budget (default: 900)
  --sarif-out=reports/hotspots.sarif  Where to write hotspot SARIF
  --verbose                  Extra logs
```

### Examples

```bash
# Fast pre-commit run on changed files
./bin/punchtrunk --mode fmt,lint

# Weekly deep clean (full scan)
./bin/punchtrunk --mode fmt,lint,hotspots --timeout=3600

# Strict CI (no autofix)
./bin/punchtrunk --mode lint,hotspots --autofix=none --base-branch=origin/main
```

---

## CI (GitHub Actions)

- Inline annotations via **trunk-io/trunk-action**.
- Optional **SARIF upload** for hotspots (`reports/hotspots.sarif`).
- Ephemeral-friendly caches: Trunk tool cache + Go build cache.

See `.github/workflows/ci.yml`.

---

## Configuring Trunk

The orchestrator honours `.trunk/trunk.yaml`. This repo includes a minimal seed which:

- Pins the Trunk CLI version
- Sets `trunk_branch` to `main` (change for your repo)
- Enables common formatters/linters (you can extend this)

Docs:

- [Hold-the-line & base branch](https://docs.trunk.io/code-quality/setup-and-installation/prevent-new-issues)
- [`trunk check` / `trunk fmt`](https://docs.trunk.io/code-quality/linters/run-linters)

---

## Hotspot method (lightweight)

- **Churn**: number of lines added/modified over a sliding 90-day window (customisable).
- **Complexity**: rough token/line ratio as a proxy.
- **Score**: `log(1 + churn) * (1 + complexity_z)`; we rank descending.
- **Output**: SARIF `note` results with a file-level message for dashboards.

This is a heuristic to prioritise attention, inspired by defect prediction literature and “hotspots” practice. It’s intentionally conservative; it does not label code “bad”, it highlights **recently-touched, potentially-risky** areas for review.

---

## Security & supply chain

- **Distroless** container for the CLI runtime (no shell, minimal surface).
- Optional signing step with **cosign** (keyless OIDC supported) when publishing your image.
- Pin Trunk CLI version in `.trunk/trunk.yaml` for reproducibility.

References:

- [Distroless images](https://github.com/GoogleContainerTools/distroless)
- [Docker doc on distroless](https://docs.docker.com/dhi/core-concepts/distroless/)
- [Cosign](https://github.com/sigstore/cosign)
- [GitHub SARIF upload](https://docs.github.com/en/code-security/code-scanning/integrating-with-code-scanning/uploading-a-sarif-file-to-github)

---

## Extending

- Add Semgrep with autofix rules under `semgrep/` and wire it in `.trunk/trunk.yaml` (optional).
- Integrate Reviewdog for extra PR comments (especially in non-GitHub or where you want diff-only noise).

---

## Troubleshooting

- **No issues appearing?** Trunk uses hold-the-line; run `trunk check --all` locally or push a change.
- **Slow cold starts in CI?** Ensure caches are restoring; check cache key inputs (lockfiles, `.trunk` state).
- **Autofix surprises?** Set `--autofix=none` in CI and rely on inline annotations.

---

## License

MIT for the CLI code and scripts. Trunk and other tools are their own licenses.
