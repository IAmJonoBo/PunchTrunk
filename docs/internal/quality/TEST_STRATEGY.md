# Test Strategy

The in-depth plan already lives in `../testing-strategy.md`. Use that document when updating scope or adding new suites. This page keeps a quick mnemonic of the pillars so teams can scan the footprint without duplicating the details.

- **Unit:** fast, deterministic, mock externalities.
- **Integration:** real boundaries (DB, queues, HTTP), contract tests.
- **E2E/UI:** key flows only; avoid flaky nightmares.
- **Property/Fuzz:** for parsers/transformers.
- **Security:** SAST/DAST/dep scan; secrets detection.
- **Performance:** latency/throughput under load; budgets & SLOs.

Coverage principle: aim high where it matters; never ship untested core logic.
