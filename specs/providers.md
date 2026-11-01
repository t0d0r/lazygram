# Providers

```go
type Provider interface {
    Name() string
    Chats(ctx) ([]domain.Chat, error)
    Sync(ctx, since time.Time) ([]domain.Message, error)
    Recent(ctx, chatID string, limit int) ([]domain.Message, error)
    Send(ctx, chatID, text string) error
    Press(ctx, chatID, msgID string, b domain.Button) (domain.PressAnswer, error)
    Fetch(ctx, chatID, msgID string) (data []byte, name string, err error)
    SendFile(ctx, chatID, name string, data []byte, caption string) error
    Edit(ctx, chatID, msgID, text string) error
    Delete(ctx, chatID, msgID string) error
}
```

`Chats` returns every chat with its kind (user, group, channel), alias,
title, unread count and deleted flag. `Sync` returns what arrived since the
last call (`since` is the newest cached time; a provider may ignore it).
`Recent` returns the last `limit` messages of one chat, oldest first, for
`/win` and `/last`. `Send` posts text to a chat; the sent message is not
returned, it arrives through `Sync`. `Press` presses a callback button and
returns the bot's answer (text, optional URL) and the message as it is
afterwards (`Updated`), since bots edit their message in response; the
caller replaces the message. `Fetch` downloads a message's attachment into memory and returns its
bytes and a file name; the caller seals it into the vault. `SendFile`
uploads bytes as a photo (image file name) or as a document with that
file name and a caption; the message arrives through `Sync`. `Edit` and
`Delete` change or remove one of the account's own messages. `Sync` may return messages together with an error
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
`me` that the next `Sync` returns. `@demobot` asks a question with
`Deploy`, `Hold` and a `Runbook` link; `Press` answers and edits the
question. `@linus` sends one photo; `Fetch` draws a small PNG in memory for it. Started with `lazygram demo`; nothing
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
  https://my.telegram.org). Decision (2026-10-31): like nchat, lazygram
  ships its author's registration in the source
  (`cmd/lazygram/app_public.go`, byte slices so scanners stay quiet; it is
  public, not secret), so brew and source builds work without setup. The
  known risk, as nchat's SCOPE.md states, is that a registration shared by
  many clients can be throttled or banned by Telegram, which would stop
  every build carrying it, the author's included; forks must register
  their own. Resolution order at every start: `app.json`, then the
  environment (`TG_API_ID`/`TG_API_HASH`, or nchat's `TG_APIID`/`TG_APIHASH`),
  then the
  value carried by the binary: the git-ignored `cmd/lazygram/app_local.go`
  that `make embed` generates from `TG_API_ID`/`TG_API_HASH` in the
  environment (nchat keeps its registration in source the same way, but
  committed; lazygram's file is ignored because the registration is
  personal). `make embed` reads the values through the shell environment,
  never expands them into the recipe, requires digits and hex, and refuses
  to write a file git would track.
  then the public registration shipped in the source. `lazygram login`
  stores whatever it resolved from the environment, or asks (hash hidden)
  when nothing at all is found, so later starts need no environment.
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
terminal on stdin; it prints "connecting to Telegram" first since the
connection can take a while. Ctrl-C cancels and is reported as
"login cancelled", never as success (gotd's Run swallows cancellation, so
Login checks the context itself); Ctrl-C at the hidden password prompt
takes effect after Enter. A two-step password is sent exactly as typed.
Only the session file is written.

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
as `me`. Inline and reply keyboards are converted to buttons row by row
(callback, URL, plain text; others unsupported). `Press` is
`messages.getBotCallbackAnswer` with the button's data (a
BOT_RESPONSE_TIMEOUT, common with bots that never answer the query, is
reported as "no answer" and not as a failure), then a short grace period
for the bot's edit, then `messages.getMessages` (or
`channels.getMessages` for channels and supergroups) to return the edited
message. Message edits are otherwise not
tracked. Photos and documents become `Media` (`photo`; `image` for a
document whose mime type is image/jpeg, image/png, image/gif or
image/webp; `file` otherwise, with the sender's file name for display);
`Fetch` fetches the message again for a fresh file reference, picks the
largest photo size (or the document), streams it through gotd's
downloader into memory (64 MiB cap, four threads, without holding the
client lock so syncs and sends continue) and returns the bytes with a name
whose extension follows the content type for images (never the sender's
name, which decides nothing about what gets opened). Other media (polls,
locations, ...) shows as `[media]` and cannot be fetched. The only calls the connector makes: messages.getDialogs,
messages.getHistory, messages.sendMessage (Ctrl-X, /me, reply buttons),
messages.getBotCallbackAnswer, messages.getMessages and
channels.getMessages (/press, /open), upload.getFile (/open),
messages.editMessage (/edit), messages.deleteMessages with revoke or
channels.deleteMessages (/delete), upload.saveFilePart /
upload.saveBigFilePart and messages.sendMedia (/send: gotd's uploader in
four threads and its message sender, run without the client lock;
inputMediaUploadedPhoto for image names, inputMediaUploadedDocument with
the base name and mime type otherwise; 64 MiB cap). Service messages are skipped;
media-only messages show as `[media]`. Sender: user name, else post author,
else `me` for own messages, else the peer's name in private chats, else
`channel`. A FLOOD_WAIT of up to 60s is slept once and the request retried;
longer waits surface as errors.

Limits: messages come by polling only (the update stream serves typing
reports, nothing else), no topics/forums, no edit/delete tracking, no
reconnect, chats beyond the 100 most recent are not shown, the per-chat
cursor is in memory only (a restart shows nothing until a chat gets a new
message; `/last` fills in).

### Typing

`provider.TypingSource` is optional: `Typing() <-chan domain.Typing`
returns a feed of reports (chat, member name, kind, time), closed when the
service stops; nil means the service reports nothing. The Telegram
connector registers an update dispatcher for `updateUserTyping` (private
chats), `updateChatUserTyping` (basic groups) and `updateChannelUserTyping`
(supergroups and channels) and ignores every other update. Actions fold
into four kinds: typing; recording (audio, video, round video); uploading
(photo, video, audio, document, round); choosing (sticker, contact,
location, game); `cancel` is a stop. Live drafts (a bot or a user
streaming a reply) count as typing and their stop as a stop; history
imports, speaking in a group call and emoji interactions are not reports.
The member's name comes from a registry of the users seen in dialog and
history responses and in the update containers themselves (the same name
messages show); a member never seen, or a channel posting as itself, is
unnamed. gotd runs the update handler concurrently, one goroutine per
received frame, so the handler never blocks and touches only the
non-blocking feed and the locked name registry: the feed holds 64 reports
and a report that does not fit is dropped (the next one renews it).
Reports are not guaranteed to reach the TUI in the order they were
received (typing updates carry no sequence number), so a stop can
occasionally be applied before the start it follows; the six-second TTL
then ends it. The feed is closed only after the client's Run has
returned, which is after the read loop has waited for every handler, so
no handler can write to a closed channel. Nothing is sent: the connector
never calls `messages.setTyping`.

## Adding a service

Implement Provider in internal/provider/<name>, add a subcommand in
cmd/lazygram. Chat IDs must be unique across services (prefix with the
service name).
