# AI Guardrails

- **Data minimisation:** Never include secrets or PII in prompts unless explicitly approved; scrub logs before sharing.
- **Provenance:** Prefer retrieval-augmented responses referencing project docs (`docs/`) and clearly cite sources in generated content.
- **Bias & safety:** Maintain a red-team prompt set covering injection, bias, and policy violations; log and review hits each sprint.
- **Human oversight:** All AI-generated code must be reviewed by a maintainer and covered by tests (`go test` once available, plus CI checks).
- **Rollbacks:** Keep changes compartmentalised (small commits) so reverts are trivial; document AI assistance in PR descriptions.

References: OWASP Top 10, ASVS, n00tropic security policy.
