# Releasing Guide

_This is a How-to guide._

## Release Goals

Deliver reproducible `PunchTrunk` binaries and offline bundles with minimal manual steps.

## Prerequisites

- Clean `main` branch with passing CI.
- Go toolchain aligned with `make build` requirements.
- Access to GitHub Releases (publish binaries + offline bundles).

## Steps

1. **Version Bump (if applicable)**
   - Update version references in documentation or tagging scripts.
   - Communicate changes in `docs/CONVENTIONS.md`.
2. **Build Binary**

   ```bash
   make build
   ```

   - Verify binary via `./bin/punchtrunk --help`.

3. **Run Validation**

   ```bash
   make run
   ```

   - Ensure `reports/hotspots.sarif` updates and remains valid JSON.

4. **Build Offline Bundle**

   ```bash
   make offline-bundle
   ls dist/punchtrunk-offline-*.tar.gz
   ```

   - Builder hydrates Trunk caches via `trunk fmt --fetch` / `trunk check --fetch` by default; pass `--skip-hydrate` only when you intentionally want an empty cache.
   - Verify the archive contains `punchtrunk-airgap.env`, `trunk/`, and checksums.
   - Inspect `manifest.json` for the expected `trunk_cli_version`, `trunk_config_sha256`, and `hydrate_status` values.
   - Source the bundle env file and run `punchtrunk --mode tool-health` to confirm pinned runtimes/linters are available offline.

5. **Tag the Release**

   ```bash
   git tag -a v<major.minor.patch> -m "PunchTrunk <major.minor.patch>"
   git push origin v<major.minor.patch>
   ```

   - Update release notes with highlights from commits and note any dependency upgrades (Go, Trunk, plugins).

6. **Sign / verify artifacts (optional)**

   ```bash
   shasum -a 256 dist/punchtrunk-*
   cosign sign-blob --output-signature dist/punchtrunk-offline-linux-amd64.tar.gz.sig dist/punchtrunk-offline-linux-amd64.tar.gz
   ```

   - Record digests in the release notes so operators can verify downloads.

7. **Publish**
   - Attach binaries, offline bundles, checksums, and signatures to the GitHub Release entry.

## Post-Release Checklist

- Update `docs/internal/ROADMAP.md` with shipped items.
- Announce changes with automation status and known issues.
- Monitor GitHub Code Scanning uploads for the new release.

## Maintainability

- Validate Go and Trunk versions before tagging. If versions change, sync documentation (`docs/trunk-config.md`, `docs/internal/workflows/local-dev.md`, `docs/security-supply-chain.md`) and open ADRs when appropriate.
- Run binary scans or SBOM tooling (for example, `trivy fs dist/`) to confirm dependencies are current.
- Capture notable release learnings in `docs/internal/ROADMAP.md` or follow-up issues to keep the project adaptable.
