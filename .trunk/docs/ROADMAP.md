# PunchTrunk Roadmap

## Now (Q4 2025)

- Add unit tests for hotspot helpers by extracting reusable packages.
- Wire optional Semgrep rules (`semgrep/print-debug.yml`) into `.trunk/trunk.yaml` behind a flag.
- Document SARIF schema expectations for downstream automation.

## Next

- Add CLI flag for custom churn windows to support long-lived branches.
- Provide prebuilt binaries via GitHub Releases with signing notes (`docs/releasing.md`).
- Publish a container image to a public registry with cosign signatures.

## Later

- Extend hotspots to include ownership metadata (CODEOWNERS integration).
- Explore additional Trunk actions for pre-commit enforcement across languages.
- Evaluate structured logging to replace `log.Printf` for richer diagnostics.

## Notes

- Keep roadmap entries in sync with `docs/releasing.md` and `docs/testing-strategy.md`.
- Update after each planning session; link issues when they become actionable.
