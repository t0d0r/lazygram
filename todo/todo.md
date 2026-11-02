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
- [x] delete the stray GitHub release and tag `v0.4.11-1-gd0bdc3f` (2026-11-02, `gh release delete --cleanup-tag`)
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
- [x] point the recipe at the latest release: v0.4.20, entry and icon from the tarball, `wl-clipboard` optional, rebuilt and tested in Arch Linux and Arch Linux ARM containers (packaging/omarchy/README.md)
- [x] v0.4.21 released (assets verified against SHA256SUMS, the stable installer URL installs it); Homebrew formula at 0.4.21; the recipe builds v0.4.21, built and tested in Arch Linux and Arch Linux ARM containers (packaging/omarchy/README.md)
- [x] submit: fork omacom/omarchy-pkgs, copy packaging/omarchy/lazygram to pkgbuilds/lazygram, open the PR: omacom/omarchy-pkgs#2022 for 0.4.21, validators rerun on their master first (packaging/omarchy/README.md, packaging/omarchy/PR.md)
- [ ] omacom/omarchy-pkgs#2022: a maintainer's build-approved label, green builds on both architectures, merge; answer review comments
- [ ] after the package is published: README install line "omarchy pkg add lazygram" first, verify both channels and architectures
- [ ] man page and shell completions (optional; the recipe would install them)

## Next

- [ ] cap the media vault size (specs/storage.md)
- [x] fuzz targets for the parsers and sanitizers that see untrusted input, `make fuzz`; the bugs they found are fixed (specs/security.md; session/2026-11-02-fuzzing.md)

