# n00tropic — AI‑Heavy Discovery Intake (10 minutes)

_Frank, fast, frontier‑grade. This is the pre‑kickoff sanity scan._

> **Outcome:** enough clarity to green‑light a deep‑dive or hit pause. Leave blanks rather than guessing. Link artefacts where you have them.

## 0) One‑liner & who it’s for

- **Working title:** PunchTrunk
- **Elevator pitch (one sentence):** A single Go binary orchestrating Trunk fmt/check and hotspot scoring so repos get hermetic linting, safe autofix, and SARIF output with minimal setup.
- **Primary users / beneficiaries (ranked):** 1) n00tropic platform teams maintaining shared repos, 2) Downstream repo owners adopting Trunk via the starter kit, 3) CI operators running ephemeral runners who need deterministic tooling.

## 1) Outcomes & must‑nots

- **Top 3 measurable outcomes:** 1) CI lint+hotspot stages finish within 10 minutes p95, 2) Hotspot SARIF publishes on every PR without schema errors, 3) Zero new hold-the-line regressions after adoption across target repos.
- **Non‑negotiables:** Hermetic tooling (pinned Trunk + Go 1.22.x), no secrets/PII in logs or SARIF, distroless runtime running as nonroot, adheres to n00tropic security disclosure process, operations must succeed in air-gapped or cached environments.

## 2) Tasks & autonomy

- **Core tasks (3–5 verbs):** Orchestrate Trunk commands, compute hotspots, emit SARIF, manage caches, signal failures via exit codes.
- **Autonomy level:** Execute (CLI runs deterministically once invoked) with Exec+Notify visibility through CI logs.
- **Human‑in‑the‑loop points:** Developers review Trunk annotations and SARIF findings before merging; maintainers approve config or mode changes; security reviews SARIF uploads.

## 3) Models & routes (sketch, not a treaty)

- **Candidates:** None — deterministic Go executable orchestrating Trunk CLI and git; no ML models in scope.
- **Routing idea:** N/A.
- **Fallback / degraded mode:** Skip hotspot scoring when git history unavailable and still surface Trunk results; fallback to running `trunk fmt/check` directly if binary unavailable.

## 4) Prompts, policies, outputs

- **System persona & boundaries:** CLI operator enforcing repository hygiene; boundaries limited to repo filesystem and configured Trunk tooling.
- **Output contract:** SARIF 2.1 file at `reports/hotspots.sarif` (`ruleId` = `hotspot`, level `note`) plus stdout/stderr logs following Trunk formatting.
- **Constrained decoding required?** No (outputs are deterministic program logs/files).

## 5) Tools & permissions

- **Tools needed:** Trunk CLI (`trunk fmt`, `trunk check` → formatted/linted workspace), Git (`git diff`, `git log --numstat` → churn stats), Go runtime (build/run CLI), optional Docker (package and run distroless image).
- **Least privilege:** Read repository working tree, execute Trunk binaries, write to `bin/` and `reports/hotspots.sarif`; no outbound network beyond Trunk downloads via cache.
- **Idempotency & retries:** Re-running with unchanged inputs yields identical SARIF and exits; safe to retry to overcome transient CI failures.

## 6) Knowledge & freshness (RAG?)

- **Authoritative sources:** Local git repository contents, `.trunk/trunk.yaml`, `.trunk/configs/` overrides, Makefile targets.
- **Freshness policy:** Hotspots inspect ~90 days of churn; caches invalidated when `.trunk/trunk.yaml` hash changes; Go/Trunk versions reviewed quarterly per docs/CONVENTIONS.md.
- **Provenance needed?** Yes — SARIF records file paths and churn metrics; CI logs capture command invocations for auditability.

## 7) Safety, privacy, compliance

- **Top 3 risks:** Secrets accidentally committed and surfaced in logs/SARIF, Trunk autofix modifying unintended files, hotspot scoring failing on shallow clones and masking risk.
- **Guardrails:** Secrets detection in Trunk config, hold-the-line defaults, git history guards that skip unreadable/binary files, human review of SARIF before acting, adherence to `.github/copilot-instructions.md` for AI assistance.
- **Data classes & rules:** Operates on source code without PII; follow n00tropic privacy policy for any logs; retain SARIF artifacts per security policy and purge when no longer needed.

## 8) Quality, latency, cost

- **Pass criteria per task:** Trunk exits succeed with no new issues, SARIF validates with `jq`, hotspot rankings highlight recently touched files, and `make run` completes without manual fixes.
- **Latency SLO (p95):** 600000 ms (10 minutes) end-to-end in CI. **Budget:** GitHub Actions runner minutes; zero token spend.
- **Offline/online evals:** Manual comparison against sample repos today; roadmap includes golden SARIF fixtures and eventual `go test` coverage for scoring helpers.

## 9) Delivery & reversibility

- **Environments & flags:** Local dev (`make run`), CI workflow (`lint-and-hotspots`), optional Docker runtime; feature control via `--mode`, `--autofix`, `--base-branch`; kill switch by disabling workflow step or flipping modes to fmt/lint only.
- **Rollout:** Introduce alongside existing lint jobs in monitor-only mode, then enforce status check once stable; rollback by reverting workflow commit or pinning previous binary.
- **Reversible/irreversible decisions:** Adoption and CLI configuration are reversible; SARIF schema stability considered effectively irreversible without coordination downstream.

## 10) Go / No‑go

- **What must be true to proceed:** Target repos validate Trunk config, hotspot scoring confirmed with representative history, CI runtime stays within budget, security sign-off on SARIF handling and disclosure channel ready.
- **Spikes to run next (≤ 1 week each):** 1) Extract hotspot helpers and add unit/golden SARIF tests, 2) Benchmark runtime on large mono-repo to size caches, 3) Run Trivy (or similar) against Docker image and document results.
- **Owners & dates:** n00tropic platform engineering (PunchTrunk maintainers); schedule spikes in upcoming sprint planning with owners tracked in ROADMAP and linked issues.

> If you can’t fill 50% of this, you’re not blocked — you’re honest. Run spikes, then the deep‑dive.
