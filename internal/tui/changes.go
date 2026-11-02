package tui

import (
	"sort"
	"time"

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

// applyChange puts one change into memory at once and into the cache: an
// edit right here (one row), a deletion by a command, since one without a
// chat touches every chat that shares ids.
func (m *Model) applyChange(c domain.Change) tea.Cmd {
	st := m.opt.Store
	switch c.Kind {
	case domain.Edited:
		now, ok, err := st.Edit(c.Msg)
		if err != nil || !ok {
			return nil // not cached: nothing shows it
		}
		m.identify([]domain.Message{now})
	case domain.Deleted:
		chats := []string{c.ChatID}
		if c.ChatID == "" {
			chats = m.sharedList()
		}
		m.dropAcross(chats, c.IDs)
		return func() tea.Msg {
			_, _ = st.DeleteMessages(chats, c.IDs)
			return nil
		}
	case domain.Stale:
		// Updates were lost (a gap the service would not replay): the
		// affected windows fetch their history again, the shown one now.
		for _, w := range m.wins {
			if w.chatID != "" && (c.ChatID == "" || w.chatID == c.ChatID) {
				w.healed = time.Time{}
			}
		}
		return m.healCmd(m.wins[m.cur])
	}
	return nil
}

// healCmd fetches a loaded chat window's newest history again, quietly,
// when it was last fetched more than healEvery ago (or a gap was
// reported), so changes the update stream missed (a sleep, a reconnect)
// still arrive; nil when nothing is due.
func (m *Model) healCmd(w *win) tea.Cmd {
	if w.chatID == "" || !w.loaded || m.loading[w.chatID] || m.now().Sub(w.healed) < healEvery {
		return nil
	}
	w.healed = m.now() // one try per period, whatever the answer
	return m.fetchRecent(w.chatID, recentOnOpen, true)
}

// learnShared adds the chats the service marks SharedIDs to the kept set:
// a deletion without a chat may be about a chat no longer among the recent
// dialogs, whose messages are still cached.
func (m *Model) learnShared() {
	added := false
	for _, c := range m.chats {
		if c.SharedIDs && !m.sharedChats[c.ID] {
			m.sharedChats[c.ID] = true
			added = true
		}
	}
	if added {
		_ = m.opt.Store.SetState(sharedKey, m.sharedList())
	}
}

// sharedList is the kept set of chats with shared ids, sorted.
func (m *Model) sharedList() []string {
	out := make([]string, 0, len(m.sharedChats))
	for id := range m.sharedChats {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// dropAcross removes the messages with the given ids from the given chats
// in memory, in one pass. A message that arrived this session from someone
// else takes its count off its window's unread messages.
func (m *Model) dropAcross(chats, ids []string) {
	inChat := make(map[string]bool, len(chats))
	for _, c := range chats {
		inChat[c] = true
	}
	isID := make(map[string]bool, len(ids))
	for _, id := range ids {
		isID[id] = true
	}
	kept := m.msgs[:0]
	for _, x := range m.msgs {
		if !inChat[x.ChatID] || !isID[x.ID] {
			kept = append(kept, x)
			continue
		}
		if m.live[x.Key()] && x.Sender != "me" {
			if _, w := m.winFor(x.ChatID); w != nil && w.unread > 0 {
				w.unread--
			}
		}
		delete(m.live, x.Key())
	}
	m.msgs = kept
	m.rcache = &renderCache{}
}
