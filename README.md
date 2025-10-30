# PunchTrunk (polyglot, ephemeral-friendly)

A lightweight CLI + CI setup that:

- Runs **Trunk** across your repo for linting and formatting
- Applies **safe autofixes** (formatters by default; linters optional)
- Surfaces **hotspots** (git churn × complexity) to guide attention
- Emits a **SARIF** file for hotspots (uploadable to GitHub code scanning)
- Integrates with **Trunk Action** for inline PR annotations
- Works out-of-the-box on **ephemeral runners** with caching
- Provides **air-gap diagnostics** via `--mode diagnose-airgap`
- Surfaces **toolchain drift** with `--mode tool-health` (version + cache verification)

> Status: starter kit. Designed to be hermetic, fast, and agent-friendly.
> Rebranding note: formerly `trunk-orchestrator`, now published as **PunchTrunk** to reflect the broader workflow focus.
> Ownership: **n00tropic** maintains the software; PunchTrunk is the name of the orchestrator binary.

---

## Installation

### Quick Install (Recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/IAmJonoBo/PunchTrunk/main/scripts/install.sh | bash
```

This installs the latest release to `/usr/local/bin/punchtrunk`.

### Manual Download

Download pre-built artifacts from [GitHub Releases](https://github.com/IAmJonoBo/PunchTrunk/releases):

**Offline Bundles (PunchTrunk + Trunk CLI + config):**

Each release ships cross-platform archives named `punchtrunk-offline-<os>-<arch>.tar.gz`. They include:

- `bin/punchtrunk`
- `trunk/bin/trunk` (matching the repository's pinned Trunk version)
- `.trunk/` configuration and optional cached toolchains
- Manifest and checksum files

Extract on the target host and source the generated env file (see [Offline / air-gapped environments](#offline--air-gapped-environments)).

**Standalone Binaries:**

- Linux (AMD64): `punchtrunk-linux-amd64`
- macOS (ARM64 - M1/M2): `punchtrunk-darwin-arm64`
- Windows (AMD64): `punchtrunk-windows-amd64.exe`

```bash
curl -L https://github.com/IAmJonoBo/PunchTrunk/releases/latest/download/punchtrunk-<os>-<arch> -o punchtrunk
chmod +x punchtrunk
sudo mv punchtrunk /usr/local/bin/
```

Windows users can download the `.exe` directly and add it to `PATH`.

### From Source

```bash
git clone https://github.com/IAmJonoBo/PunchTrunk.git
cd PunchTrunk
make build
sudo mv bin/punchtrunk /usr/local/bin/
```

---

## Quick start

1. **Install PunchTrunk** (see [Installation](#installation) above)

2. **Verify Trunk CLI availability**:
   - Offline bundles ship with a pinned Trunk binary and helper env files (`punchtrunk-airgap.env` / `.ps1`).
   - On developer laptops you can rely on PunchTrunk's auto-installer or follow the [Trunk installation guide](https://docs.trunk.io/code-quality/setup-and-installation/initialize-trunk) if you prefer manual control.
3. **Initialise** Trunk in your repo (first time only):

   ```bash
   trunk init
   ```

4. **Run PunchTrunk**:

   ```bash
   punchtrunk --mode fmt,lint,hotspots --base-branch=origin/main
   ```

5. **CI Integration**:
   - See [Integration Guide](docs/INTEGRATION_GUIDE.md) for GitHub Actions, GitLab CI, CircleCI, and more
   - Check [example workflows](.github/workflows/)

---

## What you get

- **Hold-the-line** by default (changed files only), configurable base branch in `.trunk/trunk.yaml`.
- **Autofix**: by default only formatters are applied; linter autofix can be enabled with `--autofix=lint`.
- **Hotspots**: file-level ranking computed from recent git churn and simple complexity (token count); exported at `reports/hotspots.sarif`.
- **CI**: offline bundles, GitHub Actions workflow, cache examples for ephemeral runners, optional Reviewdog step for inline comments.
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
   --mode=fmt,lint,hotspots   Which phases to run (default: fmt,lint,hotspots). Include
                              `diagnose-airgap` to emit readiness checks or `tool-health`
                              to inspect Trunk versions and cache hydration without
                              executing fmt/lint.
   --autofix=none|fmt|lint|all  Which fixes to apply (default: fmt)
   --base-branch=<git ref>    Base for change detection (default: origin/main)
   --max-procs=<n>            Parallelism cap (default: logical CPUs)
   --timeout=<seconds>        Overall wall-clock budget (default: 900)
   --sarif-out=reports/hotspots.sarif  Where to write hotspot SARIF (falls back to /tmp/punchtrunk/reports when workspace is read-only)
   --verbose                  Extra logs
   --json-logs                Emit structured JSON logs (also honours PUNCHTRUNK_JSON_LOGS)
   --tmp-dir=<path>           Override temporary directory for SARIF fallbacks and installer staging
   --trunk-config-dir=<dir>   Use an alternate Trunk config directory when reusing an existing setup
   --trunk-binary=<path>      Explicit trunk binary to run (air-gapped/offline runners)
   --trunk-arg=<value>        Additional argument forwarded to `trunk` (repeatable)
```

