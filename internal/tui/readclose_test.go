package tui

import (
	"errors"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"lazygram/internal/command"
	"lazygram/internal/domain"
)

// The texts of /x (specs/ui.md, Read receipts).
const (
	xArgWant    = "/x takes no argument; it reports this chat window read and closes it"
	xServerWant = "the server window has no chat; /win to a chat first"
)

// inAlice is a model with @alice's window (3, chat c2) current and three
// unread messages counted on it; the windows are 1 #news, 2 #dev,
// 3 @alice.
func inAlice(t *testing.T, f *fake) (*Model, *win) {
	t.Helper()
	m := setup(t, f)
	runAll(m, enter(m, "/win @alice"))
	i, w := m.winFor("c2")
	if i != 3 || m.cur != 3 || winNames(m) != "#news #dev @alice" {
		t.Fatalf("windows: %q, @alice at %d, current %d", winNames(m), i, m.cur)
	}
	w.unread = 3
	f.read, f.marks = nil, nil
	return m, w
}

// closedCount counts the "window closed:" notices of window 0.
func closedCount(m *Model) int {
	n := 0
	for _, s := range noteTexts(m.wins[0]) {
		if strings.HasPrefix(s, "window closed: ") {
			n++
		}
	}
	return n
}

// TestXReportsThenCloses: /x is /mark-as-read, then /close, in that
// order: the receipt for the newest message held goes first, with the
// unread mark cleared, whatever /read-receipts says; the window is still
// open while the report is on its way and when the service is called, and
// closes only when the answer came back without an error, as /close
// closes it (a notice in window 0, the nearest window before it current,
// the layout saved, nothing focused).
func TestXReportsThenCloses(t *testing.T) {
	f := newFake()
	m, w := inAlice(t, f)
	want, _ := m.newestID("c2")
	cmd := enter(m, "/x")
	if cmd == nil || m.isErr || m.status != "reporting @alice read, then closing its window..." {
		t.Fatalf("before the answer: cmd %v, %q (error %v)", cmd != nil, m.status, m.isErr)
	}
	if _, open := m.winFor("c2"); open == nil || m.cur != 3 || w.unread != 0 || len(f.read) != 0 || closedCount(m) != 0 {
		t.Fatalf("before the answer the window is open and read locally: %q, current %d, unread %d, sent %v", winNames(m), m.cur, w.unread, f.read)
	}
	msg := cmd() // the service is told
	if _, open := m.winFor("c2"); open == nil || len(f.read) != 1 || f.read[0] != "c2:"+want {
		t.Fatalf("the receipt goes before the window closes: %q, sent %v, want c2:%s", winNames(m), f.read, want)
	}
	if len(f.marks) != 1 || f.marks[0] != "c2:false" {
		t.Fatalf("/x clears the unread mark, as /mark-as-read: %v", f.marks)
	}
	_, next := m.Update(msg)
	if next != nil {
		t.Fatal("nothing else waits")
	}
	if m.isErr || m.status != "reported @alice read, closed its window" {
		t.Fatalf("after the answer: %q (error %v)", m.status, m.isErr)
	}
	if winNames(m) != "#news #dev" || m.cur != 2 {
		t.Fatalf("left %q, current %d; want #news #dev and #dev current", winNames(m), m.cur)
	}
	if notes := noteTexts(m.wins[0]); notes[len(notes)-1] != "window closed: @alice" || closedCount(m) != 1 {
		t.Fatalf("window 0 notes the closing once: %q", notes)
	}
	if l := storedLayout(t, m); slices.Contains(l.Windows, "c2") || len(l.Windows) != 2 {
		t.Fatalf("the stored layout drops the window: %+v", l)
	}
	if len(f.read) != 1 || len(f.marks) != 1 {
		t.Fatalf("one receipt and one mark, no more: %v %v", f.read, f.marks)
	}
	if m.receipts || persistedOn(m) {
		t.Fatal("the setting stays off and nothing is persisted")
	}
}

// TestXWithReceiptsOnSendsOne: with /read-receipts on /x still sends its
// one receipt, and the window that becomes current by the closing is not
// focused: its unread count stays and no receipt goes for it.
func TestXWithReceiptsOnSendsOne(t *testing.T) {
	f := newFake()
	m, _ := inAlice(t, f)
	enter(m, "/read-receipts on")
	m.wins[2].unread = 5
	runAll(m, enter(m, "/x"))
	if len(f.read) != 1 || !strings.HasPrefix(f.read[0], "c2:") {
		t.Fatalf("one receipt, for @alice: %v", f.read)
	}
	if m.cur != 2 || m.wins[2].unread != 5 {
		t.Fatalf("#dev becomes current unfocused: current %d, unread %d", m.cur, m.wins[2].unread)
	}
}

