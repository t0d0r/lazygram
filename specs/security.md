# Security

- Credentials come from environment variables only; never from flags or
  tracked files. Validation errors name variables, never values (tested).
- The tree must contain no secrets, session files, phone numbers, user ids,
  real chat titles or messages. Demo data is invented. `.gitignore` covers
  `.env*`, `*.pem`, `*.key`, session files and `.state/`.
- Cache files are owner-only.
- lazygram does not log message content.
- No interactive Telegram login is performed by the project's tooling; the
  connector fails closed when no authorized session exists.
- `lazygram login` reads phone, code and password only from the interactive
  terminal (password without echo) and never prints or stores them; the only
  artifact is the session file.
- The session file grants account access: keep `LAZYGRAM_TG_SESSION` outside
  the repository (or in a git-ignored path) and treat it like a password.
- Only explicitly configured chats are fetched.
- Third-party dependency: github.com/gotd/td; review upgrades before bumping.
- Run gitleaks before the first push and review `git log -p`.

## Terminal injection

Sender and text from remote services are untrusted. The renderer drops all
control characters (C0/C1, including ESC and BEL), turns tabs into spaces and
keeps newlines in text only. Chat aliases come from local configuration.
