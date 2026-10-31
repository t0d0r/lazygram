# Security

- No flags carry credentials. The Telegram application registration comes
  from the git-ignored `app.json`, from `TG_API_ID`/`TG_API_HASH` in the
  environment (a direnv `.envrc` is git-ignored too), from a personal build,
  or is typed once at `lazygram login` (hash hidden); errors never print its
  value. lazygram ships no registration and does not reuse nchat's.
- The tree must contain no secrets, session files, phone numbers, user ids,
  real chat titles or messages. Demo data is invented. `.gitignore` covers
  `.env*`, `*.pem`, `*.key`, session files and `.state/`.
- Cache files are owner-only.
- lazygram does not log message content.
- No interactive Telegram login is performed by the project's tooling; the
  connector fails closed when no authorized session exists.
- `lazygram login` links by QR code, so no phone number or SMS code passes
  through lazygram; a two-step password is read without echo and never
  stored. The only login artifact is the session file.
- The session file grants account access: the state directory (default
  `~/.local/state/lazygram`, or `.state/` for make targets, git-ignored) is
  0700 and must be treated like a password store.
- Unlinking is done from the phone (Settings -> Devices -> terminate); the
  next start then exits with the "not linked" hint.
- Third-party dependency: github.com/gotd/td; review upgrades before bumping.
- Run gitleaks before the first push and review `git log -p`.

## Terminal injection

Sender and text from remote services are untrusted. The renderer drops all
control characters (C0/C1, including ESC and BEL), turns tabs into spaces and
keeps newlines in text only. Chat aliases come from local configuration.
