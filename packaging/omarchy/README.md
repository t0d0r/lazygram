# Arch Linux / Omarchy package: maintainer notes

`lazygram/` is the package directory for omacom/omarchy-pkgs
(`pkgbuilds/lazygram/` there) and a plain Arch recipe: `PKGBUILD`,
`.omarchy/package.json` (required by that repository: every recipe has
one), `lazygram.desktop` and `lazygram.svg`. `PR.md` is the pull request
text. The user side is in docs/arch-omarchy.md, the decisions in
specs/release.md.

## Audit: what packaging needed and what was missing

Checked on 2026-10-31 against the ArchWiki (PKGBUILD, Go package
guidelines), the omarchy-pkgs README and docs, Omarchy's own TUI launcher
scripts and packaged TUI entry, and the omarchy-pkgs recipes of two Go
programs (omarchy-speedtest, cliamp).

| item | state |
|------|-------|
| package name | `lazygram` is free in the Arch repositories and the AUR |
| toolchain | go.mod says 1.27.3; Arch `extra` and Arch Linux ARM both ship go 2:1.27.3-1 |
| build | pure Go, no cgo of its own; builds PIE with external linking per the guidelines; `-tags release` as the upstream release tarballs |
| tests | hermetic; `go test ./...` passes inside makepkg on both architectures |
| runtime | `glibc` only; `xdg-utils` and `xdg-terminal-exec` optional; pictures use half-block characters, no terminal graphics protocol |
| version | `-X main.version=0.4.14`; `lazygram -version` prints it; `update.Parse` accepts it |
| state | `~/.local/state/lazygram`, created by lazygram, owner-only; nothing in `/etc` |
| license | BSD-2-Clause, installed to `/usr/share/licenses/lazygram/` |
| Omarchy metadata | required (`.omarchy/package.json`); GitHub release watch so releases are proposed by the repository bot |
| gap, fixed in code | `lazygram update` would try to replace `/usr/bin/lazygram`; now `update.Detect` reports `Package` for a binary under `/usr` outside `/usr/local`, the update refuses with an error before asking anything, and the exit hint names the package manager. Shipped in v0.4.15 (released 2026-10-31); the 0.4.14 package stops with "cannot write /usr/bin (rerun with sudo, ...)" and with sudo would replace the file; the user documentation says not to |
| gap, fixed | no launcher entry or icon; added in the form of Omarchy's own TUI entries (`xdg-terminal-exec --app-id=org.omarchy.lazygram -e sh -c "lazygram \|\| { ...; read -r _; }"`, `Terminal=false`, `TryExec=xdg-terminal-exec`): the class `omarchy-launch-tui` would assign, and the window stays open on the not-linked hint instead of flashing shut |
| gap, open | the entry and icon live next to the recipe until a release tarball carries `packaging/`; then install them from the tarball and bump `pkgrel` |
| gap, open | the recipe names 0.4.14; upstream is at v0.4.16, which refuses to self-update and carries `packaging/`: point the recipe at it before submitting (Submit, step 0) |
| gap, open | no man page, no shell completions (optional) |
| gap, open | the stray upstream tag `v0.4.11-1-gd0bdc3f` (ignored by the watch pattern, which matches whole tags) should be deleted |

Decisions and their reasons are in specs/release.md.

## Build and test it here

Both runs use Docker on this machine (x86_64 under emulation, aarch64
native), a throwaway container each, an unprivileged `build` user for
makepkg, nothing from the host but the package directory (read-only):