// TestXFailedReportKeepsTheWindow: when the receipt or the unread mark
// fails, /x says so in red and in window 0, as /mark-as-read does, adds
// that the window is kept, and closes nothing: the windows, the current
// window and the stored layout are as before, and nothing claims a
// report or a closing.
func TestXFailedReportKeepsTheWindow(t *testing.T) {
	for _, c := range []struct {
		name         string
		err, markErr error
		want         string
	}{
		{"receipt", errors.New("boom"), nil, "read receipt for @alice: boom; window kept"},
		{"mark", nil, errors.New("no mark"), "unread mark for @alice: no mark; window kept"},
	} {
		f := newFake()
		m, w := inAlice(t, f)
		planted := layout{Windows: []string{"planted"}, Current: 42}
		if err := m.opt.Store.SetState(layoutKey, planted); err != nil {
			t.Fatal(err)
		}
		wins := slices.Clone(m.wins)
		f.err, f.markErr = c.err, c.markErr
		runAll(m, enter(m, "/x"))
		if !m.isErr || m.status != c.want {
			t.Errorf("%s fails: %q (error %v); want the error %q", c.name, m.status, m.isErr, c.want)
		}
		if strings.Contains(m.status, "reported") || strings.Contains(m.status, "closed") {
			t.Errorf("%s fails: %q claims success", c.name, m.status)
		}
		if !slices.Equal(m.wins, wins) || m.cur != 3 || closedCount(m) != 0 {
			t.Errorf("%s fails: windows %q, current %d, %d closed notices; want nothing closed", c.name, winNames(m), m.cur, closedCount(m))
		}
		if notes := noteTexts(m.wins[0]); notes[len(notes)-1] != "error: "+c.want {
			t.Errorf("%s fails: window 0 collects the error: %q", c.name, notes)
		}
		if l := storedLayout(t, m); l.Current != 42 {
			t.Errorf("%s fails: wrote the stored layout: %+v", c.name, l)
		}
		if len(f.read) != 1 || len(f.marks) != 1 || w.unread != 0 {
			t.Errorf("%s fails: one attempt, the local count cleared: %v %v unread %d", c.name, f.read, f.marks, w.unread)
		}
		// Not retried; a second /x that succeeds closes the window.
		f.err, f.markErr = nil, nil
		runAll(m, enter(m, "/x"))
		if m.isErr || winNames(m) != "#news #dev" || len(f.read) != 2 {
			t.Errorf("after the %s failure a second /x closes: %q, %q, sent %v", c.name, m.status, winNames(m), f.read)
		}
	}
}

// TestXRefusedInWindowZero: window 0 has no chat to report and never
// closes, so /x there is the error of every command that needs a chat
// window; nothing is sent and nothing changes.
func TestXRefusedInWindowZero(t *testing.T) {
	f := wideFake()
	m := wide(t)
	m.opt.Provider = f
	enter(m, "/win 3")
	enter(m, "/win 0")
	u := recordUnchanged(t, m)
	cmd := enter(m, "/x")
	if !m.isErr || m.status != xServerWant || cmd != nil || len(f.read)+len(f.marks) != 0 {
		t.Fatalf("/x in window 0: %q (error %v, cmd %v, sent %v %v); want the error %q", m.status, m.isErr, cmd != nil, f.read, f.marks, xServerWant)
	}
	u.check(t, m, "/x in window 0")
}

// TestXTakesNoArgument: /x is for the current chat window only, so any
// argument, a window number, 0 or a chat name alike, is an error; it is
// checked first, so window 0 gives it too. Nothing is sent, nothing
// closes. Spaces alone are no argument.
func TestXTakesNoArgument(t *testing.T) {
	for _, c := range []struct {
		cur int
		arg string
	}{
		{3, "2"}, {3, "3"}, {3, "0"}, {3, "1-3"}, {3, "2,3"}, {3, "@alice"}, {3, "all"}, {3, "--force"}, {3, "-1"},
		{0, "3"}, {0, "0"},
	} {
		f := newFake()
		m, w := inAlice(t, f)
		if c.cur == 0 {
			enter(m, "/win 0")
		}
		u := recordUnchanged(t, m)
		line := "/x " + c.arg
		cmd := enter(m, line)
		if !m.isErr || m.status != xArgWant || cmd != nil || len(f.read)+len(f.marks) != 0 {
			t.Errorf("%q in window %d: %q (error %v, cmd %v, sent %v %v); want the error %q", line, c.cur, m.status, m.isErr, cmd != nil, f.read, f.marks, xArgWant)
		}
		u.check(t, m, line)
		if w.unread != 3 {
			t.Errorf("%q cleared the unread count", line)
		}
	}
	f := newFake()
	m, _ := inAlice(t, f)
	runAll(m, enter(m, "/x   "))
	if m.isErr || winNames(m) != "#news #dev" || len(f.read) != 1 {
		t.Fatalf("/x with spaces after it: %q, left %q, sent %v", m.status, winNames(m), f.read)
	}
}

