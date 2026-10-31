# Providers

```go
type Provider interface {
    Name() string
    Chats(ctx) ([]domain.Chat, error)
    Sync(ctx, since time.Time) ([]domain.Message, error)
    Recent(ctx, chatID string, limit int) ([]domain.Message, error)
    Send(ctx, chatID, text string) error
}
```

`Chats` returns every chat with its kind (user, group, channel), alias,
title, unread count and deleted flag. `Sync` returns what arrived since the
last call (`since` is the newest cached time; a provider may ignore it).
`Recent` returns the last `limit` messages of one chat, oldest first, for
`/win` and `/last`. `Send` posts text to a chat; the sent message is not
returned, it arrives through `Sync`. `Sync` may return messages together with an error
(a batch that failed part-way); the caller stores them before showing the
error. The store de-duplicates by (chat id, message id), so calls may
overlap. Message times are absolute; the UI converts to local
time.

## demo

Offline, synthetic, no network. Two groups (`#dev`, `#ops`), one person
(`@linus`) and one channel (`#news`, reported with 2 unread so a window
opens at start). Anchored to start time: older lines are already in the
past and replayed by the first Sync, later lines "arrive" over about 4
minutes. `Recent` serves the chat's past lines; `Send` appends a line from
`me` that the next `Sync` returns. Started with `lazygram demo`; nothing
is kept on disk.

## telegram

Library decision: github.com/gotd/td (pure Go MTProto, no cgo, actively
maintained). `telegram.Client` (Dialogs, History) is the seam; `MTProto` is
the gotd implementation, built on `tg.Client` through a small `api` interface
so the logic is tested with fakes and no network. With a nil client the
provider returns ErrNotImplemented.

### Configuration

No environment variables. Everything is under `<data>/telegram/`:

- `app.json`: the Telegram application registration (api id and hash from
  https://my.telegram.org). Decision: lazygram ships no registration of its
  own and does not reuse nchat's; nchat's SCOPE.md asks forks to obtain their
  own because a shared registration gets rate-limited or banned.
  Resolution order at every start: `app.json`, then the environment
  (`TG_API_ID`/`TG_API_HASH`, or nchat's `TG_APIID`/`TG_APIHASH`), then the
  value carried by the binary: the git-ignored `cmd/lazygram/app_local.go`
  that `make embed` generates from `TG_API_ID`/`TG_API_HASH` (nchat keeps
  its registration in source the same way, but committed; lazygram's file
  is ignored because the registration is personal), or `-ldflags -X
  main.buildAppID=... -X main.buildAppHash=...` for one build.
  `lazygram login` stores whatever it resolved from the environment, or asks
  (hash hidden) when nothing is found, so later starts need no environment.
  A variable set without its partner, or with a bad value, is an error that
  names the variable, never the value.
- `session.json`: the gotd session (auth key and account authorization),
  written 0600.
- the history cache (specs/storage.md).

### Start-up contract

`lazygram` with no subcommand:
1. No registration or no session file: exit 2 with a message naming
   `lazygram login` and `lazygram demo`. No network.
2. Session present: connect (45s limit) and check authorization. Not
   authorized (for example terminated from the phone): exit 2 with the same
   hint. Unreachable: exit 1 with a network message.
3. Authorized: start the TUI. The connector is sticky: a failure during the
   run surfaces in the status line and needs a restart.

### `lazygram login`

QR login, the way nchat and the official desktop apps link a device
(`auth.exportLoginToken`): the terminal shows a QR code encoding the
`tg://login?token=...` URL; the phone scans it under Settings -> Devices ->
Link Desktop Device. No phone number, SMS code or sign-up. When the token
expires before it is scanned a fresh code is shown. If the account has
two-step verification the password is asked once, hidden. An already
authorized session prints "already linked" and changes nothing. Needs a
terminal on stdin; Ctrl-C cancels. Only the session file is written.

The QR code is drawn as nchat draws it: two modules per character row with
half-block characters, light modules as foreground, so it reads on a dark
terminal background (41 columns for a login token). `-ascii` draws two `#`
per module instead (82 columns).

The exchange with Telegram is untested against the real service; the
callbacks, config gating and QR rendering are covered by tests.

### Chats

All dialogs are listed, up to the 100 most recent. Kind: users and bots are
people (`@`), basic groups and supergroups (megagroups) are groups (`#`),
broadcast channels are channels (`#`). Chat id: `telegram:<Bot-API style
id>` (users positive, groups `-id`, channels `-100<id>`). Alias: the
username, else a slug of the title (lower case, letters and digits, dashes,
at most 14 characters); clashes get `-2`, `-3`, ... in dialog order. The own
"Saved Messages" chat is `@saved`. Deleted accounts are returned with
`Deleted` set and alias `deleted-<id>`; the UI hides them, never opens
windows for them, and the connector never fetches their history. The
dialog's unread count is passed through. Forbidden peers and folders are
skipped.

### Sync

Like an IRC client, lazygram shows what arrives after it connected. The
first `Sync` records each chat's top message id and returns nothing; every
later poll (10s) costs one `messages.getDialogs` and fetches history only
for chats whose top message id moved, paging backwards (100 per page) until
the previously seen id, at most 300 messages per chat per poll. `Recent`
is one `messages.getHistory` for the last N (max 100) messages of a chat,
used when a window opens and for `/last`. `Send` is one
`messages.sendMessage` with a random id; the message then arrives through
the next poll as the chat's top message moved. Own messages (`out`) show
as `me`. Service messages are skipped;
media-only messages show as `[media]`. Sender: user name, else post author,
else `me` for own messages, else the peer's name in private chats, else
`channel`. A FLOOD_WAIT of up to 60s is slept once and the request retried;
longer waits surface as errors.

Limits: no update stream (polling only), no topics/forums, no edit/delete
tracking, no reconnect, chats beyond the 100 most recent are not shown, the
per-chat cursor is in memory only (a restart shows nothing until a chat
gets a new message; `/last` fills in).

## Adding a service

Implement Provider in internal/provider/<name>, add a subcommand in
cmd/lazygram. Chat IDs must be unique across services (prefix with the
service name).
