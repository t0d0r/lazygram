# 2026-11-01: typing indicators

Branch `feature/typing-indicators` in the worktree `../lazygram-typing`,
from `master` at `44c85f9`, not committed.

## Done

- domain: `Typing` (one report: chat, member, kind, time), `TypingKind`
  (typing, recording, uploading, choosing, stop), `TypingState` (per chat
  and member, six-second TTL renewed by reports, ended by a stop, a
  message from the member or expiry; `Typists` ordered by first report,
  `Chats`, `Next` for the expiry timer). Owned by one goroutine.
- provider: optional `TypingSource` (`Typing() <-chan domain.Typing`);
  nil means nothing to wait on.
- telegram: an update dispatcher for the three typing updates only,
  registered on the client; `typingEvent` maps update and action to a
  report (pure, tested); a `names` registry (mutex) learns user names
  from dialog and history responses so reports carry the names messages
  use; `typingFeed` emits without blocking (64 buffered, drop when full)
  and closes after the client's Run returns. `Provider.Typing` forwards
  the client's feed. Nothing outbound.
- tui: `typing.go`: a command reads the feed one report at a time and
  re-arms; the state expires on a timer armed once while reports are
  pending; the status line shows the indicator (chat window: its
  typists; server window: every chat with names qualified by chat) with a
  running light over the verb that bounces (right, then back) on a tick
  armed only while the current window shows someone; a message arriving
  through sync clears its sender's report; switching windows re-arms the
  light. The model's clock is `m.now` (tests pin it).
- Tests: domain (apply, renew, kind change, stop, clear, expiry order,
  several members and chats, verbs); telegram (mapping of every update
  type and action, dispatcher delivery, feed never blocks and closes
  once, names registry under the race detector); tui (chat and server
  window wording, expiry on a pinned clock, bounce sequence and
  direction, light activation only when shown and stop when not, feed
  read until closed, wording). All deterministic: no sleeps, no real
  timers; `-race` clean.
- Docs: specs/ui.md (Typing indicators), specs/providers.md (Typing,
  limits), README feature bullet.

## Decisions

- Reports are state, not messages: never in the windows, never cached.
- The indicator sits before the counters in the status line so a
  100-column terminal still shows it.
- A member the service cannot name is "someone"; several unnamed members
  in one chat collapse into one entry.
- No typing is ever sent: the feature is incoming only; the connector
  never calls `messages.setTyping`.
- Review outcome (three angles, adversarially verified): the update
  handler is called concurrently by gotd, one goroutine per received
  frame, so the feed is non-blocking and the registry locked, and two
  reports may arrive out of order (the TTL bounds that); the update
  containers' own users are learned so first-time members are named;
  live drafts count as typing and their stop as a stop; the server window
  lists only chats among the listed dialogs, in order of first report;
  the indicator takes only the room the other status segments leave (a
  short "2 typing" form, else nothing); the light follows `/fx`; the
  timers are re-armed after every message, so a closed window is
  followed too; a message from a sender nobody reported by name ends the
  chat's unnamed entry.

## Open and limits

- The demo provider reports no typing, so `lazygram demo` never shows the
  indicator; a real account does.
- Without a running Telegram session the dispatcher path is tested only
  through `tg.UpdateDispatcher.Handle` with a built update; delivery by
  gotd itself is not exercised offline.
- The names registry knows users seen in responses; a group member who
  never wrote and is not a dialog shows as "someone".
- The update stream carries no gap recovery (none is needed for
  ephemeral reports); it is not used for messages.

## Files touched

- internal/domain/typing.go, typing_test.go
- internal/provider/provider.go
- internal/provider/telegram/typing.go, typing_test.go, gotd.go,
  history.go, telegram.go
- internal/tui/typing.go, typing_test.go, model.go
- specs/ui.md, specs/providers.md, README.md, todo/todo.md, this note

## Verify

`make check` in the worktree; `go test -race -run 'Typing|Names|Bounce'
./internal/domain/ ./internal/provider/telegram/ ./internal/tui/`.

## Released

- Commit `7260a56` on `feature/typing-indicators`, fast-forwarded into
  `master` (`origin/master` had not moved from `44c85f9`); `make check`
  and `make smoke` green; Linux and macOS CI green on that commit.
- Tag `v0.4.18` at `7260a56`, pushed; `make dist` and `make release`
  published the GitHub release (four tarballs and SHA256SUMS). Read back:
  not a draft or prerelease, marked latest; every asset's digest on
  GitHub equals the local build, the downloaded SHA256SUMS is identical
  to `dist/SHA256SUMS`, `shasum -c` passes for all four, the downloaded
  darwin-arm64 binary and the linux-arm64 one (run in an Arch Linux ARM
  container) print `lazygram v0.4.18`.

## Next

- Bump the Homebrew formula and the Arch/Omarchy recipe to 0.4.18
  (packaging/omarchy/README.md, step 0), then open the omarchy-pkgs PR.
- The worktree `../lazygram-typing` and the merged branch can be removed.
