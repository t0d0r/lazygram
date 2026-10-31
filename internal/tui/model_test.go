package tui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lazygram/internal/domain"
	"lazygram/internal/provider"
	"lazygram/internal/store"
)

type fake struct {
	chats  []domain.Chat
	msgs   []domain.Message            // returned by Sync
	recent map[string][]domain.Message // returned by Recent
	err    error
	calls  []string // Recent calls as "chat:limit"
}

func (f *fake) Name() string                                 { return "fake" }
func (f *fake) Chats(context.Context) ([]domain.Chat, error) { return f.chats, f.err }
func (f *fake) Sync(context.Context, time.Time) ([]domain.Message, error) {
	return f.msgs, f.err
}
func (f *fake) Recent(_ context.Context, chatID string, limit int) ([]domain.Message, error) {
	f.calls = append(f.calls, chatID+":"+itoa(limit))
	return domain.Tail(f.recent[chatID], limit), f.err
}

func itoa(n int) string { return fmt.Sprintf("%02d", n) }

var base = time.Now().Add(-time.Hour).Truncate(time.Second)

func mk(chat, id, who, text string, min int) domain.Message {
	return domain.Message{ID: id, ChatID: chat, Sender: who, Text: text, Time: base.Add(time.Duration(min) * time.Minute)}
}

func newFake() *fake {
	return &fake{
		chats: []domain.Chat{
			{ID: "c1", Alias: "dev", Title: "Dev", Kind: domain.ChatGroup},
			{ID: "c2", Alias: "alice", Title: "Alice Smith", Kind: domain.ChatUser},
			{ID: "c3", Alias: "news", Title: "News", Kind: domain.ChatChannel, Unread: 4},
			{ID: "c4", Alias: "deleted-4", Title: "Deleted Account", Kind: domain.ChatUser, Deleted: true},
		},
		msgs: []domain.Message{
			mk("c1", "1", "ada", "first dev line", 0),
			mk("c2", "1", "alice", "hi there", 1),
			mk("c1", "2", "alan", "second dev line", 2),
		},
		recent: map[string][]domain.Message{
			"c2": {mk("c2", "0", "alice", "older hello", -30), mk("c2", "1", "alice", "hi there", 1)},
			"c3": {mk("c3", "7", "wire", "headline", -10)},
		},
	}
}

