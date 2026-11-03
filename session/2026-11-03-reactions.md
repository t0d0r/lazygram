# 2026-11-03: reactions

## Done

- Data: `domain.Message.Reactions` ({Emoji, Count, Mine, MineCount}) in
  the service's order; all custom-emoji reactions of a message summed
  into one `*`, a paid reaction as the star emoji, empty and zero
  entries dropped; a `min` report keeps the known own share. A change
  kind `Reacted` carries a message's current reactions, the reactors
  the service names and whether it was pushed.
- Cache (internal/store): reactions persist with the message; the copy
  fetched or reported later always brings its reactions, even with the
  text unchanged or an older edit time; `Store.React` changes only the
  reactions.
- Telegram provider: reactions converted in history and edits;
  `updateMessageReactions` (pushed for the account's own messages) is a
  `Reacted` change on the existing change feed; an optional
  `provider.ReactionPoller` calls `messages.getMessagesReactions` for
  given messages of one chat. A refused chat (`ErrNotPollable`) is
  skipped for the session; FLOOD_WAIT (`*provider.WaitError`) pauses
  polling until it has passed; nothing is printed.
- Demo provider: four lines carry reactions from the start; after 25 s a
  reaction to one of the account's lines is pushed, after 100 s one to
  someone else's line is there for a poll to find.
- TUI (internal/tui/reactions.go): counts after the text in every window
  and the pager, `[E 3 F 1]`, the account's own in the highlight style,
  wrapped like the text, the render cache keyed on them; polls every
  20 s of the messages on screen in the focused window that others sent
  (window 0: one chat per tick, in turn), never two calls within 15 s,
  none while a sync or a poll runs; window 0 notices when someone else
  adds a reaction (`#dev: @grace reacted E to ada "first words"`, or the
  whole set for anonymous channels; `your` for the account's messages,
  `a message` for one not cached). Never told: the first sighting this
  session, removals, the account's own reactions. Notices follow `/feed`
  and are capped apart from window 0's own notices; they open no window
  and count nothing.
- specs/ui.md (Reactions, with Decisions), specs/providers.md,
  specs/storage.md, docs/usage.md, `/help`.
- Built in a worktree by two implementing agents (backend, then TUI),
  reviewed for correctness, Telegram API use and spec/test strength,
  each finding verified by a second agent: 5 confirmed and fixed (a
  cached copy fetched unchanged did not count as learned, so the next
  reaction was silent; hidden notices pushed out window 0's own; the
  account's own Stars and custom reactions read as someone else's;
  polls closer than 15 s after a long sync), 1 refuted. Every new test
  was checked against a mutant of the code it guards.

## Decisions

- Only the messages on screen are polled, as Telegram asks apps to: a
  busy account costs one small call per 20 s however many chats it has.
- The first sighting and removals are silent, so opening a chat, a heal
  or a restart never floods window 0.
- Notices follow `/feed`: they are chat activity.

## Open

- Not run against live Telegram (no real-account runs from automation):
  pushes for own messages, polls in groups and channels, and the rule
  that a message missing from a poll answer has no reactions (taken
  from TDLib's behaviour, not the docs) need a check on a real account.
- Follow-ups: `/react`, the alt emoji of custom reactions, who reacted.

## Files touched

- internal/domain, internal/store, internal/provider/provider.go,
  internal/provider/telegram (reactions.go, changes.go, history.go,
  gotd.go, telegram.go), internal/provider/demo, internal/tui
  (reactions.go, render.go, model.go, changes.go and tests),
  internal/command/command.go; specs/ui.md, specs/providers.md,
  specs/storage.md, docs/usage.md, todo/todo.md, this note

## Verify

- `make check`, `go test -race ./...`, `make smoke`
- `lazygram -data <scratch dir> demo`: counts on the release line in
  #dev; after about 25 s a notice in window 0 for the reaction to your
  line; with #dev focused, after about 100 s one for grace's reaction

## Next

- Merge; try it on a real account; the next release carries it.
