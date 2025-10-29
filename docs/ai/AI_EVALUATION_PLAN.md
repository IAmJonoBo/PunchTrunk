# AI Evaluation Plan

## Metrics

- **Quality:** Validate SARIF schema with `sarif-tools validate`; measure hotspot ranking stability against golden datasets using Spearman ρ ≥ 0.9; track Trunk hold-the-line violations per PR.
- **Safety:** Ensure secrets detection catches seeded secrets (0 false negatives) and AI-assisted PRs document assistance per `.github/copilot-instructions.md`.
- **Efficiency:** CI runtime p95 for fmt+lint+hotspots <10 minutes; hotspot computation ≤2 minutes for large repos.

## Datasets & protocol

- Maintain synthetic repositories with known churn patterns and seeded hotspots; refresh quarterly to keep language coverage current.
- Store baseline SARIF fixtures under `testdata/sarif/*.json`; compare new runs via JSON diff to identify regressions.
- Require platform maintainer sign-off when metrics dip below thresholds before release or dependency upgrades.

## Automation

- Add `make eval-hotspots` target executing hotspot scoring against fixtures and validating SARIF output with `jq` and `sarif-tools`. Baseline SARIF lives at `testdata/sarif/hotspots-fixture.sarif`, maintained via `PUNCHTRUNK_CREATE_BASELINE=1 ./scripts/eval-hotspots.sh` when scenarios change.
- Extend CI with optional nightly workflow to run evaluation suite; fail PRs introducing regressions (e.g., ranking correlation <0.9 or runtime >20% increase).
