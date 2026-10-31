# Providers

```go
type Provider interface {
    Name() string
    Chats(ctx) ([]domain.Chat, error)
    Sync(ctx, since time.Time) ([]domain.Message, error)
}
```

`Sync` returns messages strictly newer than `since` (zero means backlog). The
store de-duplicates by (chat id, message id), so providers may overlap.
Message times are absolute; the UI converts to local time.

## demo

Offline, synthetic, no network. Anchored to start time: older lines are
already in the past, later lines "arrive" over about 4 minutes. Message ids
are fixed, so a cached run keeps its first-run timestamps (known quirk).

## telegram

Config via environment (see README). `telegram.New` validates configuration.
`telegram.Client` (Dialogs, History) is the seam; with a nil client every call
returns ErrNotImplemented.

Library decision: github.com/gotd/td (pure Go MTProto, no cgo, actively
maintained). Used through `telegram.MTProto`, built on `tg.Client` via a small
`api` interface so the logic is tested with fakes and no network.

Behavior of `MTProto`:
- Lazy: no connection until the first Dialogs/History call, and only when
  LAZYGRAM_PROVIDER=telegram. Session state: `$LAZYGRAM_TG_SESSION/session.json`
  (dir 0700, file 0600, never committed).
- The running TUI never logs in. If the session holds no authorized account
  every call fails with ErrNotLoggedIn. Sign-in is the separate, user-run
  `lazygram login` subcommand (below).
- Read-only. Only messages.getDialogs and messages.getHistory are called.
- Only chats listed in LAZYGRAM_TG_CHATS are read. Refs: `@username` or the
  Bot-API style numeric id (user `42`, group `-5`, channel `-100<id>`), found
  among the 100 most recent dialogs. Unknown aliases are reported by name.
- Chat ids are `telegram:<bot-api id>`; message ids are Telegram message ids.
- Sync pages backwards per chat until messages not newer than `since`, capped
  at 300 messages per chat per call. Service messages are skipped; media-only
  messages show as `[media]`.
- Limits: no flood-wait handling beyond the library defaults, no update
  stream (polling every 5s), no topics/forums, no edit/delete tracking.

### `lazygram login`

One-time, interactive, user-run. Requires LAZYGRAM_TG_API_ID, _API_HASH and
_SESSION (provider need not be set); refuses without a terminal on stdin.
Prompts on the terminal for phone (international format, normalized by
`NormalizePhone`), the login code, and, if the account has two-step
verification, the password (read without echo). Answers are never read from
flags, environment or files, never echoed, never logged. If an authorized
session already exists it prints "already logged in" and changes nothing.
Accounts are never created (sign-up and terms-of-service acceptance are
refused). On success it prints only the session directory and enforces mode
0600 on `session.json`. Ctrl-C cancels. The flow is covered by tests for the
prompter, phone normalization and the authenticator adapter; the exchange with
Telegram itself is untested.

## Adding a service

Implement Provider in internal/provider/<name>, add its name and config in
internal/config, and add one case in cmd/lazygram. Chat IDs must be unique
across services (prefix with the service name).