```sh
# Arch Linux x86_64
docker run --rm --platform linux/amd64 \
  -v "$PWD/packaging/omarchy/lazygram:/pkg:ro" archlinux:base-devel bash -c '
  sed -i "0,/^\[options\]/s//[options]\nDisableSandbox/" /etc/pacman.conf   # no seccomp/landlock in the container
  sed -i "s# usr/share/doc/\*##" /etc/pacman.conf       # the image NoExtracts docs; -Qkk would report them
  pacman -Syu --noconfirm --needed go git namcap desktop-file-utils file
  useradd -m build && echo "build ALL=(ALL) NOPASSWD: ALL" > /etc/sudoers.d/build
  cp -r /pkg /home/build/pkg && chown -R build /home/build/pkg && cd /home/build/pkg
  sudo -u build makepkg -sf --noconfirm && namcap PKGBUILD && namcap ./*.pkg.tar.*
  pacman -U --noconfirm ./*.pkg.tar.* && pacman -Qkk lazygram
  desktop-file-validate /usr/share/applications/lazygram.desktop
  lazygram -version && HOME=/tmp lazygram -data /tmp/st -smoke demo | head -3'

# Arch Linux ARM aarch64: the same with --platform linux/arm64 menci/archlinuxarm:base-devel
```

Evidence from the runs of 2026-10-31 with the final recipe (logs kept
outside the tree):

| check | aarch64 (Arch Linux ARM, native) | x86_64 (archlinux:base-devel, emulated) |
|-------|----------------------------------|------------------------------------------|
| `makepkg -sf` incl. `check()` | 108 s, every Go test green | 203 s, every Go test green |
| package | `lazygram-0.4.14-1-aarch64.pkg.tar.xz`, 5.2 MiB, 20.8 MiB installed | `lazygram-0.4.14-1-x86_64.pkg.tar.zst`, 7.9 MiB, 22.0 MiB installed |
| `namcap PKGBUILD` | clean | clean |
| `namcap <package>` | two contradictory glibc warnings (that image's namcap cannot resolve the libc provider; glibc is the right dependency) | clean (an earlier run's `hicolor-icon-theme` error is what added that dependency) |
| `pacman -U`, `pacman -Qkk` | 17 files, 0 altered | 17 files, 0 altered |
| binary | ELF pie, `NOW PIE`, needs `libc.so.6` only | same |
| `lazygram -version` | `lazygram 0.4.14` | same |
| `desktop-file-validate` | valid | valid |
| the entry's command line, run as the terminal would | prints the not-linked hint, then "Press Enter to close" and waits | same |
| `lazygram -smoke demo` (offline) | prints the demo screen | same |
| start without an account | exit status 2 with the login hint | same |

A first aarch64 run beside the emulated x86_64 build was killed by the
8 GiB Docker VM while compiling `github.com/gotd/td/tg`; build one at a
time, or give the VM more memory.

omarchy-pkgs has validators of its own that run in its CI. To run them
before opening the PR (from a checkout of that repository, on Arch, with
`python`, `jq`, `curl`, `git`, `libarchive`):

```sh
cp -r /path/to/lazygram/packaging/omarchy/lazygram pkgbuilds/lazygram
python helpers/upstream-watch.py validate pkgbuilds/lazygram
python helpers/upstream-watch.py check pkgbuilds/lazygram   # expect: "already current"
bash -c 'source helpers/package-metadata.sh && validate_package_metadata pkgbuilds/lazygram'
tests/pinned-sources.sh
tests/package-metadata.sh
bin/build --package lazygram --dry-run  # the plan; without --dry-run the full build in its builder image
```

Run on 2026-10-31 in an Arch Linux ARM container against omarchy-pkgs
master `dd63dc3` (before `min_release_age` was added to the metadata; the
validator accepts a number with an `s`/`m`/`h`/`d` suffix): `validate`
exit 0; `check` printed `{"status": "skipped", "current": "0.4.14",
"reason": "already current"}`; `validate_package_metadata` printed
`metadata valid`; `pinned-sources.sh` and `package-metadata.sh` PASS;
`bin/build --package lazygram --dry-run` planned `lazygram` (its four
`curl` 404 lines are the channel databases it probes for a published
version; none holds the package yet).

The watch pattern itself was checked here too: `v(?P<version>[0-9]+(?:\.[0-9]+)*)`
matches `v0.4.14` whole and does not match `v0.4.11-1-gd0bdc3f`.

