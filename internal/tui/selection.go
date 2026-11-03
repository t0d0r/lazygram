package tui

import (
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"
)

// Double click, triple click and Shift-click (specs/ui.md, Selection and
// clipboard). Presses on the same cell, each within clickGap of the one
// before, make one multi-click: the first press is a click or the start
// of a drag as before, the second selects the word under the pointer,
// the third and any after it the screen row. A Shift-click extends the
// last selection from its anchor. Everything works on the plain rows of
// the last View, in screen cells.

// clickGap is the longest pause between two presses of a double or
// triple click.
const clickGap = 400 * time.Millisecond

// clickRun is the multi-click in progress: where and when its last press
// went down and how many presses it has had.
type clickRun struct {
	at    time.Time
	x, y  int
	n     int    // presses so far: 1 single, 2 double, 3 triple (it stays at 3); 0 none
	row   string // the plain screen row under the first press
	acted bool   // the first press was a click that ran a clickable element's action
}

// leftPress starts whatever a left-button press starts: a Shift-click
// extends the last selection; a press continuing a multi-click selects
// the word or the row; any other press is the start of a click or a
// drag. Any press drops the kept highlight of the last one. The release
// finishes it (Model.mouse).
func (m *Model) leftPress(e tea.MouseMsg) {
	m.dropKept()
	if e.Shift && m.lastSel != nil {
		s := m.lastSel.extendTo(e.X, e.Y)
		m.sel = &s
		m.multi = clickRun{} // a Shift-click is no part of a double click
		return
	}
	now := m.now()
	r := &m.multi
	if r.n > 0 && e.X == r.x && e.Y == r.y && now.Sub(r.at) <= clickGap {
		r.n = min(r.n+1, 3)
	} else {
		*r = clickRun{x: e.X, y: e.Y, n: 1, row: m.plainRow(e.Y)}
	}
	r.at = now
	switch {
	case r.n == 1:
		m.sel = &selection{x0: e.X, y0: e.Y, x1: e.X, y1: e.Y}
	case r.acted && m.plainRow(e.Y) != r.row:
		// The first click opened something that replaced the row under
		// the pointer: the rest of the gesture neither selects on the new
		// screen nor acts again.
		m.sel = nil
	case r.n == 2:
		m.sel = m.wordSel(e.X, e.Y)
	default:
		m.sel = m.rowSel(e.Y)
	}
}

// plainRow is screen row y of the last View without its styling; "" off
// the screen.
func (m *Model) plainRow(y int) string {
	if y < 0 || y >= len(m.screen) {
		return ""
	}
	return ansi.Strip(m.screen[y])
}

// wordSel selects the word under the cell x, y. On a blank cell it is
// that one cell, which copies as "nothing to copy".
func (m *Model) wordSel(x, y int) *selection {
	row := m.plainRow(y)
	from, to, ok := wordAt(row, x)
	if !ok {
		from, to = x, x
	}
	return &selection{x0: from, y0: y, x1: to, y1: y, unit: byWord, row: row}
}

// rowSel selects the whole screen row y.
func (m *Model) rowSel(y int) *selection {
	row := m.plainRow(y)
	end := max(m.width, ansi.StringWidth(row), 1) - 1
	return &selection{x0: 0, y0: y, x1: end, y1: y, unit: byRow, row: row}
}

// extendTo is s grown from its anchor to the cell x, y, as a Shift-click
// makes it. A click or a drag keeps its anchor, the cell where the button
// went down. A word or a row keeps all of itself: its anchor is its end
// farther from the new cell. Every Shift-click extends the same s (the
// last click, drag, word or row), so a second one moves the end again.
func (s selection) extendTo(x, y int) selection {
	ax, ay := s.x0, s.y0
	if s.unit == byWord || s.unit == byRow {
		// Cells in reading order; rows are far narrower than rowSpan.
		const rowSpan = 1 << 20
		pos := func(x, y int) int { return y*rowSpan + x }
		sx, sy, ex, ey := s.ordered()
		ax, ay = sx, sy
		if pos(x, y)-pos(sx, sy) < pos(ex, ey)-pos(x, y) { // the end is farther
			ax, ay = ex, ey
		}
	}
	return selection{x0: ax, y0: ay, x1: x, y1: y, unit: byShift}
}

