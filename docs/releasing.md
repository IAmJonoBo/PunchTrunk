# Releasing Guide

_This is a How-to guide._

## Release Goals

Deliver a reproducible `trunk-orchestrator` binary and optional Docker image with minimal manual steps.

## Prerequisites

- Clean `main` branch with passing CI.
- Go toolchain aligned with `make build` requirements.
- Access to container registry if publishing images.

## Steps

1. **Version Bump (if applicable)**
   - Update version references in documentation or tagging scripts.
   - Communicate changes in `docs/CONVENTIONS.md`.
2. **Build Binary**

   ```bash
   make build
   ```

   - Verify binary via `./bin/trunk-orchestrator --help`.

3. **Run Validation**

   ```bash
   make run
   ```

   - Ensure `reports/hotspots.sarif` updates and remains valid JSON.

4. **Package Docker Image**

   ```bash
   make docker
   docker run --rm trunk-orchestrator:local --mode hotspots
   ```

   - Confirm the image runs as `nonroot`.

5. **Sign Artifacts (optional)**

   ```bash
   cosign sign --keyless trunk-orchestrator:local
   ```

   - Record signing digest in the release notes.

6. **Publish**
   - Attach binary and SARIF example to the release entry.
   - Push Docker image with `docker push <registry>/trunk-orchestrator:<tag>`.

## Post-Release Checklist

- Update `docs/ROADMAP.md` with shipped items.
- Announce changes with automation status and known issues.
- Monitor GitHub Code Scanning uploads for the new release.
