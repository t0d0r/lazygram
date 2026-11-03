package tui

import (
	"encoding/json"
	"maps"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lazygram/internal/command"
)

// tabWalk presses Tab n times, 100 ms apart, and returns the windows each
// one showed.
func tabWalk(m *Model, step func(time.Duration), n int) string {
	var got []string
	for i := range n {
		if i > 0 {
			step(100 * time.Millisecond)
		}
		press(m, tea.KeyTab)
		got = append(got, m.winName(m.cur))
	}
	return strings.Join(got, " ")
}

// TestDoubleTabIsOffByDefault: a new start has no double Tab, so fast Tabs
// walk the windows, one each, and never jump; a window a Tab only passed
// through still gets its unread count back and is not reported.
func TestDoubleTabIsOffByDefault(t *testing.T) {
	f := newFake()
	m := setup(t, f) // 0 server, 1 #news, 2 #dev, 3 @alice
	step := pinClock(m)
	held := holdTicks(t, m)
	enter(m, "/win 1")
	enter(m, "/win 2") // in #dev, #news last
	var kept string
	if found, _ := m.opt.Store.State(doubleTabKey, &kept); m.dblTab != tabOff || found || m.tabGap != 400*time.Millisecond {
		t.Fatalf("off and 400 ms, nothing kept yet: %v, kept %v, %v", m.dblTab, found, m.tabGap)
	}
	enter(m, "/read-receipts on")
	alice := m.wins[3]
	unread, counted := alice.unread, maps.Clone(alice.counted)
	if got := tabWalk(m, step, 2); got != "@alice server" || m.winName(m.lastWindow()) != "@alice" {
		t.Fatalf("Tab Tab fast walks two windows, no jump back to #news: %s, last %s", got, m.winName(m.lastWindow()))
	}
	if unread != 2 || alice.unread != unread || !maps.Equal(alice.counted, counted) {
		t.Fatalf("@alice was passed through: unread %d (was %d), counted %v", alice.unread, unread, alice.counted)
	}
	passGap(m, step, held)
	if len(f.read) != 0 {
		t.Fatalf("nothing passed through is reported: %v", f.read)
	}
	// On round: four fast Tabs, four windows.
	step(time.Second)
	if got := tabWalk(m, step, 4); got != "#news #dev @alice server" {
		t.Fatalf("fast Tabs walk on: %s", got)
	}
	passGap(m, step, held)
	if len(f.read) != 0 || alice.unread != 2 {
		t.Fatalf("still nothing reported, @alice still unread: %v, %d", f.read, alice.unread)
	}
}

// TestDoubleTabModesTakeEffectAtOnce: each /doubletab mode changes the
// very next Tab Tab, from the same place: #dev, #news the last window,
// @alice (2 unread) the next one.
func TestDoubleTabModesTakeEffectAtOnce(t *testing.T) {
	m, step, _ := fromDevWith(t, newFake(), "off")
	for _, c := range []struct{ mode, want, last string }{
		{"last", "@alice #news", "#dev"},    // back to the last window
		{"unread", "@alice @alice", "#dev"}, // the next unread window after #dev: @alice itself
		{"off", "@alice server", "@alice"},  // two windows on
		{"LAST", "@alice #news", "#dev"},    // the words ignore case
		{"Unread", "@alice @alice", "#dev"},
		{"OFF", "@alice server", "@alice"},
	} {
		enter(m, "/win 1") // back to the start
		enter(m, "/win 2")
		m.wins[3].unread = 2
		enter(m, "/doubletab "+c.mode)
		step(time.Second)
		if got := tabWalk(m, step, 2); got != c.want || m.winName(m.lastWindow()) != c.last {
			t.Fatalf("/doubletab %s: Tab Tab showed %s, last %s; want %s, last %s", c.mode, got, m.winName(m.lastWindow()), c.want, c.last)
		}
	}
}

