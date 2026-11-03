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
// window before the first. A window an empty-prompt Tab showed and that
// is left again within tabGap, by the second Tab of a double Tab or by any
// other focus, is left as it was: its unread count comes back and its
// read receipt, held back for tabGap, is never sent. Session only.

// tabGap is the longest pause between the two presses of a double Tab, as
// clickGap is for a double click.
const tabGap = 400 * time.Millisecond

// tabHop is what the first Tab of a possible double Tab left behind: the
// window it left and the last focused window before it (nil: none).
type tabHop struct{ from, last *win }

// tabHeld is a window an empty-prompt Tab showed with unread messages,
// until it is left or its receipt goes: what the Tab cleared, which comes
// back when it is left within tabGap, and the receipt held back for it.
type tabHeld struct {
	w       *win
	at      time.Time // when the Tab showed it
	seq     int       // its tabReadMsg's
	unread  int
	counted map[string]bool
	receipt tea.Cmd // nil: nothing to report (receipts off, no message known)
}

// tabReadMsg comes tabGap after an empty-prompt Tab held a receipt back.
type tabReadMsg struct{ seq int }

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
// to the last focused window before the first; leaving the window the
// first one showed puts its unread state back (leaveHeld). The last window
// is then the one the gesture began in, so another double Tab goes back.
// With no last window the second Tab is a plain one. The second Tab never
// arms a third: that one is a first Tab again.
func (m *Model) tab() tea.Cmd {
	now := m.now()
	second := !m.tabAt.IsZero() && now.Sub(m.tabAt) <= tabGap
	g := m.tabGo
	m.tabAt, m.tabGo = time.Time{}, tabHop{}
	if second {
		if to := m.indexOf(g.last); to >= 0 {
			cmd := m.focus(to, true)
			m.lastWin = g.from
			return cmd
		}
	}
	first := tabHop{from: m.wins[m.cur]}
	if i := m.lastWindow(); i >= 0 {
		first.last = m.wins[i]
	}
	cmd := m.focus((m.cur+1)%len(m.wins), true)
	if !second {
		m.tabAt, m.tabGo = now, first
	}
	return cmd
}

// hold keeps what an empty-prompt Tab is about to clear of w, in case w is
// only passed through, and holds its receipt back for tabGap (heldRead).
func (m *Model) hold(w *win, receipt tea.Cmd) tea.Cmd {
	m.heldSeq++
	m.held = &tabHeld{w: w, at: m.now(), seq: m.heldSeq, unread: w.unread, counted: w.counted, receipt: receipt}
	if receipt == nil {
		return nil
	}
	return m.after(tabGap, tabReadMsg{seq: m.heldSeq})
}

// leaveHeld is called as the current window is left. When a Tab showed it
// no more than tabGap ago it was only passed through: it gets back what
// the Tab cleared, and its receipt is dropped, so focusing it again
// reports it as any focus does. Later it was read, and a receipt whose
// message has not come yet goes now.
func (m *Model) leaveHeld() tea.Cmd {
	h := m.held
	if h == nil {
		return nil
	}
	m.held = nil
	if h.w != m.wins[m.cur] { // closed meanwhile
		return nil
	}
	if m.now().Sub(h.at) > tabGap {
		return h.receipt
	}
	h.w.unread = max(h.w.unread, h.unread) // an [unread] meanwhile is the same mark
	h.w.counted = h.counted                // nothing is counted in the window shown
	return nil
}

// heldRead sends the receipt an empty-prompt Tab held back, when its
// window is still current: not left since (leaveHeld settled it then) and
// not closed.
func (m *Model) heldRead(msg tabReadMsg) tea.Cmd {
	h := m.held
	if h == nil || h.seq != msg.seq {
		return nil
	}
	m.held = nil
	if m.wins[m.cur] != h.w {
		return nil
	}
	return h.receipt
}
