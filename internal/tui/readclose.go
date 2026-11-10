package tui

import tea "github.com/charmbracelet/bubbletea"

// /x is /mark-as-read, then /close, for the current chat window
// (specs/ui.md, Read receipts): one receipt for the chat on request,
// whatever /read-receipts says, and when the service answered without an
// error the window closes, as /close closes it. The report goes first and
// its answer decides: a failed receipt or unread mark is said as
// /mark-as-read says it, with "window kept", and nothing closes, so a
// closed window always means the report went through.

// What /x says instead of reporting. It takes no argument: it is for the
// current window only. Window 0 gets the error of every command that
// needs a chat window.
const (
	readCloseArgErr    = "/x takes no argument; it reports this chat window read and closes it"
	readCloseServerErr = "the server window has no chat; /win to a chat first"
)

// readClose runs /x. The checks go in order, the argument first, as
// /close-others checks it: an argument, then window 0, then a chat
// without a known message (nothing to report) are errors; on each
// nothing is sent and nothing closes. Otherwise the unread count is
// cleared and the report goes, or waits its turn, as /mark-as-read's;
// the window closes when its answer comes (closeReported).
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
	w.clearUnread()
	m.setInfo("reporting " + m.nameOf(w.chatID) + " read, then closing its window...")
	return m.report(w.chatID, "read", id, true)
}

// closeReported closes the window of the chat /x reported read, now that
// the receipt and the unread mark went through. The window is found by
// its chat, so it closes whatever window is current by now; one closed by
// hand meanwhile is not closed again. A message newer than the one the
// receipt covered arrived after /x and is not reported, so the window
// stays to show it. Each case says what happened, none as an error: the
// report itself went through.
func (m *Model) closeReported(msg readMsg) {
	name := m.nameOf(msg.chatID)
	i, w := m.winFor(msg.chatID)
	if w == nil {
		m.setInfo("reported " + name + " read; its window was closed already")
		return
	}
	if newest, ok := m.newestID(msg.chatID); msg.id != "" && ok && newest != msg.id {
		m.setInfo("reported " + name + " read; a newer message arrived, window kept")
		return
	}
	m.closeWins([]int{i}) // close.go: the notice, the current window, the layout
	m.setInfo("reported " + name + " read, closed its window")
}

// keptAfter is what a failed report adds to its error when /x asked for
// it and the chat's window is still open: the window stays.
func (m *Model) keptAfter(msg readMsg) string {
	if _, w := m.winFor(msg.chatID); !msg.closing || w == nil {
		return ""
	}
	return "; window kept"
}
