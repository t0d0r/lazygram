package tui

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lazygram/internal/domain"
)

// Read receipts are opt-in and off by default. With them on, lazygram
// tells the service that a chat was read exactly when it marks the chat
// read itself: when a window with unread messages is focused (switchTo).
// Nothing is sent for opening a list, filtering, searching, a message
// arriving in the window already shown, or a window without unread
// messages. /mark-as-read sends one receipt for the current window's chat
// on request, whatever the setting says, and changes nothing about it.
// A receipt covers the newest message lazygram holds for the chat, so
// messages it has not shown stay unread at the service.

// readMsg is the outcome of one receipt; manual says /mark-as-read asked.
type readMsg struct {
	chatID string
	manual bool
	err    error
}

// readReceipts implements /read-receipts: no argument shows the setting,
// on or off sets and keeps it.
func (m *Model) readReceipts(arg string) {
	switch strings.ToLower(strings.TrimSpace(arg)) {
	case "":
		state := "off: nothing is reported to the service"
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
		m.setInfo("read receipts off: nothing is reported")
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
	cmd := m.receiptCmd(w.chatID, true)
	if cmd == nil {
		m.setErr("no message of " + m.nameOf(w.chatID) + " is known yet, nothing to report; /last fetches some")
		return m, nil
	}
	w.unread = 0
	m.setInfo("reporting " + m.nameOf(w.chatID) + " read...")
	return m, cmd
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
