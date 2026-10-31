# 2026-10-31 - mouse selection and clipboard

Branch `feature/mouse-selection-clipboard` (worktree
`lazygram-mouse-selection`), on top of 6b68713 (the Arch/Omarchy recipe).
Nothing committed, pushed, tagged or released in this session; the
working tree holds the whole change.

## Done

- Left-button drag selects text while mouse reporting is on (specs/ui.md,
  "Selection and clipboard"): stream selection from the anchor cell to
  the pointer over any screen row, reverse-video mark while dragging,
  copied on release with `copied N characters|lines via ...` in the
  status. Clicks moved from press to release so a drag that starts on a
  link selects instead of opening it; a press and release in place is
  the click it always was.
- Two clipboard channels, `/clip auto|osc52|tool|off` kept in the store
  (`clipboard` key, default `auto`): OSC 52 written into the frame (on
  the rule row, dropped by a 250 ms tick) so it is flushed in order with
  the rendering and never races the renderer's writes; and a system tool
  on stdin (`pbcopy`, `clip`, `wl-copy`, `xclip`, `xsel`, `clip.exe`)
  chosen by platform and only when `WAYLAND_DISPLAY`/`DISPLAY`/WSL say
  the display is reachable, run in the background with a 5 s limit.
  GNU screen gets the DCS-wrapped form; tmux handles OSC 52 natively.
- New file `internal/tui/clipboard.go` (selection geometry, text
  extraction with wide and combining characters, the mark, OSC 52, tool
  choice and command, `/clip`); `internal/tui/model.go` (fields `sel`,
  `screen`, `clipMode`, `clipSeq`; mouse handler; View keeps the styled
  rows; help text); `internal/command/command.go` (`Clip`, `/clip`,
  `/clipboard`, help line). `go-osc52/v2` became a direct dependency
  (it was already in the module graph through termenv).
- Tests: `internal/tui/clipboard_test.go` (geometry, extraction, mark,
  OSC 52 incl. screen/tmux, tool choice table, drag-and-copy through
  Update with the OSC sequence in the View and gone after the flush
  tick, drag on a link, release-based click, stray release, `/clip`
  modes, persistence and fallback); the `click` test helper now presses
  and releases; a command parser case. No test runs a clipboard tool
  (model tests set `osc52`).
- Docs: specs/ui.md (Mouse section, commands and keys tables), README
  (features, commands, keys), docs/clipboard.md (usage, configuration,
  troubleshooting per terminal and multiplexer, security), todo.

## Decisions

- OSC 52 goes through the View, not a direct write to stdout: bubbletea
  v1 offers no clipboard command, and a concurrent write could land
  inside a frame the renderer is flushing through the colour-profile
  writer. The sequence is zero-width for x/ansi, survives the renderer's
  truncation, and the rule row is repainted anyway for the new status.
- Reverse video for the mark is plain SGR 7/27, not a lipgloss style:
  lipgloss drops attributes when it detects no colour profile (tests,
  dumb terminals) and 27 ends only the reverse.
- The tool is never tried on a headless host; errors from a tool that
  was tried are shown, since a Terminal.app user depends on pbcopy.
- Shift/Option-drag and `/mouse` off remain the terminal-native paths.

## Verification

- `make check`: gofmt clean, go vet, golangci-lint 0 issues, all tests
  green.
- Real binary in a private tmux server (`tmux -L`, `set-clipboard off`
  so nothing reached the clipboard): SGR press/motion/release sequences
  sent with `send-keys` produced the reverse-video mark across two rows
  mid-drag, `copied 2 lines via terminal` on release, and a press and
  release on `[1:#news]` in the window bar switched to that window.

## Open

- Nothing is committed. Suggested commit: `feat(tui): mouse drag selects
  text and copies it to the clipboard` with the body from "Done".
- Not verified by hand in iTerm2/kitty/Terminal.app with a real pointer
  (only synthetic sequences); the clipboard content itself was not
  checked end to end in a terminal that honours OSC 52.
- Next: double click selects a word, triple a row; Shift-click extends;
  then release v0.4.15 (this plus the self-update refusal for packaged
  installs) and point the Omarchy recipe at it.

## Security note

- The clipboard is written, never read: OSC 52 "set" only, base64
  payload, so no sender-controlled bytes reach the terminal as a
  sequence (control characters are stripped before rendering as well).
- The system tool is a fixed program name resolved on PATH, text on
  stdin, no shell; relative PATH entries are refused by exec.LookPath.
- Copied text leaves the encrypted cache for the system clipboard, which
  other programs and clipboard managers can read; docs/clipboard.md
  says so.

## Run and verify

`make check`, `make demo` (drag over any line, release, read the rule
row; `/clip` shows the mode and the tool), `make smoke`.
