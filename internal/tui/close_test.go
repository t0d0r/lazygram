package tui

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"lazygram/internal/command"
	"lazygram/internal/domain"
)

// wideFake is newFake with four more chats, so that seven windows can be
// open (wide).
func wideFake() *fake {
	f := newFake()
	f.chats = append(f.chats,
		domain.Chat{ID: "c5", Alias: "ops", Title: "Ops", Kind: domain.ChatGroup},
		domain.Chat{ID: "c6", Alias: "qa", Title: "QA", Kind: domain.ChatGroup},
		domain.Chat{ID: "c7", Alias: "bob", Title: "Bob", Kind: domain.ChatUser},
		domain.Chat{ID: "c8", Alias: "web", Title: "Web", Kind: domain.ChatChannel},
	)
	return f
}

// wide is a model with seven chat windows, window 0 current:
// 0 server, 1 #news, 2 #dev, 3 @alice, 4 #ops, 5 #qa, 6 @bob, 7 #web.
func wide(t *testing.T) *Model {
	t.Helper()
	m := setup(t, wideFake())
	for _, id := range []string{"c5", "c6", "c7", "c8"} {
		m.openWin(id)
	}
	if got := winNames(m); got != "#news #dev @alice #ops #qa @bob #web" || m.cur != 0 {
		t.Fatalf("windows: %q, cur %d", got, m.cur)
	}
	return m
}

// winNames names the chat windows 1..N in order.
func winNames(m *Model) string {
	var s []string
	for i := 1; i < len(m.wins); i++ {
		s = append(s, m.winName(i))
	}
	return strings.Join(s, " ")
}

// storedLayout is the layout in the store, as a restart would read it.
func storedLayout(t *testing.T, m *Model) layout {
	t.Helper()
	var l layout
	if ok, err := m.opt.Store.State(layoutKey, &l); !ok || err != nil {
		t.Fatalf("no stored layout: %v", err)
	}
	return l
}

// chatIDs are the chats of windows 1..N.
func chatIDs(m *Model) []string {
	var ids []string
	for _, w := range m.wins[1:] {
		ids = append(ids, w.chatID)
	}
	return ids
}

func noteTexts(w *win) []string {
	var s []string
	for _, n := range w.notes {
		s = append(s, n.text)
	}
	return s
}

func TestCloseSpec(t *testing.T) {
	const last = 7
	for _, c := range []struct {
		spec string
		want []int
	}{
		{"3", []int{3}},
		{"1,2,3", []int{1, 2, 3}},
		{"1-4", []int{1, 2, 3, 4}},
		{"2,3-5", []int{2, 3, 4, 5}},
		{"2, 3-5", []int{2, 3, 4, 5}},
		{"2 ,3-5", []int{2, 3, 4, 5}},
		{"2 3 5", []int{2, 3, 5}},
		{"2  3,\t5", []int{2, 3, 5}},
		{"5,2", []int{2, 5}},
		{"1,1", []int{1}},
		{"1-3,2", []int{1, 2, 3}},
		{"2-4,3-6", []int{2, 3, 4, 5, 6}},
		{"4-4", []int{4}},
		{"07", []int{7}},
		{"1-7", []int{1, 2, 3, 4, 5, 6, 7}},
	} {
		got, err := closeSpec(c.spec, last)
		if err != nil || !slices.Equal(got, c.want) {
			t.Errorf("closeSpec(%q) = %v, %v; want %v", c.spec, got, err, c.want)
		}
	}
	for _, c := range []struct{ spec, err string }{
		{"x", closeFormErr},
		{"2,x", closeFormErr},
		{"2-", closeFormErr},
		{"-2", closeFormErr},
		{"+2", closeFormErr},
		{"2--3", closeFormErr},
		{"1-2-3", closeFormErr},
		{"2,,3", closeFormErr},
		{"2,", closeFormErr},
		{",2", closeFormErr},
		{"2, ,3", closeFormErr},
		{"5-3", closeFormErr},
		{"2 - 5", closeFormErr},
		{"1.5", closeFormErr},
		{"\uff12", closeFormErr}, // a fullwidth 2 is no ASCII digit
		{"0", closeServerErr},
		{"00", closeServerErr},
		{"0-3", closeServerErr},
		{"2,0", closeServerErr},
		{"8", "no window 8"},
		{"08", "no window 8"},
		{"2,9,8", "no window 9"},
		{"6-9", "no window 9"},
		{"9-12", "no window 9"},
		{"99999999999999999999999", "no window 99999999999999999999999"},
		// Ends too large for an int are still ordered as written.
		{"99999999999999999999-10000000000000000000", closeFormErr},
		{"99999999999999999999-10000000000000000000,0", closeFormErr},
		{"99999999999999999999-9999999999999999999", closeFormErr},
		{"99999999999999999999-8", closeFormErr},
		{"10000000000000000000-99999999999999999999", "no window 10000000000000000000"},
		{"10000000000000000000-010000000000000000000", "no window 10000000000000000000"},
		{"8-99999999999999999999", "no window 8"},
		{"0-99999999999999999999", closeServerErr},
		// The checks go in order: the form anywhere, then 0, then past the end.
		{"9,0,x", closeFormErr},
		{"9,0", closeServerErr},
		{"0-9", closeServerErr},
	} {
		got, err := closeSpec(c.spec, last)
		if err == nil || err.Error() != c.err {
			t.Errorf("closeSpec(%q) = %v, %v; want the error %q", c.spec, got, err, c.err)
		}
	}
}

