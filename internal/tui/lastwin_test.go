package tui

import (
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lazygram/internal/domain"
)

// holdTicks keeps what the model schedules with after (a Tab's held read
// receipt) instead of starting a timer, so nothing sleeps; passGap
// delivers it. Whatever is held is held for the double Tab's time as it
// is set at that moment (m.tabGap).
func holdTicks(t *testing.T, m *Model) *[]tea.Msg {
	t.Helper()
	held := &[]tea.Msg{}
	m.after = func(d time.Duration, msg tea.Msg) tea.Cmd {
		if d != m.tabGap {
			t.Errorf("held for %v, want the double Tab's time %v", d, m.tabGap)
		}
		*held = append(*held, msg)
		return nil
	}
	return held
}

// passGap moves the clock on by the double Tab's time and delivers the
// held messages, as their timers would, running whatever they start.
func passGap(m *Model, step func(time.Duration), held *[]tea.Msg) {
	step(m.tabGap)
	msgs := *held
	*held = nil
	for _, msg := range msgs {
		_, cmd := m.Update(msg)
		runAll(m, cmd)
	}
}

// fromDev is a model in #dev (window 2) with #news (window 1) the last
// focused window, so that from there a Tab shows @alice (3), a plain
// second Tab the server window (0) and a double Tab #news: three
// different windows. It sets /doubletab last, as the tests of that
// gesture need; fromDevWith sets any mode.
func fromDev(t *testing.T, f *fake) (*Model, func(time.Duration), *[]tea.Msg) {
	t.Helper()
	return fromDevWith(t, f, "last")
}

// fromDevWith is fromDev with /doubletab set to mode: off, last or unread.
// #news and #dev are read; @alice has 2 unread messages.
func fromDevWith(t *testing.T, f *fake, mode string) (*Model, func(time.Duration), *[]tea.Msg) {
	t.Helper()
	m := setup(t, f) // 0 server, 1 #news, 2 #dev, 3 @alice
	step := pinClock(m)
	held := holdTicks(t, m)
	enter(m, "/doubletab "+mode)
	enter(m, "/win 1")
	enter(m, "/win 2")
	if m.cur != 2 || m.lastWindow() != 1 || m.dblTab.String() != mode || m.wins[3].unread != 2 {
		t.Fatalf("in #dev after #news, double Tab %s: cur %d, last %d, mode %v, @alice unread %d",
			mode, m.cur, m.lastWindow(), m.dblTab, m.wins[3].unread)
	}
	return m, step, held
}

// allModes runs a test of what holds whatever /doubletab says: off, last
// and unread.
func allModes(t *testing.T, test func(t *testing.T, mode string)) {
	t.Helper()
	for _, mode := range doubleTabWords {
		t.Run("doubletab "+mode, func(t *testing.T) { test(t, mode) })
	}
}

// walkModes runs a test of a second fast Tab that leaves the window the
// first one showed: off (the next window) and last (the last window). With
// unread the second Tab stays on that window when it has unread messages
// (TestDoubleTabUnread...).
func walkModes(t *testing.T, test func(t *testing.T, mode string)) {
	t.Helper()
	for _, mode := range []string{"off", "last"} {
		t.Run("doubletab "+mode, func(t *testing.T) { test(t, mode) })
	}
}

func TestWinLastSwitchesAndToggles(t *testing.T) {
	f := newFake()
	f.chats = append(f.chats, domain.Chat{ID: "c5", Alias: "last", Title: "Last", Kind: domain.ChatUser})
	m := setup(t, f)
	enter(m, "/win last")
	if !m.isErr || m.status != "no last window" || m.cur != 0 {
		t.Fatalf("no window was left yet: %q, cur %d", m.status, m.cur)
	}
	enter(m, "/win 2")
	for i, line := range []string{"/win last", "/w LAST", "/j last", "/win last"} {
		want := []int{0, 2, 0, 2}[i]
		enter(m, line)
		if m.isErr || m.cur != want {
			t.Fatalf("%q toggles between window 2 and window 0: cur %d, %q", line, m.cur, m.status)
		}
	}
	// "last" is a word of /win, as "list" is: a chat named last takes its sigil.
	enter(m, "/win @last")
	if m.isErr || m.winName(m.cur) != "@last" {
		t.Fatalf("/win @last opens the chat: %s %q", m.winName(m.cur), m.status)
	}
	enter(m, "/win last")
	if m.cur != 2 {
		t.Fatalf("and /win last still goes back: cur %d", m.cur)
	}
	// Session only: a new start has none.
	again := New(Options{Store: m.opt.Store, Provider: f})
	enter(again, "/win last")
	if !again.isErr || again.status != "no last window" {
		t.Fatalf("the last window is not kept: %q", again.status)
	}
}