// TestDoubleTabUnreadGoesToTheNextUnreadWindow: with /doubletab unread the
// second fast Tab goes where /unread goes from the window the first one
// left, wrapping round; afterwards the last window is the one the gesture
// began in, and a third fast Tab is a first Tab again.
func TestDoubleTabUnreadGoesToTheNextUnreadWindow(t *testing.T) {
	f := newFake()
	m := setup(t, f) // 0 server, 1 #news (4), 2 #dev (2), 3 @alice (2)
	step := pinClock(m)
	held := holdTicks(t, m)
	enter(m, "/doubletab unread")
	enter(m, "/read-receipts on")
	enter(m, "/win 2")
	enter(m, "/win 1") // in #news, #dev read: from #news a Tab shows #dev
	if got := tabWalk(m, step, 2); got != "#dev @alice" || m.winName(m.lastWindow()) != "#news" || m.wins[3].unread != 0 {
		t.Fatalf("Tab Tab from #news passes #dev for @alice: %s, last %s, @alice unread %d",
			got, m.winName(m.lastWindow()), m.wins[3].unread)
	}
	// @alice is the double Tab's destination: its receipt waits, as any
	// Tab's does, and goes once it stayed.
	if len(f.read) != 0 {
		t.Fatalf("not at once: %v", f.read)
	}
	passGap(m, step, held)
	if len(f.read) != 1 || f.read[0] != "c2:7" {
		t.Fatalf("@alice stayed: reported once: %v", f.read)
	}
	// From @alice it wraps round, past window 0, to #news.
	step(time.Second)
	m.wins[1].unread = 3
	if got := tabWalk(m, step, 2); got != "server #news" || m.winName(m.lastWindow()) != "@alice" {
		t.Fatalf("from @alice round to #news: %s, last %s", got, m.winName(m.lastWindow()))
	}
	// A third fast Tab is a first Tab: the next window, at once.
	step(100 * time.Millisecond)
	press(m, tea.KeyTab)
	if m.winName(m.cur) != "#dev" || m.winName(m.lastWindow()) != "#news" {
		t.Fatalf("a third fast Tab is a plain first one: %s, last %s", m.winName(m.cur), m.winName(m.lastWindow()))
	}
}

// TestDoubleTabUnreadCountsThePassedWindowBack: the window the first Tab
// only passed through gets its unread count back first, so it is the
// target when it is the next one with unread messages, ahead of any
// further one; it is then shown by the double Tab, whose receipt waits
// like any Tab's, and left within the time it is left as it was.
func TestDoubleTabUnreadCountsThePassedWindowBack(t *testing.T) {
	f := newFake()
	m, step, held := fromDevWith(t, f, "unread")
	enter(m, "/read-receipts on")
	alice := m.wins[3]
	unread, counted := alice.unread, maps.Clone(alice.counted)
	m.wins[1].unread = 1 // #news, after @alice round from #dev
	if got := tabWalk(m, step, 2); got != "@alice @alice" || alice.unread != 0 || m.winName(m.lastWindow()) != "#dev" {
		t.Fatalf("the second Tab keeps @alice, read: %s, unread %d, last %s", got, alice.unread, m.winName(m.lastWindow()))
	}
	if len(*held) != 2 || len(f.read) != 0 {
		t.Fatalf("each Tab held a receipt, none sent: held %d, read %v", len(*held), f.read)
	}
	// Left within the time of the second Tab: as it was again, never sent.
	step(300 * time.Millisecond)
	runAll(m, altN(m, '2'))
	if alice.unread != unread || !maps.Equal(alice.counted, counted) {
		t.Fatalf("@alice left soon after the double Tab: unread %d (was %d), counted %v", alice.unread, unread, alice.counted)
	}
	passGap(m, step, held)
	if len(f.read) != 0 {
		t.Fatalf("@alice was not read: %v", f.read)
	}
	// Its time runs from the second Tab: left 450 ms after the first and
	// 350 ms after the second, it was passed through.
	m, step, _ = fromDevWith(t, f, "unread")
	enter(m, "/read-receipts on")
	tabWalk(m, step, 2)
	step(m.tabGap - 50*time.Millisecond)
	runAll(m, altN(m, '2'))
	if m.wins[3].unread != 2 || len(f.read) != 0 {
		t.Fatalf("the double Tab's own hold counts from the second Tab: unread %d, read %v", m.wins[3].unread, f.read)
	}
	// Staying, it is reported once, the time after the second Tab.
	m, step, held = fromDevWith(t, f, "unread")
	enter(m, "/read-receipts on")
	tabWalk(m, step, 2)
	passGap(m, step, held)
	if len(f.read) != 1 || f.read[0] != "c2:7" {
		t.Fatalf("@alice stayed: reported once: %v", f.read)
	}
}

