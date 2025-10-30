# Agent Provisioning Guide

This guide ensures that all agents (GitHub Actions runners, CI systems, local development environments) have the necessary tools to run PunchTrunk effectively.

## Overview

PunchTrunk requires several tools to function properly:
- **Git**: For repository operations and hotspot analysis
- **Trunk CLI**: For linting and formatting orchestration
- **Go**: For building PunchTrunk from source (optional if using pre-built binaries)
- **Python**: For Semgrep security scanning (optional)

## Provisioning Strategies

### Strategy 1: Automatic Installation (Recommended for Development)

PunchTrunk includes automatic installation of the Trunk CLI:

```bash
# PunchTrunk will auto-install Trunk CLI when not found
./bin/punchtrunk --mode fmt,lint,hotspots
```

The Trunk CLI is installed to `~/.trunk/bin/` and reused across runs.

### Strategy 2: Explicit Pre-Installation (Recommended for CI)

For ephemeral runners, explicitly install tools before running PunchTrunk:

```yaml
# GitHub Actions example
- name: Install Trunk CLI
  run: |
    curl https://get.trunk.io -fsSL | bash -s -- -y
    echo "${HOME}/.trunk/bin" >> $GITHUB_PATH
  env:
    TRUNK_INIT_NO_ANALYTICS: "1"
    TRUNK_TELEMETRY_OPTOUT: "1"

- name: Cache Trunk tools
  uses: actions/cache@v4
  with:
    path: ~/.cache/trunk
    key: trunk-${{ runner.os }}-${{ hashFiles('.trunk/trunk.yaml') }}
    restore-keys: |
      trunk-${{ runner.os }}-

- name: Hydrate Trunk caches
  run: trunk install --ci
```

### Strategy 3: Offline/Air-Gapped Environments

For environments without network access:

1. **Build an offline bundle** (on a machine with network access):
   ```bash
   make offline-bundle
   # Produces: dist/punchtrunk-offline-<os>-<arch>.tar.gz
   ```

2. **Transfer and setup on the target machine**:
   ```bash
   # Extract bundle
   tar -xzf punchtrunk-offline-linux-amd64.tar.gz
   
   # Run setup script
   cd punchtrunk-offline-linux-amd64
   ./scripts/setup-airgap.sh /opt/punchtrunk
   
   # Source environment
   source /opt/punchtrunk/punchtrunk-airgap.env
   ```

3. **Run PunchTrunk in air-gapped mode**:
   ```bash
   export PUNCHTRUNK_AIRGAPPED=1
   punchtrunk --mode lint,hotspots --trunk-binary=/opt/punchtrunk/bin/trunk
   ```

## Tool Verification

Use the built-in diagnostic modes to verify provisioning:

### Diagnose Air-Gap Readiness

```bash
punchtrunk --mode diagnose-airgap
```

Output:
```json
{
  "timestamp": "2025-10-30T21:00:00Z",
  "airgapped": false,
  "checks": [
    {
      "name": "git",
      "status": "ok",
      "message": "git found at /usr/bin/git"
    },
    {
      "name": "trunk_binary",
      "status": "ok",
      "message": "trunk binary found at /home/user/.trunk/bin/trunk"
    }
  ],
  "summary": {
    "total": 4,
    "ok": 4,
    "warn": 0,
    "error": 0
  }
}
```

### Check Tool Health

```bash
punchtrunk --mode tool-health --tool-health-format summary
```

This validates:
- Trunk CLI version matches `.trunk/trunk.yaml`
- All pinned plugins/runtimes/linters are cached
- Cache directory is accessible

## GitHub Actions Integration

Complete example for ephemeral runners:

```yaml
name: CI with PunchTrunk

on:
  pull_request:
  push:
    branches: [main]

jobs:
  lint-and-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
        with:
          fetch-depth: 0  # Full history for hotspots

      # Provision: Cache Trunk tools
      - name: Cache Trunk tools
        uses: actions/cache@v4
        with:
          path: ~/.cache/trunk
          key: trunk-${{ runner.os }}-${{ hashFiles('.trunk/trunk.yaml') }}
          restore-keys: |
            trunk-${{ runner.os }}-

      # Provision: Install Trunk CLI
      - name: Install Trunk CLI
        run: |
          curl https://get.trunk.io -fsSL | bash -s -- -y
          echo "${HOME}/.trunk/bin" >> $GITHUB_PATH
        env:
          TRUNK_INIT_NO_ANALYTICS: "1"
          TRUNK_TELEMETRY_OPTOUT: "1"

      # Provision: Verify installation
      - name: Verify tools
        run: |
          git --version
          trunk --version
          echo "✓ All tools provisioned"

      # Provision: Setup Go
      - uses: actions/setup-go@v6
        with:
          go-version: 1.25.x

      # Provision: Build PunchTrunk
      - name: Build PunchTrunk
        run: make build

      # Provision: Hydrate caches
      - name: Hydrate runner
        run: |
          bash scripts/prep-runner.sh \
            --config-dir=.trunk \
            --cache-dir="$HOME/.cache/trunk" \
            --punchtrunk=./bin/punchtrunk

      # Execute: Run PunchTrunk
      - name: Run PunchTrunk
        run: |
          ./bin/punchtrunk --mode fmt,lint,hotspots \
            --base-branch=origin/${{ github.base_ref || 'main' }}

      # Upload: Results
      - name: Upload SARIF
        uses: github/codeql-action/upload-sarif@v4
        with:
          sarif_file: reports/hotspots.sarif
```

## Provisioning Scripts

PunchTrunk includes helper scripts:

### prep-runner.sh

Prepares a CI runner by hydrating caches and validating tools:

```bash
bash scripts/prep-runner.sh \
  --config-dir=.trunk \
  --cache-dir="$HOME/.cache/trunk" \
  --punchtrunk=./bin/punchtrunk \
  --json-output=reports/preflight.json
```

Options:
- `--config-dir`: Trunk configuration directory
- `--cache-dir`: Trunk cache directory
- `--punchtrunk`: Path to PunchTrunk binary
- `--trunk-binary`: Explicit trunk executable path
- `--json-output`: Where to write preflight report
- `--skip-network-check`: Skip network probe
- `--quiet`: Reduce logging

### build-offline-bundle.sh

Creates offline bundles for air-gapped environments:

```bash
./scripts/build-offline-bundle.sh \
  --output-dir dist \
  --punchtrunk-binary bin/punchtrunk \
  --target-os linux \
  --target-arch amd64
```

Options:
- `--output-dir`: Directory for generated bundle
- `--bundle-name`: Custom archive name
- `--punchtrunk-binary`: PunchTrunk binary to include
- `--trunk-binary`: Trunk CLI to include
- `--cache-dir`: Trunk cache to bundle
- `--config-dir`: Trunk config to bundle
- `--no-cache`: Skip bundling cache
- `--skip-hydrate`: Skip cache prefetch
- `--force`: Overwrite existing bundle

### setup-airgap.sh / setup-airgap.ps1

Installs offline bundles:

```bash
# Linux/macOS
./scripts/setup-airgap.sh /opt/punchtrunk

# Windows
.\scripts\setup-airgap.ps1 -InstallDir C:\PunchTrunk
```

## Firewall and Network Considerations

### Required Network Access (Non-Air-Gapped)

If using automatic installation, allow access to:
- `https://get.trunk.io` - Trunk CLI installer
- `https://trunk.io` - Trunk plugin registry
- `https://api.github.com` - For GitHub integrations (optional)

### Network-Restricted Environments

Use offline bundles (Strategy 3) and set:
```bash
export PUNCHTRUNK_AIRGAPPED=1
```

This prevents PunchTrunk from attempting network downloads.

## Linters and Formatters

PunchTrunk provisions these tools automatically via Trunk:

### Enabled by Default (per `.trunk/trunk.yaml`)

**Linters:**
- actionlint (GitHub Actions)
- checkov (Infrastructure as Code)
- git-diff-check
- golangci-lint (Go)
- hadolint (Docker)
- markdownlint (Markdown)
- osv-scanner (Dependency vulnerabilities)
- shellcheck (Shell scripts)
- trufflehog (Secrets scanning)
- yamllint (YAML)
- renovate (Dependency updates)

**Formatters:**
- gofmt (Go)
- prettier (JS/TS/JSON/Markdown/YAML)
- shfmt (Shell scripts)

**Runtimes (auto-provisioned):**
- Go 1.22.3
- Node.js 22.16.0
- Python 3.10.8

### Optional Tools

