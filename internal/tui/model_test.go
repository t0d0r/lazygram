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
	chats   []domain.Chat
	msgs    []domain.Message            // returned by Sync
	recent  map[string][]domain.Message // returned by Recent
	err     error
	calls   []string // Recent calls as "chat:limit"
	sent    []string // Send calls as "chat:text"
	pressed []string // Press calls as "chat:msg:label"
	fetched []string // Fetch calls as "chat:msg"
}

func (f *fake) Send(_ context.Context, chatID, text string) error {
	f.sent = append(f.sent, chatID+":"+text)
	return f.err
}

func (f *fake) Fetch(_ context.Context, chatID, msgID string) (string, error) {
	f.fetched = append(f.fetched, chatID+":"+msgID)
	if f.err != nil {
		return "", f.err
	}
	return "/tmp/fake/" + msgID + ".jpg", nil
}

func (f *fake) Press(_ context.Context, chatID, msgID string, b domain.Button) (domain.PressAnswer, error) {
	f.pressed = append(f.pressed, chatID+":"+msgID+":"+b.Label)
	if f.err != nil {
		return domain.PressAnswer{}, f.err
	}
	edited := domain.Message{ID: msgID, ChatID: chatID, Sender: "bot", Text: "you chose " + b.Label, Time: base.Add(3 * time.Minute)}
	return domain.PressAnswer{Text: "ack " + b.Label, Updated: &edited}, nil
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
			withBtns(mk("c2", "7", "bot", "deploy?", 3)),
		},
		recent: map[string][]domain.Message{
			"c2": {mk("c2", "0", "alice", "older hello", -30), mk("c2", "1", "alice", "hi there", 1)},
			"c3": {mk("c3", "7", "wire", "headline", -10)},
		},
	}
}

