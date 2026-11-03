package tui

import (
	"encoding/base64"
	"errors"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestSelectionOrderAndCols(t *testing.T) {
	s := selection{x0: 10, y0: 5, x1: 2, y1: 3, moved: true} // dragged up and left
	ax, ay, bx, by := s.ordered()
	if ax != 2 || ay != 3 || bx != 10 || by != 5 {
		t.Fatalf("ordered: %d,%d %d,%d", ax, ay, bx, by)
	}
	if _, _, ok := s.cols(2, 80); ok {
		t.Fatal("row above the selection")
	}
	if from, to, ok := s.cols(3, 80); !ok || from != 2 || to != 79 {
		t.Fatalf("first row: %d-%d %v", from, to, ok)
	}
	if from, to, ok := s.cols(4, 80); !ok || from != 0 || to != 79 {
		t.Fatalf("middle row: %d-%d %v", from, to, ok)
	}
	if from, to, ok := s.cols(5, 80); !ok || from != 0 || to != 10 {
		t.Fatalf("last row: %d-%d %v", from, to, ok)
	}
	one := selection{x0: 7, y0: 1, x1: 4, y1: 1, moved: true} // one row, dragged left
	if from, to, ok := one.cols(1, 80); !ok || from != 4 || to != 7 {
		t.Fatalf("one row: %d-%d %v", from, to, ok)
	}
	var none selection
	none.extend(0, 0)
	if none.moved {
		t.Fatal("no motion: not a drag")
	}
	none.extend(1, 0)
	if !none.moved {
		t.Fatal("one cell away is a drag")
	}
}

func TestSelectedText(t *testing.T) {
	rows := []string{
		"12:00 <alice> hello world   ",
		"12:01 <bob> 日本語 text",
		"12:02 <carol> last",
	}
	got := selectedText(rows, selection{x0: 14, y0: 0, x1: 3, y1: 2, moved: true})
	want := "hello world\n12:01 <bob> 日本語 text\n12:0"
	if got != want {
		t.Fatalf("stream selection:\n got %q\nwant %q", got, want)
	}
	// A cell inside a wide character takes the whole character; the end
	// cell is included.
	if got := selectedText(rows, selection{x0: 13, y0: 1, x1: 15, y1: 1, moved: true}); got != "日本" {
		t.Fatalf("wide chars: %q", got)
	}
	// Dragged backwards over the same text.
	if got := selectedText(rows, selection{x0: 15, y0: 1, x1: 13, y1: 1, moved: true}); got != "日本" {
		t.Fatalf("backwards: %q", got)
	}
	// Past the end of a line, and rows past the screen, give nothing.
	if got := selectedText(rows, selection{x0: 40, y0: 0, x1: 50, y1: 0, moved: true}); got != "" {
		t.Fatalf("past the end: %q", got)
	}
	if got := selectedText(rows, selection{x0: 0, y0: 2, x1: 5, y1: 9, moved: true}); got != "12:02 <carol> last" {
		t.Fatalf("below the screen: %q", got)
	}
	if got := cells("éx", 0, 0); got != "é" {
		t.Fatalf("combining mark stays with its base: %q", got)
	}
}

func TestMarkCells(t *testing.T) {
	line := "\x1b[31mred\x1b[0m plain \x1b[4mund\x1b[0m"
	got := markCells(line, 4, 8)
	if ansi.Strip(got) != ansi.Strip(line) {
		t.Fatalf("text changed: %q", ansi.Strip(got))
	}
	if !strings.Contains(got, "\x1b[7mplain\x1b[27m") {
		t.Fatalf("reverse video over the range: %q", got)
	}
	if got := markCells("short", 10, 20); got != "short" {
		t.Fatalf("range past the end leaves the line: %q", got)
	}
}

func TestOSC52Seq(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString([]byte("hi \x1b[31m"))
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("TMUX", "")
	seq := osc52Seq("hi \x1b[31m")
	if !strings.HasPrefix(seq, "\x1b]52;c;"+b64) {
		t.Fatalf("plain OSC 52 with base64 payload: %q", seq)
	}
	t.Setenv("TERM", "screen.xterm-256color")
	if seq := osc52Seq("x"); !strings.HasPrefix(seq, "\x1bP") {
		t.Fatalf("screen wraps in DCS: %q", seq)
	}
	t.Setenv("TMUX", "/tmp/tmux-1/default,1,0")
	if seq := osc52Seq("x"); !strings.HasPrefix(seq, "\x1b]52;") {
		t.Fatalf("tmux understands OSC 52 itself: %q", seq)
	}
}

func TestClipboardToolFor(t *testing.T) {
	have := func(names ...string) func(string) (string, error) {
		return func(n string) (string, error) {
			for _, h := range names {
				if h == n {
					return "/usr/bin/" + n, nil
				}
			}
			return "", errors.New("not found")
		}
	}
	env := func(kv map[string]string) func(string) string { return func(k string) string { return kv[k] } }
	cases := []struct {
		goos string
		env  map[string]string
		have []string
		want string
	}{
		{"darwin", nil, []string{"pbcopy"}, "pbcopy"},
		{"darwin", nil, nil, ""},
		{"windows", nil, []string{"clip"}, "clip"},
		{"linux", map[string]string{"WAYLAND_DISPLAY": "wayland-0"}, []string{"wl-copy", "xclip"}, "wl-copy"},
		{"linux", map[string]string{"DISPLAY": ":0"}, []string{"wl-copy", "xclip"}, "xclip"},
		{"linux", map[string]string{"DISPLAY": ":0"}, []string{"xsel"}, "xsel"},
		{"linux", nil, []string{"wl-copy", "xclip", "xsel"}, ""}, // headless: no display, ask nothing
		{"linux", map[string]string{"WSL_DISTRO_NAME": "Ubuntu"}, []string{"clip.exe"}, "clip.exe"},
	}
	for _, c := range cases {
		name, _, ok := clipToolFor(c.goos, env(c.env), have(c.have...))
		if name != c.want || ok != (c.want != "") {
			t.Errorf("%s %v %v: got %q %v, want %q", c.goos, c.env, c.have, name, ok, c.want)
		}
	}
}

// rowOf finds the screen row and column where text starts, as of the
// last View.
func rowOf(t *testing.T, m *Model, text string) (x, y int) {
	t.Helper()
	for y, r := range plainRows(m.screen) {
		if i := strings.Index(r, text); i >= 0 {
			return ansi.StringWidth(r[:i]), y
		}
	}
	t.Fatalf("%q is not on the screen", text)
	return 0, 0
}

func drag(m *Model, x0, y0, x1, y1 int) tea.Cmd {
	m.Update(tea.MouseMsg{X: x0, Y: y0, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	m.Update(tea.MouseMsg{X: (x0 + x1) / 2, Y: y0, Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft})
	m.Update(tea.MouseMsg{X: x1, Y: y1, Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft})
	_, cmd := m.Update(tea.MouseMsg{X: x1, Y: y1, Action: tea.MouseActionRelease, Button: tea.MouseButtonNone})
	return cmd
}

func TestDragSelectsAndCopies(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	f := newFake()
	f.msgs = append(f.msgs, mk("c1", "9", "alice", "copy me please", 4))
	m := setup(t, f)
	m.clipMode = clipOSC52 // never run a system tool from a test
	m.View()
	x, y := rowOf(t, m, "copy me please")

	// While the button is down the selection shows in reverse video.
	m.Update(tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	m.Update(tea.MouseMsg{X: x + 6, Y: y, Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft})
	if m.sel == nil || !m.sel.moved {
		t.Fatal("a motion with the button down is a drag")
	}
	row := strings.Split(m.View(), "\n")[y]
	if !strings.Contains(row, "\x1b[7mcopy me\x1b[27m") {
		t.Fatalf("highlighted selection: %q", row)
	}
	if m.hover != "" {
		t.Fatal("dragging does not hover")
	}

	_, cmd := m.Update(tea.MouseMsg{X: x + 6, Y: y, Action: tea.MouseActionRelease, Button: tea.MouseButtonNone})
	if m.sel != nil {
		t.Fatal("the release ends the drag")
	}
	if m.status != "copied 7 characters via terminal" || m.isErr {
		t.Fatalf("status: %q", m.status)
	}
	want := "\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte("copy me"))
	if !strings.HasPrefix(m.clipSeq, want) {
		t.Fatalf("clipSeq: %q", m.clipSeq)
	}
	if cmd == nil {
		t.Fatal("a flush tick is scheduled")
	}
	view := m.View()
	if !strings.Contains(view, want) {
		t.Fatal("the OSC 52 sequence rides on the frame")
	}
	// The selection stays highlighted after the release, like a terminal's.
	wantMarked(t, view, "copy me")
	m.Update(clipFlushMsg{})
	view = m.View()
	if m.clipSeq != "" || strings.Contains(view, "\x1b]52;") {
		t.Fatal("the flush tick drops the sequence from later frames")
	}
	wantMarked(t, view, "copy me")

	// Several rows: from inside one message to the start of the next;
	// lines are trimmed and joined with newlines.
	plain := plainRows(m.screen)
	drag(m, x, y, 4, y+1)
	exp := strings.TrimRight(plain[y][strings.Index(plain[y], "copy me please"):], " ") +
		"\n" + strings.TrimRight(cells(plain[y+1], 0, 4), " ")
	if got := selectedText(plain, selection{x0: x, y0: y, x1: 4, y1: y + 1, moved: true}); got != exp {
		t.Fatalf("two rows:\n got %q\nwant %q", got, exp)
	}
	if !strings.HasPrefix(m.status, "copied 2 lines via terminal") {
		t.Fatalf("status: %q", m.status)
	}
	if got := base64.StdEncoding.EncodeToString([]byte(exp)); !strings.Contains(m.clipSeq, got) {
		t.Fatalf("clipSeq carries the two lines: %q", m.clipSeq)
	}

	// Dragging backwards over blank space copies nothing and says so; the
	// selection is kept all the same.
	drag(m, m.width-1, 0, m.width-3, 0)
	if m.status != "nothing to copy" {
		t.Fatalf("status: %q", m.status)
	}
	if m.kept == nil || m.kept.x0 != m.width-1 || m.kept.x1 != m.width-3 {
		t.Fatalf("a blank selection is kept too: %+v", m.kept)
	}
}

// marked returns the runs of a frame that markCells drew in reverse
// video: what the selection highlights.
func marked(frame string) []string {
	var out []string
	for {
		_, after, ok := strings.Cut(frame, "\x1b[7m")
		if !ok {
			return out
		}
		run, rest, _ := strings.Cut(after, "\x1b[27m")
		out = append(out, run)
		frame = rest
	}
}

// wantMarked checks the runs a frame shows in reverse video; none when
// want is empty.
func wantMarked(t *testing.T, frame string, want ...string) {
	t.Helper()
	if got := marked(frame); !slices.Equal(got, want) {
		t.Fatalf("highlighted: got %q, want %q", got, want)
	}
}

func wheel(m *Model, b tea.MouseButton) {
	m.Update(tea.MouseMsg{X: 10, Y: 2, Action: tea.MouseActionPress, Button: b})
}

// keptDrag renders the server window with "copy me please" on it, drags
// over "copy me" and checks that the selection stays highlighted after
// the release. It returns the cell where "copy me" starts.
func keptDrag(t *testing.T) (m *Model, x, y int) {
	t.Helper()
	t.Setenv("TERM", "xterm-256color")
	f := newFake()
	f.msgs = append(f.msgs, mk("c1", "9", "alice", "copy me please", 4))
	m = setup(t, f)
	m.clipMode = clipOSC52 // never run a system tool from a test
	m.View()
	x, y = rowOf(t, m, "copy me please")
	drag(m, x, y, x+6, y)
	if m.status != "copied 7 characters via terminal" {
		t.Fatalf("status: %q", m.status)
	}
	wantMarked(t, m.View(), "copy me")
	return m, x, y
}

// TestKeptSelectionGoesAtTheNextClick: the next press removes the kept
// highlight at once, and a plain click leaves nothing highlighted.
func TestKeptSelectionGoesAtTheNextClick(t *testing.T) {
	m, x, y := keptDrag(t)
	mousePress(m, x+9, y, false) // on "please": plain text, no clickable element
	wantMarked(t, m.View())
	mouseRelease(m, x+9, y)
	wantMarked(t, m.View())
	if m.kept != nil {
		t.Fatalf("a click keeps nothing: %+v", m.kept)
	}
	if m.lastSel == nil || m.lastSel.x0 != x+9 || m.lastSel.moved {
		t.Fatalf("the click is still the base of a Shift-click: %+v", m.lastSel)
	}
}

// TestKeptSelectionReplacedByTheNextDrag: the next drag's own selection
// shows while the button is down, and is kept in turn.
func TestKeptSelectionReplacedByTheNextDrag(t *testing.T) {
	m, x, y := keptDrag(t)
	mousePress(m, x+8, y, false)
	wantMarked(t, m.View()) // the old one is gone at the press; nothing selected yet
	m.Update(tea.MouseMsg{X: x + 13, Y: y, Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft})
	wantMarked(t, m.View(), "please")
	mouseRelease(m, x+13, y)
	copied(t, m, "please")
	wantMarked(t, m.View(), "please")
}

// TestKeptSelectionGoesWhenItsRowsChange: the kept highlight is in screen
// cells, so it goes as soon as a row under it reads differently.
func TestKeptSelectionGoesWhenItsRowsChange(t *testing.T) {
	cases := []struct {
		name   string
		change func(m *Model)
	}{
		{"a message arrives", func(m *Model) {
			f := m.opt.Provider.(*fake)
			f.msgs = append(f.msgs, mk("c1", "10", "bob", "a new line", 5))
			m.Update(m.syncCmd()())
		}},
		{"the wheel scrolls", func(m *Model) { wheel(m, tea.MouseButtonWheelUp) }},
		{"PgUp scrolls", func(m *Model) { press(m, tea.KeyPgUp) }},
		{"another window is shown", func(m *Model) { press(m, tea.KeyCtrlN) }},
		{"/list opens", func(m *Model) { enter(m, "/list") }},
		{"/settings opens", func(m *Model) { enter(m, "/settings") }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, _, y := keptDrag(t)
			before := m.plainRow(y)
			c.change(m)
			wantMarked(t, m.View())
			if m.kept != nil {
				t.Fatalf("kept after the change: %+v", m.kept)
			}
			if m.plainRow(y) == before {
				t.Fatalf("the change did not move the row: %q", before)
			}
		})
	}
}

// TestKeptSelectionDoesNotComeBack: once dropped, the highlight stays
// gone when the same text is shown again in the same cells.
func TestKeptSelectionDoesNotComeBack(t *testing.T) {
	cases := []struct {
		name       string
		away, back func(m *Model)
	}{
		{"scrolled back", func(m *Model) { wheel(m, tea.MouseButtonWheelUp) }, func(m *Model) { wheel(m, tea.MouseButtonWheelDown) }},
		{"switched back", func(m *Model) { press(m, tea.KeyCtrlN) }, func(m *Model) { press(m, tea.KeyCtrlP) }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m, x, y := keptDrag(t)
			c.away(m)
			wantMarked(t, m.View())
			c.back(m)
			view := m.View()
			if x2, y2 := rowOf(t, m, "copy me please"); x2 != x || y2 != y {
				t.Fatalf("the text is not back in its cells: %d,%d", x2, y2)
			}
			wantMarked(t, view)
		})
	}
}

// TestWheelAloneKeepsTheSelection: the wheel does not drop the kept
// highlight by itself, only through the scroll it causes.
func TestWheelAloneKeepsTheSelection(t *testing.T) {
	m, _, _ := keptDrag(t)
	wheel(m, tea.MouseButtonWheelDown) // at the bottom already: nothing moves
	wantMarked(t, m.View(), "copy me")
}

// TestKeptSelectionGoesWhenTypedOver: a selection over the input row goes
// with the first key typed into it, wherever in the row it lands.
func TestKeptSelectionGoesWhenTypedOver(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	m := setup(t, newFake())
	m.clipMode = clipOSC52
	m.View()
	y := len(m.screen) - 1 // the input row: the prompt, then the typed text
	if !strings.HasPrefix(m.plainRow(y), "[server]") {
		t.Fatalf("input row: %q", m.plainRow(y))
	}
	drag(m, 1, y, 6, y)
	copied(t, m, "server")
	wantMarked(t, m.View(), "server")
	typ(m, "x")
	wantMarked(t, m.View())
	if m.kept != nil {
		t.Fatalf("kept after typing: %+v", m.kept)
	}
}

// TestKeptSelectionOverTheStatusGoes: the copy's own status message
// changes the rule, so a selection over it is gone at the first frame
// after the release.
func TestKeptSelectionOverTheStatusGoes(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	m := setup(t, newFake())
	m.clipMode = clipOSC52
	m.View()
	// Below the content: the rule, the status line, the window bar, one input row.
	rule := len(m.screen) - 4
	if dash := string(rune(0x2500)); !strings.HasPrefix(m.plainRow(rule), dash+dash) {
		t.Fatalf("rule: %q", m.plainRow(rule))
	}
	drag(m, 2, rule-1, 5, rule)
	if !strings.HasPrefix(m.status, "copied 2 lines via terminal") {
		t.Fatalf("status: %q", m.status)
	}
	wantMarked(t, m.View())
	if m.kept != nil {
		t.Fatalf("kept over a changed rule: %+v", m.kept)
	}
}

// TestKeptSelectionWithClipOff: with /clip off nothing is copied, but the
// highlight still stays on what was selected.
func TestKeptSelectionWithClipOff(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	f := newFake()
	f.msgs = append(f.msgs, mk("c1", "9", "alice", "copy me please", 4))
	m := setup(t, f)
	enter(m, "/clip off")
	m.View()
	x, y := rowOf(t, m, "copy me please")
	status := m.status
	drag(m, x, y, x+6, y)
	if m.clipSeq != "" || m.status != status {
		t.Fatalf("off: copied: %q %q", m.clipSeq, m.status)
	}
	wantMarked(t, m.View(), "copy me")
}

// TestKeptSelectionGoesOnResize: a resize drops the kept highlight even
// where the row under it reads the same.
func TestKeptSelectionGoesOnResize(t *testing.T) {
	m, x, y := keptDrag(t)
	m.Update(tea.WindowSizeMsg{Width: m.width + 10, Height: m.height})
	view := m.View()
	if x2, y2 := rowOf(t, m, "copy me please"); x2 != x || y2 != y {
		t.Fatalf("the text moved: %d,%d", x2, y2)
	}
	wantMarked(t, view)
	if m.kept != nil {
		t.Fatalf("kept after a resize: %+v", m.kept)
	}
}

// TestKeptSelectionGoesWithMouseOff: /mouse off hands selecting back to
// the terminal, so the kept highlight goes though its rows stay.
func TestKeptSelectionGoesWithMouseOff(t *testing.T) {
	m, _, y := keptDrag(t)
	before := m.plainRow(y)
	enter(m, "/mouse")
	if m.mouseOn {
		t.Fatal("/mouse turns reporting off")
	}
	wantMarked(t, m.View())
	if m.plainRow(y) != before {
		t.Fatalf("the row changed: %q", m.plainRow(y))
	}
	if m.kept != nil {
		t.Fatalf("kept with the mouse off: %+v", m.kept)
	}
}

// wantDropped checks that the next frame highlights nothing and that the
// kept selection is gone for good.
func wantDropped(t *testing.T, m *Model) {
	t.Helper()
	wantMarked(t, m.View())
	if m.kept != nil {
		t.Fatalf("still kept: %+v", m.kept)
	}
}

// TestKeptSelectionGoesWhenACursorOnItMoves: a cursor drawn only in
// colour (the settings menu's row, the chosen button of /list, the input
// cursor, the active window's tag) would hide under the highlight, so one
// that moves on a row the selection covers drops it though the row reads
// the same; one that moves on other rows leaves it.
func TestKeptSelectionGoesWhenACursorOnItMoves(t *testing.T) {
	bar := func(m *Model) (int, int) { y := len(m.screen) - 2; return y, y } // one input row below it
	cases := []struct {
		name string
		open func(m *Model) (y0, y1 int) // what to show; the rows to select, as of a View
		move func(m *Model)
		kept bool
	}{
		{"the settings cursor", func(m *Model) (int, int) { enter(m, "/settings"); return 1, 3 },
			func(m *Model) { press(m, tea.KeyDown) }, false},
		{"the settings cursor on other rows", func(m *Model) (int, int) { enter(m, "/settings"); return 5, 6 },
			func(m *Model) { press(m, tea.KeyDown) }, true},
		{"the chosen button", func(m *Model) (int, int) { enter(m, "/list"); return 0, 0 },
			func(m *Model) { press(m, tea.KeyRight) }, false},
		{"the input cursor", func(m *Model) (int, int) {
			typ(m, "hello world")
			m.View()
			y := len(m.screen) - 1
			return y, y
		}, func(m *Model) { press(m, tea.KeyLeft) }, false},
		{"the input cursor on another row", func(m *Model) (int, int) { typ(m, "hello world"); m.View(); return bar(m) },
			func(m *Model) { press(m, tea.KeyLeft) }, true},
		{"the active window's tag", func(m *Model) (int, int) { m.View(); return bar(m) },
			func(m *Model) { press(m, tea.KeyCtrlN) }, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("TERM", "xterm-256color")
			m := setup(t, newFake())
			m.clipMode = clipOSC52
			y0, y1 := c.open(m)
			m.View()
			drag(m, 0, y0, m.width-1, y1)
			if !strings.HasPrefix(m.status, "copied") {
				t.Fatalf("status: %q", m.status)
			}
			if len(marked(m.View())) == 0 {
				t.Fatal("not kept after the release")
			}
			var before []string
			for y := y0; y <= y1; y++ {
				before = append(before, m.plainRow(y))
			}
			c.move(m)
			if !c.kept {
				wantDropped(t, m)
			} else if len(marked(m.View())) == 0 {
				t.Fatal("dropped by a cursor on other rows")
			}
			for i, b := range before {
				if m.plainRow(y0+i) != b {
					t.Fatalf("row %d changed: %q", y0+i, m.plainRow(y0+i))
				}
			}
		})
	}
}

func TestDragOnLinkSelectsInsteadOfOpening(t *testing.T) {
	f := newFake()
	f.msgs = append(f.msgs, mk("c1", "9", "alice", "see https://example.invalid/doc now", 4))
	m := setup(t, f)
	m.clipMode = clipOSC52
	m.View()
	x, y := find(m, spanURL, "https://example.invalid/doc")
	drag(m, x, y, x+5, y)
	if !strings.HasPrefix(m.status, "copied ") {
		t.Fatalf("a drag that starts on a link selects: %q", m.status)
	}
	if strings.Contains(m.clipSeq, base64.StdEncoding.EncodeToString([]byte("https:"))) == false {
		t.Fatalf("the link text was copied: %q", m.clipSeq)
	}
	// A press and release in place is still a click.
	m.status = ""
	if cmd := click(m, x, y); cmd == nil || !strings.HasPrefix(m.status, "opening ") {
		t.Fatalf("click opens: %q", m.status)
	}
	// A release that no press started is nothing.
	m.status = ""
	if _, cmd := m.Update(tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionRelease}); cmd != nil || m.status != "" {
		t.Fatal("stray release")
	}
}

