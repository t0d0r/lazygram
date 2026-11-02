# 2026-11-02: bottom rows on the status background

## Done

- The status line, the window bar and the prompt row are painted on the
  theme's status background across the whole window width, in every
  theme. Each segment of those rows ends with a reset, so a background
  set on the whole row stopped after the last segment and the padding
  showed the terminal's own colours; `barRow` (internal/tui/theme.go)
  now paints the padding itself.
- The prompt row is on the status background too: the prompt in the dim
  colour, typed text in the status text colour, the cursor cell in the
  status colours reversed.
- The current window's tag in the bar is the status colours reversed
  instead of plain reverse video, so it uses the theme's colours.
- `TestBottomRowsAreOnTheStatusBackground` renders the three rows in
  every theme with true colour and requires the full width and a
  background on every cell; it fails on the old code (60 rows).
- specs/ui.md, Themes, describes it with the decision.

## Files touched

- internal/tui/theme.go, internal/tui/model.go,
  internal/tui/model_test.go, internal/tui/bars_test.go, go.mod
  (termenv becomes a direct dependency of the test), specs/ui.md,
  todo/todo.md, this note

## Verify

- `make check`; `go test ./internal/tui/ -run TestBottomRowsAreOnTheStatusBackground`
- `lazygram demo`, `/theme emacs` and the other themes: the three bottom
  rows are one block of colour from edge to edge

## Next

- The multi-line input (feat/multiline-input) rewrites `inputLine`; each
  of its rows has to go through `barRow` as well when the two meet.
