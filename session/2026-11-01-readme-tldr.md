# 2026-11-01: README as a TL;DR

## Done

- README.md cut from 272 to 84 lines: badges, a two-line description,
  the screenshot, Install (1. the curl installer, 2. Homebrew, 3. Arch
  Linux / Omarchy, 4. from source), the installer's trust note with the
  read-and-check-first steps and the `LAZYGRAM_VERSION` /
  `LAZYGRAM_PREFIX` knobs, Quick start with the three things to know
  (Ctrl-X sends, Enter runs commands, `/help` and `/keys`), a short
  feature list, links, Development, license.
- The manual tarball download block is gone; the installer script does
  the same with the checksum check built in.
- docs/usage.md holds what left the README: account linking and the own
  Telegram API id, windows, every command and key (the tables moved
  unchanged), sending, buttons and pictures, updating.
- docs/arch-omarchy.md points at docs/usage.md for the QR login.

## Decisions

- The README answers what it is, how to install it, how to start; the
  reference lives in docs/usage.md and in `/help` and `/keys`.
- The installer's trust trade-off stays in the README next to the
  command, as specs/release.md requires.

## Files touched

- README.md, docs/usage.md, docs/arch-omarchy.md, this note

## Verify

`make check`; every command and key row of the old README is in
docs/usage.md.

## Next

- Commit and push once the new README is approved.
