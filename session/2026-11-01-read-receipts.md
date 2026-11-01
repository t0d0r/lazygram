# 2026-11-01: read receipts

Branch `feature/read-receipts` in the worktree `../lazygram-read-receipts`,
from `master` at `87701ae`, not committed. The queued todo entries of the
main checkout were carried into the branch's `todo/todo.md`.

## Done

- `/read-receipts [on|off]` (also `/receipts`): off by default, kept in
  the state store under `read_receipts`; no argument shows the setting.
  With it on, focusing a window with unread messages reports the chat
  read to the service up to the newest message lazygram holds; nothing
  is sent for the server window, a window without unread messages, a
  message arriving in the window shown, lists, filters or the pager.
- `/mark-as-read` (also `/markread`): one report for the current
  window's chat on request, independent of the setting, which it never
  changes; clears the local unread count; errors in the server window
  and for a chat without a known message; the answer is shown.
- `provider.Provider.MarkRead(ctx, chatID, msgID)`: demo is a no-op,
  Telegram sends `channels.readHistory` for channels and supergroups and
  `messages.readHistory` for users and basic groups, `max_id` = that
  message, under the client lock, FLOOD_WAIT retried once like every
  call. The inert provider returns ErrNotImplemented.
- Code: `internal/tui/receipts.go` (setting, `readCmd` hooked into
  `switchTo` before the unread count is cleared, `markAsRead`,
  `receiptCmd`, `newestID`, `readMsg`), `model.go` (state key, field,
  command dispatch, completion of on/off), `internal/command`
  (`Receipts`, `MarkRead`, help), `internal/provider/telegram`
  (`service.MarkRead`, API interface, flood wrapper, client and
  provider wrappers), `internal/provider/demo`.
- Tests: command parsing; connector call selection, bad ids, unknown
  chat; TUI: off by default, on reports focus-with-unread only (not a
  refocus, an arrival in the shown window, lists, the pager, a chat
  without a known message), off again, error reported once, setting
  restored from the store, `/mark-as-read` semantics including that the
  setting stays off.
- Docs: specs/ui.md (commands table, Read receipts section with the
  decision), specs/security.md, specs/providers.md (interface, calls),
  README (feature bullet, command rows), `/help` text.

## Decisions

- Receipts are opt-in because a report is visible to the other side of
  a private chat and marks the chat read on every device of the account.
- The focus moment is the only existing action that clears a window's
  unread count, so it is the only automatic trigger; an arrival in the
  shown window is not reported, since lazygram cannot tell whether
  anyone is looking.
- The report covers the newest message lazygram holds, never `max_id`
  0, so what lazygram has not shown stays unread.

## Open and limits

- Messages that arrive while their window is shown are reported only on
  the next focus of that window with unread messages, or with
  `/mark-as-read`.
- A receipt for a chat the account left or a channel it cannot access
  fails with the service's error, shown once.
- The list `[read]` action (branch feature/list-actions) does not send a
  receipt yet; when the branches meet, it should call `receiptCmd` the
  way `/mark-as-read` does only if receipts are on.

## Files touched

- internal/command/command.go, command_test.go
- internal/provider/provider.go, demo/demo.go, telegram/{telegram,
  history,gotd}.go, telegram/history_test.go
- internal/tui/receipts.go, receipts_test.go, model.go, model_test.go
- specs/ui.md, specs/security.md, specs/providers.md, README.md,
  todo/todo.md, this note

## Verify

`make check` in the worktree; `go test -run 'Receipt|MarkRead|MarkAsRead'
./internal/command/ ./internal/provider/telegram/ ./internal/tui/`.

## Next

- Review, then commit on the branch after approval.
