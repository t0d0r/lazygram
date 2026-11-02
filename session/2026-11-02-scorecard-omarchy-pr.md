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

## Decisions

- No personal access token for Branch-Protection: one more secret for
  one check; the default token reads repository rules.
- No `fail_on_publish_error`: it arrives in scorecard-action v2.6.0,
  v2.5.0 is the latest release.

## Open

- actionlint reports `install-only` as unknown to
  `golangci/golangci-lint-action@v8` in linux.yml and macos.yml (both
  pass in CI); not changed here.

## Files touched

- .github/workflows/scorecard.yml, README.md, specs/ci.md,
  specs/README.md, todo/todo.md, this note

## Verify

- `make check`; `actionlint .github/workflows/scorecard.yml`
- after a push to `master`: the Scorecard run passes and
  `https://api.scorecard.dev/projects/github.com/t0d0r/lazygram/badge`
  returns a score

## Next

- Point packaging/omarchy/PR.md and the submit steps at 0.4.21, rerun
  the omarchy-pkgs validators against its current master, open the PR.
