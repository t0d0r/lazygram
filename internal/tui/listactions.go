package tui

import (
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// List actions: the selected row of /list or /win carries a strip of
// buttons at its right; Left/Right move the choice, Enter runs it, a
// click runs the clicked one. The first button is the row's default
// (open a chat, focus a window), so with nothing chosen Enter does what
// it always did. Read and unread keep the list open on the same row with
// the same button, so Down, Enter repeats them down the list; close keeps
// the list open too. Everything here is local, except that read follows
// the read-receipt rule (receipts.go): with /read-receipts on, marking a
// window with unread messages read reports it like a focus would.

// Action names; the first entry of each strip is the default.
var (
	winActions  = []string{"focus", "read", "unread", "close"}
	chatActions = []string{"open", "read", "unread"}
)

// listActions is the button strip of the current list.
func (m *Model) listActions() []string {
	if m.listSrc == listWindows {
		return winActions
	}
	return chatActions
}

// listButtons renders the strip for the selected row, which starts at
// column x0 on its screen row, with the chosen button highlighted, and
// returns one click span per button.
func (m *Model) listButtons(x0 int) (string, []span) {
	acts := m.listActions()
	m.listAct = min(max(m.listAct, 0), len(acts)-1)
	var b strings.Builder
	var spans []span
	x := x0
	for i, a := range acts {
		if i > 0 {
			b.WriteString(" ")
			x++
		}
		label := "[" + a + "]"
		if i == m.listAct {
			b.WriteString(hiStyle.Render(label))
		} else {
			b.WriteString(buttonStyle.Render(label))
		}
		w := lipgloss.Width(label)
		spans = append(spans, span{x, x + w, spanAct, strconv.Itoa(m.listCur) + ":" + strconv.Itoa(i)})
		x += w
	}
	return b.String(), spans
}

// runListAction applies the chosen button to the selected row. The
// default button behaves like Enter did before; the others keep the list
// open so the next row is one Down away.
func (m *Model) runListAction() (tea.Model, tea.Cmd) {
	acts := m.listActions()
	act := acts[min(max(m.listAct, 0), len(acts)-1)]
	if m.listSrc == listWindows {
		i := min(max(m.listCur, 0), len(m.wins)-1)
		w := m.wins[i]
		switch act {
		case "focus":
			m.input, m.curBack = nil, 0
			return m, m.switchTo(i)
		case "read":
			receipt := m.readCmd(w) // with read receipts on, reported like a focus; nil otherwise
			w.unread = 0
			m.setInfo("marked " + m.winName(i) + " read")
			return m, receipt
		case "unread":
			w.unread = max(w.unread, 1)
			m.setInfo("marked " + m.winName(i) + " unread")
		case "close":
			m.closeWin(i) // refuses window 0 with its own message
			m.listCur = min(m.listCur, len(m.wins)-1)
		}
		return m, nil
	}
	items := m.listItems() // the typed filter still applies
	if len(items) == 0 {   // nothing to act on: Enter closes the list, as it always did
		m.input, m.curBack = nil, 0
		m.mode = modeWin
		return m, nil
	}
	c := items[min(m.listCur, len(items)-1)]
	i, w := m.winFor(c.ID)
	switch act {
	case "open":
		m.input, m.curBack = nil, 0
		if w == nil {
			i, _ = m.openWin(c.ID)
		}
		return m, m.switchTo(i)
	case "read":
		var receipt tea.Cmd
		if w != nil {
			receipt = m.readCmd(w) // with read receipts on, reported like a focus; nil otherwise
			w.unread = 0
		}
		m.setChatUnread(c.ID, 0)
		m.setInfo("marked " + m.nameOf(c.ID) + " read")
		return m, receipt
	case "unread":
		if w == nil { // a window so that it shows in Act; the view stays where it is
			_, w = m.openWin(c.ID)
		}
		w.unread = max(w.unread, 1)
		m.setChatUnread(c.ID, max(c.Unread, 1))
		m.setInfo("marked " + m.nameOf(c.ID) + " unread")
	}
	return m, nil
}

// setChatUnread changes a chat's unread count as the list shows it. The
// count is local: the next chats refresh brings the service's own back.
func (m *Model) setChatUnread(chatID string, n int) {
	for i := range m.chats {
		if m.chats[i].ID == chatID {
			m.chats[i].Unread = n
		}
	}
}
