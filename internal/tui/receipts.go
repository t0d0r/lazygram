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
// messages. /mark-as-read and the [read] button of /list and /win send one
// receipt for that chat on request, whatever the setting says, and change
// nothing about it; they also clear the chat's unread mark, and [unread]
// sets it. A receipt covers the newest message lazygram holds for the
// chat, so messages it has not shown stay unread at the service; a [read]
// for a chat with no message held covers the service's newest.

// readMsg is the outcome of one receipt; manual says the user asked for
// it, markErr is the outcome of clearing the unread mark with it.
type readMsg struct {
	chatID  string
	manual  bool
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
	return m, m.report(w.chatID, "read", id)
}

// Requested reports (/mark-as-read, [read], [unread]) for one chat go to
// the service one at a time, in the order asked: while one is on its way
// the latest further request for that chat waits, and earlier waiting ones
// are dropped, so the last press decides what the service ends with.
type pendingReport struct{ act, id string }

// report sends act ("read" or "unread") for chatID now, or queues it when a
// report for that chat is on its way. id is the read limit: a message id,
// or "" for the service's newest. Nil for the server window.
func (m *Model) report(chatID, act, id string) tea.Cmd {
	if chatID == "" {
		return nil
	}
	if m.reporting[chatID] {
		m.reportNext[chatID] = pendingReport{act, id}
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
		return readMsg{chatID: chatID, manual: true, err: err, markErr: p.MarkUnread(ctx, chatID, false)}
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
	return m.report(chatID, next.act, next.id)
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
func (m *Model) newestID(chatID string) (string, bool) {
	msgs := domain.Filter(m.msgs, domain.Query{ChatID: chatID})
	if len(msgs) == 0 {
		return "", false
	}
	return msgs[len(msgs)-1].ID, true
}
