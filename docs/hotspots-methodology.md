# Hotspots Methodology

_This is an Explanation in the Diátaxis framework._

## Data Sources

- `git log --numstat` over the last 90 days (configurable) yields churn counts per file.
- `roughComplexity` computes token-per-line density from file contents.
- Changed-file bias uses `git diff --name-only <base>...HEAD` to boost active files.

## Scoring Model

1. Compute mean and standard deviation across complexity scores.
2. Convert each file's complexity into a z-score.
3. Calculate `score = log1p(churn) * (1 + complexity_z)`.
4. Apply a 15% multiplier when a file appears in the current diff.
5. Rank descending and truncate to the top 500 entries.

## Output Contract

- Results emit as SARIF 2.1 `note` level entries in `reports/hotspots.sarif`.
- Rule ID remains `hotspot`; downstream consumers expect this identifier.
- Messages include churn, complexity, and score for quick triage.

## Reliability Considerations

- **Shallow clones**: churn may be partial. The CLI skips missing history instead of failing.
- **Binary files**: `git log` reports `-` counts; we treat them as churn `1` to avoid skew.
- **Unreadable files**: `roughComplexity` returns `0` when file reads fail, keeping scoring stable.
- **Large repos**: truncating to 500 keeps SARIF manageable and avoids CI upload limits.

## Tuning Guidance

- Adjust churn window by editing the `since` argument in `gitChurn`.
- Modify bias multipliers cautiously; align changes with `docs/testing-strategy.md` and regenerate SARIF samples.
- When adding contextual data (e.g., ownership), extend SARIF properties rather than changing `ruleId`.
