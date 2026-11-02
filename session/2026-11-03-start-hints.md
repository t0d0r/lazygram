# 2026-11-03: hints at start

## Done

- Every start adds a block of `hint:` notices to window 0, under the
  tagline and the /help line: first "/list shows the chats, @people and
  #channels; Enter opens one" and "/win lists the open windows; /win N
  or Alt-N switches to one", then two drawn at random without repeats
  from a pool of 21 (internal/tui/hints.go).
- They are added in New(), so they roll in with the start-up reveal;
  later chat lists, polls and /refresh never print them again, /redraw
  drops them like other notices.
- The draw goes through `Options.Rand func(n int) int` (math/rand/v2
  `IntN` when nil), so tests choose the pair.
- The start line "/help lists commands, /list the chats" became
  "/help lists commands, /keys the shortcuts", since /list has its own
  hint.
- Review fixes: the "/mouse off" hint was false (/mouse toggles and
  ignores its argument); hints that said "this chat" for actions window 0
  refuses now say "in a chat window". TestHintsAreWellFormed checks the
  wording rules (setting words a command does not take, "this chat",
  chat-only commands without "chat window"); TestHintNeedsChatHolds
  runs each chat-only command in window 0 and expects the refusal.
- specs/ui.md (Windows) and docs/usage.md describe the hints with the
  decision: a new user's first steps are a chat and a window; the rest
  is found one tip at a time.

## Open

- docs/screenshot.svg is an old capture and does not show the hints.
- TestAmbiguousThemePrefixIsExplained can be flaky depending on test
  order (already on master).

## Files touched

- internal/tui/hints.go, internal/tui/hints_test.go,
  internal/tui/model.go, internal/tui/model_test.go, specs/ui.md,
  docs/usage.md, todo/todo.md, this note

## Verify

- `make check`; `lazygram demo` a few times: the two fixed hints, then
  two different ones each start
