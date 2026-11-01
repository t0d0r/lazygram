package tui

import (
	"fmt"
	"sort"
	"strings"
)

// At start, once the service's chat list is in, every chat with unread
// messages gets a window (autojoin, as IRC clients join their channels on
// connect), unless /autojoin off said otherwise; windows restored from
// the last session take the service's unread counts. A window fetches its
// messages when first shown, as always, so many unread chats cost no
// burst of requests.

// startCheck runs once a session, on the first chat list from the
// service (when the first request fails, each poll asks again, so it
// runs on connect, not at a later manual /refresh): a window that exists takes its chat's unread count (unless it
// is the one shown), and with autojoin on every other chat with unread
// messages opens a window; deleted accounts never do. The server window
// says what came of it.
func (m *Model) startCheck() {
	var order []int
	count := map[int]int{}
	unread := 0
	for _, c := range m.chats {
		if c.Unread == 0 || c.Deleted {
			continue
		}
		unread++
		i, w := m.winFor(c.ID)
		if w == nil {
			if !m.autojoin {
				continue
			}
			i, w = m.openWin(c.ID)
		}
		if i == m.cur {
			continue // shown: its first fetch is under way
		}
		if w.unread == 0 {
			w.unread = c.Unread
		}
		order = append(order, i)
		count[i] = c.Unread
	}
	sort.Ints(order)
	text := fmt.Sprintf("%d chats, %d with unread messages", len(m.chats), unread)
	if where := m.whereList(order, count); len(where) > 0 {
		text += ": " + strings.Join(where, ", ")
	}
	if !m.autojoin && unread > 0 {
		text += " (/autojoin is off: no windows opened; /list shows them)"
	}
	m.note(m.wins[0], text)
}

// whenAutojoin says when a changed setting first acts: at the next start,
// or, while the chat list of this start has not come yet, when it does.
func (m *Model) whenAutojoin() string {
	if m.startChecked {
		return "from the next start"
	}
	return "when the chat list arrives"
}

// setAutojoin implements /autojoin: no argument shows the setting, on or
// off sets and keeps it for the next start.
func (m *Model) setAutojoin(arg string) {
	switch strings.ToLower(strings.TrimSpace(arg)) {
	case "":
		if m.autojoin {
			m.setInfo("autojoin on: at start a window opens for every chat with unread messages; /autojoin off")
		} else {
			m.setInfo("autojoin off: at start chats with unread messages open no window; /autojoin on")
		}
	case "on":
		m.autojoin = true
		_ = m.opt.Store.SetState(autojoinKey, true)
		m.setInfo("autojoin on: " + m.whenAutojoin() + " a window opens for every chat with unread messages")
	case "off":
		m.autojoin = false
		_ = m.opt.Store.SetState(autojoinKey, false)
		m.setInfo("autojoin off: " + m.whenAutojoin() + " chats with unread messages open no window (/list shows them)")
	default:
		m.setErr("/autojoin takes on or off")
	}
}
