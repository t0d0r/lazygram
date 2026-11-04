# 2026-11-04: /close with lists and ranges

## Done

- `/close` (also `/wc`, `/part`) takes a list and ranges as well as one
  number (internal/tui/close.go): `/close 2,3-5`, `/close 1-4`,
  `/close 1,2,3`, `/close 2, 3-5`, `/close 2 3 5`. `/close` alone still
  closes the current window.
  - Every number is the window as numbered before the command, as the
    window bar showed it: `/close 2,3` closes the windows that were 2
    and 3.
  - All or nothing: the whole argument is checked first. A malformed
    item (`x`, `2-`, `+2`, `2,,3`, `2 - 5`, a reversed range `5-3`) is
    `/close takes window numbers: N, a list or ranges (as 2,3-5)`;
    window 0 anywhere is the server-window error; a number past the
    last window is `no window N` (the first, left to right). On any
    error nothing changes: windows, current and last window, notices,
    counts, the stored layout.
  - Repeats and overlaps close a window once.
  - Each closed window gets `window closed: <name>` in window 0, in
    window order; the status says `closed <name>`, or `closed N
    windows: 2:#dev, ...` cut after four as `+N more`; the layout is
    saved once.
  - The current window stays current under its new number, or, if it
    closed, the nearest open window before it becomes current (window 0
    if none). Closing focuses nothing: no read receipt, no unread count
    changes. The last window is dropped if it closed.
- Changed error texts: a non-number now gets the list error (was
  `/close takes a window number`); a number past the end is `no window
  N` (was the server-window error); `+2` is malformed (it closed window
  2 before).
- specs/ui.md (Windows, commands table, with Decision lines),
  docs/usage.md, `/help` (`/close [N,M-K]`).
- Built by one implementing agent, reviewed for correctness/test
  strength and spec conformance, each finding verified: 2 confirmed and
  fixed (notices from a chat window untested; a reversed range whose
  ends overflow an int was not caught as reversed), 5 refuted. Every
  new test was checked against a mutant (25 mutants, all caught).
- Checked in the demo: `/close 2,3-9` says `no window 9` and closes
  nothing; `/close 2,3-4` closes #dev, #ops and @linus.

## Decisions

- Numbers as shown before the command, so a list means what was read
  on screen; all or nothing, so a typo closes nothing.
- A range past the last window is an error, not cut short, under the
  same rule.

## Files touched

- internal/tui/close.go, close_test.go, model.go;
  internal/command/command.go, command_test.go; specs/ui.md,
  docs/usage.md, todo/todo.md, this note

## Verify

- `make check`, `go test -race ./internal/tui/ ./internal/command/`
- `lazygram -data <scratch dir> demo`: `/close 2,3-4`, then `/win list`

## Next

- Merge; v0.4.28 carries it with the /help notes layout.