func TestLastWindowGoesWithItsWindow(t *testing.T) {
	m := setup(t, newFake()) // 0 server, 1 #news, 2 #dev, 3 @alice
	enter(m, "/win 2")
	enter(m, "/win 3")
	enter(m, "/close 2") // #dev, the last window
	enter(m, "/win last")
	if !m.isErr || m.status != "no last window" || m.winName(m.cur) != "@alice" {
		t.Fatalf("a closed last window is none: %q, %s", m.status, m.winName(m.cur))
	}
	// Above, the current window took the closed one's place; here another
	// does, and it is still not the last window.
	other := setup(t, newFake()) // 0 server, 1 #news, 2 #dev, 3 @alice
	enter(other, "/win 1")
	enter(other, "/win 3")
	enter(other, "/close 1") // #news, the last window: #dev takes its number
	enter(other, "/win last")
	if !other.isErr || other.status != "no last window" || other.winName(other.cur) != "@alice" {
		t.Fatalf("whatever takes its number, a closed last window is none: %q, %s", other.status, other.winName(other.cur))
	}
	// Closing the current window can fall back on the last one: none either.
	enter(m, "/win 1") // #news, after @alice
	enter(m, "/win 2") // @alice, after #news
	enter(m, "/close") // @alice: #news is current again
	enter(m, "/win last")
	if !m.isErr || m.status != "no last window" || m.winName(m.cur) != "#news" {
		t.Fatalf("the current window is not the last one: %q, %s", m.status, m.winName(m.cur))
	}
	// Window 0 counts like any window.
	enter(m, "/win 0")
	enter(m, "/win 1")
	enter(m, "/win last")
	if m.isErr || m.cur != 0 {
		t.Fatalf("back to the server window: cur %d, %q", m.cur, m.status)
	}
}

func TestLastWindowFollowsMoveAndClose(t *testing.T) {
	m := setup(t, newFake()) // 0 server, 1 #news, 2 #dev, 3 @alice
	enter(m, "/win 2")
	enter(m, "/win 3")
	enter(m, "/move 1") // @alice 1, #news 2, #dev 3
	enter(m, "/win last")
	if m.isErr || m.winName(m.cur) != "#dev" || m.cur != 3 {
		t.Fatalf("/move keeps the last window: %s at %d, %q", m.winName(m.cur), m.cur, m.status)
	}
	enter(m, "/win 2")   // #news, #dev (window 3) last
	enter(m, "/close 1") // @alice: #news 1, #dev 2
	enter(m, "/win last")
	if m.isErr || m.winName(m.cur) != "#dev" || m.cur != 2 {
		t.Fatalf("closing another window keeps it: %s at %d, %q", m.winName(m.cur), m.cur, m.status)
	}
}

// TestEveryFocusSetsTheLastWindow: whatever way a window is focused, the
// window it left is the last one.
func TestEveryFocusSetsTheLastWindow(t *testing.T) {
	for _, c := range []struct {
		name string
		from int
		do   func(m *Model)
		to   string
	}{
		{"Tab", 2, func(m *Model) { press(m, tea.KeyTab) }, "@alice"},
		{"Shift-Tab", 2, func(m *Model) { press(m, tea.KeyShiftTab) }, "#news"},
		{"Ctrl-N", 2, func(m *Model) { press(m, tea.KeyCtrlN) }, "@alice"},
		{"Alt-N", 2, func(m *Model) { m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}, Alt: true}) }, "#news"},
		{"Esc N", 2, func(m *Model) { press(m, tea.KeyEsc); typ(m, "3") }, "@alice"},
		{"/win N", 2, func(m *Model) { enter(m, "/win 0") }, "server"},
		{"/win name", 2, func(m *Model) { enter(m, "/win @alice") }, "@alice"},
		{"/msg", 2, func(m *Model) { enter(m, "/msg #news hi") }, "#news"},
		{"window tag click", 2, func(m *Model) {
			m.View()
			x, y := find(m, spanWin, "1")
			click(m, x, y)
		}, "#news"},
		{"chat name click", 0, func(m *Model) {
			m.View()
			x, y := find(m, spanChat, "c2")
			click(m, x, y)
		}, "@alice"},
		{"/win list Enter", 2, func(m *Model) { enter(m, "/win"); press(m, tea.KeyUp); press(m, tea.KeyEnter) }, "#news"},
		{"/list Enter", 2, func(m *Model) { enter(m, "/list alice"); press(m, tea.KeyEnter) }, "@alice"},
		{"/help", 2, func(m *Model) { enter(m, "/help") }, "server"},
		{"/keys", 2, func(m *Model) { enter(m, "/keys") }, "server"},
		{"/settings", 2, func(m *Model) { enter(m, "/settings") }, "server"},
	} {
		t.Run(c.name, func(t *testing.T) {
			m := setup(t, newFake()) // 0 server, 1 #news, 2 #dev, 3 @alice
			pinClock(m)
			m.switchTo(c.from)
			c.do(m)
			if m.winName(m.cur) != c.to || m.lastWindow() != c.from {
				t.Fatalf("in %s with window %d last; want %s with %d last", m.winName(m.cur), m.lastWindow(), c.to, c.from)
			}
		})
	}
}

