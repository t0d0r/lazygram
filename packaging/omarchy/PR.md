Add lazygram 0.4.20: terminal Telegram client (TUI)

## What it is

lazygram is a terminal Telegram client in the BitchX/irssi tradition: one
server window with every chat interleaved, one window per chat, QR login
like the desktop apps, an encrypted local cache, pictures drawn in the
terminal. Pure Go, BSD-2-Clause. Upstream: https://github.com/t0d0r/lazygram
(I am the upstream author).

## The recipe

- `pkgbuilds/lazygram/PKGBUILD`: builds the tagged source tarball (pinned
  by sha256) per the Arch Go guidelines: PIE, trimpath, external linking
  with the system LDFLAGS, `GOTOOLCHAIN=local` (never downloads a
  toolchain), `-tags release` (the upstream release build: no private
  Telegram registration can be compiled in), `go test ./...` in
  `check()`. Runtime dependencies: glibc, hicolor-icon-theme (for the
  icon). Optional: xdg-utils (system viewer for attachments and links),
  xdg-terminal-exec (the launcher entry), wl-clipboard (the clipboard
  for mouse selections on Wayland when the terminal lacks OSC 52).
- `arch=('x86_64' 'aarch64')`.
- `.omarchy/package.json`: a GitHub release watch on `t0d0r/lazygram`
  (`v(?P<version>[0-9]+(?:\.[0-9]+)*)`), `min_release_age` 24h (time to
  pull a bad release), and `release_ring: fast`: a Go binary that links
  libc only, like omarchy-speedtest and cliamp. Drop the fast ring if you
  would rather let it ride the normal promotions.
- The release tarball's `packaging/omarchy/lazygram/lazygram.desktop` and
  `lazygram.svg`, installed by the recipe (nothing else sits beside it): a
  launcher entry in the form of Omarchy's own TUI entries
  (`Exec=xdg-terminal-exec --app-id=org.omarchy.lazygram -e sh -c "..."`,
  `Terminal=false`, `TryExec=xdg-terminal-exec`) with the window class
  `omarchy-launch-tui` would assign, so `omarchy-launch-or-focus-tui
  lazygram` finds it; the small wrapper keeps the window open on a
  non-zero exit (the "not linked yet" hint before `lazygram login`) and
  closes it on a normal quit. Plus a scalable icon.

## How I tested it

- `makepkg -sf` in an `archlinux:base-devel` container (x86_64) and an
  Arch Linux ARM container (aarch64): builds, `check()` passes (all Go
  tests), `namcap` clean on the PKGBUILD and the x86_64 package,
  `pacman -U`, `pacman -Qkk` clean, `desktop-file-validate` clean, the
  binary is PIE with BIND_NOW and needs libc.so.6 only,
  `lazygram -version`, `lazygram -smoke demo` (offline, invented data)
  prints a screen, the not-linked start exits 2 with the login hint and
  the launcher wrapper holds the window for Enter.
- With the package directory copied into a checkout of this repository
  (master at dd63dc3), in an Arch container: `helpers/upstream-watch.py
  validate` and `check` pass (`"reason": "already current"`),
  `validate_package_metadata` passes, `tests/pinned-sources.sh` and
  `tests/package-metadata.sh` pass, `bin/build --package lazygram
  --dry-run` plans the build.
- The watch pattern matches `v0.4.20` whole and ignores the stray tag
  `v0.4.11-1-gd0bdc3f` in the upstream repository (a release mishap, to
  be deleted).
- Not tested by me on Omarchy itself: the launcher entry inside Hyprland
  and `omarchy pkg add lazygram` once published.

## Notes

- No AUR or official Arch package of that name exists.
- Self-update: `lazygram update` refuses to touch a pacman-owned binary
  and points at pacman instead (since upstream 0.4.15).