// TestDoubleTabUnreadWithoutUnreadIsTwoTabs: with no window with unread
// messages the second fast Tab is a plain one, the next window, as for
// last with no last window; it arms nothing, so a third fast Tab is a
// first Tab and a fourth its second.
func TestDoubleTabUnreadWithoutUnreadIsTwoTabs(t *testing.T) {
	m, step, _ := fromDevWith(t, newFake(), "unread")
	m.wins[3].clearUnread() // nothing unread anywhere
	if got := tabWalk(m, step, 2); got != "@alice server" || m.winName(m.lastWindow()) != "@alice" {
		t.Fatalf("two plain Tabs: %s, last %s", got, m.winName(m.lastWindow()))
	}
	m.wins[2].unread = 1 // #dev
	step(100 * time.Millisecond)
	if got := tabWalk(m, step, 2); got != "#news #dev" {
		t.Fatalf("the third fast Tab is a first one (#news), the fourth its second (on to #dev, the next unread after the server window): %s", got)
	}
	m.wins[1].unread = 1 // #news, the window the third Tab passed through, comes first
	step(time.Second)
	if got := tabWalk(m, step, 2); got != "@alice #news" {
		t.Fatalf("from #dev round to #news: %s", got)
	}
}

// TestDoubleTabTimeIsTheWindow: with a time of 250 ms a second Tab 250 ms
// after the first is part of a double Tab and one 251 ms after is not, in
// last and in unread; the held receipt's timer runs 250 ms, and a window
// a Tab showed counts as read after 250 ms, in every mode.
func TestDoubleTabTimeIsTheWindow(t *testing.T) {
	for _, c := range []struct {
		mode  string
		pause time.Duration
		want  string
	}{
		{"last", 250 * time.Millisecond, "#news"},    // the last window
		{"last", 251 * time.Millisecond, "server"},   // a plain second Tab
		{"unread", 250 * time.Millisecond, "@alice"}, // @alice, unread again, stays
		{"unread", 251 * time.Millisecond, "server"},
	} {
		m, step, _ := fromDevWith(t, newFake(), c.mode)
		enter(m, "/doubletab 250")
		press(m, tea.KeyTab) // @alice
		step(c.pause)
		press(m, tea.KeyTab)
		if got := m.winName(m.cur); got != c.want {
			t.Fatalf("%s, a pause of %v: %s, want %s", c.mode, c.pause, got, c.want)
		}
	}
	// A longer time than the default: 900 ms apart is still a double Tab.
	m, step, _ := fromDev(t, newFake())
	enter(m, "/doubletab 1000")
	press(m, tea.KeyTab)
	step(900 * time.Millisecond)
	press(m, tea.KeyTab)
	if m.winName(m.cur) != "#news" {
		t.Fatalf("within 1000 ms: a double Tab: %s", m.winName(m.cur))
	}
	allModes(t, func(t *testing.T, mode string) {
		for _, c := range []struct {
			pause time.Duration
			read  bool
		}{{249 * time.Millisecond, false}, {250 * time.Millisecond, false}, {251 * time.Millisecond, true}} {
			f := newFake()
			m, step, _ := fromDevWith(t, f, mode)
			var delays []time.Duration
			m.after = func(d time.Duration, msg tea.Msg) tea.Cmd { delays = append(delays, d); return nil }
			enter(m, "/doubletab 250")
			enter(m, "/read-receipts on")
			press(m, tea.KeyTab) // @alice
			if len(delays) != 1 || delays[0] != 250*time.Millisecond {
				t.Fatalf("the receipt is held for the time set: %v", delays)
			}
			step(c.pause)
			runAll(m, altN(m, '2'))
			if read := len(f.read) == 1 && m.wins[3].unread == 0; read != c.read || len(f.read) > 1 {
				t.Fatalf("@alice left %v after the Tab: read %v (sent %v, unread %d), want %v", c.pause, read, f.read, m.wins[3].unread, c.read)
			}
		}
	})
}

