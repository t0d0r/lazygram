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
line like a terminal on a slow serial line. Under the start banner a few
`-!- hint:` lines say what to try: always `/list` (the chats, @people and
#channels; Enter opens one) and `/win` (the open windows and how to
switch), then two more tips picked at random, different from one start
to the next. Window 0 is the server window:
what every chat brings while lazygram runs, interleaved by time as
`HH:MM #chat <sender> text`, plus `-!-` notices; older messages from the
cache show in each chat's own window. `/feed off` keeps the chat
messages out of window 0, IRC style, so it holds only the notices; the
messages still open and fill their own windows, and `/feed on` brings
this session's back. A window bar above the input lists every window as `[N:name]`.
Open windows, the current one and each window's Enter-sends setting are
remembered across restarts. On exit lazygram prints the session statistics
(time wasted, messages, windows, commands) and, for a release build, a hint
when a newer release exists; `/about` shows version, author and build.
When a chat gets a message (or has unread messages at start) a window opens
for it; the status line shows `[Act: 2,3]` for windows with unread. People
are `@name`, groups and channels are `#name`. lazygram shows what arrives
while it runs; opening a chat's window fetches its last 10 messages (100
in a group whose cached messages still lack their senders' usernames), and
`/last N` fetches more.

## Commands

| command                    | action                                        |
|----------------------------|-----------------------------------------------|
| `/list [text]`             | selectable chat list; Enter opens the window; Left/Right pick `[read]` or `[unread]` for the row, Enter applies it and tells Telegram once (a read receipt, or the "marked as unread" flag), whatever `/read-receipts` says; `--show-deleted` includes deleted accounts |
| `/win N`                   | switch to window N (0 is the server window)   |
| `/win last`                | back to the window you were in before this one; again goes back, so it toggles between two windows (Tab twice fast on an empty prompt does the same); a chat named last is `/win @last` |
| `/move N`                  | move this window to position N                |
| `/win @user`, `/win #chan` | open or switch to that chat's window          |
| `/win`, `/win list`        | selectable list of the open windows; Left/Right pick `[read]`, `[unread]` or `[close]` for the row, Enter applies it (`[read]` and `[unread]` tell Telegram once, as in `/list`) |
| `/close [N]`               | close a window                                |
| `/last [N]`                | fetch the last N messages of this chat        |
| `/search [text]`           | page through this chat's local history        |
| `/whois [name]`            | what is known about a chat; in a group or channel window, about one of its members (Tab offers only its `@nicks`) |
| `/stats`                   | client statistics                             |
| `/me <text>`               | send an action line (`* you waves`)           |
| `/msg <@user|#chan> <text>` | send to a chat, opening its window if needed; the chat may be named by its title as `/names` shows it, spaces and all, in Cyrillic or Latin (`/msg @Ivan Petrov hi` sends `hi`) |
| `/send <path> [caption]`   | send a local file (Tab completes the path); pictures go as photos |
| `/edit [text]`, `/delete`  | change or remove your last message in this chat |
| `/names`                   | show chats by nick (`@alias`) or by name (`@Title`); commands and Tab accept either |
| `/press N`                 | press button N under the latest bot message; `/press <msg> N` for an older one |
| `/open [msg]`              | fetch a `[photo]`/`[file]` into the encrypted vault; pictures are shown in the window (`/open out` for the system viewer); `/open <link>` opens an http(s) link in the browser, Tab offers the chat's ten most recent links |
| `/theme [name]`            | colour themes: default, solarized, gruvbox, dracula, nord, monokai, tokyonight, vim, emacs; for light terminals: light, solarized-light, gruvbox-light |
| `/about`                   | version, author, build and libraries          |
| `/fx`                      | running lights over links and buttons on/off  |
| `/mouse`                   | mouse reporting on/off (off: the terminal's own drag selects; or hold Shift/Option while dragging) |
| `/clip [mode]`             | where a mouse selection is copied: `auto` (terminal and system tool), `osc52`, `tool`, `off`; no argument shows the mode (see [clipboard.md](clipboard.md)) |
| `/read-receipts [on\|off]` | read receipts: off by default (kept); on tells Telegram a chat was read when you focus its window with unread messages; no argument shows the setting |
| `/mark-as-read`            | one read receipt for this window's chat, now, whatever `/read-receipts` says; clears its "marked as unread" flag too |
| `/autojoin [on\|off]`      | at start, open a window for every chat with unread messages (on by default, kept); no argument shows the setting |
| `/feed [on\|off]`          | window 0 shows every chat's messages as they arrive (on, the default) or only its notices (off); kept, takes effect at once; no argument shows the setting |
| `/settings [text]`         | a menu of every kept setting (feed, autojoin, read-receipts, names, theme, fx, mouse, clip) with its value, opened in window 0 from any window; Enter or Right sets the selected one's next value, Left its previous, a click on a row its next; each change is kept and acts at once, as its command; typing (or the text) narrows the rows; Esc closes it; also `/set`, `/options` |
| `/redraw`, `/clear`        | drop notices, help text and pictures from this window; messages stay |
| `/refresh`, `/help`, `/quit` | sync now; print the notes on the mouse, themes and windows into the server window, then the command list with every short form last, so the commands are what you see when it ends (it ends by naming `/keys`); leave |
| `/keys`                    | print the keyboard shortcuts into the server window |

Both lists are wrapped to the window's width as they are drawn and
re-flow when the terminal is resized: a description continues under
itself, and in a window too narrow for two columns it goes under the
command or key, indented.

## Sending, buttons and pictures

In a chat window, type text and press **Ctrl-X** to send it to that chat,
as typed: a line that starts with `/` goes out as a message too, Ctrl-X
never runs commands. Enter is for commands (press Ctrl-X twice quickly to
make Enter send in that window, and again to switch back; a fast double
Ctrl-X never sends, it only switches; the status line ends with
`ctrl-x: to send` or `enter: to send`). The sent line shows up as `<me>`
once the service has it.

A message may have several lines: Enter (Ctrl-X when Enter sends) starts
a new line, and a paste keeps its line breaks. Each line is a row of its
own in the input area at the bottom, lined up under the first one after
the prompt, and a long line wraps onto the next row. The input area grows
upward a row at a time, up to 8 rows (at most a third of the screen), and
scrolls with the cursor past that; the messages above give up the rows.
Up and Down move between the rows, keeping the column; on the first or
the last row they walk the input history. Ctrl-A / Ctrl-E go to the start
/ end of the line, Ctrl-K deletes to its end.

Bot messages with buttons show them underneath as
`[1] Yes  [2] Docs (link)  (msg 1234)`; `/press 1` answers the bot and the
edited message replaces the old line, link buttons only print their URL,
reply-keyboard buttons send their label. Photos show as `[photo]`: click
the token or `/open` and the picture is painted inside the window from
memory (nothing is written to disk); `/open out` opens it in the system
viewer instead.

## Edits and deletions

A message edited or deleted elsewhere, on your phone or by the person who
wrote it, changes in lazygram as well: at once while lazygram runs, or
the next time the chat's history is fetched (opening its window,
`/last 50`) for what happened while it was off. A deleted message leaves
the screen, the cache and the attachment vault.

