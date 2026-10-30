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
`telegram.Client` is the seam for a real library adapter (Dialogs, History).
With a nil client every call returns ErrNotImplemented and the UI shows a
friendly status message. Nothing connects, logs in, or stores a session in
v0.1. Adding the adapter requires: credentials supplied by the operator, a
library decision recorded here, and a login flow run by the user.

## Adding a service

Implement Provider in internal/provider/<name>, add its name and config in
internal/config, and add one case in cmd/lazygram. Chat IDs must be unique
across services (prefix with the service name).
