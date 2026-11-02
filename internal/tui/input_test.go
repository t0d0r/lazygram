package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// chatOn is a model on a screen of the given size in @alice's window,
// where Enter adds a line break to the message.
func chatOn(t *testing.T, width, height int) *Model {
	t.Helper()
	m := setup(t, newFake())
	m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	enter(m, "/win @alice")
	return m
}

// shown is the last frame as the terminal shows it, without styling: the
// renderer keeps only the bottom height rows of a taller frame, so a frame
// with a row too many would shift every row the clicks map.
func shown(t *testing.T, m *Model) []string {
	t.Helper()
	rows := strings.Split(ansi.Strip(m.View()), "\n")
	if len(rows) != m.height {
		t.Fatalf("the frame has %d rows on a screen of %d", len(rows), m.height)
	}
	return rows
}

// inputShown is the input area of the last frame, trailing blanks (the
// cursor's cell past the text) dropped.
func inputShown(t *testing.T, m *Model, n int) string {
	t.Helper()
	rows := shown(t, m)
	var out []string
	for _, r := range rows[len(rows)-n:] {
		out = append(out, strings.TrimRight(r, " "))
	}
	return strings.Join(out, "\n")
}

func paste(m *Model, text string) {
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(text), Paste: true})
}

// numbered is the lines r<from>..r<to>, each after indent, joined by line
// breaks.
func numbered(indent string, from, to int) string {
	var ls []string
	for i := from; i <= to; i++ {
		ls = append(ls, fmt.Sprintf("%sr%d", indent, i))
	}
	return strings.Join(ls, "\n")
}

const pad = "         " // as wide as the prompt "[@alice] "

func TestInputAreaGrowsUpward(t *testing.T) {
	m := chatOn(t, 100, 30)
	// frame checks that the rule, the status line and the window bar sit
	// right above an input area of n rows, and the messages fill the rest.
	frame := func(n int) []string {
		t.Helper()
		rows := shown(t, m)
		h := 30 - 3 - n
		if m.contentHeight() != h || m.inputHeight() != n {
			t.Fatalf("content %d, input %d; want %d, %d", m.contentHeight(), m.inputHeight(), h, n)
		}
		if !strings.HasPrefix(rows[h], "\u2500\u2500") || !strings.Contains(rows[h+1], "  fake  [") ||
			!strings.HasPrefix(rows[h+2], " [0:server]") {
			t.Fatalf("rule, status and bar above the input:\n%s", strings.Join(rows[h:h+3], "\n"))
		}
		return rows
	}

	// One row: the screen as it always was.
	typ(m, "hello")
	rows := frame(1)
	if rows[29] != "[@alice] hello " || m.contentHeight() != m.height-4 {
		t.Fatalf("one row: %q", rows[29])
	}
	newest := rows[25] // the bottom message row

	// Three rows: each line on its own row, the later ones lined up with
	// the first; the messages give up two rows and still end at the
	// newest line.
	press(m, tea.KeyEnter)
	typ(m, "world")
	press(m, tea.KeyEnter)
	typ(m, "again")
	rows = frame(3)
	if got := strings.Join(rows[27:], "\n"); got != "[@alice] hello\n"+pad+"world\n"+pad+"again " {
		t.Fatalf("three rows:\n%s", got)
	}
	if rows[23] != newest {
		t.Fatalf("the messages still end at the newest line: %q, want %q", rows[23], newest)
	}

	// Twelve rows: the area stops at eight and shows the cursor's row; the
	// prompt's row has scrolled away.
	press(m, tea.KeyCtrlU)
	paste(m, numbered("", 1, 12))
	frame(8)
	if got := inputShown(t, m, 8); got != numbered(pad, 5, 12) {
		t.Fatalf("capped at eight rows, the cursor's in view:\n%s", got)
	}
	// Moving up inside the area does not scroll it; leaving it scrolls
	// one row at a time.
	for range 7 {
		press(m, tea.KeyUp)
		m.View()
	}
	if got := inputShown(t, m, 8); got != numbered(pad, 5, 12) || cursorRow(inputRows(m.input, 90), m.pos()) != 4 {
		t.Fatalf("the cursor moved up inside the area:\n%s", got)
	}
	press(m, tea.KeyUp)
	if got := inputShown(t, m, 8); got != numbered(pad, 4, 11) {
		t.Fatalf("one row up past the area:\n%s", got)
	}
	for range 3 {
		press(m, tea.KeyUp)
		m.View()
	}
	if got := inputShown(t, m, 8); got != "[@alice] r1\n"+numbered(pad, 2, 8) {
		t.Fatalf("back at the first row, the prompt shows:\n%s", got)
	}
	frame(8)
}

