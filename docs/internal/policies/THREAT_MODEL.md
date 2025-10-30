# Threat Model (STRIDE)

## Assets & boundaries

- **Source code repository:** Go code, Trunk configs, scripts. Boundary: git working tree accessible to PunchTrunk.
- **SARIF artefacts:** `reports/hotspots.sarif` stored locally and uploaded to GitHub Code Scanning. Boundary: local filesystem and GitHub API.
- **Build artefacts:** `bin/punchtrunk` binary and `punchtrunk:local` Docker image. Boundary: CI pipeline and container registry.
- **CI credentials:** GitHub token available to workflow. Keep usage minimal (checkout and SARIF upload only).

## Threats & mitigations

- **Spoofing:** Attacker injects malicious Trunk or git binary. _Mitigation:_ rely on Trunk-managed toolchain hashed in `.trunk/trunk.yaml`; validate PATH in CI; consider SHA checks for downloaded binaries.
- **Tampering:** Modification of SARIF or binary to hide issues. _Mitigation:_ CI rebuilds from source; SARIF validated with schema before upload; release artefacts signed with Cosign (optional but recommended).
- **Repudiation:** Contributor denies changes. _Mitigation:_ Pull requests, commit signatures (optional), and CI logs provide audit trail.
- **Information disclosure:** SARIF leaks secrets or proprietary code externally. _Mitigation:_ Secrets scanning enforced via Trunk; SARIF uploaded only to GitHub Code Scanning; redact sensitive content before external sharing.
- **Denial of Service:** Hotspot computation hangs or consumes excessive resources. _Mitigation:_ context timeouts, cap analysis to top 500 files, and degrade gracefully when git history missing.
- **Elevation of privilege:** CLI gains more access than intended. _Mitigation:_ run container as `nonroot`; limit write paths (`bin/`, `reports/`); avoid executing untrusted scripts; CI tokens have least privilege.

## Residual risks & owners

- Dependence on Trunk supply chain integrity. _Owner:_ Maintainers monitor upstream advisories and update `.trunk/trunk.yaml` promptly.
- Users bypass PunchTrunk workflows. _Owner:_ Repo owners enforce CI checks and document expectations in `CONTRIBUTING.md`.
- SARIF schema change downstream could break uploads. _Owner:_ Maintainers coordinate with GitHub; maintain compatibility tests.
