# 2026-11-03: multi-line input area

## Done

- Feedback: Enter should visibly start a new line in the input instead
  of showing a return mark (U+21B5) until the message is sent.
- The single input line is replaced by an input area
  (internal/tui/input.go): each line of the input is a row; later rows
  are indented to the prompt's width; a long line wraps at grapheme
  cluster boundaries (wide characters and emoji never split); the cursor
  is a reverse cell on its row.
- The area grows upward one row per input row, up to 8 rows and at most
  a third of the screen; past that it scrolls to keep the cursor's row
  shown. The message area gives up the same rows; rule, status line and
  window bar stay right above the input. With one row the screen is as
  before.
- `contentHeight()` and View use the real input height and `m.clicks`
  gets a row per input row, so clicks, hover lights, drags and double or
  triple clicks hit the right row with a tall input.
- Keys: Up/Down move between input rows keeping the column, and walk the
  history from the first / last row; Ctrl-A / Ctrl-E go to the start /
  end of the cursor's line; Ctrl-K deletes to the end of the line. Down
  with no history walk under way leaves the input alone (it used to
  replace a draft with a stale saved line).
- Merged with the full-width bottom rows (#10): every input row is on
  the status background and padded to the width.
- Tests in internal/tui/input_test.go (layout, cap and scrolling,
  wrapping, wide characters, cursor, Up/Down, paste, frame height, clicks
  and selection with a three-row input); specs/ui.md has an Input area
  section with the decision; docs/usage.md updated.

## Open (decisions)

- Ctrl-W does not treat a line break as a space, so it can delete across
  one.
- Ctrl-K at the end of a line deletes nothing; Emacs and zsh join the
  next line.
- Wrapping is per character, not per word.
- When the area has scrolled, nothing marks the hidden rows.
- On screens under 5 rows the mouse still misses (older behaviour,
  written in the spec).

## Files touched

- internal/tui/input.go, internal/tui/input_test.go,
  internal/tui/model.go, internal/tui/model_test.go,
  internal/tui/queued_test.go, internal/tui/bars_test.go, specs/ui.md,
  docs/usage.md, todo/todo.md, this note

## Verify

- `make check`; `lazygram demo`, open a chat window, type a few lines
  with Enter, paste a multi-line text, use Up/Down and the mouse