// TestHeldTabKeepsItsTime: a Tab whose receipt is already held keeps the
// time it was held with, whatever /doubletab sets meanwhile.
func TestHeldTabKeepsItsTime(t *testing.T) {
	f := newFake()
	m, step, held := fromDevWith(t, f, "off")
	enter(m, "/read-receipts on")
	press(m, tea.KeyTab) // @alice, held for 400 ms
	enter(m, "/doubletab 150")
	step(300 * time.Millisecond)
	runAll(m, altN(m, '2'))
	if m.wins[3].unread != 2 || len(f.read) != 0 {
		t.Fatalf("left 300 ms after a Tab held for 400: passed through, unread %d, read %v", m.wins[3].unread, f.read)
	}
	*held = nil
	press(m, tea.KeyTab) // @alice again, now held for 150 ms
	step(151 * time.Millisecond)
	runAll(m, altN(m, '2'))
	if m.wins[3].unread != 0 || len(f.read) != 1 {
		t.Fatalf("left 151 ms after a Tab held for 150: read, unread %d, read %v", m.wins[3].unread, f.read)
	}
}

// TestDoubleTabCommand: /doubletab shows, sets and keeps the setting and
// its time; anything else, on included, is an error that changes nothing,
// the store included; a new start on the same store has what was set.
func TestDoubleTabCommand(t *testing.T) {
	st := memStore(t)
	m := startWith(t, st, newFake())
	stored := func() (word string, wordFound bool, ms int, msFound bool) {
		wordFound, _ = st.State(doubleTabKey, &word)
		msFound, _ = st.State(doubleTabTimeKey, &ms)
		return
	}
	shows := func(want string) {
		t.Helper()
		before1, before2, before3, before4 := stored()
		enter(m, "/doubletab")
		after1, after2, after3, after4 := stored()
		if m.isErr || m.status != want || before1 != after1 || before2 != after2 || before3 != after3 || before4 != after4 {
			t.Fatalf("no argument shows %q and keeps nothing: %q", want, m.status)
		}
	}
	bad := func(arg string, mode doubleTab, gap time.Duration) {
		t.Helper()
		w1, f1, ms1, g1 := stored()
		enter(m, "/doubletab "+arg)
		w2, f2, ms2, g2 := stored()
		if !m.isErr || m.status != "/doubletab takes off, last, unread or a time from 150 to 1000 ms" || m.dblTab != mode || m.tabGap != gap ||
			w1 != w2 || f1 != f2 || ms1 != ms2 || g1 != g2 {
			t.Errorf("%q: an error that changes nothing: %q, %v, %v, store %q %v %d %v", arg, m.status, m.dblTab, m.tabGap, w2, f2, ms2, g2)
		}
	}
	shows("double tab off (400 ms): Tab walks the windows; /doubletab last or unread makes Tab twice fast jump")
	// 288230376151712044 is 2^58 + 300: as milliseconds it wraps int64 to
	// exactly 300 ms, so it is out of range only when the range is checked
	// on the number itself.
	for _, arg := range []string{"on", "ON", "149", "1001", "junk", "0", "-300", "+300", "3e2", "300s", "ms", "off last", "300 400", "lastt",
		"288230376151712044", "288230376151712044ms", "9223372036854775807", "99999999999999999999"} {
		bad(arg, tabOff, 400*time.Millisecond)
	}
	if _, f1, _, f2 := stored(); f1 || f2 {
		t.Fatal("an error keeps nothing")
	}
	for _, c := range []struct {
		arg, word, says string
		mode            doubleTab
	}{
		{"last", "last", "double tab last: Tab twice within 400 ms jumps back to the last window", tabLast},
		{"UNREAD", "unread", "double tab unread: Tab twice within 400 ms jumps to the next unread window if any", tabUnread},
		{" Off ", "off", "double tab off: Tab walks the windows, however fast", tabOff},
		{"Last", "last", "double tab last: Tab twice within 400 ms jumps back to the last window", tabLast},
	} {
		enter(m, "/doubletab "+c.arg)
		if word, found, _, _ := stored(); m.isErr || m.dblTab != c.mode || word != c.word || !found || m.status != c.says {
			t.Fatalf("%q: %v, kept %q (%v), %q", c.arg, m.dblTab, word, found, m.status)
		}
	}
	shows("double tab last (400 ms): Tab twice fast jumps back to the last window; /doubletab off makes Tab walk")
	for arg, want := range map[string]time.Duration{"300": 300, "250ms": 250, "150": 150, "1000MS": 1000, "350 ms": 350, " 300ms ": 300} {
		enter(m, "/doubletab "+arg)
		if _, _, ms, found := stored(); m.isErr || m.tabGap != want*time.Millisecond || !found || ms != int(want) || m.dblTab != tabLast {
			t.Errorf("%q: %v kept as %d (%v), %v: %q", arg, m.tabGap, ms, found, m.dblTab, m.status)
		}
	}
	enter(m, "/doubletab 300")
	if m.status != "double tab time 300 ms: Tab twice within it jumps back to the last window" {
		t.Fatalf("a time, last: %q", m.status)
	}
	for _, arg := range []string{"149", "1001", "junk", "on"} {
		bad(arg, tabLast, 300*time.Millisecond)
	}
	enter(m, "/doubletab unread")
	if m.status != "double tab unread: Tab twice within 300 ms jumps to the next unread window if any" {
		t.Fatalf("unread says the time set: %q", m.status)
	}
	enter(m, "/doubletab 300ms")
	if m.status != "double tab time 300 ms: Tab twice within it jumps to the next unread window if any" {
		t.Fatalf("a time, unread: %q", m.status)
	}
	shows("double tab unread (300 ms): Tab twice fast jumps to the next unread window if any; /doubletab off makes Tab walk")
	enter(m, "/doubletab off")
	enter(m, "/doubletab 300")
	if m.status != "double tab time 300 ms: Tab twice is off; a window a Tab shows for less stays unread" {
		t.Fatalf("a time, off: %q", m.status)
	}
	shows("double tab off (300 ms): Tab walks the windows; /doubletab last or unread makes Tab twice fast jump")
	// Kept across a restart on the same store.
	again := startWith(t, st, newFake())
	if again.dblTab != tabOff || again.tabGap != 300*time.Millisecond {
		t.Fatalf("a new start: %v, %v; want off, 300 ms", again.dblTab, again.tabGap)
	}
	enter(again, "/doubletab unread")
	enter(again, "/doubletab 700")
	if third := startWith(t, st, newFake()); third.dblTab != tabUnread || third.tabGap != 700*time.Millisecond {
		t.Fatalf("unread and 700 ms are kept: %v, %v", third.dblTab, third.tabGap)
	}
	enter(again, "/doubletab last")
	if fourth := startWith(t, st, newFake()); fourth.dblTab != tabLast {
		t.Fatalf("last is kept: %v", fourth.dblTab)
	}
}

