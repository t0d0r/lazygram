# Storage

Directory: `<data>/<provider>/` (default `~/.local/state/lazygram`, or
`-data DIR`), mode 0700, files 0600. Make targets use `.state/` in the
project (git-ignored). The telegram directory also holds `session.json` and
`app.json` (specs/providers.md).

- `messages.jsonl`: one Message per line, append-only. Corrupt lines are
  skipped on load. Loaded messages are merged (sorted, de-duplicated).
- `chats.json`: last known chat list, written atomically via rename.

The cache lets the history pager work offline and across restarts.
Known limits: the whole history is held in memory; no compaction or size cap.
