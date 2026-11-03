package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// The last focused window (specs/ui.md, Windows): the window that was
// current before the current one, Alt-Tab style. Every focus change sets
// it (focus), by window identity, so /move and closing other windows do
// not change which window it is; closing it leaves none. /win last goes
// there; /unread goes to the next window with unread messages after the
// current one (nextUnread). Session only.
//
// Tab on an empty prompt switches to the next window at once, as a single
// Tab always did. What a second one within the double Tab's time (tabGap)
// does is the /doubletab setting (doubletab.go): off, the default, nothing
// of its own, so fast Tabs walk the windows; last takes the gesture as a
// whole to the last window before the first Tab, unread to the next window
// with unread messages after that one. In every mode a window an
// empty-prompt Tab showed and that is left again within tabGap, by another
// Tab or by any other focus, is left as it was: its unread count comes
// back and its read receipt, held back for tabGap, is never sent.

// tabHop is what the first Tab of a possible double Tab left behind: the
// window it left and the last focused window before it (nil: none).
type tabHop struct{ from, last *win }

// tabHeld is a window an empty-prompt Tab showed with unread messages,
// until it is left or its receipt goes: what the Tab cleared, which comes
// back when it is left within gap, and the receipt held back for it.
type tabHeld struct {
	w       *win
	at      time.Time     // when the Tab showed it
	gap     time.Duration // the double Tab's time then: the receipt's timer runs this long
	seq     int           // its tabReadMsg's
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

// nextUnread is the window /unread goes to from window from: the first
// chat window with unread messages after it, in window order and wrapping
// round; window 0 has no unread count and is never one, and from itself
// comes last (it is current and so read, unless marked [unread] from
// /win). -1: no window has unread messages.
func (m *Model) nextUnread(from int) int {
	n := len(m.wins)
	for k := 1; k <= n; k++ {
		if i := (from + k) % n; i != 0 && m.wins[i].unread > 0 {
			return i
		}
	}
	return -1
}

// unreadWindow implements /unread: the next window with unread messages,
// focused as /win N focuses (its read receipt, the last window, a
// window a Tab showed left as it was); with none, an error and nothing
// changes.
func (m *Model) unreadWindow() tea.Cmd {
	i := m.nextUnread(m.cur)
	if i < 0 {
		m.setErr("no unread window")
		return nil
	}
	return m.switchTo(i)
}

// tab is Tab on an empty prompt: the next window, at once. With
// /doubletab last or unread, a second Tab within tabGap, with no other key
// or focus change between, goes instead where /win last or /unread goes
// from the window the first one left. For unread the window the first Tab
// showed gets its unread state back first (leaveHeld), since it was only
// passed through, so it is the target when it has unread messages; for
// last leaving it puts its state back, as leaving it any other way within
// tabGap does. The last window is then the one the gesture began in, so
// another double Tab of last goes back. With no target the second Tab is
// a plain one. The second Tab never arms a third: that one is a first Tab
// again. With /doubletab off every Tab is a first Tab.
func (m *Model) tab() tea.Cmd {
	now := m.now()
	second := m.dblTab != tabOff && !m.tabAt.IsZero() && now.Sub(m.tabAt) <= m.tabGap
	g := m.tabGo
	m.tabAt, m.tabGo = time.Time{}, tabHop{}
	var left tea.Cmd
	if second {
		to := -1
		switch m.dblTab {
		case tabLast:
			to = m.indexOf(g.last)
		case tabUnread:
			left = m.leaveHeld()
			if from := m.indexOf(g.from); from >= 0 {
				to = m.nextUnread(from)
			}
		}
		if to >= 0 {
			cmd := m.focus(to, true)
			m.lastWin = g.from
			return tea.Batch(left, cmd)
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
	return tea.Batch(left, cmd)
}

// hold keeps what an empty-prompt Tab is about to clear of w, in case w is
// only passed through, and holds its receipt back for tabGap (heldRead).
func (m *Model) hold(w *win, receipt tea.Cmd) tea.Cmd {
	m.heldSeq++
	m.held = &tabHeld{w: w, at: m.now(), gap: m.tabGap, seq: m.heldSeq, unread: w.unread, counted: w.counted, receipt: receipt}
	if receipt == nil {
		return nil
	}
	return m.after(m.tabGap, tabReadMsg{seq: m.heldSeq})
}

// leaveHeld is called as the current window is left. When a Tab showed it
// no more than the gap held with it ago it was only passed through: it
// gets back what the Tab cleared, and its receipt is dropped, so focusing
// it again reports it as any focus does. Later it was read, and a receipt
// whose message has not come yet goes now.
func (m *Model) leaveHeld() tea.Cmd {
	h := m.held
	if h == nil {
		return nil
	}
	m.held = nil
	if h.w != m.wins[m.cur] { // closed meanwhile
		return nil
	}
	if m.now().Sub(h.at) > h.gap {
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
