# UI

Layout, top to bottom: content, a thin rule, the status line, the window
bar, the input area (one row for one short line of input; it grows upward
one row per row of input, see Input area, and the content gives up the
rows). The rule carries the message at its left (what the
list or pager holds, an error in the error colour, or the last notice,
truncated with "..." when long) and the send hint (`ctrl-x: to send` or
`enter: to send`) at its right end; the status line holds only the
segments, so it never truncates. There is no header: the status line carries the
window tag, and in the list and pager views the rule shows what the view
holds in place of the status message, except right after a change in the
settings menu, whose rule shows what the change said until the next key
(see Settings menu). The window bar lists every window as
`[N:name]`; the current window is highlighted and windows with unread
messages stand out. The
model is BitchX/irssi: virtual windows, a slash-command input line, Tab
completion, and an activity indicator.

## Windows

- Window 0 is the server window: the messages of every chat that arrive
  during this session (everything a sync brings, and what a window's
  fetch brings that is dated after the start), interleaved by time (none
  with `/feed off`, see below), the reactions someone adds to any
  message (see Reactions; none with `/feed off` either), plus client notices (`-!-`): start-up, chat counts, windows opened or closed,
  and every error (sync, history, send, press, fetch, open), whatever
  window was current; errors also show in the status line. It opens with a figlet banner (font banner3
  where the window is at least its 76 columns wide, chunky below that,
  chosen at render time; plain ASCII, coloured top to bottom) and a
  tagline with the version, then `-!- /help lists commands, /keys the
  shortcuts` and the start hints: notices that start with `hint: `, one
  line each. The first two always show, in this order: `/list` (the
  chats, @people and #channels; Enter opens one) and `/win` (the open
  windows; `/win N` or Alt-N switches). Two more follow, drawn at random
  without repeats from a pool of at least 12 others
  (`internal/tui/hints.go`: Tab completion, Ctrl-X and Enter, the double
  Ctrl-X, Ctrl-N and Ctrl-P, `/search`, double and triple click,
  `/theme`, `/autojoin`, `/read-receipts`, `/whois`, `/send`,
  `/open <link>`, Ctrl-L, `/about`, `/mark-as-read`, `/names`, `/last`,
  Up and Down, `/mouse`, PgUp and PgDn, `/move` and `/close`, `/feed
  off`, `/settings`), so the pair
  differs between starts (`Options.Rand` draws them, `rand.IntN` unless
  a test injects one). They are printed once per start, in window 0 only,
  as part of its start-up screen, so they roll in with it when window 0
  is the window shown at start; a chat list,
  `/refresh` or a poll never prints them again, and `/redraw` drops them
  like any notice. Each hint is at most 70 columns, so it fits one row of
  an 80-column window after the stamp, plain ASCII, true of the current
  code, and names only commands the parser knows. A hint is read in
  window 0, which has no chat, so it never speaks of "this" chat or
  window, and one that names Ctrl-X or a command the server window
  refuses (`/send`, `/mark-as-read`, `/last`, `/move`, `/close`, `/edit`,
  `/delete`, `/me`, `/press`) says it works in a chat window. A word such
  as `on` or `off` after a command appears only where that command takes
  it (`/autojoin off`, `/read-receipts on`); `/mouse` takes none, it
  toggles, so its hint says so. Tests check the form, the command names,
  these wordings, and that window 0 does refuse each of those commands
  and Ctrl-X. Decision: a new user's first steps are a
  chat and a window, so those two always show; the rest is discovered
  one tip at a time, two per start, which keeps the start-up screen
  short. Decision: a hint is tried where it is read, so one that fails in
  window 0 (or that turns a kept setting the wrong way, as `/mouse off`
  did when the mouse was already off) teaches the wrong thing; the
  wording names the chat window instead.
  The `/help` line named `/list` too before; it names `/keys` now,
  since `/list` has a hint of its own. At start the content rolls in like on a slow
  serial line: the window that is current at start (the server window,
  or the one restored from the last session) shows its rendered lines
  one by one, each new line at the bottom pushing the earlier ones up, so
  the last frame is the ordinary view and nothing jumps (one line per
  28 ms, several per tick when the scrollback is long so the whole reveal
  takes at most 2 s). Only that first screen rolls in: switching windows
  shows them at once, and smoke mode shows everything. A window whose
  chat had no cached messages rolls in the same way once its first
  history fetch lands (when it is shown).
  The cached backlog loaded at start and older fetched history show only
  in the chat windows and in `/search`, not in window 0. Decision: the
  cached history in window 0 filled the start screen with old messages
  from every chat; it was a side effect of all windows reading one
  in-memory list, never a choice, and an IRC server window shows what
  arrives while connected.
  `/feed off` (kept in the state store under `feed`; absent or
  unreadable means on) keeps the chat messages out of window 0, from
  every chat (people, groups and channels alike), and with them the
  notices of reactions (see Reactions): it shows only its
  notices (the banner and tagline, the start notices and hints, `/help`
  and `/keys`, errors, windows opened and closed and every other `-!-`
  line). Nothing is dropped: the messages are still recorded as this
  session's, open their windows, count as unread and show in the status
  and in `Act` as before, and `/feed on` shows this session's messages
  in window 0 again, those that arrived while it was off included: the
  setting is a filter applied when window 0 is drawn. The chat windows
  and `/search` (in window 0 it still searches every chat) do not
  change. `/feed` with no argument shows the setting (`feed on: window 0
  shows every chat's messages as they arrive; /feed off`, `feed off:
  window 0 shows only notices; chat messages stay in their own windows;
  /feed on`); on and off take effect at once; any other argument is the
  error `/feed takes on or off`. At a start with the feed off, window 0
  says so in one notice right after the start hints, so their order
  stays: `-!- /feed is off: chat messages show only in their own windows
  (/feed on)`. A change keeps window 0's scroll offset within its new
  number of lines (a window scrolled back further than it now reaches
  shows its oldest page). Decision: window 0 interleaves every chat, and
  some prefer the IRC way, where the server window holds only notices
  and each chat stays in its own window; on by default keeps the
  behaviour lazygram always had. `/feed` is also a row of the settings
  menu, which opens in window 0 too (see Settings menu).
  After the program ends a banner is printed to the terminal the same
  way (one line per 56 ms), BitchX style, in one of several faces picked
  at random among those that fit the terminal width: the figlet block
  face, the word in lazygram's own 5-row pixel font (`internal/logo/
  pixel.go`: solid `##`, bracketed `[]`, and `##` standing on a `::`
  shadow), the compact figlet face, or a spaced wordmark over a rule;
  below 49 columns the compact face regardless. Then the tagline (`by t0d0r`), the
  session statistics (time wasted; messages arrived, sent, buttons
  pressed, files fetched, cached; windows open at exit, opened, chats
  visited, commands run) and, for a release build, a one-line hint when
  GitHub has a newer release (checked within 3 seconds, silently skipped
  offline). Not in smoke mode.
- A window opens for a chat when a message arrives for it and no window
  exists, and at start for every chat the service reports as unread,
  people, groups and channels alike (deleted accounts never open
  windows): autojoin, as IRC clients join their channels on connect.
  The start check runs once a session, on the first chat list from the
  service (a chat list cached from an earlier run does not count); when
  that request fails (offline at start, a timeout), each poll asks for
  the chat list again until it comes, so the check runs on connect, not
  at a manual `/refresh` hours later; a
  window restored from the last session takes its chat's unread count
  instead of opening a second one, unless it is the window shown. The
  server window says what came of it: `-!- 12 chats, 3 with unread
  messages: 2:#news (4), 3:@alice, 4:#dev (2)` (cut after four windows
  as `+N more`). `/autojoin off` (kept in the state store under
  `autojoin`; absent or unreadable means on) stops the windows opening
  at the next start, and the notice then ends with `(/autojoin is off:
  no windows opened; /list shows them)`; restored windows still come
  back and still take their counts, and windows still open when a
  message arrives during the session. `/autojoin` with no argument shows
  the setting; it never opens or closes windows in the running session,
  and a change acts at the next start, or, while this start's chat list
  has not come yet, when it does (the status says which).
  A window opened this way fetches its messages when first shown, as any
  window, so many unread chats cost no burst of requests. Decision: the
  check used to run only when no chat list was cached, which is the
  first run only, so unread chats silently stopped opening at start;
  it now keys on the first answer from the service. `/win @user`,
  `/win #chan` and Enter in `/list` open one on demand.
- The server window is number 0, permanent: it cannot be closed or moved.
  Chat windows are numbered from 1 in opening order, renumbered on
  `/close`, and `/move N` puts the current window at position N (1..last),
  shifting the others; the layout is saved.
- Switching to a window clears its unread count. The first time a chat
  window is shown, the last 10 messages of the chat are fetched from the
  service (`-!- fetching the last 10 messages`, then
  `-!- fetched N older message(s)`); `/last N` fetches more, up to 100.
  In a group whose newest 100 known messages include one from a sender
  without an id (cached by a lazygram that kept no ids; own messages and
  unsigned posts need none), the first show fetches the last 100, so
  `Store.Refresh` learns those senders' usernames (see Members).
  Channels keep 10: their posts never carry a sender id.
- The status line shows `[Act: 2,3]`: the windows with unread messages.
  When a sync brings messages from others the status message (on the
  rule) says where they went: `3 new messages in 2:#dev (2), 3:@alice`,
  at most four windows and then `+N more`. This account's own messages
  (sender `me`, sent here or from another device) come back through sync
  too; they show in the windows but are not news: no status, no unread
  count, no window opened for them.
- The layout (which windows are open, in order, and which is current) is
  saved in the encrypted cache whenever a window opens, closes or becomes
  current, and restored at start (`-!- N windows reopened from the last
  session`); the restored current window fetches its recent messages as on
  any first show. Unread counts and scroll positions are not kept; unread
  comes from the service again.

## Names

People are `@alias`, groups and channels are `#alias`. The provider decides
the alias (Telegram: username, else a slug of the title). `/win`, `/last`
and completion accept names with or without the sigil, by alias or by
title (see Names: nick or title); a sigil restricts the kind.