func TestInputAreaCap(t *testing.T) {
	for _, c := range []struct{ height, rows int }{{30, 8}, {24, 8}, {20, 6}, {12, 4}, {5, 1}} {
		m := chatOn(t, 100, c.height)
		paste(m, numbered("", 1, 12))
		if got := m.inputHeight(); got != c.rows || m.contentHeight() != max(c.height-3-c.rows, 1) {
			t.Errorf("height %d: %d input rows, want %d; %d message rows", c.height, got, c.rows, m.contentHeight())
		}
		if got := len(strings.Split(m.View(), "\n")); got != c.height {
			t.Errorf("height %d: the frame has %d rows", c.height, got)
		}
	}
}

// TestFrameHeight pins the rule the click map depends on: from 5 rows up
// the frame fills the screen exactly whatever the input, and a smaller
// screen still gets the 5-row frame (a message row, rule, status line,
// window bar and an input row), whose top rows the terminal hides.
func TestFrameHeight(t *testing.T) {
	for _, lines := range []int{1, 3, 12} {
		for height := 1; height <= 40; height++ {
			m := chatOn(t, 40, height)
			paste(m, numbered("", 1, lines))
			frame := strings.Split(m.View(), "\n")
			if want := max(height, 5); len(frame) != want || len(m.clicks) != want {
				t.Errorf("%d input lines, height %d: frame %d rows, click map %d; want %d",
					lines, height, len(frame), len(m.clicks), want)
			}
		}
	}
	// The list filter wraps rather than taking line breaks; it is capped
	// the same way.
	for height := 1; height <= 40; height++ {
		m := chatOn(t, 20, height)
		enter(m, "/list")
		typ(m, strings.Repeat("x", 200))
		if got, want := len(strings.Split(m.View(), "\n")), max(height, 5); got != want || m.mode != modeList {
			t.Errorf("list filter, height %d: frame %d rows, want %d", height, got, want)
		}
		if height == 30 && m.inputHeight() != 8 { // 200 cells in rows of 12 would be 17 rows
			t.Errorf("list filter: %d rows, want the cap of 8", m.inputHeight())
		}
	}
}

func TestInputRowsWrap(t *testing.T) {
	m := setup(t, newFake())
	m.width = 20 // with the prompt "> " a row holds 17 cells and the cursor's cell
	area := func() []string {
		t.Helper()
		var out []string
		for _, r := range m.inputArea("> ") {
			if w := lipgloss.Width(r); w > m.width {
				t.Fatalf("a row wider than the screen: %q (%d)", r, w)
			}
			out = append(out, ansi.Strip(r))
		}
		return out
	}
	x16 := strings.Repeat("x", 16)
	cases := []struct {
		in   string
		want []string
	}{
		{strings.Repeat("x", 17), []string{"> " + strings.Repeat("x", 17) + " "}},
		{strings.Repeat("x", 18), []string{"> " + strings.Repeat("x", 17), "  x "}},
		// A wide character that does not fit goes to the next row whole.
		{x16 + cjk, []string{"> " + x16, "  " + cjk + " "}},
		// So does an emoji sequence of several runes.
		{x16 + coder + "y", []string{"> " + x16, "  " + coder + "y "}},
		// A letter with a combining accent is one character.
		{x16 + "e" + acute + "z", []string{"> " + x16 + "e" + acute, "  z "}},
		// A line break starts a row; an empty line is a row too.
		{"ab\n\ncd", []string{"> ab", "  ", "  cd "}},
	}
	for _, c := range cases {
		m.input, m.curBack = []rune(c.in), 0
		if got := area(); strings.Join(got, "\n") != strings.Join(c.want, "\n") {
			t.Errorf("%q:\n%s\nwant:\n%s", c.in, strings.Join(got, "\n"), strings.Join(c.want, "\n"))
		}
	}
	// A screen too narrow for a wide character still never gets a row
	// wider than itself.
	m.width = 4
	m.input, m.curBack = []rune(cjk), 0
	if got := area(); len(got) != 5 {
		t.Fatalf("one wide character a row: %q", got)
	}
}

