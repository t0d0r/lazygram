package tui

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"lazygram/internal/domain"
	"lazygram/internal/store"
)

// runAll executes a command and feeds every message it yields back,
// batches included, so a window switch's side effects (the history fetch,
// a receipt) are applied.
func runAll(m *Model, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			runAll(m, c)
		}
		return
	}
	if msg != nil {
		m.Update(msg)
	}
}

func persistedOn(m *Model) bool {
	var on bool
	found, _ := m.opt.Store.State(receiptsKey, &on)
	return found && on
}

func TestReadReceiptsAreOffByDefault(t *testing.T) {
	f := newFake()
	m := setup(t, f)
	if m.receipts || persistedOn(m) {
		t.Fatal("off by default, nothing persisted")
	}
	if m.wins[1].unread == 0 {
		t.Fatal("window 1 starts with unread messages")
	}
	runAll(m, enter(m, "/win 1"))
	if len(f.read) != 0 {
		t.Fatalf("a focus with receipts off sent %v", f.read)
	}
	enter(m, "/read-receipts")
	if m.isErr || !strings.Contains(m.status, "read receipts off") {
		t.Fatalf("status %q", m.status)
	}
	enter(m, "/read-receipts maybe")
	if !m.isErr || m.receipts || persistedOn(m) {
		t.Fatalf("a bad argument changes nothing: %q", m.status)
	}
}

func TestReadReceiptsOnReportFocusWithUnreadOnly(t *testing.T) {
	f := newFake()
	m := setup(t, f)
	enter(m, "/read-receipts on")
	if !m.receipts || m.isErr || !strings.Contains(m.status, "read receipts on") || !persistedOn(m) {
		t.Fatalf("on: %v %q", m.receipts, m.status)
	}
	// Focusing a window with unread messages reports the newest message
	// lazygram holds for its chat.
	dev, w := m.winFor("c1")
	w.unread = 2
	want, _ := m.newestID("c1")
	runAll(m, enter(m, "/win "+strconv.Itoa(dev)))
	if len(f.read) != 1 || f.read[0] != "c1:"+want || w.unread != 0 {
		t.Fatalf("focus: read=%v unread=%d", f.read, w.unread)
	}
	// Focusing it again with nothing unread sends nothing.
	runAll(m, enter(m, "/win 0"))
	runAll(m, enter(m, "/win "+strconv.Itoa(dev)))
	if len(f.read) != 1 {
		t.Fatalf("a focus without unread sent: %v", f.read)
	}
	// A message arriving in the window shown is not reported; one arriving
	// elsewhere is, when that window is focused.
	m.arrived([]domain.Message{mk("c1", "9", "ada", "hi", 9)})
	if len(f.read) != 1 || w.unread != 0 {
		t.Fatalf("arrival in the shown window: read=%v unread=%d", f.read, w.unread)
	}
	m.arrived([]domain.Message{mk("c2", "9", "alice", "yo", 9)})
	ali, aw := m.winFor("c2")
	if aw.unread == 0 {
		t.Fatal("alice has unread messages")
	}
	want2, _ := m.newestID("c2")
	runAll(m, enter(m, "/win "+strconv.Itoa(ali)))
	if len(f.read) != 2 || f.read[1] != "c2:"+want2 {
		t.Fatalf("focus after arrival: %v", f.read)
	}
	// Lists, filters and the pager report nothing.
	enter(m, "/list")
	typ(m, "ne")
	press(m, tea.KeyEsc)
	enter(m, "/win list")
	press(m, tea.KeyEsc)
	enter(m, "/search hi")
	press(m, tea.KeyEsc)
	if len(f.read) != 2 {
		t.Fatalf("lists sent: %v", f.read)
	}
	// A window whose chat has no known message has nothing to report.
	m.msgs = domain.Remove(m.msgs, "c3", "7")
	news, nw := m.winFor("c3")
	nw.unread = 4
	runAll(m, enter(m, "/win "+strconv.Itoa(news)))
	if len(f.read) != 2 || nw.unread != 0 {
		t.Fatalf("no known message: read=%v unread=%d", f.read, nw.unread)
	}
	// Off again: a focus with unread messages sends nothing.
	enter(m, "/read-receipts off")
	if m.receipts || persistedOn(m) {
		t.Fatal("off and persisted off")
	}
	w.unread = 1
	runAll(m, enter(m, "/win "+strconv.Itoa(dev)))
	if len(f.read) != 2 {
		t.Fatalf("off sent: %v", f.read)
	}
}

