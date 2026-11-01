# Using lazygram

The README is the short version; this is the full reference. Inside
lazygram, `/help` prints the commands and `/keys` the keyboard shortcuts.

## Linking an account

`lazygram login` works like nchat and the official desktop apps: it shows a
QR code in the terminal; on the phone open Telegram, Settings -> Devices ->
Link Desktop Device, and scan it. No phone number or SMS code is typed. If the
account has two-step verification, the password is asked once, hidden. Use
`-ascii` if the block characters do not render in your terminal.

Started without a linked account, lazygram exits and says how to link one or
how to start the demo. No environment variables are required. Everything
lazygram keeps lives in one state directory, `~/.local/state/lazygram` (or
`-data DIR`), which is private to you and never committed; one lazygram at a
time may use it. `make run`, `make demo`, `make login` and `make smoke` use
`.state/` inside the project.

Telegram's protocol requires every client to identify with an application
id and hash. Like nchat, lazygram ships one in its source, so a brew or
source build logs in with nothing to configure. If you prefer your own
(forks must: a registration shared by many clients can be throttled or
banned by Telegram), get it from https://my.telegram.org under "API
development tools" and give it to lazygram in one of three ways; each wins
over the built-in one:

- `TG_API_ID` and `TG_API_HASH` in the environment, for example from a
  direnv `.envrc` in the project directory (git-ignored). `lazygram login`
  takes them from there, stores them in the state directory and never asks.
  nchat's names `TG_APIID` / `TG_APIHASH` are accepted too.
- Type them at the first `lazygram login`; they are stored the same way.
- Bake them into the binary like nchat: `make embed` writes the git-ignored
  `cmd/lazygram/app_local.go` from `TG_API_ID`/`TG_API_HASH` in the
  environment (direnv), and every `make build` after that carries them. No
  environment at run time, no prompt.

A stored registration wins over the environment, which wins over a private
embed, which wins over the one shipped in the source; delete
`<state>/telegram/app.json` to switch. Never commit these values: `.envrc`,
`*_local.go` and `.state/` are git-ignored for that reason.

## Windows

It works like BitchX/irssi, down to the banner at start and the parting
one at exit (one of several faces, at random), both rolling in line by
line like a terminal on a slow serial line. Window 0 is the server window:
every chat interleaved by time as `HH:MM #chat <sender> text`, plus `-!-`
notices. A window bar above the input lists every window as `[N:name]`.
Open windows, the current one and each window's Enter-sends setting are
remembered across restarts. On exit lazygram prints the session statistics
(time wasted, messages, windows, commands) and, for a release build, a hint
when a newer release exists; `/about` shows version, author and build.
When a chat gets a message (or has unread messages at start) a window opens
for it; the status line shows `[Act: 2,3]` for windows with unread. People
are `@name`, groups and channels are `#name`. lazygram shows what arrives
while it runs; opening a chat's window fetches its last 10 messages, and
`/last N` fetches more.

## Commands

