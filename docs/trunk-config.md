# Trunk Configuration Reference

_This is a Reference document within the Diátaxis framework._

## CLI Version

- `.trunk/trunk.yaml` pins `cli.version: 1.25.0`.
- Update via `trunk upgrade --yes` then commit the new version.

## Runtimes

- Go `1.21.0`
- Node `22.16.0`
- Python `3.10.8`

Keep these consistent with project tooling. If you add a runtime, document the change here.

## Enabled Linters

- `actionlint@1.7.8`
- `checkov@3.2.488`
- `git-diff-check`
- `gofmt@1.20.4`
- `golangci-lint2@2.5.0`
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
- `.trunk/configs/.yamllint.yaml`: tightens YAML quoting and duplicate-key rules.
- `.trunk/configs/.hadolint.yaml`: suppresses shell sourcing warnings (`SC1090`, `SC1091`).

Update overrides rather than global ignoring so the repository stays hermetic.

## Actions

- `trunk-announce`
- `trunk-check-pre-push`
- `trunk-fmt-pre-commit`
- `trunk-upgrade-available`

Disable an action only after communicating expectations in `docs/CONVENTIONS.md`.