func TestReadReceiptErrorIsReported(t *testing.T) {
	f := newFake()
	m := setup(t, f)
	enter(m, "/read-receipts on")
	dev, w := m.winFor("c1")
	runAll(m, enter(m, "/win "+strconv.Itoa(dev))) // loads the window's history once (and reports its unread)
	runAll(m, enter(m, "/win 0"))
	before := len(f.read)
	f.err = errors.New("boom")
	w.unread = 1
	runAll(m, enter(m, "/win "+strconv.Itoa(dev))) // only the receipt runs now
	if !m.isErr || !strings.Contains(m.status, "read receipt for #dev") || w.unread != 0 {
		t.Fatalf("error: isErr=%v status=%q unread=%d", m.isErr, m.status, w.unread)
	}
	if len(f.read) != before+1 {
		t.Fatalf("one attempt, no retry: %v", f.read)
	}
}

func TestReadReceiptsSettingSurvivesRestart(t *testing.T) {
	st, _ := store.Open(store.Memory, bytes.Repeat([]byte{9}, 32))
	defer func() { _ = st.Close() }()
	m := New(Options{Store: st, Provider: newFake()})
	m.revealAll()
	if m.receipts {
		t.Fatal("a fresh store means off")
	}
	enter(m, "/read-receipts on")
	again := New(Options{Store: st, Provider: newFake()})
	again.revealAll()
	if !again.receipts {
		t.Fatal("the setting is restored from the store")
	}
	enter(again, "/read-receipts off")
	third := New(Options{Store: st, Provider: newFake()})
	third.revealAll()
	if third.receipts {
		t.Fatal("off is restored too")
	}
}

// TestListReadButtonFollowsTheReceiptRule: the [read] button of /win and
// /list reports the chat like a focus does, with receipts on only.
func TestListReadButtonFollowsTheReceiptRule(t *testing.T) {
	f := newFake()
	m := setup(t, f)
	dev, w := m.winFor("c1")
	w.unread = 2
	enter(m, "/win")
	for m.listCur != dev {
		press(m, tea.KeyDown)
	}
	press(m, tea.KeyRight) // [read]
	runAll(m, press(m, tea.KeyEnter))
	if w.unread != 0 || len(f.read) != 0 {
		t.Fatalf("receipts off: unread=%d read=%v", w.unread, f.read)
	}
	press(m, tea.KeyEsc)
	enter(m, "/read-receipts on")
	w.unread = 2
	want, _ := m.newestID("c1")
	enter(m, "/list dev")
	press(m, tea.KeyRight) // [read]
	runAll(m, press(m, tea.KeyEnter))
	if w.unread != 0 || len(f.read) != 1 || f.read[0] != "c1:"+want || m.mode != modeList {
		t.Fatalf("receipts on: unread=%d read=%v mode=%v", w.unread, f.read, m.mode)
	}
	runAll(m, press(m, tea.KeyEnter)) // nothing unread any more: nothing sent
	if len(f.read) != 1 {
		t.Fatalf("a second read sent: %v", f.read)
	}
}

func TestMarkAsReadIsOneReceiptWhateverTheSetting(t *testing.T) {
	f := newFake()
	m := setup(t, f)
	enter(m, "/mark-as-read")
	if !m.isErr || !strings.Contains(m.status, "server window") || len(f.read) != 0 {
		t.Fatalf("server window: %q %v", m.status, f.read)
	}
	runAll(m, enter(m, "/win @alice"))
	_, w := m.winFor("c2")
	w.unread = 3
	want, _ := m.newestID("c2")
	cmd := enter(m, "/markread")
	if cmd == nil || m.isErr || !strings.Contains(m.status, "reporting @alice read") || w.unread != 0 {
		t.Fatalf("mark: cmd=%v status=%q unread=%d", cmd != nil, m.status, w.unread)
	}
	runAll(m, cmd)
	if len(f.read) != 1 || f.read[0] != "c2:"+want || !strings.Contains(m.status, "reported @alice read") {
		t.Fatalf("sent: %v status=%q", f.read, m.status)
	}
	if m.receipts || persistedOn(m) {
		t.Fatal("the setting stays off and nothing is persisted")
	}
	// Again, with nothing unread: still one receipt, on request.
	runAll(m, enter(m, "/mark-as-read"))
	if len(f.read) != 2 {
		t.Fatalf("a second request sends again: %v", f.read)
	}
	// A chat without a known message: an error, nothing sent.
	runAll(m, enter(m, "/win #news")) // fetches its history; then forget it all
	for _, x := range domain.Filter(m.msgs, domain.Query{ChatID: "c3"}) {
		m.msgs = domain.Remove(m.msgs, "c3", x.ID)
	}
	enter(m, "/mark-as-read")
	if !m.isErr || !strings.Contains(m.status, "nothing to report") || len(f.read) != 2 {
		t.Fatalf("no known message: %q %v", m.status, f.read)
	}
	// A service error is reported.
	runAll(m, enter(m, "/win @alice"))
	f.err = errors.New("boom")
	runAll(m, enter(m, "/mark-as-read"))
	if !m.isErr || !strings.Contains(m.status, "read receipt for @alice") || len(f.read) != 3 {
		t.Fatalf("error: %q %v", m.status, f.read)
	}
}
