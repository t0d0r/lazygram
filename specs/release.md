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
6. Check that `https://github.com/t0d0r/lazygram/releases/latest/download/install.sh`
   answers 200 and installs the new version into a scratch prefix
   (`LAZYGRAM_PREFIX=<dir> sh`); nothing in the script is bumped.

## Installer script

`install.sh` at the repository root installs a release without Go, brew
or root, and is itself an asset of every release, so one stable GitHub
URL always serves the installer of the release marked latest:

    curl -fsSL https://github.com/t0d0r/lazygram/releases/latest/download/install.sh | sh

GitHub redirects `releases/latest/download/<file>` to that file of the
latest release; drafts and prereleases are never latest, so the script
changes only when a release is published. What it does, in POSIX sh
(`sh -n`, `dash` and `shellcheck -s sh` clean):

1. The version is `LAZYGRAM_VERSION` when set, else the tag named by the
   `Location` of a HEAD request to `releases/latest` (not followed; the
   method `lazygram update` uses, no API token, no rate limit). Only an
   answer under this repository's `releases/tag/` counts, and only a
   strict `vX.Y.Z` is accepted, so a stray tag such as
   `v0.4.11-1-gd0bdc3f` is refused.
2. `lazygram-<version>-<os>-<arch>.tar.gz` and `SHA256SUMS` of that
   release are downloaded with `curl --proto '=https' --proto-redir
   '=https' --tlsv1.2`: HTTPS only, redirects to the asset host included.
3. The tarball must match its line in `SHA256SUMS` (`sha256sum` or
   `shasum`); a mismatch or a missing line installs nothing.
4. Only `<name>/lazygram` is extracted; it must be a regular file and
   report `lazygram <version>` before it is copied next to the target and
   renamed over `$LAZYGRAM_PREFIX/bin/lazygram` (default `~/.local/bin`,
   mode 0755), so a running lazygram keeps its file. No sudo, nothing
   written outside the prefix and a temporary directory removed at exit;
   a note when the bin directory is not in `PATH`.

Linux and macOS, amd64 and arm64; other systems stop with a message.
`make dist` copies `install.sh` into `dist/` and lists it in
`SHA256SUMS`, so a reader who downloads the script can check it against
the release; `make release` attaches it and refuses without it.
`internal/installer` tests the real script offline under `sh` and
`dash`, from a file and from stdin, with a fake `curl` serving a fake
release: the latest lookup, HTTPS-only flags on every call, a pinned
version, and refusals for a stray or malformed version, a redirect to
another host, no answer, an unknown release, a checksum mismatch, a
missing line and a binary that reports another version.

Decision: a stable "latest" URL on GitHub only, as a release asset
rather than a branch file. A release asset changes exactly when a release
is published and lives next to the tarballs it installs; a raw branch
file is cached for up to five minutes, is served from another host and
can be rewritten by any push. Trade-off, stated in the README: whoever
runs the command gets whatever the latest release carries, so a
compromised account or release changes it for everyone at once;
`SHA256SUMS` from the same release catches corruption, not tampering,
and HTTPS to GitHub is the trust anchor, as for `lazygram update`. The
README offers reading the script first and pinning a version with
`LAZYGRAM_VERSION`. GitHub's immutable releases setting, when enabled
on the repository, stops a published asset from being replaced.

The README command switches to this URL only after the first release
that carries `install.sh` answers 200 there; until then it points at the
script by commit.

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

## Arch Linux and Omarchy package

`packaging/omarchy/lazygram/` is the recipe for pkgs.omarchy.org
(omacom/omarchy-pkgs, `pkgbuilds/lazygram/`) and a plain Arch `PKGBUILD`;
`docs/arch-omarchy.md` is the user side, `packaging/omarchy/README.md`
the maintainer side (build and test evidence, submission steps).

- Source: the tag's GitHub source tarball, pinned by sha256; `pkgver` is
  the tag without the `v`. `arch=('x86_64' 'aarch64')`.