// TestDoubleTabUnreadableOrOutOfRange: what the store holds that is not a
// setting is the default: off, and 400 ms; on (the earlier on/off form)
// is not a setting, and a time far out of range is out of range even
// when as milliseconds it would wrap int64 into the range (2^58 + 300
// wraps to 300 ms).
func TestDoubleTabUnreadableOrOutOfRange(t *testing.T) {
	for _, c := range []struct {
		mode, ms any
	}{{"nonsense", "nonsense"}, {true, 2.5}, {"on", 149}, {"LAST", 1001}, {1, 0}, {nil, -400}, {"", nil},
		{"off", int64(288230376151712044)}, {nil, int64(-288230376151712044 + 600)}, {nil, json.Number("99999999999999999999")}} {
		st := memStore(t)
		if c.mode != nil {
			if err := st.SetState(doubleTabKey, c.mode); err != nil {
				t.Fatal(err)
			}
		}
		if c.ms != nil {
			if err := st.SetState(doubleTabTimeKey, c.ms); err != nil {
				t.Fatal(err)
			}
		}
		if m := startWith(t, st, newFake()); m.dblTab != tabOff || m.tabGap != 400*time.Millisecond {
			t.Errorf("stored %v, %v: %v, %v; want off, 400 ms", c.mode, c.ms, m.dblTab, m.tabGap)
		}
	}
	for _, c := range []struct {
		mode string
		ms   int
		want doubleTab
	}{{"last", 150, tabLast}, {"unread", 1000, tabUnread}, {"off", 400, tabOff}} {
		st := memStore(t)
		_ = st.SetState(doubleTabKey, c.mode)
		_ = st.SetState(doubleTabTimeKey, c.ms)
		if m := startWith(t, st, newFake()); m.dblTab != c.want || m.tabGap != time.Duration(c.ms)*time.Millisecond {
			t.Errorf("stored %s, %d: %v, %v", c.mode, c.ms, m.dblTab, m.tabGap)
		}
	}
}

