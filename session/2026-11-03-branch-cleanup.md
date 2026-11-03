# 2026-11-03: UI work merged, head branches deleted on merge

## Done

- Pull requests #10 (full-width status bar, window bar and input rows
  in every theme), #11 (v0.4.22 records), #12 (padding after a prepend
  mark), #13 (help and /keys at window width), #14 (start hints) and
  #15 (multi-line input) merged. `master` at `686c96c`: `make check`,
  `make smoke` and `make fuzz FUZZTIME=5s` (26 targets) pass.
- These features are on `master` only; the latest release, v0.4.22,
  does not have them.
- The head branches of the merged pull requests were deleted from
  GitHub, each after checking that its tip was the merged pull
  request's head. The repository now deletes a head branch when its
  pull request is merged (specs/ci.md, Repository settings).

## Open

- omacom/omarchy-pkgs#2022 waits for a maintainer.
- Code-Review needs a second GitHub identity; the Best Practices badge
  needs CONTRIBUTING.md, CHANGELOG.md and the registration.
- Not decided: a short fuzz job in CI; the input area follow-ups in
  todo/todo.md.
- When to release v0.4.23 with the merged UI features.

## Files touched

- specs/ci.md, todo/todo.md, this note

## Verify

- `gh api repos/t0d0r/lazygram --jq .delete_branch_on_merge` prints
  `true`; `git ls-remote --heads origin` lists `master` and the open
  pull requests' branches only
- `make check`

## Next

- Merge this; its branch is deleted on merge, which checks the setting.
