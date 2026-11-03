# 2026-11-03: a selection stays highlighted after the copy

## Done

- A released mouse selection (a drag, a double-click word, a
  triple-click row, a Shift-click extension) stays in reverse video
  after the copy, like a terminal's, in every `/clip` mode (`off`
  included) and also after "nothing to copy" or a clipboard error. A
  plain click keeps nothing, and neither does a word or row whose
  screen changed between press and release.
- It goes, for good, at the next left-button press of any kind; when a
  row it covers no longer reads as it did at the copy (plain text,
  styling ignored: a scroll, a new message, a window switch, a list or
  menu opening, typing into a covered input row, a status change on a
  covered status row); when a cursor drawn only in colour moves into,
  out of or along a covered row (the list cursor and its chosen button,
  the input cursor, the active window's tag), since the highlight would
  hide it; on a resize; and on `/mouse off`, also from the settings
  menu. Hover motion, the wheel without a scroll, right and middle
  presses and typing on other rows leave it.
- Model: `kept` and `keptRows` (a rowLook per covered row: plain text
  plus the colour-only cursor View records per row in `cursors`);
  `keep` on release, `keptSel` in View, `dropKept` on a press, a resize
  and `setMouse(false)`.
- specs/ui.md, Selection and clipboard: the rule, when a selection over
  the rule row stays (lists, the pager and a repeated status keep the
  rule's text), and a Decision; the keys table, `/help` notes,
  docs/usage.md and docs/clipboard.md.
- Tests in internal/tui/clipboard_test.go and selection_test.go for
  every keep and drop case; each new assertion was checked against a
  mutant that breaks it (33 mutants).
- Built in a worktree by one implementing agent, reviewed for
  correctness, spec conformance and test strength, each finding
  verified by a second agent: 9 confirmed and fixed (cursors hidden
  under a kept highlight, the spec's rule-row wording, six test gaps),
  5 refuted.

## Decisions

- The selection is in screen cells and cannot follow the text, so it
  goes as soon as the text under it changes rather than mark text that
  was not copied.
- A cursor shown only in colour counts as part of a row: a highlight
  that hid the selected settings row would let Enter change a setting
  that did not look selected.

## Open

- A drag that ends on the left half of a wide character copies that
  character but highlights one character less (older than this change;
  todo/todo.md).

## Files touched

- internal/tui/clipboard.go, clipboard_test.go, selection.go,
  selection_test.go, model.go, settings.go; specs/ui.md, docs/usage.md,
  docs/clipboard.md, todo/todo.md, this note

## Verify

- `make check`, `go test -race ./internal/tui/`
- `lazygram -data <scratch dir> demo`: drag over a line, release; the
  highlight stays until the next click or a scroll

## Next

- Merge; the next release carries it.
