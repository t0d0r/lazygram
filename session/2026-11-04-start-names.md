# 2026-11-04: chat names in the start notices

## Done

- On a first start (no chat list cached) the chat list and the first
  sync are asked for at once, and the sync usually answered first: the
  windows it opened were written up with chat ids (`window 1 opened for
  demo-1`, `24 new messages in 1:demo-1 (8)`), texts that never change,
  and a deleted account's message opened a window. Seen with
  `lazygram demo` on a fresh `-data` directory in most runs.
- Now a sync whose new messages include one from someone else in a chat
  the chat list lacks waits for the chat list (`awaitNames`,
  `Model.heldSync`) and goes ahead when it answers, failed or not
  (internal/tui/model.go, the chatsMsg and syncedMsg cases, `synced`):
  - at start it waits for the request already on its way, and the start
    check's windows open first;
  - a chat writing for the first time later asks for the chat list once
    a session (`namesAsked`), so a chat the service never lists costs
    one request and is then named by its id;
  - with no chat list this session and none on its way, nothing waits;
  - while a sync waits it counts as running (no second sync);
  - an edit or a reaction reported meanwhile is put into the waiting
    result (`heldPatch`, from applyChange and `reacted`), so it is not
    undone on screen; a reaction poll that came due goes out on release;
  - a failed chat list's error is told after the release, so the
    arrivals' status does not hide it;
  - smoke mode (`Snapshot`), which runs no commands, gets the chat list
    a waiting sync asked for.
- specs/ui.md, Windows: the rule, with the reason as a Decision.
- Tests in internal/tui/startnames_test.go; each new assertion was
  checked against a mutant of the code it guards.
- Reviewed for correctness and spec/test strength, each finding verified
  by a second agent: 3 confirmed and fixed (an edit during the wait was
  lost on screen; no test that no second chat list goes out at start; a
  failed chat list's error was hidden), 4 refuted (two of them added as
  tests anyway, and the smoke-mode guard). Rebased on the reactions
  merge, which needed the same guard for reactions and the poll after
  release.
- Checked in the real demo: fresh `-data` starts name every chat
  (`window 2 opened for #dev`, `1:#news (4), 2:#dev (8)`).

## Decisions

- Wait for the names rather than rewrite notices later: notices and the
  status are written once and keep their text.
- One chat list per unknown chat a session, not one per poll: a chat the
  service never lists must not cost a request every 30 seconds.

## Open

- On a first start the demo's unread counts add the service's count to
  the replayed arrivals (#news shows 6 for 4 + 2); the same happens
  whenever the chat list answers first, so it is older than this fix.
  Telegram's first sync brings nothing, so it does not arise there.

## Files touched

- internal/tui/model.go, changes.go, reactions.go, snapshot.go,
  startnames_test.go; specs/ui.md; todo/todo.md; this note

## Verify

- `make check`, `go test -race ./internal/tui/`
- `lazygram -data <fresh scratch dir> demo`: window 0 names every chat
  in its notices and status

## Next

- Merge; v0.4.26 carries it.

Security note: the fix sends at most one extra chat list request per
chat that is new in a session; nothing else leaves the client.