## Members

The members of a group or channel are what the messages lazygram holds
for it show (this session's and the cached backlog loaded at start, the
newest 2000 cached messages across all chats) and, for a group, its
member list from the service. Three things make a member:

- **The member list.** The first time a group's window is shown in a
  session, lazygram asks the service for the group's member list
  (`provider.MemberLister`; Telegram: the whole list of a basic group,
  the 1000 most recently active of a supergroup), once a session, kept in
  memory only. Any member of a group may read it, unless its admins hid
  the members (possible in groups of 100 and more) and the account is no
  admin; then the service refuses (`provider.ErrMembersHidden`), the
  window says `member list hidden here: the members are the people seen
  writing or mentioned`, and the two sources below remain. A broadcast
  channel, whose subscribers only admins see, is not asked and counts as
  hidden without a notice. Another error is said in the window as
  `member list: <error>` and the list is asked again on the next first
  show. A list that came is said as `member list: N members`. When the
  window was closed before the answer, the notice goes to the server
  window and names the group (`member list of #dev: N members`,
  `member list hidden in #dev: ...`).
  A listed person is the member with their id, else the one known only by
  the username they hold (a nick only mentioned, a sender cached without
  an id), else the one sender known by their display name alone; else
  they join with no messages, after those with some. The list is current:
  its name and username win, an empty username included, and anyone else
  who had that username loses it. The account itself and deleted
  accounts are not listed.

- **Writing.** A message carries the sender's own chat id and username
  when the service gives them (`domain.Message.SenderID`, `SenderNick`).
  One id is one member however their name changed: the newest of their
  messages gives the name and the username, an empty username included (a
  dropped username is not offered any more). A message without an id is an
  own message (sender `me`), an unsigned channel post (sender `channel`),
  both left out, or one whose sender the service did not identify (cached
  before ids were kept, a signed channel post), which counts for the one
  member with that display name, else for a member known by the name
  alone. A person whose name is `me` or `channel` is still a member when
  their messages carry an id. A username moves between people: when two
  members wrote with the same one (ignoring case), the one who wrote with
  it last keeps it and the other has none.
- **Being mentioned.** An `@username` in the text of any of the chat's
  messages, own ones and channel posts included (`domain.MentionsIn`: an
  `@` at the start or after a character that is no letter or digit in
  any script, no underscore and none of `/ . @ = &`, then the whole run
  of Latin letters, digits and underscores after it, which must start
  with a letter and be 4 to 32 long (so `@nickя` is none), then the end
  or a character that is no letter, digit or `/`; an `@` in a
  space-separated word that holds `://` or starts with `www.` (ignoring
  case) is part of a link, which Telegram
  reads as one: so `mail@example.com`, `имейл@example.com`,
  `youtube.com/@name` and `@ab` are none). A mention counts for the
  member with that username, ignoring case; otherwise it makes a member
  known by the username alone, with no name, the newest spelling kept.
  People are greeted, thanked and asked by `@username` without writing
  themselves.

The chat's own username is no member: channels sign their posts with it.
A mention of the group's or channel's `Username` (`domain.Chat.Username`,
from the service), or of its alias, is left out in its own window, and a
sender with that username keeps no nick there; in other chats it is a
nick like any.

A cached message learns its sender's id and username when the service
sends it again (a window's first fetch of its last messages, `/last N`, a
sync): `Store.Refresh` replaces the cached copy with the fetched one,
and the copy in memory follows (see Edits and deletions). Until then a sender known
only from older cached messages has no username here. Names and usernames
are stripped of control characters before they reach the input line or a
notice.

The nicks of a chat are its members' usernames. Completion always fills
in `@username`, the mention Telegram notifies, as people write it. A
member without a username is not offered: a message cannot mention them
by `@`, and `@name` would mention whoever holds the username `name` (a
mention by name needs a message entity, which lazygram does not send).
`/whois` still finds them by name.

In a group or channel window's own input (the list filters of `/list` and
`/win list` and the `/search` query complete as before):

- Tab after `/whois ` (and `/wi`, `/info`) completes the whole argument,
  spaces included, to the chat's members: those whose username or display
  name starts with it, ignoring case and a leading `@`, in either script
  (`domain.PrefixMatch`: `iv` completes the username of `Иван Петров`,
  `али` completes `@alice_s`, see Search in either script), as `@nick`,
  and members without a username by display name: `/whois` only looks
  them up and sends nothing. An argument starting with `#` completes chat
  names as before.
- Tab in message text (a line that does not start with `/`, wherever the
  cursor is, and the text of `/me`) completes to the nicks only, the
  members with a username, the same way, with an `@` whether one was typed
  or not. The name being typed may span
  words: it starts at the earliest of the last four word starts whose
  rest of the line (up to the cursor) some member's name or username
  begins with, members without a username included (`hi Grace K` ->
  `hi @gkelly`; `hi Ada L` stays as typed when Ada Lovelace has no
  username, rather than completing `L` to someone else), else at the last
  word. Chat names are not offered there.
- The second Tab lists the candidates separated by commas, with the
  display name after each username when it is known:
  `@alice_s (Alice Smith), @grace_h` in text, and after `/whois` also the
  members without a username: `@alice_s (Alice Smith), @grace_h, ada`.
- `/whois <name>` looks the name up among all the members first, those
  without a username too, in tiers, the first tier with any match
  deciding: exact display name, then exact username (with a leading `@`
  these two swap, so `@alan` is the member with that username and `alan`
  the one shown as alan), then a unique prefix of either, then a unique
  substring of the name, then the same in either script (Cyrillic or
  Latin): a unique prefix of either (what Tab completes), then a unique
  substring of the name. The either-script tiers come last, so they never
  make a plain match ambiguous, and a query of combining marks only gets
  none. It prints `whois @nick in #chat: Display Name`, `whois Name in
  #chat: no username, so no @mention`, or for a nick only mentioned
  `whois @nick in #chat: mentioned, not seen writing`; then how many of
  the known messages here are theirs and when the last was, with how many
  mention them (with none of theirs: how many mention them and when the
  last did, or `no known messages of theirs here`), followed by
  `; in the member list` for a listed member; and their private chat with
  its window. The group and
  the private chat are named as `/names` shows chats. A member with an id
  has the private chat with that id or none (another chat with the same
  name or username is someone else); one without an id has the person
  chat whose alias is the username, else the one person chat titled with
  the name. Several members matching in a tier is an error that asks to
  type more of the name; none falls back to the chats, as everywhere
  else, and `#name` always means a chat.
- `/whois` without a name describes the chat and adds how many members
  are known and from where (the member list and the known messages, or
  the known messages alone, saying when the list is hidden) and how many
  of them have an `@username` (those are what Tab offers in message
  text); `/whois #chat` with a name does not.

Everywhere else nothing changes: `/win`, `/msg`, `/list` and the other
commands complete and resolve every chat, in every window, and so do
`/whois` and message text in the server window and in a person's window.

## Names: nick or title

`/names` toggles how chats are shown everywhere (window bar, status tag,
server window column, lists): by nick, `@alias` / `#alias` (default), or
by name, the sigil followed by the chat's title (`@Alice Smith`,
`#News`); the choice is kept across restarts.

Commands, completion and `/whois` accept the alias and the title, in
either script, in both modes: `/win @alice`, `/win @Alice Smith` and
`/win alice smi` find the same chat (`domain.ResolveChat`, whose tiers
are under Search in either script). `/win` and `/whois` take their whole
argument as the name, spaces and all; `/msg` takes the longest leading
part of its argument that is a chat's name, compared in either script
(see Commands). Decision: `/msg` compares the leading parts in either
script because `domain.ResolveChat` compares names that way. Compared
only as typed, the Latin spelling of a Cyrillic title with spaces
(`/msg @kamen petrov hi`) fell back to its first word, which the exact
tier in either script resolves to another chat (the one titled Kamen),
and sent the rest of the name with the text to it; the Cyrillic
spelling went to the chat titled Kamen Petrov.

Tab offers a chat when the typed word begins its alias or its title as
shown (without control characters), in either script, by nick and by
name alike: with the window bar showing `[4:@Kamen]` for a person whose
alias is the unrelated username `stuckoff`, `/win @ka` offers that chat.
Tab fills in the chat's name as `/names` shows it: `@stuckoff` by nick,
`@Kamen` by name (see Completion). Invariant: every name Tab fills in
resolves through `domain.ResolveChat` to the chat it was offered for, by
nick and by name. When the shown name would not (two chats of a kind
show the same title, the title is another chat's alias or id, or showing
it dropped control characters so it no longer equals the stored title),
that chat is filled in as its `@alias` / `#alias` instead. The second Tab
lists the candidates as they would be filled in; one filled in by its
alias that matched by its title and not by its alias has the title after
it, `@stuckoff (Kamen)`, as the member listing has
`@alice_s (Alice Smith)`. Whenever a listed candidate shows a title (by
name, or in those parentheses) the candidates are separated by `, `, as
titles have spaces; otherwise by two spaces, as before.
Decision: completion offers what the window bar shows, so a name read on
the screen can be typed back. Before, Tab matched and listed only the
alias, so a chat shown by a title unrelated to its username could not be
completed from what the screen said. The fallback to the alias keeps Tab
from filling in a name that `/win` would call ambiguous or send to
another chat.

## Line format

Server window: `HH:MM #chat <sender> text` with a padded chat column.
Chat window: `HH:MM <sender> text`. Time, chat and sender always share the
first line with the start of the text; continuation lines (wrapped or
from newlines inside the message) start just past the time column, BitchX
style, and empty lines inside a message are dropped. Notices: `HH:MM -!- text`. A message
whose text starts with `* ` is an action line (`/me`) and renders as
`HH:MM * sender text`. Long lines
wrap as described above; a long line that starts with spaces keeps that
indent on every piece. A message with reactions ends with their counts,
`[E 3 F 1]` (see Reactions). The `/help` and `/keys` references have a layout
of their own (see `/help` under Modes). A dim separator marks a change of local day.
Chat and sender colors derive from a hash so they are stable. Sender, text,
titles and notices are stripped of control characters before rendering.

