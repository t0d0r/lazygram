# 2026-11-04: /help notes as labelled entries

## Done

- The notes /help prints above the command reference (internal/tui
  `helpNotes`) were paragraphs at the left margin: each wrapped back to
  the margin, so where one note ended and the next began was hard to
  see beside the aligned command list. Each note is now an entry laid
  out as a command's: two spaces, a short label (Themes, Clickable text,
  Selection, Double click, Mouse, Window 0, Reactions, New windows,
  Names), the text from column 25, continuation rows hanging under the
  text (refRows, as for the commands). The wording is the same, cut into
  label and text, without full stops, as the command descriptions.
- specs/ui.md (Commands: the notes' layout, the paragraph rule now only
  for headings and the last line, a Decision); tests that quoted the old
  sentences follow the new text; TestHelpNotesAreEntries pins that every
  note is an entry at column 25 and wraps under its text.
- Checked in the demo: `/help` shows the labels in the names column.

## Files touched

- internal/tui/model.go, model_test.go, render_test.go,
  reactions_test.go, settings_test.go; specs/ui.md; todo/todo.md; this
  note

## Verify

- `make check`; `lazygram -data <scratch dir> demo`, then `/help` and
  PgUp: the notes under `Mouse, themes and windows` line up like the
  commands

## Next

- Merge; the next release carries it.