func TestDoubleTabGoesToTheLastWindowAndBack(t *testing.T) {
	m, step, _ := fromDev(t, newFake())
	press(m, tea.KeyTab)
	if m.winName(m.cur) != "@alice" {
		t.Fatalf("the first Tab switches at once: %s", m.winName(m.cur))
	}
	step(100 * time.Millisecond)
	press(m, tea.KeyTab)
	if m.winName(m.cur) != "#news" || m.winName(m.lastWindow()) != "#dev" {
		t.Fatalf("the double Tab goes to the last window before it, and #dev is the last one now: %s, last %s",
			m.winName(m.cur), m.winName(m.lastWindow()))
	}
	// Again, later: back to #dev. Its first Tab shows #dev itself, which
	// the second one then keeps.
	step(time.Second)
	press(m, tea.KeyTab)
	step(100 * time.Millisecond)
	press(m, tea.KeyTab)
	if m.winName(m.cur) != "#dev" || m.winName(m.lastWindow()) != "#news" {
		t.Fatalf("a second double Tab toggles back: %s, last %s", m.winName(m.cur), m.winName(m.lastWindow()))
	}
	// And once more, from #dev, through @alice again.
	step(time.Second)
	press(m, tea.KeyTab)
	step(100 * time.Millisecond)
	press(m, tea.KeyTab)
	if m.winName(m.cur) != "#news" {
		t.Fatalf("and back to #news: %s", m.winName(m.cur))
	}
}

func TestDoubleTabTiming(t *testing.T) {
	m, step, _ := fromDev(t, newFake())
	press(m, tea.KeyTab)
	step(m.tabGap)
	press(m, tea.KeyTab)
	if m.winName(m.cur) != "#news" || m.wins[3].unread != 2 {
		t.Fatalf("tabGap exactly is still a double Tab, and @alice gets its count back: %s, unread %d", m.winName(m.cur), m.wins[3].unread)
	}
	m, step, _ = fromDev(t, newFake())
	press(m, tea.KeyTab)
	step(m.tabGap + time.Millisecond)
	press(m, tea.KeyTab)
	if m.cur != 0 || m.winName(m.lastWindow()) != "@alice" || m.wins[3].unread != 0 {
		t.Fatalf("a longer pause makes two plain Tabs, and @alice was read: cur %d, last %s, unread %d",
			m.cur, m.winName(m.lastWindow()), m.wins[3].unread)
	}
}

func TestDoubleTabIsBrokenByAnythingBetween(t *testing.T) {
	m, _, _ := fromDev(t, newFake())
	press(m, tea.KeyTab)
	press(m, tea.KeyEnd) // a key that changes nothing else
	press(m, tea.KeyTab)
	if m.cur != 0 {
		t.Fatalf("a key between: two plain Tabs, cur %d", m.cur)
	}
	m, _, _ = fromDev(t, newFake())
	press(m, tea.KeyTab)
	m.View()
	x, y := find(m, spanWin, "2") // back to #dev with a click
	click(m, x, y)
	press(m, tea.KeyTab)
	if m.winName(m.cur) != "@alice" {
		t.Fatalf("a focus between: a plain Tab, %s", m.winName(m.cur))
	}
	// Shift-Tab is never part of one: after Tab it is the previous window,
	// and a Tab after it is a first Tab.
	m, _, _ = fromDev(t, newFake())
	press(m, tea.KeyTab)
	press(m, tea.KeyShiftTab)
	if m.winName(m.cur) != "#dev" {
		t.Fatalf("Shift-Tab after Tab: the previous window, %s", m.winName(m.cur))
	}
	press(m, tea.KeyTab)
	if m.winName(m.cur) != "@alice" {
		t.Fatalf("Tab after Shift-Tab is a first Tab: %s", m.winName(m.cur))
	}
	m, _, _ = fromDev(t, newFake())
	press(m, tea.KeyShiftTab) // #news, #dev last
	press(m, tea.KeyTab)
	if m.winName(m.cur) != "#dev" || m.winName(m.lastWindow()) != "#news" {
		t.Fatalf("Shift-Tab then Tab: two plain moves, %s, last %s", m.winName(m.cur), m.winName(m.lastWindow()))
	}
}

