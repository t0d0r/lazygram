# Security

- No flags carry credentials. The Telegram application registration comes
  from the git-ignored `app.json`, from `TG_API_ID`/`TG_API_HASH` in the
  environment (a direnv `.envrc` is git-ignored too), from a personal build,
  or, failing all of those, from the public one shipped in the source
  (`cmd/lazygram/app_public.go`), published by decision the way nchat
  publishes its own; errors never print values. That registration is not a
  secret: it identifies the application, not an account, and anyone can
  decode it. Its exposure is the shared-use risk described in
  specs/providers.md, nothing else.
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
  Ctrl-X (or with Enter in a window where "Enter sends" is on), or given
  to /me or /msg, is sent. Ctrl-X never runs a command: a line that starts
  with / and is confirmed with Ctrl-X goes out as text (specs/ui.md,
  Sending), so a command typed by mistake can reach the current chat as a
  message; the 600 ms hold, the "as text, not a command" status and a
  second Ctrl-X within the hold are the way back. /edit and /delete
  touch only this account's newest message in the current chat, on the
  user's command. /send reads only the one local regular file named on
  the command line (64 MiB cap) and sends it to the current chat; Tab
  completion after /send lists directory entries but reads no file. Bot buttons are
  pressed only by /press; link buttons are printed, never opened.
- Read receipts are off by default: lazygram never calls
  messages.readHistory or channels.readHistory unless `/read-receipts on`
  was given (kept across runs, shown by `/read-receipts`) or
  `/mark-as-read` is typed for one chat. With receipts on, a report goes
  out only when a window with unread messages is focused, for the newest
  message lazygram holds; a report is visible to the other side of a
  private chat and marks the chat read on every device of the account
  (specs/ui.md, Read receipts).
- Attachments are downloaded only on /open or a click, never on arrival,
  and stored encrypted in the vault (hashed names, same keys as the
  cache). Pictures are shown inside the terminal from memory by default, so
  nothing plain reaches the disk; `/open out` decrypts a copy into a
  private temporary directory that is removed at exit and hands only
  raster images to the system opener, named by content type (a sender's
  "photo.html" never reaches a browser), other files only get their
  temporary path shown. Downloaded files are remote content. A monitoring
  agent with root can still read process memory and the screen;
  encryption protects data at rest only.
- A link in a message is opened only when clicked or typed after
  `/open`, only `http`/`https` with a host (the same rule for both, and
  for the links Tab offers after `/open`), through the system opener with
  the link as a single argument (no shell). Links are remote content: a click opens whatever
  the sender chose, in the browser or preview app the system assigns.
- `lazygram login` links by QR code, so no phone number or SMS code passes
  through lazygram; a two-step password is read without echo and never
  stored. The only login artifact is the session file.
- One process per state directory: `lazygram` and `lazygram login` take
  an advisory lock on `<state>/lock` (flock, released by the kernel when
  the process ends, so a crash leaves nothing stale) and a second start on
  the same directory exits with status 2 and a message naming the
  directory and the way out (`-data <another directory>`). Reason: two
  processes on one session file are two MTProto sessions on one
  authorization key, which Telegram terminates (AUTH_KEY_DUPLICATED),
  invalidating the login and, with it, the encrypted cache; they would
  also write the same cache and session file. Where the lock cannot be
  taken because the filesystem has no file locks (ENOLCK or ENOTSUP: NFS
  without a lock daemon, some FUSE, CIFS and 9p mounts) or the system
  has no flock (lazygram's own list: Linux, macOS and the BSDs have it),
  lazygram starts anyway and prints a warning that a second instance
  would not be refused, naming the directory and `-data <a directory on
  a local disk>`. Any other lock failure stops the start with the lock
  file's path in the message.
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
  never contain a registration (specs/release.md). A binary installed by
  a package manager (under `/usr`, not `/usr/local`) is never replaced
  (from v0.4.15): the update goes through the package manager, so the
  package's file list stays true and nothing lazygram downloads ever
  lands in `/usr/bin`.
- The Arch/Omarchy package (specs/release.md) builds from the pinned
  source tarball with `-tags release`; the recipe ships no credentials,
  reads none, and installs nothing the user has to trust beyond the
  binary built from the source.
- Run gitleaks before the first push and review `git log -p`.

## Terminal injection

Sender and text from remote services are untrusted. The renderer drops all
control characters (C0/C1, including ESC and BEL), turns tabs into spaces and
keeps newlines in text only. Chat aliases come from local configuration.
