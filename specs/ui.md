# UI

Layout, top to bottom: topic bar, content, status line, window bar, input
line. The window bar lists every window as `[N:name]`; the current window
is highlighted and windows with unread messages stand out. The
model is BitchX/irssi: virtual windows, a slash-command input line, Tab
completion, and an activity indicator.

## Windows

- Window 0 is the server window: every chat interleaved by time, plus
  client notices (`-!-`): start-up, chat counts, windows opened or closed,
  sync errors, `/win` output. It opens with a figlet banner (font slant,
  plain ASCII, coloured top to bottom) and a tagline with the version;
  after the program ends a banner is printed to the terminal, BitchX style
  (font lean on 80 columns or more, smslant below), with the tagline (`by t0d0r`), the
  session statistics (time wasted; messages arrived, sent, buttons
  pressed, files fetched, cached; windows open at exit, opened, chats
  visited, commands run) and, for a release build, a one-line hint when
  GitHub has a newer release (checked within 3 seconds, silently skipped
  offline). Not in smoke mode.
- A window opens for a chat when a message arrives for it and no window
  exists, and at start for every chat the service reports as unread
  (deleted accounts never open windows). `/win @user`, `/win #chan` and
  Enter in `/list` open one on demand.
- The server window is number 0, permanent: it cannot be closed or moved.
  Chat windows are numbered from 1 in opening order, renumbered on
  `/close`, and `/move N` puts the current window at position N (1..last),
  shifting the others; the layout is saved.
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
wrap as described above; a long line that starts with spaces keeps that
indent on every piece, so help columns survive narrow windows. A dim separator marks a change of local day.
Chat and sender colors derive from a hash so they are stable. Sender, text,
titles and notices are stripped of control characters before rendering.

## Modes

