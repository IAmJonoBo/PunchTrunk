# PunchTrunk Roadmap (FY26)

Focus: make PunchTrunk the default orchestrator for agents operating in offline or highly restricted environments while keeping CI-friendly ergonomics.

---

## Themes & Adoption Targets (TA)

- **Instant Offline Bootstrap** (TA: 80% of air-gapped runners provisioned without manual tweaks)
- **Agent-Friendly Diagnostics** (TA: 90% of support tickets resolved via automated guidance)
- **Observable & Scriptable Outputs** (TA: JSON logs consumed by at least two downstream agent frameworks)

Quality gates apply to every roadmap item:

1. ✅ Unit coverage ≥ 80% for new packages
2. ✅ `go test ./...` + `./bin/punchtrunk --mode fmt,lint,hotspots` (air-gapped and connected paths)
3. ✅ Markdown/docs lint clean (`trunk check` hold-the-line)
4. ✅ Scenario validation on macOS, Linux (Windows where applicable)
5. ✅ Update docs & agent playbooks before merge

---

## Near-Term Deliverables (Q4 FY25)

1. ✅ **Offline Bootstrap Bundle**

- Shipped: Oct 2025. `scripts/build-offline-bundle.sh` now produces verified archives bundling PunchTrunk, Trunk CLI, `.trunk` config, and optional cache to unblock sealed runners.
- Tests: `TestOfflineBundleSupportsAirgappedHotspots` in `cmd/punchtrunk/main_test.go` builds the bundle, unpacks it, and runs hotspots with `PUNCHTRUNK_AIRGAPPED=1`.
- Docs: README and `docs/INTEGRATION_GUIDE.md` document archive creation, verification, and usage steps for offline agents.

1. ✅ **`punchtrunk --diagnose-airgap` Command**

- Shipped: Aug 2025. Provides JSON diagnostics covering git availability, trunk binaries, airgap env vars, and SARIF writability.
- Tests: `TestDiagnoseAirgapHappyPath` and `TestDiagnoseAirgapDetectsMissingTrunk` in `cmd/punchtrunk/main_test.go` validate success/failure cases.
- Docs: README + agent guides reference the mode; outputs integrate with structured logging.

1. ✅ **JSON Logging Toggle (`--json-logs`)**

- Shipped: Oct 2025. Provides structured events for mode lifecycle, trunk discovery, SARIF writes, and diagnostics.
- Tests: `cmd/punchtrunk/main_test.go` validates JSON log shape and logger reuse.
- Docs: README updated with flag/env guidance; schema captured in `docs/logging.md`.

1. ✅ **Configurable Temp Directory (`--tmp-dir`)**

- Shipped: Oct 2025. CLI flag/env override directs SARIF fallbacks and installer staging to custom locations; integrates with diagnose checks.
- Tests: `TestRunHotspotsUsesCustomTmpDirFallback` and `TestConfigResolveTmpDirRelative` cover runtime behaviour and path resolution.
- Docs: README documents the flag and `PUNCHTRUNK_TMP_DIR`; diagnose output recommendations now mention the override.

---

## Mid-Term Initiatives (Q1 FY26)

1. **Agent Provisioning Scripts**

- Deliver `scripts/setup-airgap.sh` and a PowerShell equivalent to lay down symlinks, env vars, and caches.
- Tests: shellcheck, bats tests for Unix script; Pester tests for PowerShell.

1. **Dry-Run Planning Mode (`--dry-run`)**

- Description: print intended Trunk invocations, resolved paths, and exits without executing tools.
- Tests: unit tests ensuring no subprocesses spawn plus integration test verifying exit code zero.

1. **Air-Gapped Agent Playbook**

- Description: comprehensive guide covering provisioning, troubleshooting, FAQ, and sample CI snippets.
- Quality gate notes: peer review by two maintainers; cross-link from README and integration guide.

---

## Long-Term Exploration (Q2 FY26+)

- **Metrics Export**: integrate optional Prometheus-style counters for run durations (flag-gated for offline use).
- **Policy Packs**: curated folders of Trunk configurations tailored for languages/frameworks; publish with offline bundle.
- **Windows-Friendly Isolation**: revisit PATH isolation helper to avoid symlink dependence and expand test coverage.
- **Agent SDK Examples**: share reference integrations for leading agent frameworks consuming JSON logs & diagnostics.

---

## Stewardship Notes

- Review roadmap quarterly; sync with `docs/releasing.md` and `docs/testing-strategy.md`.
- Track feature completion in release notes and mark TA metrics met/not met.
- Every shipped item must reference issues/ADRs and include post-release validation steps.
