package tui

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lazygram/internal/domain"
)

// Read receipts are opt-in and off by default. With them on, lazygram
// tells the service that a chat was read when it marks the chat read
// itself: when a window with unread messages is focused (focus). After an
// empty-prompt Tab, whatever /doubletab says, the receipt waits until the
// window has stayed current for tabGap (the double Tab's time); a window
// left sooner was only passed through, gets its unread count back and is
// not reported for that Tab (lastwin.go).
// Nothing is sent for opening a list, filtering, searching, a message
// arriving in the window already shown, or a window without unread
// messages. /mark-as-read, /x and the [read] button of /list and /win send
// one receipt for that chat on request, whatever the setting says, and
// change nothing about it; they also clear the chat's unread mark, and
// [unread] sets it. /x then closes the window (readclose.go). A receipt
// covers the newest message lazygram holds for the chat, so messages it
// has not shown stay unread at the service; a [read] for a chat with no
// message held covers the service's newest.

// readMsg is the outcome of one receipt; manual says the user asked for
// it, markErr is the outcome of clearing the unread mark with it. x is
// the /x request this report answers, if any (readclose.go).
type readMsg struct {
	chatID  string
	manual  bool
	x       *readCloseReq
	err     error
	markErr error
}

// unreadMsg is the outcome of setting a chat's unread mark.
type unreadMsg struct {
	chatID string
	err    error
}

// readReceipts implements /read-receipts: no argument shows the setting,
// on or off sets and keeps it.
func (m *Model) readReceipts(arg string) {
	switch strings.ToLower(strings.TrimSpace(arg)) {
	case "":
		state := "off: focusing a window reports nothing (/mark-as-read and the list buttons still report on request)"
		if m.receipts {
			state = "on: focusing a window with unread messages reports it read"
		}
		m.setInfo("read receipts " + state + "; /read-receipts on | off, /mark-as-read for one chat now")
	case "on":
		m.receipts = true
		_ = m.opt.Store.SetState(receiptsKey, true)
		m.setInfo("read receipts on: focusing a window with unread messages reports it read to " + m.opt.Provider.Name())
	case "off":
		m.receipts = false
		_ = m.opt.Store.SetState(receiptsKey, false)
		m.setInfo("read receipts off: focusing a window reports nothing")
	default:
		m.setErr("/read-receipts takes on or off")
	}
}

// readCmd is the receipt for focusing w, or nil when nothing is to be
// sent: receipts off, the server window, no unread messages, or no
// message of the chat known yet. It reads w.unread before the caller
// clears it.
func (m *Model) readCmd(w *win) tea.Cmd {
	if !m.receipts || w.chatID == "" || w.unread == 0 {
		return nil
	}
	return m.receiptCmd(w.chatID, false)
}

// markAsRead implements /mark-as-read: one receipt for the current
// window's chat, now, independent of the setting, which it leaves alone.
// The server window has no chat and a chat without a known message has
// nothing to report; both are errors, and nothing is sent.
func (m *Model) markAsRead() (tea.Model, tea.Cmd) {
	w := m.wins[m.cur]
	if w.chatID == "" {
		m.setErr("the server window has no chat; /win to a chat first")
		return m, nil
	}
	id, ok := m.newestID(w.chatID)
	if !ok {
		m.setErr("no message of " + m.nameOf(w.chatID) + " is known yet, nothing to report; /last fetches some")
		return m, nil
	}
	w.clearUnread()
	m.setInfo("reporting " + m.nameOf(w.chatID) + " read...")
	return m, m.report(w.chatID, "read", id, nil)
}

// Requested reports (/mark-as-read, [read], [unread]) for one chat go to
// the service one at a time, in the order asked: while one is on its way
// the latest further request for that chat waits, and earlier waiting ones
// are dropped, so the last press decides what the service ends with.
// A waiting /x's request passes to a later /mark-as-read that replaces its
// report, which is bounded by a message id like its own. A [read] is not
// (it reports up to the service's newest, seen here or not), so it and an
// [unread] replace a waiting /x without its closing; an [unread] cancels
// the chat's /x also when its report is on its way.
type pendingReport struct {
	act, id string
	x       *readCloseReq // the /x request this report answers, if any
}

// report sends act ("read" or "unread") for chatID now, or queues it when a
// report for that chat is on its way. id is the read limit: a message id,
// or "" for the service's newest; x is the /x request it answers, if any
// (readclose.go). Nil for the server window.
func (m *Model) report(chatID, act, id string, x *readCloseReq) tea.Cmd {
	if chatID == "" {
		return nil
	}
	if act == "unread" { // a chat marked unread is not one to put away
		delete(m.readClosing, chatID)
	}
	if m.reporting[chatID] {
		if was := m.reportNext[chatID].x; was != nil && x == nil {
			if act == "read" && id != "" {
				x = was // the waiting /x is answered by this report instead
			} else {
				if m.readClosing[chatID] == was {
					delete(m.readClosing, chatID)
				}
				m.setInfo(m.status + "; the waiting /x is dropped, its window stays")
			}
		}
		m.reportNext[chatID] = pendingReport{act, id, x}
		return nil
	}
	m.reporting[chatID] = true
	p := m.opt.Provider
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if act == "unread" {
			return unreadMsg{chatID: chatID, err: p.MarkUnread(ctx, chatID, true)}
		}
		// Both even when one fails: the mark needs no message.
		err := p.MarkRead(ctx, chatID, id)
		return readMsg{chatID: chatID, manual: true, x: x, err: err, markErr: p.MarkUnread(ctx, chatID, false)}
	}
}

// reported is called when a requested report came back: the next waiting
// request for the chat, if any, goes now.
func (m *Model) reported(chatID string) tea.Cmd {
	delete(m.reporting, chatID)
	next, ok := m.reportNext[chatID]
	if !ok {
		return nil
	}
	delete(m.reportNext, chatID)
	return m.report(chatID, next.act, next.id, next.x)
}

// receiptCmd reports chatID read up to the newest message lazygram holds
// for it, or returns nil when none is known.
func (m *Model) receiptCmd(chatID string, manual bool) tea.Cmd {
	id, ok := m.newestID(chatID)
	if !ok {
		return nil
	}
	p := m.opt.Provider
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return readMsg{chatID: chatID, manual: manual, err: p.MarkRead(ctx, chatID, id)}
	}
}

// newestID is the id of the newest message lazygram holds for a chat.
// Messages of one second are ordered by id as text, "100" before "99"
// (domain.Merge); among those of the newest second the id that comes last
// as a number is the newest, so a receipt covers all of them.
func (m *Model) newestID(chatID string) (string, bool) {
	msgs := domain.Filter(m.msgs, domain.Query{ChatID: chatID})
	if len(msgs) == 0 {
		return "", false
	}
	last := msgs[len(msgs)-1]
	id := last.ID
	for i := len(msgs) - 2; i >= 0 && msgs[i].Time.Equal(last.Time); i-- {
		if idAfter(msgs[i].ID, id) {
			id = msgs[i].ID
		}
	}
	return id, true
}

// idAfter tells whether message id a comes after b: as whole numbers of
// any size when both are digits only (Telegram's ids), else as text.
func idAfter(a, b string) bool {
	if isDigits(a) && isDigits(b) {
		a, b = strings.TrimLeft(a, "0"), strings.TrimLeft(b, "0")
		if len(a) != len(b) {
			return len(a) > len(b)
		}
	}
	return a > b
}

func isDigits(s string) bool { return s != "" && strings.Trim(s, "0123456789") == "" }
