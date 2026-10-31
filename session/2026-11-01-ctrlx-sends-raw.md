# 2026-11-01: Ctrl-X sends slash lines as text

Branch `feature/ctrlx-sends-raw` in the worktree `../lazygram-ctrlx-send`,
not committed.

## Done

- Ctrl-X sends the typed line as it is (surrounding whitespace trimmed),
  a line that starts with `/` included; it never reaches the command
  parser. `sendable` lost its "looks like a command" refusal. Enter is
  unchanged: a slash line runs as a command in both modes, plain text
  gets a line break (Ctrl-X sends) or is sent (Enter sends).
- With "Enter sends" on, Ctrl-X still sends a slash line instead of
  adding a line break, because Enter would run it; the leading-space
  form ` /x` keeps the line break there, because Enter sends that form
  as text, so a message of several lines may start with a slash.
- In the list and the pager Ctrl-X now does nothing at all; before, it
  armed the 600 ms double-press window, so a Ctrl-X in the pager, Esc and
  a Ctrl-X in the window toggled "Enter sends" instead of sending.
- The status line names how such a line goes out: `sending to @name as
  text, not a command... (Ctrl-X again to keep the text and toggle)` on
  the held path, the same without the hint when the hold ends and on the
  immediate path (Enter with a leading space when Enter sends).
- Regression tests: `TestCtrlXSendsInChatWindowOnly` expects `/win 0` to
  be sent, not run; `TestCtrlXSendsSlashTextEnterRunsIt` covers both
  modes, the status on every path, Enter running the command, the
  leading-space form with Enter and with Ctrl-X, plain text still getting
  a line break, and the server window refusing;
  `TestRecalledSlashLineSendsWithCtrlXRunsWithEnter` and
  `TestCtrlXDoesNothingInListAndPager` as named;
  `TestCtrlXTrimsSurroundingWhitespace` replaces the leading-space test.
- Documentation: `/keys` text, README (prose, keys table), specs/ui.md
  (Sending, Input history, keys table), specs/security.md (what Ctrl-X
  may send and the way back).

## Decisions

- Ctrl-X is the one key that always means "send": a line that starts
  with `/` goes out as text and never reaches the command parser; Enter
  runs commands, in both send modes. Reason: a message may start with a
  slash without the leading-space trick, and a wrong key never runs a
  command. Cost: a command typed and then confirmed with Ctrl-X leaves
  as a message; the 600 ms hold, the `as text, not a command` status and
  a second Ctrl-X cover that. Recorded in specs/ui.md (Sending).
- A sent slash line stays in the history as typed; recalled, Enter runs
  it. Documented rather than special-cased: the history holds entered
  lines, and the next key decides (specs/ui.md, Input history).

## Released

- Commit `bfd45af` on `feature/ctrlx-sends-raw`, fast-forwarded into
  `master` after a clean sync with `origin/master`; `make check` and
  `make smoke` green; Linux and macOS CI green on that commit.
- Tag `v0.4.17` at `bfd45af`, pushed; `make dist` and `make release`
  published the GitHub release (four tarballs and SHA256SUMS). Read back:
  not a draft or prerelease, marked latest; every asset's digest on
  GitHub equals the local build, the downloaded SHA256SUMS is identical
  to `dist/SHA256SUMS`, `shasum -c` passes for all four tarballs, the
  downloaded darwin-arm64 binary prints `lazygram v0.4.17`.

## Open

- The Homebrew formula still builds 0.4.14; the Arch/Omarchy recipe
  still names 0.4.14 (packaging/omarchy/README.md, step 0).
- The worktree `../lazygram-ctrlx-send` and the merged branch can be
  removed.

## Files touched

- internal/tui/model.go: the Ctrl-X case in the key handler, `sendable`,
  `asText`, `send`, `holdSend`, the `sendLaterMsg` handler, `keyHelp`
- internal/tui/model_test.go
- README.md, specs/ui.md, specs/security.md, todo/todo.md, this note

## Verify

`make check` in the worktree.

## Next

- Bump the Homebrew formula and the Arch/Omarchy recipe to 0.4.17.
