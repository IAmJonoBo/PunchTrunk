# PunchTrunk Integration Guide

## Overview

This guide covers integrating PunchTrunk into CI/CD pipelines, ephemeral runners, and automated workflows. PunchTrunk is designed to be ephemeral-friendly with minimal dependencies and fast cold starts.

## Table of Contents

- [Quick Start](#quick-start)
- [GitHub Actions](#github-actions)
- [GitLab CI](#gitlab-ci)
- [CircleCI](#circleci)
- [Jenkins](#jenkins)
- [Ephemeral Runners](#ephemeral-runners)
- [Container-Based Workflows](#container-based-workflows)
- [Agent Integration](#agent-integration)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

## Quick Start

### Installation

**Binary Installation:**

```bash
curl -fsSL https://raw.githubusercontent.com/IAmJonoBo/PunchTrunk/main/scripts/install.sh | bash
```

**Container Usage:**

```bash
docker pull ghcr.io/iamjonobo/punchtrunk:latest
```

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

### Complete Integration

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
      security-events: write # For SARIF upload

    steps:
      - name: Checkout code
        uses: actions/checkout@v4
        with:
          fetch-depth: 0 # Required for hotspot analysis

      # Cache Trunk tools for faster runs
      - name: Cache Trunk
        uses: actions/cache@v4
        with:
          path: ~/.cache/trunk
          key: trunk-${{ runner.os }}-${{ hashFiles('.trunk/trunk.yaml') }}
          restore-keys: |
            trunk-${{ runner.os }}-

      # Install Trunk CLI
      - name: Install Trunk
        run: |
          curl https://get.trunk.io -fsSL | bash -s -- -y
          echo "${HOME}/.trunk/bin" >> $GITHUB_PATH

      # Install PunchTrunk
      - name: Install PunchTrunk
        run: |
          curl -fsSL https://raw.githubusercontent.com/IAmJonoBo/PunchTrunk/main/scripts/install.sh | bash

      # Run PunchTrunk
      - name: Run PunchTrunk
        run: |
          punchtrunk --mode fmt,lint,hotspots \
            --base-branch=origin/${{ github.event_name == 'pull_request' && github.event.pull_request.base.ref || 'main' }} \
            --verbose
        continue-on-error: true # Don't fail on lint issues

      # Upload SARIF to GitHub Code Scanning
      - name: Upload SARIF
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: reports/hotspots.sarif

      # Optional: Upload artifacts for debugging
      - name: Upload reports
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: punchtrunk-reports
          path: reports/
          retention-days: 7
```

### Container-Based Workflow

```yaml
jobs:
  punchtrunk:
    runs-on: ubuntu-latest
    container:
      image: ghcr.io/iamjonobo/punchtrunk:latest

    steps:
      - name: Checkout code
        uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Run PunchTrunk
        run: |
          /app/punchtrunk --mode hotspots --base-branch=HEAD~10
```

### Minimal Integration (Hotspots Only)

```yaml
jobs:
  hotspots:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Run PunchTrunk Hotspots
        run: |
          docker run --rm -v $(pwd):/workspace -w /workspace \
            ghcr.io/iamjonobo/punchtrunk:latest \
            --mode hotspots

      - name: Upload SARIF
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: reports/hotspots.sarif
```

## GitLab CI

### Complete Pipeline

```yaml
# .gitlab-ci.yml

stages:
  - quality
  - security

variables:
  PUNCHTRUNK_VERSION: "latest"

.punchtrunk-base:
  image: ghcr.io/iamjonobo/punchtrunk:${PUNCHTRUNK_VERSION}
  before_script:
    - git fetch --unshallow || true # Ensure full history

quality:fmt-lint:
  extends: .punchtrunk-base
  stage: quality
  script:
    - /app/punchtrunk --mode fmt,lint --autofix=none
  allow_failure: true
  rules:
    - if: $CI_PIPELINE_SOURCE == "merge_request_event"
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH

security:hotspots:
  extends: .punchtrunk-base
  stage: security
  script:
    - /app/punchtrunk --mode hotspots --base-branch=origin/${CI_DEFAULT_BRANCH}
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

### With Binary Installation

```yaml
quality:punchtrunk:
  image: golang:1.22
  stage: quality
  before_script:
    - curl -fsSL https://raw.githubusercontent.com/IAmJonoBo/PunchTrunk/main/scripts/install.sh | bash
    - curl https://get.trunk.io -fsSL | bash -s -- -y
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
      - image: ghcr.io/iamjonobo/punchtrunk:latest

jobs:
  quality-check:
    executor: punchtrunk
    steps:
      - checkout
      - run:
          name: Fetch full history
          command: git fetch --unshallow || true
      - run:
          name: Run PunchTrunk
          command: |
            /app/punchtrunk --mode fmt,lint,hotspots
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
    agent {
        docker {
            image 'ghcr.io/iamjonobo/punchtrunk:latest'
            args '-v $WORKSPACE:/workspace -w /workspace'
        }
    }

    environment {
        PUNCHTRUNK_MODE = 'fmt,lint,hotspots'
        BASE_BRANCH = 'origin/main'
    }

    stages {
        stage('Setup') {
            steps {
                // Ensure full git history
                sh 'git fetch --unshallow || true'
            }
        }

        stage('Quality Check') {
            steps {
                sh '/app/punchtrunk --mode ${PUNCHTRUNK_MODE} --base-branch=${BASE_BRANCH} --verbose'
            }
        }

        stage('Archive Results') {
            steps {
                archiveArtifacts artifacts: 'reports/**/*', allowEmptyArchive: true

                // Optional: Publish SARIF to security tools
                publishWarnings(
                    parserConfigurations: [[
                        parserName: 'SARIF',
                        pattern: 'reports/hotspots.sarif'
                    ]]
                )
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
- Minimal dependencies (single binary or container)
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
apiVersion: v1
kind: Pod
metadata:
  name: punchtrunk-job
spec:
  restartPolicy: Never
  containers:
    - name: punchtrunk
      image: ghcr.io/iamjonobo/punchtrunk:latest
      command: ["/app/punchtrunk"]
      args: ["--mode", "hotspots", "--base-branch", "origin/main"]
      volumeMounts:
        - name: workspace
          mountPath: /workspace
      workingDir: /workspace
  volumes:
    - name: workspace
      emptyDir: {}
  initContainers:
    - name: git-clone
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
```

## Container-Based Workflows

### Docker Compose

```yaml
# docker-compose.yml
version: "3.8"

services:
  punchtrunk:
    image: ghcr.io/iamjonobo/punchtrunk:latest
    volumes:
      - .:/workspace
    working_dir: /workspace
    command: --mode fmt,lint,hotspots
```

Usage:

```bash
docker-compose run --rm punchtrunk
```

### Makefile Integration

```makefile
.PHONY: quality hotspots docker-quality

quality:
 punchtrunk --mode fmt,lint,hotspots

hotspots:
 punchtrunk --mode hotspots --base-branch=origin/main

docker-quality:
 docker run --rm -v $(PWD):/workspace -w /workspace \
  ghcr.io/iamjonobo/punchtrunk:latest \
    --mode fmt,lint,hotspots
```

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

**Container resource limits:**

```yaml
resources:
  limits:
    memory: 1Gi
    cpu: 2
```

**Expected resource usage:**

- Memory: < 500 MB for most repos
- CPU: Scales with file count
- Disk: < 100 MB (plus repo size)

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

### Issue: "Container permission errors"

**Cause:** Container runs as nonroot, volume permissions

**Solution:**

```bash
# Option 1: Run as current user
docker run --rm --user $(id -u):$(id -g) -v $(pwd):/workspace ...

# Option 2: Fix permissions
docker run --rm -v $(pwd):/workspace ... sh -c "chown -R nonroot:nonroot /workspace"
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
