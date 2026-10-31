# Overview

lazygram is a local terminal chat client with an IRC feel (BitchX / irssi),
Telegram first, feature goals inspired by nchat.

## Window model

The UI follows BitchX/irssi. Window 1 is the server window: every chat
merged into ONE chronological timeline, each line with time, chat name,
sender and text, plus client notices. A window opens for a chat when a
message arrives for it or it has unread messages, and `/win` switches
between windows; people are `@name`, groups and channels `#name`; Tab
completes names and commands. Like an IRC client, lazygram shows what
arrives while it runs; older messages are fetched per window on demand
(`/last`). Decision: the merged server window is the default view because
the goal is to monitor many chats at once with little context switching;
per-chat windows give focus when a conversation needs it.

## Scope

Bubbletea TUI with windows, slash commands, Tab completion, selectable chat
list, searchable local history, encrypted SQLite cache, demo provider,
read-only Telegram connector with QR login. Sending messages is out of
scope.

## Live Telegram limitations

The live provider uses gotd/td (specs/providers.md) and is read-only. It
needs a one-time QR login (`lazygram login`) and the user's own Telegram
application registration, asked once and stored locally; without them
lazygram exits and says how to link an account or start the demo. No
environment variables. The live path has only been tested against fakes,
never against the real service. Polling, not push updates; 100 most recent
dialogs; no per-chat selection yet.

## Acceptance criteria

1. `make check` passes (gofmt, vet, golangci-lint, tests).
2. `lazygram demo` starts with no account; `lazygram` without a linked
   account exits with status 2 and names `lazygram login` and
   `lazygram demo`; `make smoke` prints one screen non-interactively.
3. The server window interleaves chats in time order with time, chat name,
   sender and text; people show as `@name`, groups and channels as `#name`.
4. A window opens for a chat with a new message or with unread messages;
   `[Act: ...]` lists windows with unread; `/win N`, `/win @user`, Ctrl-N/P
   and Alt-digits switch; opening a chat window fetches its last 10
   messages; `/last N` fetches more.
5. `/list` is a selectable list; Enter opens the chat's window; deleted
   accounts are hidden unless `--show-deleted`.
6. Tab completes commands and `@`/`#` names and cycles on repeat.
7. `/search` pages through the local history of the current chat and
   narrows live while typing.
8. The on-disk cache holds no readable message text, sender, chat id or
   title, and cannot be read with another session's key (tested).
9. Core packages do not import a connector package.
10. Nothing sensitive is committed.
