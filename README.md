# lazygram

A local terminal client for chat services, Telegram first. It shows messages
from all configured chats as one chronological scrollback, like a single IRC
channel (BitchX / irssi style), and lets you open the full history of any chat
in a searchable pager. Feature goals are inspired by nchat.

v0.1 ships the complete UI, a local history cache, and a demo provider. A
read-only Telegram connector (gotd/td) is included but untested against the
real service and needs an existing authorized session (see Telegram setup).

## Quick start

```sh
make run      # build and start the TUI with the offline demo provider
make smoke    # non-interactive: sync once, print one screen, exit
make check    # gofmt + go vet + golangci-lint + tests
make help     # all targets
```

No account is needed. Demo data is invented and cached under `.state/`
(git-ignored); `make clean` removes it.

## Using it

The main view is one feed: `HH:MM #chat <sender> text`, all chats interleaved
by time, with a day separator, a topic bar, a status line and an input line.

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

## Architecture

```
cmd/lazygram            wiring: config -> provider -> store -> TUI; -smoke mode
internal/domain         Chat, Message, Merge, Filter, ResolveChat (pure)
internal/store          JSONL history cache + chats.json (owner-only files)
internal/provider       Provider interface (Name, Chats, Sync)
internal/provider/demo  offline scripted provider
internal/provider/telegram  read-only Telegram connector (gotd/td MTProto)
internal/command        slash-command parser
internal/config         environment loading and validation
internal/tui            bubbletea model and rendering
```

The core never imports a connector; adding a service means implementing
`provider.Provider` and one case in `cmd/lazygram`.

## Telegram setup contract

Selected with `LAZYGRAM_PROVIDER=telegram`. Required environment variables:

| variable               | meaning                                          |
|------------------------|--------------------------------------------------|
| `LAZYGRAM_TG_API_ID`   | Telegram application id (positive integer)       |
| `LAZYGRAM_TG_API_HASH` | Telegram application hash                        |
| `LAZYGRAM_TG_SESSION`  | local git-ignored directory for session state    |
| `LAZYGRAM_TG_CHATS`    | optional `alias=chat-ref,alias2=ref2` list       |

Also: `LAZYGRAM_DATA_DIR` (history cache root, default
`~/.local/state/lazygram`). The connector connects only when
`LAZYGRAM_PROVIDER=telegram` is set, reads only the chats in
`LAZYGRAM_TG_CHATS` (`@username` or numeric id), and never logs in by itself:
with no authorized session in `LAZYGRAM_TG_SESSION` it reports an error.
Create the session once with `./lazygram login` (interactive: phone, code,
optional two-step password; answers are never echoed or stored, only the
session file is). Details: specs/providers.md.

## Security constraints

- No credentials, tokens, session files, phone numbers, user IDs, chat titles
  or real messages in git, specs, session notes or logs. `.gitignore` blocks
  `.env*`, keys, session files and `.state/`.
- Credentials are read from the environment only; config errors name the
  variable, never its value.
- Cache files are written mode 0600 in a 0700 directory. They hold message
  text; treat the data directory as private.
- Control characters (including ESC) are stripped from sender and text before
  rendering, so remote messages cannot inject terminal escape sequences.