func withBtns(m domain.Message) domain.Message {
	m.Buttons = []domain.Button{
		{Label: "Yes", Kind: domain.ButtonCallback, Data: []byte("y")},
		{Label: "Docs", Kind: domain.ButtonURL, URL: "https://example.invalid/d"},
		{Label: "Ping", Kind: domain.ButtonText},
		{Label: "Pay", Kind: domain.ButtonOther},
	}
	return m
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
	for _, want := range []string{"#dev", "@alice", "<ada>", "[fake]", "[1:server]", "4 msgs"} {
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
	if m.wins[2].unread != 2 || m.wins[3].unread != 2 {
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

// body returns the content rows of the screen: no topic, status, window
// bar or input line.
func body(m *Model) string {
	lines := strings.Split(m.View(), "\n")
	return strings.Join(lines[1:len(lines)-3], "\n")
}

func TestListIsSelectableAndHidesDeleted(t *testing.T) {
	m := setup(t, newFake())
	enter(m, "/list")
	v := body(m)
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
	if !strings.Contains(body(m), "(deleted)") {
		t.Fatal("--show-deleted")
	}
	press(m, tea.KeyEsc)
	enter(m, "/list ali")
	if v := body(m); strings.Contains(v, "#dev") || !strings.Contains(v, "@alice") {
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
	v := m.View()
	if m.mode != modeWin || !strings.Contains(v, "Ctrl-X") || !strings.Contains(v, "People are @name") {
		t.Fatalf("help should print into the window:\n%s", v)
	}
	if !strings.Contains(strings.Join(m.lines(), "\n"), "/win N                 switch") {
		t.Fatal("help columns must keep their alignment")
	}
	enter(m, "/win 2")
	enter(m, "/help")
	if !strings.Contains(m.View(), "People are @name") {
		t.Fatal("/help should print into the current window")
	}
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
	out := Snapshot(Options{Store: st, Provider: newFake()}, 80, 24)
	if !strings.Contains(out, "hi there") {
		t.Fatal(out)
	}
	if n := strings.Count(out, "\n"); n != 23 {
		t.Fatalf("screen should be 24 rows, got %d newlines", n)
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

func TestCtrlXSendsInChatWindowOnly(t *testing.T) {
	f := newFake()
	m := setup(t, f)
	typ(m, "hello there")
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	if !m.isErr || len(f.sent) != 0 {
		t.Fatalf("server window must not send: %q %v", m.status, f.sent)
	}
	m.input = nil
	enter(m, "/win @alice")
	typ(m, "hello there")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	if cmd == nil || len(m.input) != 0 {
		t.Fatal("expected a send command and a cleared input")
	}
	_, next := m.Update(cmd())
	if len(f.sent) != 1 || f.sent[0] != "c2:hello there" || m.isErr || next == nil {
		t.Fatalf("sent=%v status=%q", f.sent, m.status)
	}
	typ(m, "/quit")
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	if !m.isErr || len(f.sent) != 1 {
		t.Fatal("command-like text must not be sent by Ctrl-X")
	}
	m.input = nil
	typ(m, "plain text")
	press(m, tea.KeyEnter)
	if !strings.Contains(m.status, "Ctrl-X") || len(m.input) == 0 {
		t.Fatalf("Enter on text in a chat window should hint at Ctrl-X and keep the text: %q", m.status)
	}
}

func TestWinListIsSelectable(t *testing.T) {
	m := setup(t, newFake())
	enter(m, "/win")
	v := m.View()
	if m.mode != modeList || m.listSrc != listWindows || !strings.Contains(v, "1  server") || !strings.Contains(v, "#news") || !strings.Contains(v, "(current)") {
		t.Fatalf("\n%s", v)
	}
	press(m, tea.KeyDown)
	press(m, tea.KeyDown)
	cmd := press(m, tea.KeyEnter)
	if m.mode != modeWin || m.cur != 2 || cmd == nil {
		t.Fatalf("Enter should focus window 3: mode=%v cur=%d", m.mode, m.cur)
	}
	enter(m, "/win list")
	if m.mode != modeList || m.listCur != 2 {
		t.Fatalf("/win list should open on the current window: %v %d", m.mode, m.listCur)
	}
}

func TestStatsWhoisMe(t *testing.T) {
	f := newFake()
	m := setup(t, f)
	enter(m, "/stats")
	v := m.View()
	for _, want := range []string{"stats: fake", "chats: 4 (1 people, 1 groups, 1 channels, 1 deleted)", "windows: 4 open", "4 in this session, 4 cached"} {
		if !strings.Contains(v, want) {
			t.Errorf("stats lacks %q:\n%s", want, v)
		}
	}
	enter(m, "/whois")
	if !m.isErr {
		t.Fatal("whois without a name in the server window")
	}
	enter(m, "/whois @alice")
	v = m.View()
	for _, want := range []string{"whois @alice: Alice Smith", "person on", "id c2", "2 messages this session"} {
		if !strings.Contains(v, want) {
			t.Errorf("whois lacks %q:\n%s", want, v)
		}
	}
	enter(m, "/me waves")
	if !m.isErr || len(f.sent) != 0 {
		t.Fatal("/me in the server window must fail")
	}
	enter(m, "/win @alice")
	enter(m, "/whois")
	if !strings.Contains(m.View(), "whois @alice") {
		t.Fatal("whois defaults to the window's chat")
	}
	cmd := enter(m, "/me waves")
	if cmd == nil {
		t.Fatal("expected a send")
	}
	cmd()
	if len(f.sent) != 1 || f.sent[0] != "c2:* waves" {
		t.Fatalf("sent=%v", f.sent)
	}
	enter(m, "/me")
	if !m.isErr {
		t.Fatal("/me needs text")
	}
}

func TestListEnterPicksTheFilteredSelection(t *testing.T) {
	m := setup(t, newFake())
	enter(m, "/list ali") // only @alice matches; she is not first in the full list
	cmd := press(m, tea.KeyEnter)
	if m.winName(m.cur) != "@alice" || cmd == nil {
		t.Fatalf("picked %s", m.winName(m.cur))
	}
	enter(m, "/list")
	typ(m, "ne") // #news
	press(m, tea.KeyEnter)
	if m.winName(m.cur) != "#news" {
		t.Fatalf("picked %s", m.winName(m.cur))
	}
}

func TestWindowBarListsEveryWindow(t *testing.T) {
	m := setup(t, newFake())
	v := m.View()
	for _, want := range []string{"[1:server]", "[2:#news]", "[3:#dev]", "[4:@alice]"} {
		if !strings.Contains(v, want) {
			t.Fatalf("window bar lacks %q:\n%s", want, v)
		}
	}
	lines := strings.Split(v, "\n")
	if len(lines) != 20 || !strings.Contains(lines[18], "[1:server]") {
		t.Fatalf("bar must be the line above the input: %d lines", len(lines))
	}
}

func TestEscThenDigitSwitchesWindows(t *testing.T) {
	m := setup(t, newFake())
	typ(m, "half typed")
	press(m, tea.KeyEsc)
	typ(m, "3")
	if m.cur != 2 || string(m.input) != "half typed" {
		t.Fatalf("cur=%d input=%q", m.cur, string(m.input))
	}
	press(m, tea.KeyEsc) // a lone Esc still clears
	typ(m, "x")
	if m.cur != 2 || string(m.input) != "x" {
		t.Fatalf("cur=%d input=%q", m.cur, string(m.input))
	}
	enter(m, "/list")
	press(m, tea.KeyEsc)
	typ(m, "1")
	if m.mode != modeWin || m.cur != 0 {
		t.Fatalf("Esc closes the list and the digit switches: mode=%v cur=%d", m.mode, m.cur)
	}
	press(m, tea.KeyEsc)
	typ(m, "9")
	if !m.isErr || m.cur != 0 {
		t.Fatal("no window 9")
	}
}

func TestWindowLayoutSurvivesRestart(t *testing.T) {
	st, err := store.Open(store.Memory, bytes.Repeat([]byte{3}, 32))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	f := newFake()
	m := New(Options{Store: st, Provider: f})
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	m.Update(m.chatsCmd()())
	m.Update(m.syncCmd()()) // opens #news, #dev, @alice
	enter(m, "/close 3")    // #dev
	enter(m, "/win @alice")
	if len(m.wins) != 3 || m.cur != 2 {
		t.Fatalf("wins=%d cur=%d", len(m.wins), m.cur)
	}

	again := New(Options{Store: st, Provider: f})
	if len(again.wins) != 3 || again.wins[1].chatID != "c3" || again.wins[2].chatID != "c2" || again.cur != 2 {
		t.Fatalf("restored wins=%d cur=%d", len(again.wins), again.cur)
	}
	if n := again.wins[0].notes; len(n) == 0 || !strings.Contains(n[len(n)-1].text, "2 windows reopened") {
		t.Fatal("restore notice in the server window")
	}
	again.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	if cmd := again.Init(); cmd == nil {
		t.Fatal("Init should fetch the restored current window")
	}
	again.Update(again.chatsCmd()()) // unread #news already has a window: no duplicate
	if len(again.wins) != 3 {
		t.Fatalf("duplicate windows after chats load: %d", len(again.wins))
	}
}

func TestStartBannerAndTagline(t *testing.T) {
	st, _ := store.Open(store.Memory, bytes.Repeat([]byte{4}, 32))
	defer func() { _ = st.Close() }()
	f := newFake()
	m := New(Options{Store: st, Provider: f, Version: "v9.9.9"})
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m.Update(m.syncCmd()()) // hour-old messages must still come after the banner
	press(m, tea.KeyHome)
	v := m.View()
	for _, want := range []string{"/_/\\__,_/", "-!- lazygram v9.9.9"} {
		if !strings.Contains(v, want) {
			t.Fatalf("banner lacks %q:\n%s", want, v)
		}
	}
	first := -1
	for i, l := range strings.Split(v, "\n") {
		if strings.Contains(l, "/ /___") {
			first = i
			if strings.Contains(l, "-!-") {
				t.Fatalf("banner line carries the notice marker: %q", l)
			}
		}
	}
	if first < 1 {
		t.Fatal("banner line not found")
	}
	if a, b := strings.Index(v, "/_/\\__,_/"), strings.Index(v, "first dev line"); a < 0 || b < 0 || a > b {
		t.Fatal("banner must precede the messages")
	}
	if out := ExitBanner("v9.9.9"); !strings.Contains(out, "lazygram v9.9.9") || !strings.Contains(out, "bye") || strings.Count(out, "\n") < 4 {
		t.Fatalf("exit banner:\n%s", out)
	}
	if Tagline("") != Tagline("dev") {
		t.Fatal("empty version reads dev")
	}
}

func TestPressCallbackURLTextAndErrors(t *testing.T) {
	f := newFake()
	m := setup(t, f)
	enter(m, "/press 1")
	if !m.isErr || !strings.Contains(m.status, "server window") {
		t.Fatalf("server window: %q", m.status)
	}
	enter(m, "/win #dev")
	enter(m, "/press 1")
	if !m.isErr || !strings.Contains(m.status, "no message with buttons") {
		t.Fatalf("no buttons: %q", m.status)
	}
	enter(m, "/win @alice")
	if v := body(m); !strings.Contains(v, "[1] Yes  [2] Docs (link)  [3] Ping (reply)  [4] Pay (unsupported)  (msg 7)") {
		t.Fatalf("button line:\n%s", v)
	}
	enter(m, "/press 9")
	if !m.isErr || !strings.Contains(m.status, "pick 1..4") {
		t.Fatalf("range: %q", m.status)
	}
	enter(m, "/press 2") // url: printed, nothing sent
	if v := body(m); !strings.Contains(v, "-!- Docs: open https://example.invalid/d") || len(f.pressed) != 0 {
		t.Fatalf("url button:\n%s", v)
	}
	cmd := enter(m, "/press 3") // reply keyboard: sends the label
	if cmd == nil {
		t.Fatal("text button must send")
	}
	cmd()
	if len(f.sent) != 1 || f.sent[0] != "c2:Ping" {
		t.Fatalf("sent=%v", f.sent)
	}
	enter(m, "/press 4")
	if !m.isErr {
		t.Fatal("unsupported kind")
	}
	cmd = enter(m, "/press 1") // callback: answer shown, message replaced
	if cmd == nil {
		t.Fatal("callback must press")
	}
	m.Update(cmd())
	if len(f.pressed) != 1 || f.pressed[0] != "c2:7:Yes" {
		t.Fatalf("pressed=%v", f.pressed)
	}
	v := body(m)
	if !strings.Contains(v, "-!- pressed Yes: ack Yes") || !strings.Contains(v, "you chose Yes") || strings.Contains(v, "[1] Yes") {
		t.Fatalf("after press:\n%s", v)
	}
	if got, _ := m.opt.Store.Messages("c2", 0); len(got) != 2 || got[1].Text != "you chose Yes" {
		t.Fatalf("store must hold the edited message: %+v", got)
	}
	cmd = enter(m, "/press 7 1") // explicit message id, now without buttons
	if !m.isErr || cmd != nil {
		t.Fatalf("buttons gone: %q", m.status)
	}
	enter(m, "/press 1 2 3")
	if !m.isErr || !strings.Contains(m.status, "usage") {
		t.Fatalf("usage: %q", m.status)
	}
}

func TestPressErrorLandsInWindow(t *testing.T) {
	f := newFake()
	m := setup(t, f)
	enter(m, "/win @alice")
	f.err = errors.New("BOT_RESPONSE_TIMEOUT")
	cmd := enter(m, "/press 1")
	m.Update(cmd())
	if !m.isErr || !strings.Contains(body(m), "-!- press Yes: BOT_RESPONSE_TIMEOUT") {
		t.Fatalf("%q\n%s", m.status, body(m))
	}
}

func TestDoubleCtrlXTogglesEnterSends(t *testing.T) {
	f := newFake()
	m := setup(t, f)
	enter(m, "/win @alice")
	press(m, tea.KeyCtrlX)
	press(m, tea.KeyCtrlX) // fast: toggle on
	if !m.wins[m.cur].enterSends || !strings.Contains(m.View(), "[@alice]> ") {
		t.Fatalf("toggle on: %v %q", m.wins[m.cur].enterSends, m.status)
	}
	cmd := enter(m, "hello via enter")
	if cmd == nil || len(f.sent) != 0 {
		t.Fatal("Enter must send now")
	}
	cmd()
	if len(f.sent) != 1 || f.sent[0] != "c2:hello via enter" {
		t.Fatalf("sent=%v", f.sent)
	}
	enter(m, "/win 1")
	if m.wins[0].enterSends {
		t.Fatal("the toggle is per window")
	}
	enter(m, "/win @alice")
	enter(m, "/stats") // commands still run while Enter sends
	if !strings.Contains(body(m), "stats: fake") {
		t.Fatal("/stats should still run")
	}
	m.lastCtrlX = time.Now().Add(-2 * time.Second)
	press(m, tea.KeyCtrlX) // slow: no toggle (empty input, nothing sent)
	if !m.wins[m.cur].enterSends {
		t.Fatal("a slow Ctrl-X must not toggle")
	}
	press(m, tea.KeyCtrlX) // fast after the previous: toggle off
	if m.wins[m.cur].enterSends {
		t.Fatal("toggle off")
	}
}

func TestTabInListOnlyCompletes(t *testing.T) {
	m := setup(t, newFake())
	enter(m, "/list")
	press(m, tea.KeyDown)
	press(m, tea.KeyTab) // empty input: completes to the first name, never moves the cursor
	if m.listCur != 1 || string(m.input) != "#dev" {
		t.Fatalf("cur=%d input=%q", m.listCur, string(m.input))
	}
	m.input, m.comp = nil, nil
	typ(m, "al")
	press(m, tea.KeyTab)
	if got := string(m.input); got != "@alice " {
		t.Fatalf("Tab completes the filter: %q", got)
	}
}

func click(m *Model, x, y int) tea.Cmd {
	_, cmd := m.Update(tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
	return cmd
}

// find returns the screen row and column of the first span of kind k whose
// target is t, as of the last View.
func find(m *Model, k spanKind, t string) (int, int) {
	for y, row := range m.clicks {
		for _, s := range row {
			if s.kind == k && s.target == t {
				return s.x0, y
			}
		}
	}
	return -1, -1
}

func TestMouseClicksOpenWindowsAndLinks(t *testing.T) {
	f := newFake()
	f.msgs = append(f.msgs, mk("c1", "9", "alice", "read https://example.invalid/doc now", 4))
	var opened []string
	st, _ := store.Open(store.Memory, bytes.Repeat([]byte{5}, 32))
	defer func() { _ = st.Close() }()
	m := New(Options{Store: st, Provider: f, Open: func(s string) error { opened = append(opened, s); return nil }})
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m.Update(m.chatsCmd()())
	m.Update(m.syncCmd()())
	m.View()

	// chat column in the server window opens that chat's window
	x, y := find(m, spanChat, "c1")
	if x < 0 {
		t.Fatal("no chat span")
	}
	click(m, x, y)
	if m.winName(m.cur) != "#dev" {
		t.Fatalf("click on #dev: %s", m.winName(m.cur))
	}
	// the window bar focuses
	m.View()
	x, y = find(m, spanWin, "0")
	click(m, x, y)
	if m.cur != 0 {
		t.Fatalf("window bar: cur=%d", m.cur)
	}
	// a nick that names a chat opens it
	m.View()
	x, y = find(m, spanNick, "alice")
	click(m, x+2, y)
	if m.winName(m.cur) != "@alice" {
		t.Fatalf("nick click: %s", m.winName(m.cur))
	}
	// a nick without a chat is an error, not a crash
	enter(m, "/win 1")
	m.View()
	x, y = find(m, spanNick, "ada")
	click(m, x, y)
	if !m.isErr || !strings.Contains(m.status, "no chat named ada") {
		t.Fatalf("unknown nick: %q", m.status)
	}
	// a link is handed to the opener
	m.View()
	x, y = find(m, spanURL, "https://example.invalid/doc")
	cmd := click(m, x, y)
	if cmd == nil {
		t.Fatal("link click must open")
	}
	m.Update(cmd())
	if len(opened) != 1 || opened[0] != "https://example.invalid/doc" || m.isErr {
		t.Fatalf("opened=%v status=%q", opened, m.status)
	}
	// list rows open on click
	enter(m, "/list")
	m.View()
	x, y = find(m, spanRow, "2") // #news
	cmd = click(m, x, y)
	if m.mode != modeWin || m.winName(m.cur) != "#news" {
		t.Fatalf("row click: mode=%v win=%s", m.mode, m.winName(m.cur))
	}
	_ = cmd
	// the wheel scrolls
	m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
	if m.wins[m.cur].scroll != 3 {
		t.Fatalf("wheel: %d", m.wins[m.cur].scroll)
	}
	m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	if m.wins[m.cur].scroll != 0 {
		t.Fatal("wheel down")
	}
	// clicks outside any span do nothing
	click(m, 0, 0)
	click(m, 99, 999)
}

func TestOpenFetchesAndOpensImages(t *testing.T) {
	f := newFake()
	photo := mk("c1", "20", "ada", "look", 5)
	photo.Media = &domain.Media{Kind: "photo"}
	file := mk("c1", "21", "ada", "", 6)
	file.Media = &domain.Media{Kind: "file", Name: "report.pdf"}
	f.msgs = append(f.msgs, photo, file)
	var opened []string
	st, _ := store.Open(store.Memory, bytes.Repeat([]byte{6}, 32))
	defer func() { _ = st.Close() }()
	m := New(Options{Store: st, Provider: f, Open: func(s string) error { opened = append(opened, s); return nil }})
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m.Update(m.chatsCmd()())
	m.Update(m.syncCmd()())
	v := body(m)
	if !strings.Contains(v, "[photo] look") || !strings.Contains(v, "[file: report.pdf]") {
		t.Fatalf("tokens:\n%s", v)
	}
	enter(m, "/open")
	if !m.isErr {
		t.Fatal("server window")
	}
	enter(m, "/win #dev")
	cmd := enter(m, "/open") // latest with media: the file
	if cmd == nil {
		t.Fatal("expected a fetch")
	}
	_, next := m.Update(cmd())
	if f.fetched[0] != "c1:21" || next != nil || !strings.Contains(body(m), "-!- saved /tmp/fake/21.jpg") || len(opened) != 0 {
		t.Fatalf("file: fetched=%v opened=%v\n%s", f.fetched, opened, body(m))
	}
	cmd = enter(m, "/open 20") // the photo: fetched then opened
	_, next = m.Update(cmd())
	if next == nil {
		t.Fatal("images must be opened")
	}
	m.Update(next())
	if len(opened) != 1 || opened[0] != "/tmp/fake/20.jpg" {
		t.Fatalf("opened=%v", opened)
	}
	enter(m, "/open 1") // a message without media
	if !m.isErr {
		t.Fatal("no media")
	}
	// clicking the token does the same
	m.View()
	x, y := find(m, spanMedia, "c1\x0020")
	if x < 0 {
		t.Fatal("no media span")
	}
	cmd = click(m, x, y)
	_, next = m.Update(cmd())
	m.Update(next())
	if len(opened) != 2 {
		t.Fatalf("click: opened=%v", opened)
	}
	f.err = errors.New("FILE_REFERENCE_EXPIRED")
	_, next = m.Update(enter(m, "/open 20")())
	if !m.isErr || !strings.Contains(body(m), "-!- fetch: FILE_REFERENCE_EXPIRED") || next != nil {
		t.Fatal("fetch errors land in the window")
	}
}