// TestXWithoutAKnownMessage: a chat without a known message has nothing
// to report, as for /mark-as-read; /x then sends nothing and closes
// nothing, and the error names /close for closing without a receipt.
func TestXWithoutAKnownMessage(t *testing.T) {
	f := newFake()
	m := setup(t, f)
	runAll(m, enter(m, "/win #news"))
	for _, x := range domain.Filter(m.msgs, domain.Query{ChatID: "c3"}) {
		m.msgs = domain.Remove(m.msgs, "c3", x.ID)
	}
	f.read, f.marks = nil, nil
	u := recordUnchanged(t, m)
	cmd := enter(m, "/x")
	const want = "no message of #news is known yet, nothing to report, window kept; /last fetches some, /close closes it unreported"
	if !m.isErr || m.status != want || cmd != nil || len(f.read)+len(f.marks) != 0 {
		t.Fatalf("no known message: %q (error %v, cmd %v, sent %v %v); want %q", m.status, m.isErr, cmd != nil, f.read, f.marks, want)
	}
	u.check(t, m, "/x without a known message")
}

// TestXClosesItsWindowWhereverItIs: the window /x was typed in closes
// when the answer comes, also when another window is current by then,
// which stays current; a window closed by hand meanwhile is not closed
// twice, and the status says so.
func TestXClosesItsWindowWhereverItIs(t *testing.T) {
	f := newFake()
	m, _ := inAlice(t, f)
	cmd := enter(m, "/x")
	enter(m, "/win 1")
	news := m.wins[1]
	runAll(m, cmd)
	if m.isErr || m.status != "reported @alice read, closed its window" || winNames(m) != "#news #dev" || m.wins[m.cur] != news {
		t.Fatalf("answer with #news current: %q, left %q, current %d", m.status, winNames(m), m.cur)
	}

	f = newFake()
	m, _ = inAlice(t, f)
	cmd = enter(m, "/x")
	enter(m, "/close")
	if closedCount(m) != 1 {
		t.Fatalf("/close while the report is on its way closes: %q", noteTexts(m.wins[0]))
	}
	runAll(m, cmd)
	if m.isErr || m.status != "reported @alice read; its window was closed already" || winNames(m) != "#news #dev" || closedCount(m) != 1 {
		t.Fatalf("answer with the window gone: %q, left %q, %d closed notices", m.status, winNames(m), closedCount(m))
	}
	// A failure with the window gone is the plain error: nothing is kept.
	f = newFake()
	m, _ = inAlice(t, f)
	f.err = errors.New("boom")
	cmd = enter(m, "/x")
	enter(m, "/close")
	runAll(m, cmd)
	if !m.isErr || m.status != "read receipt for @alice: boom" {
		t.Fatalf("failure with the window gone: %q (error %v)", m.status, m.isErr)
	}
}

// TestXKeepsAWindowWithANewerMessage: the receipt covers the newest
// message held when /x was typed. One arriving before the answer is not
// covered, so the window stays open to show it; the status tells, as
// information: the report itself went through.
func TestXKeepsAWindowWithANewerMessage(t *testing.T) {
	f := newFake()
	m, _ := inAlice(t, f)
	sent, _ := m.newestID("c2")
	cmd := enter(m, "/x")
	f.msgs = append(f.msgs, mk("c2", "9", "alice", "one more thing", 5))
	m.Update(m.syncCmd()())
	if id, _ := m.newestID("c2"); id != "9" {
		t.Fatalf("the new message is held: newest %q", id)
	}
	runAll(m, cmd)
	if m.isErr || m.status != "reported @alice read; a newer message arrived, window kept" {
		t.Fatalf("a newer message: %q (error %v)", m.status, m.isErr)
	}
	if winNames(m) != "#news #dev @alice" || m.cur != 3 || closedCount(m) != 0 || len(f.read) != 1 || f.read[0] != "c2:"+sent {
		t.Fatalf("the window stays: %q, current %d, sent %v", winNames(m), m.cur, f.read)
	}
}

