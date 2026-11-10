package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lazygram/internal/domain"
)

// /x is /mark-as-read, then /close, for the current chat window
// (specs/ui.md, Read receipts): one receipt for the chat on request,
// whatever /read-receipts says, and when the service answered without an
// error the window closes, as /close closes it. The report goes first and
// its answer decides: a failed receipt or unread mark is said as
// /mark-as-read says it, with "window kept", and nothing closes, so a
// closed window always means the report went through.
//
// The answer comes later, and by then the window may not be what /x was
// typed in. So the request remembers the window itself and the messages
// held, and the answer closes only that window, only when it is still
// read and holds nothing newer, and only while the request stands.

// What /x says instead of reporting. It takes no argument: it is for the
// current window only. Window 0 gets the error of every command that
// needs a chat window.
const (
	readCloseArgErr    = "/x takes no argument; it reports this chat window read and closes it"
	readCloseServerErr = "the server window has no chat; /win to a chat first"
)

// readCloseReq is one /x not yet answered: the window it was typed in, by
// identity, so a window opened for the chat later is another one; the ids
// of the chat's messages held then and the time of the newest, to tell
// one that arrived since. Model.readClosing holds the request that stands
// for a chat: a later /x replaces it, an [unread] removes it (report),
// and the report that answers it carries it, so an answer finds only its
// own request.
type readCloseReq struct {
	w      *win
	seen   map[string]bool
	newest time.Time
}

// readClose runs /x. The checks go in order, the argument first, as
// /close-others checks it: an argument, then window 0, then a chat
// without a known message (nothing to report) are errors; on each
// nothing is sent and nothing closes. Otherwise the unread count is
// cleared and the report goes, or waits its turn, as /mark-as-read's;
// the window closes when its answer comes (closeReported). What an
// empty-prompt Tab held of the window is settled: /x reports it read
// itself, so leaving it puts no unread count back and the held receipt
// is not sent as a second one.
func (m *Model) readClose(arg string) tea.Cmd {
	if arg != "" {
		m.setErr(readCloseArgErr)
		return nil
	}
	w := m.wins[m.cur]
	if w.chatID == "" {
		m.setErr(readCloseServerErr)
		return nil
	}
	id, ok := m.newestID(w.chatID)
	if !ok {
		m.setErr("no message of " + m.nameOf(w.chatID) + " is known yet, nothing to report, window kept; /last fetches some, /close closes it unreported")
		return nil
	}
	if m.held != nil && m.held.w == w {
		m.held = nil
	}
	w.clearUnread()
	x := &readCloseReq{w: w, seen: map[string]bool{}}
	for _, msg := range domain.Filter(m.msgs, domain.Query{ChatID: w.chatID}) {
		x.seen[msg.ID] = true
		if msg.Time.After(x.newest) {
			x.newest = msg.Time
		}
	}
	m.readClosing[w.chatID] = x
	m.setInfo("reporting " + m.nameOf(w.chatID) + " read, then closing its window...")
	return m.report(w.chatID, "read", id, x)
}

// closeReported closes the window /x was typed in, now that the receipt
// and the unread mark of the report answering it went through. Each case
// that keeps the window says why, none as an error: the report itself
// went through.
//   - The request no longer stands: an [unread] for the chat removed it, or
//     a later /x replaced it and its own answer is still to come.
//   - The window is gone: closed by hand meanwhile. A window opened for the
//     chat since is another window and stays.
//   - The window counts unread messages again.
//   - A message arrived since /x: one not held then and not older than the
//     newest held then. The receipt does not cover it, or covers it unseen
//     (a [read] answering for /x reports up to the service's newest).
func (m *Model) closeReported(msg readMsg) {
	name := m.nameOf(msg.chatID)
	switch stands := m.readClosing[msg.chatID]; {
	case stands == nil:
		m.setInfo("reported " + name + " read; marked unread since, window kept")
		return
	case stands != msg.x:
		m.setInfo("reported " + name + " read")
		return
	}
	delete(m.readClosing, msg.chatID)
	i := m.indexOf(msg.x.w)
	switch {
	case i <= 0:
		m.setInfo("reported " + name + " read; its window was closed already")
	case msg.x.w.unread > 0:
		m.setInfo("reported " + name + " read; unread again since, window kept")
	case m.arrivedSince(msg.chatID, msg.x):
		m.setInfo("reported " + name + " read; a newer message arrived, window kept")
	default:
		m.closeWins([]int{i}) // close.go: the notice, the current window, the layout
		m.setInfo("reported " + name + " read, closed its window")
	}
}

// arrivedSince tells whether the chat holds a message that /x did not
// see: its id was not held then and it is not older than the newest held
// then. Older history fetched since (/last) is not an arrival.
func (m *Model) arrivedSince(chatID string, x *readCloseReq) bool {
	for _, msg := range domain.Filter(m.msgs, domain.Query{ChatID: chatID}) {
		if !x.seen[msg.ID] && !msg.Time.Before(x.newest) {
			return true
		}
	}
	return false
}

// keptAfter is what a failed report adds to its error when it answered a
// /x that stands and whose window is still open: the window stays. The
// request is over either way; /x again asks again.
func (m *Model) keptAfter(msg readMsg) string {
	if msg.x == nil || m.readClosing[msg.chatID] != msg.x {
		return ""
	}
	delete(m.readClosing, msg.chatID)
	if m.indexOf(msg.x.w) <= 0 {
		return ""
	}
	return "; window kept"
}
