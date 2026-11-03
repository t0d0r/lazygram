# 2026-11-03: the mouse, theme and window notes move to /help

## Done

- The notes under `Mouse, themes and windows` (themes, the running
  light, selection and the clipboard, clicks, window 0, when windows
  open, `@` and `#` names) are printed by `/help` and F1 now, before the
  command reference; `/keys` prints the Keys table alone.
- `/help` ends with the command reference and its `/keys` line, so that
  is what the window shows when printing ends; the notes are one scroll
  up, as the notes were in `/keys` before.
- The `/help` entry in the command reference and the F1 row of the key
  table say that the notes come first.
- Decision (specs/ui.md, Commands): the notes describe using the client,
  not keys, and `/help` is where a new user looks first.

## Open

- The `feat/feed-settings` branch (`/feed`, `/settings`) changes the
  window 0 note once `/feed` exists ("every chat interleaved" holds
  only with `/feed on`); whichever merges second updates the note.

## Files touched

- internal/tui/model.go (`helpNotes`, `keyHelp`, `help()`),
  internal/command/command.go (the `/help` entry),
  internal/tui/model_test.go, internal/tui/render_test.go,
  specs/ui.md, docs/usage.md, todo/todo.md, this note

## Verify

- `make check`; `make run` or `lazygram demo`, then `/help` and `/keys`

## Next

- Merge; then bring `feat/feed-settings` up to date with `master`.