Set `PUNCHTRUNK_JSON_LOGS=true` to enable JSON output without editing invocation scripts.
Set `PUNCHTRUNK_TMP_DIR=/path/to/tmp` to choose a writable fallback location for SARIF and installer files.

### Examples

```bash
# Fast pre-commit run on changed files
./bin/punchtrunk --mode fmt,lint

# Weekly deep clean (full scan)
./bin/punchtrunk --mode fmt,lint,hotspots --timeout=3600

# Strict CI (no autofix)
./bin/punchtrunk --mode lint,hotspots --autofix=none --base-branch=origin/main

# Reuse an existing Trunk config and scope to specific tools
./bin/punchtrunk --mode fmt,lint --trunk-config-dir=/path/to/.trunk --trunk-arg=--filter=tool:eslint

# Air-gapped runner (skip installer, point at cached binary)
PUNCHTRUNK_AIRGAPPED=1 ./bin/punchtrunk --mode lint --trunk-binary=/opt/trunk/bin/trunk

# Emit structured logs for ingestion
./bin/punchtrunk --mode fmt,lint --json-logs

# Verify bundle hydration and version alignment
./bin/punchtrunk --mode tool-health

# Redirect tmp usage off the default /tmp mount
./bin/punchtrunk --mode hotspots --tmp-dir=/var/punchtrunk/tmp

```

## Offline / air-gapped environments

- PunchTrunk auto-installs Trunk when it cannot find the CLI on `PATH`. Set `PUNCHTRUNK_AIRGAPPED=1` to disable the download step on runners without outbound network access.
- Supply the executable explicitly with `--trunk-binary=/path/to/trunk` or `PUNCHTRUNK_TRUNK_BINARY=/path/to/trunk`. The path is validated for existence and executability before any Trunk command is executed.
- Cached installs created by PunchTrunk live under `~/.trunk/bin`; reuse that path for future jobs if you pre-bake the toolchain.
- When the workspace is read-only, hotspot SARIF output automatically falls back to `/tmp/punchtrunk/reports/<file>` and a log line explains the redirect.
- Build an offline bootstrap bundle with `make offline-bundle` (or `./scripts/build-offline-bundle.sh` for custom paths). The script now runs `trunk install --ci` before packaging (failing back to warnings when downloads are blocked) and records the CLI version, config checksum, hydration status, and source cache path in `manifest.json`. Pass `--skip-hydrate` to opt out when you intentionally want an empty cache.
- Use `scripts/setup-airgap.sh` (Linux/macOS) or `scripts/setup-airgap.ps1` (Windows) to unpack the bundle, create stable symlinks/wrappers, wire caches, and emit an env file you can source in provisioning jobs.
- `scripts/build-offline-bundle.sh` accepts `--target-os` and `--target-arch` so you can build archives for any supported platform from a single host, auto-downloading the matching Trunk CLI when not provided.
- Bundle installs persist under the directory you pass to `setup-airgap.*`; add `$(install-dir)/bin` to `PATH` or source the generated env helper so callers resolve the embedded PunchTrunk and Trunk binaries without extra exports.
- `scripts/run-quality-suite.sh` executes the full preflight (`prep-runner.sh`) and then runs `punchtrunk --mode fmt,lint,hotspots` against your chosen base branch, emitting Markdown + JSON reports that agents can ingest even when the network is sealed.
- See `docs/INTEGRATION_GUIDE.md` for a step-by-step walkthrough on verifying the bundle, running the setup scripts, and wiring environment variables before running PunchTrunk in sealed networks.

### Diagnose offline readiness

Run the diagnostic mode to confirm an environment is ready for sealed networks:

```bash
punchtrunk --mode diagnose-airgap --sarif-out=/workspace/reports/hotspots.sarif
```

- Produces a JSON report on stdout with check summaries (git availability, trunk binary, air-gap env vars, SARIF destination writability)
- Returns a non-zero exit when blocking errors remain so agents can gate provisioning workflows
- Skips Trunk installs and other side effects, making it safe to run before network access is revoked

### Inspect toolchain health

`tool-health` inspects the resolved Trunk CLI, parses `.trunk/trunk.yaml`, and verifies that a usable cache exists for each pinned plugin, runtime, and linter. Use `--tool-health-format summary` when you want human-friendly tables and `--tool-health-json <path>` to capture machine-readable output:

```bash
punchtrunk --mode tool-health --tool-health-format summary --tool-health-json reports/tool-health.json
```

- Emits a JSON report (version alignment, cache directory, manifest metadata)
- Warns (and exits non-zero) when the installed Trunk version differs from `cli.version` or when pinned tools/runtimes are missing from the cache directory
- Works well with offline bundles to confirm hydration _before_ provisioning new runners

---

## CI (GitHub Actions)

