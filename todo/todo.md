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
- [ ] omacom/omarchy-pkgs#2022 (now 0.4.30): a maintainer's build-approved label, green builds on both architectures, merge; answer review comments
- [ ] after the package is published: README install line "omarchy pkg add lazygram" first, verify both channels and architectures
- [ ] man page and shell completions (optional; the recipe would install them)

## Next

- [ ] cap the media vault size (specs/storage.md)
- [x] fuzz targets for the parsers and sanitizers that see untrusted input, `make fuzz`; the bugs they found are fixed (specs/security.md; session/2026-11-02-fuzzing.md)

- [ ] sending messages (post-v0.1)
- [ ] unread markers and mention highlight
- [x] rows padded to the window width stay full after an Arabic prepend mark (U+0600..U+0605), a regression with charmbracelet/x/ansi 0.10.4 found by FuzzFitLine (session/2026-11-03-prepend-padding.md)
- [ ] cmd/lazygram tests for provider selection
- [x] status line, window bar and prompt row on the theme's status background across the whole width, in every theme (specs/ui.md, Themes)
- [ ] gitleaks pre-commit hook
- [x] CI: Linux and macOS workflows with README badges (specs/ci.md)
- [x] OpenSSF Scorecard workflow, results published, score and workflow badges in the README (specs/ci.md)
- [x] fix: Tab after `/send ~/` or `/send ~` panicked (slice out of range) since v0.4.7; regression test and FuzzPathCandidates (specs/ui.md, Completion; session/2026-11-02-path-completion.md)
- [x] input cursor: Left/Right, Ctrl-A/E, Delete, Ctrl-K, Ctrl-W before the cursor; completion at the cursor (specs/ui.md)
- [x] start hints in window 0: /list and /win always, then two drawn at random from a pool of 21, each true where it is read (specs/ui.md, Windows; session/2026-11-03-start-hints.md)
- [x] mouse drag selects text and copies it on release: OSC 52 in the frame plus a system tool, /clip modes kept, clicks act on release (specs/ui.md, docs/clipboard.md)
- [x] double click selects a word, triple a row; Shift-click extends (specs/ui.md, Selection and clipboard)
- [x] a released selection stays highlighted after the copy until the next press, a change of a row under it, a cursor moving on it, a resize or /mouse off (specs/ui.md, Selection and clipboard; session/2026-11-03-keep-selection.md)
- [ ] fix: a drag that ends on the left half of a wide character copies that character but highlights one character less (markCells and cells disagree; specs/ui.md, Selection and clipboard)
- [x] multi-line input area: each line of the input on its own row (no return mark), growing up to 8 rows or a third of the screen, Up/Down between rows and to the history at the edges, rows on the status background (specs/ui.md, Input area; session/2026-11-03-multiline-input.md)
- [ ] input area follow-ups to decide: Ctrl-W across a line break, Ctrl-K at a line's end joining the next line, word instead of character wrapping, a marker for rows scrolled out of the area
- [x] typing indicators: Telegram typing updates to domain.TypingState with expiry, status line with a running light (specs/ui.md, specs/providers.md); released in v0.4.18 (assets verified against SHA256SUMS); formula and the Arch recipe still at 0.4.14
- [x] reactions: counts after each message in every window, kept current from Telegram pushes (own messages), polls of the messages on screen every 20 s and fetches; window 0 tells when someone adds one to any message (specs/ui.md, Reactions; specs/providers.md; specs/storage.md; session/2026-11-03-reactions.md)
- [ ] reactions against live Telegram: check that pushes arrive for own messages, that polls find others' reactions in groups and channels, and that a message missing from a poll answer really has none (taken from TDLib, not the docs)
- [ ] reactions follow-ups to decide: the alt emoji of custom reactions (messages.getCustomEmojiDocuments), who reacted (messages.getMessageReactionsList); sending one is /like (below)
- [x] /like [name]: the account's reaction on the newest message someone else sent in the current chat window (domain.LastFromOthers), one of twelve names (like, the default; dislike, love, fire, laugh, sad, party, clap, thanks, ok, think, eyes; aliases +1, -1, heart, tada, pray), through the optional provider.Reactor: Telegram messages.sendReaction with one reactionEmoji (refusals in words, MESSAGE_NOT_MODIFIED done, the answer's reactions applied like a poll's), the demo in memory; errors for another name, window 0, a provider without reactions, no target; /help, Tab, a start hint (specs/ui.md, Reactions, Sending a reaction; specs/providers.md; session/2026-11-06-like.md)
- [ ] /like off (remove the account's reaction): confirm from a source (TDLib, or the API docs) that messages.sendReaction with an empty or absent reaction list removes it, then add it (specs/ui.md, Reactions, Sending a reaction)
- [ ] /like against live Telegram, by the operator on an account of their choosing: a reaction in a private chat, a group and a channel; REACTION_INVALID in a chat that allows few reactions; whether the sendReaction answer carries updateMessageReactions (the window then shows it at once)
- [x] Ctrl-X sends a line starting with / as text, bypassing the command parser; Enter keeps running commands (specs/ui.md); released in v0.4.17 (assets verified against SHA256SUMS); formula and the Arch recipe still at 0.4.14
- [x] release v0.4.15 with the mouse selection and the self-update refusal (v0.4.15 and v0.4.16 released 2026-10-31; formula still at 0.4.14)
- [x] /read-receipts on|off (off by default, kept) and /mark-as-read: a read receipt when a window with unread messages is focused or marked read in a list, or once on request (specs/ui.md, specs/security.md); released in v0.4.19 (assets verified against SHA256SUMS)
- [x] Ctrl-L performs /redraw (specs/ui.md; attribution: Yanko Simeonov); released in v0.4.20
- [x] /win last and a fast double Tab jump to the last focused window; a window a Tab only passed through keeps its unread count and sends no read receipt (specs/ui.md, Windows, Read receipts; session/2026-11-03-win-last.md)
- [x] /unread focuses the next window with unread messages; /doubletab [off|last|unread|<ms>], off by default: Tab twice fast jumps as /win last or /unread, within a time of 150 to 1000 ms (default 400) that also times the pass-through protection; two /settings rows; /set and /options removed, /settings is the only name of the menu (specs/ui.md, Windows, Settings menu; session/2026-11-04-doubletab.md)
- [x] /open autocompletes the 10 most recent links in the current chat (specs/ui.md); released in v0.4.20
- [x] /open http://... opens an explicitly supplied URL (specs/ui.md); released in v0.4.20
- [x] /help and /keys use the window's width (one logical line per entry, wrapped when drawn with a hanging indent, re-flowed on resize); /keys prints the notes first and ends with the key table; /about thanks @zechkata and @pau4o for their feedback (specs/ui.md, Commands; session/2026-11-03-help-keys-width.md)
- [x] the notes on the mouse, themes and windows move from /keys to /help, printed before the command reference; /keys is the key table alone (specs/ui.md, Commands; session/2026-11-03-help-notes.md)
- [x] /help notes laid out as labelled entries like the commands (Themes, Clickable text, Selection, Double click, Mouse, Window 0, Reactions, New windows, Names): two spaces, the label, the text from column 25 with a hanging indent; they were paragraphs at the margin that ran into each other (specs/ui.md, Commands; session/2026-11-04-help-notes.md)
- [x] /close takes a list and ranges (/close 2,3-5, /close 1-4, /close 1,2,3; commas and/or spaces), numbered as shown before the command, all or nothing (a malformed item, window 0, a reversed range or a missing window closes none), each window once; notices in window order, one status naming them (cut after four), the layout saved once, the current window kept or the nearest before it (specs/ui.md, Windows; session/2026-11-04-close-ranges.md)
- [x] /close-others closes every chat window but the current one at once, through the code of /close (a notice per window in window 0, one status, the layout saved once, nothing focused, the current window kept as window 1); window 0 stays; an argument and window 0 are errors and with no other chat window it is information, none of them changing anything or writing the layout; /help entry, Tab completion, a start hint (specs/ui.md, Windows; session/2026-11-05-close-others.md)
- [x] /x reports the current chat window read, as /mark-as-read (one receipt, the unread mark cleared, whatever /read-receipts says), then closes it through the code of /close once the service answered without an error; a failed receipt or mark keeps the window and is said in red with `; window kept`; an argument, window 0 and a chat without a known message are errors that send and close nothing; a message arriving meanwhile keeps the window; /help entry, Tab completion, a start hint (specs/ui.md, Read receipts, Read and close; session/2026-11-10-x-read-close.md)
- [ ] /x against live Telegram, by the operator, on an account of their choosing: tested with the fake provider and the demo only
- [x] /feed on|off: off keeps every chat's messages out of window 0, which then shows only its notices; on by default, kept, takes effect at once (specs/ui.md, Windows; session/2026-11-03-feed-settings.md)
- [x] /settings (also /set, /options): a menu in window 0 of every kept setting (feed, autojoin, read-receipts, names, theme, fx, mouse, clip); Enter or Right next value, Left previous, through each command's own code (specs/ui.md, Settings menu; session/2026-11-03-feed-settings.md)
- [x] /vim on|off: an optional vim mode, off by default, kept, a /settings row; Insert is today's keys, Esc enters Normal (motions, i/a/I/A/o/O, x/r/d/c/y/p, one-level undo, a kill buffer in memory only, scroll keys, gt/gT, : / ?); /list, /win, /settings and the pager open in a Normal mode of their own (j/k, h/l, gg/G, q); NORMAL on the status line (specs/ui.md, Vim mode; session/2026-11-03-vim-mode.md)
- [x] vim Normal: Tab and Ctrl-W take the character under the cursor, as a, the key and Esc do in vi ('hi @ali' Esc Tab completes @alice; it gave 'hi @alice i'), the cursor stepping back after (specs/ui.md, Vim mode)
- [ ] decide: in vim Normal the keys that keep their meaning (Tab, Ctrl-W, Ctrl-K, Backspace, Delete, a paste) keep no undo snapshot, so u skips them and can take back the change before them as well; snapshot them as the vi keys do? (specs/ui.md, Vim mode, Undo)
- [ ] decide: Ctrl-W treats only a space as a word break, so in a multi-line input it deletes across a line break (in Insert and with vim off too); vi stops at the line start (specs/ui.md, Input area)
- [x] README: an animated GIF of the demo (docs/demo.gif, in place of docs/screenshot.svg): the QR login with an invented token, then the server window, /list, sending, a picture in the window, a bot button and /search (session/2026-11-03-demo-gif.md)
- [x] start notices name the chats: a sync whose new messages come from a chat the chat list lacks waits for the chat list (at start the one on its way; later one request per new chat a session), so "window 1 opened for demo-1" and "1:demo-1 (8)" no longer appear on a first start and a deleted account opens no window; edits and reactions made while it waits, a failed chat list's error and smoke mode are kept (specs/ui.md, Windows; session/2026-11-04-start-names.md)
- [ ] decide: text typed in a list's filter (/list, /win, /settings) or the pager stays in the input when another window is switched to, and with "Enter sends" on there it can be sent as a message; clear it on a switch out of a list? (specs/ui.md, Modes)
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
- [x] Tab completes a chat by its title too and fills in the name as /names shows it, a title across words included; /win, /msg and /whois resolve the exact title and the exact and prefix tiers in either script; /msg takes a name with spaces (specs/ui.md, Names: nick or title, Search in either script; session/2026-11-03-complete-titles.md)
- [ ] decide: name completion in name mode resolves every candidate (quadratic in the chat count: about 1 ms at 100 chats, 0.9 s at 2000); index the names if the chat list grows past the provider's 100
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
- [x] v0.4.22 from the Release workflow: assets and provenance verified, Homebrew formula at 0.4.22, PKGBUILD at 0.4.22 built and tested on both architectures, omacom/omarchy-pkgs#2022 moved to 0.4.22 (session/2026-11-03-release-0422.md)
- [x] v0.4.23 from the Release workflow: assets and provenance verified, the installer URL installs it, PKGBUILD at 0.4.23 built and tested on both architectures (session/2026-11-03-release-0423.md)
- [x] Homebrew formula at 0.4.23 (tap `f2d169b`): built from source, `brew test` passed
- [x] ruleset on the tap's `main`: no deletion, no force push, no bypass (specs/release.md, Publishing)
- [x] omacom/omarchy-pkgs#2022 moved to 0.4.23: validators rerun on their master `30fed32`, recipe built on both architectures (packaging/omarchy/README.md, Submit)
- [x] v0.4.24 from the Release workflow (/feed, /settings, the notes in /help): assets and provenance verified, the installer URL installs it, PKGBUILD at 0.4.24 built and tested on both architectures, omacom/omarchy-pkgs#2022 moved to 0.4.24 with the validators rerun on their master `35687e5` (session/2026-11-03-release-0424.md)
- [x] Homebrew formula at 0.4.24 (tap `5853157`): built from source, `brew test` passed
- [x] v0.4.25 from the Release workflow (the kept selection highlight, completion and resolution by title in either script): assets and provenance verified, the installer URL installs it, PKGBUILD at 0.4.25 built and tested on both architectures, omacom/omarchy-pkgs#2022 moved to 0.4.25 with the validators rerun on their master `e0ac3a7` (session/2026-11-03-release-0425.md)
- [x] Homebrew formula at 0.4.25 (tap `f6a17fa`): built from source, `brew test` passed
- [x] v0.4.26 from the Release workflow (vim mode, /win last and the double Tab, reactions, chat names in the start notices, the README demo GIF): assets and provenance verified, a tampered file refused, the installer URL installs it; PKGBUILD at 0.4.26 built and tested on both architectures, omacom/omarchy-pkgs#2022 moved to 0.4.26 with the validators rerun on their master `be513f7` (session/2026-11-04-release-0426.md)
- [x] Homebrew formula at 0.4.26 (tap `9d4dc1f`): built from source, `brew test` passed
- [x] v0.4.27 from the Release workflow (/unread, the /doubletab setting, /settings as the only name of the menu): assets and provenance verified, a tampered file refused, the installer URL installs it; PKGBUILD at 0.4.27 built and tested on both architectures, omacom/omarchy-pkgs#2022 moved to 0.4.27 with the validators rerun on their master `8896655` (session/2026-11-04-release-0427.md)
- [x] Homebrew formula at 0.4.27 (tap `f7ad46a`, the v0.4.27 source tarball's sha256 checked on two downloads): built from source, `brew test` passed
- [x] v0.4.28 from the Release workflow (the /help notes as labelled entries, /close with lists and ranges): assets and provenance verified, a tampered file refused, the installer URL installs it; PKGBUILD at 0.4.28 built and tested on both architectures, omacom/omarchy-pkgs#2022 moved to 0.4.28 with the validators rerun on their master `c9e0d94` (session/2026-11-04-release-0428.md)
- [x] Homebrew formula at 0.4.28 (tap `c35175f`, the v0.4.28 source tarball's sha256 checked on two downloads): built from source, `brew test` passed
- [x] v0.4.29 from the Release workflow (/close-others): assets and provenance verified, a tampered file refused, the installer URL installs it; PKGBUILD at 0.4.29 built and tested on both architectures, omacom/omarchy-pkgs#2022 moved to 0.4.29 with the validators rerun on their master `525f91b` (session/2026-11-05-release-0429.md)
- [x] Homebrew formula at 0.4.29 (tap `a294169`, the v0.4.29 source tarball's sha256 checked on two downloads): built from source, `brew test` passed
- [x] v0.4.30 from the Release workflow (/like): assets and provenance verified, a tampered file refused, the installer URL installs it; PKGBUILD at 0.4.30 built and tested on both architectures, omacom/omarchy-pkgs#2022 moved to 0.4.30 with the validators rerun on their master `bf44164` (session/2026-11-07-release-0430.md)
- [x] Homebrew formula at 0.4.30 (tap `49f8b9f`, the v0.4.30 source tarball's sha256 checked on two downloads): built from source, `brew test` passed
- [x] delete the GitHub releases v0.3.0 to v0.4.21 after v0.4.22 is verified; their tags stay (specs/release.md, Decisions)
- [x] repository settings: Dependabot alerts and security updates, immutable releases, ruleset `release tags` (no deletion or move of `v*`), ruleset `master` requires a pull request and a passing `Linux` check on an up-to-date branch, no bypass (specs/ci.md, Repository settings)
- [x] head branches deleted when their pull request is merged; the merged ones from #11 to #14 removed (specs/ci.md, Repository settings)
- [ ] OpenSSF Best Practices badge: CONTRIBUTING.md, CHANGELOG.md with the release notes taken from it, then the registration at bestpractices.dev by the owner
- [ ] Code-Review: approvals need a second GitHub identity (a second maintainer, or a machine account that opens the pull requests)