// TestDoubleTabSettingsRows: the menu's two rows show the setting and its
// time and change them through /doubletab's own code; a time the row does
// not list steps to the next longer or shorter listed one, wrapping.
func TestDoubleTabSettingsRows(t *testing.T) {
	dt, tm := settingNamed(t, "doubletab"), settingNamed(t, "doubletab-time")
	if dt.key != "doubletab" || !reflect.DeepEqual(dt.values(), []string{"off", "last", "unread"}) || dt.command() != "doubletab" {
		t.Fatalf("the doubletab row: %+v %q", dt, dt.values())
	}
	if tm.key != "doubletab-time" || tm.command() != "doubletab" ||
		!reflect.DeepEqual(tm.values(), []string{"200ms", "300ms", "400ms", "500ms", "600ms", "800ms"}) {
		t.Fatalf("the doubletab-time row: %+v %q", tm, tm.values())
	}
	st := memStore(t)
	m := startWith(t, st, newFake())
	openAt(t, m, "doubletab")
	for _, c := range []struct {
		key  tea.KeyType
		want string
	}{{tea.KeyEnter, "last"}, {tea.KeyRight, "unread"}, {tea.KeyRight, "off"}, {tea.KeyLeft, "unread"}, {tea.KeyLeft, "last"}} {
		press(m, c.key)
		again := New(Options{Store: st, Provider: newFake()})
		if menuValue(m, "doubletab") != c.want || m.dblTab.String() != c.want || again.dblTab.String() != c.want {
			t.Fatalf("%v: shown %q, set %v, kept %v; want %s", c.key, menuValue(m, "doubletab"), m.dblTab, again.dblTab, c.want)
		}
	}
	if m.status != "double tab last: Tab twice within 400 ms jumps back to the last window" || !m.settingsOpen() {
		t.Fatalf("the row says what /doubletab says: %q", m.status)
	}
	press(m, tea.KeyEsc)
	openAt(t, m, "doubletab-time")
	for _, c := range []struct {
		key  tea.KeyType
		want string
	}{{tea.KeyEnter, "500ms"}, {tea.KeyRight, "600ms"}, {tea.KeyRight, "800ms"}, {tea.KeyRight, "200ms"}, {tea.KeyLeft, "800ms"}, {tea.KeyLeft, "600ms"}} {
		press(m, c.key)
		again := New(Options{Store: st, Provider: newFake()})
		if menuValue(m, "doubletab-time") != c.want || msOf(m.tabGap) != c.want || msOf(again.tabGap) != c.want {
			t.Fatalf("%v: shown %q, set %v, kept %v; want %s", c.key, menuValue(m, "doubletab-time"), m.tabGap, again.tabGap, c.want)
		}
	}
	if m.status != "double tab time 600 ms: Tab twice within it jumps back to the last window" || !m.settingsOpen() {
		t.Fatalf("the row says what /doubletab says: %q", m.status)
	}
	// A time the command set and the row does not list shows as it is, and
	// steps to its listed neighbours, wrapping round past the ends.
	for _, c := range []struct {
		set         string
		right, left string
	}{{"250", "300ms", "200ms"}, {"1000", "200ms", "800ms"}, {"150", "200ms", "800ms"}, {"799", "800ms", "600ms"}, {"201ms", "300ms", "200ms"}} {
		for _, k := range []struct {
			key  tea.KeyType
			want string
		}{{tea.KeyRight, c.right}, {tea.KeyEnter, c.right}, {tea.KeyLeft, c.left}} {
			enter(m, "/doubletab "+c.set)
			openAt(t, m, "doubletab-time")
			if menuValue(m, "doubletab-time") != strings.TrimSuffix(c.set, "ms")+"ms" {
				t.Fatalf("%s shows as it is: %q", c.set, menuValue(m, "doubletab-time"))
			}
			press(m, k.key)
			if got := menuValue(m, "doubletab-time"); got != k.want || msOf(m.tabGap) != k.want {
				t.Errorf("from %s, %v: %s (%v), want %s", c.set, k.key, got, m.tabGap, k.want)
			}
		}
	}
	// The rows are what typing "tab" leaves; their descriptions keep out of
	// the other filters (TestSettingsNarrowByTypedText).
	press(m, tea.KeyEsc)
	enter(m, "/settings tab")
	if got := menuRows(m); len(got) != 2 || got[0][0] != "doubletab" || got[1][0] != "doubletab-time" {
		t.Fatalf("/settings tab: %q", got)
	}
	for _, filter := range []string{"mo", "receipts", "selection", "colours", "feed", "vim", "theme", "mouse", "clip", "names", "auto"} {
		press(m, tea.KeyEsc)
		enter(m, "/settings "+filter)
		for _, r := range menuRows(m) {
			if strings.HasPrefix(r[0], "doubletab") {
				t.Errorf("/settings %s shows %s", filter, r[0])
			}
		}
	}
}

