# 2026-11-01: integration of list actions and read receipts

## Done

- `feature/list-actions` (`e99654f`) and `feature/read-receipts`
  (`185f161`) were replayed onto `origin/master` at `21bf5f0` (the
  installer commits) in the worktree `../lazygram-integrate`, branch
  `integrate/receipts-list`, as `ae30142` and `7caee1d`. The one
  conflict was `todo/todo.md`: both entries kept, the list items as in
  progress and the read-receipts item added.
- The two features now agree: the `[read]` button of `/win` and `/list`
  follows the read-receipt rule, reporting a window with unread messages
  read exactly as a focus would, and only with `/read-receipts on`
  (`internal/tui/listactions.go` calls `readCmd` before clearing the
  count; `TestListReadButtonFollowsTheReceiptRule`); specs/ui.md (Modes,
  Read receipts) and the README bullet say so.
- The main checkout's uncommitted `todo/todo.md` (the queued todo-only
  update) was left untouched; its content is a subset of the merged
  todo, which carries the same lines plus the progress marks.

## Verify

`make check`, `go test -race` on command, telegram and tui, `make smoke`
in the integration worktree.

## Released

- `integrate/receipts-list` fast-forwarded `origin/master` from
  `21bf5f0` to `f19bbe0` (`ae30142` list actions, `7caee1d` read
  receipts, `f19bbe0` the list read button following the receipt rule);
  Linux and macOS CI green on `f19bbe0`.
- Tag `v0.4.19` (annotated, `ab0cd60` -> `f19bbe0`), pushed; `make dist`
  and `make release` from the integration worktree published the GitHub
  release (four tarballs and SHA256SUMS). Read back: remote master and
  tag point at `f19bbe0`, the release is neither draft nor prerelease
  and is marked latest; every asset's digest on GitHub equals the local
  build, the downloaded SHA256SUMS is identical to `dist/SHA256SUMS`,
  `shasum -c` passes for all four, the downloaded tarballs are
  byte-identical to `dist/`, the released darwin-arm64 binary and the
  linux-arm64 one (in an Arch Linux ARM container) print
  `lazygram v0.4.19`, and the binary carries the new commands' help.
- The main checkout's `master` ref stayed at `21bf5f0` on purpose: its
  uncommitted `todo/todo.md` must not be overwritten. Every line of that
  file is on `master` now (the merged todo adds only progress marks), so
  `git checkout -- todo/todo.md && git pull --ff-only` brings it in line
  without losing anything.

## Next

- Bump `VERSION` in `install.sh` and the README's installer commit, the
  Homebrew formula and the Arch/Omarchy recipe to 0.4.19.
