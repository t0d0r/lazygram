package tui

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"lazygram/internal/domain"
	"lazygram/internal/store"
)

// firstStart is setup before its chat list and sync answer: nothing
// cached, both asked for at once, as Init leaves them.
func firstStart(t *testing.T, f *fake) *Model {
	t.Helper()
	st, err := store.Open(store.Memory, bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	m := New(Options{Store: st, Provider: f})
	m.fx = false
	m.revealAll()
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	m.syncing, m.chatsAsked = true, true
	return m
}

// told is everything window 0's notices and the status line say.
func told(m *Model) string {
	var b strings.Builder
	for _, n := range m.wins[0].notes {
		b.WriteString(n.text + "\n")
	}
	return b.String() + m.status
}

// winChats lists the chat of every window after the server window.
func winChats(m *Model) []string {
	var ids []string
	for _, w := range m.wins[1:] {
		ids = append(ids, w.chatID)
	}
	return ids
}

// TestStartSyncWaitsForTheChatList: on a first start the sync can answer
// before the chat list; its result waits, so the windows, the notices and
// the status name the chats (never "c1"), the start check opens its
// windows first, and a deleted account opens none.
func TestStartSyncWaitsForTheChatList(t *testing.T) {
	f := newFake()
	f.msgs = append(f.msgs, mk("c4", "9", "ghost", "from a deleted account", 4))
	m := firstStart(t, f)
	m.Update(m.syncCmd()())
	if len(m.wins) != 1 || m.heldSync == nil || !m.syncing || strings.Contains(told(m), "opened for") {
		t.Fatalf("the sync waits for the chat list: %d windows, held %v, syncing %v\n%s", len(m.wins), m.heldSync != nil, m.syncing, told(m))
	}
	m.Update(m.chatsCmd()())
	if m.heldSync != nil || m.syncing {
		t.Fatalf("the chat list releases the sync: held %v, syncing %v", m.heldSync != nil, m.syncing)
	}
	if got := strings.Join(winChats(m), " "); got != "c3 c1 c2" {
		t.Fatalf("windows: %q, want #news (the start check), then #dev and @alice; none for the deleted account", got)
	}
	s := told(m)
	for _, want := range []string{"window 1 opened for #news", "window 2 opened for #dev", "window 3 opened for @alice", "in 2:#dev (2), 3:@alice (2)"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in\n%s", want, s)
		}
	}
	for _, id := range []string{"c1", "c2", "c3", "c4"} {
		if strings.Contains(s, " "+id) || strings.Contains(s, ":"+id) {
			t.Errorf("%s named by its id in\n%s", id, s)
		}
	}
}

// TestNewChatAsksForTheChatList: a chat that writes for the first time
// later in the session is not in the list; the sync asks for it and waits,
// so its window is named. A chat the service never lists is asked for once
// and then named by its id; what the account sent opens no window and asks
// for nothing.
func TestNewChatAsksForTheChatList(t *testing.T) {
	f := newFake()
	m := setup(t, f)
	f.chats = append(f.chats, domain.Chat{ID: "c5", Alias: "bob", Title: "Bob", Kind: domain.ChatUser})
	f.msgs = append(f.msgs, mk("c5", "1", "bob", "hello, new here", 5))
	m.syncing = true
	_, cmd := m.Update(m.syncCmd()())
	if m.heldSync == nil || !m.chatsAsked || !m.syncing || strings.Contains(told(m), "c5") {
		t.Fatalf("a new chat waits for the chat list: held %v, asked %v\n%s", m.heldSync != nil, m.chatsAsked, told(m))
	}
	runAll(m, cmd)
	if m.heldSync != nil || m.syncing || !strings.Contains(told(m), "opened for @bob") {
		t.Fatalf("the new chat's window is named: held %v, syncing %v\n%s", m.heldSync != nil, m.syncing, told(m))
	}

	f.msgs = append(f.msgs, mk("c6", "1", "eve", "not in any list", 6))
	m.syncing = true
	_, cmd = m.Update(m.syncCmd()())
	if m.heldSync == nil {
		t.Fatal("an unknown chat is asked for once")
	}
	runAll(m, cmd)
	if m.heldSync != nil || !strings.Contains(told(m), "opened for c6") {
		t.Fatalf("still unknown after the chat list: named by its id\n%s", told(m))
	}
	_, w := m.winFor("c6")
	f.msgs = append(f.msgs, mk("c6", "2", "eve", "again", 7))
	m.syncing = true
	m.Update(m.syncCmd()())
	if m.heldSync != nil || m.chatsAsked || m.syncing || w.unread != 2 {
		t.Fatalf("asked once: no second wait (held %v, asked %v, syncing %v, unread %d)", m.heldSync != nil, m.chatsAsked, m.syncing, w.unread)
	}

	f.msgs = append(f.msgs, mk("c7", "1", "me", "sent from my phone", 8))
	m.syncing = true
	m.Update(m.syncCmd()())
	if m.heldSync != nil || m.chatsAsked || m.syncing {
		t.Fatalf("what the account sent waits for nothing: held %v, asked %v", m.heldSync != nil, m.chatsAsked)
	}
	if _, w := m.winFor("c7"); w != nil {
		t.Fatal("what the account sent opens no window")
	}
}

// TestChatListFailsNamesFallBack: when the chat list fails the waiting
// sync goes ahead, the chats named by their ids; with no chat list this
// session and none on its way a sync waits for nothing (the smoke mode's
// order after a failed chat list).
func TestChatListFailsNamesFallBack(t *testing.T) {
	m := firstStart(t, newFake())
	m.Update(m.syncCmd()())
	m.Update(chatsMsg{err: errors.New("offline")})
	if m.heldSync != nil || m.syncing || strings.Join(winChats(m), " ") != "c1 c2" || !strings.Contains(told(m), "opened for c1") {
		t.Fatalf("a failed chat list lets the sync go ahead: held %v, syncing %v, windows %v\n%s", m.heldSync != nil, m.syncing, winChats(m), told(m))
	}

	m = firstStart(t, newFake())
	m.Update(chatsMsg{err: errors.New("offline")})
	m.syncing = true
	m.Update(m.syncCmd()())
	if m.heldSync != nil || m.syncing || len(m.wins) != 3 {
		t.Fatalf("no chat list on its way: nothing waits (held %v, syncing %v, %d windows)", m.heldSync != nil, m.syncing, len(m.wins))
	}
}