## Modes

- window: the current window's scrollback (default).
- list (`/list [--show-deleted|-a] [text]`): one row per chat: its window
  number when open, name, title, unread count, `(deleted)`. Up/Down (or
  Tab/Shift-Tab) move, Enter opens or focuses that chat's window, typing
  narrows the rows (in either script, see Search in either script), Esc returns (it also arms the Esc-digit prefix, but never restores the
  filter as a draft). Deleted accounts are hidden unless asked for. Tab completes the typed filter like anywhere else and never moves
  the cursor. `/win` (or `/win list`) shows the same kind of list for the
  open windows (number, name, title, unread, `(current)`); Enter focuses.
  The selected row ends in a strip of action buttons: `[open] [read]
  [unread]` in `/list`, `[focus] [read] [unread] [close]` in `/win`.
  Left/Right move the choice (clamped at the ends), Enter runs it, a
  click runs the clicked button, and a click on the row itself runs the
  default. The first button is the default, so with nothing chosen Enter
  opens or focuses as before; a new list starts there. Read sets the
  window's unread count to 0 (and, in `/list`, the chat's count in the
  row); unread sets it to at least 1, opening a window for a chat that
  has none without switching to it, so it shows in `Act`; close closes
  that window (window 0 refuses as `/close` does). Read, unread and
  close keep the list open on the same row with the same button, so
  Down, Enter repeats them down the list; the filter stays and the
  action applies to the filtered selection. Read and unread also tell the
  service once per press, whatever `/read-receipts` says and without
  changing it, as an explicit request like `/mark-as-read`: `[read]`
  sends a read receipt for the chat and clears its unread mark, `[unread]`
  sets the unread mark (`Provider.MarkUnread`, Telegram's "mark as
  unread"), so the official apps show the same. The server window tells
  nobody. The status line says `marked #chat read, telling the
  service...`, then `reported #chat read` (or `unread`) or the error
  (Read receipts). The next chats refresh (`/refresh`, a restart) brings
  the service's own count back into the `/list` row, which after a
  report agrees. In a list Left/Right no longer move the
  filter cursor; Backspace, Ctrl-U and typing still edit the filter.
  Decision: buttons on the row rather than letter keys, because letters
  type into the filter, and Left/Right are otherwise idle in a list.
  Scrolling: the cursor moves down the screen until it reaches the middle,
  then stays there while the list scrolls under it, and moves on to the
  bottom only when the list ends; the same on the way up.
- pager (`/search [text]`): the local history of the current window's chat
  (all chats in window 0) with the full date on every line; typed text is a
  live case-insensitive search over text and sender, in either script
  (a Latin query finds Cyrillic text by its transliteration and a
  Cyrillic one finds Latin text, see Search in either script), with matches
  highlighted; the status line shows `N of M messages`; Esc returns.
- settings (`/settings [text]`): window 0's menu of every kept setting,
  a list without a button strip whose Enter, Right and Left change the
  selected setting in place (see Settings menu).
`/help` and F1 focus the server window (window 0) and print there, as
plain lines (no time stamp or marker; command lines start after two
spaces), one logical line every 28 ms like a terminal on a slow serial
line, first the notes on themes, the mouse, selection, windows and
reactions (under the heading `Mouse, themes and windows`), then the command reference
(under `Commands`), last, so that when printing ends the commands are
what the window shows (the window follows the newest line) and the
notes are one scroll up; there is no separate help view. The command
reference lists the slash commands only, each as its `/name` with every
short form the parser accepts spelled out (see Short forms below), and
ends with a line that says to type `/keys` for the keyboard shortcuts.
`/keys` prints the key reference the same way, into the server window,
slowly, as plain lines: the Keys table below (under `Keys`) and nothing
else. F1 prints what `/help` prints, and that reference names `/keys`,
so the keys are one step away from either.

Both references are kept as logical lines and wrapped when drawn, to
the server window's text width, so they use the whole width of any
terminal and re-flow on resize:

- An entry is one line: two spaces, the names (single spaces inside),
  spaces to the description column (column 25; at least two spaces
  after names that reach it), then the whole description. Drawn, the
  description wraps with its continuation rows indented to the column
  where it starts on the first row (a hanging indent).
- When that leaves the description fewer than 20 columns beside the
  names, the names take their own row (wrapped, keeping their two-space
  indent, if the window is narrower than they are) and the description
  goes on the rows under them, indented 4.
- Any other line (a heading, a note) is a paragraph wrapped to the
  width.

No row is wider than the window (at least 10 columns), and no word is
dropped or reordered; only a word wider than its room is broken.
Decision: the references were wrapped by hand at 70 columns, which left
most of a wide terminal empty, could not follow a resize, and broke
oddly in a narrow window; laying them out when drawn uses whatever width
there is. `/keys` printed the table first and about 20 lines of notes
after it, so the notes filled the screen and the table had to be
scrolled back to; the table is what `/keys` is asked for, so it comes
last and the notes are one scroll up. Later the notes moved to `/help`:
they describe using the client (themes, the mouse, selection, windows),
not keys, and `/help` (or F1) is where a new user looks first; `/keys`
is the table alone. `/help` is longer for it, so its commands come
last, as the table does in `/keys`.

## Commands

| command                       | action                                               |
|-------------------------------|------------------------------------------------------|
| `/help`, `/?`                 | focus the server window and print the notes on the mouse, themes and windows there, then the command reference; it ends by naming `/keys` |
| `/keys`, `/shortcuts`         | focus the server window and print the key table there |
| `/redraw`, `/clear`           | drop the window's notices, help text, pictures and banner; the messages stay; a slow print still queued for it is dropped |
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
| `/whois [name]`               | print what is known about a chat (default: this window's); in a group or channel window a name is one of its members first (see Members) |
| `/stats`                      | print client statistics                              |
| `/me <text>`                  | send `* text` to this window's chat (shown as `* you text`) |
| `/msg <@user|#chan> <text>`   | send to that chat, opening and focusing its window if missing; the name may have spaces: it is the longest leading part of the argument, ending at a space, that is (ignoring case, in either script) a chat's name as `/names` shows it, its `@Title` / `#Title` or its title alone (`/msg @Ivan Petrov hi` sends `hi`, by nick and by name, and so does `/msg @ivan petrov hi` when the title is in Cyrillic; with chats titled Ivan and Ivan Petrov, `/msg @Ivan Petrov hi` goes to Ivan Petrov), else the first word, as before (`/msg @alice hi there` sends `hi there`); an argument that is all such a name has no text and gets the usage error, so it never sends its last word |
| `/send <path> [caption]`      | send a local file to this chat (pictures as photos), with an optional caption |
| `/edit [text]`                | edit your last message in this chat; without text it is put in the prompt as `/edit <text>` |
| `/delete`                     | delete your last message in this chat, for everyone  |
| `/press N`, `/press <msg> N`  | press a bot button (see Buttons)                     |
| `/open [msg]`                 | fetch an attachment; a picture is shown in the window |
| `/open out [msg]`             | the same, but a picture goes to the system viewer    |
| `/open <link>`                | hand one `http(s)` link with a host to the browser, as a click does (any window; a link with more words after it, or one without a host, is refused); Tab after `/open` offers the chat's ten most recent distinct links, newest first, beside `out` and the attachment ids |
| `/theme [name]`               | list the colour themes or switch (see Themes)        |
| `/about`, `/version`          | version, author, repository, license, thanks to the people whose feedback shaped it, build, libraries, uptime |
| `/fx`                         | running lights over clickable text on or off (kept) |
| `/mouse`                      | mouse reporting on or off; off lets the terminal select text (kept) |
| `/clip [mode]`                | where a mouse selection is copied: `auto`, `osc52`, `tool`, `off`; no argument shows the mode and the tool (kept) |
| `/read-receipts [on\|off]`, `/receipts` | read receipts to the service: off by default (kept); on reports a chat read when its window with unread messages is focused; no argument shows the setting (see Read receipts) |
| `/mark-as-read`               | one read receipt for this window's chat, now, whatever `/read-receipts` says; an error in the server window and for a chat without a known message |
| `/autojoin [on\|off]`         | at start open a window for every chat with unread messages: on by default, kept; no argument shows the setting (see Windows) |
| `/feed [on\|off]`             | window 0 shows every chat's messages as they arrive and the reactions someone adds (on, the default) or only its notices (off); kept; no argument shows the setting (see Windows, Reactions) |
| `/names`                      | show chats by nick (`@alias`) or by name (`@Title`), kept |
| `/settings [text]`, `/set`, `/options` | focus the server window and open the menu of every kept setting there; Enter or Right sets the selected one's next value, Left its previous; text narrows the rows (see Settings menu) |

Short forms: `/h` for `/help`; `/shortcuts`, `/bindings` for `/keys`;
`/exit` for `/quit`; `/sync` for `/refresh`; `/ls`, `/chats` for `/list`;
`/w`, `/window`, `/j`, `/join`, `/query` for `/win`; `/wc`, `/part` for
`/close`; `/mv` for `/move`; `/history`, `/hist` for `/last`; `/find`,
`/log` for `/search`; `/wi`, `/info` for `/whois`; `/uptime` for
`/stats`; `/action` for `/me`; `/say`, `/tell` for `/msg`; `/file`,
`/upload` for `/send`; `/e` for `/edit`; `/del`, `/rm` for `/delete`;
`/button`, `/btn` for `/press`; `/fetch`, `/download` for `/open`;
`/colors`, `/colours` for `/theme`; `/version` for `/about`; `/lights`,
`/anim` for `/fx`; `/nicks` for `/names`; `/set`, `/options` for
`/settings`. `/help` prints every spelling
the parser accepts, the short forms as `also /x, /y` after their
command, so the printed reference and the parser table never disagree
(a test checks each table entry against the help text and against the
printed output). Errors show in the status line in red; provider errors
also land in the server window.

## Settings menu

`/settings [text]` (also `/set`, `/options`) opens a menu of every kept
setting. It belongs to window 0: typed in any window, `/settings` first
focuses the server window, as `/help` and `/keys` do, and opens the menu
there. While it is open the window bar and the status line show
`[0:server]` as the current window, window 0's content area shows the
menu's rows, the prompt is `settings > ` and the rule explains the keys:
`settings (8): Up/Down, Enter or Right next value, Left previous, Esc
closes, type to narrow` (the number counts the rows shown).

One row per kept setting, in this order: the setting's name (its command
without the slash), its current value and a short description, in
aligned columns. The value column is as wide as the longest value any
row can take, so a change never moves the descriptions.

| row             | values                                   | command          |
|-----------------|------------------------------------------|------------------|
| `feed`          | `on`, `off`                              | `/feed`          |
| `autojoin`      | `on`, `off`                              | `/autojoin`      |
| `read-receipts` | `on`, `off`                              | `/read-receipts` |
| `names`         | `nick`, `name`                           | `/names`         |
| `theme`         | the theme names, sorted as `/theme` lists them | `/theme`   |
| `fx`            | `on`, `off`                              | `/fx`            |
| `mouse`         | `on`, `off`                              | `/mouse`         |
| `clip`          | `auto`, `osc52`, `tool`, `off`           | `/clip`          |

The selected row is highlighted like the other lists; there is no
button strip. Up and Down (and the mouse wheel) select. Enter or Right
sets the selected setting to its next value, Left to its previous one,
wrapping at the ends; for a two-valued setting both toggle. A click on a
row selects it and sets its next value, as Enter does; the later presses
of a double or triple click on it do nothing, since the first one changed
the row under the pointer (see Selection and clipboard). A change
runs the command's own code: the same store key, the same status
message and the same side effects (mouse reporting turned on or off in
the terminal, the theme switched, the rendered lines redrawn after
`names`, `clip` as `/clip <mode>`, so a mode that cannot work here
reports as `/clip` does), so it is kept and takes effect at once. The
menu stays open on the same row with the new value shown, and the rule
shows the change's status message (an error in the error colour) in
place of the keys until the next key. A line starting with `/` typed in
the menu runs as a command on Enter, as in any list; one that leaves the
menu open (`/feed off`, `/clip`, `/theme nord`) shows its status the same
way, and the rows follow, since they always show the current values.

Typed text narrows the rows to those whose name or description holds it,
ignoring case and in either script, as the `/list` filter narrows (see
Search in either script); `/settings <text>` opens the menu narrowed.
The filter stays after a change. With no row left the menu says `no
setting matches`, and Enter, Left and Right change nothing. Tab in the
filter completes the names of the settings; on an empty prompt Tab and
Shift-Tab walk the windows as they do in a window (in `/list` and `/win`
Tab completes the filter instead), and that switch closes the menu.

Esc closes the menu and leaves window 0 current, on its usual view
(messages and notices), as after `/help`; Alt-N or `/win N` goes
elsewhere. Any switch of window closes the menu, because it is window
0's view: Alt-N and Esc N (Alt-0 too: it shows window 0's own view),
Ctrl-N and Ctrl-P (Alt-Right and Alt-Left), Tab and Shift-Tab on the
empty prompt, a click on the window bar, and a command typed in the menu
that focuses a window (`/win N`, `/win @name`, `/msg`, `/help`, `/keys`).
The other lists and the pager (`/list`, `/win`, `/search`) replace it.

