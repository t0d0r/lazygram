package tui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

// The double Tab's setting (specs/ui.md, Windows): /doubletab says what a
// second empty-prompt Tab within the double Tab's time of the first does,
// and how long that time is. off, the default, makes it nothing special:
// every Tab goes to the next window, so fast Tabs walk the windows. last
// takes the gesture as a whole where /win last goes from the window the
// first Tab left, unread where /unread goes from there (lastwin.go, tab).
// The time is also how long a window an empty-prompt Tab showed must stay
// current to count as read, in every mode (hold, leaveHeld). Both are kept
// in the state store; the settings menu's doubletab and doubletab-time
// rows run setDoubleTab, so they keep and say the same.

const (
	doubleTabKey     = "doubletab"      // store state key for what the double Tab does, as its word; absent, unreadable or any other value means off
	doubleTabTimeKey = "doubletab-time" // store state key for its time in whole ms; absent, unreadable or out of range means defaultTabGap
)

// doubleTab is what the second of two fast empty-prompt Tabs does.
type doubleTab int

const (
	tabOff    doubleTab = iota // nothing of its own: a plain Tab, the next window
	tabLast                    // the last window before the first Tab, as /win last
	tabUnread                  // the next window with unread messages after that one, as /unread
)

// doubleTabWords name the settings, in the order the settings menu steps
// through them: the index is the doubleTab.
var doubleTabWords = []string{"off", "last", "unread"}

func (d doubleTab) String() string { return doubleTabWords[d] }

// jump is what Tab twice does under d, for the status messages; "" for
// off.
func (d doubleTab) jump() string {
	switch d {
	case tabLast:
		return "jumps back to the last window"
	case tabUnread:
		return "jumps to the next unread window"
	}
	return ""
}

// The double Tab's time (Model.tabGap): the longest pause between the two
// presses of a double Tab, as clickGap is for a double click, and how long
// a window an empty-prompt Tab showed must stay current to count as read.
// /doubletab <ms> sets it within minTabGap..maxTabGap, both included.
const (
	defaultTabGap = 400 * time.Millisecond
	minTabGap     = 150 * time.Millisecond
	maxTabGap     = 1000 * time.Millisecond
)

// tabGapValues are the times the settings menu's doubletab-time row steps
// through, shortest first.
var tabGapValues = []string{"200ms", "300ms", "400ms", "500ms", "600ms", "800ms"}

// msOf is a time as the settings menu shows it: whole milliseconds and
// "ms", as /doubletab takes it.
func msOf(d time.Duration) string { return strconv.FormatInt(d.Milliseconds(), 10) + "ms" }

// parseTabGap reads the time /doubletab takes: whole milliseconds, "ms"
// after them or not (lower case: the caller folds it), from minTabGap to
// maxTabGap.
func parseTabGap(s string) (time.Duration, bool) {
	s = strings.TrimSpace(strings.TrimSuffix(s, "ms"))
	if s == "" || strings.Trim(s, "0123456789") != "" {
		return 0, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return tabGapOf(n)
}

// tabGapOf is n milliseconds as the double Tab's time, when n is within
// minTabGap..maxTabGap. The range is checked on n itself, before it
// becomes a Duration: n * time.Millisecond can wrap int64 for a large n
// (2^58 + 300 wraps to exactly 300 ms), which would let a number far out
// of range through as one within it.
func tabGapOf(n int64) (time.Duration, bool) {
	if n < minTabGap.Milliseconds() || n > maxTabGap.Milliseconds() {
		return 0, false
	}
	return time.Duration(n) * time.Millisecond, true
}

// nearTabGap is the listed time the doubletab-time row steps to from a
// time it does not list (one /doubletab <ms> set): the next longer one
// for Right and Enter (d = 1), the next shorter one for Left (d = -1),
// wrapping round past the ends as the other rows wrap.
func nearTabGap(cur time.Duration, d int) string {
	n := len(tabGapValues)
	for k := range n {
		i := k
		if d < 0 {
			i = n - 1 - k
		}
		v, _ := parseTabGap(tabGapValues[i])
		if (d > 0 && v > cur) || (d < 0 && v < cur) {
			return tabGapValues[i]
		}
	}
	if d > 0 {
		return tabGapValues[0]
	}
	return tabGapValues[n-1]
}

// loadDoubleTab reads the kept setting and its time into the model.
func (m *Model) loadDoubleTab() {
	m.dblTab = tabOff
	var word string
	if found, err := m.opt.Store.State(doubleTabKey, &word); found && err == nil {
		m.dblTab = doubleTab(max(slices.Index(doubleTabWords, word), 0)) // another word: off
	}
	m.tabGap = defaultTabGap
	var ms int64
	if found, err := m.opt.Store.State(doubleTabTimeKey, &ms); found && err == nil {
		if d, ok := tabGapOf(ms); ok {
			m.tabGap = d
		}
	}
}

// setDoubleTab implements /doubletab: no argument shows the setting and its
// time; off, last or unread sets what Tab twice does, and a time in
// milliseconds how fast the two Tabs must be; either takes effect at once
// and is kept. Anything else is an error that changes nothing. The
// settings menu's two rows run it too.
func (m *Model) setDoubleTab(arg string) {
	a := strings.ToLower(strings.TrimSpace(arg))
	ms := m.tabGap.Milliseconds()
	if a == "" {
		if m.dblTab == tabOff {
			m.setInfo(fmt.Sprintf("double tab off (%d ms): Tab walks the windows; /doubletab last or unread makes Tab twice fast jump", ms))
		} else {
			m.setInfo(fmt.Sprintf("double tab %s (%d ms): Tab twice fast %s; /doubletab off makes Tab walk", m.dblTab, ms, m.dblTab.jump()))
		}
		return
	}
	if i := slices.Index(doubleTabWords, a); i >= 0 {
		m.dblTab = doubleTab(i)
		_ = m.opt.Store.SetState(doubleTabKey, m.dblTab.String())
		if m.dblTab == tabOff {
			m.setInfo("double tab off: Tab walks the windows, however fast")
		} else {
			m.setInfo(fmt.Sprintf("double tab %s: Tab twice within %d ms %s", m.dblTab, ms, m.dblTab.jump()))
		}
		return
	}
	d, ok := parseTabGap(a)
	if !ok {
		m.setErr("/doubletab takes off, last, unread or a time from 150 to 1000 ms")
		return
	}
	m.tabGap = d
	_ = m.opt.Store.SetState(doubleTabTimeKey, d.Milliseconds())
	if m.dblTab == tabOff {
		m.setInfo(fmt.Sprintf("double tab time %d ms: Tab twice is off; a window a Tab shows for less stays unread", d.Milliseconds()))
	} else {
		m.setInfo(fmt.Sprintf("double tab time %d ms: Tab twice within it %s", d.Milliseconds(), m.dblTab.jump()))
	}
}
