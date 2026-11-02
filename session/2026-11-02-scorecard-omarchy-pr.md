# 2026-11-02: OpenSSF Scorecard and the omarchy-pkgs submission

## Done

- `.github/workflows/scorecard.yml`: `ossf/scorecard-action` v2.5.0 on
  every push to `master`, weekly and on branch protection changes;
  SARIF to the workflow artifacts and to code scanning, results
  published to api.scorecard.dev. Every action pinned to a commit SHA
  (the versions of ossf/scorecard's own example workflow); the workflow
  is `read-all`, the analysis job alone has `security-events: write` and
  `id-token: write`. actionlint finds nothing in it.
- README: the OpenSSF Scorecard badge (score, linked to the
  scorecard.dev viewer) and the Scorecard workflow badge, after the
  release badge.
- specs/ci.md: the Scorecard section with the API's workflow rules, the
  pinning and token decisions and what the badges mean; specs/README.md
  line updated.
- First Scorecard run on `932196a` passed and published: score 3.7. The
  badge endpoint serves an SVG with that score.
- packaging/omarchy/PR.md and the submit steps name 0.4.21. The
  omarchy-pkgs validators were rerun on the 0.4.21 recipe against their
  master `1c774e2` in an Arch Linux ARM container: `upstream-watch.py
  validate` and `check` ("already current", 0.4.21), metadata valid,
  `pinned-sources.sh` and `package-metadata.sh` PASS, `bin/build
  --dry-run` plans the package.
- Submitted: omacom/omarchy-pkgs#2022 from the fork t0d0r/omarchy-pkgs,
  branch `lazygram`, commit `51a84c0` with only
  `pkgbuilds/lazygram/PKGBUILD` and `.omarchy/package.json`; the PR body
  is PR.md without its title line.

## Scorecard result, first run

- 10: Dangerous-Workflow, Token-Permissions, Binary-Artifacts, License.
- 7: Vulnerabilities, three OSV advisories in required modules:
  GO-2026-5932 (golang.org/x/crypto/openpgp, no fix), GO-2026-6860 and
  GO-2026-6866 (golang.org/x/net/html, fixed in 0.62.0; go.mod has
  0.61.0, indirect). govulncheck v1.1.4: lazygram's code calls none of
  them.
- 6: Contributors. 3: Pinned-Dependencies (linux.yml and macos.yml use
  tags, not SHAs).
- 0: Maintained (the repository is younger than 90 days), Code-Review,
  SAST, Branch-Protection, Security-Policy (no SECURITY.md),
  Signed-Releases, Dependency-Update-Tool, Fuzzing, CII-Best-Practices.
- Not scored: Packaging, CI-Tests.

## Decisions

- No personal access token for Branch-Protection: one more secret for
  one check; the default token reads repository rules.
- No `fail_on_publish_error`: it arrives in scorecard-action v2.6.0,
  v2.5.0 is the latest release.
- The omarchy-pkgs commit uses a Conventional Commit subject
  (`feat(lazygram): add lazygram 0.4.21`); the PR title keeps that
  repository's "Add <package>" form.

## Open

- PR #2022 waits for a maintainer's `build-approved` label; about 190
  "Add ..." PRs were open there, some for weeks.
- Raising the score, if wanted: pin linux.yml and macos.yml actions by
  SHA, SECURITY.md, Dependabot for gomod and github-actions, bump
  golang.org/x/net to 0.62.0, a ruleset on `master`, CodeQL for SAST.
- actionlint reports `install-only` as unknown to
  `golangci/golangci-lint-action@v8` in linux.yml and macos.yml (both
  pass in CI); not changed here.
- The stray tag `v0.4.11-1-gd0bdc3f` is still on GitHub.

## Files touched

- .github/workflows/scorecard.yml, README.md, specs/ci.md,
  specs/README.md, packaging/omarchy/PR.md, packaging/omarchy/README.md,
  todo/todo.md, this note

## Verify

- `make check`; `actionlint .github/workflows/scorecard.yml`
- `curl -fsS https://api.scorecard.dev/projects/github.com/t0d0r/lazygram`
  shows the latest published score
- `gh pr view 2022 -R omacom/omarchy-pkgs` for the submission

## Next

- Follow PR #2022: answer review comments; after it merges and publishes,
  put `omarchy pkg add lazygram` first in the README install section and
  check every channel and architecture (packaging/omarchy/README.md,
  step 5).
