package tui

import (
	"bytes"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"lazygram/internal/store"
)

// startWith starts a model on st as the program does: the cached chat
// list and layout first, then the service's chat list and a sync.
func startWith(t *testing.T, st *store.Store, f *fake) *Model {
	t.Helper()
	m := New(Options{Store: st, Provider: f})
	m.fx = false
	m.revealAll()
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	m.Update(m.chatsCmd()())
	m.Update(m.syncCmd()())
	return m
}

func memStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(store.Memory, bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestUnreadChatsOpenAtEveryStart(t *testing.T) {
	st := memStore(t)
	f := newFake()                               // #news has 4 unread at the service
	if err := st.SetChats(f.chats); err != nil { // a cached chat list from an earlier run
		t.Fatal(err)
	}
	m := startWith(t, st, f)
	i, w := m.winFor("c3")
	if w == nil || w.unread != 4 {
		t.Fatalf("#news opens with its unread count although a chat list was cached: %v", w)
	}
	if v := body(m); !strings.Contains(v, "4 chats, 1 with unread messages: ") || !strings.Contains(v, "#news (4)") {
		t.Errorf("the notice names the windows:\n%s", v)
	}
	// A refresh later in the session reopens nothing that was closed.
	m.closeWin(i)
	m.Update(m.chatsCmd()())
	if _, w := m.winFor("c3"); w != nil {
		t.Error("a refresh reopened a closed window")
	}
}

func TestRestoredWindowTakesTheUnreadCount(t *testing.T) {
	st := memStore(t)
	first := startWith(t, st, newFake()) // opens #news and keeps the layout
	n := len(first.wins)
	_, w := first.winFor("c3")
	w.unread = 0
	m := startWith(t, st, newFake())
	if len(m.wins) != n {
		t.Fatalf("no second window for a restored chat: %d windows, want %d", len(m.wins), n)
	}
	if _, w := m.winFor("c3"); w == nil || w.unread != 4 {
		t.Fatalf("the restored window takes the service's count: %v", w)
	}
}

func TestAutojoinOffIsKept(t *testing.T) {
	st := memStore(t)
	m := startWith(t, st, newFake())
	enter(m, "/autojoin")
	if m.isErr || !strings.Contains(m.status, "autojoin on") {
		t.Fatalf("on by default: %q", m.status)
	}
	enter(m, "/autojoin off")
	if m.isErr || !strings.Contains(m.status, "from the next start") {
		t.Fatalf("off: %q", m.status)
	}
	if i, w := m.winFor("c3"); w != nil {
		m.closeWin(i) // so the layout does not bring it back
	}
	m2 := startWith(t, st, newFake())
	if _, w := m2.winFor("c3"); w != nil {
		t.Fatal("autojoin off opens no window at the next start")
	}
	if v := body(m2); !strings.Contains(v, "/autojoin is off: no windows opened") {
		t.Errorf("the notice says why:\n%s", v)
	}
	enter(m2, "/autojoin x")
	if !m2.isErr || !strings.Contains(m2.status, "on or off") {
		t.Errorf("a bad argument: %q", m2.status)
	}
	m2.input = nil
	enter(m2, "/autojoin on")
	m3 := startWith(t, st, newFake())
	if _, w := m3.winFor("c3"); w == nil {
		t.Error("on again: the next start opens it")
	}
}

func TestTabCompletesEachCommandsOwnArguments(t *testing.T) {
	m := setup(t, newFake())
	for typed, want := range map[string]string{
		"/autojoin of":   "/autojoin off ",
		"/autojoin al":   "/autojoin al", // a setting, not a chat
		"/receipts of":   "/receipts off ",
		"/clip t":        "/clip tool ",
		"/clipboard au":  "/clipboard auto ",
		"/j li":          "/j list ", // every spelling of /win
		"/win al":        "/win @alice ",
		"/msg al":        "/msg @alice ",
		"/msg @alice al": "/msg @alice al", // the message after the chat is free text
		"/whois ne":      "/whois #news ",
		"/last al":       "/last al", // numbers
		"/search al":     "/search al",
		"/press al":      "/press al",
		"hi al":          "hi @alice ", // message text names chats, as before
	} {
		m.input = nil
		typ(m, typed)
		press(m, tea.KeyTab)
		if got := string(m.input); got != want {
			t.Errorf("%q: got %q, want %q", typed, got, want)
		}
	}
	m.input = nil
	typ(m, "/autojoin o")
	press(m, tea.KeyTab)
	press(m, tea.KeyTab)
	if v := body(m); !strings.Contains(v, "off  on") && !strings.Contains(v, "on  off") {
		t.Errorf("the second Tab lists on and off:\n%s", v)
	}
}
