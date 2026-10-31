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
how to start the demo. No environment variables are required. Everything
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
forks not to reuse theirs). Get yours once from https://my.telegram.org under
"API development tools" and give them to lazygram in one of three ways:

- `TG_API_ID` and `TG_API_HASH` in the environment, for example from a
  direnv `.envrc` in the project directory (git-ignored). `lazygram login`
  takes them from there, stores them in the state directory and never asks.
  nchat's names `TG_APIID` / `TG_APIHASH` are accepted too.
- Type them at the first `lazygram login`; they are stored the same way.
- Bake them into the binary like nchat: `make embed` writes the git-ignored
  `cmd/lazygram/app_local.go` from `TG_API_ID`/`TG_API_HASH` in the
  environment (direnv), and every `make build` after that carries them. No
  environment at run time, no prompt.

A stored registration wins over the environment, which wins over the baked-in
one; delete `<state>/telegram/app.json` to switch. Never commit these values:
`.envrc`, `*_local.go` and `.state/` are git-ignored for that reason.

## Using it

It works like BitchX/irssi. Window 1 is the server window: every chat
interleaved by time as `HH:MM #chat <sender> text`, plus `-!-` notices. A
window bar above the input lists every window as `[N:name]`. Open windows
and the current one are remembered across restarts.
When a chat gets a message (or has unread messages at start) a window opens
for it; the status line shows `[Act: 2,3]` for windows with unread. People
are `@name`, groups and channels are `#name`. lazygram shows what arrives
while it runs; opening a chat's window fetches its last 10 messages, and
`/last N` fetches more.

| command                    | action                                        |
|----------------------------|-----------------------------------------------|
| `/list [text]`             | selectable chat list; Enter opens the window; `--show-deleted` includes deleted accounts |
| `/win N`                   | switch to window N (1 is the server window)   |
| `/win @user`, `/win #chan` | open or switch to that chat's window          |
| `/win`, `/win list`        | selectable list of the open windows           |
| `/close [N]`               | close a window                                |
| `/last [N]`                | fetch the last N messages of this chat        |
| `/search [text]`           | page through this chat's local history        |
| `/whois [name]`            | what is known about a chat                    |
| `/stats`                   | client statistics                             |
| `/me <text>`               | send an action line (`* you waves`)           |
| `/refresh`, `/help`, `/quit` | sync now; print the help into the window; leave |

In a chat window, type text and press **Ctrl-X** to send it to that chat;
Enter is for commands. The sent line shows up as `<me>` once the service
has it.

| key                 | action                                             |
|---------------------|----------------------------------------------------|
| Tab                 | complete a command or a `@`/`#` name; again cycles |
| Ctrl-N / Ctrl-P     | next / previous window (Alt-Right / Alt-Left too)  |
| Alt-1 .. Alt-0      | window 1 .. 10 (Esc then the digit works too)      |
| PgUp / PgDn         | scroll a page; Up / Down a line; Home / End ends   |
| Ctrl-R              | sync now                                           |
| F1                  | print the help into the window                     |
| Esc                 | close the list or pager, else clear the input      |
| Ctrl-X              | send the typed text to this window's chat          |
| Ctrl-C              | quit                                               |

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
internal/store          encrypted SQLite message cache (AES-256-GCM per row)
internal/provider       Provider interface (Name, Chats, Sync)
internal/provider/demo  offline scripted provider
internal/provider/telegram  Telegram connector (gotd/td MTProto), QR login
internal/qrcode         QR code to terminal text (half-block or ASCII)
internal/command        slash-command parser
internal/config         state directory and stored application registration
internal/tui            bubbletea model and rendering
```

The core never imports a connector; adding a service means implementing
`provider.Provider` and one case in `cmd/lazygram`.

## Telegram connector

It lists your 100 most recent dialogs and polls every 10 seconds
with one request, fetching history only for chats whose newest message
changed. Nothing older than the start is fetched unless you open a window
or run `/last`. Short FLOOD_WAIT replies are honoured automatically.
Sending happens only on Ctrl-X in a chat window. Details and limits:
specs/providers.md.

## Security constraints

- No credentials, tokens, session files, phone numbers, user IDs, chat titles
  or real messages in git, specs, session notes or logs. `.gitignore` blocks
  `.env*`, keys, session files and `.state/`.
- The state directory holds `telegram/session.json` (account access: treat it
  like a password), `telegram/app.json` (your application registration) and
  `telegram/cache.sqlite`, the message cache. Directories are 0700, files
  0600.
- The cache is encrypted per row with AES-256-GCM; the key is derived from
  the session's auth key, so the cache is unreadable without `session.json`
  and a new login starts a fresh one. Only message ids, timestamps and
  counts are stored in clear (specs/storage.md).
- Nothing typed at `lazygram login` is echoed or stored except the
  application registration; the session file is the only login artifact.
- Control characters (including ESC) are stripped from sender and text before
  rendering, so remote messages cannot inject terminal escape sequences.
