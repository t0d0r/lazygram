package tui

import (
	"maps"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"lazygram/internal/command"
)

// markUnread sets the [unread] mark on window i from the /win list, as a
// user does, and closes the list again.
func markUnread(t *testing.T, m *Model, i int) {
	t.Helper()
	enter(m, "/win")
	m.listCur = i
	for range 2 { // focus, read, unread
		press(m, tea.KeyRight)
	}
	press(m, tea.KeyEnter)
	press(m, tea.KeyEsc)
	if m.wins[i].unread == 0 || m.mode != modeWin {
		t.Fatalf("window %d marked unread from /win: unread %d, mode %v", i, m.wins[i].unread, m.mode)
	}
}

// TestUnreadGoesToTheNextUnreadWindow: /unread focuses the first window
// with unread messages after the current one, in window order and
// wrapping round, skipping the read ones and window 0; with none it is an
// error and nothing changes.
func TestUnreadGoesToTheNextUnreadWindow(t *testing.T) {
	m := setup(t, newFake()) // 0 server, 1 #news (4), 2 #dev (2), 3 @alice (2)
	for _, want := range []string{"#news", "#dev", "@alice"} {
		from := m.cur
		enter(m, "/unread")
		if m.isErr || m.winName(m.cur) != want || m.lastWindow() != from || m.wins[m.cur].unread != 0 {
			t.Fatalf("/unread from %d: in %s (last %d, unread %d, %q); want %s, last %d",
				from, m.winName(m.cur), m.lastWindow(), m.wins[m.cur].unread, m.status, want, from)
		}
	}
	// None left: an error, and nothing moves.
	enter(m, "/unread")
	if !m.isErr || m.status != "no unread window" || m.winName(m.cur) != "@alice" || m.winName(m.lastWindow()) != "#dev" {
		t.Fatalf("no unread window: %q, in %s, last %s", m.status, m.winName(m.cur), m.winName(m.lastWindow()))
	}
	// It wraps round past window 0, from the last window to #news.
	m.wins[1].unread = 1
	enter(m, "/unread")
	if m.isErr || m.winName(m.cur) != "#news" {
		t.Fatalf("wraps round to #news: %s %q", m.winName(m.cur), m.status)
	}
	// It skips the windows without unread messages: from #news past #dev.
	m.wins[3].unread = 1
	enter(m, "/unread")
	if m.isErr || m.winName(m.cur) != "@alice" {
		t.Fatalf("skips #dev for @alice: %s %q", m.winName(m.cur), m.status)
	}
	// Window 0 is never one, even marked [unread] from /win: the server
	// window tells nobody and keeps no count of its own.
	markUnread(t, m, 0)
	enter(m, "/unread")
	if !m.isErr || m.status != "no unread window" || m.winName(m.cur) != "@alice" {
		t.Fatalf("window 0 is not an unread window: %q, in %s", m.status, m.winName(m.cur))
	}
	// The current window comes last: marked [unread], it is the target only
	// when it is the only one, and /unread then reads it as /win N would.
	markUnread(t, m, 3)
	m.wins[2].unread = 1
	enter(m, "/unread")
	if m.winName(m.cur) != "#dev" || m.wins[3].unread == 0 {
		t.Fatalf("another unread window comes first: in %s, @alice unread %d", m.winName(m.cur), m.wins[3].unread)
	}
	enter(m, "/win 3")
	markUnread(t, m, 3)
	enter(m, "/unread")
	if m.isErr || m.winName(m.cur) != "@alice" || m.wins[3].unread != 0 || m.winName(m.lastWindow()) != "#dev" {
		t.Fatalf("the only one, it is read in place: in %s, unread %d, last %s, %q",
			m.winName(m.cur), m.wins[3].unread, m.winName(m.lastWindow()), m.status)
	}
}

