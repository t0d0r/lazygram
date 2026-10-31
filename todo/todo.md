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
- [x] user-run `lazygram login` by QR code, nchat style (specs/providers.md)
- [x] no environment variables: state dir flag, app registration stored locally
- [x] exit with a hint when not linked; `lazygram demo` subcommand
- [x] all dialogs with generated aliases; change-driven sync; flood-wait retry
- [ ] first real-service login and smoke test by the operator
- [ ] reconnect after a dropped connection
- [ ] dialogs beyond the 100 most recent; per-chat include/exclude list
- [ ] persist the per-chat cursor so a restart shows what arrived meanwhile
- [ ] cache size cap or compaction (specs/storage.md)
- [ ] `lazygram logout` (delete session file)

## v0.3 windows and encrypted cache (done)

- [x] BitchX-style windows: server window 1, auto-open per chat, /win, /close, Act (specs/ui.md)
- [x] @people and #channels, Tab completion of names and commands (specs/ui.md)
- [x] selectable /list, deleted accounts hidden unless --show-deleted
- [x] no backlog: first sync records cursors; Recent on window open and /last (specs/providers.md)
- [x] encrypted SQLite cache keyed from the session auth key (specs/storage.md)
- [x] centred-cursor list scrolling; selectable /win list; Ctrl-X sends; /help prints in-window (specs/ui.md)
- [x] /stats, /whois, /me; compact wrapping past the time column (specs/ui.md)
- [x] window bar, Esc+digit, window layout kept across restarts (specs/ui.md)

## v0.3.0 release

- [x] LICENSE (BSD-2-Clause), git-describe version, make dist/release (specs/release.md)
- [x] lazygram update: brew or binary, SHA256SUMS-checked (specs/release.md)
- [~] GitHub repository, v0.3.0 release, Homebrew formula in t0d0r/homebrew-tap (tag, dist and formula prepared; publish pending)
- [x] figlet banners at start and exit (specs/ui.md)
- [ ] bot inline buttons: render under the message, /press to answer callbacks

## Next

- [ ] sending messages (post-v0.1)
- [ ] unread markers and mention highlight
- [ ] cmd/lazygram tests for provider selection
- [ ] gitleaks pre-commit hook