// TestCloseTakesListsAndRanges: every form, the short forms of /close
// too, numbered as the windows were before the command.
func TestCloseTakesListsAndRanges(t *testing.T) {
	for _, c := range []struct{ line, left string }{
		{"/close 3", "#news #dev #ops #qa @bob #web"},
		{"/close 1,2,3", "#ops #qa @bob #web"},
		{"/close 1-4", "#qa @bob #web"},
		{"/close 2,3-5", "#news @bob #web"},
		{"/close 2, 3-5", "#news @bob #web"},
		{"/close 2 3 5", "#news #ops @bob #web"},
		// The windows that were 2 and 3, not 2 and then the one renumbered to 2.
		{"/close 2,3", "#news #ops #qa @bob #web"},
		{"/close 5,2", "#news @alice #ops @bob #web"},
		{"/close 1,1", "#dev @alice #ops #qa @bob #web"},
		{"/close 1-3,2", "#ops #qa @bob #web"},
		{"/close 2-4,3-6", "#news #web"},
		{"/close 1-7", ""},
		{"/wc 6,7", "#news #dev @alice #ops #qa"},
		{"/part 1 7", "#dev @alice #ops #qa @bob"},
	} {
		m := wide(t)
		enter(m, c.line)
		if m.isErr || winNames(m) != c.left {
			t.Errorf("%q leaves %q (%q); want %q", c.line, winNames(m), m.status, c.left)
		}
		if l := storedLayout(t, m); !slices.Equal(l.Windows, chatIDs(m)) {
			t.Errorf("%q: the stored layout %v is not the windows %v", c.line, l.Windows, chatIDs(m))
		}
	}
}