func setup(t *testing.T, f *fake) *Model {
	t.Helper()
	st, err := store.Open(store.Memory, bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
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

func press(m *Model, t tea.KeyType) tea.Cmd {
	_, cmd := m.Update(tea.KeyMsg{Type: t})
	return cmd
}

func enter(m *Model, line string) tea.Cmd {
	typ(m, line)
	return press(m, tea.KeyEnter)
}

func TestServerWindowInterleavesChats(t *testing.T) {
	v := setup(t, newFake()).View()
	a, b, c := strings.Index(v, "first dev line"), strings.Index(v, "hi there"), strings.Index(v, "second dev line")
	if a < 0 || a >= b || b >= c {
		t.Fatalf("expected interleaved chronological order:\n%s", v)
	}
	for _, want := range []string{"#dev", "@alice", "<ada>", "[fake]", "[1:server]", "3 msgs"} {
		if !strings.Contains(v, want) {
			t.Errorf("view lacks %q", want)
		}
	}
}

func TestWindowsOpenForNewMessagesAndUnreadChats(t *testing.T) {
	m := setup(t, newFake())
	if len(m.wins) != 4 { // server, #news (unread at start), #dev, @alice
		t.Fatalf("windows: %d", len(m.wins))
	}
	if m.wins[1].chatID != "c3" || m.wins[1].unread != 4 {
		t.Fatalf("unread chat should open first with its count: %+v", m.wins[1])
	}
	if m.wins[2].unread != 2 || m.wins[3].unread != 1 {
		t.Fatalf("activity counts: %+v %+v", m.wins[2], m.wins[3])
	}
	if !strings.Contains(m.View(), "[Act: 2,3,4]") {
		t.Fatalf("activity missing:\n%s", m.View())
	}
	for _, w := range m.wins {
		if w.chatID == "c4" {
			t.Fatal("deleted accounts must not open windows")
		}
	}
}

func TestWinByNumberShowsOnlyThatChatAndClearsUnread(t *testing.T) {
	m := setup(t, newFake())
	cmd := enter(m, "/win 3")
	if m.cur != 2 || m.wins[2].unread != 0 || cmd == nil {
		t.Fatalf("cur=%d unread=%d cmd=%v", m.cur, m.wins[2].unread, cmd)
	}
	v := m.View()
	if strings.Contains(v, "hi there") || !strings.Contains(v, "first dev line") || !strings.Contains(v, "[3:#dev]") {
		t.Fatalf("window 3 should show only #dev:\n%s", v)
	}
	if strings.Contains(v, "Act: 2,3") || !strings.Contains(v, "Act: 2,4") {
		t.Fatalf("activity after switch:\n%s", v)
	}
	enter(m, "/win 9")
	if !m.isErr {
		t.Fatal("bad window number must be an error")
	}
}

func TestWinByNameFetchesRecentOnce(t *testing.T) {
	f := newFake()
	m := setup(t, f)
	cmd := enter(m, "/win @alice")
	if cmd == nil || m.winName(m.cur) != "@alice" {
		t.Fatalf("cmd=%v name=%s", cmd, m.winName(m.cur))
	}
	m.Update(cmd())
	if len(f.calls) != 1 || f.calls[0] != "c2:10" {
		t.Fatalf("recent calls: %v", f.calls)
	}
	v := m.View()
	if !strings.Contains(v, "older hello") || !strings.Contains(v, "fetched 1 older") {
		t.Fatalf("fetched history missing:\n%s", v)
	}
	if cmd := enter(m, "/win 1"); cmd != nil {
		t.Fatal("server window fetches nothing")
	}
	if cmd := enter(m, "/win @alice"); cmd != nil {
		t.Fatal("second open must not fetch again")
	}
	enter(m, "/win #nope")
	if !m.isErr {
		t.Fatal("unknown chat must be an error")
	}
}

func TestLastFetchesN(t *testing.T) {
	f := newFake()
	m := setup(t, f)
	m.Update(enter(m, "/win @alice")())
	cmd := enter(m, "/last 5")
	if cmd == nil {
		t.Fatal("expected a fetch")
	}
	cmd()
	if f.calls[len(f.calls)-1] != "c2:05" {
		t.Fatalf("calls: %v", f.calls)
	}
	enter(m, "/win 1")
	enter(m, "/last")
	if !m.isErr {
		t.Fatal("/last in the server window must be an error")
	}
}

func TestTabCompletesNamesAndCommands(t *testing.T) {
	m := setup(t, newFake())
	typ(m, "/win @al")
	press(m, tea.KeyTab)
	if got := string(m.input); got != "/win @alice " {
		t.Fatalf("got %q", got)
	}
	m.input = nil
	typ(m, "/win ne")
	press(m, tea.KeyTab)
	if got := string(m.input); got != "/win #news " {
		t.Fatalf("bare prefix should get its sigil: %q", got)
	}
	m.input = nil
	typ(m, "/li")
	press(m, tea.KeyTab)
	if got := string(m.input); got != "/list " {
		t.Fatalf("command: %q", got)
	}
	m.input = nil
	typ(m, "/win d")
	press(m, tea.KeyTab)
	first := string(m.input)
	press(m, tea.KeyTab)
	second := string(m.input)
	if first == second || !strings.HasPrefix(first, "/win #") && !strings.HasPrefix(first, "/win @") {
		t.Fatalf("cycling: %q then %q", first, second)
	}
	if strings.Contains(first+second, "deleted") {
		t.Fatal("deleted accounts must not be completed")
	}
}

func TestListIsSelectableAndHidesDeleted(t *testing.T) {
	m := setup(t, newFake())
	enter(m, "/list")
	v := m.View()
	for _, want := range []string{"@alice", "#dev", "#news", "4 unread"} {
		if !strings.Contains(v, want) {
			t.Errorf("list lacks %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "deleted") {
		t.Fatal("deleted accounts shown by default")
	}
	press(m, tea.KeyDown)
	cmd := press(m, tea.KeyEnter)
	if m.mode != modeWin || m.winName(m.cur) != "@alice" || cmd == nil {
		t.Fatalf("Enter should open the selected chat: mode=%v name=%s", m.mode, m.winName(m.cur))
	}
	enter(m, "/list --show-deleted")
	if !strings.Contains(m.View(), "(deleted)") {
		t.Fatal("--show-deleted")
	}
	press(m, tea.KeyEsc)
	enter(m, "/list ali")
	if v := m.View(); strings.Contains(v, "#dev") || !strings.Contains(v, "@alice") {
		t.Fatalf("filtered list:\n%s", v)
	}
}

func TestCloseWindow(t *testing.T) {
	m := setup(t, newFake())
	enter(m, "/win 3")
	enter(m, "/close")
	if len(m.wins) != 3 || m.cur != 1 {
		t.Fatalf("wins=%d cur=%d", len(m.wins), m.cur)
	}
	enter(m, "/close 1")
	if !m.isErr || len(m.wins) != 3 {
		t.Fatal("server window cannot be closed")
	}
}

func TestWinListsWindows(t *testing.T) {
	m := setup(t, newFake())
	enter(m, "/win")
	v := m.View()
	if !strings.Contains(v, "1: server") || !strings.Contains(v, "#news") || !strings.Contains(v, "4 unread") {
		t.Fatalf("\n%s", v)
	}
}

func TestSearchPagerOverCurrentChat(t *testing.T) {
	m := setup(t, newFake())
	enter(m, "/win 3")
	enter(m, "/search second")
	v := m.View()
	if m.mode != modePager || !strings.Contains(v, "1 of 2 messages") || strings.Contains(v, "first dev") {
		t.Fatalf("\n%s", v)
	}
	press(m, tea.KeyEsc)
	if m.mode != modeWin {
		t.Fatal("Esc should close the pager")
	}
}

func TestCtrlNAndAltDigitSwitch(t *testing.T) {
	m := setup(t, newFake())
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
	if m.cur != 1 {
		t.Fatalf("ctrl-n: %d", m.cur)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}, Alt: true})
	if m.cur != 2 {
		t.Fatalf("alt-3: %d", m.cur)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	if m.cur != 0 {
		t.Fatalf("ctrl-p twice: %d", m.cur)
	}
}

func TestUnknownAndPlainInputReportErrors(t *testing.T) {
	m := setup(t, newFake())
	enter(m, "/nope")
	if !m.isErr || !strings.Contains(m.status, "unknown command") {
		t.Fatalf("status=%q", m.status)
	}
	enter(m, "hello")
	if !strings.Contains(m.status, "not a command") {
		t.Fatalf("status=%q", m.status)
	}
}

func TestQuitAndHelp(t *testing.T) {
	m := setup(t, newFake())
	press(m, tea.KeyF1)
	if !strings.Contains(m.View(), "/win") {
		t.Fatal("help should list commands")
	}
	press(m, tea.KeyEsc)
	typ(m, "/quit")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil || !m.Quitting() {
		t.Fatal("expected quit")
	}
}

func TestProviderErrorsLandInServerWindow(t *testing.T) {
	m := setup(t, &fake{err: provider.ErrNotImplemented})
	if !m.isErr || !strings.Contains(m.View(), "-!- sync: live connector not available") {
		t.Fatalf("status=%q\n%s", m.status, m.View())
	}
}

func TestScrollAnchorsWhenNewMessagesArrive(t *testing.T) {
	f := newFake()
	m := setup(t, f)
	for i := 0; i < 30; i++ {
		f.msgs = append(f.msgs, domain.Message{ID: "f" + itoa(i), ChatID: "c1", Sender: "x", Text: "filler", Time: base.Add(time.Duration(i) * time.Second)})
	}
	m.Update(m.syncCmd()())
	m.View()
	press(m, tea.KeyPgUp)
	m.View()
	before := m.wins[0].scroll
	f.msgs = []domain.Message{{ID: "new", ChatID: "c2", Sender: "y", Text: "late", Time: time.Now()}}
	m.Update(m.syncCmd()())
	if m.wins[0].scroll <= before {
		t.Fatalf("scroll should grow to stay anchored: %d -> %d", before, m.wins[0].scroll)
	}
}

func TestSnapshot(t *testing.T) {
	st, _ := store.Open(store.Memory, bytes.Repeat([]byte{2}, 32))
	defer func() { _ = st.Close() }()
	out := Snapshot(Options{Store: st, Provider: newFake()}, 80, 12)
	if !strings.Contains(out, "hi there") {
		t.Fatal(out)
	}
	if n := strings.Count(out, "\n"); n != 11 {
		t.Fatalf("screen should be 12 rows, got %d newlines", n)
	}
}

func TestBackspaceVariants(t *testing.T) {
	m := setup(t, newFake())
	typ(m, "/win")
	for _, k := range []tea.KeyType{tea.KeyBackspace, tea.KeyCtrlH, tea.KeyDelete} {
		press(m, k)
	}
	if got := string(m.input); got != "/" {
		t.Fatalf("got %q", got)
	}
	press(m, tea.KeyBackspace)
	press(m, tea.KeyBackspace) // empty input stays empty
	if len(m.input) != 0 {
		t.Fatal("empty")
	}
}

type partialFake struct{ fake }

func (p *partialFake) Sync(context.Context, time.Time) ([]domain.Message, error) {
	return p.msgs, errors.New("boom after two chats")
}

func TestPartialSyncResultsAreKept(t *testing.T) {
	f := &partialFake{*newFake()}
	m := setup(t, &f.fake)
	m.Update(m.syncCmd()()) // plain fake: baseline
	pf := &partialFake{*newFake()}
	pf.msgs = []domain.Message{mk("c1", "9", "ada", "kept despite error", 5)}
	m.opt.Provider = pf
	m.Update(m.syncCmd()())
	if !m.isErr || !strings.Contains(m.status, "boom") {
		t.Fatalf("error must be reported: %q", m.status)
	}
	if n, _ := m.opt.Store.Count("c1"); n != 3 {
		t.Fatalf("partial batch must be stored, count=%d", n)
	}
	if !strings.Contains(m.View(), "kept despite error") {
		t.Fatal("partial batch must be shown")
	}
}