// glyph is one character of a plain screen row as the terminal draws it:
// a grapheme cluster (a base with its combining marks, a whole emoji
// sequence) and the cells it covers.
type glyph struct {
	text string
	col  int // first column
	w    int // width in cells: 1, 2 for wide characters and emoji, 0 for a stray mark
}

// glyphs splits a plain row into its characters. The widths are the ones
// lipgloss lays the screen out with (grapheme clusters, as ansi.StringWidth).
func glyphs(s string) []glyph {
	var out []glyph
	col, state := 0, -1
	for s != "" {
		var c string
		var w int
		c, s, w, state = uniseg.FirstGraphemeClusterInString(s, state)
		out = append(out, glyph{text: c, col: col, w: w})
		col += w
	}
	return out
}

// Characters trimmed from the ends of a word (specs/ui.md).
const (
	wordOpen  = "([{<"   // trimmed at the start when unmatched or paired with the end
	wordClose = ")]}>"   // trimmed at the end when unmatched
	wordQuote = "\"'`"   // trimmed at either end
	wordStop  = ".,;:!?" // trailing sentence punctuation, trimmed at the end
)

// wordAt returns the columns, both ends inclusive, of the word under
// column x of the plain row s, and false when x is on a blank or past the
// text. The word is the run of non-blank characters around x with
// brackets, quotes and trailing sentence punctuation trimmed (trimWord),
// so a link, an @name or a #chan is one word; when x is on a trimmed
// character the word is the whole run.
func wordAt(s string, x int) (from, to int, ok bool) {
	gs := glyphs(s)
	at := -1
	for i, g := range gs {
		if g.w > 0 && x >= g.col && x < g.col+g.w {
			at = i
			break
		}
	}
	if at < 0 || blank(gs[at].text) {
		return 0, 0, false
	}
	i, j := at, at
	for i > 0 && !blank(gs[i-1].text) {
		i--
	}
	for j < len(gs)-1 && !blank(gs[j+1].text) {
		j++
	}
	a, b := trimWord(gs, i, j)
	if at < a || at > b {
		a, b = i, j
	}
	return gs[a].col, gs[b].col + gs[b].w - 1, true
}

// blank reports whether a character is white space (a space, a tab, a
// no-break space, any Unicode space).
func blank(c string) bool { return strings.TrimFunc(c, unicode.IsSpace) == "" }

// trimWord trims the run gs[i..j] down to its word, repeating until
// nothing changes: trailing sentence punctuation goes; a quote at either
// end goes; an opening bracket at the start goes with the bracket that
// closes it at the end, or alone when the run opens it more often than it
// closes it; a closing bracket at the end goes when the run closes it
// more often than it opens it. A bracket the word needs, like the one in
// "Go_(game)", stays. The result is empty (i > j) when every character
// was trimmed.
func trimWord(gs []glyph, i, j int) (int, int) {
	count := func(c string) (n int) {
		for k := i; k <= j; k++ {
			if gs[k].text == c {
				n++
			}
		}
		return n
	}
	for i <= j {
		first, last := gs[i].text, gs[j].text
		switch {
		case oneOf(wordStop, last), oneOf(wordQuote, last):
			j--
		case oneOf(wordQuote, first):
			i++
		case oneOf(wordOpen, first) && i < j && closes(gs, i, j):
			i, j = i+1, j-1
		case oneOf(wordOpen, first) && count(first) > count(partner(first)):
			i++
		case oneOf(wordClose, last) && count(last) > count(partner(last)):
			j--
		default:
			return i, j
		}
	}
	return i, j
}

// oneOf reports whether c is a single character of set.
func oneOf(set, c string) bool { return len(c) == 1 && strings.Contains(set, c) }

// partner is the other bracket of a pair: ")" for "(" and "(" for ")".
func partner(b string) string {
	if i := strings.Index(wordOpen, b); i >= 0 {
		return wordClose[i : i+1]
	}
	i := strings.Index(wordClose, b)
	return wordOpen[i : i+1]
}

// closes reports whether the opening bracket gs[i] is closed by gs[j],
// counting the brackets of its kind in between.
func closes(gs []glyph, i, j int) bool {
	open, shut := gs[i].text, partner(gs[i].text)
	depth := 0
	for k := i; k <= j; k++ {
		switch gs[k].text {
		case open:
			depth++
		case shut:
			depth--
			if depth == 0 {
				return k == j
			}
		}
	}
	return false
}
