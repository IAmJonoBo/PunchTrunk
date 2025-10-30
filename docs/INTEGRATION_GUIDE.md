# PunchTrunk Integration Guide

## Overview

This guide covers integrating PunchTrunk into CI/CD pipelines, ephemeral runners, and automated workflows. PunchTrunk is designed to be ephemeral-friendly with minimal dependencies and fast cold starts.

> **📖 For comprehensive provisioning strategies, see the [Agent Provisioning Guide](AGENT_PROVISIONING.md)** which covers tool installation, validation, and troubleshooting in detail.

## Table of Contents

- [Quick Start](#quick-start)
- [GitHub Actions](#github-actions)
- [GitLab CI](#gitlab-ci)
- [CircleCI](#circleci)
- [Jenkins](#jenkins)
- [Ephemeral Runners](#ephemeral-runners)
- [Offline & Air-Gapped Environments](#offline--air-gapped-environments)
- [Agent Integration](#agent-integration)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

## Quick Start

### Installation

**Offline bundle (recommended for CI and ephemeral runners):**

```bash
curl -L https://github.com/IAmJonoBo/PunchTrunk/releases/latest/download/punchtrunk-offline-<os>-<arch>.tar.gz \
  -o punchtrunk-offline.tgz
./scripts/setup-airgap.sh --bundle punchtrunk-offline.tgz --install-dir /opt/punchtrunk --force
source /opt/punchtrunk/punchtrunk-airgap.env
```

The bundle contains PunchTrunk, a pinned Trunk CLI, `.trunk/` configs, optional cached toolchains, and ready-to-use environment helpers (`punchtrunk-airgap.env` / `.ps1`). Sourcing the helper exports `PUNCHTRUNK_TRUNK_BINARY`, toggles airgapped mode, and prepends the bundled binaries to `PATH`.

**Install script (developer laptops):**

```bash
curl -fsSL https://raw.githubusercontent.com/IAmJonoBo/PunchTrunk/main/scripts/install.sh | bash
```

This installs the latest release into `/usr/local/bin`. PunchTrunk will auto-install Trunk on demand when you have sudo rights. For restricted environments, supply `--trunk-binary` or source an offline bundle instead.

### Basic Usage

```bash
# Initialize Trunk (first time only)
trunk init

# Run all modes
punchtrunk --mode fmt,lint,hotspots

# Format only
punchtrunk --mode fmt

# Lint with autofix
punchtrunk --mode lint --autofix=lint

# Hotspots only
punchtrunk --mode hotspots --base-branch=origin/main
```

## GitHub Actions

### Recommended: Explicit Tool Provisioning (2025)

This approach explicitly installs and validates all required tools, ensuring ephemeral runners are fully provisioned. This is the strategy used in PunchTrunk's own CI.

```yaml
name: Quality Checks

on:
  pull_request:
  push:
    branches: [main]

jobs:
  punchtrunk:
    runs-on: ubuntu-latest
    timeout-minutes: 30
    permissions:
      contents: read
      security-events: write # Required for SARIF uploads

    steps:
      - name: Checkout code
        uses: actions/checkout@v5
        with:
          fetch-depth: 0 # Required for hotspot analysis

      # Step 1: Cache Trunk tools
      - name: Cache Trunk tools
        uses: actions/cache@v4
        with:
          path: ~/.cache/trunk
          key: trunk-${{ runner.os }}-${{ hashFiles('.trunk/trunk.yaml') }}
          restore-keys: |
            trunk-${{ runner.os }}-

      # Step 2: Explicitly install Trunk CLI
      - name: Install Trunk CLI
        run: |
          curl https://get.trunk.io -fsSL | bash -s -- -y
          echo "${HOME}/.trunk/bin" >> $GITHUB_PATH
        env:
          TRUNK_INIT_NO_ANALYTICS: "1"
          TRUNK_TELEMETRY_OPTOUT: "1"

      # Step 3: Verify installation
      - name: Verify Trunk CLI
        run: |
          trunk --version
          echo "✓ Trunk CLI is available"

      # Step 4: Setup Go
      - name: Setup Go
        uses: actions/setup-go@v6
        with:
          go-version: 1.25.x

      # Step 5: Build PunchTrunk
      - name: Build PunchTrunk
        run: make build

      # Step 6: Validate tool provisioning
      - name: Validate tool provisioning
        run: |
          echo "=== Validating Tool Provisioning ==="
          git --version
          trunk --version
          go version
          ./bin/punchtrunk --version
          echo "✅ All required tools are provisioned"

      # Step 7: Hydrate caches
      - name: Prepare runner environment
        run: make prep-runner
        continue-on-error: true

      # Step 8: Run PunchTrunk
      - name: Run PunchTrunk
        run: |
          ./bin/punchtrunk --mode fmt,lint,hotspots \
            --base-branch=origin/${{ github.event_name == 'pull_request' && github.event.pull_request.base.ref || 'main' }}

      # Step 9: Upload SARIF
      - name: Upload SARIF
        uses: github/codeql-action/upload-sarif@v4
        with:
          sarif_file: reports/hotspots.sarif

      # Step 10: Upload reports (optional)
      - name: Upload reports
        if: always()
        uses: actions/upload-artifact@v5
        with:
          name: punchtrunk-reports
          path: reports/
          retention-days: 7
```

**Key benefits:**
- ✅ Explicit tool installation ensures no missing dependencies
- ✅ Validation step catches provisioning issues early
- ✅ Trunk CLI cached across runs for speed
- ✅ Works reliably on ephemeral runners
- ✅ Easy to debug and understand

### Alternative: Complete integration (offline bundle)

```yaml
name: Quality Checks

on:
  pull_request:
  push:
    branches: [main]

jobs:
  punchtrunk:
    runs-on: ubuntu-latest
    timeout-minutes: 20
    permissions:
      contents: read
      security-events: write # Required for SARIF uploads

    steps:
      - name: Checkout code
        uses: actions/checkout@v4
        with:
          fetch-depth: 0 # Required for hotspot analysis

      - name: Restore Trunk cache
        uses: actions/cache@v4
        with:
          path: ~/.cache/trunk
          key: trunk-${{ runner.os }}-${{ hashFiles('.trunk/trunk.yaml') }}
          restore-keys: |
            trunk-${{ runner.os }}-

      - name: Install PunchTrunk bundle
        run: |
          curl -L https://github.com/IAmJonoBo/PunchTrunk/releases/latest/download/punchtrunk-offline-linux-amd64.tar.gz \
            -o $RUNNER_TEMP/punchtrunk-offline.tgz
          mkdir -p $RUNNER_TEMP/punchtrunk
          tar -xzf $RUNNER_TEMP/punchtrunk-offline.tgz -C $RUNNER_TEMP/punchtrunk
          BUNDLE_DIR=$(find $RUNNER_TEMP/punchtrunk -maxdepth 1 -type d -name 'punchtrunk-offline-*' | head -n1)
          echo "PUNCHTRUNK_HOME=${BUNDLE_DIR}" >> $GITHUB_ENV
          echo "PUNCHTRUNK_TRUNK_BINARY=${BUNDLE_DIR}/trunk/bin/trunk" >> $GITHUB_ENV
          echo "${BUNDLE_DIR}/bin" >> $GITHUB_PATH
          echo "${BUNDLE_DIR}/trunk/bin" >> $GITHUB_PATH

      - name: Run PunchTrunk
        run: |
          punchtrunk --mode fmt,lint,hotspots \
            --base-branch=origin/${{ github.event_name == 'pull_request' && github.event.pull_request.base.ref || 'main' }}

      - name: Upload SARIF
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: reports/hotspots.sarif

      - name: Upload reports (optional)
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: punchtrunk-reports
          path: reports/
          retention-days: 7
```

### Lightweight integration (reuse installer)

For developer-focused pipelines where sudo is available, you can keep using the install script. PunchTrunk detects missing Trunk binaries and installs them automatically; add a cache on `~/.trunk/bin` to speed up subsequent jobs.

```yaml
jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/cache@v4
        with:
          path: |
            ~/.cache/trunk
            ~/.trunk/bin
          key: trunk-${{ runner.os }}-${{ hashFiles('.trunk/trunk.yaml') }}

      - name: Install PunchTrunk
        run: |
          curl -fsSL https://raw.githubusercontent.com/IAmJonoBo/PunchTrunk/main/scripts/install.sh | bash

      - name: Run hotspots only
        run: punchtrunk --mode hotspots --base-branch=origin/main
```

### Security scanning with Semgrep

PunchTrunk includes an offline Semgrep configuration for security scanning. To integrate it into your GitHub Actions workflow:

```yaml
jobs:
  security:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Setup Python
        uses: actions/setup-python@v5
        with:
          python-version: '3.x'

      - name: Install Semgrep
        run: pip install semgrep

      - name: Run security scan
        run: make security

      # Or run semgrep directly
      - name: Run Semgrep (alternative)
        run: semgrep --config=semgrep/offline-ci.yml --metrics=off .
```

The offline configuration (`semgrep/offline-ci.yml`) includes rules for:
- Python debug statements
- Shell command injection risks (Go)
- Unsafe curl-to-shell patterns

You can integrate this as a separate job or as an additional step in your existing quality checks job.

## GitLab CI

### Complete pipeline (offline bundle)

```yaml
# .gitlab-ci.yml

stages:
  - quality
  - security

variables:
  PUNCHTRUNK_BUNDLE_URL: "https://github.com/IAmJonoBo/PunchTrunk/releases/latest/download/punchtrunk-offline-linux-amd64.tar.gz"
  PUNCHTRUNK_INSTALL_DIR: "$CI_PROJECT_DIR/.punchtrunk"

cache:
  key: "trunk-${CI_COMMIT_REF_SLUG}"
  paths:
    - .cache/trunk

.punchtrunk-base:
  before_script:
    - git fetch --unshallow || true # Ensure full history
    - curl -L "${PUNCHTRUNK_BUNDLE_URL}" -o ${CI_PROJECT_DIR}/punchtrunk-offline.tgz
    - bash ./scripts/setup-airgap.sh --bundle ${CI_PROJECT_DIR}/punchtrunk-offline.tgz --install-dir "${PUNCHTRUNK_INSTALL_DIR}" --force
    - source "${PUNCHTRUNK_INSTALL_DIR}/punchtrunk-airgap.env"
    - export PATH="${PUNCHTRUNK_HOME}/bin:${PATH}"

quality:fmt-lint:
  extends: .punchtrunk-base
  stage: quality
  script:
    - punchtrunk --mode fmt,lint --autofix=none
  allow_failure: true
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH

security:hotspots:
  extends: .punchtrunk-base
  stage: security
  script:
    - punchtrunk --mode hotspots --base-branch=origin/${CI_DEFAULT_BRANCH}
  artifacts:
    reports:
      sast: reports/hotspots.sarif
    paths:
      - reports/
    expire_in: 1 week
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH
```

### Lightweight job (installer reuse)

```yaml
quality:punchtrunk:
  image: golang:1.22
  stage: quality
  cache:
    key: "trunk-${CI_COMMIT_REF_SLUG}"
    paths:
      - .cache/trunk
      - ${HOME}/.trunk/bin
  before_script:
    - curl -fsSL https://raw.githubusercontent.com/IAmJonoBo/PunchTrunk/main/scripts/install.sh | bash
    - export PATH="${HOME}/.trunk/bin:/usr/local/bin:${PATH}"
  script:
    - punchtrunk --mode fmt,lint,hotspots --verbose
  artifacts:
    reports:
      sast: reports/hotspots.sarif
```

## CircleCI

```yaml
# .circleci/config.yml

version: 2.1

executors:
  punchtrunk:
    docker:
      - image: cimg/base:stable

commands:
  install-punchtrunk:
    steps:
      - run:
          name: Install PunchTrunk bundle
          command: |
            curl -L https://github.com/IAmJonoBo/PunchTrunk/releases/latest/download/punchtrunk-offline-linux-amd64.tar.gz -o /tmp/punchtrunk-offline.tgz
            bash ./scripts/setup-airgap.sh --bundle /tmp/punchtrunk-offline.tgz --install-dir "${HOME}/.punchtrunk" --force
            echo 'source ${HOME}/.punchtrunk/punchtrunk-airgap.env' >> $BASH_ENV

jobs:
  quality-check:
    executor: punchtrunk
    steps:
      - checkout
      - run:
          name: Fetch full history
          command: git fetch --unshallow || true
      - install-punchtrunk
      - run:
          name: Run PunchTrunk
          command: punchtrunk --mode fmt,lint,hotspots --base-branch=origin/main
      - store_artifacts:
          path: reports/
          destination: punchtrunk-reports

workflows:
  version: 2
  build-and-test:
    jobs:
      - quality-check
```

## Jenkins

### Jenkinsfile (Declarative Pipeline)

```groovy
pipeline {
    agent any

    environment {
        PUNCHTRUNK_MODE = 'fmt,lint,hotspots'
        BASE_BRANCH = 'origin/main'
        PUNCHTRUNK_BUNDLE_URL = 'https://github.com/IAmJonoBo/PunchTrunk/releases/latest/download/punchtrunk-offline-linux-amd64.tar.gz'
        PUNCHTRUNK_INSTALL_DIR = "${env.WORKSPACE}/.punchtrunk"
    }

    stages {
        stage('Setup') {
            steps {
                sh 'git fetch --unshallow || true'
                sh '''
                  curl -L ${PUNCHTRUNK_BUNDLE_URL} -o ${WORKSPACE}/punchtrunk-offline.tgz
                  bash ./scripts/setup-airgap.sh --bundle ${WORKSPACE}/punchtrunk-offline.tgz --install-dir ${PUNCHTRUNK_INSTALL_DIR} --force
                '''
                sh 'echo "source ${PUNCHTRUNK_INSTALL_DIR}/punchtrunk-airgap.env" >> ${WORKSPACE}/.punchtrunk-env'
            }
        }

        stage('Quality Check') {
            steps {
                sh '. ${WORKSPACE}/.punchtrunk-env && punchtrunk --mode ${PUNCHTRUNK_MODE} --base-branch=${BASE_BRANCH} --verbose'
            }
        }

        stage('Archive Results') {
            steps {
                archiveArtifacts artifacts: 'reports/**/*', allowEmptyArchive: true
            }
        }
    }

    post {
        always {
            cleanWs()
        }
    }
}
```

## Ephemeral Runners

PunchTrunk is optimized for ephemeral runners with:

- Fast cold starts (< 1 minute with caching)
- Minimal dependencies (single binary or offline bundle)
- Effective caching strategies
- Graceful degradation on shallow clones
- Explicit offline controls (`PUNCHTRUNK_AIRGAPPED=1` to skip installer downloads, `--trunk-binary` or `PUNCHTRUNK_TRUNK_BINARY` to point at a pre-baked CLI)

### GitHub Actions with Ephemeral Runners

```yaml
jobs:
  quality:
    runs-on: self-hosted-ephemeral # Your ephemeral runner label
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0 # Critical for hotspot analysis

      # Cache restoration is automatic with actions/cache
      - uses: actions/cache@v4
        with:
          path: |
            ~/.cache/trunk
            /usr/local/bin/punchtrunk
          key: punchtrunk-${{ runner.os }}-${{ hashFiles('.trunk/trunk.yaml') }}

      - name: Install or use cached PunchTrunk
        run: |
          if ! command -v punchtrunk &> /dev/null; then
            curl -fsSL https://raw.githubusercontent.com/IAmJonoBo/PunchTrunk/main/scripts/install.sh | bash
          fi

      - name: Run PunchTrunk
        run: punchtrunk --mode fmt,lint,hotspots
```

### Kubernetes Ephemeral Pods

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: punchtrunk-job
spec:
  template:
    spec:
      restartPolicy: Never
      volumes:
        - name: workspace
          emptyDir: {}
        - name: punchtrunk-bundle
          emptyDir: {}
      initContainers:
        - name: fetch-code
          image: alpine/git:latest
          command:
            - sh
            - -c
            - |
              git clone --depth 100 ${GIT_REPO} /workspace
              cd /workspace
              git checkout ${GIT_COMMIT}
          volumeMounts:
            - name: workspace
              mountPath: /workspace
        - name: unpack-punchtrunk
          image: cgr.dev/chainguard/wolfi-base:latest
          command:
            - sh
            - -c
            - |
              curl -L ${PUNCHTRUNK_BUNDLE_URL} -o /tmp/punchtrunk-offline.tgz
              bash /workspace/scripts/setup-airgap.sh --bundle /tmp/punchtrunk-offline.tgz --install-dir /opt/punchtrunk --force
              cp -R /opt/punchtrunk/. /bundle
          env:
            - name: PUNCHTRUNK_BUNDLE_URL
              valueFrom:
                secretKeyRef:
                  name: punchtrunk-artifact
                  key: bundle-url
          volumeMounts:
            - name: workspace
              mountPath: /workspace
            - name: punchtrunk-bundle
              mountPath: /bundle
      containers:
        - name: punchtrunk
          image: cgr.dev/chainguard/wolfi-base:latest
          workingDir: /workspace
          command:
            - sh
            - -c
            - |
              source /punchtrunk/punchtrunk-airgap.env
              punchtrunk --mode fmt,lint,hotspots --base-branch=origin/main
          volumeMounts:
            - name: workspace
              mountPath: /workspace
            - name: punchtrunk-bundle
              mountPath: /punchtrunk
```

## Offline & Air-Gapped Environments

PunchTrunk provides an offline bundle workflow so agents can run without external network access. The bundle includes the PunchTrunk binary, a Trunk CLI executable, the repo-specific Trunk configuration, optional cached toolchain assets, and verified checksums. The generated `manifest.json` now also records the pinned CLI version, trunk.yaml checksum, cache source path, and hydration outcome for reproducibility audits.

### Build the offline bundle

```bash
make offline-bundle
# or customize the output
./scripts/build-offline-bundle.sh \
  --punchtrunk-binary ./bin/punchtrunk \
  --target-os linux \
  --target-arch amd64 \
  --output-dir dist \
  --bundle-name punchtrunk-offline-linux-amd64.tar.gz
```

Useful flags include:

- `--punchtrunk-binary` to point at a custom build (for example a nightly artifact).
- `--trunk-binary` to reuse a pre-installed Trunk CLI path when building on a staging host.
- `--cache-dir` to embed an existing `~/.cache/trunk` so linters run without outbound downloads.
- `--no-cache` to produce a minimal archive when storage is tight.
- `--skip-hydrate` to disable the default prefetch step (the script issues `trunk install --ci` to warm caches when hydration is enabled).
- `--target-os` / `--target-arch` to fetch the correct Trunk binary for another platform from the same build host.
- Omitting `--trunk-binary` instructs the script to auto-download the pinned Trunk release that matches `.trunk/trunk.yaml`.

The script writes `<bundle>.tar.gz` and a companion `<bundle>.tar.gz.sha256` checksum to the chosen output directory.

### Use the bundle on target hosts

1. Copy the archive and its checksum to the offline machine.
1. Verify integrity:

```bash
shasum -a 256 punchtrunk-offline-linux-amd64.tar.gz
cat punchtrunk-offline-linux-amd64.tar.gz.sha256
```

1. Extract the bundle and source the environment helper:

```bash
tar -xzf punchtrunk-offline-linux-amd64.tar.gz
cd punchtrunk-offline-linux-amd64
source ./punchtrunk-airgap.env
```

1. Run PunchTrunk as usual, forwarding `--trunk-binary` when scripts need an explicit path:

```bash
${PUNCHTRUNK_HOME}/bin/punchtrunk --mode hotspots --base-branch HEAD~1 --trunk-binary "${PUNCHTRUNK_TRUNK_BINARY}"
```

The bundle ships `punchtrunk-airgap.env` (POSIX shells) and `punchtrunk-airgap.ps1` (PowerShell) alongside `README.txt`. When you install with `setup-airgap.sh` or `setup-airgap.ps1`, the scripts reuse the same helpers and emit them in the installation directory for consistency.

### Provision with setup scripts

The repository ships helper scripts that unpack the offline bundle, create stable symlinks/wrappers, wire cache directories, and emit reusable environment exports.

#### Linux/macOS

```bash
./scripts/setup-airgap.sh \
  --bundle /path/to/punchtrunk-offline-linux-amd64.tar.gz \
  --install-dir /opt/punchtrunk \
  --force

source /opt/punchtrunk/punchtrunk-airgap.env
```

#### Windows (PowerShell 7+)

```powershell
pwsh ./scripts/setup-airgap.ps1 `
  -BundlePath C:\Artifacts\punchtrunk-offline-windows-amd64.tar.gz `
  -InstallDir "C:\ProgramData\PunchTrunk" `
  -Force

. "C:\ProgramData\PunchTrunk\punchtrunk-airgap.ps1"
```

Both scripts validate optional checksum files, lay down cache directories, and print the locations of wrapper binaries (`punchtrunk.cmd`, `trunk.cmd`) alongside the environment export file.

### Validate the air-gapped setup

Before sealing network access, run PunchTrunk's diagnostic mode to confirm prerequisites:

```bash
${PUNCHTRUNK_HOME}/bin/punchtrunk --mode diagnose-airgap \
  --trunk-binary "${PUNCHTRUNK_TRUNK_BINARY}" \
  --sarif-out "${PUNCHTRUNK_HOME}/reports/hotspots.sarif"
```

- Emits a JSON document on stdout enumerating git availability, the resolved Trunk binary, air-gap environment flags, and SARIF writeability
- Exits non-zero when any check reports `error`, allowing provisioning scripts to halt before runtime failures
- Skips installer downloads and other side effects so it is safe to run on staging hosts and production images alike

Follow up with `punchtrunk --mode tool-health` to confirm the pinned Trunk CLI version matches `.trunk/trunk.yaml` and that cached runtimes/linters are present before revoking network access.

### Bundle contents

- `bin/punchtrunk` – the PunchTrunk CLI the bundle was built from.
- `trunk/bin/trunk` – the pinned Trunk CLI executable.
- `trunk/config` – repository Trunk configuration used by PunchTrunk.
- `trunk/cache` – optional cached toolchain assets for offline execution.
- `manifest.json` – metadata including creation timestamp, pinned CLI version, trunk.yaml checksum, cache source path, and hydration status.
- `checksums.txt` – SHA-256 hashes for every bundled file.
- `README.txt` – manual setup instructions and environment helpers.
- `punchtrunk-airgap.env` / `punchtrunk-airgap.ps1` – sourcing scripts that set `PUNCHTRUNK_HOME`, toggle airgapped mode, and prepend the bundled binaries to `PATH`.

## Agent Integration

### GitHub Copilot / AI Agents

PunchTrunk is AI agent-friendly:

```yaml
# .github/copilot-instructions.md or agent config

Tools available:
- **punchtrunk**: Quality orchestrator
  - Commands: fmt, lint, hotspots
  - Outputs: SARIF files in reports/
  - Usage: punchtrunk --mode <modes> --base-branch <branch>

Workflow:
1. Run punchtrunk --mode fmt before commits
2. Run punchtrunk --mode lint to check code quality
3. Run punchtrunk --mode hotspots to identify risky areas
4. Parse reports/hotspots.sarif for priority files
```

### Pre-commit Hooks

```yaml
# .pre-commit-config.yaml
repos:
  - repo: local
    hooks:
      - id: punchtrunk-fmt
        name: PunchTrunk Format
        entry: punchtrunk --mode fmt
        language: system
        pass_filenames: false

      - id: punchtrunk-lint
        name: PunchTrunk Lint
        entry: punchtrunk --mode lint --autofix=none
        language: system
        pass_filenames: false
```

## Best Practices

### 1. Git History Depth

**Always fetch full history for hotspots:**

```yaml
- uses: actions/checkout@v4
  with:
    fetch-depth: 0 # Not fetch-depth: 1
```

**Why:** Hotspot analysis requires git history to compute churn. Shallow clones will produce incomplete results.

### 2. Caching Strategy

**Cache these directories:**

```yaml
~/.cache/trunk       # Trunk tool cache
/usr/local/bin/punchtrunk  # Binary cache (if installed)
```

**Expected speedup:**

- First run: ~5-10 minutes (tool downloads)
- Cached run: ~1-2 minutes

### 3. Timeout Configuration

**Recommended timeouts:**

- Small repos (< 100 files): 10 minutes
- Medium repos (100-1000 files): 20 minutes
- Large repos (> 1000 files): 30 minutes

```yaml
jobs:
  quality:
    timeout-minutes: 20
```

### 4. Error Handling

**Don't fail the pipeline on lint issues:**

```yaml
- name: Run PunchTrunk
  run: punchtrunk --mode lint
  continue-on-error: true
```

**Do fail on critical errors:**

```yaml
- name: Run PunchTrunk
  run: |
    punchtrunk --mode fmt,lint,hotspots || {
      echo "PunchTrunk failed"
      exit 1
    }
```

### 5. Base Branch Configuration

**For PRs:**

```bash
punchtrunk --mode hotspots --base-branch=origin/${BASE_BRANCH}
```

**For main branch:**

```bash
punchtrunk --mode hotspots --base-branch=HEAD~10
```

### 6. Resource Limits

**Kubernetes resource limits:**

```yaml
resources:
  limits:
    memory: 1Gi
    cpu: 2
```

**Expected resource usage:**

- Memory: < 500 MB for most repos (cache-heavy linters may use more)
- CPU: Scales with file count; two cores keep Trunk responsive
- Disk: < 100 MB for PunchTrunk/Trunk binaries plus repository size

## Troubleshooting

### Issue: "No hotspots generated"

**Cause:** Shallow clone or no git history

**Solution:**

```yaml
- uses: actions/checkout@v4
  with:
    fetch-depth: 0
```

### Issue: "trunk: command not found"

**Cause:** Trunk CLI not installed

**Solution:**

```bash
curl https://get.trunk.io -fsSL | bash -s -- -y
echo "${HOME}/.trunk/bin" >> $GITHUB_PATH
```

### Issue: "Outbound network blocked"

**Cause:** Runners cannot reach <https://get.trunk.io>

**Solution:**

```bash
# Pre-install Trunk and skip downloads
export PUNCHTRUNK_AIRGAPPED=1
punchtrunk --mode lint --trunk-binary=/opt/trunk/bin/trunk
```

### Issue: "Slow CI runs"

**Cause:** No caching, cold starts

**Solution:**

```yaml
- uses: actions/cache@v4
  with:
    path: ~/.cache/trunk
    key: trunk-${{ runner.os }}-${{ hashFiles('.trunk/trunk.yaml') }}
```

### Issue: "Workspace permission errors"

**Cause:** Runner user cannot write to repo or cache directories

**Solution:**

```bash
# Ensure the workspace is writable
sudo chown -R $(id -u):$(id -g) .

# Or direct PunchTrunk to a writable temp directory
export PUNCHTRUNK_OUTPUT_ROOT=/tmp/punchtrunk
```

### Issue: "SARIF upload failed"

**Cause:** Invalid SARIF format or permissions

**Solution:**

```bash
# Validate SARIF
jq empty reports/hotspots.sarif

# Check permissions
# Ensure security-events: write in workflow
# Check fallback path
ls -R /tmp/punchtrunk/reports  # PunchTrunk logs when it redirects output
```

## Next Steps

- [SARIF Schema Documentation](SARIF_SCHEMA.md)
- [Testing Strategy](testing-strategy.md)
- [Deployment Pipeline](delivery/DEPLOYMENT_PIPELINE.md)
- [Contributing Guide](../CONTRIBUTING.md)