func TestInputCursorRowAndColumn(t *testing.T) {
	rows := inputRows([]rune("ab\n\ncdefgh"), 4)
	want := []inRow{{start: 0, end: 2, last: true}, {start: 3, end: 3, last: true}, {start: 4, end: 8}, {start: 8, end: 10, last: true}}
	if len(rows) != len(want) {
		t.Fatalf("rows: %+v", rows)
	}
	for i, r := range rows {
		if r.start != want[i].start || r.end != want[i].end || r.last != want[i].last {
			t.Fatalf("row %d: %+v, want %+v", i, r, want[i])
		}
	}
	for _, c := range []struct{ p, row, col int }{
		{0, 0, 0}, {2, 0, 2}, // past the text of "ab": on its row
		{3, 1, 0}, // the empty line
		{4, 2, 0}, {7, 2, 3},
		{8, 3, 0}, // where a line wraps: at the start of the next row
		{10, 3, 2},
	} {
		r := cursorRow(rows, c.p)
		if r != c.row || rows[r].col(c.p) != c.col {
			t.Errorf("cursor %d: row %d col %d, want row %d col %d", c.p, r, rows[r].col(c.p), c.row, c.col)
		}
	}
	// A wrapped row ends before its last character, a line's last row past
	// its text.
	if rows[2].at(10) != 7 || rows[3].at(10) != 10 || rows[1].at(5) != 3 || rows[0].at(1) != 1 {
		t.Fatalf("at: %d %d %d %d", rows[2].at(10), rows[3].at(10), rows[1].at(5), rows[0].at(1))
	}
	// Wide characters: a column inside one goes to its start.
	rows = inputRows([]rune("a"+cjk), 10)
	if len(rows) != 2 || rows[0].end != 5 || rows[0].at(2) != 1 || rows[0].col(2) != 3 {
		t.Fatalf("wide: %+v at %d col %d", rows, rows[0].at(2), rows[0].col(2))
	}
	// A cursor inside an emoji sequence is drawn on the whole character.
	in := []rune("x" + coder)
	rows = inputRows(in, 10)
	if p := len(in) - 1; rows[0].under(p) != 1 || rows[0].col(p) != 1 || rows[0].under(len(in)) != 2 {
		t.Fatalf("emoji: under %d col %d", rows[0].under(p), rows[0].col(p))
	}
}

func TestUpDownMoveBetweenInputRows(t *testing.T) {
	m := chatOn(t, 30, 30) // after the prompt "[@alice] " a row holds 20 cells
	a25 := strings.Repeat("a", 25)
	paste(m, "the first line\n2nd\n"+a25+"\nthe last line of all")
	at := func(row, col int) {
		t.Helper()
		m.View()
		rows, cur, _, _ := m.inputLayout(m.prompt())
		if cur != row || rows[cur].col(m.pos()) != col {
			t.Fatalf("cursor on row %d col %d, want row %d col %d", cur, rows[cur].col(m.pos()), row, col)
		}
	}
	at(4, 20)
	press(m, tea.KeyUp)
	at(3, 5) // the rest of the wrapped line: as far as it allows
	press(m, tea.KeyUp)
	at(2, 19) // a wrapped row: on its last character, the column kept
	press(m, tea.KeyUp)
	at(1, 3)
	press(m, tea.KeyUp)
	at(0, 14)
	// Up on the first row walks the history; Down brings the draft back.
	press(m, tea.KeyUp)
	if string(m.input) != "/win @alice" {
		t.Fatalf("Up on the first row recalls: %q", string(m.input))
	}
	press(m, tea.KeyDown)
	if !strings.HasPrefix(string(m.input), "the first line\n") {
		t.Fatalf("Down brings the draft back: %q", string(m.input))
	}
	at(4, 20)
	// From the start, Down keeps column 0 on every row, then on the last
	// row it walks the history, which has nothing newer: the draft stays.
	m.curBack = len(m.input)
	draft := string(m.input)
	for row := 1; row <= 4; row++ {
		press(m, tea.KeyDown)
		at(row, 0)
	}
	press(m, tea.KeyDown)
	if string(m.input) != draft {
		t.Fatalf("Down past the last row keeps the draft: %q", string(m.input))
	}
	at(4, 0)
	// The kept column ends with any other key.
	press(m, tea.KeyCtrlE)
	press(m, tea.KeyUp)
	at(3, 5)
	press(m, tea.KeyLeft)
	press(m, tea.KeyUp)
	at(2, 4) // from column 4, not the 20 kept before
}

func TestUpDownWithOneRowWalkTheHistory(t *testing.T) {
	m := chatOn(t, 100, 30)
	enter(m, "/stats")
	typ(m, "draft")
	press(m, tea.KeyDown) // no walk yet: nothing newer, the text stays
	if string(m.input) != "draft" {
		t.Fatalf("Down before any Up: %q", string(m.input))
	}
	press(m, tea.KeyUp)
	if string(m.input) != "/stats" {
		t.Fatalf("Up recalls: %q", string(m.input))
	}
	press(m, tea.KeyDown)
	if string(m.input) != "draft" || m.pos() != 5 {
		t.Fatalf("Down brings the draft back: %q", string(m.input))
	}
	press(m, tea.KeyDown)
	if string(m.input) != "draft" {
		t.Fatalf("Down with nothing newer keeps the text: %q", string(m.input))
	}
}

