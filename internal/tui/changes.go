package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"lazygram/internal/domain"
	"lazygram/internal/provider"
)

// Edits and deletions the service reports while lazygram runs (made here,
// on another device, or by the other side) are applied to the cache and
// the windows at once: an edited message reads as it does now, a deleted
// one disappears, with its attachment, and nothing says it was there. A
// change the feed dropped, or one made while lazygram was off, is healed
// the next time the chat's history is fetched (Store.Refresh, Store.Prune).

type (
	changeMsg     struct{ c domain.Change } // one change from the service
	changeDoneMsg struct{}                  // the feed closed
)

// changesInit starts reading the service's changes, when it has any.
func (m *Model) changesInit() tea.Cmd {
	cs, ok := m.opt.Provider.(provider.ChangeSource)
	if !ok {
		return nil
	}
	ch := cs.Changes()
	if ch == nil {
		return nil
	}
	m.changeCh = ch
	return waitChange(ch)
}

// waitChange delivers the next change, or changeDoneMsg once the feed is
// closed; it is re-armed after each change.
func waitChange(ch <-chan domain.Change) tea.Cmd {
	return func() tea.Msg {
		c, ok := <-ch
		if !ok {
			return changeDoneMsg{}
		}
		return changeMsg{c: c}
	}
}

// applyChange puts one change into the cache and memory.
func (m *Model) applyChange(c domain.Change) {
	st := m.opt.Store
	switch c.Kind {
	case domain.Edited:
		now, ok, err := st.Edit(c.Msg)
		if err != nil || !ok {
			return // not cached: nothing shows it
		}
		m.identify([]domain.Message{now})
	case domain.Deleted:
		chats := []string{c.ChatID}
		if c.ChatID == "" {
			chats = chats[:0]
			for _, ch := range m.chats {
				if ch.SharedIDs {
					chats = append(chats, ch.ID)
				}
			}
		}
		refs, _ := st.DeleteMessages(chats, c.IDs)
		byChat := map[string][]string{}
		for _, r := range refs {
			byChat[r.ChatID] = append(byChat[r.ChatID], r.ID)
		}
		if c.ChatID == "" { // memory may hold what the store did not
			for _, ch := range chats {
				byChat[ch] = c.IDs
			}
		} else {
			byChat[c.ChatID] = c.IDs
		}
		for chat, ids := range byChat {
			m.dropMsgs(chat, ids)
		}
	}
}
