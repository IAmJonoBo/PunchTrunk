# Type Safety & Validation

- Prefer static typing; keep logic in small, testable functions. When hotspot helpers move to packages, expose minimal public surface.
- Validate external inputs: command-line flags, environment variables, and git/trunk outputs. Return descriptive errors when parsing fails.
- Use explicit error wrapping (`fmt.Errorf("context: %w", err)`) for diagnostic depth; never ignore errors from subprocesses.
- For SARIF, validate the struct before writing. Consider adding JSON schema checks in evaluation pipeline.
- Capture testing implications in `../testing-strategy.md` and security considerations in `../security-supply-chain.md`.
