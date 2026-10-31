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
- [x] GitHub repository (github.com/t0d0r/lazygram), releases v0.3.0, v0.4.0, v0.4.2, v0.4.3, v0.4.8; Homebrew formula in t0d0r/homebrew-tap at 0.4.11
- [x] v0.4.12 released (tarballs, SHA256SUMS verified); formula at 0.4.12
- [x] v0.4.13 released after green Linux and macOS CI (tarballs, SHA256SUMS verified); formula at 0.4.13
- [x] v0.4.14 released (input cursor) after green CI; formula at 0.4.14
- [ ] delete the stray GitHub release and tag `v0.4.11-1-gd0bdc3f` (needs `gh release delete ... --cleanup-tag` by hand)
- [x] figlet banners at start and exit (specs/ui.md)
- [x] /theme with nine colour schemes, persisted (specs/ui.md)
- [x] pictures rendered in the terminal from memory; /open out for the viewer (specs/ui.md)
- [x] /send <path> [caption] uploads photos and files; Tab completes paths and /open out, /win list (specs/ui.md)
- [x] Tab completes attachment message ids after /open and /open out (specs/ui.md)
- [x] light themes: light, solarized-light, gruvbox-light (specs/ui.md)
- [x] /redraw (/clear) drops a window's notes, keeps its messages (specs/ui.md)
- [x] /help and F1 print into the server window and focus it (specs/ui.md)
- [x] /help lists the commands only and names /keys; /keys prints the key reference the same way (specs/ui.md)
- [x] /help spells out every parser alias; tests hold the table and the help text together (specs/ui.md)
- [x] block-face banners (figlet banner3/chunky); start-up windows and the exit banner print line by line (specs/ui.md)
- [x] exit banner picks one of several faces at random (own pixel font, figlet, wordmark); a window's first history rolls in (specs/ui.md)
- [x] /edit and /delete for the last own message (specs/ui.md)
- [x] public application registration shipped in the source for brew builds (specs/providers.md)
- [x] attachments sealed in the vault; plain copies only while viewing; image names from content type (specs/storage.md)
- [x] clickable text bright; running light under the pointer, /fx; scrolled-up marker; status line without brackets (specs/ui.md)
- [x] Up/Down walk the input history (specs/ui.md)
- [x] /about; exit statistics
- [x] /msg <@user|#chan> <text> sends and opens the window
- [x] bash-style Tab completion (common prefix, second Tab lists)
- [x] server window is 0 and permanent; /move N; Enter-sends saved and shown; wide exit banner with summary and release check (specs/ui.md)
- [x] attachments: [photo]/[file] tokens, /open and click fetch; images open in the system viewer (specs/ui.md)
- [x] mouse: clicks on chat names, nicks, window bar, list rows, links; wheel scroll (specs/ui.md)
- [x] double Ctrl-X toggles Enter-sends; help fits 80 columns; Tab only completes (specs/ui.md)
- [x] bot inline buttons: rendered under the message, /press answers callbacks, links printed, reply text sent (specs/ui.md)

## Arch Linux and Omarchy package (specs/release.md)

- [x] audit: name free on Arch and AUR, pure Go, go 1.27.3 in Arch and Arch Linux ARM, tests hermetic
- [x] recipe: packaging/omarchy/lazygram (PKGBUILD, .omarchy/package.json, desktop entry, icon)
- [x] self-update never replaces a package-managed binary; exit hint names the package manager
- [x] docs: docs/arch-omarchy.md, README install line, specs/release.md and specs/security.md
- [x] built and tested in Arch x86_64 and Arch Linux ARM aarch64 containers; omarchy-pkgs validators run
- [ ] release v0.4.15 (self-update refusal for packaged installs; tarball carries packaging/), then point the recipe at it: pkgver, updpkgsums, entry and icon from the tarball instead of the local sources
- [ ] submit: fork omacom/omarchy-pkgs, copy packaging/omarchy/lazygram to pkgbuilds/lazygram, open the PR (packaging/omarchy/README.md has the steps and packaging/omarchy/PR.md the text); needs the build-approved label from a maintainer
- [ ] after the package is published: README install line "omarchy pkg add lazygram" first, verify both channels and architectures
- [ ] man page and shell completions (optional; the recipe would install them)

## Next

- [ ] cap the media vault size (specs/storage.md)

- [ ] sending messages (post-v0.1)
- [ ] unread markers and mention highlight
- [ ] cmd/lazygram tests for provider selection
- [ ] gitleaks pre-commit hook
- [x] CI: Linux and macOS workflows with README badges (specs/ci.md)
- [x] input cursor: Left/Right, Ctrl-A/E, Delete, Ctrl-K, Ctrl-W before the cursor; completion at the cursor (specs/ui.md)
- [x] mouse drag selects text and copies it on release: OSC 52 in the frame plus a system tool, /clip modes kept, clicks act on release (specs/ui.md, docs/clipboard.md)
- [ ] double click selects a word, triple a row; Shift-click extends (specs/ui.md)
- [ ] release v0.4.15 with the mouse selection and the self-update refusal