func TestDoubleTabWithTextCompletes(t *testing.T) {
	m, _, _ := fromDev(t, newFake())
	typ(m, "/w")
	press(m, tea.KeyTab)
	press(m, tea.KeyTab) // the second Tab lists, as before
	if m.cur != 2 || string(m.input) != "/w" || !strings.Contains(body(m), "/whois") {
		t.Fatalf("with text Tab completes: cur %d, input %q", m.cur, string(m.input))
	}
	m.input = nil
	typ(m, "/win la")
	press(m, tea.KeyTab)
	if string(m.input) != "/win last " {
		t.Fatalf("Tab completes last after /win: %q", string(m.input))
	}
}

func TestDoubleTabWithoutLastWindowIsTwoTabs(t *testing.T) {
	m := setup(t, newFake()) // 0 server, 1 #news, 2 #dev, 3 @alice; nothing left yet
	pinClock(m)
	enter(m, "/doubletab last")
	var got []int
	for range 4 {
		press(m, tea.KeyTab)
		got = append(got, m.cur)
	}
	// The second Tab is a plain one and arms nothing: the third is a first
	// Tab, and the fourth goes to the last window before the third (#news,
	// which the second Tab left).
	if want := []int{1, 2, 3, 1}; !slices.Equal(got, want) {
		t.Fatalf("Tabs went to %v, want %v", got, want)
	}
}

func TestThirdFastTabIsAFirstTab(t *testing.T) {
	m := setup(t, newFake()) // 0 server, 1 #news, 2 #dev, 3 @alice
	step := pinClock(m)
	enter(m, "/doubletab last")
	enter(m, "/win 3")
	enter(m, "/win 1") // in #news, @alice last
	var got []string
	for range 4 {
		press(m, tea.KeyTab)
		got = append(got, m.winName(m.cur))
		step(100 * time.Millisecond)
	}
	// #dev, then the double Tab's @alice; the third hops on to the server
	// window at once and the fourth goes back to #news, the last window
	// before the third.
	if strings.Join(got, " ") != "#dev @alice server #news" {
		t.Fatalf("Tabs went to %v", got)
	}
}

func TestDoubleTabFromTheSettingsMenu(t *testing.T) {
	m, step, _ := fromDev(t, newFake())
	enter(m, "/settings") // window 0, #dev last
	press(m, tea.KeyTab)
	if m.winName(m.cur) != "#news" || m.settingsOpen() {
		t.Fatalf("Tab leaves the menu for the next window: %s", m.winName(m.cur))
	}
	step(100 * time.Millisecond)
	press(m, tea.KeyTab)
	if m.winName(m.cur) != "#dev" {
		t.Fatalf("the double Tab goes to the last window before the menu's: %s", m.winName(m.cur))
	}
}

// TestDoubleTabHopLeavesNoTrace: the window the first Tab passed through
// gets its unread count back, and no receipt is sent for it.
func TestDoubleTabHopLeavesNoTrace(t *testing.T) {
	f := newFake()
	m, step, held := fromDev(t, f)
	enter(m, "/read-receipts on")
	alice := m.wins[3]
	if alice.unread != 2 || len(alice.counted) != 2 {
		t.Fatalf("@alice starts with 2 unread: %d %v", alice.unread, alice.counted)
	}
	runAll(m, press(m, tea.KeyTab))
	if m.wins[m.cur] != alice || alice.unread != 0 || len(f.read) != 0 || len(*held) != 1 {
		t.Fatalf("the first Tab shows @alice and holds its receipt back: unread %d, read %v, held %d", alice.unread, f.read, len(*held))
	}
	// Its first fetch ran meanwhile, and the service no longer has c2:7: the
	// count that comes back is the one message still there.
	if !m.gone[domain.Message{ChatID: "c2", ID: "7"}.Key()] {
		t.Fatal("the fetch drops c2:7")
	}
	step(100 * time.Millisecond)
	press(m, tea.KeyTab)
	want := map[string]bool{domain.Message{ChatID: "c2", ID: "1"}.Key(): true}
	if m.winName(m.cur) != "#news" || alice.unread != 1 || !maps.Equal(alice.counted, want) || !strings.Contains(m.View(), "Act: 3") {
		t.Fatalf("the hop is undone: in %s, @alice unread %d counted %v", m.winName(m.cur), alice.unread, alice.counted)
	}
	passGap(m, step, held)
	if len(f.read) != 0 {
		t.Fatalf("a window passed through is not reported read: %v", f.read)
	}
	// From #news the first Tab shows #dev, the last window itself: the
	// double Tab stays there, read, and its receipt goes.
	step(time.Second)
	dev := m.wins[2]
	dev.unread = 1
	runAll(m, press(m, tea.KeyTab))
	step(100 * time.Millisecond)
	runAll(m, press(m, tea.KeyTab))
	if m.wins[m.cur] != dev || dev.unread != 0 || len(f.read) != 0 {
		t.Fatalf("in #dev, read, the receipt still held: unread %d, read %v", dev.unread, f.read)
	}
	passGap(m, step, held)
	if want, _ := m.newestID("c1"); len(f.read) != 1 || f.read[0] != "c1:"+want {
		t.Fatalf("#dev stayed: reported once tabGap passed: %v", f.read)
	}
}