// TestCloseIsAllOrNothing: a bad argument is an error and nothing closes
// or changes: the windows, the current and the last window, the notices,
// the unread counts and the stored layout.
func TestCloseIsAllOrNothing(t *testing.T) {
	for _, c := range []struct{ arg, err string }{
		{"x", closeFormErr}, // was "/close takes a window number"
		{"2,x", closeFormErr},
		{"1-3,4-", closeFormErr},
		{"2,,3", closeFormErr},
		{"5-3", closeFormErr},
		{"1,5-3", closeFormErr},
		{"-1", closeFormErr},
		{"+2", closeFormErr},
		{"99999999999999999999-10000000000000000000", closeFormErr},
		{"0", closeServerErr},
		{"0-3", closeServerErr},
		{"1-3,0", closeServerErr},
		{"8", "no window 8"}, // was the server window's error
		{"1,2,9", "no window 9"},
		{"3-8", "no window 8"},
		{"1-3,12,9", "no window 12"},
	} {
		m := wide(t)
		enter(m, "/win 4")
		enter(m, "/win 2") // #dev current, #ops the last window
		wins, cur, last := slices.Clone(m.wins), m.cur, m.lastWindow()
		notes := noteTexts(m.wins[0])
		unread := map[*win]int{}
		for _, w := range m.wins {
			unread[w] = w.unread
		}
		stored := storedLayout(t, m)
		cmd := enter(m, "/close "+c.arg)
		if !m.isErr || m.status != c.err || cmd != nil {
			t.Errorf("/close %s: %q; want the error %q", c.arg, m.status, c.err)
		}
		if !slices.Equal(m.wins, wins) || m.cur != cur || m.lastWindow() != last {
			t.Errorf("/close %s changed the windows: %q, cur %d, last %d", c.arg, winNames(m), m.cur, m.lastWindow())
		}
		if got := noteTexts(m.wins[0]); !slices.Equal(got, notes) {
			t.Errorf("/close %s noted %q", c.arg, got[len(notes):])
		}
		for _, w := range m.wins {
			if w.unread != unread[w] {
				t.Errorf("/close %s changed the unread count of %s", c.arg, m.nameOf(w.chatID))
			}
		}
		if l := storedLayout(t, m); !reflect.DeepEqual(l, stored) {
			t.Errorf("/close %s changed the stored layout: %+v, was %+v", c.arg, l, stored)
		}
	}
	// No argument and N keep their meaning: /close alone in window 0 refuses.
	m := wide(t)
	notes := len(m.wins[0].notes)
	enter(m, "/close")
	if !m.isErr || m.status != closeServerErr || len(m.wins) != 8 || len(m.wins[0].notes) != notes {
		t.Fatalf("/close in window 0: %q, %d windows", m.status, len(m.wins))
	}
}

// TestCloseKeepsOrMovesTheCurrentWindow: the current window stays current
// when it stays open, renumbered; when it closes, the nearest window
// before it that stays becomes current, window 0 when none does.
func TestCloseKeepsOrMovesTheCurrentWindow(t *testing.T) {
	for _, c := range []struct {
		cur  int
		arg  string
		want string
		at   int
	}{
		{5, "2,3", "#qa", 3},    // stays, down by the two before it
		{5, "1,6-7", "#qa", 4},  // stays, down by the one before it
		{3, "4-6", "@alice", 3}, // stays, nothing closed before it
		{4, "3-5", "#dev", 2},   // closed: the nearest before it that stays
		{4, "2,4", "@alice", 2}, // closed: the window just before, renumbered
		{2, "1-3", "server", 0}, // closed with every window before it
		{1, "1", "server", 0},
		{7, "7", "@bob", 6},
		{7, "1,7", "@bob", 5},
	} {
		m := wide(t)
		enter(m, fmt.Sprint("/win ", c.cur))
		enter(m, "/close "+c.arg)
		if m.isErr || m.cur != c.at || m.winName(m.cur) != c.want {
			t.Errorf("in window %d, /close %s: current %d %s (%q); want %d %s", c.cur, c.arg, m.cur, m.winName(m.cur), m.status, c.at, c.want)
		}
		if l := storedLayout(t, m); l.Current != c.at {
			t.Errorf("in window %d, /close %s: stored current %d, want %d", c.cur, c.arg, l.Current, c.at)
		}
	}
}

// TestCloseFocusesNothing: the window that becomes current when the
// current one closes is not focused: no read receipt, even with receipts
// on, and every window keeps its unread count.
func TestCloseFocusesNothing(t *testing.T) {
	for _, c := range []struct{ arg, cur string }{
		{"", "#ops"},
		{"5", "#ops"},
		{"4,5", "@alice"},
		{"4-6", "@alice"},
		{"3-5", "#dev"},
	} {
		f := wideFake()
		m := setup(t, f)
		for _, id := range []string{"c5", "c6", "c7", "c8"} {
			m.openWin(id)
		}
		enter(m, "/read-receipts on")
		enter(m, "/win 5")   // #qa
		m.wins[4].unread = 3 // #ops; #dev (2) and @alice (3) have 2 unread each, and messages
		unread := map[*win]int{}
		for _, w := range m.wins {
			unread[w] = w.unread
		}
		if m.wins[2].unread != 2 || m.wins[3].unread != 2 {
			t.Fatalf("#dev unread %d, @alice unread %d", m.wins[2].unread, m.wins[3].unread)
		}
		arg := c.arg
		cmd := enter(m, strings.TrimSpace("/close "+arg))
		runAll(m, cmd)
		if m.isErr || m.winName(m.cur) != c.cur || cmd != nil || len(f.read) != 0 {
			t.Errorf("/close %s: current %s (%q), want %s; cmd %v, receipts %v", arg, m.winName(m.cur), m.status, c.cur, cmd != nil, f.read)
		}
		for _, w := range m.wins {
			if w.unread != unread[w] {
				t.Errorf("/close %s: %s unread %d, was %d", arg, m.nameOf(w.chatID), w.unread, unread[w])
			}
		}
	}
}

