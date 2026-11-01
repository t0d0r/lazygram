# 2026-11-01: installer script

## Done

- `install.sh` at the repository root (specs/release.md, Installer
  script): downloads the pinned release's tarball and `SHA256SUMS` over
  HTTPS only, verifies the checksum with `sha256sum` or `shasum`, refuses
  on a mismatch, extracts only the binary and installs it to
  `$LAZYGRAM_PREFIX/bin` (default `~/.local/bin`) without root. Version
  pinned to `v0.4.18`, overridable with `LAZYGRAM_VERSION`; Linux and
  macOS, amd64 and arm64.
- Validated: `bash -n`, `shellcheck` clean, a real install into a
  scratch prefix on macOS (darwin-arm64, `lazygram v0.4.18`) and inside
  an Arch Linux ARM container (linux-arm64, `lazygram v0.4.18`); the
  error paths for an unknown release and a malformed version stop
  without installing anything.
- The release procedure gets a step: bump `VERSION` in `install.sh` and
  the commit the README's installer command points at.

## Decisions

- The script pins a version and the README pins the script by commit, so
  a copied command always installs the same bytes; "latest" is left to
  `lazygram update`.
- No `sudo`, no system paths: the user's own bin directory only.

## Files touched

- install.sh, specs/release.md, this note

## Verify

`bash -n install.sh && shellcheck install.sh`; `LAZYGRAM_PREFIX=/tmp/p
bash install.sh` installs and prints the version.

- `install.sh` reached `origin/master` in `88c7f62`; the raw URL at that
  commit serves the same bytes as the file (sha256 compared), and the
  README's one-line command was run end to end into a scratch prefix.
- README (Install): the `curl ... | bash` command pointing at
  `raw.githubusercontent.com/t0d0r/lazygram/88c7f621e7.../install.sh`,
  a security warning, the read-first alternative, and the
  `LAZYGRAM_VERSION` / `LAZYGRAM_PREFIX` knobs.

## Next

- With the next release: bump `VERSION` in `install.sh` and the commit in
  the README's command (specs/release.md, step 6).
