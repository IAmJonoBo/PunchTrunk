# QA Checklist (pre-release)

- [ ] Acceptance criteria met: feature behaves as described in linked issue/ADR; demo recorded if applicable.
- [ ] Static analysis clean: `./bin/punchtrunk --mode fmt,lint` and `make hotspots` run without new findings; SARIF validated with `jq`.
- [ ] Docs updated: README, AGENTS, docs/ directory entries, release notes (if behaviour or flags change).
- [ ] Tests executed: `go test ./...` (when tests exist) and any evaluation suites (`make eval-hotspots`, validates SARIF output against baseline fixtures).
- [ ] Rollback ready: previous binary/docker tag verified, changelog notes include rollback guidance.