- window: the current window's scrollback (default).
- list (`/list [--show-deleted|-a] [text]`): one row per chat: its window
  number when open, name, title, unread count, `(deleted)`. Up/Down (or
  Tab/Shift-Tab) move, Enter opens or focuses that chat's window, typing
  narrows the rows, Esc returns (it also arms the Esc-digit prefix, but never restores the
  filter as a draft). Deleted accounts are hidden unless asked for. Tab completes the typed filter like anywhere else and never moves
  the cursor. `/win` (or `/win list`) shows the same kind of list for the
  open windows (number, name, title, unread, `(current)`); Enter focuses.
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
| `/win N`                      | switch to window N (0 is the server window)          |
| `/move N`                     | move this window to position N                       |
| `/win @user`, `/win #chan`    | open or switch to that chat's window                 |
| `/close [N]`                  | close window N or the current one                    |
| `/last [N]`                   | fetch the last N messages (default 10, max 100)      |
| `/search [text]`              | pager over the current window's local history        |
| `/whois [name]`               | print what is known about a chat (default: this window's) |
| `/stats`                      | print client statistics                              |
| `/me <text>`                  | send `* text` to this window's chat (shown as `* you text`) |
| `/msg <@user|#chan> <text>`   | send to that chat, opening and focusing its window if missing |
| `/press N`, `/press <msg> N`  | press a bot button (see Buttons)                     |
| `/open [msg]`                 | fetch an attachment; open it when it is an image     |
| `/theme [name]`               | list the colour themes or switch (see Themes)        |
| `/about`, `/version`          | version, author, repository, license, build, libraries, uptime |
| `/fx`                         | running lights over clickable text on or off (kept) |
| `/mouse`                      | mouse reporting on or off; off lets the terminal select text (kept) |

Short forms: `/w`, `/j`, `/join`, `/query` for `/win`; `/wc`, `/part` for
`/close`; `/history` for `/last`; `/find`, `/log` for `/search`; `/wi`,
`/info` for `/whois`; `/uptime` for `/stats`; `/action` for `/me`. Errors
show in the status line in red; provider errors also land in the server
window.

## Buttons

A message with buttons (a bot's inline keyboard, or a reply keyboard)
shows them under its text, numbered in row order:
`[1] Deploy  [2] Hold  [3] Runbook (link)  (msg 1234)`, where `msg` is the
service's message id. Kinds: callback, link `(link)`, reply-keyboard text
`(reply)`, anything else `(unsupported)`. `/press N` acts on the latest
message with buttons in the current window's chat; `/press <msg> N` on a
given message; both fail in the server window, on a bad number, or when
the message has no buttons any more. Callback: `Provider.Press`, the bot's
answer is a notice (`-!- pressed Deploy: Got it`), and the message is
fetched again so the bot's edit replaces the old line (in the window, the
mirror and the cache). Link: the URL is printed as a notice, never opened.
Reply text: the label is sent like Ctrl-X. Pressing is always explicit.

## Themes

`/theme` prints the colour schemes into the window with the current one
marked; `/theme <name>` (exact or unique prefix, Tab completes) switches
at once and the choice is kept in the cache state across restarts.
Built in: `default`, `solarized`, `gruvbox`, `dracula`, `nord`,
`monokai`, `tokyonight`, `vim` (classic dark Vim: grey comments, cyan,
yellow, magenta, reverse status) and `emacs` (its default faces on a dark
terminal with the grey mode line). A theme sets time stamps, separators,
notices, buttons, the topic bar, the status line and window bar, the
activity and error colours, the nick/chat palette and the banner
gradient; the terminal's own background is never changed. Colours are
hex values, downsampled by lipgloss on 256-colour terminals.

## Mouse

Clickable text looks clickable: links are underlined in the theme's link
colour, attachment tokens bold in the same colour, bot buttons in the
button colour. A running light (a single bright character bouncing back and forth
over the element, moving every 30 ms) plays over the element under the
mouse pointer only: a chat
name, a nick, a link, an attachment token, a button line or a window-bar
tag. Hovering needs the terminal to report pointer motion (all-motion
mouse mode). The animation ticks only while something is hovered, and
rendered messages are cached per width and theme and the hovered message
is simply not cached, so only it re-renders each tick. `/fx` turns the light off or on; the choice is
kept. While lazygram reports the mouse, the terminal's own drag-select
needs a modifier: Shift in most Linux terminals, Option in macOS
Terminal and iTerm2. `/mouse` turns mouse reporting off (drag selects as
usual, clicks do nothing) and on again; the choice is kept and applied at
start. Clicks act on what is under the pointer as of the last render: a chat name
in the server window opens or focuses that chat's window; a nick opens the
chat whose alias or title equals it exactly (otherwise "no chat named ..."); a
`[N:name]` tag in the window bar focuses window N; a row in `/list` or
`/win` opens it; an `http(s)` link in a message or notice is handed to the
system opener (`open`, `xdg-open`, `rundll32`), which starts the default
browser (links always go to the browser; only fetched attachments go to
the image viewer). The wheel scrolls the window three
lines (the cursor in a list). Opening happens only on a click, only for
http and https links with a host; nothing is opened automatically.

## Attachments

A message with a photo or file shows a token before its caption:
`[photo]`, `[image: name]` or `[file: name]`. Nothing is downloaded when
the message arrives. Clicking the token, or `/open` (the latest such
message in the window's chat; `/open <msg>` for a given one), fetches the
attachment through `Provider.Fetch` into memory, seals it into the vault
(specs/storage.md: encrypted file, hashed name; fetched once, served from
the vault afterwards), then writes a plain copy into a private temporary
directory (0700, file 0600). An image (`photo`, or `image`: a document
whose mime type is image/jpeg, png, gif or webp) is handed to the system
opener, which starts the preview app; its copy is named by the content
type, never by the sender, and only raster image extensions are ever
opened. Any other file only gets `-!- decrypted to <path> until exit`
under its (sanitized) sender name and is never opened. The plain copies
are removed when lazygram exits. Media lazygram cannot fetch (polls,
locations, ...) shows as `[media]`. Fetch errors land in the window.

## Sending

In a chat window, typed text plus Ctrl-X sends it to that chat through
`Provider.Send`: the text leaves the input at once but goes out only
after the double Ctrl-X window (600 ms), so that a second Ctrl-X within
that window cancels the send, puts the text back into the input and only
toggles "Enter sends". The status says "sending", and a sync is triggered
so the line appears as `<me>` as soon as the service has it. By default
Enter never sends: on plain text it reminds about Ctrl-X and keeps the
text. Two Ctrl-X within 600 ms toggle "Enter sends" for the current
window (the prompt becomes `[@name]> `); with Enter sends on, Enter sends
at once. Commands still run, and Ctrl-X still sends. The toggle is per
window and is saved with the layout, so it survives restarts; the status
line ends with `ctrl-x: to send` or `enter: to send` in a chat window. Ctrl-X refuses in the server window, on empty text, and on text that
starts with `/` unless the line begins with a space (the space is
dropped, the rest is sent as text). Send errors go to the
status line and the window.

## Keys

| key                  | action                                            |
|----------------------|---------------------------------------------------|
| Tab / Shift-Tab      | empty prompt: next / previous window; else complete like bash (a second Tab lists) |
| Ctrl-X twice         | toggle "Enter sends" for this window              |
| Ctrl-N / Ctrl-P      | next / previous window (Alt-Right / Alt-Left too) |
| Alt-0 .. Alt-9       | window 0 .. 9; Esc followed by the digit does the same (the typed input survives) |
| Up / Down            | walk the input history (commands and sent lines; in the pager they scroll) |
| PgUp / PgDn          | scroll a page (the wheel scrolls three lines)     |
| Home / End           | oldest / newest                                   |
| Ctrl-R               | sync now                                          |
| F1                   | print the help into the window                    |
| Esc                  | close the list or pager, else clear the input     |
| Ctrl-X               | send the typed text to this window's chat         |
| Ctrl-U / Ctrl-W      | clear the input / delete the last word            |
| Ctrl-C               | quit                                              |

## Input history

Every entered line (a command, or text sent with Ctrl-X or Enter) goes
into a per-session history of up to 200 lines, immediate duplicates
collapsed. In a window, Up walks back through it and Down forward; past
the newest entry the line that was being typed comes back. Typing leaves
the walk. In a list Up/Down move the cursor, in the pager they scroll.

## Completion

On an empty prompt in a window, Tab switches to the next window and
Shift-Tab to the previous one. With text typed, Tab completes the last
word of the input like bash. A line that is one
word starting with `/` completes command names; after `/theme ` theme
names; otherwise chat names: every non-deleted chat whose `@name`/`#name`
starts with the word, or whose bare alias starts with it when the word has
no sigil (then the sigil is added). One candidate is filled in with a
trailing space; several are extended to their longest common prefix, and
a second Tab that cannot extend further prints the candidates into the
window as a notice. There is no cycling.

## Scrolling

Scroll is lines above the bottom, per window. At 0 the window follows new
messages. When scrolled back, arriving messages keep the view anchored,
the last content row becomes a marker in the activity colour (`vvv N more
line(s) below - End or a click here jumps to the newest vvv`, clickable)
and the status line shows `[scrolled up N lines]` in the same colour.

## Status line

`time  provider  [N:name]  Act: ...  M msgs  sync state  scrolled up K lines  message  ctrl-x: to send`
(brackets only around the window tag; two spaces between segments)
(the last item says how text is sent in a chat window: `ctrl-x` or `enter`).
Each segment has its own palette colour from the theme (the sync state
changes colour: syncing, synced, never); the window bar stays in one
colour: only the active window is highlighted (reversed) and windows with
new messages show in the activity colour as the indicator.
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
