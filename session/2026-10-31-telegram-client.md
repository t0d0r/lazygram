# 2026-10-31 - telegram client integration

## Done

- Added github.com/gotd/td v0.163.0 and `telegram.MTProto`, the real
  `telegram.Client`: lazy connect, session file under LAZYGRAM_TG_SESSION,
  fails closed with ErrNotLoggedIn, read-only (getDialogs, getHistory).
- Pure logic split out and tested with a fake API: ref resolution
  (@username, Bot-API ids), pagination with `since` cutoff and per-chat cap,
  message conversion (sender, media, service messages).
- main wires `telegram.NewClient` and closes providers that implement
  io.Closer.

## Decisions

- gotd/td: pure Go, no cgo, covers MTProto user accounts.
- Only configured chats are read (privacy, fewer calls); none configured is an
  error that names the variable.
- No login flow: user-run login stays a separate, explicit step.
- Sticky failure: after a failed start (e.g. not logged in) restart lazygram
  after logging in; avoids reconnecting on every 5s poll.

## Open

- Never exercised against the real service (no credentials used here).
- Login flow, flood-wait/backoff, refs beyond the 100 latest dialogs.

## Files touched

internal/provider/telegram/{gotd,entities,history}.go, history_test.go,
telegram.go, telegram_test.go, cmd/lazygram/main.go, go.mod, go.sum, README,
specs/{overview,providers,security}.md, todo/todo.md.

## Verify

`make check`; `LAZYGRAM_PROVIDER=telegram ./lazygram -smoke` with no env must
print config errors and exit 1 without any network access.

## Next step

Design the user-run login flow (phone/code/2FA prompts in a dedicated
subcommand, session saved to LAZYGRAM_TG_SESSION), then an operator smoke test.