- [ ] sending messages (post-v0.1)
- [ ] unread markers and mention highlight
- [ ] cmd/lazygram tests for provider selection
- [x] status line, window bar and prompt row on the theme's status background across the whole width, in every theme (specs/ui.md, Themes)
- [ ] gitleaks pre-commit hook
- [x] CI: Linux and macOS workflows with README badges (specs/ci.md)
- [x] OpenSSF Scorecard workflow, results published, score and workflow badges in the README (specs/ci.md)
- [x] fix: Tab after `/send ~/` or `/send ~` panicked (slice out of range) since v0.4.7; regression test and FuzzPathCandidates (specs/ui.md, Completion; session/2026-11-02-path-completion.md)
- [x] input cursor: Left/Right, Ctrl-A/E, Delete, Ctrl-K, Ctrl-W before the cursor; completion at the cursor (specs/ui.md)
- [x] mouse drag selects text and copies it on release: OSC 52 in the frame plus a system tool, /clip modes kept, clicks act on release (specs/ui.md, docs/clipboard.md)
- [x] double click selects a word, triple a row; Shift-click extends (specs/ui.md, Selection and clipboard)
- [x] multi-line input area: each line of the input on its own row (no return mark), growing up to 8 rows or a third of the screen, Up/Down between rows and to the history at the edges, rows on the status background (specs/ui.md, Input area; session/2026-11-03-multiline-input.md)
- [ ] input area follow-ups to decide: Ctrl-W across a line break, Ctrl-K at a line's end joining the next line, word instead of character wrapping, a marker for rows scrolled out of the area
- [x] typing indicators: Telegram typing updates to domain.TypingState with expiry, status line with a running light (specs/ui.md, specs/providers.md); released in v0.4.18 (assets verified against SHA256SUMS); formula and the Arch recipe still at 0.4.14
- [x] Ctrl-X sends a line starting with / as text, bypassing the command parser; Enter keeps running commands (specs/ui.md); released in v0.4.17 (assets verified against SHA256SUMS); formula and the Arch recipe still at 0.4.14
- [x] release v0.4.15 with the mouse selection and the self-update refusal (v0.4.15 and v0.4.16 released 2026-10-31; formula still at 0.4.14)
- [x] /read-receipts on|off (off by default, kept) and /mark-as-read: a read receipt when a window with unread messages is focused or marked read in a list, or once on request (specs/ui.md, specs/security.md); released in v0.4.19 (assets verified against SHA256SUMS)
- [x] Ctrl-L performs /redraw (specs/ui.md; attribution: Yanko Simeonov); released in v0.4.20
- [x] /open autocompletes the 10 most recent links in the current chat (specs/ui.md); released in v0.4.20
- [x] /open http://... opens an explicitly supplied URL (specs/ui.md); released in v0.4.20
- [x] one process per data/state folder: a second lazygram started on the same folder notifies the user and refuses to run (force-closes itself) instead of sharing the session, cache and settings; reason: two MTProto sessions on one auth key trigger AUTH_KEY_DUPLICATED, which invalidates the login and wipes the cache (specs/security.md); released in v0.4.20
- [x] /win list actions: mark the selected window unread, mark it read, close it (specs/ui.md): a button strip on the selected row, Left/Right pick, Enter runs, clickable; released in v0.4.19
- [x] /list actions: mark the selected chat read or unread, as in the /win list (specs/ui.md); released in v0.4.19
- [x] pasted multi-line text keeps its newline characters in the input: a paste must not collapse or drop line breaks (specs/ui.md, Sending: a message may have several lines); released in v0.4.20
- [x] search matches Cyrillic by transliteration: a query in Cyrillic or in its Latin transliteration finds both the Cyrillic text and its transliterated equivalent in chats and messages (specs/ui.md, pager; attribution: Yanko Simeonov); released in v0.4.20
- [x] group and channel members: in a group or channel window Tab after /whois and in message text (and /me) offers only that chat's @nicks: the usernames of its senders and the @usernames its messages mention, always filled in as @username (members without one are not offered); /whois <name> there describes a member and falls back to the chats; /win, /msg and the rest keep every chat; messages carry the sender's id and username (specs/ui.md, Members; specs/providers.md)
- [x] /mark-as-read is the only spelling: the /markread alias is gone (specs/ui.md)
- [x] cached messages learn their sender's id and username when the service sends them again (Store.Identify), so members from an older cache get usernames (specs/ui.md, Members; specs/storage.md)
- [x] the second Tab lists members separated by commas: display names have spaces (specs/ui.md, Members)
- [x] Tab completion matches chat names and members in Cyrillic or Latin, like search (domain.PrefixMatch; specs/ui.md, Search in either script)
- [x] mentions: links (youtube.com/@name), e-mail addresses in any script and the chat's own username are no members; a moved username stays with its newest holder; one member scan per Tab (specs/ui.md, Members)
- [x] group members beyond the senders seen: the participant list where the account may read it (specs/ui.md, Members); done as the member list below
- [ ] a mention by name for members without a username: Tab fills in the display name, and Send passes an inputMessageEntityMentionName (offset and length in UTF-16 units, the user as inputUserFromMessage: the chat, one of their message ids, their user id) so Telegram links and notifies them; needs an entities argument on Provider.Send
- [x] learn usernames for old cached messages without waiting: a group window's first fetch asks for the last 100 instead of 10 while its newest 100 known messages include one without a sender id, so Store.Identify fills them in (specs/ui.md, Windows and Members)
- [x] Tab after /whois offers every member, those without a username by display name; message text keeps @nicks only (specs/ui.md, Members)
- [x] edits and deletions made elsewhere: live from the update stream (updateEditMessage, updateDeleteMessages and the channel kinds) and healed when a chat's history is fetched (Store.Refresh, Store.Prune); deleted messages leave the cache and the vault (specs/ui.md, Edits and deletions; specs/storage.md; specs/security.md)
- [ ] catch up after a restart: remember each chat's last seen message and fetch what arrived meanwhile (with "persist the per-chat cursor" above)
- [ ] show forwarded messages with their origin (`fwd from Name`) instead of `<me>` in Saved Messages
- [x] Tab completes each command's own arguments (/autojoin on|off, /read-receipts on|off, /clip modes, /win list, whatever the spelling) and offers chat names only where a chat fits (/win, /msg, /whois, /list, /me and /edit text, message text) (specs/ui.md, completion)
- [x] autojoin: at every start a window opens for each chat with unread messages (the check ran only on the first run, when no chat list was cached); restored windows take the service's unread counts; /autojoin on|off, on by default, kept (specs/ui.md, Windows)
- [x] the [read] and [unread] buttons of /list and /win tell the service once per press, whatever /read-receipts says: [read] sends a read receipt (up to the service's newest message when lazygram holds none) and clears the unread mark, [unread] sets it (messages.markDialogUnread); /mark-as-read clears the mark too (specs/ui.md, Modes and Read receipts; specs/security.md)
- [x] the server window shows only what arrives during the session; the cached backlog and older fetched history stay in the chat windows and /search (specs/ui.md, Windows)
- [x] a group's member list from Telegram (messages.getFullChat, channels.getParticipants, up to 1000), asked once a session when the window opens and kept in memory, so members who never wrote complete too; hidden lists and broadcast channels keep the members seen (specs/ui.md, Members; specs/providers.md; specs/security.md)
- [x] stable latest installer on GitHub only (specs/release.md, Installer script): `curl -fsSL https://github.com/t0d0r/lazygram/releases/latest/download/install.sh | sh`; install.sh becomes POSIX sh, resolves the latest release from the `releases/latest` redirect (vX.Y.Z only) unless LAZYGRAM_VERSION is set, downloads over HTTPS only (redirects too), checks the tarball against that release's SHA256SUMS, installs to ~/.local/bin without sudo and checks the binary's version; `make dist` copies install.sh into dist/ and lists it in SHA256SUMS, `make release` attaches it; README switches from the commit-pinned command only after the first release that carries the asset answers 200, with the trust/update trade-off and the inspect-first alternative; released in v0.4.20: the URL serves the v0.4.20 install.sh and installs v0.4.20 (checked with sh and dash on macOS arm64, Linux arm64 and amd64)

## Supply chain and OpenSSF Scorecard (specs/ci.md, specs/release.md, specs/security.md)

- [x] CodeQL for Go on pull requests, `master` and weekly (specs/ci.md, CodeQL)
- [x] Dependabot weekly for Go modules and GitHub Actions, grouped, 7-day cooldown (specs/ci.md, Dependabot)
- [x] every action pinned to a commit SHA; golangci-lint pinned to v2.15.0 (specs/ci.md, Pinned actions)
- [x] SECURITY.md with private vulnerability reporting, which is enabled on the repository (specs/security.md)
- [x] golang.org/x/net 0.62.0; `osv-scanner.toml` skips GO-2026-5932 (openpgp, not imported) until 2027-05-01 (specs/security.md)
- [x] ruleset on `master`: no force push, no deletion, no bypass
- [x] Release workflow: a pushed vX.Y.Z tag is built, attested (SLSA build provenance, `actions/attest`) and published; `make release` refuses (specs/release.md, Publishing)
- [ ] v0.4.22 from the Release workflow: verify assets and provenance, Homebrew formula, PKGBUILD bump, update omacom/omarchy-pkgs#2022
- [ ] delete the GitHub releases v0.3.0 to v0.4.21 after v0.4.22 is verified; their tags stay (specs/release.md, Decisions)
- [x] repository settings: Dependabot alerts and security updates, immutable releases, ruleset `release tags` (no deletion or move of `v*`), ruleset `master` requires a pull request and a passing `Linux` check on an up-to-date branch, no bypass (specs/ci.md, Repository settings)
- [ ] OpenSSF Best Practices badge: CONTRIBUTING.md, CHANGELOG.md with the release notes taken from it, then the registration at bestpractices.dev by the owner
- [ ] Code-Review: approvals need a second GitHub identity (a second maintainer, or a machine account that opens the pull requests)
