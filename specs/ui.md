# UI

Layout, top to bottom: topic bar, content, status line, window bar, input
line. The window bar lists every window as `[N:name]`; the current window
is highlighted and windows with unread messages stand out. The
model is BitchX/irssi: virtual windows, a slash-command input line, Tab
completion, and an activity indicator.

## Windows

- Window 1 is the server window: every chat interleaved by time, plus
  client notices (`-!-`): start-up, chat counts, windows opened or closed,
  sync errors, `/win` output.
- A window opens for a chat when a message arrives for it and no window
  exists, and at start for every chat the service reports as unread
  (deleted accounts never open windows). `/win @user`, `/win #chan` and
  Enter in `/list` open one on demand.
- Windows are numbered from 1 in opening order and renumbered on `/close`.
  Window 1 cannot be closed.
- Switching to a window clears its unread count. The first time a chat
  window is shown, the last 10 messages of the chat are fetched from the
  service (`-!- fetching the last 10 messages`, then
  `-!- fetched N older message(s)`); `/last N` fetches more, up to 100.
- The status line shows `[Act: 2,3]`: the windows with unread messages.
- The layout (which windows are open, in order, and which is current) is
  saved in the encrypted cache whenever a window opens, closes or becomes
  current, and restored at start (`-!- N windows reopened from the last
  session`); the restored current window fetches its recent messages as on
  any first show. Unread counts and scroll positions are not kept; unread
  comes from the service again.

## Names

People are `@alias`, groups and channels are `#alias`. The provider decides
the alias (Telegram: username, else a slug of the title). `/win`, `/last`
and completion accept names with or without the sigil; a sigil restricts
the kind.

## Line format

Server window: `HH:MM #chat <sender> text` with a padded chat column.
Chat window: `HH:MM <sender> text`. Time, chat and sender always share the
first line with the start of the text; continuation lines (wrapped or
from newlines inside the message) start just past the time column, BitchX
style, and empty lines inside a message are dropped. Notices: `HH:MM -!- text`. A message
whose text starts with `* ` is an action line (`/me`) and renders as
`HH:MM * sender text`. Long lines
wrap as described above. A dim separator marks a change of local day.
Chat and sender colors derive from a hash so they are stable. Sender, text,
titles and notices are stripped of control characters before rendering.

## Modes

- window: the current window's scrollback (default).
- list (`/list [--show-deleted|-a] [text]`): one row per chat: its window
  number when open, name, title, unread count, `(deleted)`. Up/Down (or
  Tab/Shift-Tab) move, Enter opens or focuses that chat's window, typing
  narrows the rows, Esc returns. Deleted accounts are hidden unless asked
  for. `/win` (or `/win list`) shows the same kind of list for the open
  windows (number, name, title, unread, `(current)`); Enter focuses.
  Scrolling: the cursor moves down the screen until it reaches the middle,
  then stays there while the list scrolls under it, and moves on to the
  bottom only when the list ends; the same on the way up.
- pager (`/search [text]`): the local history of the current window's chat
  (all chats in window 1) with the full date on every line; typed text is a
  live case-insensitive search over text and sender with matches
  highlighted; topic shows `N of M messages`; Esc returns.
`/help` and F1 print the command and key reference into the current window
as notices; there is no separate help view.

## Commands

| command                       | action                                               |
|-------------------------------|------------------------------------------------------|
| `/help`, `/?`                 | print the help into the current window               |
| `/quit`, `/q`                 | leave                                                |
| `/refresh`, `/r`              | sync now                                             |
| `/list [text]`, `/ls`         | selectable chat list; `--show-deleted` includes deleted accounts |
| `/win`, `/win list`           | selectable list of the open windows; Enter focuses   |
| `/win N`                      | switch to window N                                   |
| `/win @user`, `/win #chan`    | open or switch to that chat's window                 |
| `/close [N]`                  | close window N or the current one                    |
| `/last [N]`                   | fetch the last N messages (default 10, max 100)      |
| `/search [text]`              | pager over the current window's local history        |
| `/whois [name]`               | print what is known about a chat (default: this window's) |
| `/stats`                      | print client statistics                              |
| `/me <text>`                  | send `* text` to this window's chat (shown as `* you text`) |

Short forms: `/w`, `/j`, `/join`, `/query` for `/win`; `/wc`, `/part` for
`/close`; `/history` for `/last`; `/find`, `/log` for `/search`; `/wi`,
`/info` for `/whois`; `/uptime` for `/stats`; `/action` for `/me`. Errors
show in the status line in red; provider errors also land in the server
window.

## Sending

In a chat window, typed text plus Ctrl-X sends it to that chat through
`Provider.Send`; the input clears, the status says "sending", and a sync
is triggered so the line appears as `<me>` as soon as the service has it.
Enter never sends: on plain text it reminds about Ctrl-X and keeps the
text. Ctrl-X refuses in the server window, on empty text, and on text that
starts with `/` (prefix a space to send such text). Send errors go to the
status line and the window.

## Keys

| key                  | action                                            |
|----------------------|---------------------------------------------------|
| Tab                  | complete; again cycles the candidates             |
| Ctrl-N / Ctrl-P      | next / previous window (Alt-Right / Alt-Left too) |
| Alt-1 .. Alt-0       | window 1 .. 10; Esc followed by the digit does the same (the typed input survives) |
| PgUp / PgDn          | scroll a page; Up / Down one line                 |
| Home / End           | oldest / newest                                   |
| Ctrl-R               | sync now                                          |
| F1                   | print the help into the window                    |
| Esc                  | close the list or pager, else clear the input     |
| Ctrl-X               | send the typed text to this window's chat         |
| Ctrl-U / Ctrl-W      | clear the input / delete the last word            |
| Ctrl-C               | quit                                              |

## Completion

Tab completes the last word of the input. A line that is one word starting
with `/` completes command names. Otherwise it completes chat names: the
candidates are every non-deleted chat whose `@name`/`#name` starts with the
word, or whose bare alias starts with it when the word has no sigil (then
the sigil is added). One candidate is inserted with a trailing space; with
several, the first is inserted and further Tabs cycle. Any other key ends
the cycle.

## Scrolling

Scroll is lines above the bottom, per window. At 0 the window follows new
messages. When scrolled back, arriving messages keep the view anchored.

## Status line

`[time] [provider] [N:name] [Act: ...] [M msgs] [sync state] [-K lines] message`.
A background sync runs every 5s (demo) or 10s (telegram) with a 90s
budget (enough for a FLOOD_WAIT retry); polls skip while a sync runs.

## Subcommands and flags

`lazygram` (linked Telegram account), `lazygram demo`, `lazygram login`,
`lazygram update` (specs/release.md).
Flags: `-data DIR` (state directory, default `~/.local/state/lazygram`),
`-smoke`, `-ascii` (login QR code with `#`), `-version`. Flags may come
before or after the subcommand (`lazygram login -ascii`); any other extra
argument is an error. A session file that cannot be used is reported with
the hint to remove it and log in again.

## Non-interactive mode

When stdin or stdout is not a terminal, or with -smoke, lazygram loads
chats, syncs once, prints one 100x24 screen of the server window and exits
0. `login` refuses without a terminal.