// TestDoubleTabHoldsItsDestinationsReceipt: the window a double Tab goes
// to was focused by a Tab too; its receipt waits tabGap like a single
// Tab's, and the window passed through is never reported.
func TestDoubleTabHoldsItsDestinationsReceipt(t *testing.T) {
	f := newFake()
	m := setup(t, f) // 0 server, 1 #news, 2 #dev, 3 @alice
	step := pinClock(m)
	held := holdTicks(t, m)
	enter(m, "/doubletab last")
	enter(m, "/win 3")
	enter(m, "/win 1") // in #news, @alice last
	enter(m, "/read-receipts on")
	m.wins[2].unread, m.wins[3].unread = 1, 2
	runAll(m, press(m, tea.KeyTab)) // #dev
	step(100 * time.Millisecond)
	runAll(m, press(m, tea.KeyTab)) // @alice
	if m.winName(m.cur) != "@alice" || len(f.read) != 0 || m.wins[2].unread != 1 {
		t.Fatalf("in %s, nothing sent yet, #dev unread again: read %v, unread %d", m.winName(m.cur), f.read, m.wins[2].unread)
	}
	passGap(m, step, held)
	if len(f.read) != 1 || f.read[0] != "c2:7" {
		t.Fatalf("only @alice is reported, tabGap later: %v", f.read)
	}
}

// TestTabReceiptWaitsForTabGap: a window a single Tab focused is reported
// read once it stayed current for tabGap; every other way of focusing
// reports at once. Whatever /doubletab says.
func TestTabReceiptWaitsForTabGap(t *testing.T) { allModes(t, testTabReceiptWaitsForTabGap) }

func testTabReceiptWaitsForTabGap(t *testing.T, mode string) {
	f := newFake()
	m, step, held := fromDevWith(t, f, mode)
	enter(m, "/read-receipts on")
	runAll(m, press(m, tea.KeyTab)) // @alice, 2 unread
	if len(f.read) != 0 || len(*held) != 1 {
		t.Fatalf("not at once: read %v, held %d", f.read, len(*held))
	}
	passGap(m, step, held)
	if len(f.read) != 1 || f.read[0] != "c2:7" {
		t.Fatalf("tabGap later it is sent: %v", f.read)
	}
	for _, c := range []struct {
		name string
		to   int
		key  tea.KeyMsg
	}{
		{"Shift-Tab", 2, tea.KeyMsg{Type: tea.KeyShiftTab}},
		{"Alt-N", 3, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}, Alt: true}},
		{"Ctrl-P", 2, tea.KeyMsg{Type: tea.KeyCtrlP}},
	} {
		m.wins[c.to].unread = 1
		before := len(f.read)
		_, cmd := m.Update(c.key)
		runAll(m, cmd)
		if m.cur != c.to || len(f.read) != before+1 || len(*held) != 0 {
			t.Fatalf("%s reports at once: cur %d, read %v, held %d", c.name, m.cur, f.read, len(*held))
		}
	}
	m.wins[3].unread = 1
	before := len(f.read)
	runAll(m, enter(m, "/win 3"))
	if len(f.read) != before+1 || len(*held) != 0 {
		t.Fatalf("/win N reports at once: %v", f.read)
	}
}

