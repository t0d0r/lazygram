# 2026-11-01: queued todo items

Branch `feature/queued-todos` in the worktree `../lazygram-todos`, from
`master` at `e7044f8`, not committed; nothing pushed, tagged or released.

## Done

- Ctrl-L runs `/redraw` for the current window (`/keys`, README, spec).
- `/open <link>`: an http(s) link with a host goes to the browser like a
  click, from any window; anything else starting with `http` is refused
  with a message; `/open` without a link keeps its attachment meaning.
  `opener.IsLink` is the one rule: the opener enforces it for a click,
  `/open <link>` checks it first (`isWebLink`), and completion offers
  only links that pass it.
- Tab after `/open` offers the chat's ten most recent distinct links,
  newest message first (`recentLinks`), taken from the sanitised text a
  click uses (no control character reaches the input; trailing
  punctuation excluded), matched case-insensitively and filled in with
  their own case (the typed case wins between links differing only in
  case), beside `out` and the attachment ids; no links after `out` (only
  the ids), nothing in the server window. `/open` with more than one
  word after a link is refused, so no text with spaces reaches the
  opener.
- A pasted text keeps its line breaks in a window (CR LF as one; other
  control characters still dropped); in a list or the pager a pasted
  break is dropped.
- One process per state directory: `config.TryLock` holds an advisory
  flock on `<state>/lock` (Linux, macOS, the BSDs), taken by `lazygram`
  and `lazygram login` through `lockState`; a second start exits 2 with
  a message naming the directory and `-data <another directory>`. A
  filesystem without file locks (ENOLCK, ENOTSUP) or a system without
  flock is used with a warning; any other lock error stops with the lock
  file's path. The lock package builds on every Go platform (whole-repo
  builds for solaris, illumos, dragonfly and plan9 fail before and after,
  in `modernc.org/sqlite`).
- Search in either script: `domain.Translit` (Bulgarian streamlined
  system with the stressed i, plus the Russian and Ukrainian letters it
  lacks; combining marks dropped) and `domain.Matcher` (query
  transliterated once, every text into a reused buffer; exact because
  the mapping is rune by rune; as fast as the old lower-case filter on
  20,000 messages), used by `domain.Filter` (the `/search` pager), the
  `/list` filter, and a last either-script substring step of
  `domain.ResolveChat` (`/whois`, `/win @name`, `/msg`) that runs only
  when the plain steps found nothing and never for a name that
  transliterates to nothing.
- Review led to: punctuation-cut, sanitised and case-aware link
  candidates, one-link `/open`, the opener's rule reused, the
  ResolveChat step and its guard, the flock degradation and the
  explicit platform list, the matcher, the stressed i and stress marks,
  ASCII-only comments, a pager-highlight note, spec and comment updates,
  and tests for the pager paste, the refused login and the lock warning
  text.
- Tests: `internal/config/lock_test.go`, `lock_unix_test.go`;
  `internal/domain/translit_test.go` (table, cross-script matching, the
  matcher against the full comparison, ResolveChat, Filter);
  `internal/tui/queued_test.go` (Ctrl-L, `/open` link and refusals, link
  completion order, distinctness, limit, punctuation, case and
  placement, pasted line breaks in a window, a list and the pager, list,
  pager, `/whois` and `/win` matching in either script);
  `cmd/lazygram` `TestSecondInstanceIsRefused` (run and login),
  `TestLockStateWarnsWhenItCannotLock`.
- Docs: specs/ui.md (keys, `/open` rows, Tab row, Modes, pager, Sending,
  Completion, Mouse, the Search in either script section with the
  decision), specs/security.md (the lock, the link rule), README
  (feature bullet, rows), `/help`.

## Decisions

- Only Cyrillic is transliterated, never Latin, so the rule never hides
  a plain match and needs no dictionary; in chat-name resolution the
  either-script step comes last so it never makes a unique match
  ambiguous.
- The lock is advisory and per state directory, not global: a second
  account in another directory is still fine. Where locking is
  impossible lazygram warns instead of refusing to start.
- Pasted line breaks are kept only where a message can have them.

## Open

- The pager highlights plain matches only; mapping transliterated
  offsets back to the original runes is not done.

## Files touched

- internal/config/lock.go, lock_unix.go, lock_other.go, lock_test.go,
  lock_unix_test.go
- internal/domain/translit.go, translit_test.go, domain.go
- internal/tui/links.go, queued_test.go, model.go
- internal/command/command.go
- cmd/lazygram/main.go, login.go, main_test.go
- specs/ui.md, specs/security.md, README.md, todo/todo.md, this note

## Verify

`make check` and `make smoke` in the worktree; `go test -race
./internal/config/ ./internal/domain/ ./internal/tui/ ./cmd/lazygram/`;
`go test -run 'CtrlL|OpenLink|OpenCompletes|Pasted|EitherScript|Translit|MatchText|Matcher|FilterSearches|ResolveChat|TryLock|SecondInstance|LockState' ./internal/config/ ./internal/domain/ ./internal/tui/ ./cmd/lazygram/`.

## Released in v0.4.20

- Replayed onto `origin/master` as `7491701` together with the stable
  latest installer (`c13c04b`); CI green; tag `v0.4.20` at `c13c04b`;
  assets verified against `SHA256SUMS`; the released binaries print
  `lazygram v0.4.20`. The release record is in
  session/2026-11-01-installer-latest.md.

## Next

- The Homebrew formula builds v0.4.20; the Arch/Omarchy recipe is still
  at 0.4.14.
