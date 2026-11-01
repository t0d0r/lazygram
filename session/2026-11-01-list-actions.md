# 2026-11-01: list actions

Branch `feature/list-actions` in the worktree `../lazygram-list-actions`,
from `master` at `87701ae`, not committed. The queued todo entries of the
main checkout were carried into the branch's `todo/todo.md`.

## Done

- `/win` and `/list` rows carry a strip of action buttons on the selected
  row: `[focus] [read] [unread] [close]` for windows, `[open] [read]
  [unread]` for chats. Left/Right move the choice (clamped), Enter runs
  it, a click on a button runs that button, a click on the row runs the
  default. The first button is the default, so Enter without a choice
  opens or focuses as before and a new list starts there.
- Read sets the window's unread count to 0 (and the chat's count in the
  `/list` row); unread sets it to at least 1, opening a window for a chat
  without one, without switching; close closes the window (window 0
  refuses). The three keep the list open on the same row and button, so
  Down, Enter repeats down the list; the filter stays in force.
- Code: `internal/tui/listactions.go` (actions, button strip with click
  spans, `runListAction`, `setChatUnread`), `model.go` (field `listAct`,
  Left/Right in list mode, Enter and click dispatch, the selected row's
  highlight and buttons in View with the row's click target ending where
  the buttons start, info lines, `/keys` text), `render.go` (`spanAct`).
- Tests: `internal/tui/listactions_test.go`: window actions in order
  with clamping, window 0 refusing, the default focus; chat actions with
  a filter kept, unread opening a window without switching, an empty
  list without buttons; Left/Right moving the cursor only outside lists;
  buttons and rows as click targets without overlap.
- Docs: specs/ui.md (Modes, Keys, Mouse), README (list rows, keys row).

## Decisions

- Buttons on the row rather than letter keys: letters type into the
  filter, Left/Right are otherwise idle in a list. The cost is that the
  filter cursor no longer moves with Left/Right inside a list.
- The counts are local: nothing is reported to the service, so the
  account's read receipts stay untouched; the next chats refresh brings
  the service's count back into the `/list` row.

## Open

- None in the code; the branch awaits review and a commit.

## Files touched

- internal/tui/listactions.go, listactions_test.go, model.go, render.go
- specs/ui.md, README.md, todo/todo.md, this note

## Verify

`make check` in the worktree; `go test -run
'ListActions|WinListActions|LeftRight|ListButtons' ./internal/tui/`.

## Next

- Commit on `feature/list-actions` and merge it into `master`.
