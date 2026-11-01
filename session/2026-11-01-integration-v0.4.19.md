# 2026-11-01: integration of list actions and read receipts

## Done

- `feature/list-actions` (`e99654f`) and `feature/read-receipts`
  (`185f161`) were replayed onto `origin/master` at `21bf5f0` (the
  installer commits) in the worktree `../lazygram-integrate`, branch
  `integrate/receipts-list`, as `ae30142` and `7caee1d`. The one
  conflict was `todo/todo.md`: both entries kept, the list items as in
  progress and the read-receipts item added.
- The two features now agree: the `[read]` button of `/win` and `/list`
  follows the read-receipt rule, reporting a window with unread messages
  read exactly as a focus would, and only with `/read-receipts on`
  (`internal/tui/listactions.go` calls `readCmd` before clearing the
  count; `TestListReadButtonFollowsTheReceiptRule`); specs/ui.md (Modes,
  Read receipts) and the README bullet say so.
- The main checkout's uncommitted `todo/todo.md` (the queued todo-only
  update) was left untouched; its content is a subset of the merged
  todo, which carries the same lines plus the progress marks.

## Verify

`make check`, `go test -race` on command, telegram and tui, `make smoke`
in the integration worktree.

## Next

- Push `integrate/receipts-list` to `origin/master` (fast-forward), tag
  `v0.4.19` after green CI, `make dist` and `make release`, verify the
  assets, record the release.
