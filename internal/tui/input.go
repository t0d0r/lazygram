package tui

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
)

// The input area (specs/ui.md, Input area): every line of the input is a
// row of its own, a line wider than the area wraps onto further rows, and
// the area grows upward to inputCap rows, scrolling past that so the
// cursor's row stays in view. The content area above gives up the rows.

// maxInputRows is the most rows the input area takes; a third of the
// screen caps it further.
const maxInputRows = 8

// inGlyph is one character of the input as the screen lays it out: a
// grapheme cluster, the runes [at, at+n) of the input, w cells wide.
type inGlyph struct {
	text  string
	at, n int
	w     int
}

// inRow is one screen row of the input: the runes [start, end) and their
// characters. last marks the last row of a line, where the cursor may
// stand past the text (on the line break, or at the end of the input).
type inRow struct {
	start, end int
	last       bool
	gs         []inGlyph
}

// inputRows lays the input out in rows of at most room cells. A line
// break starts a row; a line wider than room wraps between characters,
// so a wide character or an emoji is never split (one wider than room
// still gets a row to itself).
func inputRows(in []rune, room int) []inRow {
	var rows []inRow
	start := 0
	for {
		end := start
		for end < len(in) && in[end] != '\n' {
			end++
		}
		r, col, at := inRow{start: start}, 0, start
		for _, g := range glyphs(string(in[start:end])) {
			n := utf8.RuneCountInString(g.text)
			if col > 0 && col+g.w > room {
				r.end = at
				rows = append(rows, r)
				r, col = inRow{start: at}, 0
			}
			r.gs = append(r.gs, inGlyph{text: g.text, at: at, n: n, w: g.w})
			col += g.w
			at += n
		}
		r.end, r.last = end, true
		rows = append(rows, r)
		if end == len(in) {
			return rows
		}
		start = end + 1
	}
}

// cursorRow is the row the cursor at rune p is shown on: the one holding
// the rune under it, or the line's last row when it stands past the text.
func cursorRow(rows []inRow, p int) int {
	for i, r := range rows {
		if p >= r.start && (p < r.end || p == r.end && r.last) {
			return i
		}
	}
	return len(rows) - 1
}

// col is the cell column of rune p within the row; a cursor inside a
// character (between the runes of an emoji sequence) is on its first cell.
func (r inRow) col(p int) int {
	c := 0
	for _, g := range r.gs {
		if g.at+g.n > p {
			break
		}
		c += g.w
	}
	return c
}

// under is the index of the character the cursor at rune p is drawn on, a
// whole character even when p is inside it; len(r.gs) is the cell past
// the text.
func (r inRow) under(p int) int {
	for k, g := range r.gs {
		if p < g.at+g.n {
			return k
		}
	}
	return len(r.gs)
}

// at is the rune index of the cursor put at column col of the row: the
// last character start at or left of col, or past the text when the row
// is shorter. A wrapped row ends before its last character, since the
// place after it is the start of the next row.
func (r inRow) at(col int) int {
	p, c := r.start, 0
	for i, g := range r.gs {
		if c+g.w > col || (!r.last && i == len(r.gs)-1) {
			return g.at
		}
		c += g.w
		p = g.at + g.n
	}
	return p
}

// prompt is what the input area starts with in the current mode.
func (m *Model) prompt() string {
	switch m.mode {
	case modeList:
		if m.listSrc == listWindows {
			return "windows > "
		}
		return "list > "
	case modePager:
		return "search > "
	}
	if m.wins[m.cur].enterSends {
		return "[" + m.winName(m.cur) + "]> "
	}
	return "[" + m.winName(m.cur) + "] "
}

// inputCap is how many rows the input area may take: at most
// maxInputRows and at most a third of the screen, never less than one.
func (m *Model) inputCap() int { return max(min(maxInputRows, m.height/3), 1) }

// inputLayout is the input laid out under prompt: every row, the cursor's
// row, and the rows shown, [top, top+n). The first row shown is the one
// shown last time unless the cursor left the area, which then moves just
// enough to hold the cursor's row again.
func (m *Model) inputLayout(prompt string) (rows []inRow, cur, top, n int) {
	rows = inputRows(m.input, max(m.width-lipgloss.Width(prompt)-1, 1)) // a cell for the cursor after the text
	cur = cursorRow(rows, m.pos())
	n = min(len(rows), m.inputCap())
	top = min(max(m.inTop, cur-n+1), cur)
	top = min(max(top, 0), len(rows)-n)
	return rows, cur, top, n
}

// inputHeight is how many rows the input area takes now.
func (m *Model) inputHeight() int {
	_, _, _, n := m.inputLayout(m.prompt())
	return n
}

// inputArea renders the input area under prompt, top to bottom: the
// input's first row after the prompt, every other row indented to the
// prompt's width so the text lines up, and the cursor as a reverse-video
// cell on its row (on the character under it, or past the line's text).
// Every row is on the status background across the whole width, like the
// status line and the window bar above it: the prompt in the dim colour,
// the text in the status text colour, the cursor in the two reversed.
func (m *Model) inputArea(prompt string) []string {
	rows, cur, top, n := m.inputLayout(prompt)
	m.inTop = top
	p := m.pos()
	indent := strings.Repeat(" ", lipgloss.Width(prompt))
	cursor := infoStyle.Reverse(true)
	out := make([]string, 0, n)
	for i := top; i < top+n; i++ {
		var b strings.Builder
		if i == 0 {
			b.WriteString(dimStyle.Background(statusBG()).Render(prompt))
		} else {
			b.WriteString(infoStyle.Render(indent))
		}
		r := rows[i]
		under := -1
		if i == cur {
			under = r.under(p)
		}
		var text strings.Builder // runs of plain text share one style
		flush := func() {
			if text.Len() > 0 {
				b.WriteString(infoStyle.Render(text.String()))
				text.Reset()
			}
		}
		for k, g := range r.gs {
			if k == under {
				flush()
				b.WriteString(cursor.Render(g.text))
			} else {
				text.WriteString(g.text)
			}
		}
		flush()
		if under == len(r.gs) {
			b.WriteString(cursor.Render(" "))
		}
		line := b.String()
		if m.width > 0 { // cut to the screen (a no-op unless the prompt or one character is wider) and padded
			line = barRow(line, m.width)
		}
		out = append(out, line)
	}
	return out
}

// inputStep moves the cursor to the input row above (d -1) or below
// (d 1), keeping its column as far as that row allows; the column is
// kept across rows too short for it while the steps go on. It reports
// false on the first or last row, where Up and Down walk the history.
func (m *Model) inputStep(d int) bool {
	rows, cur, _, _ := m.inputLayout(m.prompt())
	next := cur + d
	if next < 0 || next >= len(rows) {
		m.goalOn = false
		return false
	}
	if !m.goalOn {
		m.goal, m.goalOn = rows[cur].col(m.pos()), true
	}
	m.curBack = len(m.input) - rows[next].at(m.goal)
	return true
}

// lineStart and lineEnd are the rune indexes where the input line holding
// rune p starts and ends (its line break, or the end of the input).
func (m *Model) lineStart(p int) int {
	for p > 0 && m.input[p-1] != '\n' {
		p--
	}
	return p
}

func (m *Model) lineEnd(p int) int {
	for p < len(m.input) && m.input[p] != '\n' {
		p++
	}
	return p
}
