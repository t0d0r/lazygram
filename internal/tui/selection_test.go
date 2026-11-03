package tui

import (
	"encoding/base64"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"
)

// pinClock pins the model's clock for the double-click timing; the
// returned func moves it on.
func pinClock(m *Model) func(time.Duration) {
	at := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	m.now = func() time.Time { return at }
	return func(d time.Duration) { at = at.Add(d) }
}

func mousePress(m *Model, x, y int, shift bool) {
	m.Update(tea.MouseMsg{X: x, Y: y, Shift: shift, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
}

func mouseRelease(m *Model, x, y int) tea.Cmd {
	_, cmd := m.Update(tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionRelease, Button: tea.MouseButtonNone})
	return cmd
}

// multiClick clicks n times on one cell, 150 ms apart, and renders after
// every release as the program does; it returns the last release's
// command.
func multiClick(m *Model, step func(time.Duration), n, x, y int) tea.Cmd {
	var cmd tea.Cmd
	for i := range n {
		if i > 0 {
			step(150 * time.Millisecond)
		}
		mousePress(m, x, y, false)
		cmd = mouseRelease(m, x, y)
		m.View()
	}
	return cmd
}

// shiftClick presses and releases with Shift held.
func shiftClick(m *Model, x, y int) tea.Cmd {
	mousePress(m, x, y, true)
	cmd := mouseRelease(m, x, y)
	m.View()
	return cmd
}

func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

// Text beyond ASCII, kept as escapes so this file stays plain ASCII.
const (
	cjk    = "\U000065E5\U0000672C\U00008A9E\U00003067\U00003059" // five CJK characters, two cells each
	cjk2   = "\U0000672C"                                         // the second of them
	coder  = "\U0001F469\U0000200D\U0001F4BB"                     // an emoji ZWJ sequence: three runes, one character in two cells
	thumbs = "\U0001F44D\U0001F3FD"                               // an emoji with a skin tone: two runes, one character
	acute  = "\U00000301"                                         // a combining acute accent
	nbsp   = "\U000000A0"                                         // a no-break space
)

// copied checks that text went to the clipboard through the terminal.
func copied(t *testing.T, m *Model, text string) {
	t.Helper()
	if !strings.HasPrefix(m.clipSeq, "\x1b]52;c;"+b64(text)) {
		t.Fatalf("clipboard: want %q, status %q", text, m.status)
	}
}

func TestWordAt(t *testing.T) {
	cases := []struct {
		row  string
		at   string // the text the pointer is on
		off  int    // cells into it
		want string // "" when there is no word
	}{
		{"see (https://example.invalid/a_(b)). ok", "example", 0, "https://example.invalid/a_(b)"},
		{"12:00 <ada> hi", "ada", 1, "ada"},
		{"12:00 <ada> hi", "<ada>", 0, "<ada>"}, // on a trimmed bracket: the whole run
		{"@alan, are you there?", "alan", 0, "@alan"},
		{"#dev: build is green", "#dev", 0, "#dev"},
		{`he said "fine," and left`, "fine", 2, "fine"},
		{"read about Go_(game) today", "game", 0, "Go_(game)"},
		{"(see this)", "see", 0, "see"},
		{"(see this)", "this", 1, "this"},
		{"wow!!! nice", "wow", 0, "wow"},
		{"really?! ok", "really", 0, "really"},
		{"[photo] look", "photo", 0, "photo"},
		{"`make check` passes", "make", 0, "make"},
		{"wait ... what", "...", 1, "..."}, // nothing left after trimming: the whole run
		{"at 12:34 sharp", "12:34", 2, "12:34"},
		{"ada " + cjk + " ok", cjk2, 1, cjk}, // the second cell of a wide character
		{"hi " + coder + " there", coder, 1, coder},
		{coder + " ada", "ada", 0, "ada"}, // the columns after an emoji sequence stay right
		{"nice" + thumbs + "work, ada", "work", 0, "nice" + thumbs + "work"},
		{"cafe" + acute + " au lait", "cafe", 3, "cafe" + acute}, // a combining mark stays with its base
		{"a" + nbsp + "b", "a", 0, "a"},                          // a no-break space is blank
		{"two  spaces", "  ", 1, ""},                             // on a blank
	}
	for _, c := range cases {
		x := ansi.StringWidth(c.row[:strings.Index(c.row, c.at)]) + c.off
		from, to, ok := wordAt(c.row, x)
		got := ""
		if ok {
			got = cells(c.row, from, to)
		}
		if got != c.want || ok != (c.want != "") {
			t.Errorf("%q at %d: got %q (%d-%d, %v), want %q", c.row, x, got, from, to, ok, c.want)
		}
	}
	if _, _, ok := wordAt("short", 40); ok {
		t.Error("past the end of the text is no word")
	}
	// A drag reads the same cells: an emoji sequence takes two, not four.
	if got := cells(coder+" ada", 3, 5); got != "ada" {
		t.Errorf("cells after an emoji sequence: %q", got)
	}
}

func TestDoubleClickCopiesTheWord(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	f := newFake()
	f.msgs = append(f.msgs, mk("c1", "9", "ada", "please copy (this-word), thanks", 4))
	m := setup(t, f)
	m.clipMode = clipOSC52 // never run a system tool from a test
	step := pinClock(m)
	m.View()
	x, y := rowOf(t, m, "this-word")
	x += 3

	m.status = ""
	if cmd := multiClick(m, step, 1, x, y); cmd != nil || m.status != "" || m.clipSeq != "" {
		t.Fatalf("the first click is a click: %q", m.status)
	}
	step(150 * time.Millisecond)
	mousePress(m, x, y, false)
	if m.sel == nil || m.sel.unit != byWord {
		t.Fatalf("the second press selects the word: %+v", m.sel)
	}
	// While the button is down the word shows in reverse video, as a drag does.
	if row := strings.Split(m.View(), "\n")[y]; !strings.Contains(row, "\x1b[7mthis-word\x1b[27m") {
		t.Fatalf("highlighted word: %q", row)
	}
	// The pointer moving before the release does not change the word.
	m.Update(tea.MouseMsg{X: x + 8, Y: y, Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft})
	cmd := mouseRelease(m, x+8, y)
	if m.sel != nil || cmd == nil {
		t.Fatal("the release copies and ends the selection")
	}
	if m.status != "copied 9 characters via terminal" || m.isErr {
		t.Fatalf("status: %q", m.status)
	}
	copied(t, m, "this-word")
	// The word stays highlighted after the release, as a drag does.
	wantMarked(t, m.View(), "this-word")

	// A double click on a blank cell has nothing to copy.
	step(time.Second)
	multiClick(m, step, 2, m.width-1, 0)
	if m.status != "nothing to copy" {
		t.Fatalf("blank: %q", m.status)
	}

	// /clip off: a double click selects and copies nothing, like a drag.
	enter(m, "/clip off")
	m.clipSeq, m.status = "", ""
	step(time.Second)
	if cmd := multiClick(m, step, 2, x, y); cmd != nil || m.clipSeq != "" || m.status != "" {
		t.Fatalf("clip off: %q", m.status)
	}
}

func TestTripleClickCopiesTheRow(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	f := newFake()
	f.msgs = append(f.msgs, mk("c1", "9", "ada", "the whole row goes", 4))
	m := setup(t, f)
	m.clipMode = clipOSC52
	step := pinClock(m)
	m.View()
	x, y := rowOf(t, m, "whole")
	want := strings.TrimRight(m.plainRow(y), " ")

	multiClick(m, step, 2, x, y)
	copied(t, m, "whole")
	step(150 * time.Millisecond)
	mousePress(m, x, y, false)
	if m.sel == nil || m.sel.unit != byRow {
		t.Fatalf("the third press selects the row: %+v", m.sel)
	}
	if row := strings.Split(m.View(), "\n")[y]; !strings.HasPrefix(row, "\x1b[7m") || ansi.Strip(row) != m.plainRow(y) {
		t.Fatalf("the row shows in reverse video from its first cell: %q", row)
	}
	mouseRelease(m, x, y)
	if m.status != "copied "+strconv.Itoa(uniseg.GraphemeClusterCount(want))+" characters via terminal" {
		t.Fatalf("status: %q", m.status)
	}
	copied(t, m, want)
	// The row stays highlighted after the release.
	wantMarked(t, m.View(), m.plainRow(y))

	// More presses in the same run keep selecting the row.
	step(150 * time.Millisecond)
	mousePress(m, x, y, false)
	if m.sel == nil || m.sel.unit != byRow {
		t.Fatalf("a fourth press: %+v", m.sel)
	}
	mouseRelease(m, x, y)
	copied(t, m, want)
}

func TestDoubleClickTiming(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	f := newFake()
	f.msgs = append(f.msgs, mk("c1", "9", "ada", "timing matters here", 4))
	m := setup(t, f)
	m.clipMode = clipOSC52
	step := pinClock(m)
	m.View()
	x, y := rowOf(t, m, "matters")
	x += 2
	single := func(what string) {
		t.Helper()
		if m.sel == nil || m.sel.unit != byCell || m.multi.n != 1 {
			t.Fatalf("%s: a single click, got %+v (n=%d)", what, m.sel, m.multi.n)
		}
		mouseRelease(m, m.sel.x0, m.sel.y0)
		m.View()
	}

	multiClick(m, step, 1, x, y)
	step(clickGap + time.Millisecond)
	mousePress(m, x, y, false)
	single("a pause longer than clickGap")

	step(clickGap)
	mousePress(m, x, y, false)
	if m.sel == nil || m.sel.unit != byWord {
		t.Fatalf("clickGap exactly is still a double click: %+v", m.sel)
	}
	mouseRelease(m, x, y)
	m.View()

	step(2 * clickGap)
	multiClick(m, step, 1, x, y)
	step(100 * time.Millisecond)
	mousePress(m, x+1, y, false)
	single("the next cell")

	// A drag is no first click: a quick press where it started is a click.
	step(2 * clickGap)
	drag(m, x, y, x+4, y)
	m.View()
	step(100 * time.Millisecond)
	mousePress(m, x, y, false)
	single("after a drag")

	// Nor does a Shift-click take part in one: click, Shift-click, click,
	// all on one cell and quick, is no double click.
	step(2 * clickGap)
	multiClick(m, step, 1, x, y)
	step(100 * time.Millisecond)
	shiftClick(m, x, y)
	step(100 * time.Millisecond)
	mousePress(m, x, y, false)
	single("after a Shift-click")
}

func TestMultiClickOnALinkOpensItOnce(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	const link = "https://example.invalid/doc"
	f := newFake()
	f.msgs = append(f.msgs, mk("c1", "9", "ada", "docs at "+link+" today", 4))
	m := setup(t, f)
	var opened []string
	m.opt.Open = func(s string) error { opened = append(opened, s); return nil }
	m.clipMode = clipOSC52
	step := pinClock(m)
	m.View()
	x, y := find(m, spanURL, link)
	x += 4

	// The first click acts on release, as a single click does.
	mousePress(m, x, y, false)
	cmd := mouseRelease(m, x, y)
	if cmd == nil || m.status != "opening "+link+"..." {
		t.Fatalf("first click: %q", m.status)
	}
	m.Update(cmd())
	m.View()
	// The second press selects the link instead of opening it again.
	step(150 * time.Millisecond)
	mousePress(m, x, y, false)
	mouseRelease(m, x, y)
	if m.status != "copied 27 characters via terminal" {
		t.Fatalf("second click: %q", m.status)
	}
	copied(t, m, link)
	m.View()
	// The third selects the row.
	step(150 * time.Millisecond)
	row := strings.TrimRight(m.plainRow(y), " ")
	mousePress(m, x, y, false)
	mouseRelease(m, x, y)
	copied(t, m, row)
	if len(opened) != 1 || opened[0] != link {
		t.Fatalf("opened once: %v", opened)
	}
}

func TestMultiClickThatReplacedTheScreenDoesNothingMore(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	m := setup(t, newFake())
	m.clipMode = clipOSC52
	step := pinClock(m)
	m.View()

	// A chat name in the server window: the first click opens the window,
	// the rest of the double click neither selects on it nor clicks there.
	x, y := find(m, spanChat, "c1")
	multiClick(m, step, 1, x, y)
	if m.winName(m.cur) != "#dev" {
		t.Fatalf("first click: %s", m.winName(m.cur))
	}
	cur := m.cur
	for range 2 {
		step(150 * time.Millisecond)
		mousePress(m, x, y, false)
		if m.sel != nil {
			t.Fatalf("a later press on the new screen: %+v", m.sel)
		}
		if cmd := mouseRelease(m, x, y); cmd != nil || m.cur != cur || strings.HasPrefix(m.status, "copied") {
			t.Fatalf("the later presses do nothing: cur=%d status=%q", m.cur, m.status)
		}
		m.View()
	}
	// After a pause a press is a click again.
	step(2 * clickGap)
	mousePress(m, x, y, false)
	if m.sel == nil || m.sel.unit != byCell {
		t.Fatalf("a fresh click: %+v", m.sel)
	}
	mouseRelease(m, x, y)

	// A /list row: opened once, nothing copied.
	enter(m, "/list")
	m.View()
	x, y = find(m, spanRow, "2") // #news
	step(time.Second)
	multiClick(m, step, 3, x, y)
	if m.mode != modeWin || m.winName(m.cur) != "#news" || strings.HasPrefix(m.status, "copied") {
		t.Fatalf("row: mode=%v win=%s status=%q", m.mode, m.winName(m.cur), m.status)
	}
}

func TestShiftClickExtendsTheSelection(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	f := newFake()
	f.msgs = append(f.msgs, mk("c1", "9", "ada", "alpha beta gamma delta", 4), mk("c1", "10", "ada", "see https://example.invalid/x", 5))
	m := setup(t, f)
	var opened []string
	m.opt.Open = func(s string) error { opened = append(opened, s); return nil }
	m.clipMode = clipOSC52
	step := pinClock(m)
	m.View()

	// With nothing selected yet a Shift-click is a plain click.
	lx, ly := find(m, spanURL, "https://example.invalid/x")
	if cmd := shiftClick(m, lx, ly); cmd == nil || !strings.HasPrefix(m.status, "opening ") {
		t.Fatalf("first Shift-click: %q", m.status)
	}

	x, y := rowOf(t, m, "alpha beta gamma delta") // alpha x..x+4, beta x+6..x+9, gamma x+11..x+15, delta x+17..x+21
	step(time.Second)
	multiClick(m, step, 1, x+6, y)
	// From the click's cell to the Shift-clicked one, shown while the button is down.
	step(time.Second)
	mousePress(m, x+15, y, true)
	if m.sel == nil || m.sel.unit != byShift {
		t.Fatalf("Shift press: %+v", m.sel)
	}
	if row := strings.Split(m.View(), "\n")[y]; !strings.Contains(row, "\x1b[7mbeta gamma\x1b[27m") {
		t.Fatalf("highlighted extension: %q", row)
	}
	mouseRelease(m, x+15, y)
	if m.status != "copied 10 characters via terminal" {
		t.Fatalf("status: %q", m.status)
	}
	copied(t, m, "beta gamma")
	wantMarked(t, m.View(), "beta gamma") // the extension stays highlighted
	// Another Shift-click moves the end from the same anchor, either way,
	// and its result replaces the highlight.
	step(time.Second)
	shiftClick(m, x+21, y)
	copied(t, m, "beta gamma delta")
	wantMarked(t, m.View(), "beta gamma delta")
	step(time.Second)
	shiftClick(m, x, y)
	copied(t, m, "alpha b")
	wantMarked(t, m.View(), "alpha b")

	// A drag's anchor is where it started; the pointer moving with the
	// button down after a Shift press drags the end.
	step(time.Second)
	drag(m, x+11, y, x+13, y)
	copied(t, m, "gam")
	step(time.Second)
	mousePress(m, x+13, y, true)
	m.Update(tea.MouseMsg{X: x + 21, Y: y, Action: tea.MouseActionMotion, Button: tea.MouseButtonLeft})
	mouseRelease(m, x+21, y)
	copied(t, m, "gamma delta")

	// A word stays whole whichever side the Shift-click is on.
	step(time.Second)
	multiClick(m, step, 2, x+8, y)
	copied(t, m, "beta")
	step(time.Second)
	shiftClick(m, x+21, y)
	copied(t, m, "beta gamma delta")
	step(time.Second)
	shiftClick(m, x, y)
	copied(t, m, "alpha beta")

	// A row too: a Shift-click on the row above takes it from that cell to
	// the end of the row.
	step(time.Second)
	multiClick(m, step, 3, x+2, y)
	step(time.Second)
	shiftClick(m, 3, y-1)
	want := selectedText(plainRows(m.screen), selection{x0: 3, y0: y - 1, x1: m.width - 1, y1: y})
	if !strings.HasPrefix(m.status, "copied 2 lines via terminal") {
		t.Fatalf("row extension: %q", m.status)
	}
	copied(t, m, want)

	// Nothing was opened on the way, and /clip off copies nothing.
	if len(opened) != 0 {
		t.Fatalf("opened: %v", opened)
	}
	enter(m, "/clip off")
	m.clipSeq, m.status = "", ""
	step(time.Second)
	if cmd := shiftClick(m, x+21, y); cmd != nil || m.clipSeq != "" || m.status != "" {
		t.Fatalf("clip off: %q", m.status)
	}
}

func TestDoubleClickOnWideCharacters(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	f := newFake()
	f.msgs = append(f.msgs, mk("c1", "9", "ada", "ada "+cjk+" and "+coder+" ok", 4))
	m := setup(t, f)
	m.clipMode = clipOSC52
	step := pinClock(m)
	m.View()

	x, y := rowOf(t, m, cjk)
	multiClick(m, step, 1, x+3, y) // the second cell of the second character
	step(150 * time.Millisecond)
	mousePress(m, x+3, y, false)
	if row := strings.Split(m.View(), "\n")[y]; !strings.Contains(row, "\x1b[7m"+cjk+"\x1b[27m") {
		t.Fatalf("highlighted wide word: %q", row)
	}
	mouseRelease(m, x+3, y)
	if m.status != "copied 5 characters via terminal" {
		t.Fatalf("status: %q", m.status)
	}
	copied(t, m, cjk)

	step(time.Second)
	x, y = rowOf(t, m, coder)
	multiClick(m, step, 2, x+1, y)
	if m.status != "copied 1 character via terminal" {
		t.Fatalf("an emoji sequence is one character: %q", m.status)
	}
	copied(t, m, coder)
	step(time.Second)
	multiClick(m, step, 2, x+3, y) // the word after it: the columns stay right
	copied(t, m, "ok")
}

// TestShiftClickInsideAWordOrRowKeepsTheFarEnd: a Shift-click inside the
// word or row just selected anchors at its end farther from the click.
func TestShiftClickInsideAWordOrRowKeepsTheFarEnd(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	f := newFake()
	f.msgs = append(f.msgs, mk("c1", "9", "ada", "alpha beta gamma delta", 4))
	m := setup(t, f)
	m.clipMode = clipOSC52
	step := pinClock(m)
	m.View()
	x, y := rowOf(t, m, "alpha beta gamma delta") // gamma x+11..x+15
	multiClick(m, step, 2, x+12, y)
	copied(t, m, "gamma")
	step(time.Second)
	shiftClick(m, x+11, y) // on its first letter: from its last
	copied(t, m, "gamma")
	step(time.Second)
	row := strings.TrimRight(plainRows(m.screen)[y], " ")
	multiClick(m, step, 3, x+2, y)
	copied(t, m, row)
	step(time.Second)
	shiftClick(m, x+6, y) // nearer the row's start: from its end
	copied(t, m, strings.TrimRight(row[x+6:], " "))
}

// TestWordCopyDroppedWhenTheScreenChanged: a word whose row changed
// between the second press and its release is not copied, and nothing
// stays highlighted: what is there now was never selected.
func TestWordCopyDroppedWhenTheScreenChanged(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	f := newFake()
	f.msgs = append(f.msgs, mk("c1", "9", "ada", "alpha beta gamma delta", 4))
	m := setup(t, f)
	m.clipMode = clipOSC52
	step := pinClock(m)
	m.View()
	x, y := rowOf(t, m, "alpha beta gamma delta")
	multiClick(m, step, 1, x+12, y)
	step(150 * time.Millisecond)
	mousePress(m, x+12, y, false) // the word is selected on this press
	for i := 0; i < 3; i++ {      // and then the screen moves
		m.note(m.wins[m.cur], "something new arrived")
	}
	m.View()
	mouseRelease(m, x+12, y)
	if m.clipSeq != "" || m.status != "nothing copied: the screen changed" {
		t.Fatalf("copied from the new screen: %q %q", m.clipSeq, m.status)
	}
	wantMarked(t, m.View())
	if m.kept != nil {
		t.Fatalf("kept on the new screen: %+v", m.kept)
	}

	// The same with /clip off: nothing is said, and nothing is kept.
	enter(m, "/clip off")
	m.View()
	x, y = rowOf(t, m, "alpha beta gamma delta")
	step(time.Second)
	multiClick(m, step, 1, x+12, y)
	step(150 * time.Millisecond)
	mousePress(m, x+12, y, false)
	m.note(m.wins[m.cur], "something else arrived")
	m.View()
	status := m.status
	mouseRelease(m, x+12, y)
	if m.status != status {
		t.Fatalf("clip off says nothing: %q", m.status)
	}
	wantMarked(t, m.View())
	if m.kept != nil {
		t.Fatalf("clip off: kept on the new screen: %+v", m.kept)
	}
}