func TestLineKeysActOnTheCursorLine(t *testing.T) {
	m := chatOn(t, 100, 30)
	paste(m, "one two\nthree four\nfive")
	in := string(m.input)
	m.curBack = len(m.input) - strings.Index(in, "four")
	press(m, tea.KeyCtrlA)
	if m.pos() != strings.Index(in, "three") {
		t.Fatalf("Ctrl-A: %d", m.pos())
	}
	press(m, tea.KeyCtrlE)
	if m.pos() != strings.Index(in, "\nfive") {
		t.Fatalf("Ctrl-E: %d", m.pos())
	}
	m.curBack = len(m.input) - strings.Index(in, " four")
	press(m, tea.KeyCtrlK)
	if got := string(m.input); got != "one two\nthree\nfive" || m.pos() != 13 {
		t.Fatalf("Ctrl-K: %q at %d", got, m.pos())
	}
	press(m, tea.KeyCtrlK) // at the end of the line: nothing to delete
	if got := string(m.input); got != "one two\nthree\nfive" {
		t.Fatalf("Ctrl-K at the end of a line: %q", got)
	}
	m.curBack = 0
	press(m, tea.KeyCtrlA)
	if m.pos() != 14 {
		t.Fatalf("Ctrl-A on the last line: %d", m.pos())
	}
	m.curBack = len(m.input) - 2
	press(m, tea.KeyCtrlA)
	if m.pos() != 0 {
		t.Fatalf("Ctrl-A on the first line: %d", m.pos())
	}
}

func TestPasteWithLineBreaksShowsRows(t *testing.T) {
	m := chatOn(t, 100, 30)
	typ(m, "say:")
	paste(m, " first\r\nsecond\nthird")
	if got := inputShown(t, m, 3); got != "[@alice] say: first\n"+pad+"second\n"+pad+"third" {
		t.Fatalf("pasted lines:\n%s", got)
	}
	if rows, cur, _, _ := m.inputLayout(m.prompt()); cur != 2 || rows[cur].col(m.pos()) != 5 {
		t.Fatalf("cursor after the paste: row %d", cur)
	}
	if m.contentHeight() != 30-3-3 {
		t.Fatalf("message area: %d rows", m.contentHeight())
	}
}

func TestClicksAndSelectionWithATallInput(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	const link = "https://example.invalid/doc"
	f := newFake()
	f.msgs = append(f.msgs, mk("c1", "9", "alice", "read "+link+" now", 4))
	m := setup(t, f)
	m.clipMode = clipOSC52
	step := pinClock(m)
	paste(m, "one\ntwo\nthree")
	rows := shown(t, m)
	n := m.height
	if got := strings.Join(rows[n-3:], "\n"); got != "[server] one\n         two\n         three " {
		t.Fatalf("a three-row input:\n%s", got)
	}

	// The link is clicked where it is drawn, and the light follows the
	// pointer there.
	x, y := find(m, spanURL, link)
	if y < 0 || !strings.HasPrefix(rows[y][x:], link) {
		t.Fatalf("the link's click target is not where it is drawn: %d,%d", x, y)
	}
	m.Update(tea.MouseMsg{X: x + 2, Y: y, Action: tea.MouseActionMotion, Button: tea.MouseButtonNone})
	if m.hover != hoverKey(span{kind: spanURL, target: link}) {
		t.Fatalf("hover: %q", m.hover)
	}
	if cmd := click(m, x+2, y); cmd == nil || m.status != "opening "+link+"..." {
		t.Fatalf("click on the link: %q", m.status)
	}
	m.View()

	// A drag over the message copies what it covers.
	rx, ry := rowOf(t, m, "read https")
	step(time.Second)
	drag(m, rx, ry, rx+3, ry)
	copied(t, m, "read")
	m.View()
	// A double click on a word of it.
	step(time.Second)
	multiClick(m, step, 2, rx+1, ry)
	copied(t, m, "read")
	// A drag over the input rows copies them as shown.
	step(time.Second)
	drag(m, 0, n-3, 20, n-1)
	copied(t, m, "[server] one\n         two\n         three")

	// The window bar sits above the input and its tags still focus.
	bx, by := find(m, spanWin, "2")
	if by != n-4 || !strings.Contains(rows[by], "[2:") {
		t.Fatalf("window bar row %d: %q", by, rows[by])
	}
	step(time.Second)
	click(m, bx+1, by)
	if m.cur != 2 {
		t.Fatalf("the tag focused window %d", m.cur)
	}

	// A chat name in the server window opens its chat.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'0'}, Alt: true})
	shown(t, m)
	cx, cy := find(m, spanChat, "c1")
	step(time.Second)
	click(m, cx, cy)
	if m.wins[m.cur].chatID != "c1" {
		t.Fatalf("the chat name focused %q", m.wins[m.cur].chatID)
	}
}
