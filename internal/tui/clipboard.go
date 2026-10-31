package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/aymanbagabas/go-osc52/v2"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// Mouse selection: a left-button drag over the screen selects text like
// the terminal's own drag would if lazygram were not reporting the mouse.
// The selection is a stream from the anchor cell to the cell under the
// pointer, in reading order, like a terminal's. On release the text is
// put on the clipboard; a press and release without motion is a click.

// Clipboard modes for /clip. The choice is kept in the store.
const (
	clipAuto  = "auto"  // OSC 52 through the terminal and the system tool when there is one
	clipOSC52 = "osc52" // the terminal only (works over SSH when the terminal allows it)
	clipTool  = "tool"  // the system tool only (pbcopy, wl-copy, xclip, xsel, clip)
	clipOff   = "off"   // dragging selects nothing; clicks still work

	clipKey  = "clipboard"            // store state key
	clipHold = 250 * time.Millisecond // how long the OSC 52 sequence stays in the frame
	toolWait = 5 * time.Second        // a clipboard tool that hangs is given up after this
)

var clipModes = []string{clipAuto, clipOSC52, clipTool, clipOff}

func validClipMode(s string) bool {
	for _, m := range clipModes {
		if s == m {
			return true
		}
	}
	return false
}

// selection is a drag in progress, in screen cells of the last View. The
// anchor is where the button went down; the end follows the pointer.
type selection struct {
	x0, y0 int // anchor
	x1, y1 int // end
	moved  bool
}

func (s *selection) extend(x, y int) {
	s.x1, s.y1 = x, y
	if x != s.x0 || y != s.y0 {
		s.moved = true
	}
}

// ordered returns the two cells in reading order: start first.
func (s selection) ordered() (ax, ay, bx, by int) {
	if s.y1 < s.y0 || (s.y1 == s.y0 && s.x1 < s.x0) {
		return s.x1, s.y1, s.x0, s.y0
	}
	return s.x0, s.y0, s.x1, s.y1
}

// cols returns the selected column range of row y, both ends inclusive,
// and false when the row is outside the selection. Rows strictly between
// the two ends are selected whole.
func (s selection) cols(y, width int) (from, to int, ok bool) {
	ax, ay, bx, by := s.ordered()
	if y < ay || y > by {
		return 0, 0, false
	}
	from, to = 0, max(width-1, 0)
	if y == ay {
		from = max(ax, 0)
	}
	if y == by {
		to = max(bx, 0)
	}
	if from > to {
		return 0, 0, false
	}
	return from, to, true
}

// selectedText extracts the selection from the plain (unstyled) rows of
// the last View. A cell inside a wide character takes the whole
// character; trailing blanks are dropped from every line; lines are
// joined with newlines, so a multi-row drag pastes as several lines.
func selectedText(rows []string, s selection) string {
	_, ay, _, by := s.ordered()
	var out []string
	for y := max(ay, 0); y <= by && y < len(rows); y++ {
		from, to, ok := s.cols(y, 1<<30) // no right edge: a row ends where its text ends
		if !ok {
			continue
		}
		out = append(out, strings.TrimRight(cells(rows[y], from, to), " "))
	}
	return strings.Join(out, "\n")
}

// cells returns the runes of plain s whose columns touch [from, to]. A
// zero-width rune (a combining mark) stays with the rune before it.
func cells(s string, from, to int) string {
	var b strings.Builder
	col, took := 0, false
	for _, r := range s {
		w := ansi.StringWidth(string(r))
		if w == 0 {
			if took {
				b.WriteRune(r)
			}
			continue
		}
		took = col+w-1 >= from && col <= to
		if took {
			b.WriteRune(r)
		}
		col += w
	}
	return b.String()
}

// markCells paints columns [from, to] of a styled line in reverse video
// so the drag shows what it has selected. Styles outside the range stay.
func markCells(line string, from, to int) string {
	width := ansi.StringWidth(line)
	if from >= width {
		return line
	}
	// Plain SGR rather than a lipgloss style: the mark must show whatever
	// colour profile lipgloss detected, and 27 only ends the reverse.
	mid := ansi.Strip(ansi.Cut(line, from, to+1))
	return ansi.Cut(line, 0, from) + "\x1b[7m" + mid + "\x1b[27m" + ansi.Cut(line, to+1, width)
}

// plainRows strips the styling from the screen rows.
func plainRows(rows []string) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = ansi.Strip(r)
	}
	return out
}

// osc52Seq is the escape sequence that asks the terminal to put text on
// the system clipboard (OSC 52). The text travels base64-encoded, so
// nothing in it can be read as a control sequence by the terminal. Under
// GNU screen the sequence is wrapped in DCS chunks so screen passes it
// on; tmux understands OSC 52 itself (set-clipboard on or external).
func osc52Seq(text string) string {
	seq := osc52.New(text).Clipboard(osc52.SystemClipboard)
	if term := os.Getenv("TERM"); strings.HasPrefix(term, "screen") && os.Getenv("TMUX") == "" {
		seq = seq.Screen()
	}
	return seq.String()
}