// TestUnreadFocusesAsWinNDoes: /unread is a focus like /win N: the same
// read receipt at once, the same last window, the same counts; and a
// window a Tab showed that /unread leaves within the double Tab's time
// gets its count back and is not reported, as for any focus.
func TestUnreadFocusesAsWinNDoes(t *testing.T) {
	fu, fw := newFake(), newFake()
	byUnread, byWin := setup(t, fu), setup(t, fw)
	for _, m := range []*Model{byUnread, byWin} {
		enter(m, "/read-receipts on")
		runAll(m, enter(m, "/win 1")) // #news read; #dev (2) is next
	}
	runAll(byUnread, enter(byUnread, "/unread"))
	runAll(byWin, enter(byWin, "/win 2"))
	if byUnread.cur != 2 || byUnread.cur != byWin.cur || byUnread.lastWindow() != byWin.lastWindow() ||
		strings.Join(fu.read, " ") != strings.Join(fw.read, " ") || len(fu.read) != 1 || fu.read[0] != "c1:2" {
		t.Fatalf("/unread: in %d, last %d, read %v; /win 2: in %d, last %d, read %v",
			byUnread.cur, byUnread.lastWindow(), fu.read, byWin.cur, byWin.lastWindow(), fw.read)
	}
	for i := range byUnread.wins {
		if byUnread.wins[i].unread != byWin.wins[i].unread {
			t.Fatalf("window %d: unread %d, /win N leaves %d", i, byUnread.wins[i].unread, byWin.wins[i].unread)
		}
	}
	// A Tab shows @alice; /unread right after leaves it for #news, the next
	// unread window after it (round past window 0): @alice was only passed
	// through.
	f := newFake()
	m, step, held := fromDev(t, f)
	enter(m, "/read-receipts on")
	alice := m.wins[3]
	unread, counted := alice.unread, maps.Clone(alice.counted)
	m.wins[1].unread = 1
	press(m, tea.KeyTab) // @alice, its receipt held (its first fetch not run)
	runAll(m, enter(m, "/unread"))
	if m.winName(m.cur) != "#news" || alice.unread != unread || !maps.Equal(alice.counted, counted) || len(f.read) != 0 {
		t.Fatalf("in %s; @alice gets its count back (%d, was %d), nothing sent: %v", m.winName(m.cur), alice.unread, unread, f.read)
	}
	passGap(m, step, held)
	if len(f.read) != 0 {
		t.Fatalf("@alice, passed through, is never reported: %v", f.read)
	}
	// So the next /unread goes on to @alice (#dev was read), and reports it
	// at once.
	runAll(m, enter(m, "/unread"))
	if m.winName(m.cur) != "@alice" || alice.unread != 0 || len(f.read) != 1 || f.read[0] != "c2:7" {
		t.Fatalf("on to @alice, reported: in %s, unread %d, read %v", m.winName(m.cur), alice.unread, f.read)
	}
}

// TestUnreadIsACommandOfItsOwn: one spelling, Tab completes it, /help
// lists it.
func TestUnreadIsACommandOfItsOwn(t *testing.T) {
	for line, ok := range map[string]bool{"/unread": true, "/UNREAD": true, "/unread x": true, "/unr": false, "/next": false} {
		c, err := command.Parse(line)
		if (err == nil && c.Kind == command.Unread) != ok {
			t.Errorf("%q: %+v %v", line, c, err)
		}
	}
	m := setup(t, newFake())
	for _, typed := range []string{"/unr", "/un"} {
		m.input, m.curBack = nil, 0
		typ(m, typed)
		press(m, tea.KeyTab)
		if got := string(m.input); got != "/unread " {
			t.Errorf("Tab after %q: %q", typed, got)
		}
	}
	m.input, m.curBack = nil, 0
	enter(m, "/help")
	drain(m)
	var out []string
	for _, n := range m.wins[0].notes {
		out = append(out, n.text)
	}
	help := strings.Join(out, "\n")
	for _, want := range []string{
		"  /unread                the next window with unread messages after this one, in window order, wrapping round; none: an error",
		"[Act: 1,2] in the status line lists windows with unread messages, and /unread goes to the next of them",
	} {
		if !strings.Contains(help, want) {
			t.Errorf("/help lacks %q", want)
		}
	}
}