// TestCloseListDropsTheLastWindow: the last focused window goes when it
// closes, and stays (renumbered) when it does not.
func TestCloseListDropsTheLastWindow(t *testing.T) {
	m := wide(t)
	enter(m, "/win 3")
	enter(m, "/win 6") // @bob, @alice the last window
	enter(m, "/close 2-3")
	enter(m, "/win last")
	if !m.isErr || m.status != "no last window" || m.winName(m.cur) != "@bob" {
		t.Fatalf("a closed last window is none: %q, %s", m.status, m.winName(m.cur))
	}
	m = wide(t)
	enter(m, "/win 6")
	enter(m, "/win 5") // #qa, @bob the last window
	enter(m, "/close 1,3")
	enter(m, "/win last")
	if m.isErr || m.winName(m.cur) != "@bob" || m.cur != 4 {
		t.Fatalf("the last window stays when it stays open: %q, %s at %d", m.status, m.winName(m.cur), m.cur)
	}
	m = wide(t)
	enter(m, "/win 3")
	enter(m, "/win 5") // #qa, @alice the last window
	enter(m, "/close 4,5")
	enter(m, "/win last") // @alice is current again: none
	if !m.isErr || m.status != "no last window" || m.winName(m.cur) != "@alice" {
		t.Fatalf("the current window is not the last one: %q, %s", m.status, m.winName(m.cur))
	}
}

// TestCloseNoticesAndStatus: one notice per closed window in window 0, in
// window order, and a status naming one window, or several cut after four.
func TestCloseNoticesAndStatus(t *testing.T) {
	for _, c := range []struct {
		arg, status string
		closed      []string
	}{
		{"3", "closed @alice", []string{"@alice"}},
		{"4,2", "closed 2 windows: 2:#dev, 4:#ops", []string{"#dev", "#ops"}},
		{"5,1,3", "closed 3 windows: 1:#news, 3:@alice, 5:#qa", []string{"#news", "@alice", "#qa"}},
		{"2,2", "closed #dev", []string{"#dev"}},
		{"1-3,2", "closed 3 windows: 1:#news, 2:#dev, 3:@alice", []string{"#news", "#dev", "@alice"}},
		{"1-4", "closed 4 windows: 1:#news, 2:#dev, 3:@alice, 4:#ops", []string{"#news", "#dev", "@alice", "#ops"}},
		{"1-5", "closed 5 windows: 1:#news, 2:#dev, 3:@alice, 4:#ops, +1 more", []string{"#news", "#dev", "@alice", "#ops", "#qa"}},
		{"7,1-6", "closed 7 windows: 1:#news, 2:#dev, 3:@alice, 4:#ops, +3 more", []string{"#news", "#dev", "@alice", "#ops", "#qa", "@bob", "#web"}},
	} {
		m := wide(t)
		before := len(m.wins[0].notes)
		enter(m, "/close "+c.arg)
		if m.isErr || m.status != c.status {
			t.Errorf("/close %s: status %q; want %q", c.arg, m.status, c.status)
		}
		var want []string
		for _, name := range c.closed {
			want = append(want, "window closed: "+name)
		}
		if got := noteTexts(m.wins[0])[before:]; !slices.Equal(got, want) {
			t.Errorf("/close %s noted %q; want %q", c.arg, got, want)
		}
	}
}

