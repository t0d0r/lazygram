# 2026-11-02: supply chain, releases from CI, Scorecard

## Done

- Pinned actions: every `uses:` in linux.yml, macos.yml, codeql.yml,
  release.yml and scorecard.yml names a commit SHA with the version in a
  comment; checkout v7.0.1, setup-go v7.0.0, golangci-lint-action v9.3.0
  (v9 knows `install-only`, which v8 ignored, so the action no longer
  runs the linter a second time); golangci-lint itself pinned to
  v2.15.0, the local version.
- `.github/workflows/codeql.yml`: CodeQL for Go on pull requests, pushes
  to `master` and weekly; `contents: read` and `security-events: write`.
- `.github/dependabot.yml`: weekly Go module and action updates, one
  grouped pull request per ecosystem, 7-day cooldown, Conventional
  Commit prefixes `build(deps)` and `ci(deps)`.
- SECURITY.md: private reports through GitHub (private vulnerability
  reporting enabled on the repository), 7 / 14 / 90 day expectations,
  scope, and how to verify a release.
- golang.org/x/net 0.61.0 -> 0.62.0 (GO-2026-6860, GO-2026-6866);
  `osv-scanner.toml` skips GO-2026-5932 (golang.org/x/crypto/openpgp,
  no fixed version, not imported per govulncheck) until 2027-05-01.
- Repository settings (specs/ci.md, Repository settings): ruleset
  `master` (no deletion, no force push, pull request required, `Linux`
  check required on an up-to-date branch, no bypass); ruleset `release
  tags` for `refs/tags/v*` (no deletion, no move, no bypass); immutable
  releases; Dependabot alerts and security updates; private
  vulnerability reporting.
- The Linux and macOS jobs are named `Linux` and `macOS` (both were
  `check`), so the required check is unambiguous.
- `.github/workflows/release.yml`: a pushed `vX.Y.Z` tag on `master`
  is tested and built with `make dist` in a read-only job; a second job,
  which runs no repository code, checks `SHA256SUMS`, creates one SLSA
  build provenance attestation for all files (`actions/attest` v4.2.2),
  attaches it as `lazygram-vX.Y.Z.intoto.jsonl` and creates the release.
  `make release` now refuses and points at the workflow.
- The stray release and tag `v0.4.11-1-gd0bdc3f` were deleted from
  GitHub (2026-11-02).
- A review of these changes (three dimensions, each finding checked by
  a skeptic) confirmed 15 findings, all fixed here: the documented
  `gh attestation verify` command now pins `--signer-workflow`,
  `--source-ref` and GitHub-hosted runners (`--repo` alone accepts any
  workflow of the repository from any branch); no publishing fallback
  without provenance; golangci-lint pinned; CodeQL's `contents: read`
  explicit; `make fuzz` fails when a package's tests do not build (in
  the fuzzing change); SECURITY.md and README no longer promise `pacman
  -Syu` before the package is published; specs/release.md no longer
  describes `make release` as a step.

## Decisions

- GitHub artifact attestations instead of slsa-github-generator: the
  generator is unmaintained since August 2026 (its README says so and
  points to artifact attestations) and cannot be pinned to a commit.
- Build and attestation in separate jobs; only the job without
  repository code holds `contents: write`, `id-token: write` and
  `attestations: write`.
- The releases up to v0.4.21 carry no provenance and are deleted once
  v0.4.22 is verified; their tags stay for the Homebrew formula, the
  Arch recipe and the history.
- Changes go through pull requests from here on, enforced by the
  `master` ruleset; only `Linux` is a required check because macOS
  runners are sometimes unavailable.

## Open

- Code-Review stays 0 while the same account writes and merges: GitHub
  does not let an author approve their own pull request.
- OpenSSF Best Practices badge (passing): CONTRIBUTING.md and
  CHANGELOG.md first, then the owner registers at bestpractices.dev.
- Maintained becomes scorable when the repository is 90 days old
  (created 2026-10-31).
- The macOS workflow sometimes fails with "The job was not acquired by
  Runner" (GitHub capacity); such a run is re-run (specs/release.md).

## Files touched

- .github/workflows/{linux,macos,codeql,release}.yml,
  .github/dependabot.yml, SECURITY.md, osv-scanner.toml, go.mod, go.sum,
  Makefile, specs/{ci,release,security,README}.md, README.md,
  packaging/omarchy/{PR,README}.md, todo/todo.md, this note

## Verify

- `make check`; `actionlint .github/workflows/*.yml`;
  `go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...`
- after merge: the CodeQL and Scorecard runs on `master` pass;
  `curl -fsS https://api.scorecard.dev/projects/github.com/t0d0r/lazygram`

## Next

- After the pull requests are merged: tag v0.4.22 on `master` and push
  the tag; verify per specs/release.md, Publishing, step 4; Homebrew;
  PKGBUILD and omacom/omarchy-pkgs#2022; then delete the old releases.
