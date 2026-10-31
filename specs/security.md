# Security

- No flags carry credentials. The Telegram application registration comes
  from the git-ignored `app.json`, from `TG_API_ID`/`TG_API_HASH` in the
  environment (a direnv `.envrc` is git-ignored too), from the git-ignored
  generated `cmd/lazygram/app_local.go` (`make embed`, mode 0600), or is
  typed once at `lazygram login` (hash hidden); errors never print its
  value. lazygram ships no registration and does not reuse nchat's. The
  registration is never written to a tracked file: `*_local.go` is ignored
  and `make embed` refuses to write a file git would track.
- The tree must contain no secrets, session files, phone numbers, user ids,
  real chat titles or messages. Demo data is invented. `.gitignore` covers
  `.env*`, `*.pem`, `*.key`, session files and `.state/`.
- The message cache is encrypted: AES-256-GCM per row, keys derived from the
  session's auth key with HKDF and a random salt, chat ids as HMACs; only
  message ids, timestamps and counts are in clear (specs/storage.md). Cache
  files are owner-only. Earlier plain-text caches are deleted at start.
- lazygram does not log message content.
- No interactive Telegram login is performed by the project's tooling; the
  connector fails closed when no authorized session exists.
- Sending is explicit: only text typed in a chat window and confirmed with
  Ctrl-X, or given to /me, is sent; Enter never sends. Bot buttons are
  pressed only by /press; link buttons are printed, never opened.
- Attachments are downloaded only on /open or a click, into a 0700 media
  directory with 0600 files; only images are handed to the system opener,
  other files are saved and their path shown. Downloaded files are remote
  content; the media directory is part of the private state.
- A link in a message is opened only when clicked, only `http`/`https`
  with a host, through the system opener with the link as a single
  argument (no shell). Links are remote content: a click opens whatever
  the sender chose, in the browser or preview app the system assigns.
- `lazygram login` links by QR code, so no phone number or SMS code passes
  through lazygram; a two-step password is read without echo and never
  stored. The only login artifact is the session file.
- The session file grants account access: the state directory (default
  `~/.local/state/lazygram`, or `.state/` for make targets, git-ignored) is
  0700 and must be treated like a password store. `make clean` leaves it
  alone; only `make purge` removes it.
- Unlinking is done from the phone (Settings -> Devices -> terminate); the
  next start then exits with the "not linked" hint.
- Third-party dependencies: github.com/gotd/td, modernc.org/sqlite; review
  upgrades before bumping.
- `lazygram update` is the only network use besides Telegram; downloads are
  HTTPS-only, size-capped and checked against SHA256SUMS; release binaries
  never contain a registration (specs/release.md).
- Run gitleaks before the first push and review `git log -p`.

## Terminal injection

Sender and text from remote services are untrusted. The renderer drops all
control characters (C0/C1, including ESC and BEL), turns tabs into spaces and
keeps newlines in text only. Chat aliases come from local configuration.