// TestCloseNoticesGoToWindowZero: run from a chat window, the notices
// still go to window 0 and none to the window current before or after,
// whether that window stays open or closes.
func TestCloseNoticesGoToWindowZero(t *testing.T) {
	for _, c := range []struct {
		cur    int
		arg    string
		now    string // the current window after
		closed []string
	}{
		{6, "1,3", "@bob", []string{"#news", "@alice"}}, // stays open
		{3, "3-4", "#dev", []string{"@alice", "#ops"}},  // closes: #dev current
	} {
		m := wide(t)
		enter(m, fmt.Sprint("/win ", c.cur))
		notes := map[*win]int{} // the notices of every window before
		for _, w := range m.wins {
			notes[w] = len(w.notes)
		}
		zero := m.wins[0]
		enter(m, "/close "+c.arg)
		if m.isErr || m.winName(m.cur) != c.now {
			t.Fatalf("in window %d, /close %s: current %s (%q); want %s", c.cur, c.arg, m.winName(m.cur), m.status, c.now)
		}
		var want []string
		for _, name := range c.closed {
			want = append(want, "window closed: "+name)
		}
		if got := noteTexts(zero)[notes[zero]:]; !slices.Equal(got, want) {
			t.Errorf("in window %d, /close %s noted %q in window 0; want %q", c.cur, c.arg, got, want)
		}
		for _, w := range m.wins[1:] {
			if len(w.notes) != notes[w] {
				t.Errorf("in window %d, /close %s noted %q in %s", c.cur, c.arg, noteTexts(w)[notes[w]:], m.nameOf(w.chatID))
			}
		}
	}
}

// TestCloseListSavesTheLayout: the layout after the windows closed is
// what a new model over the same store opens.
func TestCloseListSavesTheLayout(t *testing.T) {
	f := wideFake()
	m := setup(t, f)
	for _, id := range []string{"c5", "c6", "c7", "c8"} {
		m.openWin(id)
	}
	enter(m, "/win 6") // @bob
	enter(m, "/close 2,4-5")
	if winNames(m) != "#news @alice @bob #web" || m.cur != 3 {
		t.Fatalf("windows %q, cur %d", winNames(m), m.cur)
	}
	l := storedLayout(t, m)
	if want := []string{"c3", "c2", "c7", "c8"}; !slices.Equal(l.Windows, want) || l.Current != 3 {
		t.Fatalf("stored layout %+v; want %v current 3", l, want)
	}
	again := New(Options{Store: m.opt.Store, Provider: f})
	if got := chatIDs(again); !slices.Equal(got, l.Windows) || again.cur != 3 {
		t.Fatalf("a new start opens %v at %d; want %v at 3", got, again.cur, l.Windows)
	}
}

func TestHelpNamesCloseLists(t *testing.T) {
	var line string
	for _, l := range strings.Split(command.HelpText, "\n") {
		if strings.HasPrefix(l, "  /close ") {
			line = l
		}
	}
	for _, want := range []string{"/close [N,M-K]", "a list or ranges (as 2,3-5)", "numbered as shown before", "one bad number closes none", "also /wc, /part"} {
		if !strings.Contains(line, want) {
			t.Errorf("/help's /close line lacks %q: %q", want, line)
		}
	}
}

// TestHelpNamesCloseOthers: /help lists /close-others on a line of its
// own, right after /close, saying what it keeps and that it takes no
// argument.
func TestHelpNamesCloseOthers(t *testing.T) {
	lines := strings.Split(command.HelpText, "\n")
	at := -1
	for i, l := range lines {
		if strings.HasPrefix(l, "  /close-others ") {
			at = i
		}
	}
	if at < 1 || !strings.HasPrefix(lines[at-1], "  /close ") {
		t.Fatalf("/help has no /close-others line right after /close: at %d", at)
	}
	for _, want := range []string{"every chat window but this one", "window 0 stays", "takes no argument"} {
		if !strings.Contains(lines[at], want) {
			t.Errorf("/help's /close-others line lacks %q: %q", want, lines[at])
		}
	}
}

// closedNotes are the window 0 notices of closing the windows names.
func closedNotes(names []string) []string {
	var want []string
	for _, name := range names {
		want = append(want, "window closed: "+name)
	}
	return want
}

