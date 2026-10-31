# Release

## Versioning

Tags `vX.Y.Z`. `make build` stamps the binary with `git describe` through
`-ldflags -X main.version=...`; a plain build reports `dev`, a build between
tags something like `v0.3.0-2-gabc1234-dirty`. Homebrew passes the version
without the `v`. `lazygram -version` prints it.

## Artifacts

`make dist` cross-builds for `DIST_PLATFORMS` (linux and darwin, amd64 and
arm64; `CGO_ENABLED=0`, pure Go, so static) into
`dist/lazygram-vX.Y.Z-<os>-<arch>.tar.gz`, each holding a directory of the
same name with `lazygram`, `LICENSE` and `README.md`, plus `SHA256SUMS` in
sha256sum format. Tarballs are owned by root:root with no macOS metadata.

Release builds use `-tags release`; the git-ignored
`cmd/lazygram/app_local.go` that `make embed` writes carries
`//go:build !release`, and `make dist` refuses to run if that file would
be compiled in. A published binary therefore never contains a personal
registration (decision: registrations are personal, see
specs/providers.md).

## Publishing

1. `make check`, commit, `git tag -a vX.Y.Z -m "lazygram vX.Y.Z"`. HEAD
   must be exactly at that tag when releasing: `make release` checks
   that `git describe` is `vX.Y.Z` with nothing after it (a commit past
   the tag would otherwise create a release and a tag named like
   `v0.4.11-1-gd0bdc3f`, which `lazygram update` cannot parse).
2. `make dist`, then `make release`: `gh release create` with the tarballs
   and `SHA256SUMS`, notes generated from the commits. HEAD must be at a
   clean tag.
3. Verify: download every asset back and check `SHA256SUMS`.
4. Homebrew: in `t0d0r/homebrew-tap`, update `Formula/lazygram.rb` with the
   tag's source tarball URL and its sha256 (`curl -L .../archive/refs/tags/
   vX.Y.Z.tar.gz | shasum -a 256`), then `brew install --build-from-source
   t0d0r/tap/lazygram` and `brew test lazygram`.
5. Push `main` and the tag.

## Self-update

`lazygram update` (internal/update, after lazyai's): reads the latest
version from the `releases/latest` redirect without following it, compares
it with the built-in version, and installs by the method the running binary
was installed with: `brew upgrade t0d0r/tap/lazygram` when the resolved
executable is under `/Cellar/lazygram/`, otherwise the platform tarball is
downloaded (HTTPS only, 64 MiB cap), checked against `SHA256SUMS`, the
`lazygram` file extracted (regular file only, from the expected directory)
and renamed over the running binary (temp file next to it, so never half
written). `-check` only reports; `-yes` skips the question. A build from
source (`dev` or between tags) is reported but never replaced. The
checksums catch corruption, not tampering; HTTPS to GitHub is the trust
anchor.

## Homebrew formula

`Formula/lazygram.rb` builds from the tag's source tarball with
`go build ./cmd/lazygram` and `-X main.version=#{version}`; it depends on
Go at build time only. Its test checks `-version` and the not-linked exit
(status 2). No registration ships with it.
