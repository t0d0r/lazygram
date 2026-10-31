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

## v0.2 telegram (in progress)

- [x] choose library: gotd/td (specs/providers.md)
- [x] implement telegram.Client: dialogs, history, conversion, tested with fakes
- [ ] user-run login flow storing session under LAZYGRAM_TG_SESSION
- [ ] first real-service smoke test by the operator
- [ ] flood-wait handling and error backoff
- [ ] resolve chat refs beyond the 100 most recent dialogs

## Next

- [ ] load history on demand from the service when the cache lacks it
- [ ] sending messages (post-v0.1)
- [ ] unread markers and mention highlight
- [ ] store size cap or compaction (specs/storage.md)
- [ ] cmd/lazygram tests for provider selection
- [ ] gitleaks pre-commit hook
