# 2026-11-01: package state and sync

## Done

- `master` fast-forwarded onto `origin/master` (four commits from the
  mouse-selection and help-keys branches: `deeecf7`, `573afbe`,
  `f1484fe`, `607296d`), no local-only commits, no force push.
- README (Install) states where the Arch Linux / Omarchy package stands:
  built and checked on x86_64 and aarch64, accepted by the omarchy-pkgs
  validators, not in pkgs.omarchy.org; the omarchy-pkgs pull request is
  not opened yet, and once opened needs a maintainer's `build-approved`
  label before it builds and merges.
- Releases v0.4.15 (tag at `deeecf7`, mouse selection and clipboard) and
  v0.4.16 (tag at `607296d`, help split) exist on GitHub since
  2026-10-31; v0.4.16 is the latest. The Homebrew formula still builds
  0.4.14. The packaging recipe still names 0.4.14.
- The clipboard code that arrived with the sync (specs/ui.md,
  docs/clipboard.md) falls back to `wl-copy`, `xclip` or `xsel` when the
  terminal does not take OSC 52; the 0.4.14 recipe is unaffected (that
  release has no clipboard code), the recipe for v0.4.16 gets
  `wl-clipboard` as an optional dependency (todo). The only go.mod change
  is `go-osc52` becoming a direct dependency; still no cgo.

## Verify

`make check` (gofmt, vet, golangci-lint, tests) and `make smoke` on the
merged head; CI (Linux, macOS) runs on the push.

## Next

- Point `packaging/omarchy/lazygram/PKGBUILD` at v0.4.16 (entry and icon
  from the tarball, `wl-clipboard` optional), rebuild in the containers,
  then open the omarchy-pkgs PR (packaging/omarchy/README.md, steps 0 to
  6).
- Bump the Homebrew formula to 0.4.16.
- Delete the stray tag `v0.4.11-1-gd0bdc3f` on GitHub.