// TestDoubleTabCompletesAndIsDescribed: Tab completes off, last and unread
// after /doubletab; /help lists it and says /win last's and /unread's Tab
// twice needs it; /keys says what Tab twice does depends on it; the start
// hints say no more than the default does.
func TestDoubleTabCompletesAndIsDescribed(t *testing.T) {
	m := setup(t, newFake())
	for typed, want := range map[string]string{
		"/doubletab o":  "/doubletab off ",
		"/doubletab l":  "/doubletab last ",
		"/doubletab u":  "/doubletab unread ",
		"/doubletab on": "/doubletab on", // no such setting
		"/doubletab al": "/doubletab al", // a setting, not a chat
		"/double":       "/doubletab ",
	} {
		m.input, m.curBack = nil, 0
		typ(m, typed)
		press(m, tea.KeyTab)
		if got := string(m.input); got != want {
			t.Errorf("%q: got %q, want %q", typed, got, want)
		}
	}
	if c, err := command.Parse("/DoubleTab 300ms"); err != nil || c.Kind != command.DoubleTab || c.Arg != "300ms" {
		t.Fatalf("/doubletab parses: %+v %v", c, err)
	}
	printed := func(line string) string {
		m := setup(t, newFake())
		enter(m, line)
		drain(m)
		var out []string
		for _, n := range m.wins[0].notes {
			out = append(out, n.text)
		}
		return strings.Join(out, "\n")
	}
	help, keys := printed("/help"), printed("/keys")
	for _, want := range []string{
		"  /doubletab [off|last|unread|<ms>]  what Tab twice fast on an empty prompt does: off (the default) nothing, every Tab goes to the next window; last: back to the last window, as /win last; unread: to the next window with unread messages, as /unread (none: the next window); <ms>: how fast",
		"again goes back (with /doubletab last, Tab twice fast does it too)",
		"wrapping round (with /doubletab unread, Tab twice fast does it too)",
	} {
		if !strings.Contains(help, want) {
			t.Errorf("/help lacks %q", want)
		}
	}
	if !strings.Contains(keys, "next window (Shift-Tab previous); twice fast: as /doubletab says (off, /win last or /unread);") {
		t.Errorf("/keys: what the double Tab does is /doubletab's:\n%s", keys)
	}
	pool := strings.Join(hintPool, "\n")
	for _, h := range append(append([]string(nil), hintsFixed...), hintPool...) {
		if strings.Contains(h, "Tab twice") && !strings.Contains(h, "/doubletab") {
			t.Errorf("%q: Tab twice jumps only as /doubletab says", h)
		}
	}
	for _, want := range []string{
		"hint: /win last jumps back, /unread to the next unread window",
		"hint: /doubletab last or unread turns Tab twice fast into a jump",
	} {
		if !strings.Contains(pool, want) {
			t.Errorf("the start hints lack %q", want)
		}
	}
}