## Manual validation still to do on an Omarchy machine

- Super + Space, type `lazygram`: the entry appears (icon shown), opens
  the default terminal with class `org.omarchy.lazygram` and the terminal
  styling (Hyprland tags `org.omarchy.*` as a terminal); before
  `lazygram login` the window shows the hint and waits for Enter; after a
  normal `/quit` it closes; `omarchy-launch-or-focus-tui lazygram`
  focuses the open window.
- Install > TUI with the name `lazygram` makes a user entry that shadows
  the packaged one in the launcher (same desktop-file name under
  `~/.local/share/applications`); Remove > TUI removes only that one and
  the packaged entry shows again.
- `omarchy pkg add lazygram` after publication; `omarchy-update` carries
  the next version; `lazygram update` (0.4.15 and later) names pacman.
- A real QR login and a session on the packaged binary; nothing in the
  recipe changes that code path, but it has not been run from the package.

## Submit it to omarchy-pkgs

0. Point the recipe at the latest release first. v0.4.16 (released
   2026-10-31) carries the self-update refusal and `packaging/` in its
   tarball: set `pkgver=0.4.16`, install `lazygram.desktop` and
   `lazygram.svg` from `packaging/omarchy/lazygram/` inside the tarball
   instead of the two local sources, add `wl-clipboard` to `optdepends`
   (the clipboard fallback for mouse selections on Wayland), run
   `updpkgsums`, rebuild in the containers, and retitle PR.md. Submitting
   0.4.14 as it is also works; PR.md says what 0.4.14 does and the docs
   state the boundary.
1. Keep this repository's `packaging/omarchy/lazygram/` as the source of
   truth; the PR copies it.
2. Fork and branch:

   ```sh
   gh repo fork omacom/omarchy-pkgs --clone omarchy-pkgs && cd omarchy-pkgs
   git checkout -b lazygram
   mkdir -p pkgbuilds/lazygram
   cp -r /path/to/lazygram/packaging/omarchy/lazygram/. pkgbuilds/lazygram/
   ```

3. Check it with their tooling (previous section), or at least
   `cd pkgbuilds/lazygram && updpkgsums && git diff` (no change expected).
4. Commit only `pkgbuilds/lazygram/` (a PR that touches anything else needs
   a maintainer to merge it by hand), one package per PR:

   ```sh
   git add pkgbuilds/lazygram
   git commit -m "Add lazygram 0.4.14"
   git push -u origin lazygram
   gh pr create -R omacom/omarchy-pkgs --title "Add lazygram 0.4.14: terminal Telegram client (TUI)" \
     --body-file /path/to/lazygram/packaging/omarchy/PR.md
   ```

5. A first PR waits for a maintainer to label it `build-approved`
   ("Awaiting build approval"); the builds then run for x86_64 and
   aarch64 and the PR merges itself when green. It is published to `edge`
   within minutes and, with the fast ring, to `rc` and `stable` in the same
   run. Check:

   ```sh
   for c in edge rc stable; do for a in x86_64 aarch64; do
     printf '%s %s: ' $c $a; curl -fsSL "https://pkgs.omarchy.org/$c/$a/omarchy.db" | bsdtar -tf - | grep '^lazygram-' | head -1 || echo missing
   done; done
   ```

6. Afterwards: put `omarchy pkg add lazygram` first in the README's install
   section. New releases arrive through the watch; a packaging change at
   the same version is a PR that bumps `pkgrel`.

## Security note

The recipe downloads one tarball from GitHub over HTTPS and checks its
sha256; the two local files are hashed too. `go mod download` verifies
every module against go.sum. The build runs as an unprivileged user in
makepkg, the package carries no credentials and the installed program
never writes outside the user's state directory. Do not run the
omarchy-pkgs scripts on a machine you care about without reading them
first; the commands above are meant for a container or a fresh checkout.