// TestCloseOthersKeepsOnlyTheCurrentChatWindow: from any chat window,
// /close-others closes every other chat window at once, as /close with
// their numbers would: the current window stays current, as window 1,
// window 0 stays, each closed window is noted in window 0 in window order
// (none in the window kept), one status names them, and the stored layout
// is the one window left.
func TestCloseOthersKeepsOnlyTheCurrentChatWindow(t *testing.T) {
	for _, c := range []struct {
		cur    int
		status string
		closed []string
	}{
		{1, "closed 6 windows: 2:#dev, 3:@alice, 4:#ops, 5:#qa, +2 more", []string{"#dev", "@alice", "#ops", "#qa", "@bob", "#web"}},
		{4, "closed 6 windows: 1:#news, 2:#dev, 3:@alice, 5:#qa, +2 more", []string{"#news", "#dev", "@alice", "#qa", "@bob", "#web"}},
		{7, "closed 6 windows: 1:#news, 2:#dev, 3:@alice, 4:#ops, +2 more", []string{"#news", "#dev", "@alice", "#ops", "#qa", "@bob"}},
	} {
		m := wide(t)
		enter(m, fmt.Sprint("/win ", c.cur))
		server, kept := m.wins[0], m.wins[c.cur]
		notes := map[*win]int{}
		for _, w := range m.wins {
			notes[w] = len(w.notes)
		}
		cmd := enter(m, "/close-others")
		if m.isErr || m.status != c.status || cmd != nil {
			t.Errorf("in window %d, /close-others: %q (error %v, cmd %v); want %q", c.cur, m.status, m.isErr, cmd != nil, c.status)
		}
		if len(m.wins) != 2 || m.wins[0] != server || m.wins[1] != kept || m.cur != 1 {
			t.Fatalf("in window %d, /close-others left %q, current %d; want only the window kept, current 1", c.cur, winNames(m), m.cur)
		}
		if got := noteTexts(server)[notes[server]:]; !slices.Equal(got, closedNotes(c.closed)) {
			t.Errorf("in window %d, /close-others noted %q in window 0; want %q", c.cur, got, closedNotes(c.closed))
		}
		if len(kept.notes) != notes[kept] {
			t.Errorf("in window %d, /close-others noted %q in the window kept", c.cur, noteTexts(kept)[notes[kept]:])
		}
		if l := storedLayout(t, m); !slices.Equal(l.Windows, []string{kept.chatID}) || l.Current != 1 {
			t.Errorf("in window %d, /close-others stored %+v; want [%s] current 1", c.cur, l, kept.chatID)
		}
	}
}

// TestCloseOthersNamesWhatClosed: the status is /close's: one window by
// name, several as a list.
func TestCloseOthersNamesWhatClosed(t *testing.T) {
	m := setup(t, newFake()) // 1 #news, 2 #dev, 3 @alice
	enter(m, "/win 2")
	enter(m, "/close-others")
	if m.isErr || m.status != "closed 2 windows: 1:#news, 3:@alice" || winNames(m) != "#dev" {
		t.Fatalf("/close-others with two others: %q, left %q", m.status, winNames(m))
	}
	m = setup(t, newFake())
	enter(m, "/close 3")
	enter(m, "/win 1")
	before := len(m.wins[0].notes)
	enter(m, "/close-others")
	if m.isErr || m.status != "closed #dev" || winNames(m) != "#news" {
		t.Fatalf("/close-others with one other: %q, left %q", m.status, winNames(m))
	}
	if got := noteTexts(m.wins[0])[before:]; !slices.Equal(got, closedNotes([]string{"#dev"})) {
		t.Fatalf("/close-others with one other noted %q", got)
	}
}

// TestCloseOthersFocusesNothing: the window kept is not focused again: no
// read receipt goes, even with receipts on, and it keeps an unread mark
// it has (as [unread] on its own /win row leaves it).
func TestCloseOthersFocusesNothing(t *testing.T) {
	f := wideFake()
	m := setup(t, f)
	for _, id := range []string{"c5", "c6", "c7", "c8"} {
		m.openWin(id)
	}
	enter(m, "/read-receipts on")
	enter(m, "/win 3")   // @alice: focused, its unread count read
	m.wins[3].unread = 2 // marked unread again
	receipts := len(f.read)
	cmd := enter(m, "/close-others")
	runAll(m, cmd)
	if m.isErr || cmd != nil || len(f.read) != receipts {
		t.Fatalf("/close-others: %q, cmd %v, receipts %v (had %d)", m.status, cmd != nil, f.read, receipts)
	}
	if winNames(m) != "@alice" || m.cur != 1 || m.wins[1].unread != 2 {
		t.Fatalf("/close-others left %q current %d, unread %d; want @alice current, unread 2", winNames(m), m.cur, m.wins[1].unread)
	}
}

