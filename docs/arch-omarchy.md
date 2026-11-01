# lazygram on Arch Linux and Omarchy

## Install

Omarchy, once the package is in pkgs.omarchy.org (see packaging/omarchy/
README.md for where that stands):

```sh
omarchy pkg add lazygram      # or Install > Package in the Omarchy menu (Super + Space)
```

Arch Linux, or Omarchy before the package is published, from the recipe
in this repository:

```sh
git clone https://github.com/t0d0r/lazygram
cd lazygram/packaging/omarchy/lazygram
makepkg -si
```

makepkg downloads the release tarball the PKGBUILD names, checks its
sha256, builds with the Go from the repositories and installs the package
with pacman. The package puts `lazygram` in `/usr/bin`, a launcher entry in
`/usr/share/applications`, an icon, the license and the README.

## Launch

- Terminal: `lazygram login` once (QR code, see the README), then
  `lazygram`. `lazygram demo` runs with invented data and no account.
- Omarchy launcher: Super + Space, type `lazygram`. The entry opens the
  default terminal through `xdg-terminal-exec` with the window class
  `org.omarchy.lazygram`, the class Omarchy's own TUI launcher
  (`omarchy-launch-tui`) assigns, so Hyprland styles the window as a
  terminal and `omarchy-launch-or-focus-tui lazygram` finds it. Before an
  account is linked the window stops at the login hint and waits for
  Enter: run `lazygram login` in a terminal first. A normal `/quit` closes
  the window.
- Launchers that honour `TryExec` hide the entry when `xdg-terminal-exec`
  is missing; others list it and the launch fails. Omarchy installs
  `xdg-terminal-exec`.
- A floating window, another name or icon: Install > TUI in the Omarchy
  menu makes a user entry under `~/.local/share/applications` (command
  `lazygram`, style `float`, icon `lazygram`, the one the package
  installs). With the name `lazygram` it has the same desktop-file id as
  the packaged entry and replaces it in the launcher until Remove > TUI
  deletes it; under another name both show. Neither touches the packaged
  files.

## Dependencies and architectures

- Runtime: `glibc` and `hicolor-icon-theme` (the icon's directory). The
  binary is pure Go, built position-independent against the system libc.
  Optional: `xdg-utils` (`xdg-open` for `/open out` and clicked links),
  `xdg-terminal-exec` (the launcher entry), `wl-clipboard` (`wl-copy`,
  the clipboard for mouse selections on Wayland when the terminal does
  not take OSC 52).
- Build: `go` (go.mod names 1.27.3; the package in `extra` matches on
  both architectures; the recipe builds with that Go only, it never
  downloads a toolchain).
- Architectures: `x86_64` and `aarch64`. The recipe builds and its tests
  pass in Arch Linux and Arch Linux ARM containers (packaging/omarchy/
  README.md has the evidence).
- Terminal: 256 colours or true colour; pictures are drawn with half-block
  characters, so no kitty or sixel graphics protocol is needed; a monospace
  font with block characters for the QR code (`lazygram login -ascii`
  otherwise); mouse through the terminal's mouse reporting.

## Troubleshooting

- `lazygram is not linked to a Telegram account yet` (exit status 2):
  run `lazygram login`.
- The launcher window shows only that hint: same thing; link the account
  in a terminal once, then use the launcher.
- `the stored Telegram session is no longer authorized`: the device was
  terminated from the phone. Run `lazygram login` again.
- The QR code is a block of garbage: `lazygram login -ascii`.
- `/open out` or a clicked link opens nothing: install `xdg-utils` and
  give the file type a default application (`xdg-mime`).
- No launcher entry, or nothing happens when it is chosen: install
  `xdg-terminal-exec` (Install > TUI needs it too).
- Colours are off on a light terminal: `/theme light`,
  `/theme solarized-light` or `/theme gruvbox-light`.
- `lazygram update` complains instead of updating: by design, see
  Upgrade.
- Where things are: `~/.local/state/lazygram` (session, encrypted cache,
  settings), owner-only. Nothing under `/etc` or `~/.config`.

## Upgrade

```sh
sudo pacman -Syu             # Omarchy: omarchy-update does this
```

New releases reach pkgs.omarchy.org on their own: the recipe's upstream
watch follows the GitHub releases of this repository, one day after each
release is published.

`lazygram update` on a packaged install refuses with an error that names
pacman (every package from 0.4.15 on; the recipe builds 0.4.20),
`lazygram update -check` still reports whether a newer release exists,
and the exit banner points at the package manager. Never run
`sudo lazygram update`; use `sudo pacman -Syu`.

## Uninstall

```sh
sudo pacman -Rns lazygram    # Omarchy: omarchy pkg drop lazygram, or Remove > Package
```

A user entry made with Install > TUI goes with Remove > TUI. The state
directory stays. Before removing it, unlink the device from the phone
(Telegram: Settings -> Devices), then:

```sh
rm -rf ~/.local/state/lazygram
```

That removes the session, the encrypted message cache and the settings.

## Security note

The package carries no Telegram registration beyond the public one in the
source and no account data; the build uses `-tags release` like the
upstream tarballs. Everything private lives in `~/.local/state/lazygram`,
which pacman never reads or removes. Keep updates to the package manager:
a file replaced under `/usr/bin` by hand is overwritten by the next
upgrade and fails `pacman -Qkk` until then.
