package tui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"lazygram/internal/domain"
	"lazygram/internal/logo"
	"lazygram/internal/provider"
	"lazygram/internal/store"
)

type fake struct {
	chats     []domain.Chat
	msgs      []domain.Message            // returned by Sync
	recent    map[string][]domain.Message // returned by Recent
	err       error
	calls     []string // Recent calls as "chat:limit"
	sent      []string // Send calls as "chat:text"
	pressed   []string // Press calls as "chat:msg:label"
	fetched   []string // Fetch calls as "chat:msg"
	edited    []string // Edit calls as "chat:msg:text"
	deleted   []string // Delete calls as "chat:msg"
	picture   []byte   // what Fetch returns when set
	sentFiles []string // SendFile calls as "chat:name:bytes:caption"
}

func (f *fake) Send(_ context.Context, chatID, text string) error {
	f.sent = append(f.sent, chatID+":"+text)
	return f.err
}

func (f *fake) Fetch(_ context.Context, chatID, msgID string) ([]byte, string, error) {
	f.fetched = append(f.fetched, chatID+":"+msgID)
	if f.err != nil {
		return nil, "", f.err
	}
	if f.picture != nil {
		return f.picture, msgID + ".png", nil
	}
	return []byte("BYTES-" + msgID), msgID + ".jpg", nil
}

func (f *fake) SendFile(_ context.Context, chatID, name string, data []byte, caption string) error {
	f.sentFiles = append(f.sentFiles, fmt.Sprintf("%s:%s:%d:%s", chatID, name, len(data), caption))
	return f.err
}

func (f *fake) Edit(_ context.Context, chatID, msgID, text string) error {
	f.edited = append(f.edited, chatID+":"+msgID+":"+text)
	return f.err
}

