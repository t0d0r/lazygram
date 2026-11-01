# 2026-11-01: stable latest installer

Branch `feature/installer-latest` in the worktree
`../lazygram-installer-latest`, from `master` at `e7044f8`, not
committed; nothing pushed, tagged or released.

## Done

- `install.sh` rewritten for one stable GitHub URL that always serves
  the installer of the latest release (specs/release.md, Installer
  script): POSIX sh; the latest version from the `Location` of a HEAD
  request to `releases/latest`, accepted only under this repository's
  `releases/tag/` and only as a strict `vX.Y.Z` (the old glob check let
  `v0.4.11-1-gd0bdc3f` through); `LAZYGRAM_VERSION` pins a release;
  downloads with `--proto '=https' --proto-redir '=https' --tlsv1.2`;
  checksum against the release's `SHA256SUMS`; only the binary
  extracted, a regular file that must report its version, then copied
  and renamed into `$LAZYGRAM_PREFIX/bin`; no sudo.
- `Makefile`: `dist` copies `install.sh` into `dist/` and lists it in
  `SHA256SUMS`; `release` attaches it and refuses without it; `lint`
  runs `sh -n` and `shellcheck -s sh` (skipped when not installed).
- `internal/installer/install_test.go`: the real script, offline, under
  `sh` and `dash`, from a file and from stdin, with a fake `curl`
  serving a fake release: latest lookup, HTTPS-only flags on every call,
  pinned version without lookup, and nine refusals that install nothing.
- Real runs into scratch prefixes, against GitHub, read-only: macOS arm64
  piped into `sh` installed v0.4.19; pinned v0.4.18 installed v0.4.18;
  Arch Linux ARM arm64 and Arch Linux amd64 piped into `dash` installed
  v0.4.19; every installed binary printed its version and every checksum
  matched the published `SHA256SUMS`; the stray tag was refused.
- specs/release.md: the Installer script section and release step 6.
- README: the stable command, the trust and update trade-off, the
  read-and-verify-first steps (prepared; see Next for when it lands).

## Decisions

- A release asset, not a branch file: it changes exactly when a release
  is published, lives next to the tarballs, and cannot be rewritten by a
  push; a raw branch file is cached up to five minutes on another host.
- The script resolves "latest" itself, so the same file works from the
  release, from the repository and pinned with `LAZYGRAM_VERSION`.

## Open

- `https://github.com/t0d0r/lazygram/releases/latest/download/install.sh`
  answers 404 until a release carries the script.
- The todo item for this work lives on `feature/queued-todos`; mark it
  done when both branches are on master and the URL answers 200.
- Optional: GitHub's immutable releases setting on the repository.

## Files touched

- install.sh, Makefile, internal/installer/install_test.go,
  specs/release.md, README.md, this note

## Verify

`make check` (now with `sh -n` and `shellcheck` on install.sh);
`go test -run . ./internal/installer/`; `make dist` lists `install.sh`
in `dist/SHA256SUMS`; `make -n release` shows it attached.

## Released in v0.4.20

The order was kept because the README command only works once a release
carries the script:

1. `feature/queued-todos` (`736f335`) and `feature/installer-latest`
   (`95ca4ef`, README held back) replayed onto `origin/master` at
   `e7044f8` as `7491701` and `c13c04b`; `make check` (with `sh -n` and
   `shellcheck`), `go test -race` on the changed packages and
   `make smoke` green; pushed as a fast-forward; Linux and macOS CI green
   on `c13c04b`.
2. Tag `v0.4.20` (annotated, `a2310cf` -> `c13c04b`), pushed;
   `make dist` and `make release` published four tarballs, `install.sh`
   and `SHA256SUMS`. Read back: remote master and tag at `c13c04b`, not a
   draft or prerelease, marked latest; every asset's digest equals the
   local build, the downloaded `SHA256SUMS` matches `dist/`, `shasum -c`
   passes for all five files, the released `install.sh` is byte-identical
   to `v0.4.20:install.sh`, and the darwin-arm64 and linux-arm64
   binaries print `lazygram v0.4.20`.
3. `https://github.com/t0d0r/lazygram/releases/latest/download/install.sh`
   redirects to `releases/download/v0.4.20/install.sh` and serves the
   released file. `curl -fsSL <that URL> | LAZYGRAM_PREFIX=<scratch> sh`
   installed v0.4.20 on macOS arm64, and with `sh` and with `dash` in
   Arch Linux ARM arm64 and Arch Linux amd64; each installed binary
   reported `lazygram v0.4.20` and each checksum matched.
4. README.md switched to the stable command, with the trust and update
   trade-off and the read-and-verify-first steps, in the commit that
   records this release.

## Next

- Optional: enable GitHub's immutable releases on the repository so a
  published `install.sh` cannot be replaced.
- Homebrew formula and the Arch/Omarchy recipe are still at 0.4.14.
