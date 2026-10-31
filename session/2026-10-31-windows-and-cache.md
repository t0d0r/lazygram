# 2026-10-31 - BitchX windows, encrypted cache

## Done

- TUI rebuilt around virtual windows (specs/ui.md): window 1 = server
  window (all chats + `-!-` notices); a window per chat opens on a new
  message or for chats with unread at start; `/win N|@user|#chan`,
  `/close`, `/win` listing, `[Act: ...]`, Ctrl-N/P, Alt-digits/arrows.
- People `@name`, groups/channels `#name`; `domain.ChatKind`, `Chat.Name()`,
  sigil-aware `ResolveChat` (exact alias clashes across kinds are
  ambiguous).
- Tab completion of commands and names with cycling; selectable `/list`
  (Enter opens the window, typing narrows, `--show-deleted`).
- Provider interface gained `Recent(chatID, limit)`; Telegram sync takes no
  backlog (first call records top ids), skips deleted accounts; kinds,
  unread counts, deleted flag and supergroups mapped. Demo got a person and
  an unread channel.
- Store replaced: encrypted SQLite (`modernc.org/sqlite`, pure Go). Per-row
  AES-256-GCM with AAD binding, HMAC chat keys, HKDF from the session auth
  key with a stored random salt, wrong-key detection (specs/storage.md).
  Legacy plain-text `messages.jsonl`/`chats.json` removed at start.
- Demo cache is in memory; nothing on disk.

## Decisions

- Session-derived cache key rather than a key file or SQLCipher: no new
  secret, pure Go, cache dies with the session; acceptable because the
  cache is a convenience, not the history of record.
- First Telegram sync returns nothing (IRC semantics); history comes per
  window on demand.
- Bars are truncated, never wrapped (`fitLine`), so the screen height is
  stable.

## Open

- Live run only smoke-tested (no backlog means the live smoke screen holds
  counts only). The operator's interactive run is the real test.
- Windows and cursors are per session; nothing persists across restarts
  except the cache.
- The adversarial review workflow started on the previous code is still
  running; its surviving findings are to be triaged against this version.

## Files touched

internal/domain, internal/store (rewritten), internal/provider/{provider,
demo,telegram}, internal/command, internal/tui (model/render rewritten),
cmd/lazygram/main.go, go.mod/go.sum (modernc.org/sqlite), README, specs/
{overview,ui,providers,storage,security}.md, todo/todo.md.

## Verify

```sh
make check
make smoke                 # demo server window
./lazygram -data .state -smoke   # live: counts only, no backlog
make demo                  # interactive: /win 2, Tab, /list, /search
```

## Next step

Triage the review findings, then persist per-chat cursors and open windows
across restarts.
