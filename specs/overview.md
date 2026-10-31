# Overview

lazygram is a local terminal chat client with an IRC feel (BitchX / irssi),
Telegram first, feature goals inspired by nchat.

## Single-channel model

All configured chats are merged into ONE chronological timeline, displayed
like a single IRC channel. Each line shows time, chat alias, sender and text.
The chat is a label on a line, not a separate window. The user may narrow the
view to one chat (select) or by text (filter) and can open the full persisted
history of a chat in a pager with search. Decision: one feed by default because
the goal is monitoring many chats at once with low context switching.

## Scope v0.1

Bubbletea TUI, slash commands, key bindings, chat list, history pager with
search, JSONL cache, demo provider, config validation, inert Telegram stub.
Read-only: sending messages is out of scope.

## Live Telegram limitations (v0.1)

The live provider uses gotd/td (specs/providers.md) and is read-only. It
needs operator-supplied API credentials and an already authorized session in
LAZYGRAM_TG_SESSION; lazygram has no login flow yet and fails with a clear
error otherwise. The live path has only been tested against fakes, never
against the real service. Polling, not push updates.

## Acceptance criteria

1. `make check` passes (gofmt, vet, golangci-lint, tests).
2. `make run` starts with no account using the demo provider; `make smoke`
   prints one screen non-interactively.
3. Feed lines from different chats interleave in time order and show time,
   alias, sender and text.
4. Commands help, quit, refresh, chats, select, filter, history, search work;
   Tab cycles chats; PgUp/PgDn scroll.
5. History view shows every cached message for the chat (or all) and narrows
   live while typing a search.
6. History survives restart (cache reload).
7. Core packages do not import a connector package.
8. Nothing sensitive is committed.
