# Continuous integration

Two GitHub Actions workflows, `.github/workflows/linux.yml` (name
`Linux`, ubuntu-latest) and `macos.yml` (name `macOS`, macos-latest),
run on every pull request and on every push to `master`. Each job checks
the repository out, installs the Go version from `go.mod`
(`actions/setup-go`) and golangci-lint (`golangci/golangci-lint-action`,
install only), then runs `make check` (gofmt, go vet, golangci-lint,
tests) and `make smoke` (the demo, non-interactive, one screen). The
jobs need no secrets and get read-only repository permissions; they
never touch Telegram.

The README shows the two workflow badges and the latest release badge,
after nchat's README. A red badge means `make check` or `make smoke`
fails on that platform at the head of `master`.

## OpenSSF Scorecard

`.github/workflows/scorecard.yml` (name `Scorecard`) runs
`ossf/scorecard-action` on every push to `master`, weekly (Saturday
01:30 UTC, so `Maintained` stays current) and on a branch protection
change. It writes `results.sarif`, keeps it 5 days as the workflow
artifact, uploads it to the repository's code scanning alerts, and
publishes the result to api.scorecard.dev (`publish_results: true`).

- The workflow follows the Scorecard API's rules for published results,
  or the API rejects them: no workflow-level `env` or write permission
  (`permissions: read-all`), the analysis job alone gets
  `security-events: write` (the code scanning upload) and
  `id-token: write` (the OIDC token that proves where the result came
  from), it runs on `ubuntu-latest` with no container, and its steps use
  only `actions/checkout`, `ossf/scorecard-action`,
  `actions/upload-artifact` and `github/codeql-action/upload-sarif`.
- Every action is pinned to a full commit SHA with the version in a
  comment, the versions of ossf/scorecard's own example workflow.
  Decision: pinning is what Scorecard's Pinned-Dependencies check asks
  for, and a moved tag cannot change what runs with `id-token: write`.
- `actions/checkout` runs with `persist-credentials: false`; the job
  needs no secret and uses the default `GITHUB_TOKEN`. Branch-Protection
  is scored only as far as that token can read it (repository rules
  yes, classic branch protection no); decision against a personal
  access token: one more secret for one check.
- Two README badges: the score from api.scorecard.dev, linked to the
  viewer at scorecard.dev, and the workflow's own status. The score
  badge shows no score until the first run on `master` has published.
  A red workflow badge means the analysis or an upload failed; a
  rejected publish is only a warning in the job summary (v2.5.0 has no
  `fail_on_publish_error`).

Releases stay a manual step (`make dist`, `make release`, see
release.md); CI publishes nothing but the Scorecard result.

Security note: the Linux and macOS workflows run only the Makefile
targets on code from the repository, with `contents: read`; a pull
request from a fork cannot reach any secret because there are none, and
the release tarballs are never built or signed by CI. The Scorecard
workflow does not run on pull requests, runs no repository code, and is
the only one with write scopes (code scanning upload, OIDC token), each
limited to its one job.