func TestClipCommand(t *testing.T) {
	f := newFake()
	f.msgs = append(f.msgs, mk("c1", "9", "alice", "copy me please", 4))
	m := setup(t, f)
	if m.clipMode != clipAuto {
		t.Fatalf("default mode: %q", m.clipMode)
	}
	enter(m, "/clip")
	if !strings.HasPrefix(m.status, "clipboard: auto (") || m.isErr {
		t.Fatalf("status: %q", m.status)
	}
	enter(m, "/clip bogus")
	if !m.isErr || m.clipMode != clipAuto {
		t.Fatalf("bad mode: %q %v", m.status, m.clipMode)
	}
	enter(m, "/clip OFF")
	if m.clipMode != clipOff || m.isErr {
		t.Fatalf("off: %q %q", m.clipMode, m.status)
	}
	var kept string
	if found, _ := m.opt.Store.State(clipKey, &kept); !found || kept != clipOff {
		t.Fatalf("kept: %v %q", found, kept)
	}
	m.View()
	x, y := rowOf(t, m, "copy me please")
	m.status = ""
	if cmd := drag(m, x, y, x+6, y); cmd != nil || m.status != "" || m.clipSeq != "" {
		t.Fatalf("off: a drag copies nothing: %q", m.status)
	}
	enter(m, "/clip osc52")
	if m.clipMode != clipOSC52 {
		t.Fatalf("osc52: %q", m.clipMode)
	}
	drag(m, x, y, x+6, y)
	if m.status != "copied 7 characters via terminal" {
		t.Fatalf("osc52: %q", m.status)
	}
	// The kept mode survives a restart; an unknown stored value falls back.
	_ = m.opt.Store.SetState(clipKey, "nonsense")
	if n := New(Options{Store: m.opt.Store, Provider: f}); n.clipMode != clipAuto {
		t.Fatalf("fallback: %q", n.clipMode)
	}
	_ = m.opt.Store.SetState(clipKey, clipTool)
	if n := New(Options{Store: m.opt.Store, Provider: f}); n.clipMode != clipTool {
		t.Fatalf("restored: %q", n.clipMode)
	}
}
