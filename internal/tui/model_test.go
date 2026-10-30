package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lazygram/internal/domain"
	"lazygram/internal/provider"
	"lazygram/internal/store"
)

type fake struct {
	chats []domain.Chat
	msgs  []domain.Message
	err   error
}

func (f *fake) Name() string                                              { return "fake" }
func (f *fake) Chats(context.Context) ([]domain.Chat, error)              { return f.chats, f.err }
func (f *fake) Sync(context.Context, time.Time) ([]domain.Message, error) { return f.msgs, f.err }

func newFake() *fake {
	base := time.Now().Add(-time.Hour)
	mk := func(chat, id, who, text string, min int) domain.Message {
		return domain.Message{ID: id, ChatID: chat, Sender: who, Text: text, Time: base.Add(time.Duration(min) * time.Minute)}
	}
	return &fake{
		chats: []domain.Chat{{ID: "c1", Alias: "dev", Title: "Dev"}, {ID: "c2", Alias: "ops", Title: "Ops"}},
		msgs: []domain.Message{
			mk("c1", "1", "ada", "first dev line", 0),
			mk("c2", "1", "grace", "ops alert", 1),
			mk("c1", "2", "alan", "second dev line", 2),
		},
	}
}

func setup(t *testing.T, f *fake) *Model {
	t.Helper()
	st, _ := store.Open("")
	m := New(Options{Store: st, Provider: f})
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	m.Update(m.chatsCmd()())
	m.Update(m.syncCmd()())
	return m
}

func typ(m *Model, s string) {
	for _, r := range s {
		if r == ' ' {
			m.Update(tea.KeyMsg{Type: tea.KeySpace})
		} else {
			m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		}
	}
}

func press(m *Model, t tea.KeyType) { m.Update(tea.KeyMsg{Type: t}) }

func TestFeedInterleavesChatsInOrder(t *testing.T) {
	v := setup(t, newFake()).View()
	a, b, c := strings.Index(v, "first dev line"), strings.Index(v, "ops alert"), strings.Index(v, "second dev line")
	if a < 0 || (a >= b || b >= c) {
		t.Fatalf("expected interleaved chronological order:\n%s", v)
	}
	for _, want := range []string{"#dev", "#ops", "<ada>", "[fake]", "3 msgs"} {
		if !strings.Contains(v, want) {
			t.Errorf("view lacks %q", want)
		}
	}
}

func TestSelectCommandAndAll(t *testing.T) {
	m := setup(t, newFake())
	typ(m, "/select ops")
	press(m, tea.KeyEnter)
	v := m.View()
	if strings.Contains(v, "first dev line") || !strings.Contains(v, "ops alert") {
		t.Fatalf("select should scope the feed:\n%s", v)
	}
	typ(m, "/all")
	press(m, tea.KeyEnter)
	if !strings.Contains(m.View(), "first dev line") {
		t.Fatal("/all should restore the feed")
	}
}

func TestTabCyclesChats(t *testing.T) {
	m := setup(t, newFake())
	press(m, tea.KeyTab)
	if m.sel.Alias != "dev" {
		t.Fatalf("got %+v", m.sel)
	}
	press(m, tea.KeyTab)
	press(m, tea.KeyTab)
	if m.sel.ID != "" {
		t.Fatalf("should wrap to all, got %+v", m.sel)
	}
	press(m, tea.KeyShiftTab)
	if m.sel.Alias != "ops" {
		t.Fatalf("got %+v", m.sel)
	}
}

func TestFilterCommand(t *testing.T) {
	m := setup(t, newFake())
	typ(m, "/filter alert")
	press(m, tea.KeyEnter)
	v := m.View()
	if strings.Contains(v, "dev line") || !strings.Contains(v, "ops alert") {
		t.Fatalf("filter:\n%s", v)
	}
}

