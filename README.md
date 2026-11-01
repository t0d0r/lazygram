# lazygram

[![Linux](https://github.com/t0d0r/lazygram/actions/workflows/linux.yml/badge.svg)](https://github.com/t0d0r/lazygram/actions/workflows/linux.yml)
[![macOS](https://github.com/t0d0r/lazygram/actions/workflows/macos.yml/badge.svg)](https://github.com/t0d0r/lazygram/actions/workflows/macos.yml)
[![Release](https://img.shields.io/github/v/release/t0d0r/lazygram)](https://github.com/t0d0r/lazygram/releases)

Telegram in your terminal, IRC style (BitchX / irssi): every chat in one
scrollback, a window per chat, QR login, an encrypted local cache.

![lazygram demo: server window](docs/screenshot.svg)

## Install

1. Installer script, Linux and macOS (amd64, arm64): installs the latest
   release into `~/.local/bin`, checked against its `SHA256SUMS`, no sudo.

   ```sh
   curl -fsSL https://github.com/t0d0r/lazygram/releases/latest/download/install.sh | sh
   ```

2. Homebrew:

   ```sh
   brew install t0d0r/tap/lazygram
   ```

3. Arch Linux / Omarchy: `makepkg -si` in `packaging/omarchy/lazygram`
   ([docs/arch-omarchy.md](docs/arch-omarchy.md)).
4. From source: `make install` (needs Go).

Piping into `sh` runs the script unseen and trusts this repository's
latest release; the checksum catches a broken download, not a malicious
release. To read and check it first:

```sh
curl -fsSLO https://github.com/t0d0r/lazygram/releases/latest/download/install.sh
curl -fsSLO https://github.com/t0d0r/lazygram/releases/latest/download/SHA256SUMS
shasum -a 256 -c SHA256SUMS --ignore-missing   # sha256sum on Linux
less install.sh && sh install.sh
```

`LAZYGRAM_VERSION=vX.Y.Z` pins a release, `LAZYGRAM_PREFIX` changes the
prefix. Later: `lazygram update` (or `brew upgrade`, or `pacman -Syu`).

## Quick start

```sh
lazygram login    # scan the QR code: Telegram > Settings > Devices > Link Desktop Device
lazygram          # read your chats
lazygram demo     # try it with invented data, no account needed
```

- Type a message and press **Ctrl-X** to send; **Enter** runs `/commands`.
- `/help` lists the commands, `/keys` the shortcuts; `/list` picks a chat,
  `/win N` or Alt-N switches windows, Tab completes.
- Window 0 shows every chat; a window opens for each chat with news.

## Features

- Windows per chat, `Act:` list, Tab completion, a remembered layout
- `/send` files; `/open` pictures inside the terminal or links in the
  browser; bot buttons with `/press`
- Encrypted cache and attachment vault; read receipts off unless you ask
- Search and names in Cyrillic or its Latin transliteration
- Typing indicators, mouse clicks and drag-to-copy, 12 colour themes
- One process per state directory, self-update checked against
  `SHA256SUMS`

## More

- [docs/usage.md](docs/usage.md): every command and key, account linking,
  your own Telegram API id, updating
- [docs/arch-omarchy.md](docs/arch-omarchy.md),
  [docs/clipboard.md](docs/clipboard.md), [specs/](specs/README.md)

## Development

```sh
make check    # gofmt, vet, golangci-lint, shellcheck, tests
make smoke    # print one demo screen and exit
make help     # every target
```

Releases: specs/release.md. License: BSD 2-Clause, see `LICENSE`.
