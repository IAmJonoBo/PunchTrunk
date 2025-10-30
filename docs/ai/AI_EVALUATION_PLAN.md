# AI Evaluation Plan

This plan ties together evaluation requirements from the quality and testing docs under `docs/`. Use it to ensure new features—and AI-assisted changes—satisfy our gates before merge or release.

## Canonical References

- `docs/testing-strategy.md`: Test pyramid, coverage goals, execution commands.
- `docs/internal/quality/QUALITY_GATES.md`: Merge/release gates, including evaluation suite.
- `docs/internal/quality/QA_CHECKLIST.md`: Pre-release verification steps.
- `docs/internal/operations/ci.md`: CI workflow expectations and maintenance.
- `docs/releasing.md` & `docs/RELEASE_PREP_SUMMARY.md`: Release hand-off requirements.
- `docs/CONVENTIONS.md` & `docs/security-supply-chain.md`: Coding, security, and supply chain guardrails.

## Metrics & Success Criteria

| Dimension         | Target / Guardrail                                                                                                                                    | Source                          |
| ----------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------- |
| **Quality**       | SARIF validates via `jq` + `sarif validate`; Spearman ρ ≥ 0.9 against baseline hotspots; Trunk hold-the-line violations = 0                           | Quality gates, Testing strategy |
| **Safety**        | Secrets detection catches seeded secrets (0 false negatives); dependency advisories triaged; AI usage disclosed per `.github/copilot-instructions.md` | QA checklist, security docs     |
| **Efficiency**    | fmt+lint+hotspots CI stage p95 < 10 min; hotspots computation ≤ 2 min on reference repo; Kitchen Sink < 1 min                                         | Testing strategy, CI operations |
| **Coverage**      | Core logic ≥ 80%, CLI orchestration ≥ 70%, error paths ≥ 60%                                                                                          | Testing strategy                |
| **Documentation** | README / AGENTS / docs updated when flags or workflows change                                                                                         | QA checklist                    |

## Test & Evaluation Matrix

| Layer       | Command                                                        | Purpose                                              | Related Docs                            |
| ----------- | -------------------------------------------------------------- | ---------------------------------------------------- | --------------------------------------- |
| Unit        | `go test ./...` (or focused packages)                          | Fast regression safety net                           | testing-strategy                        |
| E2E         | `go test ./cmd/punchtrunk -run "TestE2E"`                      | Validate real workflows & Kitchen Sink               | testing-strategy, QUALITY_GATES         |
| Integration | CI workflows + `trunk check`                                   | Exercise real Trunk CLI and SARIF upload             | operations/ci.md                        |
| Evaluation  | `make eval-hotspots`                                           | Diff hotspots vs baseline fixture; schema validation | QA_CHECKLIST, QUALITY_GATES             |
| Manual      | `./bin/punchtrunk --mode fmt,lint,hotspots`, `jq` over reports | Smoke test prior to release                          | QA_CHECKLIST, releasing.md              |
| Security    | `govulncheck`, secrets scans, offline bundle integrity checks  | Maintain security gates                              | QUALITY_GATES, security-supply-chain.md |

## Datasets & Fixtures

- Synthetic repos (Go/Python/JS/Markdown) generated on the fly mirror the Kitchen Sink scenario—update templates under `scripts/` when language coverage changes.
- Baseline SARIF files live in `testdata/sarif/`; refresh with `PUNCHTRUNK_CREATE_BASELINE=1 ./scripts/eval-hotspots.sh` after intentional scoring changes.
- Maintain golden datasets for churn and complexity edge cases; add new fixtures when bugs regress.
- Archive evaluation outputs for quarterly review to spot drift in hotspot ranking or runtime.

## Automation & Tooling

- `make test`: Runs Go tests and Bats shell suites (per QA checklist).
- `make eval-hotspots`: Builds binary, executes `scripts/eval-hotspots.sh`, validates SARIF, and compares against baseline. Fails the gate if output diverges.
- Manage Python tooling with `uv`: `uv venv` (once) then `uv pip sync requirements.lock` to provision the `sarif` CLI for schema checks.
- `./bin/punchtrunk --mode fmt,lint`: Required static analysis gate prior to merge; mirrors QA checklist.
- Nightly (optional) workflow: schedule `make eval-hotspots` and extended integration runs; alert on metric regressions (ρ, runtime, diff).
- CI must retain `fetch-depth: 0` and upload SARIF via `codeql-action` (operations/ci.md); record deviations in `docs/CONVENTIONS.md`.
- When `sarif` CLI is unavailable, log the warning and track follow-up in QA checklist to install `sarif-tools` on runners.

## Quality Gate Compliance Checklist

1. **Static Analysis**: `./bin/punchtrunk --mode fmt,lint` or `make run` (fmt+lint+hotspots) clean—aligns with QA checklist and quality gates.
2. **Tests**: `make test` (unit + Bats) + `make eval-hotspots` pass locally and in CI.
3. **Metrics Review**: Confirm runtime, coverage, and hotspot correlation targets; update `docs/testing-strategy.md` if thresholds shift.
4. **Docs**: Update README, AGENTS, relevant `docs/` pages, and release notes when flags or workflows change (QA checklist requirement).
5. **Security**: Run `govulncheck`, secrets detection, offline bundle integrity verification (quality gates) before release.
6. **Output Validation**: `jq` + `sarif validate` on `reports/hotspots.sarif`; ensure upload step configured per operations doc.
7. **Release Prep**: Follow `docs/releasing.md` and `docs/RELEASE_PREP_SUMMARY.md` for sign-off, rollback plan, and documentation hand-off.

## Governance & Maintenance

- Revisit baseline fixtures quarterly or when algorithms change; document decisions in `docs/hotspots-methodology.md`.
- Track evaluation trends (runtime, correlation, false positives) in release retros; file issues when gates approach limits.
- Rotate CI caches and dependency versions per `operations/ci.md`; log updates in `docs/CONVENTIONS.md`.
- Record AI tooling adjustments, prompts, or guardrails in `docs/ai/AGENT_SYSTEM_SPEC.md` and ensure `.github/copilot-instructions.md` remains in sync.

## Incident Response

- If evaluation gates fail (baseline diff, schema invalid, runtime regression), pause release, open an incident per `docs/internal/policies/SECURITY_POLICY.md`, and document mitigation.
- For hotfixes, capture bypass rationale and schedule follow-up to restore full evaluation coverage within 24 hours (per QUALITY_GATES).

## Future Enhancements

- Expand evaluation fixtures to include additional languages and large-repo churn patterns.
- Automate Spearman correlation reporting inside `scripts/eval-hotspots.sh`.
- Add dashboards tracking evaluation metrics over time (tie into `docs/internal/ops/OBSERVABILITY_SPEC.md`).
- Integrate offline bundle validation and dependency scans into nightly evaluation workflows.
