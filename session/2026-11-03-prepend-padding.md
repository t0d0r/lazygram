# 2026-11-03: padding after a prepend mark

## Done

- Dependabot's patch update of charmbracelet/x/ansi 0.10.1 -> 0.10.4
  (#9, merged) brings the clipperhouse/displaywidth grapheme widths.
  With them an Arabic prepend mark (U+0600..U+0605, number signs such
  as U+0603) at the end of a row joins the first pad space into one
  grapheme cluster, so `fitLine` and `barRow` padded one round and left
  the row a column short. `make fuzz` on `master` found it
  (FuzzFitLine, input "؃" at width 2).
- Both now pad until the row measures the width; a space adds at most
  one column, so the loop never overshoots.
  `TestBarRowFillsAfterAPrependMark` covers both (fails 15 times on the
  old code); the fuzz input stays under
  `internal/tui/testdata/fuzz/FuzzFitLine/`.
- With the fix, FuzzFitLine ran 60 s and the other TUI targets 15 s
  each on ansi 0.10.4 without a failure; the truncate-versus-width
  mismatch seen with ansi 0.11.9 does not occur with 0.10.4.

## Decisions

- Fix forward rather than revert #9: the cause is in lazygram's padding,
  which assumed one space adds one column, and the fix holds for any
  width library.

## Files touched

- internal/tui/render.go, internal/tui/theme.go,
  internal/tui/bars_test.go,
  internal/tui/testdata/fuzz/FuzzFitLine/250ed96a1a5ce774, todo/todo.md,
  this note

## Verify

- `make check`; `go test ./internal/tui/ -run '^$' -fuzz '^FuzzFitLine$' -fuzztime 60s`

## Next

- Run `make fuzz` after each Dependabot module update that touches
  charmbracelet or clipperhouse; CI runs only the seeds.
