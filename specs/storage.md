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
sender, chat id, alias, title, kind, unread) is inside `data`.

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
and id), `Messages(chatID, limit)` (newest limit, oldest first; empty chat
id means all; limit <= 0 means all), `Count`, `Latest`, `Chats`,
`SetChats` (replaces the list, keeping order), `Close`.

## Limits

No compaction or size cap. The TUI mirrors the newest 2000 cached messages
in memory at start; windows and `/search` work on that mirror plus what
arrives during the session.
