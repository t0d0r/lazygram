# Storage

Directory: `$LAZYGRAM_DATA_DIR/<provider>/` (default
`~/.local/state/lazygram/<provider>/`), mode 0700, files 0600. `make run`
uses `.state/` in the project (git-ignored).

- `messages.jsonl`: one Message per line, append-only. Corrupt lines are
  skipped on load. Loaded messages are merged (sorted, de-duplicated).
- `chats.json`: last known chat list, written atomically via rename.

The cache lets the history pager work offline and across restarts.
Known limits: the whole history is held in memory; no compaction or size cap.