Decision: the menu lives in window 0 because settings belong to the
client, not to a chat, and window 0 is where `/help` and `/keys` already
put the client's own text. Decision: values are stepped in place instead
of picked from a button strip, because the theme has a dozen values and
a strip of them would not fit a row; Enter keeps the menu open because
changing a setting is not leaving it. Decision: the rule shows a change's
status message until the next key, because a list's rule otherwise shows
what the list holds, and what the command says (the clipboard tool that
is missing, the theme now in use) is the answer to the change.

## Editing and deleting

## Sending files

`/send <path> [caption]` reads a local file (`~` expands; regular files
only, 64 MiB cap) and sends it to the current window's chat through
`Provider.SendFile`: a name with a raster image extension (jpg, jpeg,
png, gif, webp) goes as a photo, anything else as a document with its
base name and mime type. The status shows `sending photo name (N KiB) to
@chat...` while the upload runs; the message then arrives through sync
like any other and shows as `[photo]`/`[file: name]`. In the server
window `/send` is an error. Nothing is read without the command.

## Edits and deletions made elsewhere

A message edited or deleted after lazygram cached it, on another device,
by the account or by the other side (a "delete for everyone"), changes
in lazygram too:

- While lazygram runs the service reports it (`provider.ChangeSource`;
  Telegram: the update stream). An edit replaces the message's text,
  attachment and buttons in the cache and in every window at once; who
  sent it and when stay as cached. A deletion removes the message from
  the cache, its attachment from the vault, and the line from the
  windows, without a notice: a deleted message is gone. A deletion the
  service reports without a chat (Telegram's private chats and basic
  groups share one id space) removes the id from every chat ever marked
  `Chat.SharedIDs` (kept in the state store under `shared_chats`, so a
  chat no longer among the recent dialogs is reached too), where it is
  unique. An edit of a message lazygram does not hold changes nothing.
  Edits carry their edit time and the later one wins, so updates that
  arrive out of order cannot undo a newer edit. An edit that swaps or
  removes the attachment (another photo or document id, kind or name)
  drops the vault's copy of the old one. A deleted message that was
  counted in its window's unread messages (it arrived while another
  window was shown, and the window has not been read since) takes its
  count back; a message already read takes nothing. The deletions are
  remembered for the session, in the cache and in memory, so a history
  page or sync fetched just before cannot bring a message back, in any
  chat of the shared id space.
- A change made while lazygram was off, one the feed dropped, or one
  lost in a gap the service would not replay (a sleep, a reconnect;
  `updatesTooLong` and `updateChannelTooLong` say so) is healed when the
  chat's history is fetched: a window's first show, `/last N`, and a
  quiet fetch of the shown window's last 10 messages whenever it was last
  fetched more than 2 minutes ago (on each poll and on switching to it),
  at once after a reported gap. The fetched page replaces cached copies
  that differ unless the cached copy carries a later edit
  (`Store.Refresh`), and removes the cached messages inside the page's
  range that the page lacks, and those above it older than the fetch
  (`Store.Prune`), since a page of the newest history has no gaps. A
  quiet fetch says nothing unless it brings messages. Older cached
  messages, outside every page fetched, keep their last known state. A
  quiet fetch can store a message before the sync does; one newer than
  everything the window held is handled as an arrival (status, unread
  count, typing settled), not as older history.

Decision: the polled history only sees new messages, so without the
update stream an edit never arrived and a deleted message stayed
readable in the cache; a message deleted for everyone should not
outlive the deletion in lazygram.

## Reactions

Reactions (`Message.Reactions`, specs/providers.md) show after a
message's text in every window: chat windows, window 0 and the `/search`
pager alike. Two spaces, then `[`, each reaction as its emoji, a space
and its count, the reactions separated by single spaces in the
service's order, then `]`: `<ada> fixed it  [E 3 F 1]`, where E is,
say, the thumbs-up emoji and F the heart, drawn as the service sends
them. They follow the caption of an attachment
(`[photo] the view  [E 3]`) and the last line of the text, before any
button line. A
reaction the account sent too (`Mine`) is drawn in the highlight style
(reverse and bold, as a search hit); the others are plain. All
custom-emoji reactions of a message show as one `*` with their sum, a
paid reaction (a Telegram Star) as the star emoji (specs/providers.md).
An emoji from the service is stripped of control characters and cut to
its first character (a whole emoji sequence; `?` when nothing visible is
left), so it can neither style the screen nor take more than one place.
An emoji and its count stay together on a row: what does not fit the
last row of the text continues on rows of its own, indented like the
text's continuation rows. Selection and copy read the screen as drawn.
The render cache keys a message on its reactions, so a count that
changed is never drawn stale.

Changes reach lazygram two ways and are applied by the same code
(`applyChange`, `Store.React`): the change feed, on which Telegram
pushes a message's reactions only for the account's own messages, and
polls. The cache and every window show the new counts at once; the text
and everything else stay. A sync, a window's first fetch, `/last` and a
heal bring a message's reactions too, through `Store.Refresh`.

Polls (`provider.ReactionPoller`): every 20 seconds
(`reactionPollEvery`; Telegram asks apps for every 15 to 30 seconds),
while the background syncs run, lazygram asks the service about the
messages on screen in the focused window that others sent. On screen
means the rows of the window view as last drawn: not a list, the pager
or the settings menu over it; not the row under the scrolled-up marker
nor the rows still to roll in at start; a notice row is not the message
above it; window 0 with `/feed off` shows no messages. The account's own
messages are left out: Telegram pushes those. A chat window asks about
all of its messages on screen in one call; window 0 asks about those of
one chat per tick, the chats on screen taken in turn (in the order of
their ids, the one after the chat asked last). At most one call per
tick: a tick while a poll is on its way makes none, then or later; a
tick while a sync runs polls when the sync ends, since on Telegram both
need the one client. Nothing is asked when nothing is on screen. A chat
the service refuses (`provider.ErrNotPollable`) is not asked again this
session; when the service asks for a pause (`*provider.WaitError`,
Telegram's FLOOD_WAIT) nothing is polled until it has passed; other
errors are ignored. Nothing is printed about a poll, in the window or
the status line. Decision: only the messages on screen are polled, as
Telegram asks apps to, so a busy account costs one small call per 20
seconds however many chats it has; the rest of the cache catches up
when its window is fetched.

Notices: when a message's reactions change and someone other than the
account added one compared with what lazygram knew of that message (a
count of the others' reactions rose, the account's own taken out by
`Mine`, or a new emoji came), window 0 says so in a `-!-` notice that
starts with the chat's tag, the chat named as `/names` shows chats:

- When the change names who reacted and lazygram has not seen them
  react with that emoji to that message this session:
  `#dev: @grace reacted E to ada "first words"`. At most as many per
  emoji as its count rose, in the service's order (Telegram lists the
  newest first); several in one notice:
  `#dev: @grace reacted E, @alan reacted F to ada "first words"`. A
  reactor is named as `/names` shows their chat when lazygram lists it,
  else as `@` and the name the service gave; reactors the service gives
  no id are told apart by that name; the account itself is never named.
