# lazygram

A local terminal client for chat services, Telegram first. It shows messages
from all your chats as one chronological scrollback, like a single IRC
channel (BitchX / irssi style), and lets you open the full history of any chat
in a searchable pager. Feature goals are inspired by nchat.

## Quick start

```sh
make build
./lazygram login    # link your Telegram account: scan a QR code with the phone app
./lazygram          # read your chats
./lazygram demo     # or try it with invented data, no account needed
```

Started without a linked account, lazygram exits and says how to link one or
how to start the demo. No environment variables are involved. Everything
lazygram keeps lives in one state directory, `~/.local/state/lazygram` (or
`-data DIR`), which is private to you and never committed. `make run`,
`make demo`, `make login` and `make smoke` use `.state/` inside the project.

### Linking an account

`lazygram login` works like nchat and the official desktop apps: it shows a
QR code in the terminal; on the phone open Telegram, Settings -> Devices ->
Link Desktop Device, and scan it. No phone number or SMS code is typed. If the
account has two-step verification, the password is asked once, hidden. Use
`-ascii` if the block characters do not render in your terminal.

Telegram's protocol requires every client to identify with an application id
and hash. nchat ships its own; lazygram does not (nchat's documentation asks
forks not to reuse theirs). The first `lazygram login` asks for yours once,
from https://my.telegram.org under "API development tools", and stores them
in the state directory. A personal build can embed them instead:
`make build TG_API_ID=... TG_API_HASH=...` (never commit those values).

## Using it

The main view is one feed: `HH:MM #chat <sender> text`, all chats interleaved
by time, with a day separator, a topic bar, a status line and an input line.
Chats are named after their Telegram username, or a slug of their title.

| key              | action                                          |
|------------------|-------------------------------------------------|
| Tab / Shift-Tab  | cycle chats: all, chat1, chat2, ...             |
| PgUp / PgDn      | scroll a page; Up / Down scroll a line          |
| Home / End       | oldest / newest                                 |
| Ctrl-O           | open history for the selected chat              |
| Ctrl-L           | chat list (type to filter, Enter selects)       |
| Ctrl-R           | sync now                                        |
| F1               | help                                            |
| Esc              | close view / clear input                        |
| Ctrl-C           | quit                                            |

Commands: `/help /quit /refresh /chats [text] /select <chat> /all
/filter [text] /history [chat] /search <text>`. Short forms: `/q /r /ls /j /f
/open`. In the history pager, plain typing searches live.

## Development

```sh
make check    # gofmt + go vet + golangci-lint + tests
make smoke    # non-interactive: demo sync once, print one screen, exit
make help     # all targets
```

## Architecture

```
cmd/lazygram            subcommands (login, demo, default), wiring, smoke mode
internal/domain         Chat, Message, Merge, Filter, ResolveChat (pure)
internal/store          JSONL history cache + chats.json (owner-only files)
internal/provider       Provider interface (Name, Chats, Sync)
internal/provider/demo  offline scripted provider
internal/provider/telegram  read-only Telegram connector (gotd/td MTProto), QR login
internal/qrcode         QR code to terminal text (half-block or ASCII)
internal/command        slash-command parser
internal/config         state directory and stored application registration
internal/tui            bubbletea model and rendering
```

The core never imports a connector; adding a service means implementing
`provider.Provider` and one case in `cmd/lazygram`.

## Telegram connector

Read-only: it lists your 100 most recent dialogs and polls every 10 seconds
with one request, fetching history only for chats whose newest message
changed. The first run takes a short backlog per chat. Short FLOOD_WAIT
replies are honoured automatically. Details and limits: specs/providers.md.

## Security constraints

- No credentials, tokens, session files, phone numbers, user IDs, chat titles
  or real messages in git, specs, session notes or logs. `.gitignore` blocks
  `.env*`, keys, session files and `.state/`.
- The state directory holds `telegram/session.json` (account access: treat it
  like a password), `telegram/app.json` (your application registration) and
  the message cache. Directories are 0700, files 0600.
- Nothing typed at `lazygram login` is echoed or stored except the
  application registration; the session file is the only login artifact.
- Control characters (including ESC) are stripped from sender and text before
  rendering, so remote messages cannot inject terminal escape sequences.