- Inline annotations via **trunk-io/trunk-action**.
- `scripts/prep-runner.sh` hydrates Trunk caches, runs `tool-health`, and emits a summary artifact so sealed runners retain context even when downloads are blocked.
- `scripts/run-quality-suite.sh` wraps the preflight plus a full PunchTrunk run; the integration leg of `e2e.yml` now calls it directly so local agents and CI ingest the same summaries and JSON artifacts.
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

### Coexisting with existing toolchains

- Projects that already ship Trunk configs can pass `--trunk-config-dir=/path/to/.trunk` so PunchTrunk reuses their pinned toolchain instead of the bundled defaults.
- Use repeatable `--trunk-arg` flags to forward options like `--filter=tool:eslint` or `--config-dir=...` directly to `trunk`.
- PunchTrunk inspects common formatter and linter configs (Prettier, Black, ESLint, Rubocop, etc.) and prints guidance when overlap is detected so you can disable duplicate runners or filter scopes.

---

## Hotspot method (lightweight)

- **Churn**: number of lines added/modified over a sliding 90-day window (customisable).
- **Complexity**: rough token/line ratio as a proxy.
- **Score**: `log(1 + churn) * (1 + complexity_z)`; we rank descending.
- **Output**: SARIF `note` results with a file-level message for dashboards.

This is a heuristic to prioritise attention, inspired by defect prediction literature and “hotspots” practice. It’s intentionally conservative; it does not label code “bad”, it highlights **recently-touched, potentially-risky** areas for review.

---

## Security & supply chain

- Offline bundles ship with per-file SHA-256 checksums and a manifest so you can verify integrity before installation.
- `setup-airgap.sh` / `.ps1` validate optional checksum files and create wrapper scripts that pin `PUNCHTRUNK_TRUNK_BINARY` for reproducible runs.
- Releases are built in CI with pinned Go and Trunk versions; keep `.trunk/trunk.yaml` committed so updates remain explicit.

References:

- [Cosign](https://github.com/sigstore/cosign)
- [GitHub SARIF upload](https://docs.github.com/en/code-security/code-scanning/integrating-with-code-scanning/uploading-a-sarif-file-to-github)

---

## Testing

PunchTrunk includes comprehensive test coverage:

- **Unit tests**: Core logic validation (`main_test.go`)
- **E2E tests**: Complete workflow scenarios (`e2e_test.go`)
- **Kitchen Sink test**: Comprehensive validation of all features end-to-end

Run tests locally:

```bash
# All tests
go test -v ./...

# Unit tests only
go test -v ./cmd/punchtrunk -run "^Test[^E2E]"

# E2E tests only
go test -v ./cmd/punchtrunk -run "TestE2E"

# Kitchen sink test (comprehensive validation)
go test -v ./cmd/punchtrunk -run "TestE2EKitchenSink"

# Provisioning scripts
bats scripts/tests
```

See [Testing Strategy](docs/testing-strategy.md) for details.

---

## Documentation

- Browse the [PunchTrunk documentation index](docs/README.md) for tutorials, how-to guides, explanations, and reference material organised with Diátaxis.

---

## Quality Gates

PunchTrunk enforces quality gates at each pipeline stage:

- **Pre-commit**: Format, lint, unit tests, build
- **PR**: All tests pass, E2E validation, security scans, SARIF validation
- **Release**: Multi-platform builds, offline bundle integrity, performance validation

See [Quality Gates](docs/internal/quality/QUALITY_GATES.md) for the full internal checklist.

---

## Deployment

PunchTrunk follows a comprehensive deployment pipeline:

1. **Local Dev** → Format, lint, test, build
2. **PR CI** → Full validation with E2E tests
3. **Main CI** → Integration and performance checks
4. **Release** → Multi-platform builds, offline bundle publishing, integrity verification
5. **Post-Release** → Monitoring and validation

See [Deployment Pipeline](docs/internal/delivery/DEPLOYMENT_PIPELINE.md) and [E2E Strategy](docs/internal/delivery/E2E_STRATEGY.md) for details.

---

## Extending

- Add Semgrep with autofix rules under `semgrep/` and wire it in `.trunk/trunk.yaml` (optional).
- Integrate Reviewdog for extra PR comments (especially in non-GitHub or where you want diff-only noise).
- Extend E2E tests for new features or edge cases.

---

## Troubleshooting

- **No issues appearing?** Trunk uses hold-the-line; run `trunk check --all` locally or push a change.
- **Slow cold starts in CI?** Ensure caches are restoring; check cache key inputs (lockfiles, `.trunk` state).
- **Autofix surprises?** Set `--autofix=none` in CI and rely on inline annotations.
- **Tests failing?** Run `go test -v ./...` locally to diagnose. Check git is configured properly for E2E tests.
- **SARIF validation errors?** Validate with `jq empty reports/hotspots.sarif` to ensure valid JSON.
- **Workspace read-only?** PunchTrunk will redirect hotspot output to `/tmp/punchtrunk/reports` automatically; check the log entry for the new path if uploads cannot find the file.

---

## License

MIT for the CLI code and scripts. Trunk and other tools are their own licenses.
