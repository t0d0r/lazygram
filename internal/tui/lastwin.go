package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// The last focused window (specs/ui.md, Windows): the window that was
// current before the current one, Alt-Tab style. Every focus change sets
// it (focus), by window identity, so /move and closing other windows do
// not change which window it is; closing it leaves none. /win last goes
// there, and so does Tab twice fast on an empty prompt: the first Tab
// switches to the next window at once, as a single Tab always did, and a
// second one within tabGap takes the gesture as a whole to the last
// window before the first. The window the first Tab passed through is left
// as it was: its unread count comes back and its read receipt, held back
// for tabGap after every empty-prompt Tab, is never sent. Session only.

// tabGap is the longest pause between the two presses of a double Tab, as
// clickGap is for a double click.
const tabGap = 400 * time.Millisecond

// tabHop is what the first Tab of a possible double Tab left behind: the
// window it left, the last focused window before it (nil: none), and the
// window it showed with that window's unread state, which the second Tab
// puts back.
type tabHop struct {
	from, last, hop *win
	unread          int
	counted         map[string]bool
}

// tabReadMsg comes tabGap after an empty-prompt Tab focused w: its read
// receipt goes only if w is still current and no focus change came since.
type tabReadMsg struct {
	w       *win
	seq     int // focusN when the Tab focused w
	receipt tea.Cmd
}

// tickAfter delivers msg after d (Model.after).
func tickAfter(d time.Duration, msg tea.Msg) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return msg })
}

// indexOf is the position of window w, or -1 when it is not open.
func (m *Model) indexOf(w *win) int {
	for i, x := range m.wins {
		if x == w {
			return i
		}
	}
	return -1
}

// lastWindow is the index of the last focused window, or -1 when there is
// none: no window was left yet, it was closed, or it is current again
// (closing the current window can fall back on it).
func (m *Model) lastWindow() int {
	if i := m.indexOf(m.lastWin); i != m.cur {
		return i
	}
	return -1
}

// tab is Tab on an empty prompt: the next window, at once. A second Tab
// within tabGap, with no other key or focus change between, goes instead
// to the last focused window before the first, putting back the unread
// state of the window the first one showed (unless that is where it goes);
// the last window is then the one the gesture began in, so another double
// Tab goes back. With no last window the second Tab is a plain one. The
// second Tab never arms a third: that one is a first Tab again.
func (m *Model) tab() tea.Cmd {
	now := m.now()
	second := !m.tabAt.IsZero() && now.Sub(m.tabAt) <= tabGap
	g := m.tabGo
	m.tabAt, m.tabGo = time.Time{}, tabHop{}
	if second {
		if to := m.indexOf(g.last); to >= 0 {
			if g.hop != g.last {
				g.hop.unread, g.hop.counted = g.unread, g.counted
			}
			cmd := m.focus(to, true)
			m.lastWin = g.from
			return cmd
		}
	}
	next := (m.cur + 1) % len(m.wins)
	hop := m.wins[next]
	first := tabHop{from: m.wins[m.cur], hop: hop, unread: hop.unread, counted: hop.counted}
	if i := m.lastWindow(); i >= 0 {
		first.last = m.wins[i]
	}
	cmd := m.focus(next, true)
	if !second {
		m.tabAt, m.tabGo = now, first
	}
	return cmd
}

// heldRead sends the receipt an empty-prompt Tab held back, when its
// window is still current and was not left in between: a window a double
// Tab only passed through, or one left within tabGap, was not read.
func (m *Model) heldRead(msg tabReadMsg) tea.Cmd {
	if msg.seq != m.focusN || m.wins[m.cur] != msg.w {
		return nil
	}
	return msg.receipt
}