// TestHeldReceiptNeverGoesTwice: the held receipt goes only when its
// window was not left in between: off and last, whose second Tab leaves
// it.
func TestHeldReceiptNeverGoesTwice(t *testing.T) {
	walkModes(t, testHeldReceiptNeverGoesTwice)
}

func testHeldReceiptNeverGoesTwice(t *testing.T, mode string) {
	f := newFake()
	m, step, held := fromDevWith(t, f, mode)
	enter(m, "/read-receipts on")
	press(m, tea.KeyTab) // @alice, receipt held
	step(100 * time.Millisecond)
	press(m, tea.KeyTab) // back to #news (last) or on to the server window (off): @alice unread again
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}, Alt: true})
	runAll(m, cmd) // @alice again, reported at once
	if len(f.read) != 1 {
		t.Fatalf("Alt-3 reports @alice: %v", f.read)
	}
	passGap(m, step, held)
	if len(f.read) != 1 {
		t.Fatalf("the Tab's held receipt does not go as well: %v", f.read)
	}
}

// TestHeldReceiptNeverGoesForAClosedWindow: nor when its window was
// closed. Whatever /doubletab says.
func TestHeldReceiptNeverGoesForAClosedWindow(t *testing.T) {
	allModes(t, testHeldReceiptNeverGoesForAClosedWindow)
}

func testHeldReceiptNeverGoesForAClosedWindow(t *testing.T, mode string) {
	f := newFake()
	m, step, held := fromDevWith(t, f, mode)
	enter(m, "/read-receipts on")
	press(m, tea.KeyTab) // @alice, receipt held
	enter(m, "/close")   // gone before tabGap
	step(m.tabGap + time.Millisecond)
	runAll(m, altN(m, '1')) // and the window shown since is left later
	passGap(m, step, held)
	if len(f.read) != 0 {
		t.Fatalf("a closed window is not reported: %v", f.read)
	}
}

// altN presses Alt and a window number.
func altN(m *Model, n rune) tea.Cmd {
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{n}, Alt: true})
	return cmd
}

// TestFocusingTheCurrentWindowKeepsTheLastOne: only a focus that changes
// the window sets the last one; focusing the window already shown keeps
// it.
func TestFocusingTheCurrentWindowKeepsTheLastOne(t *testing.T) {
	for _, c := range []struct {
		name   string
		server bool // from window 0 (#dev last) instead of #dev (#news last)
		do     func(m *Model)
	}{
		{"Alt-N", false, func(m *Model) { altN(m, '2') }},
		{"/win N", false, func(m *Model) { enter(m, "/win 2") }},
		{"/win name", false, func(m *Model) { enter(m, "/win #dev") }},
		{"window tag click", false, func(m *Model) {
			m.View()
			x, y := find(m, spanWin, "2")
			click(m, x, y)
		}},
		{"/help", true, func(m *Model) { enter(m, "/help") }},
		{"F1", true, func(m *Model) { press(m, tea.KeyF1) }},
		{"/keys", true, func(m *Model) { enter(m, "/keys") }},
		{"/settings", true, func(m *Model) { enter(m, "/settings") }},
	} {
		t.Run(c.name, func(t *testing.T) {
			m, _, _ := fromDev(t, newFake())
			want := 1
			if c.server {
				enter(m, "/win 0")
				want = 2
			}
			c.do(m)
			if m.lastWindow() != want {
				t.Fatalf("in %s with window %d last; want %d", m.winName(m.cur), m.lastWindow(), want)
			}
		})
	}
}

// TestDoubleTabSurvivesAClickOnTheCurrentWindow: a click that focuses the
// window already shown is no focus change, and does not break a double
// Tab.
func TestDoubleTabSurvivesAClickOnTheCurrentWindow(t *testing.T) {
	m, step, _ := fromDev(t, newFake())
	press(m, tea.KeyTab) // @alice
	m.View()
	x, y := find(m, spanWin, "3") // @alice's own tag
	click(m, x, y)
	step(100 * time.Millisecond)
	press(m, tea.KeyTab)
	if m.winName(m.cur) != "#news" {
		t.Fatalf("still a double Tab: %s", m.winName(m.cur))
	}
}

