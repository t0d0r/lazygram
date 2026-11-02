# Continuous integration

Two GitHub Actions workflows, `.github/workflows/linux.yml` (workflow
and job `Linux`, ubuntu-latest) and `macos.yml` (workflow and job
`macOS`, macos-latest),
run on every pull request and on every push to `master`; CodeQL,
Scorecard and Release are described below. Each job checks
the repository out, installs the Go version from `go.mod`
(`actions/setup-go`) and golangci-lint v2.15.0
(`golangci/golangci-lint-action`, install only), then runs `make check` (gofmt, go vet, golangci-lint,
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
- Every action is pinned to a full commit SHA (see Pinned actions),
  first at the versions of ossf/scorecard's own example workflow.
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

## Pinned actions

Every `uses:` in every workflow names a full commit SHA with the version
in a comment (`actions/checkout@<sha> # v7.0.1`). A moved or replaced
tag cannot change what runs, which matters most where a job holds write
scopes. Dependabot (below) proposes the action updates. golangci-lint
itself is pinned to an exact release with the action's `version:` input
(v2.15.0, the same as the local install); Dependabot does not update
that input, so it is bumped by hand in linux.yml and macos.yml
together. The action downloads that release by tag; it does not check
the archive against a hash.
`actions/checkout` always runs with `persist-credentials: false`; no job
pushes with git.

## CodeQL

`.github/workflows/codeql.yml` (name `CodeQL`) analyses the Go code with
GitHub's CodeQL on every pull request, every push to `master` and weekly
(Monday 03:17 UTC): Go from `go.mod`, `build-mode: autobuild`, results
in the repository's code scanning alerts next to Scorecard's. Its one
job has `contents: read` (checkout) and `security-events: write`. Scorecard's SAST
check counts the workflow (7 of 10 while changes land on `master`
without pull requests; 10 needs every recent change to come through a
pull request that CodeQL checked).

## Dependabot

`.github/dependabot.yml` asks for weekly version updates of the Go
modules and of the GitHub Actions, each ecosystem grouped into one pull
request, with a 7-day cooldown before a new version is proposed (time
for a compromised release to be pulled). Commit messages follow the
repository's convention: `build(deps): ...` for modules,
`ci(deps): ...` for actions. `github.com/charmbracelet/x/*` gets patch
updates only: its minor releases follow the charm v2 stack, and the first
one proposed (ansi 0.10.1 -> 0.11.9) broke the build of the cellbuf that
lipgloss v1.1.0 and bubbletea v1.3.10 use; with cellbuf moved along too,
the fuzz targets showed ansi.Truncate and lipgloss.Width disagreeing on
widths (a digit with an emoji variation selector, an Arabic prepend
mark). Decision: those libraries move together with lipgloss and
bubbletea, as one migration to v2, not one Dependabot pull request at a
time. The pull requests run the Linux, macOS
and CodeQL workflows; merging them is a manual decision. Dependabot
alerts and Dependabot security updates are separate repository
settings (Settings -> Advanced Security), both enabled: an advisory
against a dependency opens its own pull request at once, without the
cooldown.

## Repository settings

Kept on GitHub, not in the tree; changed only by the owner.

- Ruleset `master` (the default branch), no bypass actors, not even
  for administrators: no deletion, no force push; changes arrive only
  through a pull request (no approval required: a single maintainer
  cannot approve their own); the `Linux` check must pass on a branch
  that is up to date with `master`. `macOS` is not required, because
  GitHub's macOS runners are sometimes unavailable for a while; a red
  `macOS` run is still read before merging.
- Ruleset `release tags` (`refs/tags/v*`), no bypass actors: a tag
  cannot be deleted or moved once pushed.
- Immutable releases: a published release's assets and tag cannot be
  changed. Releases published before it was enabled (up to v0.4.21)
  stay mutable and can be deleted.
- Private vulnerability reporting (SECURITY.md), Dependabot alerts and
  Dependabot security updates: enabled.

Decision: the rules apply to everyone, so a compromised token or a
mistake cannot rewrite `master` or a release tag. The cost is that
every change, notes included, goes through a pull request.

## Release

`.github/workflows/release.yml` (name `Release`) runs on a pushed
`vX.Y.Z` tag and publishes the GitHub release with SLSA build
provenance; the procedure and its decisions are in release.md. It is
the only workflow that writes to the repository (`contents: write`, in
the publish job only).

Security note: the Linux, macOS and CodeQL workflows run only on code
from the repository with a read-only `contents` scope (CodeQL also has
`security-events: write`); a pull request from a fork
cannot reach any secret because there are none. Write scopes exist only
where needed and only on one job each: Scorecard (code scanning upload,
OIDC token for the published result), CodeQL (code scanning upload)
and Release (create the release, sign and store the provenance). The
Scorecard and Release workflows never run on pull requests. The release
build job, which compiles code and downloads modules, has a read-only
token; the job that signs and publishes runs no repository code.
