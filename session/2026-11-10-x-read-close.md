# 2026-11-10: /x reports a chat read, then closes its window

## Outcome

`/x` is implemented, tested and committed locally on the branch
`feature/x-mark-read-close` (not pushed, no pull request, no release).
No blocker. One item is open: a check against live Telegram (todo).

## Done

- `/x` is `/mark-as-read`, then `/close`, for the current chat window
  (internal/tui/readclose.go):
  - The report is `/mark-as-read`'s: one receipt for the newest message
    held of the chat, the unread mark cleared (also when the receipt
    fails), whatever `/read-receipts` says, which it leaves alone; the
    window's unread count is cleared when the command is typed; the
    request goes through the per-chat queue of requested reports.
    Status: `reporting @name read, then closing its window...`.
  - The window stays open until the answer. Receipt and mark both went
    through: the window closes through `closeWins`, the code of `/close`
    (a `window closed: <name>` notice in window 0, the current window
    kept or the nearest open one before it, nothing focused, the layout
    saved once); status `reported @name read, closed its window`. The
    window is found by its chat, so it closes also when another window
    is current by then.
  - The receipt or the mark failed: `read receipt for @name: <why>;
    window kept` or `unread mark for @name: <why>; window kept`, red and
    in window 0; nothing closes, no closing is noted, the layout is not
    written. Not retried.
  - The window was closed by hand meanwhile: `reported @name read; its
    window was closed already`; a failure then is the plain error.
  - A message newer than the one the receipt covered is held when the
    answer comes: `reported @name read; a newer message arrived, window
    kept` (information).
  - Checks before anything is sent, in order, each sending and closing
    nothing: any argument, in any window: `/x takes no argument; it
    reports this chat window read and closes it`; window 0: `the server
    window has no chat; /win to a chat first`; no known message: `no
    message of #chat is known yet, nothing to report, window kept; /last
    fetches some, /close closes it unreported`.
  - A `/x` typed while a requested report for the chat is on its way
    waits and closes after its own answer. A later `/mark-as-read` or
    `[read]` replacing a waiting `/x` keeps the closing; a later
    `[unread]` drops it.
- `readMsg` carries `closing` and the id reported; `pendingReport`
  carries `closing`; `report` takes it (internal/tui/receipts.go). The
  `readMsg` case of `Update` appends `; window kept` to a failure and
  closes on a clean answer (internal/tui/model.go).
- Parser: `command.ReadClose`, one spelling `/x` (`/xx` is unknown).
- `/help`: a `/x` line right after `/mark-as-read`. Tab completes `/x`
  to `/x ` and offers nothing after it. Start hint: `hint: /x in a chat
  window reports it read, then closes the window`; `/x` is in the test
  list of commands window 0 refuses.
- specs/ui.md (Read receipts, new section Read and close: /x, with
  Decisions; commands table; short forms; Completion; hint lists),
  specs/security.md, specs/providers.md, docs/usage.md, todo/todo.md.

## Decisions

- The receipt first, the close on its answer, not both at once: the
  receipt is asynchronous (up to 30 s), and closing at once would leave
  a failed report as a line about a window no longer on screen. A closed
  window always means the service took the report; a failure leaves the
  chat where `/x` can be typed again.
- A failed unread mark keeps the window as a failed receipt does: the
  chat would still show as unread elsewhere.
- The current window only, no argument: `/x 3` would send a receipt,
  visible to the other side of a private chat, for a chat not on
  screen. The argument is checked first, as `/close-others` does.
- No closing without a report: with no known message `/x` refuses and
  names `/close`, which closes unreported.
- A message that arrived after `/x` keeps the window: the receipt does
  not cover it, and closing would hide a message never reported.
- A later `[unread]` drops a waiting `/x`'s closing: a chat just marked
  unread is not one to put away.
- The closing is `closeWins`, so `/close` and `/x` cannot drift apart.

## Open

- A check against live Telegram, by the operator, on an account of
  their choosing (todo): no receipt was sent to any real account. The
  TUI is tested with the fake provider and the offline demo; the
  Telegram calls are the ones `/mark-as-read` already uses, unchanged.
- The demo's `MarkRead` always succeeds, so the failure paths are shown
  by the tests only, not in the demo.

## Files touched

- internal/tui/readclose.go (new), readclose_test.go (new), receipts.go,
  listactions.go, model.go, hints.go, hints_test.go
- internal/command/command.go, command_test.go
- specs/ui.md, specs/security.md, specs/providers.md, docs/usage.md,
  todo/todo.md, this note

## Verify

- `make check`: gofmt clean, `go vet`, golangci-lint `0 issues`, `sh -n`
  and shellcheck on install.sh, `go test ./...` (13 packages ok).
- `go test -race -count=1 ./internal/tui/ ./internal/command/`: ok.
- Demo only (a build in a scratch folder, `lazygram -data <scratch dir>
  demo` in tmux): `/x` in window 0 gave the chat-window error; `/x 2`,
  `/x 0` and `/x @linus` the argument error; in `#dev`, `/x` said
  `reported #dev read, closed its window`, the bar lost `#dev` and
  window 0 got `window closed: #dev`; again in `#news` the same;
  `/mark-as-read` and `/close` in window 0 gave their own errors as
  before.

### TDD record (RED, then GREEN)

1. Parser: `go test ./internal/command/`: RED, build failed, `undefined:
   ReadClose`. GREEN after the kind and the table entry.
2. Help: RED, `TestHelpTextCoversEveryName: /x is accepted by the parser
   but missing from HelpText`. GREEN after the HelpText line.
3. TUI: `go test ./internal/tui/ -run 'TestX|...'`: RED, every `/x`
   test failed because the command did nothing (`before the answer: cmd
   false`, `one receipt, for @alice: []`, the window 0 and argument
   errors missing), and `TestHintNeedsChatHolds: "/x" in window 0: ""`.
   GREEN after readclose.go, the `closing` flag and the dispatch.
4. Hint: `TestHintNamesX`: RED, `no start hint names /x`. GREEN after
   the hint.
5. Completion (`TestTabCompletesX`): green at first run, since
   completion reads the parser table.

Mutants, each caught: closing whatever the answer
(`TestXFailedReportKeepsTheWindow`); closing before the answer
(`TestXReportsThenCloses`); window 0 checked before the argument
(`TestXTakesNoArgument`); no newer-message check
(`TestXKeepsAWindowWithANewerMessage`); a replaced waiting `/x` losing
its closing (`TestXWaitsItsTurn`); the unread count not cleared
(`TestXReportsThenCloses`). One mutant was equivalent (an `[unread]`
inheriting the flag: its answer never closes), so the condition was
reduced to the part that matters. Every existing `/mark-as-read`,
`/close` and `/close-others` test passes unchanged.

## Next

- Review the branch `feature/x-mark-read-close`, then push it and open
  a pull request to `master` (the ruleset needs one and a passing
  `Linux` check); a release carries it after the merge.

## Security note

A read receipt is visible to the other side of a private chat and marks
the chat read on every device of the account. `/x` sends exactly one,
for the chat on screen, only on that explicit command; it takes no
argument, so it cannot report a chat that is not shown. A failed report
is never shown as success: the window stays and the error is red and
kept in window 0. Tests used a fake provider and the offline demo with
a scratch state folder; nothing was sent to a real account, and no
credentials or personal data are in these files.
