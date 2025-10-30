# Trunk Configuration Reference

_This is a Reference document within the Diátaxis framework._

## CLI Version

- `.trunk/trunk.yaml` pins `cli.version: 1.25.0`.
- Update via `trunk upgrade --yes` then commit the new version.

## Runtimes

- Go `1.22.3`
- Node `22.16.0`
- Python `3.10.8`

Keep these consistent with project tooling. If you add a runtime, document the change here.

## Enabled Linters

- `actionlint@1.7.8`
- `checkov@3.2.488`
- `git-diff-check`
- `gofmt@1.22.3`
- `golangci-lint2@2.6.0`
- `hadolint@2.14.0`
- `markdownlint@0.45.0`
- `osv-scanner@2.2.4`
- `prettier@3.6.2`
- `shellcheck@0.11.0`
- `shfmt@3.6.0`
- `trufflehog@3.90.12`
- `yamllint@1.37.1`

Use `trunk check --all` when verifying new linter additions.

## Config Overrides

- `.trunk/configs/.markdownlint.yaml`: aligns markdownlint with Prettier formatting.
- `.trunk/configs/.shellcheckrc`: enables all checks but disables `SC2154` to tolerate Trunk env vars.
- `.golangci.yml`: uses config schema `version: "2"` and disables test runs so golangci-lint stays aligned with the newer `golangci-lint2` binary shipped by Trunk.
- `.trunk/configs/.yamllint.yaml`: tightens YAML quoting and duplicate-key rules.
- `.trunk/configs/.hadolint.yaml`: suppresses shell sourcing warnings (`SC1090`, `SC1091`).

Update overrides rather than global ignoring so the repository stays hermetic.

## Actions

- `trunk-announce`
- `trunk-check-pre-push`
- `trunk-fmt-pre-commit`
- `trunk-upgrade-available`

Disable an action only after communicating expectations in `docs/CONVENTIONS.md`.

## Maintaining Versions

- Run `trunk upgrade --yes` in a feature branch to pick up new runtime or linter releases; review the diff in `.trunk/trunk.yaml` and `.trunk/configs/` before committing.
- After upgrades, execute `trunk check --all` and `make run` to ensure the orchestrator still succeeds locally.
- Update the version lists in this document so incoming contributors know what to expect, and note significant changes in `docs/overview.md` or relevant ADRs.
- Keep the Go runtime pinned in `.trunk/trunk.yaml` aligned with the minimum Go version declared in `go.mod`; if they diverge temporarily, document the reason in the release notes.
