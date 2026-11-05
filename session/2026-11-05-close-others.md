# 2026-11-05: /close-others

## Done

- `/close-others` closes every chat window except the current one, at
  once (internal/tui/close.go, `closeOthers`). It collects the numbers
  of all the other chat windows and hands them to `closeWins`, the code
  `/close 2,3-5` uses, so it behaves as `/close` with those numbers:
  - each closed window gets `window closed: <name>` in window 0, in
    window order, none in the window kept;
  - the status is `closed <name>` for one window, `closed N windows:
    1:#news, 3:@alice` for several (numbers as before the command), cut
    after four as `+N more`;
  - the current window stays current, as window 1; window 0 stays;
  - nothing is focused: no read receipt (with `/read-receipts on` too),
    no unread count changes, the kept window's included;
  - the last focused window goes when it was a chat window (window 0 as
    the last window stays);
  - the layout is saved once.
- Cases that close nothing, checked in this order; on each nothing
  changes (windows, current and last window, notices, unread counts)
  and the stored layout is not written:
  1. any argument, in any window: `/close-others takes no argument;
     /close 2,3-5 closes the windows given` (error); spaces alone are
     no argument;
  2. window 0, with chat windows open or none: `the server window has
     no chat; /win to a chat first` (error, the text every command that
     needs a chat window gives);
  3. the current window is the only chat window: `no other chat window
     to close` (information, not red).
- Parser: `command.CloseOthers`, one spelling `/close-others`, no short
  form (`/closeothers`, `/close-other` are unknown commands).
- `/help`: `/close-others  close every chat window but this one; window
  0 stays; refused in window 0; takes no argument`, right after `/close`.
- Start hint added to the pool: `hint: /close-others in a chat window
  closes every other chat window`; `/close-others` is in the test list
  of commands window 0 refuses (hintNeedsChat).
- Tab: `/close-o` completes to `/close-others `; nothing is offered
  after it. `/close` Tab now stops at `/close`, the prefix it shares
  with `/close-others` (as `/clip` beside `/clipboard`); a second Tab
  lists both. `/close 2` and its other forms are unchanged.
- specs/ui.md (Windows bullet with Decision lines, start hint lists,
  commands table, short forms, Completion), docs/usage.md (commands
  table), todo/todo.md.

## Decisions

- The window kept is the current chat window, so window 0, which has
  none to keep, refuses instead of closing every chat window (`/close
  1-N` does that). It uses the shared chat-window error, as `/send`,
  `/mark-as-read` and `/edit` do, so window 0's refusals read alike.
- An argument is refused, not ignored: `/close-others 3` could mean
  keep 3 or close 3, so it closes nothing and the error points to
  `/close` for given windows. The argument is checked first, as `/close`
  checks the form before any number.
- Nothing to close is information, not an error: what the command asks
  for already holds. Nothing changed, so the layout is not written.
- Closing goes through `closeWins` instead of a second close path, so
  the notices, status, current window, receipts and single save of the
  two commands cannot drift apart.
- One spelling, no alias, like the other recent commands (`/settings`,
  `/unread`, `/doubletab`).

## Open

- Nothing blocking. As after `/close`, a closed chat's window reopens
  when a new message arrives for it (auto-open); seen in the demo.

## Files touched

- internal/command/command.go, command_test.go
- internal/tui/close.go, close_test.go, model.go, hints.go, hints_test.go
- specs/ui.md, docs/usage.md, todo/todo.md, this note

## Verify

- `make check`: gofmt clean, `go vet`, golangci-lint `0 issues`,
  `sh -n` and shellcheck on install.sh, `go test ./...` all `ok`.
- `go test -race -count=1 ./internal/tui/ ./internal/command/`: `ok`.
- Demo: `make build`, then `./lazygram -data <scratch dir> demo`. In
  window 0 `/close-others` gives the chat-window error; `/close-others 2`
  the argument error; from `@linus` it said `closed 2 windows: 1:#ops,
  2:#dev` and left `[0:server] [1:@linus]`; again at once, `no other
  chat window to close`; `/close 1,9` still says `no window 9`.

### TDD record (RED, then GREEN)

Each slice: test first, run, the expected failure, minimal code, green.

1. Parser. `go test ./internal/command/ -run 'TestParse$'`: RED, build
   failed, `undefined: CloseOthers`. GREEN after the kind and the table
   entry.
2. Help. `go test ./internal/command/`: RED, `TestHelpTextCoversEveryName:
   /close-others is accepted by the parser but missing from HelpText`;
   `go test ./internal/tui/ -run TestHelpNamesCloseOthers`: RED, `/help
   has no /close-others line right after /close: at -1`. GREEN after the
   HelpText entry.
3. Normal path (`TestCloseOthersKeepsOnlyTheCurrentChatWindow`,
   `NamesWhatClosed`, `FocusesNothing`, `DropsTheLastWindow`,
   `SavesTheLayout`). `go test ./internal/tui/ -run TestCloseOthers`:
   RED, 5 failures, nothing closed (`left "#news #dev @alice #ops #qa
   @bob #web"`): the TUI had no case for the kind. GREEN after
   `closeOthers` and its `run` case.
4. No-op (`TestCloseOthersAloneIsANoOp`, with a layout planted in the
   store so any write shows): RED, `panic: runtime error: index out of
   range [0] with length 0` in `closeWins`. GREEN after the
   `no other chat window to close` branch.
5. Window 0 (`TestCloseOthersRefusedInWindowZero`, `TestHintNeedsChatHolds`):
   RED, `"closed 7 windows: 1:#news, 2:#dev, 3:@alice, 4:#ops, +3 more"
   (error false)`. GREEN after the window 0 check.
6. Argument (`TestCloseOthersTakesNoArgument`): RED, `"/close-others 3"
   in window 4: "closed 6 windows: ..."` and `wrote the stored layout`.
   GREEN after the argument check.
7. Completion (`TestTabCompletesCloseOthers`): green at first run, since
   completion reads the parser table; with the table entry removed it
   fails (`"/close-o": got "/close-o", want "/close-others "`).
8. Hint (`TestHintNamesCloseOthers`): RED, `no start hint names
   /close-others`. GREEN after the hint; `TestHintsAreWellFormed`
   checks its length, ASCII and "chat window".

Mutants of close.go, each caught: the no-op writes the layout
(`AloneIsANoOp`), the no-op as an error (`AloneIsANoOp`), window 0
checked before the argument (`TakesNoArgument`), a notice on the
window 0 refusal (`RefusedInWindowZero`). Every existing `/close` test
(`TestCloseSpec`, `TestCloseTakesListsAndRanges`, `TestCloseIsAllOrNothing`
and the rest) passes unchanged.

## Next

- Push the branch `feature/close-others` and open a pull request to
  `master` (the ruleset needs one and a passing `Linux` check); after
  the merge, a release carries it.

## Security note

- No credentials, personal data or network calls are involved: the
  command only changes the local window layout in the encrypted cache.
  Closing focuses nothing, so it never sends a read receipt to the
  service. The demo ran with a scratch state folder, not a linked
  account.