// TestDeletionDuringTheHopIsNotPutBack: a message deleted while a Tab
// shows its window is not counted again when the hop is undone, so the
// count still goes with the window's messages.
func TestDeletionDuringTheHopIsNotPutBack(t *testing.T) {
	m, step, _ := fromDev(t, newFake())
	alice := m.wins[3]
	press(m, tea.KeyTab) // @alice; its first fetch is not run
	runAll(m, m.applyChange(domain.Change{Kind: domain.Deleted, ChatID: "c2", IDs: []string{"7"}}))
	step(100 * time.Millisecond)
	press(m, tea.KeyTab) // #news
	want := map[string]bool{domain.Message{ChatID: "c2", ID: "1"}.Key(): true}
	if m.winName(m.cur) != "#news" || alice.unread != 1 || !maps.Equal(alice.counted, want) {
		t.Fatalf("only the message still there comes back: in %s, @alice unread %d, counted %v",
			m.winName(m.cur), alice.unread, alice.counted)
	}
	runAll(m, m.applyChange(domain.Change{Kind: domain.Deleted, ChatID: "c2", IDs: []string{"1"}}))
	if alice.unread != 0 || len(alice.counted) != 0 || strings.Contains(m.View(), "Act: 3") {
		t.Fatalf("the last one takes the mark with it: unread %d, counted %v", alice.unread, alice.counted)
	}
}

// TestPlainSecondTabLeavesNoTraceEither: with no last window (or with
// /doubletab off) the second fast Tab is a plain one, and the window the
// first showed is left as it was all the same; focused again, it is
// reported as that focus reports.
func TestPlainSecondTabLeavesNoTraceEither(t *testing.T) {
	walkModes(t, testPlainSecondTabLeavesNoTraceEither)
}

func testPlainSecondTabLeavesNoTraceEither(t *testing.T, mode string) {
	f := newFake()
	m, step, held := fromDevWith(t, f, mode)
	enter(m, "/close 1") // #news, the last window: none now; 0 server, 1 #dev, 2 @alice
	enter(m, "/read-receipts on")
	alice := m.wins[2]
	unread, counted := alice.unread, maps.Clone(alice.counted)
	press(m, tea.KeyTab) // @alice, receipt held
	step(100 * time.Millisecond)
	press(m, tea.KeyTab) // a plain Tab: on to the server window
	if m.cur != 0 || unread != 2 || alice.unread != unread || !maps.Equal(alice.counted, counted) {
		t.Fatalf("@alice is left as it was: cur %d, unread %d (was %d), counted %v", m.cur, alice.unread, unread, alice.counted)
	}
	passGap(m, step, held)
	if len(f.read) != 0 {
		t.Fatalf("a window passed through is not reported read: %v", f.read)
	}
	runAll(m, altN(m, '2'))
	if len(f.read) != 1 || f.read[0] != "c2:7" || alice.unread != 0 {
		t.Fatalf("focused again, it is reported at once: %v, unread %d", f.read, alice.unread)
	}
}

// TestWindowLeftWithinTabGapComesBack: a window a Tab showed and another
// focus left within tabGap was only passed through too; whatever brings it
// back reports it, once. Whatever /doubletab says.
func TestWindowLeftWithinTabGapComesBack(t *testing.T) {
	allModes(t, testWindowLeftWithinTabGapComesBack)
}

func testWindowLeftWithinTabGapComesBack(t *testing.T, mode string) {
	f := newFake()
	m, step, held := fromDevWith(t, f, mode)
	enter(m, "/read-receipts on")
	alice := m.wins[3]
	press(m, tea.KeyTab) // @alice, receipt held
	step(100 * time.Millisecond)
	altN(m, '2') // #dev
	if alice.unread != 2 || len(alice.counted) != 2 {
		t.Fatalf("left within tabGap, @alice gets its count back: %d %v", alice.unread, alice.counted)
	}
	step(100 * time.Millisecond)
	runAll(m, altN(m, '3')) // @alice again, still within tabGap of the Tab
	if len(f.read) != 1 || f.read[0] != "c2:7" || alice.unread != 0 {
		t.Fatalf("Alt-3 reports @alice at once: %v, unread %d", f.read, alice.unread)
	}
	passGap(m, step, held)
	if len(f.read) != 1 {
		t.Fatalf("the Tab's held receipt does not go as well: %v", f.read)
	}
	// Tab, Shift-Tab, Tab: the last Tab is a first Tab, and its own receipt
	// goes tabGap later, once.
	f.read = nil
	m, step, held = fromDevWith(t, f, mode)
	enter(m, "/read-receipts on")
	alice = m.wins[3]
	press(m, tea.KeyTab)      // @alice
	press(m, tea.KeyShiftTab) // #dev: @alice's count back
	step(300 * time.Millisecond)
	press(m, tea.KeyTab) // @alice again
	if len(f.read) != 0 || alice.unread != 0 || len(*held) != 2 {
		t.Fatalf("in @alice, its receipt held: %v, unread %d, held %d", f.read, alice.unread, len(*held))
	}
	// The first Tab's message comes 100 ms into the last one's wait: it is
	// not that Tab's, and sends nothing.
	step(100 * time.Millisecond)
	runAll(m, func() tea.Msg { return (*held)[0] })
	*held = (*held)[1:]
	if len(f.read) != 0 {
		t.Fatalf("a message of a Tab whose window was left sends nothing: %v", f.read)
	}
	passGap(m, step, held)
	if len(f.read) != 1 || f.read[0] != "c2:7" {
		t.Fatalf("@alice is reported once, tabGap later: %v", f.read)
	}
}