func (f *fake) Delete(_ context.Context, chatID, msgID string) error {
	f.deleted = append(f.deleted, chatID+":"+msgID)
	return f.err
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
	m.fx = false // the running lights schedule ticks; tests switch them on explicitly
	m.revealAll()
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
	for _, want := range []string{"#dev", "@alice", "<ada>", "  fake  [0:server]", "4 msgs"} {
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
	if !strings.Contains(m.View(), "Act: 1,2,3") {
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
	cmd := enter(m, "/win 2")
	if m.cur != 2 || m.wins[2].unread != 0 || cmd == nil {
		t.Fatalf("cur=%d unread=%d cmd=%v", m.cur, m.wins[2].unread, cmd)
	}
	v := m.View()
	if strings.Contains(v, "hi there") || !strings.Contains(v, "first dev line") || !strings.Contains(v, "[2:#dev]") || !strings.Contains(v, "ctrl-x: to send") {
		t.Fatalf("window 2 should show only #dev:\n%s", v)
	}
	if strings.Contains(v, "Act: 1,2") || !strings.Contains(v, "Act: 1,3") {
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
	if cmd := enter(m, "/win 0"); cmd != nil {
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
	enter(m, "/win 0")
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
	typ(m, "/lis")
	press(m, tea.KeyTab)
	if got := string(m.input); got != "/list " {
		t.Fatalf("command: %q", got)
	}
	m.input = nil
	typ(m, "/w")
	press(m, tea.KeyTab) // /w, /wc, /whois, /win, /window: common prefix is /w
	if got := string(m.input); got != "/w" {
		t.Fatalf("ambiguous: %q", got)
	}
	press(m, tea.KeyTab) // second Tab lists them
	if v := body(m); !strings.Contains(v, "/whois") || !strings.Contains(v, "/window") {
		t.Fatalf("list of candidates:\n%s", v)
	}
	m.input = nil
	typ(m, "/wh")
	press(m, tea.KeyTab)
	if got := string(m.input); got != "/whois " {
		t.Fatalf("unique: %q", got)
	}
	m.input = nil
	typ(m, "/win #")
	press(m, tea.KeyTab) // #dev, #news share only "#"
	if got := string(m.input); got != "/win #" {
		t.Fatalf("common prefix of names: %q", got)
	}
	press(m, tea.KeyTab)
	if v := body(m); !strings.Contains(v, "#dev  #news") {
		t.Fatalf("names listed:\n%s", v)
	}
	if strings.Contains(body(m), "deleted") {
		t.Fatal("deleted accounts must not be completed")
	}
	if commonPrefix([]string{"/win", "/window", "/wi"}) != "/wi" || commonPrefix(nil) != "" {
		t.Fatal("commonPrefix")
	}
}

// body returns the content rows of the screen: no topic, status, window
// bar or input line.
func body(m *Model) string {
	lines := strings.Split(m.View(), "\n")
	return strings.Join(lines[:len(lines)-4], "\n")
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
	enter(m, "/win 2")
	enter(m, "/close")
	if len(m.wins) != 3 || m.cur != 1 {
		t.Fatalf("wins=%d cur=%d", len(m.wins), m.cur)
	}
	enter(m, "/close 0")
	if !m.isErr || len(m.wins) != 3 {
		t.Fatal("server window cannot be closed")
	}
}

func TestMoveWindow(t *testing.T) {
	m := setup(t, newFake()) // 0 server, 1 #news, 2 #dev, 3 @alice
	enter(m, "/move 2")
	if !m.isErr {
		t.Fatal("the server window cannot move")
	}
	enter(m, "/win 3") // @alice
	enter(m, "/move 1")
	if m.cur != 1 || m.winName(1) != "@alice" || m.winName(2) != "#news" || m.winName(3) != "#dev" {
		t.Fatalf("after move: %s %s %s cur=%d", m.winName(1), m.winName(2), m.winName(3), m.cur)
	}
	enter(m, "/move 3")
	if m.cur != 3 || m.winName(3) != "@alice" || m.winName(1) != "#news" {
		t.Fatalf("move down: %s cur=%d", m.winName(3), m.cur)
	}
	enter(m, "/move 9")
	if !m.isErr {
		t.Fatal("out of range")
	}
	enter(m, "/move x")
	if !m.isErr {
		t.Fatal("not a number")
	}
	var l layout
	if ok, _ := m.opt.Store.State(layoutKey, &l); !ok || l.Windows[2] != "c2" || l.Current != 3 {
		t.Fatalf("layout must follow: %+v", l)
	}
}

func TestSearchPagerOverCurrentChat(t *testing.T) {
	m := setup(t, newFake())
	enter(m, "/win 2")
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
	if m.cur != 3 {
		t.Fatalf("alt-3: %d", m.cur)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'0'}, Alt: true})
	if m.cur != 0 {
		t.Fatalf("alt-0 is the server window: %d", m.cur)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
	if m.cur != 3 {
		t.Fatalf("ctrl-p wraps: %d", m.cur)
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

// drain delivers every queued slow-print line.
func drain(m *Model) {
	for len(m.slow) > 0 {
		m.Update(slowTickMsg{})
	}
}

func TestHelpPrintsSlowlyWithoutIndent(t *testing.T) {
	m := setup(t, newFake())
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyF1})
	if cmd == nil || len(m.slow) == 0 || strings.Contains(body(m), "/quit") {
		t.Fatal("help starts a slow print, nothing shown yet")
	}
	m.Update(slowTickMsg{})
	if n := m.wins[0].notes; !n[len(n)-1].plain || n[len(n)-1].text != "Commands" {
		t.Fatalf("first line: %+v", n[len(n)-1])
	}
	drain(m)
	v := strings.Join(m.lines(), "\n")
	if !strings.Contains(v, "\n  /quit, /q") || !strings.Contains(v, "\nCommands") || !strings.Contains(v, "\nKeys") {
		t.Fatalf("command lines start after two spaces, headers at the margin:\n%s", v)
	}
	if strings.Contains(v, "-!- Commands") {
		t.Fatal("no marker on help lines")
	}
}

func TestQuitAndHelp(t *testing.T) {
	m := setup(t, newFake())
	press(m, tea.KeyF1)
	drain(m)
	v := m.View()
	if m.mode != modeWin || !strings.Contains(v, "People are @name") || !strings.Contains(strings.Join(m.lines(), "\n"), "Ctrl-X") {
		t.Fatalf("help should print into the window:\n%s", v)
	}
	if !strings.Contains(strings.Join(m.lines(), "\n"), "/win N                 switch") {
		t.Fatal("help columns must keep their alignment")
	}
	enter(m, "/win 1")
	enter(m, "/help")
	drain(m)
	if m.cur != 0 || !strings.Contains(m.View(), "People are @name") {
		t.Fatalf("/help from window 1 focuses the server window and prints there (cur=%d)", m.cur)
	}
	for _, n := range m.wins[1].notes {
		if strings.Contains(n.text, "People are @name") {
			t.Fatal("nothing printed into window 1")
		}
	}
	typ(m, "/quit")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil || !m.Quitting() {
		t.Fatal("expected quit")
	}
}

func TestProviderErrorsLandInServerWindow(t *testing.T) {
	m := setup(t, &fake{err: provider.ErrNotImplemented})
	if !m.isErr || !strings.Contains(m.View(), "-!- error: sync: live connector not available") {
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
	_, tick := m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	if tick == nil || len(m.input) != 0 || m.pending == nil {
		t.Fatal("expected the text held back and a timer")
	}
	_, cmd := m.Update(tick()) // the pause passed: now it is sent
	if cmd == nil {
		t.Fatal("expected a send command after the pause")
	}
	_, next := m.Update(cmd())
	if len(f.sent) != 1 || f.sent[0] != "c2:hello there" || m.isErr || next == nil {
		t.Fatalf("sent=%v status=%q", f.sent, m.status)
	}
	m.lastCtrlX = time.Time{}
	typ(m, "/quit")
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	if !m.isErr || len(f.sent) != 1 || m.pending != nil {
		t.Fatal("command-like text must not be sent by Ctrl-X")
	}
	m.input = nil
	typ(m, "plain text")
	press(m, tea.KeyEnter)
	if string(m.input) != "plain text\n" {
		t.Fatalf("Enter adds a line while Ctrl-X sends: %q", string(m.input))
	}
	typ(m, "second line")
	if !strings.Contains(m.View(), "plain text\u21b5second line") {
		t.Fatal("the input shows the line break")
	}
	m.lastCtrlX = time.Time{}
	_, tick = m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	if tick == nil || m.pending == nil || m.pending.text != "plain text\nsecond line" {
		t.Fatalf("multi-line text is sent as one message: %v", m.pending)
	}
}

func TestWinListIsSelectable(t *testing.T) {
	m := setup(t, newFake())
	enter(m, "/win")
	v := m.View()
	if m.mode != modeList || m.listSrc != listWindows || !strings.Contains(v, "0  server") || !strings.Contains(v, "#news") || !strings.Contains(v, "(current)") {
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
	for _, want := range []string{"[0:server]", "[1:#news]", "[2:#dev]", "[3:@alice]"} {
		if !strings.Contains(v, want) {
			t.Fatalf("window bar lacks %q:\n%s", want, v)
		}
	}
	lines := strings.Split(v, "\n")
	if len(lines) != 20 || !strings.Contains(lines[18], "[0:server]") || !strings.HasPrefix(lines[19], "[") {
		t.Fatalf("rule, status, bar, input: %d lines", len(lines))
	}
	if !strings.Contains(lines[16], "\u2500") || !strings.Contains(lines[17], "fake") {
		t.Fatalf("the rule precedes the status line:\n%s", strings.Join(lines[16:20], "\n"))
	}
	m.setErr("boom happened")
	lines = strings.Split(m.View(), "\n")
	if !strings.Contains(lines[16], "boom happened") || strings.Contains(lines[17], "boom happened") {
		t.Fatalf("the message sits on the rule, not in the status line:\n%s", strings.Join(lines[16:18], "\n"))
	}
}

func TestEscThenDigitSwitchesWindows(t *testing.T) {
	m := setup(t, newFake())
	typ(m, "half typed")
	press(m, tea.KeyEsc)
	typ(m, "2")
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
	typ(m, "0")
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
	m.revealAll()
	m.fx = false
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	m.Update(m.chatsCmd()())
	m.Update(m.syncCmd()()) // opens #news, #dev, @alice
	enter(m, "/close 2")    // #dev
	enter(m, "/win @alice")
	press(m, tea.KeyCtrlX)
	press(m, tea.KeyCtrlX) // Enter sends in @alice, remembered
	if len(m.wins) != 3 || m.cur != 2 || !m.wins[2].enterSends {
		t.Fatalf("wins=%d cur=%d", len(m.wins), m.cur)
	}

	again := New(Options{Store: st, Provider: f})
	again.revealAll()
	if len(again.wins) != 3 || again.wins[1].chatID != "c3" || again.wins[2].chatID != "c2" || again.cur != 2 {
		t.Fatalf("restored wins=%d cur=%d", len(again.wins), again.cur)
	}
	if !again.wins[2].enterSends || again.wins[1].enterSends {
		t.Fatal("Enter-sends state must be restored per window")
	}
	again.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	if !strings.Contains(again.View(), "enter: to send") {
		t.Fatal("status line must say how to send")
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
	m.revealAll()
	m.fx = false
	m.revealAll()
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m.Update(m.syncCmd()()) // hour-old messages must still come after the banner
	press(m, tea.KeyHome)
	v := m.View()
	for _, want := range []string{"######## ##    ##", "-!- lazygram v9.9.9"} {
		if !strings.Contains(v, want) {
			t.Fatalf("banner lacks %q:\n%s", want, v)
		}
	}
	first := -1
	for i, l := range strings.Split(v, "\n") {
		if strings.Contains(l, "##          ###") {
			first = i
			if strings.Contains(l, "-!-") {
				t.Fatalf("banner line carries the notice marker: %q", l)
			}
		}
	}
	if first < 1 {
		t.Fatal("banner line not found")
	}
	if a, b := strings.Index(v, "######## ##    ##"), strings.Index(v, "first dev line"); a < 0 || b < 0 || a > b {
		t.Fatal("banner must precede the messages")
	}
	if out := ExitBanner("v9.9.9", 100, "up 1s"); !strings.Contains(out, "lazygram v9.9.9") || !strings.Contains(out, "thanks") || !strings.Contains(out, "up 1s") || strings.Count(out, "\n") < 6 {
		t.Fatalf("exit banner:\n%s", out)
	}
	for _, width := range []int{120, 60, 20} {
		art := ansi.Strip(ExitBanner("v1", width, ""))
		found := false
		for _, v := range logo.Variants(width) {
			found = found || strings.Contains(art, v)
		}
		if !found {
			t.Fatalf("width %d: the exit banner is one of the faces that fit:\n%s", width, art)
		}
	}
	enter(m, "/win 1")
	enter(m, "/stats")
	drain(m)
	m.Update(sentMsg{chatID: "c3"})
	s := m.Summary()
	for _, want := range []string{"time wasted:", "arrived", "1 sent", "buttons pressed", "files fetched", "in the cache", "opened", "1 chats visited", "2 commands run"} {
		if !strings.Contains(s, want) {
			t.Fatalf("summary lacks %q:\n%s", want, s)
		}
	}
	enter(m, "/about")
	if v := body(m); !strings.Contains(v, "by t0d0r") || !strings.Contains(v, "lazygram v9.9.9") || !strings.Contains(v, "gotd/td") {
		t.Fatalf("about:\n%s", v)
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
	n := m.wins[0].notes
	if !m.isErr || !strings.Contains(n[len(n)-1].text, "press Yes: BOT_RESPONSE_TIMEOUT") {
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
	press(m, tea.KeyTab) // empty input: nothing in common, never moves the cursor
	if m.listCur != 1 || string(m.input) != "" {
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
	m.revealAll()
	m.fx = false
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
	enter(m, "/win 0")
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

func TestOpenFetchesSealsAndOpensImages(t *testing.T) {
	f := newFake()
	photo := mk("c1", "20", "ada", "look", 5)
	photo.Media = &domain.Media{Kind: "photo"}
	file := mk("c1", "21", "ada", "", 6)
	file.Media = &domain.Media{Kind: "file", Name: "../report.pdf"}
	f.msgs = append(f.msgs, photo, file)
	var opened []string
	st, _ := store.Open(store.Memory, bytes.Repeat([]byte{6}, 32))
	defer func() { _ = st.Close() }()
	m := New(Options{Store: st, Provider: f, Open: func(s string) error { opened = append(opened, s); return nil }})
	m.revealAll()
	m.fx = false
	defer m.Cleanup()
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m.Update(m.chatsCmd()())
	m.Update(m.syncCmd()())
	v := body(m)
	if !strings.Contains(v, "[photo] look") || !strings.Contains(v, "[file: ../report.pdf]") {
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
	if len(f.fetched) != 1 || f.fetched[0] != "c1:21" || next != nil || !strings.Contains(body(m), "-!- decrypted to") || len(opened) != 0 {
		t.Fatalf("file: fetched=%v opened=%v\n%s", f.fetched, opened, body(m))
	}
	if data, found, _ := st.Media("c1", "21"); !found || string(data) != "BYTES-21" {
		t.Fatal("the attachment must be in the vault")
	}
	if len(m.viewFiles) != 1 || !strings.HasSuffix(m.viewFiles[0], "-report.pdf") || strings.Contains(m.viewFiles[0], "..") {
		t.Fatalf("plain copy with a safe name: %v", m.viewFiles)
	}
	if got, _ := os.ReadFile(m.viewFiles[0]); string(got) != "BYTES-21" {
		t.Fatal("plain copy content")
	}
	cmd = enter(m, "/open out 20") // the photo: fetched, sealed, then opened outside
	_, next = m.Update(cmd())
	if next == nil {
		t.Fatal("images must be opened")
	}
	m.Update(next())
	if len(opened) != 1 || !strings.HasSuffix(opened[0], "20.jpg") {
		t.Fatalf("opened=%v", opened)
	}
	cmd = enter(m, "/open out 20") // second time: from the vault, no download
	m.Update(cmd())
	if len(f.fetched) != 2 {
		t.Fatalf("vault must serve the second open: fetched=%v", f.fetched)
	}
	enter(m, "/open 1") // a message without media
	if !m.isErr {
		t.Fatal("no media")
	}
	m.View()
	x, y := find(m, spanMedia, "c1\x0020")
	if x < 0 {
		t.Fatal("no media span")
	}
	cmd = click(m, x, y) // a click shows it in the terminal: the bytes are not a picture here
	_, next = m.Update(cmd())
	if next != nil || !m.isErr || len(opened) != 1 {
		t.Fatalf("click: opened=%v status=%q", opened, m.status)
	}
	dir := m.viewDir
	m.Cleanup()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("plain copies must be gone after Cleanup")
	}
}

func TestImageTokenWithATrapNameIsNeverOpened(t *testing.T) {
	// A provider that labels something an image but hands back a name the
	// viewer would not treat as one: the copy is saved, not opened.
	f := newFake()
	trap := mk("c1", "30", "ada", "", 7)
	trap.Media = &domain.Media{Kind: "image", Name: "photo.html"}
	f.msgs = append(f.msgs, trap)
	var opened []string
	st, _ := store.Open(store.Memory, bytes.Repeat([]byte{8}, 32))
	defer func() { _ = st.Close() }()
	m := New(Options{Store: st, Provider: f, Open: func(s string) error { opened = append(opened, s); return nil }})
	m.revealAll()
	m.fx = false
	defer m.Cleanup()
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m.Update(m.chatsCmd()())
	m.Update(m.syncCmd()())
	enter(m, "/win #dev")
	_, next := m.Update(enter(m, "/open out 30")())
	// the fake names the copy 30.jpg (an image extension), so it opens; a
	// name without one would not
	if next == nil || len(m.viewFiles) != 1 || !imageFile(m.viewFiles[0]) {
		t.Fatalf("copy %v", m.viewFiles)
	}
	if imageFile("/x/photo.html") || imageFile("/x/a.svg") || !imageFile("/x/A.PNG") {
		t.Fatal("imageFile")
	}
	if sanitizeName("../../.hidden") != "hidden" || sanitizeName("") != "attachment" || sanitizeName("ok.png") != "ok.png" {
		t.Fatal("sanitizeName")
	}
}

func TestMsgSendsAndOpensTheWindow(t *testing.T) {
	f := newFake()
	m := setup(t, f)
	enter(m, "/close 3") // @alice has no window now
	cmd := enter(m, "/msg @alice hello from the server window")
	if cmd == nil || m.winName(m.cur) != "@alice" {
		t.Fatalf("window must open and focus: %s", m.winName(m.cur))
	}
	// the batch holds the recent fetch for the new window and the send
	for _, msg := range []tea.Msg{} {
		m.Update(msg)
	}
	run := func(c tea.Cmd) {
		if c == nil {
			return
		}
		switch r := c().(type) {
		case tea.BatchMsg:
			for _, x := range r {
				m.Update(x())
			}
		default:
			m.Update(r)
		}
	}
	run(cmd)
	if len(f.sent) != 1 || f.sent[0] != "c2:hello from the server window" {
		t.Fatalf("sent=%v", f.sent)
	}
	enter(m, "/msg #dev")
	if !m.isErr || !strings.Contains(m.status, "usage") {
		t.Fatalf("usage: %q", m.status)
	}
	enter(m, "/msg #nope hi")
	if !m.isErr {
		t.Fatal("unknown chat")
	}
	cmd = enter(m, "/msg #dev ping") // existing window: just send and focus
	run(cmd)
	if m.winName(m.cur) != "#dev" || len(f.sent) != 2 || f.sent[1] != "c1:ping" {
		t.Fatalf("sent=%v win=%s", f.sent, m.winName(m.cur))
	}
}

func TestUpDownWalkTheInputHistory(t *testing.T) {
	f := newFake()
	m := setup(t, f)
	press(m, tea.KeyUp) // nothing yet: no crash, no change
	enter(m, "/win @alice")
	typ(m, "hello")
	_, tick := m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	m.Update(tick()) // the pause passed: sent and remembered
	m.lastCtrlX = time.Time{}
	enter(m, "/stats")
	enter(m, "/stats") // duplicates collapse
	typ(m, "draft")
	press(m, tea.KeyUp)
	if got := string(m.input); got != "/stats" {
		t.Fatalf("up: %q", got)
	}
	press(m, tea.KeyUp)
	if got := string(m.input); got != "hello" {
		t.Fatalf("up up: %q", got)
	}
	press(m, tea.KeyUp)
	if got := string(m.input); got != "/win @alice" {
		t.Fatalf("oldest: %q", got)
	}
	press(m, tea.KeyUp) // stays at the oldest
	if got := string(m.input); got != "/win @alice" {
		t.Fatalf("past oldest: %q", got)
	}
	press(m, tea.KeyDown)
	press(m, tea.KeyDown)
	press(m, tea.KeyDown)
	if got := string(m.input); got != "draft" {
		t.Fatalf("down restores the draft: %q", got)
	}
	press(m, tea.KeyUp)
	typ(m, "!") // editing a recalled line leaves the walk
	press(m, tea.KeyUp)
	if got := string(m.input); got != "/stats" {
		t.Fatalf("after edit: %q", got)
	}
	if m.wins[m.cur].scroll != 0 {
		t.Fatal("Up must not scroll the window")
	}
	m.input = nil
	enter(m, "/search x")
	press(m, tea.KeyUp)
	if m.wins[m.cur].scroll != 1 {
		t.Fatal("in the pager Up scrolls")
	}
}

func TestFastDoubleCtrlXKeepsTheTextAndOnlyToggles(t *testing.T) {
	f := newFake()
	m := setup(t, f)
	enter(m, "/win @alice")
	typ(m, "not yet")
	_, tick := m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlX}) // fast: cancel the send, toggle
	if !m.wins[m.cur].enterSends || string(m.input) != "not yet" || m.pending != nil {
		t.Fatalf("enterSends=%v input=%q pending=%v", m.wins[m.cur].enterSends, string(m.input), m.pending)
	}
	if _, cmd := m.Update(tick()); cmd != nil || len(f.sent) != 0 {
		t.Fatal("the stale timer must not send")
	}
	cmd := press(m, tea.KeyEnter) // Enter sends now, at once
	if cmd == nil {
		t.Fatal("Enter should send")
	}
	cmd()
	if len(f.sent) != 1 || f.sent[0] != "c2:not yet" {
		t.Fatalf("sent=%v", f.sent)
	}
}

func TestTabOnEmptyPromptCyclesWindows(t *testing.T) {
	m := setup(t, newFake()) // 0 server, 1 #news, 2 #dev, 3 @alice
	press(m, tea.KeyTab)
	if m.cur != 1 {
		t.Fatalf("tab: %d", m.cur)
	}
	press(m, tea.KeyTab)
	press(m, tea.KeyTab)
	press(m, tea.KeyTab)
	if m.cur != 0 {
		t.Fatalf("wraps to the server window: %d", m.cur)
	}
	press(m, tea.KeyShiftTab)
	if m.cur != 3 {
		t.Fatalf("shift-tab goes back: %d", m.cur)
	}
	typ(m, "/wh")
	press(m, tea.KeyTab) // with text, Tab completes
	if m.cur != 3 || string(m.input) != "/whois " {
		t.Fatalf("completion: cur=%d input=%q", m.cur, string(m.input))
	}
}

func TestFxToggleIsKept(t *testing.T) {
	m := setup(t, newFake())
	m.fx = true
	enter(m, "/fx")
	if m.fx {
		t.Fatal("/fx toggles")
	}
	var saved bool
	if ok, _ := m.opt.Store.State(fxKey, &saved); !ok || saved {
		t.Fatal("the choice is kept")
	}
	m.View()
	if _, cmd := m.Update(fxTickMsg{}); cmd != nil || m.animOn {
		t.Fatal("no ticks while off")
	}
}

func TestScrolledUpIndicator(t *testing.T) {
	f := newFake()
	m := setup(t, f)
	for i := 0; i < 40; i++ {
		f.msgs = append(f.msgs, mk("c1", fmt.Sprintf("f%d", i), "x", "filler", i))
	}
	m.Update(m.syncCmd()())
	m.View()
	press(m, tea.KeyPgUp)
	v := m.View()
	if !strings.Contains(v, fmt.Sprintf("vvv %d more vvv", m.wins[m.cur].scroll)) || !strings.Contains(v, "scrolled up") {
		t.Fatalf("indicator missing:\n%s", v)
	}
	x, y := find(m, spanEnd, "")
	if x < 0 {
		t.Fatal("the marker must be clickable")
	}
	click(m, x, y)
	if m.wins[m.cur].scroll != 0 || strings.Contains(m.View(), "more vvv") {
		t.Fatal("a click jumps to the newest line")
	}
}

func TestFetchErrorLandsInWindow(t *testing.T) {
	f := newFake()
	photo := mk("c1", "20", "ada", "look", 5)
	photo.Media = &domain.Media{Kind: "photo"}
	f.msgs = append(f.msgs, photo)
	m := setup(t, f)
	defer m.Cleanup()
	enter(m, "/win #dev")
	f.err = errors.New("FILE_REFERENCE_EXPIRED")
	_, next := m.Update(enter(m, "/open 20")())
	n := m.wins[0].notes
	if !m.isErr || !strings.Contains(n[len(n)-1].text, "fetch: FILE_REFERENCE_EXPIRED") || next != nil {
		t.Fatal("fetch errors land in the server window")
	}
}

func TestRunningLightFollowsTheMouse(t *testing.T) {
	f := newFake()
	f.msgs = append(f.msgs, mk("c1", "9", "alice", "see https://example.invalid/doc now", 4))
	m := setup(t, f)
	m.fx = true
	m.View()
	if m.animOn {
		t.Fatal("nothing hovered: no animation")
	}
	x, y := find(m, spanURL, "https://example.invalid/doc")
	_, first := m.Update(tea.MouseMsg{X: x, Y: y, Action: tea.MouseActionMotion})
	if m.hover != hoverKey(span{kind: spanURL, target: "https://example.invalid/doc"}) {
		t.Fatalf("hover=%q", m.hover)
	}
	if first == nil || !m.ticking {
		t.Fatal("the very first hover must schedule a tick without waiting for another event")
	}
	m.View()
	_, cmd := m.Update(fxTickMsg{})
	if !m.animOn || cmd == nil || m.phaseOpt() < 0 {
		t.Fatal("the light runs under the pointer")
	}
	m.Update(tea.MouseMsg{X: 0, Y: 0, Action: tea.MouseActionMotion})
	m.View()
	if m.hover != "" || m.animOn {
		t.Fatal("leaving the element stops the light")
	}
	enter(m, "/win @alice")
	m.View()
	bx, by := find(m, spanButtons, "c2\x007")
	if bx < 0 {
		t.Fatal("button line span")
	}
	m.Update(tea.MouseMsg{X: bx + 1, Y: by, Action: tea.MouseActionMotion})
	m.View()
	if !m.animOn {
		t.Fatal("buttons animate under the pointer")
	}
}

func TestLeadingSpaceSendsSlashText(t *testing.T) {
	f := newFake()
	m := setup(t, f)
	enter(m, "/win @alice")
	typ(m, " /not a command")
	_, tick := m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	if tick == nil || m.pending == nil || m.pending.text != "/not a command" {
		t.Fatalf("a leading space must send the text: pending=%v", m.pending)
	}
}

func TestEscFromListRestoresNoFilter(t *testing.T) {
	m := setup(t, newFake())
	enter(m, "/list ali")
	press(m, tea.KeyEsc)
	typ(m, "2")
	if m.cur != 2 || len(m.input) != 0 {
		t.Fatalf("cur=%d input=%q", m.cur, string(m.input))
	}
}

func TestNickClickNeedsAnExactName(t *testing.T) {
	f := newFake()
	f.msgs = append(f.msgs, mk("c1", "9", "al", "short name", 4)) // "al" is a prefix of alice
	m := setup(t, f)
	m.View()
	x, y := find(m, spanNick, "al")
	click(m, x, y)
	if !m.isErr || m.cur != 0 {
		t.Fatalf("a prefix must not open a chat: cur=%d status=%q", m.cur, m.status)
	}
	x, y = find(m, spanNick, "alice")
	click(m, x, y)
	if m.winName(m.cur) != "@alice" {
		t.Fatal("an exact alias opens the chat")
	}
}

func TestFetchInFlightGuard(t *testing.T) {
	f := newFake()
	photo := mk("c1", "20", "ada", "look", 5)
	photo.Media = &domain.Media{Kind: "photo"}
	f.msgs = append(f.msgs, photo)
	m := setup(t, f)
	defer m.Cleanup()
	enter(m, "/win #dev")
	first := enter(m, "/open 20")
	second := enter(m, "/open 20")
	if first == nil || second != nil || !strings.Contains(m.status, "already fetching") {
		t.Fatalf("second fetch must be ignored: %q", m.status)
	}
	m.Update(first())
	if third := enter(m, "/open 20"); third == nil {
		t.Fatal("after the first completes, a fetch may run again")
	}
}

func TestMouseToggleIsKeptAndAppliedAtStart(t *testing.T) {
	st, _ := store.Open(store.Memory, bytes.Repeat([]byte{9}, 32))
	defer func() { _ = st.Close() }()
	m := New(Options{Store: st, Provider: newFake()})
	m.revealAll()
	m.fx = false
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	cmd := enter(m, "/mouse")
	if m.mouseOn || cmd == nil || !strings.Contains(m.status, "drag selects") {
		t.Fatalf("off: %v %q", m.mouseOn, m.status)
	}
	var saved bool
	if ok, _ := st.State(mouseKey, &saved); !ok || saved {
		t.Fatal("kept")
	}
	again := New(Options{Store: st, Provider: newFake()})
	again.revealAll()
	if again.mouseOn || again.Init() == nil {
		t.Fatal("a saved off state is applied at start")
	}
	if cmd := enter(m, "/mouse"); !m.mouseOn || cmd == nil {
		t.Fatal("on again")
	}
}

func TestCtrlXAddsALineWhenEnterSends(t *testing.T) {
	f := newFake()
	m := setup(t, f)
	enter(m, "/win @alice")
	press(m, tea.KeyCtrlX)
	press(m, tea.KeyCtrlX) // Enter sends now
	typ(m, "first")
	m.lastCtrlX = time.Time{}
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	if string(m.input) != "first\n" || m.pending != nil {
		t.Fatalf("Ctrl-X adds a line: %q", string(m.input))
	}
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlX}) // fast second press: toggle back, line removed
	if m.wins[m.cur].enterSends || string(m.input) != "first" {
		t.Fatalf("toggle: enterSends=%v input=%q", m.wins[m.cur].enterSends, string(m.input))
	}
}

func TestEditAndDeleteLastOwnMessage(t *testing.T) {
	f := newFake()
	f.msgs = append(f.msgs, mk("c1", "5", "me", "my typo", 3), mk("c1", "6", "ada", "reply", 4))
	m := setup(t, f)
	enter(m, "/edit x")
	if !m.isErr {
		t.Fatal("server window")
	}
	enter(m, "/win @alice")
	enter(m, "/delete")
	if !m.isErr || !strings.Contains(m.status, "no message of yours") {
		t.Fatalf("no own message: %q", m.status)
	}
	enter(m, "/win #dev")
	enter(m, "/edit")
	if string(m.input) != "/edit my typo" {
		t.Fatalf("prefill: %q", string(m.input))
	}
	m.input = nil
	cmd := enter(m, "/edit my fix")
	if cmd == nil {
		t.Fatal("expected an edit")
	}
	m.Update(cmd())
	if len(f.edited) != 1 || f.edited[0] != "c1:5:my fix" || m.isErr {
		t.Fatalf("edited=%v status=%q", f.edited, m.status)
	}
	if !strings.Contains(body(m), "<me> my fix") || strings.Contains(body(m), "my typo") {
		t.Fatalf("edit must show:\n%s", body(m))
	}
	if got, _ := m.opt.Store.Messages("c1", 0); got[len(got)-2].Text != "my fix" {
		t.Fatal("cache must hold the edit")
	}
	cmd = enter(m, "/delete")
	m.Update(cmd())
	if len(f.deleted) != 1 || f.deleted[0] != "c1:5" || strings.Contains(body(m), "<me> my fix") || !strings.Contains(body(m), "deleted your message: my fix") {
		t.Fatalf("deleted=%v\n%s", f.deleted, body(m))
	}
	if n, _ := m.opt.Store.Count("c1"); n != 3 {
		t.Fatalf("cache count %d", n)
	}
	f.err = errors.New("MESSAGE_EDIT_TIME_EXPIRED")
	f.msgs = nil
	enter(m, "/win 0")
	enter(m, "/win #dev")
	enter(m, "/delete")
	if !m.isErr {
		t.Fatal("nothing left of mine")
	}
}

func TestNamesTogglesNickOrTitle(t *testing.T) {
	m := setup(t, newFake())
	if !strings.Contains(m.View(), "[3:@alice]") {
		t.Fatal("nick by default")
	}
	enter(m, "/names")
	v := m.View()
	if !m.byName || !strings.Contains(v, "[3:@Alice Smith]") || !strings.Contains(v, "[1:#News]") || strings.Contains(v, "[3:@alice]") {
		t.Fatalf("by name:\n%s", v)
	}
	if !strings.Contains(body(m), "@Alice Smith") {
		t.Fatal("the server window column follows")
	}
	var saved bool
	if ok, _ := m.opt.Store.State(namesKey, &saved); !ok || !saved {
		t.Fatal("kept")
	}
	enter(m, "/win @alice") // commands still take the alias
	if m.winName(m.cur) != "@Alice Smith" {
		t.Fatalf("win: %s", m.winName(m.cur))
	}
	enter(m, "/names")
	if m.byName || !strings.Contains(m.View(), "[3:@alice]") {
		t.Fatal("back to nicks")
	}
}

func TestOpenShowsPicturesInTheTerminal(t *testing.T) {
	f := newFake()
	photo := mk("c1", "20", "ada", "look", 5)
	photo.Media = &domain.Media{Kind: "photo"}
	f.msgs = append(f.msgs, photo)
	f.picture = testPNG(t, 200, 100)
	var opened []string
	m := setup(t, f)
	m.opt.Open = func(s string) error { opened = append(opened, s); return nil }
	defer m.Cleanup()
	enter(m, "/win #dev")
	_, next := m.Update(enter(m, "/open")())
	if next != nil || m.isErr || len(opened) != 0 || len(m.viewFiles) != 0 {
		t.Fatalf("in-terminal: next=%v err=%q files=%v", next, m.status, m.viewFiles)
	}
	n := m.wins[m.cur].notes
	var pic *note
	for i := range n {
		if n[i].pre {
			pic = &n[i]
		}
	}
	if pic == nil || strings.Count(pic.text, "\u2580") == 0 || !strings.Contains(n[len(n)-1].text, "200x100 px") {
		t.Fatalf("picture note missing:\n%+v", n[len(n)-1])
	}
	if !strings.Contains(body(m), "\u2580") {
		t.Fatal("the picture renders in the window")
	}
	if got, found, _ := m.opt.Store.Media("c1", "20"); !found || !bytes.Equal(got, f.picture) {
		t.Fatal("the vault keeps the picture")
	}
}

func TestSendFile(t *testing.T) {
	f := newFake()
	m := setup(t, f)
	dir := t.TempDir()
	pic := filepath.Join(dir, "cat.png")
	if err := os.WriteFile(pic, []byte("PNG"), 0o600); err != nil {
		t.Fatal(err)
	}
	enter(m, "/send "+pic)
	if !m.isErr {
		t.Fatal("server window")
	}
	enter(m, "/win @alice")
	enter(m, "/send")
	if !m.isErr || !strings.Contains(m.status, "usage") {
		t.Fatalf("usage: %q", m.status)
	}
	enter(m, "/send "+filepath.Join(dir, "missing.png"))
	if !m.isErr {
		t.Fatal("missing file")
	}
	enter(m, "/send "+dir)
	if !m.isErr || !strings.Contains(m.status, "regular") {
		t.Fatalf("directory: %q", m.status)
	}
	cmd := enter(m, "/send "+pic+" my cat")
	if cmd == nil || !strings.Contains(m.status, "sending photo cat.png") {
		t.Fatalf("expected a send: %q", m.status)
	}
	m.Update(cmd())
	if len(f.sentFiles) != 1 || f.sentFiles[0] != "c2:cat.png:3:my cat" || m.isErr {
		t.Fatalf("sentFiles=%v status=%q", f.sentFiles, m.status)
	}
}

func TestCompleteArgumentsAndPaths(t *testing.T) {
	m := setup(t, newFake())
	for typed, want := range map[string]string{
		"/win l": "/win list ", "/list -": "/list --show-deleted ",
		"/win @al": "/win @alice ", "/open o": "/open o", // nothing after /open in the server window
		" ": " ", "  ": "  ", " @al": " @alice ", "   x": "   x", // blank prefixes never crash
	} {
		m.input = []rune(typed)
		m.complete()
		if got := string(m.input); got != want {
			t.Errorf("%q: got %q, want %q", typed, got, want)
		}
	}
	dir := t.TempDir()
	for _, n := range []string{"cat.png", "cow.jpg", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	m.input = []rune("/send " + filepath.Join(dir, "n"))
	m.complete()
	if got := string(m.input); got != "/send "+filepath.Join(dir, "notes.txt")+" " {
		t.Fatalf("single file: %q", got)
	}
	m.input = []rune("/send " + filepath.Join(dir, "c"))
	m.complete()
	if got := string(m.input); got != "/send "+filepath.Join(dir, "c") {
		t.Fatalf("no common prefix beyond c: %q", got)
	}
	m.complete() // second Tab lists them
	if !strings.Contains(body(m), "cat.png  ") || !strings.Contains(body(m), "cow.jpg") {
		t.Fatalf("list: %q", body(m))
	}
	m.input = []rune("/send " + filepath.Join(dir, "s"))
	m.complete()
	if got := string(m.input); got != "/send "+filepath.Join(dir, "sub")+"/" {
		t.Fatalf("directory: %q", got)
	}
	m.input = []rune("/send " + filepath.Join(dir, "cat.png") + " my ca")
	m.complete()
	if got := string(m.input); got != "/send "+filepath.Join(dir, "cat.png")+" my ca" {
		t.Fatalf("caption untouched: %q", got)
	}
}

func TestStartupRevealsTheWindowLineByLine(t *testing.T) {
	st, err := store.Open(store.Memory, bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	m := New(Options{Store: st, Provider: newFake()})
	m.fx = false
	_, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	if cmd == nil || !m.wins[0].revealing {
		t.Fatal("the server window rolls in at start, a tick is scheduled")
	}
	if b := strings.TrimSpace(ansi.Strip(body(m))); b != "" {
		t.Fatalf("nothing shown before the first tick: %q", b)
	}
	m.Update(revealTickMsg{})
	rows := strings.Split(ansi.Strip(body(m)), "\n")
	if last := rows[len(rows)-1]; strings.TrimSpace(last) == "" || strings.TrimSpace(rows[len(rows)-2]) != "" {
		t.Fatalf("one line at the bottom of the screen after one tick:\n%s", body(m))
	}
	m.Update(revealTickMsg{})
	rows = strings.Split(ansi.Strip(body(m)), "\n")
	if strings.TrimSpace(rows[len(rows)-2]) == "" || strings.TrimSpace(rows[len(rows)-3]) != "" {
		t.Fatalf("the second line pushes the first one up:\n%s", body(m))
	}
	for i := 0; i < 200 && m.wins[0].revealing; i++ {
		m.Update(revealTickMsg{})
		m.View()
	}
	if m.wins[0].revealing {
		t.Fatal("the reveal ends")
	}
	full := body(m)
	m.revealAll()
	if body(m) != full || !strings.Contains(ansi.Strip(full), "/help lists commands") {
		t.Fatalf("after the reveal the window looks as usual:\n%s", full)
	}
	_, cmd = m.Update(revealTickMsg{})
	if cmd != nil {
		t.Fatal("no more ticks once revealed")
	}
	if _, w := m.openWin("c1"); w.revealing {
		t.Fatal("other windows appear at once")
	}
}

func TestPrintSlowlyKeepsEveryLine(t *testing.T) {
	var b bytes.Buffer
	PrintSlowly(&b, "one\ntwo\nthree\n")
	if b.String() != "one\ntwo\nthree\n" {
		t.Fatalf("%q", b.String())
	}
}

func TestArrivalStatusNamesTheWindows(t *testing.T) {
	m := setup(t, newFake())
	now := time.Now()
	m.Update(syncedMsg{at: now, fresh: []domain.Message{
		{ID: "n1", ChatID: "c1", Sender: "ada", Text: "one", Time: now},
		{ID: "n2", ChatID: "c1", Sender: "ada", Text: "two", Time: now},
		{ID: "n3", ChatID: "c2", Sender: "alice", Text: "three", Time: now},
	}})
	if m.status != "3 new messages in 2:#dev (2), 3:@alice" {
		t.Fatalf("status %q", m.status)
	}
	m.Update(syncedMsg{at: now, fresh: []domain.Message{{ID: "n4", ChatID: "c2", Sender: "alice", Text: "four", Time: now}}})
	if m.status != "1 new message in 3:@alice" {
		t.Fatalf("status %q", m.status)
	}
	// What this account sent comes back through sync: no news, no unread.
	m.setInfo("")
	_, w := m.winFor("c2")
	unread, seen := w.unread, m.seen
	m.Update(syncedMsg{at: now, fresh: []domain.Message{{ID: "n5", ChatID: "c2", Sender: "me", Text: "mine", Time: now}}})
	if m.status != "" || w.unread != unread || m.seen != seen {
		t.Fatalf("own message counted: status %q unread %d->%d seen %d->%d", m.status, unread, w.unread, seen, m.seen)
	}
	if !strings.Contains(strings.Join(m.lines(), "\n"), "<me> mine") {
		t.Fatal("the own message still shows in the window")
	}
	m.Update(syncedMsg{at: now, fresh: []domain.Message{
		{ID: "n6", ChatID: "c2", Sender: "me", Text: "mine too", Time: now},
		{ID: "n7", ChatID: "c1", Sender: "ada", Text: "five", Time: now},
	}})
	if m.status != "1 new message in 2:#dev" {
		t.Fatalf("status %q", m.status)
	}
}

func TestFirstHistoryOfAWindowRollsIn(t *testing.T) {
	m := setup(t, newFake())
	now := time.Now()
	_, w := m.winFor("c3") // #news: opened for its unread count, no messages yet
	if w == nil || w.revealing {
		t.Fatal("window for c3 open and still")
	}
	_, cmd := m.Update(recentMsg{chatID: "c3", fresh: []domain.Message{{ID: "h1", ChatID: "c3", Sender: "wire", Text: "old news", Time: now.Add(-time.Hour)}}})
	if !w.revealing || w.revealed != 0 {
		t.Fatal("the first fetched history rolls in")
	}
	if cmd != nil { // not current: the ticks start when it is shown
		t.Fatal("no tick for a window that is not shown")
	}
	enter(m, "/win #news")
	if !w.revealing {
		t.Fatal("still pending when switched to")
	}
	m.revealAll()
	_, w2 := m.winFor("c1") // #dev already has lines: a later fetch appends quietly
	m.Update(recentMsg{chatID: "c1", fresh: []domain.Message{{ID: "h2", ChatID: "c1", Sender: "ada", Text: "older", Time: now.Add(-2 * time.Hour)}}})
	if w2.revealing {
		t.Fatal("a window with content does not roll in again")
	}
}

func TestCompleteOpenMessageIDs(t *testing.T) {
	m := setup(t, newFake())
	now := time.Now()
	m.Update(syncedMsg{at: now, fresh: []domain.Message{
		{ID: "143541", ChatID: "c2", Sender: "alice", Text: "pic", Time: now, Media: &domain.Media{Kind: "photo"}},
		{ID: "143999", ChatID: "c2", Sender: "alice", Text: "doc", Time: now, Media: &domain.Media{Kind: "file", Name: "a.pdf"}},
		{ID: "150000", ChatID: "c2", Sender: "alice", Text: "no attachment", Time: now},
		{ID: "143000", ChatID: "c1", Sender: "ada", Text: "other chat", Time: now, Media: &domain.Media{Kind: "photo"}},
	}})
	enter(m, "/win @alice")
	for typed, want := range map[string]string{
		"/open 1435":      "/open 143541 ",
		"/open out 1439":  "/open out 143999 ",
		"/fetch 1435":     "/fetch 143541 ",
		"/open o":         "/open out ",
		" /open o":        " /open out ",
		"/open 15":        "/open 15",        // no attachment
		"/open 1430":      "/open 1430",      // another chat
		"/open out 1 x":   "/open out 1 x",   // only the first id
		"/open out out 1": "/open out out 1", // "out" once
	} {
		m.input = []rune(typed)
		m.comp = nil
		m.complete()
		if got := string(m.input); got != want {
			t.Errorf("%q: got %q, want %q", typed, got, want)
		}
	}
	m.input = []rune("/open 14")
	m.comp = nil
	m.complete()
	if got := string(m.input); got != "/open 143" {
		t.Fatalf("common prefix: %q", got)
	}
	m.complete()
	if !strings.Contains(body(m), "143541  143999") {
		t.Fatalf("second Tab lists the ids:\n%s", body(m))
	}
	m.input = nil
	enter(m, "/win 0")
	for _, typed := range []string{"/open 14", "/open o", "/open out 14"} {
		m.input = []rune(typed)
		m.comp = nil
		m.complete()
		if got := string(m.input); got != typed {
			t.Fatalf("server window completes nothing after /open: %q -> %q", typed, got)
		}
	}
}

func TestRedrawDropsNotesKeepsMessages(t *testing.T) {
	m := setup(t, newFake())
	enter(m, "/win @alice")
	w := m.wins[m.cur]
	m.note(w, "a notice")
	w.notes = append(w.notes, note{at: time.Now(), text: "a picture", pre: true})
	enter(m, "/help") // queued slow print into window 0
	m.input = nil
	enter(m, "/win @alice")
	v := strings.Join(m.lines(), "\n")
	if !strings.Contains(v, "a notice") || !strings.Contains(v, "a picture") || !strings.Contains(v, "hi there") {
		t.Fatalf("before:\n%s", v)
	}
	enter(m, "/redraw")
	v = strings.Join(m.lines(), "\n")
	if strings.Contains(v, "a notice") || strings.Contains(v, "a picture") || !strings.Contains(v, "hi there") || len(w.notes) != 0 {
		t.Fatalf("after:\n%s", v)
	}
	if m.isErr {
		t.Fatalf("status %q", m.status)
	}
	// Window 0 was mid help print: /clear there drops the queue too.
	m.input = nil
	enter(m, "/win 0")
	if len(m.slow) == 0 {
		t.Fatal("help still printing into window 0")
	}
	enter(m, "/clear")
	if len(m.slow) != 0 || m.slowWin != nil || len(m.wins[0].notes) != 0 {
		t.Fatal("clear drops the notes and the queued lines")
	}
	if !strings.Contains(strings.Join(m.lines(), "\n"), "first dev line") {
		t.Fatal("messages stay in the server window")
	}
}