// TestCloseOthersDropsTheLastWindow: the last focused window goes when it
// is a chat window, as every other chat window closes; window 0 as the
// last window stays.
func TestCloseOthersDropsTheLastWindow(t *testing.T) {
	m := wide(t)
	enter(m, "/win 3")
	enter(m, "/win 6") // @bob, @alice the last window
	enter(m, "/close-others")
	enter(m, "/win last")
	if !m.isErr || m.status != "no last window" || m.winName(m.cur) != "@bob" {
		t.Fatalf("a closed last window is none: %q, %s", m.status, m.winName(m.cur))
	}
	m = wide(t)
	enter(m, "/win 6") // from window 0: the server window is the last window
	enter(m, "/close-others")
	enter(m, "/win last")
	if m.isErr || m.cur != 0 {
		t.Fatalf("window 0 stays the last window: %q, current %d", m.status, m.cur)
	}
}

// TestCloseOthersSavesTheLayout: what a new model over the same store
// opens is the window kept, current.
func TestCloseOthersSavesTheLayout(t *testing.T) {
	f := wideFake()
	m := setup(t, f)
	for _, id := range []string{"c5", "c6", "c7", "c8"} {
		m.openWin(id)
	}
	enter(m, "/win 6") // @bob
	enter(m, "/close-others")
	again := New(Options{Store: m.opt.Store, Provider: f})
	if got := chatIDs(again); !slices.Equal(got, []string{"c7"}) || again.cur != 1 {
		t.Fatalf("a new start opens %v at %d; want [c7] at 1", got, again.cur)
	}
}

// unchanged records what /close-others must leave alone when it closes
// nothing, and plants a layout in the store that no window shows, so a
// layout written by the command, even an equal one, is seen.
type unchanged struct {
	wins    []*win
	cur     int
	last    int
	notes   map[*win]int
	unread  map[*win]int
	planted layout
}

func recordUnchanged(t *testing.T, m *Model) unchanged {
	t.Helper()
	u := unchanged{wins: slices.Clone(m.wins), cur: m.cur, last: m.lastWindow(), notes: map[*win]int{}, unread: map[*win]int{},
		planted: layout{Windows: []string{"planted"}, Current: 42}}
	for _, w := range m.wins {
		u.notes[w], u.unread[w] = len(w.notes), w.unread
	}
	if err := m.opt.Store.SetState(layoutKey, u.planted); err != nil {
		t.Fatal(err)
	}
	return u
}

func (u unchanged) check(t *testing.T, m *Model, what string) {
	t.Helper()
	if !slices.Equal(m.wins, u.wins) || m.cur != u.cur || m.lastWindow() != u.last {
		t.Errorf("%s changed the windows: %q, current %d, last %d", what, winNames(m), m.cur, m.lastWindow())
	}
	for _, w := range m.wins {
		if len(w.notes) != u.notes[w] {
			t.Errorf("%s noted %q in %s", what, noteTexts(w)[u.notes[w]:], m.nameOf(w.chatID))
		}
		if w.unread != u.unread[w] {
			t.Errorf("%s changed the unread count of %s: %d, was %d", what, m.nameOf(w.chatID), w.unread, u.unread[w])
		}
	}
	if l := storedLayout(t, m); !reflect.DeepEqual(l, u.planted) {
		t.Errorf("%s wrote the stored layout: %+v", what, l)
	}
}

// TestCloseOthersAloneIsANoOp: when the current window is the only chat
// window there is nothing to close: the status says so, as information,
// not an error, and nothing changes: no notice, the windows, the current
// and the last window, unread counts, and the stored layout, which is
// not written.
func TestCloseOthersAloneIsANoOp(t *testing.T) {
	f := newFake()
	m := setup(t, f)
	enter(m, "/read-receipts on")
	enter(m, "/close 2-3")
	enter(m, "/win 1")   // #news alone; window 0 the last window
	m.wins[1].unread = 2 // marked unread again
	receipts := len(f.read)
	u := recordUnchanged(t, m)
	cmd := enter(m, "/close-others")
	runAll(m, cmd)
	if m.isErr || m.status != "no other chat window to close" || cmd != nil || len(f.read) != receipts {
		t.Fatalf("/close-others alone: %q (error %v, cmd %v, receipts %v)", m.status, m.isErr, cmd != nil, f.read)
	}
	u.check(t, m, "/close-others alone")
}