// TestWindowLeftAfterTabGapIsReported: a window that stayed current for
// tabGap was read; left before its held message came (a late timer), its
// receipt goes as it is left, and only then. Whatever /doubletab says.
func TestWindowLeftAfterTabGapIsReported(t *testing.T) {
	allModes(t, testWindowLeftAfterTabGapIsReported)
}

func testWindowLeftAfterTabGapIsReported(t *testing.T, mode string) {
	f := newFake()
	m, step, held := fromDevWith(t, f, mode)
	enter(m, "/read-receipts on")
	alice := m.wins[3]
	press(m, tea.KeyTab) // @alice, receipt held
	step(m.tabGap + time.Millisecond)
	runAll(m, altN(m, '2'))
	if len(f.read) != 1 || f.read[0] != "c2:7" || alice.unread != 0 {
		t.Fatalf("@alice was read, and is reported as it is left: %v, unread %d", f.read, alice.unread)
	}
	passGap(m, step, held)
	if len(f.read) != 1 {
		t.Fatalf("the late message sends nothing more: %v", f.read)
	}
	// Reported when its message came, a window stays read as it is left,
	// however soon after.
	alice.unread = 2     // news for @alice
	press(m, tea.KeyTab) // from #dev: @alice, receipt held
	passGap(m, step, held)
	runAll(m, altN(m, '2'))
	if len(f.read) != 2 || m.wins[3].unread != 0 {
		t.Fatalf("@alice was reported tabGap later, and stays read: %v, unread %d", f.read, m.wins[3].unread)
	}
}

// TestTabHoldsItsReceiptOnARealTimer: without holdTicks the model's own
// after holds the receipt (the command is not run, so nothing waits), and
// tickAfter delivers its message only after the pause.
func TestTabHoldsItsReceiptOnARealTimer(t *testing.T) {
	m := setup(t, newFake()) // 0 server, 1 #news, 2 #dev, 3 @alice
	enter(m, "/win 1")
	enter(m, "/win 2")
	enter(m, "/read-receipts on")
	if cmd := press(m, tea.KeyTab); cmd == nil || m.winName(m.cur) != "@alice" || m.held == nil || m.held.receipt == nil {
		t.Fatalf("Tab onto @alice holds its receipt: in %s", m.winName(m.cur))
	}
	start := time.Now()
	if got := tickAfter(time.Millisecond, tabReadMsg{seq: 7})(); got != (tabReadMsg{seq: 7}) || time.Since(start) < time.Millisecond {
		t.Fatalf("tickAfter delivered %v after %v", got, time.Since(start))
	}
}

// TestWinLastDoubleTabInVimNormal: with vim on and /doubletab last, a
// double Tab on an empty prompt goes back to the last window in Normal as
// in Insert, and gt is a window switch like Ctrl-N: a Tab right after it
// is a first Tab.
func TestWinLastDoubleTabInVimNormal(t *testing.T) {
	m, _, _ := fromDevWith(t, newFake(), "last")
	enter(m, "/vim on")
	press(m, tea.KeyEsc)
	if !m.inNormal() || len(m.input) != 0 || m.cur != 2 {
		t.Fatalf("Normal on an empty prompt in #dev: normal=%v input %q cur %d", m.inNormal(), string(m.input), m.cur)
	}
	press(m, tea.KeyTab)
	press(m, tea.KeyTab)
	if m.cur != 1 || m.lastWindow() != 2 || !m.inNormal() {
		t.Fatalf("Tab Tab in Normal: window %d, last %d, normal=%v; want #news (1), last #dev (2)", m.cur, m.lastWindow(), m.inNormal())
	}
	typ(m, "gt")
	if m.cur != 2 || m.lastWindow() != 1 {
		t.Fatalf("gt from #news: window %d, last %d; want #dev (2), last #news (1)", m.cur, m.lastWindow())
	}
	press(m, tea.KeyTab)
	if m.cur != 3 {
		t.Fatalf("a Tab right after gt is a first Tab: window %d, want @alice (3)", m.cur)
	}
}
