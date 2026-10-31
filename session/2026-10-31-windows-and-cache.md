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

## Review triage

An adversarial review (5 lenses, 3 skeptics per finding, 71 agents)
confirmed 18 findings on the previous version. Already gone in the
rewrite: the time-based since cut-off, unsanitized titles in the chat list,
the 5s doc. Applied: partial sync results kept on error; cancelled QR
login no longer reported as success (gotd swallows context cancellation);
flags accepted after the subcommand; `make build` no longer echoes the
hash (ldflags embedding removed, `make embed` is the way); `make clean`
keeps `.state/`, `make purge` removes it; usernames claim aliases before
title slugs; `make embed` reads values from the shell environment and
validates digits/hex (no injection); the two-step password is sent as
typed; sync budget 90s so a FLOOD_WAIT retry fits; unusable session files
get a recovery hint; "no environment variables" wording corrected; login
registration resolution extracted and tested. Known and documented: Ctrl-C
at the hidden password prompt takes effect only after Enter.
- Window bar `[N:name]` above the input; Esc then a digit switches windows
  (input restored). The window layout (open windows, current) is stored
  sealed in the cache (`Store.SetState`) on every change and restored at
  start.

## Release tooling

- BSD-2-Clause LICENSE; `version` set from `git describe` by `make build`;
  `make dist` builds static tarballs for 4 platforms with `-tags release`
  and refuses if `app_local.go` (now `//go:build !release`) would be
  compiled in; `make release` publishes with gh. `lazygram update` ported
  from lazyai (brew or binary, SHA256SUMS-checked, no deb).

## v0.3.0 prepared, publish pending

- Tag `v0.3.0` created locally on 51aa408. `make dist` built the four
  tarballs and SHA256SUMS in `dist/`; the darwin-arm64 binary reports
  `lazygram v0.3.0`, exits 2 when not linked, and contains no registration
  (the dev build does, as intended).
- `t0d0r/homebrew-tap` cloned to `../homebrew-tap` with
  `Formula/lazygram.rb` (sha256 placeholder until the tag is on GitHub) and
  a README row; uncommitted.
- Creating the GitHub repository and pushing were not performed from this
  session (publishing requires the operator). Remaining steps, in order:
  `gh repo create t0d0r/lazygram --public --source=. --remote=origin --push`,
  `git push origin v0.3.0`, `make release`, fill the formula's sha256 from
  the tag's source tarball, `brew install --build-from-source
  t0d0r/tap/lazygram`, `brew test lazygram`, push the tap.
- Figlet banners (`internal/logo`, generated with figlet slant/smslant,
  ASCII only): start banner as a raw note at the top of the server window
  with a version tagline; exit banner printed after the alt screen closes.

## v0.3.0 released

- github.com/t0d0r/lazygram is public; `master` and tag `v0.3.0`
  (23c1de5, with the banners) are pushed and the GitHub release holds the
  four tarballs and SHA256SUMS. All assets downloaded back and verified:
  sums match, binary reports `lazygram v0.3.0`, exits 2 when not linked,
  no registration inside. The source tarball holds no private files.
- `Formula/lazygram.rb` in t0d0r/homebrew-tap points at the v0.3.0 source
  tarball (sha256 recorded in the formula) and installs with
  `brew install t0d0r/tap/lazygram`.

## Bot buttons

- `domain.Button`/`Message.Buttons`, `Provider.Press` returning the bot's
  answer and the edited message; `Store.Replace`; buttons rendered under
  the message; `/press N` and `/press <msg> N`. Telegram: inline and reply
  keyboards converted; press = getBotCallbackAnswer + a refetch through
  messages.getMessages or channels.getMessages (entity keeps an
  InputChannel). Demo: `@demobot` with Deploy/Hold/Runbook.
- Double Ctrl-X (within 600 ms) toggles "Enter sends" per window (prompt
  `[@name]> `). Help text re-laid out under 70 columns; long indented
  lines keep their indent when wrapped. Tab only completes, in every view.
- Mouse support (`tea.WithMouseCellMotion`): `renderRich` returns click
  spans per line (chat column, nick, links; list rows; window bar tags);
  the model keeps the span map of the last View; wheel scrolls. Links open
  through `internal/opener` (open/xdg-open/rundll32, http(s) only, no
  shell); `Options.Open` is injectable for tests.
- Attachments: `domain.Media`, `Provider.Fetch`; Telegram downloads the
  largest photo size or the document via gotd's downloader after a refetch
  for a fresh file reference, into `<state>/telegram/media/`; `/open` and
  the `[photo]` token click; only images are opened, files are saved. Demo
  draws a PNG. Untested against the real service (fakes only).
- `/theme`: `internal/tui/theme.go` holds nine schemes (official palette
  values) as colour roles; `applyTheme` repaints the package styles; the
  name is kept in the cache state (`theme`) and restored at start; Tab
  completes theme names after `/theme `.
- Server window is number 0 (permanent); chat windows from 1; Alt-0 /
  Esc-0 reach it. `/move N` reorders. The Enter-sends toggle is saved in
  the layout per window and the status line ends with "ctrl-x: to send" or
  "enter: to send". Exit banner uses figlet lean on wide terminals with a
  session summary; a release build checks GitHub (3s) for a newer version.
- Tab completion is bash-like: unique match filled in, else the longest
  common prefix; a second Tab that cannot extend prints the candidates.
- `/msg <@user|#chan> <text>` resolves the chat, opens and focuses its
  window when missing (fetching its recent messages) and sends.
