package tui

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Closing windows (specs/ui.md, Windows): /close closes the current
// window, /close N window N, and /close 2,3-5 the windows a list of
// numbers and ranges names. Every number is the window's number before
// the command, as the window bar shows it, and the whole list is checked
// before anything closes, so one bad item closes nothing. /close-others
// closes every chat window but the current one, the same way.

// The errors of /close. The server window's is also the /win list's
// [close] button's on window 0.
const (
	closeFormErr   = "/close takes window numbers: N, a list or ranges (as 2,3-5)"
	closeServerErr = "cannot close that window (0 is the server window)"
)

// What /close-others says instead of closing. It takes no argument, and
// names /close for closing given windows. Window 0 is no chat window to
// keep, so it gets the error of every command that needs a chat window.
// With the current window the only chat window what it asks for already
// holds: information, not an error.
const (
	closeOthersArgErr    = "/close-others takes no argument; /close 2,3-5 closes the windows given"
	closeOthersServerErr = "the server window has no chat; /win to a chat first"
	closeOthersNone      = "no other chat window to close"
)

// closeCommand runs /close with its argument; none closes the current
// window.
func (m *Model) closeCommand(arg string) {
	if arg == "" {
		m.closeWin(m.cur)
		return
	}
	idx, err := closeSpec(arg, len(m.wins)-1)
	if err != nil {
		m.setErr(err.Error())
		return
	}
	m.closeWins(idx)
}

// closeOthers runs /close-others: every chat window but the current one
// closes at once, through closeWins, so it is /close with their numbers.
// The checks go in order, the argument first, as /close checks the form
// first: an argument, then window 0, are errors; then none to close is
// information. On each of them nothing changes and the layout is not
// written.
func (m *Model) closeOthers(arg string) {
	if arg != "" {
		m.setErr(closeOthersArgErr)
		return
	}
	if m.cur == 0 {
		m.setErr(closeOthersServerErr)
		return
	}
	var idx []int
	for i := 1; i < len(m.wins); i++ {
		if i != m.cur {
			idx = append(idx, i)
		}
	}
	if len(idx) == 0 {
		m.setInfo(closeOthersNone)
		return
	}
	m.closeWins(idx)
}

// closeWin closes window i. Window 0, or a number that is no window, is
// refused with the server window's error and nothing changes.
func (m *Model) closeWin(i int) {
	if i <= 0 || i >= len(m.wins) {
		m.setErr(closeServerErr)
		return
	}
	m.closeWins([]int{i})
}

// windowNumber is one number of a /close argument: ASCII digits only, no
// sign. Its text, for an error, is the number without leading zeros; one
// too large for an int is past any window and reads as the largest int.
type windowNumber struct {
	n    int
	text string
}

// atMost tells whether x <= y as whole numbers of any size, so a range
// whose ends are both too large for an int is still ordered as written:
// with no leading zeros, the shorter text is the smaller number, and texts
// of one length compare digit by digit.
func (x windowNumber) atMost(y windowNumber) bool {
	if len(x.text) != len(y.text) {
		return len(x.text) < len(y.text)
	}
	return x.text <= y.text
}

func readWindowNumber(s string) (windowNumber, bool) {
	if s == "" || strings.Trim(s, "0123456789") != "" {
		return windowNumber{}, false
	}
	text := strings.TrimLeft(s, "0")
	if text == "" {
		text = "0"
	}
	n, err := strconv.Atoi(text)
	if err != nil { // digits only, so it overflowed
		n = math.MaxInt
	}
	return windowNumber{n: n, text: text}, true
}

// closeSpec reads a /close argument against windows 0..last. Items are
// separated by a comma, spaces, or a comma with spaces around it; an empty
// item (2,,3, a comma at either end) is malformed. An item is N or A-B
// with A <= B. It returns the windows named, ascending and each once, or
// the error of the first failing check: a malformed item or a reversed
// range anywhere, then window 0 anywhere, then the first number past last
// as written.
func closeSpec(spec string, last int) ([]int, error) {
	var spans [][2]windowNumber
	for _, part := range strings.Split(spec, ",") {
		items := strings.Fields(part)
		if len(items) == 0 {
			return nil, errors.New(closeFormErr)
		}
		for _, item := range items {
			a, b, isRange := strings.Cut(item, "-")
			lo, ok := readWindowNumber(a)
			hi := lo
			if isRange {
				var okB bool
				hi, okB = readWindowNumber(b)
				ok = ok && okB && lo.atMost(hi)
			}
			if !ok {
				return nil, errors.New(closeFormErr)
			}
			spans = append(spans, [2]windowNumber{lo, hi})
		}
	}
	for _, s := range spans {
		if s[0].n == 0 { // lo <= hi: a 0 anywhere is the low end
			return nil, errors.New(closeServerErr)
		}
	}
	for _, s := range spans {
		for _, x := range s {
			if x.n > last {
				return nil, fmt.Errorf("no window %s", x.text)
			}
		}
	}
	named := make([]bool, last+1)
	for _, s := range spans {
		for i := s[0].n; i <= s[1].n; i++ {
			named[i] = true
		}
	}
	var idx []int
	for i, ok := range named {
		if ok {
			idx = append(idx, i)
		}
	}
	return idx, nil
}

// closeWins closes the windows idx, numbered as they are now: ascending,
// each once, all open and none of them 0 (closeSpec, closeWin). The
// current window stays current when it stays open, else the nearest open
// window before it becomes current (window 0 at worst). Nothing is
// focused, so no read receipt goes and no unread count changes; the last
// window, kept by identity, is gone when it closed. Each closed window is
// noted in window 0 in window order, the status names what closed, and
// the layout is saved once.
func (m *Model) closeWins(idx []int) {
	gone := make([]bool, len(m.wins))
	for _, i := range idx {
		gone[i] = true
	}
	stay := m.cur
	for gone[stay] { // window 0 never closes
		stay--
	}
	names := make([]string, len(idx))
	for k, i := range idx {
		names[k] = m.winName(i)
	}
	status := "closed " + names[0]
	if len(idx) > 1 { // "closed 3 windows: 2:#dev, 3:@alice, 5:#ops", cut after four
		status = fmt.Sprintf("closed %d windows: %s", len(idx), strings.Join(m.whereList(idx, nil), ", "))
	}
	kept := make([]*win, 0, len(m.wins)-len(idx))
	for i, w := range m.wins {
		if i == stay {
			m.cur = len(kept)
		}
		if !gone[i] {
			kept = append(kept, w)
		}
	}
	m.wins = kept
	for _, name := range names {
		m.note(m.wins[0], "window closed: "+name)
	}
	m.setInfo(status)
	m.saveLayout()
}