// clipboardTool names the system clipboard command for this platform, or ok
// false when there is none worth trying: a Wayland or X11 tool only when
// that display is reachable, so a headless SSH host asks nothing. Only
// fixed program names are looked up on PATH; the text goes in on stdin.
func clipboardTool() (name string, args []string, ok bool) {
	return clipToolFor(runtime.GOOS, os.Getenv, exec.LookPath)
}

func clipToolFor(goos string, getenv func(string) string, lookPath func(string) (string, error)) (string, []string, bool) {
	have := func(n string) bool { _, err := lookPath(n); return err == nil }
	switch goos {
	case "darwin":
		if have("pbcopy") {
			return "pbcopy", nil, true
		}
	case "windows":
		if have("clip") {
			return "clip", nil, true
		}
	default:
		if getenv("WAYLAND_DISPLAY") != "" && have("wl-copy") {
			return "wl-copy", nil, true
		}
		if getenv("DISPLAY") != "" {
			if have("xclip") {
				return "xclip", []string{"-selection", "clipboard", "-in"}, true
			}
			if have("xsel") {
				return "xsel", []string{"--clipboard", "--input"}, true
			}
		}
		if getenv("WSL_DISTRO_NAME") != "" && have("clip.exe") { // WSL: the Windows clipboard
			return "clip.exe", nil, true
		}
	}
	return "", nil, false
}

type clipDoneMsg struct {
	tool string
	err  error
}

type clipFlushMsg struct{} // the OSC 52 sequence has had its frame; drop it

// copyToolCmd pipes text into the clipboard tool in the background. The
// tool's output is not collected: xclip forks and keeps serving the
// selection, which would hold a pipe open for ever.
func copyToolCmd(name string, args []string, text string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), toolWait)
		defer cancel()
		c := exec.CommandContext(ctx, name, args...)
		c.Stdin = strings.NewReader(text)
		err := c.Run()
		if ctx.Err() != nil {
			err = fmt.Errorf("gave up after %s", toolWait)
		}
		return clipDoneMsg{tool: name, err: err}
	}
}

// copySelection puts the dragged text on the clipboard through the
// channels the clipboard mode allows and says so in the status.
func (m *Model) copySelection(s selection) (tea.Model, tea.Cmd) {
	text := selectedText(plainRows(m.screen), s)
	if strings.TrimSpace(text) == "" {
		m.setInfo("nothing to copy")
		return m, nil
	}
	var via []string
	var cmds []tea.Cmd
	if m.clipMode == clipAuto || m.clipMode == clipOSC52 {
		m.clipSeq = osc52Seq(text)
		via = append(via, "terminal")
		cmds = append(cmds, tea.Tick(clipHold, func(time.Time) tea.Msg { return clipFlushMsg{} }))
	}
	if m.clipMode == clipAuto || m.clipMode == clipTool {
		if name, args, ok := clipboardTool(); ok {
			via = append(via, name)
			cmds = append(cmds, copyToolCmd(name, args, text))
		} else if m.clipMode == clipTool {
			m.setErr("no clipboard tool found (pbcopy, wl-copy, xclip, xsel, clip); /clip auto uses the terminal")
			return m, nil
		}
	}
	what := fmt.Sprintf("%d lines", strings.Count(text, "\n")+1)
	if !strings.Contains(text, "\n") {
		what = fmt.Sprintf("%d characters", len([]rune(text)))
	}
	m.setInfo(fmt.Sprintf("copied %s via %s", what, strings.Join(via, " and ")))
	return m, tea.Batch(cmds...)
}

// clip runs /clip: no argument shows the mode and the tool this platform
// would use; a mode name switches and is kept.
func (m *Model) clip(arg string) {
	tool := "no clipboard tool found"
	if name, _, ok := clipboardTool(); ok {
		tool = "tool: " + name
	}
	if arg == "" {
		m.setInfo(fmt.Sprintf("clipboard: %s (%s); /clip %s", m.clipMode, tool, strings.Join(clipModes, " | ")))
		return
	}
	arg = strings.ToLower(arg)
	if !validClipMode(arg) {
		m.setErr("clipboard modes: " + strings.Join(clipModes, ", "))
		return
	}
	m.clipMode = arg
	_ = m.opt.Store.SetState(clipKey, m.clipMode)
	switch arg {
	case clipOff:
		m.setInfo("clipboard off: dragging selects nothing")
	default:
		m.setInfo(fmt.Sprintf("clipboard: %s (%s)", arg, tool))
	}
}
