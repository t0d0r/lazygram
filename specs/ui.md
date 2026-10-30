# UI

Layout, top to bottom: topic bar, content, status line, input line.

## Modes

- feed (default): messages filtered by selected chat and text filter, newest
  at the bottom. A dim day separator appears when the local date changes.
- history: full cache for the chat given to /history (or the selection, or
  all). Lines carry the full date. Typed text is a live case-insensitive
  search over text and sender; matches are highlighted. Topic shows
  "N of M messages".
- chats: "all chats" row plus chats with message counts and last activity.
  Typed text filters; Up/Down move; Enter selects; selected chat is starred.
- help: command and key reference.

## Line format

`HH:MM #alias <sender> text`, wrapped with a hanging indent. Chat and sender
colors derive from a hash so they are stable.

## Scrolling

Scroll is lines above the bottom. At 0 the view follows new messages. When
scrolled back, new messages keep the view anchored on the same lines.

## Commands

/help /quit /refresh(/sync) /chats [text] /select <chat> /all /filter [text]
/history [chat] /search <text>. Chat references resolve by id, alias, unique
prefix, then unique substring of alias or title; ambiguity is an error.
Non-command input is rejected (sending unsupported). Parse errors show in the
status line in red.

## Keys

Tab/Shift-Tab cycle chats, PgUp/PgDn/Up/Down/Home/End scroll, Ctrl-O history,
Ctrl-L chats, Ctrl-R sync, F1 help, Esc close, Ctrl-C quit.

## Status line

`[time] [provider] [N msgs] [N chats] [sync state] [-N lines] message`.
A background sync runs every 5 seconds (Poll option).

## Non-interactive mode

When stdin or stdout is not a terminal, or with -smoke, lazygram syncs once,
prints one 100x24 screen and exits 0.
