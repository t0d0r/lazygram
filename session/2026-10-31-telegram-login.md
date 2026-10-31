# 2026-10-31 - telegram login subcommand

## Done

- `lazygram login`: interactive sign-in (phone, code, optional 2FA password
  without echo) via gotd auth.Flow; session saved to
  `$LAZYGRAM_TG_SESSION/session.json` (dir 0700, file 0600 enforced).
- `telegram.Login`, `Prompter`, `NormalizePhone`, authenticator adapter;
  terminal prompter in cmd/lazygram/login.go.
- Fixed `isTerminal`: it treated /dev/null as a terminal (char device); now
  uses x/term.IsTerminal. Affects the non-interactive fallback and login guard.

## Decisions

- Sign-up and ToS acceptance are refused: lazygram never creates accounts.
- Answers come only from the terminal, never flags/env/files, so they cannot
  leak via shell history or process listings.
- Already-authorized session: no prompts, nothing changed.
- Output after success names only the session directory.

## Open

- The exchange with Telegram is untested: no credentials were used and no login
  was run. An operator must run `./lazygram login` once.
- Flood-wait/backoff, refs beyond 100 recent dialogs, rate limits at login.

## Files touched

cmd/lazygram/{main,login,login_test}.go, internal/provider/telegram/{gotd,
login,login_test}.go, go.mod/go.sum (golang.org/x/term), README, specs/
{overview,providers,security}.md, todo/todo.md.

## Verify

`make check`; `./lazygram login </dev/null` prints "login needs an interactive
terminal" and exits 1; `make run </dev/null` prints one demo screen.

## Next step

Operator runs `./lazygram login` with their own API credentials, then
`LAZYGRAM_PROVIDER=telegram ./lazygram` with `LAZYGRAM_TG_CHATS` set; fix what
the real service reveals.