- v0.4.0 tagged and released on GitHub (four tarballs + SHA256SUMS);
  Homebrew formula bumped to 0.4.0 in t0d0r/homebrew-tap.
- Exit statistics: time wasted, messages arrived/sent, buttons pressed,
  files fetched, cache size, windows open/opened, chats visited, commands.
  `/about` prints version, author, repository, license, Go/OS/arch,
  libraries, provider, theme and uptime.
- Up/Down walk a per-session input history (commands and sent lines, 200
  max, duplicates collapsed); line scrolling stays on PgUp/PgDn and the
  wheel; the pager keeps Up/Down for scrolling.
- v0.4.0 released. Tag v0.4.1 exists on GitHub without a release (it was
  superseded before publishing; remote tags are not deleted from here).
  v0.4.2 released from d5b4ef5 with /msg, /about, statistics, input
  history, the double Ctrl-X change and the coloured bars; assets
  verified, no registration inside; Homebrew formula at 0.4.2, brew
  upgrade and brew test passed, tap pushed.
- Ctrl-X holds the text for 600 ms before sending so a fast second Ctrl-X
  cancels the send, restores the text and only toggles "Enter sends".
- Status bar segments in distinct palette colours; window bar tags in each
  chat's colour.
- Window bar back to one colour: active window reversed, unread in the
  activity colour, nothing else coloured.
- Tab on an empty prompt cycles windows (Shift-Tab backwards); with text
  it completes as before.
- v0.4.3 released from 776b2ff (single-colour window bar, Tab cycles
  windows on an empty prompt): assets verified, no registration inside;
  formula at 0.4.3, brew upgrade and brew test passed, tap pushed.
- Clickable text styled (link colour, underline; tokens bold; buttons) with
  a running light (sparkStyle, period 7, 120 ms tick only while visible;
  per-message render cache keyed by content, options and phase). `/fx`
  toggles, kept in state. Scrolled-up windows show a clickable marker row
  and a bright status segment.
- Attachments: providers return bytes (64 MiB cap); `Store.PutMedia`/
  `Media` seal them into `<state>/telegram/media/<hash>.enc`; viewing
  writes a plain copy into a private temp dir removed by `Model.Cleanup`
  at exit. Review finding (high) fixed: an image's extension follows the
  mime allowlist (jpeg/png/gif/webp), never the sender's name; svg and
  other image types are plain files; only raster extensions are opened.
- Running light plays only under the mouse pointer (all-motion mouse
  mode, hover key per span); status line uses brackets only around the
  window tag, two spaces between segments.
- Second review (64 agents, 19 confirmed) applied: mime allowlist for
  auto-opened images (high); BOT_RESPONSE_TIMEOUT treated as "no answer";
  grace period before the post-press refetch; downloads unlocked and
  4-threaded; fetch in-flight guard; exact nick clicks; trailing
  punctuation trimmed from links, exact URL preferred over prefix mapping,
  notice links whole; status text sanitized; " /text" sends; Esc from a
  list restores no filter; more extensions; docs say links open in the
  browser. Refetch of the pressed message still races a very slow bot.
- Running light: one character bouncing ping-pong over the hovered element
  at 60 ms; the hovered message is not cached.
- `/mouse` toggles mouse reporting (tea.DisableMouse / EnableMouseAllMotion),
  kept in state and applied at start; docs note the Shift/Option drag for
  selecting text while reporting is on.
- Light speed derives from element length: a pass takes 400 ms (hover
  clock in milliseconds instead of a tick counter).
- Fix: the first hover after an idle screen now schedules the light's tick
  at once (the decision used the previous render's state).
- Public registration: `cmd/lazygram/app_public.go` carries the author's
  api id and hash as byte slices (decision recorded in specs/providers.md
  with the shared-use risk); it is the last fallback after app.json, the
  environment and a private embed. Brew and source builds need no setup.
- v0.4.5 tagged (built-in registration, media vault, hover light, /mouse,
  review fixes). Homebrew formula at 0.4.5, brew upgrade and brew test
  passed, tap pushed. The GitHub release assets for v0.4.4 and v0.4.5 are
  built in dist/ but not uploaded (upload refused from this session);
  `make release` at the tag publishes them. Brew does not need them.
- Status bar: fixed-width, fixed-colour sync segment; "nothing new" polls
  leave the status alone (no ten-second flicker).