// TestCloseOthersRefusedInWindowZero: window 0 is no chat window to keep,
// so /close-others there is the error of every command that needs a chat
// window and nothing changes, with chat windows open or none.
func TestCloseOthersRefusedInWindowZero(t *testing.T) {
	const want = "the server window has no chat; /win to a chat first"
	m := wide(t)
	enter(m, "/win 3")
	enter(m, "/win 0") // @alice the last window
	u := recordUnchanged(t, m)
	cmd := enter(m, "/close-others")
	if !m.isErr || m.status != want || cmd != nil {
		t.Fatalf("/close-others in window 0: %q (error %v); want the error %q", m.status, m.isErr, want)
	}
	u.check(t, m, "/close-others in window 0")
	m = setup(t, newFake())
	enter(m, "/close 1-3")
	u = recordUnchanged(t, m)
	enter(m, "/close-others")
	if !m.isErr || m.status != want {
		t.Fatalf("/close-others in window 0 alone: %q (error %v); want the error %q", m.status, m.isErr, want)
	}
	u.check(t, m, "/close-others in window 0 alone")
}

// TestCloseOthersTakesNoArgument: any argument is an error and nothing
// changes; it is checked first, so window 0 and a window alone give it
// too. Spaces alone are no argument.
func TestCloseOthersTakesNoArgument(t *testing.T) {
	const want = "/close-others takes no argument; /close 2,3-5 closes the windows given"
	for _, c := range []struct {
		cur  int
		arg  string
		solo bool // the current window is the only chat window
	}{
		{4, "3", false},
		{4, "4", false},
		{4, "1-3", false},
		{4, "2,3-5", false},
		{4, "x", false},
		{4, "all", false},
		{4, "--force", false},
		{4, "0", false},
		{0, "3", false},
		{1, "3", true},
	} {
		m := wide(t)
		if c.solo {
			enter(m, "/close 2-7")
		}
		enter(m, fmt.Sprint("/win ", c.cur))
		u := recordUnchanged(t, m)
		line := "/close-others " + c.arg
		cmd := enter(m, line)
		if !m.isErr || m.status != want || cmd != nil {
			t.Errorf("%q in window %d: %q (error %v); want the error %q", line, c.cur, m.status, m.isErr, want)
		}
		u.check(t, m, fmt.Sprintf("%q in window %d", line, c.cur))
	}
	m := wide(t)
	enter(m, "/win 2")
	enter(m, "/close-others   ")
	if m.isErr || winNames(m) != "#dev" {
		t.Fatalf("/close-others with spaces after it: %q, left %q", m.status, winNames(m))
	}
}

// TestTabCompletesCloseOthers: Tab completes /close-others from its
// prefix, offers nothing after it (it takes no argument: no chat names),
// and /close Tab now stops at /close, the prefix both commands share, as
// /clip does beside /clipboard; a second Tab lists both.
func TestTabCompletesCloseOthers(t *testing.T) {
	m := setup(t, newFake())
	for typed, want := range map[string]string{
		"/close-o":         "/close-others ",
		"/CLOSE-OT":        "/close-others ",
		"/close-others ":   "/close-others ",
		"/close-others al": "/close-others al",
		"/close":           "/close",
		"/close al":        "/close al", // numbers
	} {
		m.input, m.comp = nil, nil
		typ(m, typed)
		press(m, tea.KeyTab)
		if got := string(m.input); got != want {
			t.Errorf("%q: got %q, want %q", typed, got, want)
		}
	}
	m.input, m.comp = nil, nil
	typ(m, "/close")
	press(m, tea.KeyTab)
	press(m, tea.KeyTab)
	if v := body(m); !strings.Contains(v, "/close  /close-others") {
		t.Errorf("the second Tab lists /close and /close-others:\n%s", v)
	}
}

// TestHintNamesCloseOthers: a start hint makes /close-others known; read
// in window 0, which refuses it, it says it works in a chat window
// (TestHintsAreWellFormed checks the form).
func TestHintNamesCloseOthers(t *testing.T) {
	for _, h := range hintPool {
		if strings.Contains(h, "/close-others ") && strings.Contains(h, "chat window") {
			return
		}
	}
	t.Fatal("no start hint names /close-others")
}
