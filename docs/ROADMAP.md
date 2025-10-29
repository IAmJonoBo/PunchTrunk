# PunchTrunk Roadmap

## Completed (Q4 2025)

- ✅ Add unit tests for hotspot helpers by extracting reusable packages.
  - Added comprehensive unit tests for `roughComplexity`, `meanStd`, `splitCSV`, `atoiSafe`
  - All tests passing, 100% coverage for helper functions
- ✅ Wire optional Semgrep rules (`semgrep/print-debug.yml`) into `.trunk/trunk.yaml` behind a flag.
  - Added commented configuration in trunk.yaml with instructions
  - Users can enable by uncommenting the Semgrep section
- ✅ Document SARIF schema expectations for downstream automation.
  - Created comprehensive `docs/SARIF_SCHEMA.md` documentation
  - Covers structure, validation, GitHub integration, troubleshooting
- ✅ Provide prebuilt binaries via GitHub Releases with signing notes.
  - Implemented multi-platform release workflow (Linux, macOS, Windows on AMD64/ARM64)
  - Added SHA256 checksums for all binaries
  - Created installation script at `scripts/install.sh`
- ✅ Publish a container image to a public registry with cosign signatures.
  - Container images published to ghcr.io with multi-arch support
  - Images signed with cosign (keyless OIDC)
  - Automated post-release validation

## Next

- Add CLI flag for custom churn windows to support long-lived branches.
- Add structured release notes with changelog generation.
- Implement CODEOWNERS-aware hotspot scoring.

## Later

- Extend hotspots to include ownership metadata (CODEOWNERS integration).
- Explore additional Trunk actions for pre-commit enforcement across languages.
- Evaluate structured logging to replace `log.Printf` for richer diagnostics.
- Add golden SARIF fixtures for regression testing.
- Implement performance benchmarking suite.

## Notes

- Keep roadmap entries in sync with `docs/releasing.md` and `docs/testing-strategy.md`.
- Update after each planning session; link issues when they become actionable.
- Release workflow is ready for use via tags (e.g., `git tag v1.0.0 && git push --tags`).