- Build: per the Arch Go guidelines, `-buildmode=pie -trimpath
  -mod=readonly -modcacherw`, external linking with the system `LDFLAGS`
  (`CGO_ENABLED=1` said explicitly), `GOPATH` inside `$srcdir`,
  `GOTOOLCHAIN=local` so the build uses the Go that Arch ships and a
  go.mod ahead of it fails instead of downloading a toolchain,
  `-X main.version=$pkgver`, and `-tags release` like `make dist`, so a
  package can never carry a private registration. `check()` runs
  `go test ./...`. `options=('!debug' '!lto')`: a `-trimpath` build
  carries no `$srcdir` paths, so makepkg's debug split would be useless,
  and the Go compiler has no LTO (`-flto` would only reach gcc for the
  final link). Runtime dependencies `glibc` and `hicolor-icon-theme` (the
  icon's theme directory; namcap requires it); `xdg-utils` and
  `xdg-terminal-exec` are optional.
- Installed: `/usr/bin/lazygram`, the license, the README, a launcher
  entry and an SVG icon. The entry runs `xdg-terminal-exec
  --app-id=org.omarchy.lazygram -e sh -c "lazygram || { ...; read -r _; }"`
  with `Terminal=false`: the form of Omarchy's own TUI entries
  (`Docker.desktop`, `omarchy-tui-install`) with the window class
  `omarchy-launch-tui` would assign, so Hyprland's terminal rule
  (`org\.omarchy\..*`) applies and `omarchy-launch-or-focus-tui lazygram`
  finds the window; the wrapper holds the window on a non-zero exit (the
  not-linked hint, an error) and closes it on a normal quit. Decision
  against a shared class such as `TUI.tile`: focus-or-launch and dock
  attribution need a class of its own. `TryExec=xdg-terminal-exec` lets
  launchers that honour it hide the entry where the helper is missing;
  others show it and the launch fails. The entry and icon sit next to the
  recipe (local sources with checksums) until a release tarball carries
  `packaging/`; then the recipe installs them from the tarball instead.
- Omarchy metadata (`.omarchy/package.json`): `source` local; a GitHub
  release watch on `t0d0r/lazygram` with the pattern
  `v(?P<version>[0-9]+(?:\.[0-9]+)*)` (whole tags only, so the stray
  `v0.4.11-1-gd0bdc3f` is ignored), so the repository's bot opens the
  version-bump PR itself and merges it when its CI is green;
  `min_release_age` 24h, so a release reaches Omarchy a day after it is
  published, time to pull a bad one (a maintainer can bypass the hold for
  a deliberate fix); `release_ring` fast: the repository reserves the ring
  for packages that do not care which Arch snapshot they run on and
  enforces it with `bin/check-fast-ring` (`ldd -r` on every channel),
  which omarchy-speedtest, a Go tool built the same way with
  `depends=('glibc')`, passes; a package that fails the check is moved off
  the ring, nothing else changes. Consequence, accepted: a release reaches
  `stable` with no review beyond the upstream author's and the 24 h hold.
- Self-update (from v0.4.15): `update.Detect` reports `Package` for a
  binary under `/usr` outside `/usr/local`; `lazygram update` then
  refuses (error, nothing downloaded, no question) and names the package
  manager, `-check` still reports, and the exit banner's hint points at
  the package manager. Decision: a file pacman owns is never replaced by
  lazygram, so `pacman -Qkk` stays clean and the next upgrade cannot
  silently undo an update. The 0.4.14 package predates this: its
  `lazygram update` stops with `cannot write /usr/bin (rerun with sudo,
  ...)`, and with sudo it would replace the file; the user documentation
  says not to.
- A new release needs: the tag, `make release`, then the watch does the
  rest; a packaging change at the same version bumps `pkgrel`. The first
  submission should carry the latest release (v0.4.16 at the time of
  writing; v0.4.15 and later carry the refusal and `packaging/` in the
  tarball, so the entry and icon are installed from it); until then the
  recipe names 0.4.14 and the documentation states the boundary.