// TestXWaitsItsTurn: requested reports for one chat go one at a time. A
// /x typed while one is on its way waits and closes after its own
// answer, not the earlier one's; a later /mark-as-read that replaces the
// waiting /x keeps the closing; a later [unread] replaces it and the
// closing goes with it, since the last request decides.
func TestXWaitsItsTurn(t *testing.T) {
	f := newFake()
	m, _ := inAlice(t, f)
	first := enter(m, "/mark-as-read")
	if cmd := enter(m, "/x"); cmd != nil {
		t.Fatal("/x waits for the report on its way")
	}
	if m.isErr || m.status != "reporting @alice read, then closing its window..." {
		t.Fatalf("a waiting /x: %q", m.status)
	}
	_, next := m.Update(first())
	if next == nil || winNames(m) != "#news #dev @alice" || m.status != "reported @alice read" {
		t.Fatalf("the earlier answer closes nothing and sends the waiting /x: next %v, %q, %q", next != nil, winNames(m), m.status)
	}
	runAll(m, next)
	if winNames(m) != "#news #dev" || len(f.read) != 2 || m.status != "reported @alice read, closed its window" {
		t.Fatalf("its own answer closes: %q, sent %v, %q", winNames(m), f.read, m.status)
	}

	f = newFake()
	m, _ = inAlice(t, f)
	first = enter(m, "/mark-as-read")
	enter(m, "/x")
	enter(m, "/mark-as-read") // replaces the waiting /x's report, not its closing
	_, next = m.Update(first())
	runAll(m, next)
	if winNames(m) != "#news #dev" || len(f.read) != 2 {
		t.Fatalf("a later /mark-as-read keeps the closing: %q, sent %v", winNames(m), f.read)
	}

	f = newFake()
	m, _ = inAlice(t, f)
	first = enter(m, "/mark-as-read")
	enter(m, "/x")
	enter(m, "/list alice")
	press(m, tea.KeyRight) // [read]
	press(m, tea.KeyRight) // [unread]
	if cmd := press(m, tea.KeyEnter); cmd != nil {
		t.Fatal("[unread] waits too")
	}
	_, next = m.Update(first())
	runAll(m, next)
	if strings.Join(f.marks, ",") != "c2:false,c2:true" || !strings.Contains(m.status, "reported @alice unread") {
		t.Fatalf("the last request decides: marks %v, %q", f.marks, m.status)
	}
	if _, open := m.winFor("c2"); open == nil || closedCount(m) != 0 {
		t.Fatalf("a chat marked unread keeps its window: %q", winNames(m))
	}
}

// TestMarkAsReadAndCloseStayApart: /mark-as-read alone still closes
// nothing and /close alone still reports nothing.
func TestMarkAsReadAndCloseStayApart(t *testing.T) {
	f := newFake()
	m, _ := inAlice(t, f)
	runAll(m, enter(m, "/mark-as-read"))
	if winNames(m) != "#news #dev @alice" || m.cur != 3 || m.status != "reported @alice read" || len(f.read) != 1 {
		t.Fatalf("/mark-as-read: %q, %q, sent %v", m.status, winNames(m), f.read)
	}
	runAll(m, enter(m, "/close"))
	if winNames(m) != "#news #dev" || m.status != "closed @alice" || len(f.read) != 1 || len(f.marks) != 1 {
		t.Fatalf("/close: %q, %q, sent %v %v", m.status, winNames(m), f.read, f.marks)
	}
}

// TestHelpNamesX: /help lists /x on a line of its own, right after
// /mark-as-read, saying what it does in order, what a failure does and
// that it takes no argument.
func TestHelpNamesX(t *testing.T) {
	lines := strings.Split(command.HelpText, "\n")
	at := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "  /x ") {
			at = i
		}
	}
	if at < 1 || !strings.HasPrefix(lines[at-1], "  /mark-as-read ") {
		t.Fatalf("/help has no /x line right after /mark-as-read: at %d", at)
	}
	for _, want := range []string{"as /mark-as-read, then close the window", "a failed report keeps it open", "refused in window 0", "takes no argument"} {
		if !strings.Contains(lines[at], want) {
			t.Errorf("/help's /x line lacks %q: %q", want, lines[at])
		}
	}
	m := setup(t, newFake())
	enter(m, "/help")
	drain(m)
	if !slices.Contains(noteTexts(m.wins[0]), lines[at]) {
		t.Errorf("the printed /help lacks the /x entry %q", lines[at])
	}
}

// TestTabCompletesX: Tab completes /x to itself with a space, in either
// case, and offers nothing after it: it takes no argument, so no chat
// names and no numbers.
func TestTabCompletesX(t *testing.T) {
	m := setup(t, newFake())
	for typed, want := range map[string]string{
		"/x":    "/x ",
		"/X":    "/x ",
		"/x ":   "/x ",
		"/x al": "/x al",
		"/x @":  "/x @",
	} {
		m.input, m.comp = nil, nil
		typ(m, typed)
		press(m, tea.KeyTab)
		if got := string(m.input); got != want {
			t.Errorf("%q: got %q, want %q", typed, got, want)
		}
	}
}

// TestHintNamesX: a start hint makes /x known; read in window 0, which
// refuses it, it says it works in a chat window (TestHintsAreWellFormed
// checks the form).
func TestHintNamesX(t *testing.T) {
	for _, h := range hintPool {
		if strings.Contains(h, "/x ") && strings.Contains(h, "chat window") {
			return
		}
	}
	t.Fatal("no start hint names /x")
}