- Otherwise (a broadcast channel, whose reactors are anonymous, or
  nobody new named) the whole current set:
  `#news: E 13 F 4 on wire "first words"`.
- The account's own message: `to your "first words"`, `on your "first
  words"`.
- A message the cache does not hold: `to a message`, `on a message`; a
  pushed change about one, which Telegram sends for the account's own
  messages, says `to your message`, `on your message`. Such a message is
  compared with its last report this session.

The author is the sender as message lines show it. "first words" are
the start of the text (the attachment token when there is no text), on
one line, stripped of control characters, cut at a word to about 30
columns with `...`.

Never told: the first time lazygram learns a message's reactions this
session, a removal or a lowered count, a change identical to what is
known, and the account's own reaction. A message's reactions are
learned from the service when a sync or a fetch brings it (a new
message, a cached copy that changed, and a cached copy that came back
the same, since the service has just said what its reactions are; the
cached history loaded at start does not count), from an edit the
service reports, and from any reported change. A sync, a window's
first fetch, `/last`, a heal and a new message with reactions update
the counts and say nothing; a reaction a poll then finds added to any
message they brought is told. A poll's
first report on a message not learned this session (one cached in an
earlier session and on screen, not fetched since) is applied silently;
a push is compared with the cache whatever was learned. The notices are
chat activity: with `/feed off` window 0 does not show them, as it shows
no chat messages, and since the setting is a filter applied when window
0 is drawn, `/feed on` shows them again; the counts change either way.
They open no window, count as no unread message, and change neither
`Act` nor the status line; `/redraw` drops them like any notice. A
window keeps its newest 500 notices of the chats' activity apart from
its newest 500 other notices, so with `/feed off` the hidden ones never
push out the notices window 0 shows, nor these the hidden ones. What
lazygram remembers of who reacted and of the messages it does not hold
lives in memory for the session, at most 20000 entries each, then
starts over (which can name a reactor twice, never make a notice of a
change that added nothing).

Decision: a first sighting and a removal are silent, so opening a
chat, a fetch or a heal never floods window 0 with reactions made while
lazygram was off, or with every count a cache from before reactions
lacked; window 0 tells what happens while lazygram watches. Decision:
the notices follow `/feed` because they are chat activity, not the
client's own notices. Decision: a reactor is named only when new, and
no more of them than the count rose, because Telegram lists the recent
reactors with every report, old ones included.

Not yet: sending reactions (`/react`), the emoji behind a custom
reaction (it needs a document fetch), and the list of who reacted.

## Editing and deleting

