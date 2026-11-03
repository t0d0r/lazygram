# 2026-11-03: /feed and the /settings menu

## Done

- `/feed [on|off]` (internal/tui/feed.go): with it off, window 0 shows
  no chat messages, only its notices (banner, hints, /help and /keys,
  errors, windows opened and closed). It is a filter applied when
  window 0 is drawn, so nothing is lost: `/feed on` shows this
  session's messages again, those that arrived while it was off
  included. Chat windows, unread counts, Act, windows opening on a new
  message and `/search` are unchanged. On by default (state key `feed`,
  absent or unreadable means on), kept, takes effect at once; a start
  notice in window 0 says when it is off; window 0's scroll offset is
  clamped when the setting changes its line count.
- `/settings [text]`, also `/set` and `/options` (internal/tui/settings.go):
  a menu of every kept setting, opened in window 0 from any window, as
  `/help` and `/keys` focus it. Rows in order: feed, autojoin,
  read-receipts, names, theme, fx, mouse, clip; name, value and
  description in aligned columns (the value column fits the longest
  value any row can take). Enter or Right sets the next value, Left the
  previous one, wrapping; a click sets the clicked row's next value; the
  menu stays open on the row. Each change runs the command's own code:
  the handlers of /names, /fx and /mouse moved into setNames, setFx and
  setMouse, shared by the commands and the menu. Typed text narrows the
  rows; Tab completes setting names. Esc closes the menu and leaves
  window 0 current; any switch of window closes it. After a change the
  rule shows that change's status message until the next key.
- `/help`: the window 0 note says `/feed off` leaves only the notices;
  the mouse note says a click on a settings row sets its value. Start
  hints for `/feed off` and `/settings`.
- specs/ui.md: Windows, Modes, the command table and short forms, a
  Settings menu section with its decisions, Layout, Mouse, Input area
  prompts, Keys and Completion; docs/usage.md: commands and keys.
- Tests: internal/tui/feed_test.go and settings_test.go, including a
  test that fails when a kept setting key has no menu row; each new
  assertion was checked against a mutant that breaks it.
- Built in a worktree by one implementing agent, reviewed for
  correctness, spec conformance and test strength, each finding
  verified by a second agent: 9 confirmed and fixed, 4 refuted (they
  describe behaviour /list and /win already have).

## Decisions

- The menu lives in window 0: settings belong to the client, not to a
  chat, and window 0 is where /help and /keys put the client's text.
- Values step in place instead of a button strip: the theme has a dozen
  values, which would not fit a row. Enter keeps the menu open, since
  changing a setting is not leaving it.
- `/feed` is on by default, which keeps the behaviour before it.

## Open

- Text typed in a list's filter or the pager stays in the input when
  another window is switched to (todo/todo.md).

## Files touched

- internal/tui/feed.go, feed_test.go, settings.go, settings_test.go,
  model.go, input.go, hints.go; internal/command/command.go,
  command_test.go; specs/ui.md, docs/usage.md, todo/todo.md, this note

## Verify

- `make check`, `make smoke`, `make fuzz FUZZTIME=5s`
- `lazygram -data <scratch dir> demo`, then `/settings`, `/feed off`

## Next

- Merge, then release v0.4.24.
