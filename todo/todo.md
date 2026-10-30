# Todo

## v0.1 (done)

- [x] bootstrap repo, Makefile, docs layout
- [x] domain: merge, filter, chat resolution (specs/overview.md)
- [x] store: JSONL cache and chats file (specs/storage.md)
- [x] command parser (specs/ui.md)
- [x] config validation (specs/providers.md)
- [x] demo provider (specs/providers.md)
- [x] telegram connector boundary, inert (specs/providers.md)
- [x] TUI: feed, history pager with search, chat list, help (specs/ui.md)
- [x] strip control characters before render (specs/security.md)
- [x] non-interactive smoke mode
- [x] README and specs

## Next

- [ ] choose a Telegram client library and implement telegram.Client (specs/providers.md)
- [ ] user-run first login flow, session state under LAZYGRAM_TG_SESSION
- [ ] load history on demand from the service when the cache lacks it
- [ ] sending messages (post-v0.1)
- [ ] unread markers and mention highlight
- [ ] store size cap or compaction (specs/storage.md)
- [ ] cmd/lazygram tests for provider selection
- [ ] gitleaks pre-commit hook
