# 2026-11-02: edits and deletions made elsewhere

## Done

- The Saved Messages window drifted from the live chat: the polled
  history only sees new messages, so edits made elsewhere never arrived
  (even a fetch kept the first cached copy) and deleted messages stayed
  in the cache for ever.
- Live: the Telegram connector hands updateEditMessage,
  updateEditChannelMessage, updateDeleteMessages and
  updateDeleteChannelMessages to a change feed (`provider.ChangeSource`,
  internal/provider/telegram/changes.go), read by the TUI like the typing
  feed (internal/tui/changes.go). An edit replaces text, media and
  buttons in the cache (`Store.Edit`) and the windows, keeping the cached
  sender; a deletion removes the message and its attachment
  (`Store.DeleteMessages`), without a notice. Deletions without a chat
  (private chats and basic groups share one id space) look in the chats
  marked `Chat.SharedIDs`, which dialogs now set.
- On fetch: a window's history fetch replaces cached copies that differ
  (`Store.Refresh`, which replaces `Store.Identify`) and deletes cached
  messages inside the fetched range that the page lacks (`Store.Prune`).
- `Store.Delete` (own `/delete`) now removes the attachment too.

- A review found and fixed: an edit that swapped an attachment left the
  old one in the vault (`Media.ID` now tells attachments apart and the
  vault copy is dropped); missed updates were never recovered (the shown
  window now quietly fetches its last 10 messages every 2 minutes, on
  switching to it when older, and at once after `updatesTooLong` or
  `updateChannelTooLong`); deletions without a chat reached only the
  recent dialogs (the chats ever marked shared are kept); a chat's
  deleted newest message was never pruned (Prune now covers ids above
  the page older than the fetch); edits could apply out of order and a
  stale page could undo a live edit or bring back a deleted message
  (edit times, session-long deletion memory); a bulk deletion ran
  row by row in the UI loop (one transaction, in a command); a deleted
  message kept its unread count; deleted rows stayed in SQLite free
  pages (secure_delete on).

- A second review of the fixes found and fixed: a deletion could take
  the unread count of another message (windows now track which arrived
  messages they counted); a heal running beside a sync could store a
  new message first and present it as older history (a heal's messages
  newer than the window's newest go through the arrival path); a
  deletion without a chat remembered only stored rows, and a late fetch
  result could put a deleted message back on screen (every pair is
  remembered in the store, and the TUI keeps its own session set).

## Decisions

- No notice for a message deleted elsewhere: it is gone, as in the
  official apps; a deleted-for-everyone message should not outlive the
  deletion in lazygram.
- A dropped change (feed full, lazygram off, a gap) is healed by the
  next fetch of the chat; the shown window is fetched again quietly
  every 2 minutes; messages older than every fetched page keep their
  last known state. No getDifference replay: the periodic fetch is
  simpler and bounded (one history request per 2 minutes).

## Open

- Catch up after a restart (persist each chat's last seen message).
- Forwarded messages in Saved Messages still show as `<me>`.

## Files touched

- internal/domain/domain.go (Change, Chat.SharedIDs)
- internal/provider/provider.go (ChangeSource)
- internal/provider/telegram/changes.go, changes_test.go, gotd.go,
  telegram.go, history.go
- internal/store/store.go, store_test.go
- internal/tui/changes.go, changes_test.go, model.go
- specs/ui.md, specs/storage.md, specs/providers.md, specs/security.md,
  docs/usage.md, todo/todo.md, this note

## Verify

`make check`, `go test -race ./internal/...`, `make smoke`; the TUI tests
send edits and deletions through a fake change feed and a history fetch
with an edited and a missing message. On a real account: edit and delete
a note in Saved Messages on the phone while lazygram shows it.

## State and next

- Branch `feature/live-edits` (worktree `../lazygram-live-edits`) from
  master, three local commits; not merged into master or pushed.
- Next: merge into master when approved; master is ahead of
  origin/master and unpushed.
