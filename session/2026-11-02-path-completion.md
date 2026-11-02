# 2026-11-02: crash on path completion after ~

## Done

- Tab after `/send ~/` (or `/send ~`) crashed the TUI with "slice bounds
  out of range" in `pathCandidates`, in every release since v0.4.7.
  `expandHome("~/")` goes through `filepath.Join` and loses the trailing
  slash, so `filepath.Split` took the home directory's own name for the
  typed base name, longer than the text typed.
- Now the typed text is split first and only its directory part is
  expanded; a lone `~` completes to `~/`. `TestCompleteArgumentsAndPaths`
  covers `~`, `~/`, `~/n`, `~/s`, `~/sub/` (it panics on the old code);
  `FuzzPathCandidates` checks that completion never panics and keeps the
  typed directory part (30 s, 1.5 million inputs, nothing found).
- specs/ui.md, Completion, records the rule and the reason.

## Decisions

- Split what was typed, expand only the directory part: the shown
  candidates keep `~/` as typed, and nothing depends on how
  `filepath.Join` cleans the expanded path.

## Files touched

- internal/tui/model.go, internal/tui/model_test.go, specs/ui.md,
  todo/todo.md, this note

## Verify

- `make check`; `go test ./internal/tui/ -run TestCompleteArgumentsAndPaths`
- `go test ./internal/tui/ -run '^$' -fuzz FuzzPathCandidates -fuzztime 30s`
- in `lazygram demo`: type `/send ~/` and press Tab twice; the home
  directory's entries are listed, nothing crashes

## Next

- Ship it in v0.4.22, the first release built by the Release workflow
  (session/2026-11-02-scorecard-omarchy-pr.md).