`/edit` and `/delete` act on the newest message in the current window's
chat that this account sent (sender `me`). `/edit` alone puts its text
into the prompt as `/edit <text>` for changing; `/edit <new text>` sends
the edit through `Provider.Edit`, after which the line, the mirror and
the cache show the new text. `/delete` removes it for everyone through
`Provider.Delete`, drops it from the mirror and the cache, and leaves a
notice with the deleted text in the window. Both fail in the server
window and when there is no own message in the chat. Errors (for example
an edit past Telegram's time limit) go to the status rule and window 0.

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
at once and the choice is kept in the cache state across restarts. A
prefix that matches several themes is refused with their names
(`ambiguous: solarized, solarized-light`).
Built in: `default`, `solarized`, `gruvbox`, `dracula`, `nord`,
`monokai`, `tokyonight`, `vim` (classic dark Vim: grey comments, cyan,
yellow, magenta, reverse status) and `emacs` (its default faces on a dark
terminal with the grey mode line); for terminals with a light background
`light` (dark saturated colours, light grey bars), `solarized-light` and
`gruvbox-light` (each scheme's light variant with its official values).
A theme sets time stamps, separators,
notices, buttons, the topic bar, the status line and window bar, the
activity and error colours, the nick/chat palette and the banner
gradient; the terminal's own background is never changed. Colours are
hex values, downsampled by lipgloss on 256-colour terminals.

The status line, the window bar and the prompt row are painted on the
theme's status background across the whole width of the window, the
padding after the last segment included; the prompt itself is in the
dim colour, typed text in the status text colour, the cursor cell and
the current window's tag in the bar are the status colours reversed.
Decision: in every theme the bottom rows read as one block, like an
Emacs mode line and minibuffer; before, every segment ended with a
reset, so the background stopped after the last one and the rest of the
row showed the terminal's own colours.

## Mouse

Clickable text looks clickable: links are underlined in the theme's link
colour, attachment tokens bold in the same colour, bot buttons in the
button colour. A running light (a single bright character bouncing back and forth
over the element; one end-to-end pass takes 400 ms whatever the length,
redrawn every 30 ms) plays over the element under the mouse pointer only: a chat
name, a nick, a link, an attachment token, a button line or a window-bar
tag. Hovering needs the terminal to report pointer motion (all-motion
mouse mode). The animation ticks only while something is hovered, and
rendered messages are cached per width and theme and the hovered message
is simply not cached, so only it re-renders each tick. `/fx` turns the light off or on; the choice is
kept. While lazygram reports the mouse, the terminal's own drag-select
needs a modifier: Shift in most Linux terminals, Option in macOS
Terminal and iTerm2. `/mouse` turns mouse reporting off (drag selects as
usual, clicks do nothing) and on again; the choice is kept and applied at
start. Clicks act on release, on what was under the pointer when the
button went down, as of the last render, and only once in a double or
triple click (see Selection and clipboard): a chat name
in the server window opens or focuses that chat's window; a nick opens the
chat whose alias or title equals it exactly (otherwise "no chat named ..."); a
`[N:name]` tag in the window bar focuses window N; a row in `/list` or
`/win` opens it and one of the action buttons at the selected row's
right runs that action (see Modes); a row of the settings menu sets its
next value (see Settings menu); an `http(s)` link in a message or notice is handed to the
system opener (`open`, `xdg-open`, `rundll32`), which starts the default
browser (links always go to the browser; only fetched attachments go to
the image viewer). The wheel scrolls the window three
lines (the cursor in a list). Opening happens only on a click or an
`/open <link>`, only for http and https links with a host (the opener's
own rule, `opener.IsLink`, for both); nothing is opened automatically.

A link runs from `http://` or `https://` to the next white space (any
Unicode space, the no-break space included), quote (`"` `'` or a
backtick) or angle bracket, then ends as a double-clicked word does
(Selection and clipboard): trailing `.` `,` `;` `:` `!` `?` is dropped,
and a closing bracket at the end is dropped when the link holds more of
it than of its opening bracket, so `Go_(game)` keeps its bracket. A
scheme with nothing left after it (`https://!`) is no link. The link a
click opens, the one Tab offers after `/open` and the text a double
click on it copies are the same (for a link that starts its word apart
from leading brackets and quotes).

### Selection and clipboard

A left-button drag selects text, so the mouse mode costs nothing that
the terminal's own drag gave: the selection is a stream from the cell
where the button went down to the cell under the pointer, in reading
order (the first row from the anchor to its end, whole rows between,
the last row up to and including the pointer's cell), over any screen
row including the status rows and the rows of the input area. While the button is
down the selected cells show in reverse video, and they stay so after
the release until the next press, or until the text under them changes
or a cursor in them moves (see below). On release the text is
put on the clipboard and the status says `copied N characters` (one
row; `copied 1 character` for one) or `copied N lines`, naming the
channels used; a press and release
on the same cell is a click, so a drag that starts on a link selects
instead of opening it, and a drag that moved copies nothing else. The
text is the plain characters of the rendered rows: a character is a
grapheme cluster as the screen lays it out (a base with its combining
marks, a whole emoji sequence), a cell inside a wide character takes
the whole character, trailing blanks are dropped
from each row, rows are joined with newlines. Blank selections say
`nothing to copy`.

Double and triple clicks select like a terminal's. Left-button presses
on the same cell, each at most 400 ms (`clickGap`) after the one
before, form one multi-click: the first press is a click (or the start
of a drag) as above, the second selects the word under the pointer,
the third the whole screen row under it (the row as shown, from its
first cell to its last character; not the whole wrapped message), and
any further quick press on that cell keeps the row. A pause longer
than 400 ms, a press on another cell, a drag or a Shift-click ends the
multi-click, so the next press starts a new one. The word or row shows
in reverse video while the button is down, is copied on release and
stays highlighted after it exactly as a drag's selection does (the same
channels, `/clip` modes and status); moving the pointer before the
release does not change it. A
double click on a blank cell says `nothing to copy`.

A word is taken from what the screen shows, cell by cell: the longest
run of non-blank characters around the pointer's cell on that row
(blank: a space, a no-break space or any other Unicode space; a wide
CJK character or an emoji sequence is one character in two cells, and a
pointer on either cell takes it). Then, from the ends of the run and
repeated until nothing changes: trailing sentence punctuation (`.` `,`
`;` `:` `!` `?`) is dropped; a quote (`"`, `'` or a backtick) at either
end is dropped; an opening bracket (`(` `[` `{` `<`) at the start is
dropped together with the bracket that closes it at the end, or alone
when the run holds more of it than of its closing bracket; a closing
bracket at the end is dropped when the run holds more of it than of its
opening bracket. So a link, an `@nick` or a `#chan` is one word:
`(https://example.org/a_(b)).` gives `https://example.org/a_(b)`,
`Go_(game)` keeps its bracket, `<ada>` gives `ada`, `@alan,` gives
`@alan`, `12:34` stays whole. When the pointer is on a dropped
character (the `<` of `<ada>`), or nothing is left, the word is the
whole run. Decision: these rules make the common things in a chat line
(links in brackets or at the end of a sentence, quoted words, nicks in
angle brackets) copy without editing, and a word never splits at
anything but a blank.

A Shift-click extends the current selection to the clicked cell and
copies the result. It extends from the anchor of the last drag (the
cell where its button went down) or of the last plain click (its cell);
after a double or triple click, from the end of the word or row farther
from the clicked cell, so the word or row stays whole. Every Shift-click
extends that same selection, so a second one moves the end again, to
either side; moving the pointer with the button still down after a
Shift press drags the end. With nothing clicked or selected yet, a
Shift-click is a plain click. The anchor is a screen cell, not a place
in the text: after scrolling or switching windows it is the same cell.
Many terminals keep Shift+click and Shift-drag for their own selection
while the mouse is reported (most Linux terminals among them), so a
Shift-click reaches lazygram only where the terminal passes Shift
through to the application; elsewhere the terminal's own selection
answers it.

Clickable text (links, nicks, chat names, window tags, `[photo]`
tokens, list rows and their buttons, bot button lines, the scrolled-up
marker) never acts twice in one multi-click: the first click acts on
release as a single click does, the second and third presses select
instead of acting. A double click on a link opens it once and copies
it; a triple click opens it once and copies the row. When the first
click replaced the row under the pointer (it opened or focused a
window, ran a list row, jumped to the newest line), the rest of that
multi-click does nothing: it neither selects nor clicks on the new
screen. Likewise, when the row under a word or row selection changed
between its press and the release (a picture fetched by the first
click landed, a message arrived), nothing is copied and the status
says `nothing copied: the screen changed`. Decision: acting on the first click keeps single clicks
immediate (no wait to see whether a second press follows), selecting
on the second press is what a terminal's double click does, and
ignoring the rest after the screen changed keeps the clipboard from
being overwritten with text that was not under the pointer when the
gesture began.

After the release the selection stays in reverse video, as a
terminal's does: every selection that showed while the button was
down (a drag that moved, a word, a row, a Shift extension) is kept, in
every `/clip` mode (with `off` the highlight marks what was selected
though nothing is copied), also when the release says `nothing to
copy` or the clipboard tool reports an error. A plain click (press and
release on one cell) leaves nothing highlighted. A word or row whose
release says `nothing copied: the screen changed` leaves nothing
highlighted either: what is there now was never selected. The kept
highlight goes, and never comes back, when:

- the next left-button press happens, anywhere and of any kind: a
  click, the start of a drag, a press of a double or triple click, a
  Shift-click. The new gesture's own selection shows while the button
  is down, as above, and is kept in turn after its release; a
  Shift-click still extends the last selection as described above.
- a screen row it covers no longer reads, as plain text (styling
  aside), as it did when it was copied: as the last frame before the
  release showed it, which is what the copy read. Scrolling, a message
  that moves the lines, switching windows, opening `/list` or
  `/settings`, typing into an input row it covers and a new status
  message on the rule it covers all do this. The copy's own `copied
  ...` status does it too when the rule shows the status and the new
  one reads differently from what the rule showed: in a window the
  first copy over the rule drops the selection at the first frame
  after the release. In `/list`, `/win`, `/settings` (unless a setting
  was just changed) and the history pager the rule shows the view's
  own text instead, and a copy that reports the same as the one before
  leaves the rule as it was, so there a selection over the rule stays.
  The rows are compared when the screen is drawn; scrolling back to
  the same text does not bring the highlight back. The mouse wheel
  does not remove it by itself, only through what it moves: the lines,
  or the cursor of a list.
- a cursor drawn only in colour moves into, out of or along a row it
  covers, though the row reads the same: the cursor row of `/list`,
  `/win` and `/settings` (Up, Down), the chosen button of `/list` and
  `/win` (Left, Right), the input cursor (Left, Right, Home, End, Up,
  Down between input rows), the active window's tag in the window bar
  (switching windows). A cursor that moves on rows the selection does
  not cover leaves it, so typing a reply below a kept message keeps
  its highlight.
- the terminal is resized, or mouse reporting is turned off (`/mouse`,
  or the `mouse` row of the settings menu), since the terminal's own
  selection takes over.

Keeping the highlight changes nothing else: the base of a Shift-click
(the last click, drag, word or row), the text copied, the status
messages, the OSC 52 sequence and the multi-click timing are as
described above. Decision: a terminal's selection stays after copying,
so one sees what was copied; lazygram's selection is in screen cells
and cannot follow the text, so it goes as soon as the text under it
changes rather than mark text that was not copied. The cursors drawn
only in colour count as part of their row because the highlight is
plain reverse video over the cells and hides them: under it one could
not see which setting or chat Enter acts on, or where typing goes.

Two channels carry the text, and `/clip` chooses (the choice is kept):

- `osc52`: the OSC 52 escape sequence, base64 payload, written into the
  frame so it reaches the terminal in order with the rendering. The
  terminal, not lazygram, sets the clipboard, so it works over SSH when
  the terminal allows OSC 52 writes (iTerm2, kitty, WezTerm, foot,
  Alacritty, Windows Terminal, Ghostty; macOS Terminal does not; tmux
  passes it on with `set-clipboard on` or `external`; GNU screen gets it
  DCS-wrapped). The sequence rides on the rule row for 250 ms and is
  then dropped from the frames.
- `tool`: a system clipboard command on stdin, chosen by platform:
  `pbcopy` on macOS, `clip` on Windows, `wl-copy` under Wayland
  (`WAYLAND_DISPLAY` set), `xclip -selection clipboard -in` or `xsel
  --clipboard --input` under X11 (`DISPLAY` set), `clip.exe` under WSL;
  only fixed names looked up on PATH, never a shell, and never on a
  headless host. The tool runs in the background with a 5 s limit; a
  failure lands in the status as an error.
- `auto` (default): both; whichever the environment honours wins.
- `off`: dragging, double, triple and Shift clicks copy nothing (the
  highlight still shows while the button is down and is kept after the
  release, as above); single clicks still work, and the later
  presses of a double click still do not act.

Shift-drag (Option on macOS) still reaches the terminal's own
selection, as does `/mouse` off. Reason for the in-app selection: the
terminal's drag is gone while the mouse is reported, and copying a
line from a chat is the most common reason to reach for the mouse.

## Attachments

A message with a photo or file shows a token before its caption:
`[photo]`, `[image: name]` or `[file: name]`. Nothing is downloaded when
the message arrives. Clicking the token, or `/open` (the latest such
message in the window's chat; `/open <msg>` for a given one), fetches the
attachment through `Provider.Fetch` into memory, seals it into the vault
(specs/storage.md: encrypted file, hashed name; fetched once, served from
the vault afterwards), then shows it. A picture (`photo`, or `image`: a document whose mime
type is image/jpeg, png, gif or webp) is decoded in memory and painted
into the window as colour cells (upper half blocks, two pixel rows per
text row, box-filtered, at most 24 rows and 80 columns, aspect kept),
followed by a notice with its size; nothing is written to disk. `/open
out [msg]` instead writes a plain copy into a private temporary directory
(0700, file 0600) and hands it to the system opener, which starts the
preview app; that copy is named by the content type, never by the
sender, only raster image extensions are ever opened, and the copies are
removed when lazygram exits. Any other file only gets `-!- decrypted to
<path> until exit` under its (sanitized) sender name and is never opened. Media lazygram cannot fetch (polls,
locations, ...) shows as `[media]`. Fetch errors land in the window.

## Sending

In a chat window, typed text plus Ctrl-X sends it to that chat through
`Provider.Send`: the text leaves the input at once but goes out only
after the double Ctrl-X window (600 ms), so that a second Ctrl-X within
that window cancels the send, puts the text back into the input and only
toggles "Enter sends". The status says "sending", and a sync is triggered
so the line appears as `<me>` as soon as the service has it. By default
Enter never sends: on plain text in a chat window it adds a line break to
the message, and typing goes on in a new row of the input area (see
Input area), so messages can have several lines and show them while they
are written; the whole text goes out with Ctrl-X. A pasted text
keeps its line breaks (a CR LF pair is one break; other control
characters are dropped as always); in a list or the pager a pasted line
break is dropped, since the filter is one line. With "Enter
sends" on, the roles swap: Enter sends and a single Ctrl-X adds a line
break to plain text (a fast second Ctrl-X still toggles and removes that break). Two Ctrl-X within 600 ms toggle "Enter sends" for the current
window (the prompt becomes `[@name]> `); with Enter sends on, Enter sends
at once. Commands still run, and Ctrl-X still sends. The toggle is per
window and is saved with the layout, so it survives restarts; the status
line ends with `ctrl-x: to send` or `enter: to send` in a chat window.
Ctrl-X sends what was typed (surrounding whitespace trimmed), a line that
starts with `/` included: it never goes through the command parser, so
`/help me` typed into a chat and sent with Ctrl-X reaches the chat as
that text, and the status says `sending to @name as text, not a
command... (Ctrl-X again to keep the text and toggle)` while it is held.
Enter is what runs commands, in both modes. With "Enter sends" on,
Ctrl-X still sends such a line instead of adding a line break (Enter
would run it). A leading space is dropped as before, so ` /x` plus Enter
still sends `/x` as text when Enter sends (no hold on that path; its
status is `sending to @name as text, not a command...`), and ` /x` plus
Ctrl-X keeps adding a line break in that mode, so a message of several
lines may start with a slash. Ctrl-X refuses in the server window and on
empty text, and does nothing in the list or the pager. Decision: Ctrl-X is the one key
that always means "send", so a message may start with a slash without a
trick and a wrong key never runs a command; the cost is that a command
typed and sent with Ctrl-X goes out as text, which the 600 ms hold and
the status line make visible and a second Ctrl-X takes back. Send errors
go to the status line and the window.

## Input area

The input area is the bottom of the screen. Each line of the input (a
line break from Enter, from Ctrl-X with "Enter sends" on, or from a
paste) is a row of its own; there is no return mark. The first row
starts after the prompt (`[@name] `, `[@name]> `, `list > `, `windows > `,
`settings > `, `search > `); every later row is indented to the prompt's width, so the
text lines up under the first. A row holds the screen width less the
prompt's width and one cell, the cell the cursor takes past the end of a
line. A longer line wraps onto further rows between characters (grapheme
clusters, as the screen lays them out), so a wide CJK character or an
emoji sequence is never split, and no row is wider than the screen (a
character wider than the room left gets a row to itself). The cursor is a
reverse-video cell on its row: on the character under it (the whole
character, also when the cursor stands between the runes of an emoji
sequence), or one cell past the text at the end of a line. At the place
where a line wraps the cursor shows at the start of the next row.

The area grows upward by one row per row of input, up to a cap: at most 8
rows and at most a third of the screen height, never less than one. The
content area above it shrinks by the same number of rows; the rule, the
status line and the window bar stay directly above the input area, in
that order, and are drawn as before. With one row of input the screen is
exactly as it was when the input was always one line. Past the cap the area
scrolls: the rows shown stay put while the cursor moves among them, and
when the cursor leaves them they move just far enough to show its row
again; the prompt scrolls away with the first row. On a screen of at
least 5 rows the frame has exactly the screen's height (the cap of a
third of the screen always leaves the content area a row), and clicks,
the running light under the pointer, drags and double and triple clicks
map screen rows through the rows of that frame (content, rule, status
line, window bar, then the input rows), so they hit the row they point
at whatever the height of the input area; a drag over the input rows
copies them as shown, prompt and indent included. The content area and
the input area each keep at least one row, so on a screen of fewer than
5 rows the frame still has 5 rows; the terminal shows only its bottom
rows, and the mouse misses by the rows it hides, as it did before the
input area could grow. The list filter and the pager search take no line
breaks, so they stay one row unless they are longer than a row.

In a window (not in a list or the pager), when the input has more than
one row, Up and Down move the cursor to the row above or below, keeping
its column (counted in cells from the start of the text) as far as that
row allows: on the last row of a line the cursor can stand past the
text; a wrapped row, whose end is the next row's start, ends on its last
character, and a column inside a wide character goes to its start. A run
of Up and Down presses keeps the column it started from, so passing a
short row does not lose it; any other key ends the run. Up on the first
row and Down on the last row walk the input history as before (see Input
history), so with one row they always do. Ctrl-A and Ctrl-E go to the
start and the end of the cursor's line (the text between line breaks,
not one row of a wrap), and Ctrl-K deletes from the cursor to the end of
that line (at its end it deletes nothing; Delete or Backspace remove a
line break). The other editing keys are unchanged: Ctrl-U clears the
whole input, Ctrl-W deletes the word before the cursor, Left and Right
move one rune and cross line breaks.

Decision: the input shows the message as it will be sent, line for line,
so a message of several lines can be read and corrected while it is
written; a long line wraps rather than scrolling sideways for the same
reason. The cap keeps the messages in view on any screen. The content
height is computed from the real height of the input area, and the click
rows come from the same frame, so the mouse hits what is drawn. A screen
of fewer than 5 rows keeps the 5-row frame rather than dropping the one
message row or the one input row, because without either the screen has
nothing to work with; the mouse is not corrected for the rows such a
screen hides.

## Typing indicators

A service that reports who is composing (`provider.TypingSource`, the
Telegram connector does) feeds `domain.TypingState`, which the TUI owns:
one report per member and chat, good for six seconds (the official
clients renew every five), renewed by the next report, ended by a stop
report, by the member's message arriving through sync, or by the expiry.
A member the service cannot name is `someone`; unnamed members of one
chat share that single entry, which a message from a sender nobody
reported by name also ends (the sender most likely is that member). The
status line shows, between the activity list and the counters,
`ada is typing`, `ada and bob are recording`, `ada, bob and 2 others are
typing` (the verb is the common kind: typing, recording, uploading,
choosing; `typing` when they differ) for the current chat window; the
server window shows every chat among the listed dialogs, the earliest
report first, each name with its chat: `ada (#dev) and someone (@alice)
are typing`; a report for a chat lazygram does not list (beyond the 100
most recent, or before the first dialog list arrived) is kept for its
expiry but not shown. The indicator takes only the room the other
segments leave: when the names do not fit it becomes `2 typing`, and
when even that does not fit it is left out, so the counters never go. A
running light crosses the verb, to the right then back (150 ms a step);
it follows `/fx` like the lights over clickable text, and with the lights
off the verb is plain.

Two timers serve this, each armed only while needed: an expiry timer,
armed once while any report is pending and re-armed by its own firing;
the light's tick, armed only while the current window shows someone and
the lights are on, and dropped by the first tick that finds nothing to
light. The timers are armed again after every message the model handles,
so a window switch, a closed window or `/fx` is followed at once. The
reports are read one at a time from the service's feed by a command that
re-arms itself; the service closes the feed when it stops, which ends
the reads. Nothing is ever sent back: lazygram does not report its own
typing. Decision: reports are state, not messages; they never enter the
windows or the cache.

## Read receipts

Off by default: lazygram reports nothing read to the service on its
own, whatever it shows; only an explicit request (`/mark-as-read`, the
`[read]` and `[unread]` buttons) reports, once. `/read-receipts` with no
argument says so. `/read-receipts on` (kept in the state store under
`read_receipts`; absent or unreadable means off) makes lazygram report a
chat read at the moments it marks the chat read itself: when a window
with unread messages is focused (`/win N`, `/win @name`, Alt-N, Ctrl-N,
Tab, a click on the window bar or a list row). The report covers the
newest message lazygram holds for the chat (`Provider.MarkRead` with that
message id), so what lazygram has not shown stays unread at the service;
with no message of the chat known yet nothing is sent. Nothing is sent
for the server window, for a window without unread messages, for a
message arriving in the window already shown, for opening or filtering a
list, or for the pager. A failed report shows in the status line and the
server window and is not retried; the local unread count is cleared
either way. `/mark-as-read` sends one report for the current window's
chat on request, clears its unread count, and leaves the setting alone:
it works with receipts off and never turns them on. It is an error in
the server window ("/win to a chat first") and for a chat without a
known message ("nothing to report"); the service's answer is shown
("reported @name read" or the error). The `[read]` button of `/win` and
`/list` is a one-time request for its row's chat too, with the setting
off or on, and covers the service's newest message, not the newest
lazygram holds (`Provider.MarkRead` with an empty id, the dialog's top
message on Telegram): a row's chat often holds only old cached messages
while newer ones are unread at the service, and the button means "this
chat is read". Both requests also clear the chat's unread mark
(`Provider.MarkUnread(false)`), even when the receipt fails, so a chat
marked unread from lazygram or the phone ends up read everywhere.
`[unread]` is the opposite one-time request: it sets the unread mark,
which the chat list then shows as one unread (a chat the service reports
marked and without unread messages counts 1). Requests for one chat go
out one at a time in the order pressed; a press while one is on its way
waits, a later press replaces a waiting one, so the service ends where
the last press said. A failed receipt shows as `read receipt for #chat:
...`, a failed mark as `unread mark for #chat: ...`. Decision: a receipt is visible to
the other side of a private chat and marks the chat read on every device
of the account, so it is opt-in; the focus moment is chosen because it
is the only existing action that clears a window's unread count, and a
message arriving while a window is shown is not reported because
lazygram cannot tell whether anyone is looking.

## Search in either script

The `/search` pager and the `/list` filter compare a query with the text
in two ways: as typed, lower-cased, and with both sides transliterated to
Latin (`domain.Matcher`: the query is transliterated once per search and
every text into a reused buffer, so the search allocates nothing per
message; since the mapping is rune by rune, a plain match is always a
transliterated match too, and comparing the transliterations is the
whole rule). Combining marks are dropped from Latin text as well, so a
decomposed accent does not hide a name. A query that transliterates to
nothing (combining marks only) matches every text, like an empty one,
but never resolves a chat name.

Chat names given to `/whois`, `/win @name` and `/msg` resolve through
`domain.ResolveChat` in tiers, ignoring case, the first tier with any
hit deciding (one hit resolves, several are ambiguous) and a sigil
restricting the kind as before: (1) the exact id or alias; (2) the exact
title (trimmed), so the name `/names` shows resolves even when another
title contains it or another alias begins with it; (3) a prefix of the
alias; (4) a substring of the alias or the title; then in either script,
both sides transliterated (`domain.Matcher`): (5) the exact alias or
title; (6) a prefix of the alias or the title; (7) a substring of either.
A query that transliterates to nothing gets no either-script tier. So
with two people aliased `ivan` and `ivanka` in Cyrillic, `ivan` and
`@ivan` find the first, as its Cyrillic spelling does; with channels
aliased `novini` and `novini-sport` in Cyrillic, `novini` and `#novini`
find the first; `kamen` and `@Kamen` (Cyrillic) find the chat titled
Kamen even when one titled Kamenov exists, as long as no plain tier
matches first; a title two chats share stays ambiguous. A plain match
still wins: when Kamenov's alias is the Latin username `kamenov`,
`kamen` begins it as typed (3) and finds Kamenov, while `Kamen` written
in Cyrillic finds Kamen by its exact title (2); Tab, which offers both,
is not affected. Decision: the exact and prefix tiers in either script
make a Latin spelling as decisive as the Cyrillic one once no plain tier
matches (before, `ivan` was a substring of both transliterated aliases
and ambiguous while the Cyrillic spelling was exact), and they come after
the plain tiers, so they never make a plain match ambiguous or change a
name that resolved before. The exact title comes right after the exact
alias so the name the window bar shows by name finds its chat, unless
two chats of the kind show that title, it is another chat's alias or id,
or showing it dropped control characters (Tab then fills in the alias,
see Names: nick or title); the only names that resolve differently
because of it are queries that are some chat's exact title and begin
another chat's alias: they now find the titled chat (or are ambiguous
when several chats of the kind have that title) instead of the one whose
alias they begin.

Tab completion matches prefixes in either script (`domain.PrefixMatch`,
the transliteration of the word a prefix of the transliteration of the
name): chat names by alias or title (`/win iv` completes `@иван-петров`,
`/win нов` completes `#novini`) and the members of a group (see
Members). A chat is filled in as `/names` shows it (see Names: nick or
title), a member as `@username`. A common prefix of several candidates
extends the word only when it says more than the word (compared in
transliteration, so `zhiv` is extended to `Живк`) and still starts with
it, sigils aside and in either script; otherwise the word stays and the
second Tab lists the candidates. So members matched by their names are
never replaced by an unrelated prefix of their usernames (`ив` does not
become `@van`).

The transliteration is the Bulgarian streamlined system (zh, ts, ch, sh,
sht, yu, ya, hard sign as a, soft sign as y, the stressed i as i)
extended with the Russian and Ukrainian letters it lacks (yo, y, e, ye,
i, yi, g); combining marks such as stress accents are dropped; Latin
letters pass through, so `ivan` finds the name written in Cyrillic, the
Cyrillic spelling finds `Ivan`, and `sreshta` finds the Cyrillic word for
a meeting. The spelling is the system's, not every habit: the Cyrillic
ya is `ya`, so the city of Sofia written in Cyrillic is found by
`sofiya`, not by `sofia`. A Cyrillic query finds Latin text when its
transliteration spells it (the Cyrillic letters for "smit" find
`Smith`), but a phonetic Cyrillic spelling that does not (the letters
for "elis" do not find `Alice`) is no match: Latin is never
transliterated back. The pager highlights plain matches only; a line
found through the transliteration is listed without a highlight. (The
examples are kept in Latin here; the test files carry the Cyrillic.)
Decision: this is the cheapest rule that lets a Latin keyboard find
Cyrillic chats and messages without a dictionary, and it never hides a
match the plain comparison would find.

## Keys

| key                  | action                                            |
|----------------------|---------------------------------------------------|
| Tab / Shift-Tab      | empty prompt: next / previous window (from the settings menu too, which the switch closes); else complete like bash (a second Tab lists): commands, chats, themes, `out`, attachment ids and the chat's recent links after `/open`, paths after `/send`, a command's settings (`/autojoin off`, `/clip tool`); in a group or channel window its `@nicks` after `/whois` and in message text |
| Ctrl-X twice         | toggle "Enter sends" for this window (Ctrl-X then adds a line break to plain text) |
| Ctrl-N / Ctrl-P      | next / previous window (Alt-Right / Alt-Left too) |
| Alt-0 .. Alt-9       | window 0 .. 9; Esc followed by the digit does the same (the typed input survives) |
| Up / Down            | with several input rows: the row above / below, keeping the column (see Input area); on the first / last row: walk the input history (commands and sent lines); in a list they move, in the pager they scroll |
| PgUp / PgDn          | scroll a page (the wheel scrolls three lines)     |
| mouse drag           | select text; copied to the clipboard on release, and it stays highlighted until the next press, or until the text under it changes or a cursor in it moves (see Mouse) |
| double / triple click | select the word / the screen row under the pointer and copy it, kept highlighted as a drag's selection is; on clickable text the first click acts, the next ones only select (see Mouse) |
| Shift-click          | extend the last selection to the clicked cell and copy it, kept highlighted as a drag's selection is (only where the terminal passes Shift through) |
| Home / End           | oldest / newest                                   |
| Ctrl-R               | sync now                                          |
| Ctrl-L               | redraw: drop this window's notices, help text and pictures, as `/redraw` |
| F1                   | focus the server window and print the command reference there (as `/help`) |
| Esc                  | close the list, the settings menu or the pager, else clear the input |
| Ctrl-X               | send the typed text to this window's chat, as typed: a line starting with `/` is sent, not run (Enter runs commands); with "Enter sends" on it still sends such a line |
| Ctrl-U / Ctrl-W      | clear the input / delete the word before the cursor (and the spaces after it) |
| Left / Right         | move the cursor one rune; never past the ends; in `/list` or `/win`: pick the row's action button (open or focus, read, unread, close), Enter runs it; in the settings menu: the selected setting's previous / next value |
| Ctrl-A / Ctrl-E      | cursor to the start / end of the cursor's line (between line breaks) |
| Delete / Ctrl-K      | delete the rune under the cursor (at the end of the input Delete acts as Backspace, since some terminals send it for that key) / to the end of the cursor's line |
| Ctrl-C               | quit                                              |

## Input history

Every entered line (a command, or text sent with Ctrl-X or Enter) goes
into a per-session history of up to 200 lines, immediate duplicates
collapsed. In a window, Up walks back through it and Down forward (from
the first and the last row of an input of several rows; between its
rows they move the cursor, see Input area); past the newest entry the
line that was being typed comes back. Typing leaves the walk. Down with
no walk under way changes nothing: the typed text stays. Decision: Down
used to put back the text saved when the last walk began (nothing after
a send), which dropped what was typed since; with several rows a Down
too many on the last row would have thrown a draft away. In a list
Up/Down move the cursor, in the pager they scroll.
A recalled line is input like a typed one, and the next key decides:
Ctrl-X sends it as text, Enter runs it when it starts with `/` (in both
modes). To resend a slash line, recall it and press Ctrl-X.

## Completion

The input has a cursor, drawn as a reverse-video cell (on the character
under it, or one past the text at the end of a line); typing inserts at
the cursor, Backspace deletes before it, Delete under it. A line longer
than a row of the input area wraps onto the next row (see Input area).
History recall, Esc, Enter and the list and pager prompts put the cursor
at the end.

On an empty prompt in a window, and in the settings menu (the switch
closes it), Tab switches to the next window and Shift-Tab to the
previous one. In the settings menu's filter Tab completes the names of
the settings (`re` -> `read-receipts`); a line starting with `/` there
completes as a command line. With text typed, Tab completes the word
before the cursor like bash and keeps what follows the cursor. A line that is one
word starting with `/` completes command names; after `/theme ` theme
names; after `/send ` (and its aliases) local paths: entries of the typed
directory that start with the typed base name, `~` expanded (a lone `~`
completes to `~/`; the typed directory part is kept as typed and only
the base name after the last `/` is matched), hidden files only when a
dot is typed, directories filled in with a trailing `/` and no space so
the next Tab descends; the caption after the path is free text and never
completed. Decision, after a crash: the typed text is split before `~`
is expanded, because the expanded `~/` loses its trailing slash and the
home directory's name would be taken for the typed base name. After `/open` (and `/fetch`, `/download`): `out` as
the first word, the chat's ten most recent distinct `http(s)` links
(newest message first, taken from the same sanitised text a click uses,
so no control character reaches the input and trailing punctuation is
not included; matched case-insensitively and filled in with their own
case, and when some match the typed case exactly only those are
offered, so two links that differ in case stay apart) and the ids of the current window's messages that
have an attachment; after `/open out` only the ids; nothing in the
server window (an `/open <link>` typed there still works).
In a group or channel window the `/whois` argument and the words of
message text complete that chat's `@nicks` instead (see Members).
Otherwise a command's own first-argument words, whatever its spelling
(`/win list`, `/list --show-deleted`, `/autojoin on|off`,
`/feed on|off`, `/read-receipts on|off`, `/clip auto|osc52|tool|off`),
and chat names
where the command takes a chat: the first argument of `/win`, `/msg`,
`/whois` and `/list`, every word of `/me` and `/edit` text, and every
word of a line without a command (message text). After any other
command (`/last`, `/search`, `/press`, the settings) Tab offers no chat
names: those arguments are numbers, settings or free text. A chat is
offered when the word begins its alias or its title as shown, in either
script (see Search in either script), by nick and by name alike; a
typed `@` or `#` must be the chat's own sigil, and without one the alias
or the title alone is matched (the sigil is added). Deleted accounts are
never offered. The chat is filled in as `/names` shows it, or by its
alias when that name would not find it (see Names: nick or title). A
title may have spaces, so the name being typed may span words: the first
argument of `/win`, `/msg`, `/whois` and `/list` is one name when all of
it begins a chat's alias or title (by name `/msg @Ivan P` -> `/msg @Ivan
Petrov `, and a common prefix may end in a space: `/win @Kal` -> `/win
@Kaloyan ` when two titles go on differently, after which the second Tab
lists them), and in message text and in `/me` and `/edit` text the name starts
at the earliest of the last four word starts whose rest of the line
begins some chat's alias or title, else at the last word, as for members
(`hi Kaloyan G` -> `hi @Kaloyan GPS`). Text after the name of `/msg`
stays free text. Decision: chat names after every command filled
settings like `/autojoin` with nicks. Decision: a title completes across
its spaces because a common prefix that ends in a space would otherwise
leave the next Tab with an empty word. One
candidate is filled in with a trailing space; several are extended to
their longest common prefix, and a second Tab that cannot extend further
prints the candidates into the window as a notice (base names for paths).
There is no cycling.

## Scrolling

Scroll is lines above the bottom, per window. At 0 the window follows new
messages. When scrolled back, arriving messages keep the view anchored,
the last content row shows a short centred marker in the activity colour
(`vvv N more vvv`; a click on it, or End, jumps to the newest line)
and the status line shows `[scrolled up N lines]` in the same colour.

## Status line

`time  provider  [N:name]  Act: ...  ada is typing  M msgs  sync state  scrolled up K lines`
(brackets only around the window tag; two spaces between segments; the
message and the send hint live on the rule above; the typing segment
appears only while someone is composing, see Typing indicators). The
sync segment keeps a fixed width and colour (`synced: HH:MM:SS` or
`syncing HH:MM:SS` with the last completion time) and a poll that brings
nothing changes no status text, so the line does not flicker every ten
seconds.
(the last item says how text is sent in a chat window: `ctrl-x` or `enter`).
Each segment has its own palette colour from the theme (the sync state
changes colour: syncing, synced, never); the window bar stays in one
colour: only the active window is highlighted (reversed) and windows with
new messages show in the activity colour as the indicator.
A background sync runs every 5s (demo) or 10s (telegram) with a 90s
budget (enough for a FLOOD_WAIT retry); polls skip while a sync runs.
The reactions on screen are asked about every 20s, never beside a sync
(see Reactions).

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
chats, syncs once, prints one 100x24 screen of the server window (20
content rows, rule, status, window bar, input) and exits 0. `login` refuses without a terminal.
