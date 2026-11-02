# 2026-11-03: /help and /keys follow the window width

## Done

- Feedback: the help text was too narrow for wide terminals, and /keys
  printed its notes after the key table, so the table scrolled off the
  top of the window and had to be scrolled back to.
- `command.HelpText` and `keyHelp` hold one logical line per entry (two
  spaces, the names, padding to column 25, the whole description) and
  notes as single paragraphs. `refEntry` and `refRows` (render.go) lay
  them out when drawn: descriptions wrap with a hanging indent at their
  column; with under 20 columns beside the names the description goes on
  the rows below, indented 4; other lines wrap as paragraphs. Layout
  happens at render time, so the text re-flows on resize.
- /keys prints "Mouse, themes and windows" (the notes) first and the
  "Keys" table last; /help keeps its order and still ends by naming
  /keys.
- /about: "thanks to @zechkata and @pau4o for their feedback" after the
  author line, and its last line names /keys next to /help.
- Tests: TestHelpTextIsOneLinePerEntry (replaces the 70-column check),
  TestReferenceFollowsTheWidth (40, 80, 160 columns), TestKeysEndsWithTheKeyTable,
  TestHelpReflowsOnResize, FuzzRefRows; all fail on the old code.
- specs/ui.md and docs/usage.md describe the layout with the decision.

## Open

- Each 28 ms slow-print step now delivers one logical line, so a long
  entry appears several rows at once (written in the spec).
- In a window under 49 columns the start banner is still wider than the
  window (older behaviour).

## Files touched

- internal/command/command.go, internal/command/command_test.go,
  internal/tui/render.go, internal/tui/render_test.go,
  internal/tui/model.go, internal/tui/model_test.go,
  internal/tui/fuzz_test.go, specs/ui.md, docs/usage.md, todo/todo.md,
  this note

## Verify

- `make check`; `lazygram demo`, then F1 and /keys at different
  terminal widths, and resize while the text is shown
