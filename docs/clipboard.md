# Mouse selection and the clipboard

lazygram reports the mouse so clicks work (chat names, links, window
tags, `[photo]` tokens). The price is that the terminal's own drag
selection is gone while that is on. So lazygram selects for you: drag
with the left button, release, and the text is on the clipboard.

## Usage

- Hold the left button and drag. The selected cells show in reverse
  video. It is a stream selection like a terminal's: from the cell
  where you pressed to the cell under the pointer, whole rows in
  between, any row of the screen.
- Release. The status line (the rule above the status row) says
  `copied 42 characters via terminal and pbcopy` or `copied 3 lines ...`.
  One row is reported in characters, several rows in lines. Trailing
  blanks are dropped from each row; rows are joined with newlines.
- A press and release on the same cell is a click, as before. A drag
  that starts on a link selects the link text instead of opening it.
- Blank space gives `nothing to copy`.

Pictures painted into a window copy as block characters, which is what
the screen holds. The terminal's own selection still works too: hold
Shift while dragging in most Linux terminals, Option in macOS Terminal
and iTerm2, or run `/mouse` to turn mouse reporting off (clicks stop
until `/mouse` again).

## Configuration

`/clip` shows the mode and the system tool this platform would use;
`/clip <mode>` switches. The choice is kept across restarts in the
encrypted state, like `/mouse`, `/fx` and the theme.

| mode    | what carries the text                                           |
|---------|-----------------------------------------------------------------|
| `auto`  | both channels below (the default)                               |
| `osc52` | the terminal, by the OSC 52 escape sequence                     |
| `tool`  | a system clipboard command: `pbcopy`, `wl-copy`, `xclip`, `xsel`, `clip` |
| `off`   | dragging selects and copies nothing; clicks still work          |

OSC 52 is the escape sequence terminals accept to set the clipboard.
lazygram writes it into its own frame, so it reaches the terminal in
order with the rendering, and the terminal application sets the
clipboard of the machine the terminal runs on. That is why it works
over SSH and inside containers: nothing on the remote side touches a
clipboard. The text travels base64-encoded. The sequence is sent once
per copy and dropped from later frames.

The system tool is chosen by platform and only when its display is
reachable: `pbcopy` on macOS, `clip` on Windows, `wl-copy` when
`WAYLAND_DISPLAY` is set, `xclip -selection clipboard -in` or `xsel
--clipboard --input` when `DISPLAY` is set, `clip.exe` under WSL. On a
headless host (SSH without X forwarding) no tool is tried and OSC 52
alone does the job. The tool gets the text on stdin and is given five
seconds; a failure shows as an error in the status line.

There is no environment variable and no config file for this; `/clip`
is the whole configuration.

## Troubleshooting

The status says `copied` but nothing is on the clipboard:

- **macOS Terminal.app** does not support OSC 52. `auto` also runs
  `pbcopy`, so it works locally; over SSH from Terminal.app nothing can
  set the local clipboard. Use iTerm2, kitty, WezTerm, Ghostty or
  Alacritty for remote copying, or Option-drag to use Terminal's own
  selection.
- **iTerm2** asks once: Preferences > General > Selection > "Applications
  in terminal may access clipboard" must be on.
- **tmux**: `set -g set-clipboard on` (or `external`, the default since
  tmux 3.2) in `~/.tmux.conf`, and the outer terminal must support OSC
  52; tmux forwards the sequence itself, no passthrough option is
  needed. With `set-clipboard off`, tmux drops the sequence; the system
  tool still works for local sessions.
- **GNU screen**: lazygram wraps the sequence for screen when `TERM`
  starts with `screen` and `TMUX` is not set. The outer terminal must
  still support OSC 52.
- **Over SSH on Linux**: the remote host usually has no display, so no
  tool is tried; everything depends on the local terminal accepting
  OSC 52. If `xclip` is installed on the host and `DISPLAY` is forwarded
  by `ssh -X`, the tool channel copies to the forwarded X server, which
  is usually what you want too.
- **Wayland without wl-copy**: install `wl-clipboard`. The X11 tools
  (`xclip`, `xsel`) need `DISPLAY`; under Wayland with XWayland that
  copies into the X clipboard, which most compositors sync.
- **Windows Terminal / WSL**: OSC 52 is supported; `clip.exe` is also
  used under WSL.
- `/clip` with no argument prints which tool was found. `/clip osc52`
  or `/clip tool` isolates one channel when the other misbehaves.

The status shows `xclip failed: ...` or `gave up after 5s`: the tool is
there but cannot reach the clipboard (no display, a stale `DISPLAY`,
a sandboxed terminal). Switch to `/clip osc52` or fix the display.

Dragging does nothing and clicks still work: `/clip` is `off`. Dragging
selects with the terminal's own highlight instead: `/mouse` is off, or
you are holding Shift or Option.

The highlight looks wrong on wide characters: a cell inside a
two-column character selects the whole character; the copied text is
right even if the reverse-video edge sits one cell off.

## Security

- The clipboard is written, never read. lazygram sends OSC 52 "set" with
  a base64 payload; it never sends the "?" query that would read the
  clipboard back, and it never asks the terminal for anything.
- Message text already has control characters stripped before it is
  rendered, and base64 on top of that means nothing a sender wrote can
  reach the terminal as an escape sequence through the copy.
- The system tool is run by fixed program name resolved on `PATH`, with
  the text on stdin: no shell, no argument built from the text. The
  usual `PATH` hygiene applies, as it does for `open` and `xdg-open`.
- Whatever you copy leaves the encrypted cache for the system clipboard,
  which other applications and clipboard managers can read. Clear the
  clipboard after copying something sensitive.
