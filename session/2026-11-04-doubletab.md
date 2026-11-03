# 2026-11-04: /unread, the double Tab as a setting, /settings only

## Done

- `/unread` (internal/tui): focuses the next window with unread messages
  after the current one, in window order, wrapping round; window 0 is
  never a target; the current window only when it alone is marked
  unread. It focuses as `/win N` does (receipts, the last window,
  restored counts). With none: `no unread window`, nothing changes. Tab
  completes it, `/help` lists it, no aliases.
- `/doubletab [off|last|unread|<ms>]` (internal/tui/doubletab.go), kept
  under `doubletab` (the word) and `doubletab-time` (ms); absent,
  unreadable or another value means off and 400 ms:
  - off (the default): every empty-prompt Tab goes to the next window,
    so fast Tabs walk the windows;
  - last: a second Tab within the time goes to the last window, as
    before (and as `/win last`);
  - unread: it goes where `/unread` would from the window before the
    first Tab; the window the first Tab passed through gets its unread
    count back first, so it can be the target; with nothing unread the
    second Tab is a plain Tab;
  - a time of 150 to 1000 ms (`300`, `300ms`, `300 ms`) replaces the
    fixed 400 ms everywhere: the double Tab, the held read receipt and
    the restored counts of a window a Tab only passed through, which
    stay protected in every mode;
  - anything else (`on` included) is an error and keeps nothing.
- `/settings` has two more rows (11): `doubletab` (off, last, unread)
  and `doubletab-time` (200 to 800 ms; a time set by the command off the
  list shows as it is and steps to the next listed one).
- `/set` and `/options` are gone; `/settings` is the only name of the
  menu, and they are unknown commands like any other.
- `/help`, `/keys`, two start hints, specs/ui.md (Windows, the `/win
  last` row, Keys, Completion, Read receipts, Settings menu, commands
  table, hints), specs/overview.md and docs/usage.md describe it, with
  Decision lines.
- Built by one implementing agent (it took over the uncommitted work of
  an earlier on/off version), reviewed for correctness, spec conformance
  and test strength, each finding verified: 2 confirmed and fixed (a huge
  `/doubletab` number wrapped round into the range and was kept; the spec
  said fast Tabs never walk in unread mode, but they do with nothing
  unread), 3 refuted; two of those, `/help` texts no test pinned, got a
  test anyway. Every new test was checked against a mutant.
- Checked in the real demo: `/unread`, the three modes, a custom time in
  `/settings`, `/set` unknown.

## Decisions

- The double Tab is off by default: it changes what fast Tabs do
  (walking two windows), so it is opt-in; last and unread are both a
  jump one may want under the hand.
- The pass-through protection stays on in every mode: a window flashed
  for a moment was not read.
- One name for the settings menu, so the command list has one way to it.

## Open

- `[unread]` on window 0's row in `/win` gives window 0 an unread mark
  that `/unread` never visits (older behaviour of that button).

## Files touched

- internal/tui/doubletab.go, doubletab_test.go, unread_test.go,
  lastwin.go, lastwin_test.go, model.go, model_test.go, settings.go,
  settings_test.go, hints.go, receipts.go, vim_test.go;
  internal/command/command.go, command_test.go; specs/ui.md,
  specs/overview.md, docs/usage.md, todo/todo.md, this note

## Verify

- `make check`, `go test -race ./internal/tui/ ./internal/command/`
- `lazygram -data <scratch dir> demo`: `/unread`; `/doubletab unread`,
  then Tab Tab fast; `/settings` shows `doubletab` and `doubletab-time`;
  `/set` is unknown

## Next

- Merge; v0.4.27 carries it.

Security note: the two new state keys hold a word and a number; a window
a Tab only passes through still sends no read receipt in any mode.