**Semgrep** (security scanning):
```bash
pip install semgrep
make security
```

Custom rules: `semgrep/offline-ci.yml`

## Validation Checklist

Before running PunchTrunk in production, verify:

- [ ] Git is installed and accessible: `git --version`
- [ ] Trunk CLI is installed: `trunk --version`
- [ ] PunchTrunk binary is built: `./bin/punchtrunk --version`
- [ ] Trunk config exists: `.trunk/trunk.yaml`
- [ ] Cache directory is writable: `~/.cache/trunk`
- [ ] Diagnostics pass: `punchtrunk --mode diagnose-airgap`
- [ ] Tool health passes: `punchtrunk --mode tool-health`

## Troubleshooting

### "trunk executable not found"

**Solution 1**: Let PunchTrunk auto-install:
```bash
./bin/punchtrunk --mode fmt,lint  # Will auto-install trunk
```

**Solution 2**: Install manually:
```bash
curl https://get.trunk.io -fsSL | bash -s -- -y
export PATH="${HOME}/.trunk/bin:${PATH}"
```

**Solution 3**: Use offline bundle:
```bash
export PUNCHTRUNK_AIRGAPPED=1
punchtrunk --trunk-binary=/path/to/trunk
```

### "PUNCHTRUNK_AIRGAPPED is set"

This error occurs when:
- `PUNCHTRUNK_AIRGAPPED=1` is set
- No trunk binary is provided via `--trunk-binary` or `PUNCHTRUNK_TRUNK_BINARY`

**Solution**:
```bash
# Option 1: Unset air-gap mode
unset PUNCHTRUNK_AIRGAPPED

# Option 2: Provide explicit trunk path
export PUNCHTRUNK_TRUNK_BINARY=/path/to/trunk
```

### Cache hydration failures

If `trunk install --ci` fails:

1. Check network connectivity
2. Verify `.trunk/trunk.yaml` is valid
3. Clear cache and retry:
   ```bash
   rm -rf ~/.cache/trunk
   trunk install --ci
   ```

### Missing linters/formatters

If specific tools aren't working:

1. Verify they're enabled in `.trunk/trunk.yaml`
2. Run `trunk install --ci` to hydrate caches
3. Check tool-health: `punchtrunk --mode tool-health`

## Best Practices

1. **CI/CD**: Always explicitly install Trunk CLI in ephemeral runners
2. **Caching**: Use GitHub Actions cache for `~/.cache/trunk`
3. **Verification**: Run `prep-runner.sh` before PunchTrunk execution
4. **Air-Gap**: Pre-build offline bundles for sealed networks
5. **Diagnostics**: Use `--mode diagnose-airgap` and `--mode tool-health` in preflight checks
6. **Documentation**: Keep `.trunk/trunk.yaml` version-controlled and documented

## Agent-Friendly Usage

For GitHub Copilot agents and automation:

```bash
# Quick validation
punchtrunk --mode diagnose-airgap

# Full quality suite
bash scripts/run-quality-suite.sh \
  --base-branch origin/main \
  --modes fmt,lint,hotspots \
  --punchtrunk ./bin/punchtrunk

# Individual phases
punchtrunk --mode fmt              # Format only
punchtrunk --mode lint             # Lint only
punchtrunk --mode hotspots         # Hotspots only
punchtrunk --mode tool-health      # Validate setup
```

### Makefile Targets

PunchTrunk includes agent-friendly Makefile targets:

```bash
make help          # Display all available targets
make validate-env  # Validate environment has all required tools
make prep-runner   # Hydrate caches and run health checks
make build         # Build PunchTrunk binary
make test          # Run all tests
make run           # Build and run PunchTrunk
make security      # Run Semgrep security scan
```

Common workflows:
```bash
# First time setup
make validate-env  # Check environment
make build         # Build binary
make prep-runner   # Hydrate caches

# Development cycle
make build test    # Build and test
make run           # Run PunchTrunk

# CI/CD
make prep-runner run  # Prepare and execute
```

## Further Reading

- [Integration Guide](INTEGRATION_GUIDE.md) - CI/CD setup examples
- [Testing Strategy](testing-strategy.md) - Running tests
- [Trunk Configuration](trunk-config.md) - Customizing linters
- [Offline Bundles](security-supply-chain.md) - Air-gapped deployments
