# Storage

The message cache is an SQLite file whose sensitive content is encrypted in
Go before it reaches the database, after nchat's message cache (plain
SQLite, tables for messages and chats) but unreadable without the key.

## Where

- Telegram: `<data>/telegram/cache.sqlite` next to `session.json`, directory
  0700, file 0600 (created before SQLite opens it, so journal files inherit
  the mode). `journal_mode=DELETE`, so no `-wal`/`-shm` files linger.
- Demo: in memory (`store.Memory`), nothing on disk.
- Unencrypted `messages.jsonl` / `chats.json` from earlier versions are
  deleted at start with a notice.

## Schema

```
meta     (key TEXT PRIMARY KEY, value BLOB)           -- salt, check
chats    (chat_key BLOB PRIMARY KEY, pos INTEGER, data BLOB)
messages (chat_key BLOB, id TEXT, time INTEGER, data BLOB,
          PRIMARY KEY (chat_key, id))
index messages(time), messages(chat_key, time)
```

In clear: the service's message ids, timestamps (Unix nanoseconds), keyed
hashes of chat ids, and row counts. That reveals activity volume and timing
per (unidentifiable) chat and nothing else. Everything identifying (text,
sender, the sender's id and username, reactions, chat id, alias, title,
kind, unread) is inside `data`. The sender's id and username and the
reactions are optional JSON fields (`sender_id`, `sender_nick`,
`reactions`: a list of `emoji`, `count`, `mine`, `mine_count`), so rows
cached before they existed still decode. `MineUnknown` is never stored. `Add` never
overwrites a stored row. The cache follows the service in five ways
(specs/ui.md, Edits and deletions, Reactions):
- `Refresh(msgs)`: a fetched copy of a stored message that differs
  (edited text, media or buttons, the sender's name, id or username, the
  reactions) replaces the stored row whole; the replacements are
  returned. A heal fetch of a window's newest page therefore refreshes
  its reactions too.
- Reactions have no edit time: of two copies the later edit wins, but
  the copy fetched or reported later always carries the current
  reactions, even when its text is unchanged and even when the stored
  copy's edit is newer (then that copy keeps its text, media and buttons
  and takes only the reactions). A copy whose reactions came without the
  account's own (`MineUnknown`, Telegram's `min`) keeps the stored `Mine`
  flag and `MineCount` of each emoji.
- `React(msg)`: reactions the service reported (a `domain.Reacted`
  change) replace the stored message's reactions only; its text, sender,
  times and everything else stay. It returns the stored copy before and
  as stored now (the same when nothing changed, and then nothing is
  written), or not ok when the message is not stored.
- `Prune(chatID, page, before)`: a page of a chat's newest history has
  no gaps, so a stored message of that chat whose numeric id lies
  strictly between the page's lowest and highest and that the page lacks
  was deleted at the service, and so was one above the page's highest
  stored before `before` (the fetch time less a margin; a newer one may
  have arrived after the page was taken); it is deleted with its
  attachment. Non-numeric ids and anything below the page are never
  pruned (a short page may only have skipped service messages).
- `Edit(msg)`: an edit the service reported live replaces the stored
  message's text, media, buttons, edit time and reactions; sender, time
  and ids stay. An edit older than the stored copy's changes nothing but
  the reactions, and `Refresh` takes nothing but the reactions from a
  fetched copy older than the stored edit. When the
  attachment was swapped or removed (`Media.ID`, kind or name differ),
  the vault's copy is dropped.
- Deletions (`Delete`, `DeleteMessages`, `Prune`) are remembered for the
  session; `Add` does not insert a message deleted this session, so a
  page fetched before the deletion cannot bring it back. A deletion
  across several chats remembers every chat and id pair, stored or not.
- The database runs with `PRAGMA secure_delete = ON`: deleted and
  replaced rows are overwritten, not left in free pages.
- `DeleteMessages(chatIDs, ids)`: deletes those ids from any of those
  chats (a deletion reported without a chat, for the chats sharing one id
  space), with their attachments.
A stored row that does not open or decode is skipped by `Refresh`,
`Edit` and `React`, so one bad row never fails the fetch around it. `Delete` (an own
`/delete`) removes the attachment from the vault as well.

## Keys

- Secret: for Telegram the session's MTProto authorization key
  (`session.Data.AuthKey`, 256 bytes), read from `session.json`. Decision:
  it is already the secret that grants account access and already on disk
  with the same protection, so the cache adds no new secret; without
  `session.json` the cache is unreadable, and a new login (new key) starts a
  fresh cache, which is acceptable because lazygram keeps only what arrived
  while it ran plus `/last` fetches. A DC migration can also change the key;
  the effect is the same. The demo uses a random secret per run.
- Derivation: `HKDF-SHA256(secret, salt, "lazygram message cache v1")` gives
  64 bytes: an AES-256-GCM key and an HMAC-SHA256 key. The salt is 32
  random bytes stored in `meta` when the file is created, so two caches
  made from the same secret have different keys.
- `chat_key = HMAC-SHA256(mac key, chat id)` so lookups by chat work
  without revealing the id.
- `data = nonce(12 random bytes) || AES-256-GCM(key, nonce, json, aad)`.
  The associated data binds a blob to its row: `"msg:" + chat_key + id` for
  messages, `"chat:" + chat_key` for chats, `"check"` for the verifier. A
  blob moved to another row fails to decrypt (tested).
- `meta.check` is the sealed string "lazygram"; a file that does not decrypt
  it was written under another secret: `ErrWrongKey`, and the caller
  discards the file.
- MTProto server salts are not used: they are not secret.

Why not SQLCipher: it needs cgo and an external library; per-row AEAD keeps
the build pure Go and the key out of the database engine. The cost is that
text search decrypts rows in Go, which is fine at the scale of a personal
client.

## API

`Open(path, secret)`, `Add(msgs) (fresh, err)` (insert-or-ignore by chat
and id), `Refresh`, `Prune`, `Edit`, `React`, `DeleteMessages`, `Delete`, `Replace`, `Messages(chatID, limit)` (newest limit, oldest first; empty chat
id means all; limit <= 0 means all), `Count`, `Latest`, `Chats`,
`SetChats` (replaces the list, keeping order), `SetState`/`State` (small
sealed JSON documents such as the window layout, in `meta`), `Close`.

## Media vault

`<data>/telegram/media/` holds fetched attachments sealed like message
rows: `AES-256-GCM(key, nonce, bytes, aad "media:"+name)` in a file named
by a keyed hash of chat and message id (`<hash>.enc`), directory 0700,
files 0600, written as `.part` then renamed. Nothing on disk reveals which
message or chat a file belongs to, or its content. The demo keeps its
attachments in memory. To view one, lazygram decrypts it into a private
temporary directory (`lazygram-view-*`, 0700, file 0600) and removes those
copies at exit; the vault copy stays. A download is capped at 64 MiB.

## Limits

No compaction or size cap. The TUI mirrors the newest 2000 cached messages
in memory at start; the chat windows and `/search` work on that mirror
plus what arrives during the session, while the server window shows only
what arrives during the session (specs/ui.md).