## Members of groups and channels

In a group or channel window, Tab completes its nicks: after `/whois` and
in the text you type (`thanks @al` Tab -> `thanks @alan`). It always fills
in `@username`, so Telegram notifies them, and part of a display name
finds the username too, in Cyrillic or Latin (`hi iv` -> `hi @ivan_p` for
Иван Петров). `/win`, `/msg` and the other commands keep completing every
chat. People without a Telegram username are not offered in text, because
a message cannot @-mention them; after `/whois` Tab offers them by name.
`/whois alan` there describes the member: their username, how many of the
known messages are theirs and mention them, and their private chat if you
have one.

The members are the group's member list, which lazygram asks Telegram
for when the window first opens (`-!- member list: N members`), plus the
people seen writing and the `@usernames` mentioned, this session or in
the cached messages loaded at start. When the admins hid the member list,
and in broadcast channels, whose subscribers only admins see, only the
people seen writing or mentioned are known; `/last 50` fetches more
history and with it more names. Messages cached by an older
lazygram have no usernames; they learn them when Telegram sends them
again: opening a group window fetches its last 100 messages while some
lack them, and `/last 100` does the same at any time. A second Tab lists
the candidates separated by commas.

## Keys

| key                 | action                                             |
|---------------------|----------------------------------------------------|
| Tab                 | empty prompt: next window at once (Shift-Tab previous); twice within 400 ms: back to the last window, as `/win last`, and the window the first Tab showed keeps its unread messages and is not reported read; else complete like bash: commands, chats, themes, `/open out`, attachment message ids and the chat's ten most recent links, `/win list` and `/win last`, a command's settings (`/autojoin off`, `/clip tool`), paths after `/send`; in a group or channel window, its `@nicks` after `/whois` and in message text (`@ad` -> `@ada`); names match in Cyrillic or Latin, chats by alias or by title (a title may have spaces), and a chat is filled in as `/names` shows it, so the name on the window bar can be typed back |
| Ctrl-N / Ctrl-P     | next / previous window (Alt-Right / Alt-Left too)  |
| Alt-0 .. Alt-9      | window 0 .. 9 (Esc then the digit works too)       |
| Up / Down           | in a message of several rows: the row above / below, keeping the column; on the first / last row: input history (commands and sent lines; a recalled line is input again: Ctrl-X sends it, Enter runs it when it starts with `/`) |
| Left / Right        | move the cursor in the input (in `/list` or `/win`: pick the row's action button; in `/settings`: the selected setting's previous / next value); Ctrl-A / Ctrl-E start / end of the line; Delete removes under the cursor (as Backspace at the end), Ctrl-W the word before it, Ctrl-K to the end of the line |
| PgUp / PgDn         | scroll a page; Home / End oldest / newest          |
| Ctrl-R              | sync now                                           |
| Ctrl-L              | redraw this window (`/redraw`): notices and pictures go, messages stay |
| F1                  | print the notes and the command list into the server window, as `/help` (`/keys` for this table) |
| Esc                 | close the list, the settings menu or the pager, else clear the input |
| Ctrl-X              | send the typed text to this window's chat, as typed (a line starting with `/` is sent, not run; Enter runs commands) |
| Ctrl-X twice        | toggle "Enter sends" for this window (Ctrl-X then adds a line, but still sends a line starting with `/` unless it has a leading space) |
| Ctrl-C              | quit                                               |
| mouse               | click chat names, nicks, window tags, list rows, links and `[photo]` tokens (a running light shows what is under the pointer); wheel scrolls; drag selects text and copies it on release; double click copies the word under the pointer, triple click the row (on clickable text the first click acts, the next ones only select); Shift-click extends the selection where the terminal passes Shift through; the selection stays highlighted until the next click, or until the text under it changes or a cursor in it moves (see [clipboard.md](clipboard.md)) |

## Updating

`lazygram update` upgrades through brew when installed that way,
otherwise replaces the binary with the latest release after checking it
against the release's SHA256SUMS. `lazygram update -check` only reports;
`-yes` skips the question. A build from source is never replaced, and
neither is a binary installed by a package manager (pacman): those update
through the package manager. Running the installer script again also
installs the latest release.