func TestHistoryPagerAndSearch(t *testing.T) {
	m := setup(t, newFake())
	typ(m, "/history")
	press(m, tea.KeyEnter)
	if m.mode != modeHistory || !strings.Contains(m.View(), "3 of 3 messages") {
		t.Fatalf("history:\n%s", m.View())
	}
	typ(m, "alan")
	v := m.View()
	if !strings.Contains(v, "1 of 3 messages") || strings.Contains(v, "ops alert") {
		t.Fatalf("search:\n%s", v)
	}
	press(m, tea.KeyEsc)
	if m.mode != modeFeed {
		t.Fatal("Esc should close history")
	}
}

func TestHistoryOfNamedChat(t *testing.T) {
	m := setup(t, newFake())
	typ(m, "/open dev")
	press(m, tea.KeyEnter)
	if !strings.Contains(m.View(), "history: #dev - 2 of 2") {
		t.Fatalf("\n%s", m.View())
	}
}

func TestChatListSelect(t *testing.T) {
	m := setup(t, newFake())
	press(m, tea.KeyCtrlL)
	typ(m, "ops")
	if !strings.Contains(m.View(), "#ops") {
		t.Fatal("list should show ops")
	}
	press(m, tea.KeyDown)
	press(m, tea.KeyEnter)
	if m.sel.Alias != "ops" || m.mode != modeFeed {
		t.Fatalf("sel=%+v mode=%v", m.sel, m.mode)
	}
}

func TestUnknownAndPlainInputReportErrors(t *testing.T) {
	m := setup(t, newFake())
	typ(m, "/nope")
	press(m, tea.KeyEnter)
	if !m.isErr || !strings.Contains(m.status, "unknown command") {
		t.Fatalf("status=%q", m.status)
	}
	typ(m, "hello")
	press(m, tea.KeyEnter)
	if !strings.Contains(m.status, "not a command") {
		t.Fatalf("status=%q", m.status)
	}
	typ(m, "/select zzz")
	press(m, tea.KeyEnter)
	if !m.isErr {
		t.Fatal("unknown chat should be an error")
	}
}

func TestQuit(t *testing.T) {
	m := setup(t, newFake())
	typ(m, "/quit")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil || !m.Quitting() {
		t.Fatal("expected quit")
	}
}

func TestHelpView(t *testing.T) {
	m := setup(t, newFake())
	press(m, tea.KeyF1)
	if !strings.Contains(m.View(), "/history") {
		t.Fatal("help should list commands")
	}
}

func TestLiveProviderUnavailableIsFriendly(t *testing.T) {
	f := &fake{err: provider.ErrNotImplemented}
	m := setup(t, f)
	if !m.isErr || !strings.Contains(m.status, "not available") {
		t.Fatalf("status=%q", m.status)
	}
}

func TestScrollAnchorsWhenNewMessagesArrive(t *testing.T) {
	f := newFake()
	m := setup(t, f)
	for i := 0; i < 30; i++ {
		f.msgs = append(f.msgs, domain.Message{ID: string(rune('a' + i)), ChatID: "c1", Sender: "x", Text: "filler", Time: time.Now().Add(-30*time.Minute + time.Duration(i)*time.Second)})
	}
	m.Update(m.syncCmd()())
	m.View()
	press(m, tea.KeyPgUp)
	m.View()
	before := m.scroll
	f.msgs = []domain.Message{{ID: "new", ChatID: "c2", Sender: "y", Text: "late", Time: time.Now()}}
	m.Update(m.syncCmd()())
	if m.scroll <= before {
		t.Fatalf("scroll should grow to stay anchored: %d -> %d", before, m.scroll)
	}
}

func TestSnapshot(t *testing.T) {
	st, _ := store.Open("")
	out := Snapshot(Options{Store: st, Provider: newFake()}, 80, 12)
	if !strings.Contains(out, "ops alert") {
		t.Fatal(out)
	}
	if n := strings.Count(out, "\n"); n != 11 {
		t.Fatalf("screen should be 12 rows, got %d newlines", n)
	}
}
