# Contributing Guide

Thanks for helping improve PunchTrunk. This guide follows Google developer documentation style and aligns with our Diátaxis set of docs.

## Before You Start

- Install Go 1.20+ and Trunk CLI.
- Run `trunk init` once per clone to download the pinned toolchain.
- Familiarize yourself with `docs/overview.md` and `docs/workflows/local-dev.md`.

## Development Workflow

1. **Sync main**

   ```bash
   git checkout main
   git pull origin main
   git checkout -b feature/<short-description>
   ```

2. **Build**

   ```bash
   make build
   ```

3. **Run fmt + lint**

   ```bash
   ./bin/trunk-orchestrator --mode fmt,lint --autofix=fmt --base-branch=origin/main
   ```

4. **Hotspots (optional but recommended)**

   ```bash
   make hotspots
   ```

5. **Tests**
   - When tests exist run `go test ./...`.
   - Document any new test commands in `docs/testing-strategy.md`.
6. **Review**
   - Ensure `reports/hotspots.sarif` is updated or explain why it is unchanged.
   - Run `git status` to confirm Trunk autofixes are staged.

## Commit Checklist

- Write informative commit messages (present tense). See `docs/CONVENTIONS.md`.
- Include documentation updates for new flags, modes, or workflows.
- Ensure CI passes or explain failures in the pull request description.

## Pull Requests

- Complete the pull request template checklist.
- Link related issues and ADRs.
- Request reviewers familiar with Trunk integration when changing `.trunk/` assets.

## Code of Conduct

This project abides by the [Contributor Covenant](https://www.contributor-covenant.org/). Report unacceptable behavior to the maintainers.
