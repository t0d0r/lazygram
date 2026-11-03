# 2026-11-03: complete and resolve chats by title, in either script

## Done

- Symptom fixed: with `/names` showing names, the window bar shows a
  chat by its title, but Tab matched only aliases, so a person whose
  alias is an unrelated username could not be completed by the name on
  the screen, and the second Tab listed aliases. Separately, a Latin
  spelling was weaker than the Cyrillic one: with chats aliased in
  Cyrillic as ivan and ivanka, `/win ivan` was ambiguous while the
  Cyrillic spelling resolved.
- Completion (internal/tui/model.go: chatBegins, chatCompletions,
  argStart, shownTitle; members.go: nameStart shared with members): a
  chat is a candidate when the typed word begins its alias or its shown
  title, in either script, in both `/names` modes; a typed sigil must be
  the chat's own; deleted accounts are skipped. Tab fills in the name as
  `/names` shows it; when that name would not resolve to the same chat
  (a title two chats share, another chat's alias, control characters
  dropped when shown) the chat is offered by its alias. The second Tab
  lists names as they would be filled in, `", "` between them whenever
  one shows a title; an alias offered because of its title shows the
  title in brackets. A title may span words: the first argument of
  `/win`, `/msg`, `/whois` and `/list` is one name when all of it begins
  a chat; in text the name starts as for members.
- domain.ResolveChat tiers, the first with any hit deciding: exact id or
  alias; exact title (new); alias prefix; alias or title substring; then
  in either script exact alias or title (new), alias or title prefix
  (new), substring. Matcher.MatchExact and MatchPrefix, no allocation.
- `/msg`: the target is the longest leading part that is a chat's shown
  name, its sigil-and-title form or its title, compared in either
  script; otherwise the first word. An argument that is all a chat's
  name is the usage error, so nothing is sent.
- specs/ui.md (Names: nick or title, Search in either script,
  Completion, the `/msg` row, Decisions), docs/usage.md, the `/msg` help
  line.
- Tests with Cyrillic data in internal/domain and internal/tui
  (names_test.go): the observed scenario, both modes, the fallback to
  aliases, a property test that every completed name resolves to its
  own chat, `/msg` with spaces and the longest name, the tier order;
  FuzzResolveChat gained properties and seeds. Each new assertion was
  checked against a mutant.
- Built in a worktree by one implementing agent, reviewed for
  correctness, spec conformance and test strength, each finding
  verified by a second agent: 8 confirmed and fixed (the main one:
  `/msg` with a Latin spelling of a multi-word Cyrillic title went to
  the shorter name), 3 refuted.

## Decisions

- Completion offers what the window bar shows, so a name read on the
  screen can be typed back.
- The exact and prefix tiers in either script make a Latin spelling as
  decisive as the Cyrillic one; they come after the plain tiers, so they
  never make a plain match ambiguous. The exact-title tier comes before
  the alias prefix, so a name as shown wins over an alias that merely
  begins with it; that is the one case where a name that resolved
  before now finds another chat.
- An alias fallback keeps the invariant that every completed name
  resolves to the chat it was offered for.

## Open

- Name mode resolves every candidate on each Tab: quadratic in the chat
  count, fine at the provider's 100 dialogs (todo/todo.md).

## Files touched

- internal/domain/domain.go, translit.go, domain_test.go,
  translit_test.go, fuzz_test.go; internal/tui/model.go, members.go,
  names_test.go; internal/command/command.go; specs/ui.md,
  docs/usage.md, todo/todo.md, this note

## Verify

- `make check`, `go test -race ./internal/...`, `make fuzz FUZZTIME=5s`
- `lazygram -data <scratch dir> demo`: `/names`, then `/win @` and Tab

## Next

- Merge; the next release carries it.