| command                    | action                                        |
|----------------------------|-----------------------------------------------|
| `/list [text]`             | selectable chat list; Enter opens the window; Left/Right pick `[read]` or `[unread]` for the row, Enter applies it; `--show-deleted` includes deleted accounts |
| `/win N`                   | switch to window N (0 is the server window)   |
| `/move N`                  | move this window to position N                |
| `/win @user`, `/win #chan` | open or switch to that chat's window          |
| `/win`, `/win list`        | selectable list of the open windows; Left/Right pick `[read]`, `[unread]` or `[close]` for the row, Enter applies it |
| `/close [N]`               | close a window                                |
| `/last [N]`                | fetch the last N messages of this chat        |
| `/search [text]`           | page through this chat's local history        |
| `/whois [name]`            | what is known about a chat                    |
| `/stats`                   | client statistics                             |
| `/me <text>`               | send an action line (`* you waves`)           |
| `/msg <@user|#chan> <text>` | send to a chat, opening its window if needed |
| `/send <path> [caption]`   | send a local file (Tab completes the path); pictures go as photos |
| `/edit [text]`, `/delete`  | change or remove your last message in this chat |
| `/names`                   | show chats by nick (`@alias`) or by name (`@Title`) |
| `/press N`                 | press button N under the latest bot message; `/press <msg> N` for an older one |
| `/open [msg]`              | fetch a `[photo]`/`[file]` into the encrypted vault; pictures are shown in the window (`/open out` for the system viewer); `/open <link>` opens an http(s) link in the browser, Tab offers the chat's ten most recent links |
| `/theme [name]`            | colour themes: default, solarized, gruvbox, dracula, nord, monokai, tokyonight, vim, emacs; for light terminals: light, solarized-light, gruvbox-light |
| `/about`                   | version, author, build and libraries          |
| `/fx`                      | running lights over links and buttons on/off  |
| `/mouse`                   | mouse reporting on/off (off: the terminal's own drag selects; or hold Shift/Option while dragging) |
| `/clip [mode]`             | where a mouse selection is copied: `auto` (terminal and system tool), `osc52`, `tool`, `off`; no argument shows the mode (see [clipboard.md](clipboard.md)) |
| `/read-receipts [on\|off]` | read receipts: off by default (kept); on tells Telegram a chat was read when you focus its window with unread messages; no argument shows the setting |
| `/mark-as-read`            | one read receipt for this window's chat, now, whatever `/read-receipts` says |
| `/redraw`, `/clear`        | drop notices, help text and pictures from this window; messages stay |
| `/refresh`, `/help`, `/quit` | sync now; print the command list, with every short form, into the server window (it ends by naming `/keys`); leave |
| `/keys`                    | print the keyboard shortcuts into the server window |

## Sending, buttons and pictures

In a chat window, type text and press **Ctrl-X** to send it to that chat,
as typed: a line that starts with `/` goes out as a message too, Ctrl-X
never runs commands. Enter is for commands (press Ctrl-X twice quickly to
make Enter send in that window, and again to switch back; a fast double
Ctrl-X never sends, it only switches; the status line ends with
`ctrl-x: to send` or `enter: to send`). The sent line shows up as `<me>`
once the service has it. Bot messages with buttons show them underneath as
`[1] Yes  [2] Docs (link)  (msg 1234)`; `/press 1` answers the bot and the
edited message replaces the old line, link buttons only print their URL,
reply-keyboard buttons send their label. Photos show as `[photo]`: click
the token or `/open` and the picture is painted inside the window from
memory (nothing is written to disk); `/open out` opens it in the system
viewer instead.

## Keys

| key                 | action                                             |
|---------------------|----------------------------------------------------|
| Tab                 | empty prompt: next window (Shift-Tab previous); else complete like bash: commands, chats, themes, `/open out`, attachment message ids and the chat's ten most recent links, `/win list`, paths after `/send` |
| Ctrl-N / Ctrl-P     | next / previous window (Alt-Right / Alt-Left too)  |
| Alt-0 .. Alt-9      | window 0 .. 9 (Esc then the digit works too)       |
| Up / Down           | input history (commands and sent lines; a recalled line is input again: Ctrl-X sends it, Enter runs it when it starts with `/`) |
| Left / Right        | move the cursor in the input (in `/list` or `/win`: pick the row's action button); Ctrl-A / Ctrl-E start / end; Delete removes under the cursor (as Backspace at the end), Ctrl-W the word before it, Ctrl-K to the end |
| PgUp / PgDn         | scroll a page; Home / End oldest / newest          |
| Ctrl-R              | sync now                                           |
| Ctrl-L              | redraw this window (`/redraw`): notices and pictures go, messages stay |
| F1                  | print the command list into the server window (`/keys` for this table) |
| Esc                 | close the list or pager, else clear the input      |
| Ctrl-X              | send the typed text to this window's chat, as typed (a line starting with `/` is sent, not run; Enter runs commands) |
| Ctrl-X twice        | toggle "Enter sends" for this window (Ctrl-X then adds a line, but still sends a line starting with `/` unless it has a leading space) |
| Ctrl-C              | quit                                               |
| mouse               | click chat names, nicks, window tags, list rows, links and `[photo]` tokens (a running light shows what is under the pointer); wheel scrolls; drag selects text and copies it on release |

## Updating

`lazygram update` upgrades through brew when installed that way,
otherwise replaces the binary with the latest release after checking it
against the release's SHA256SUMS. `lazygram update -check` only reports;
`-yes` skips the question. A build from source is never replaced, and
neither is a binary installed by a package manager (pacman): those update
through the package manager. Running the installer script again also
installs the latest release.
