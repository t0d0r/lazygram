package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// chatLines are the texts of newFake's messages, one per chat line.
var chatLines = []string{"first dev line", "hi there", "second dev line", "deploy?"}

// showsAny reports which of the texts the screen shows.
func showsAny(v string, texts ...string) []string {
	var out []string
	for _, s := range texts {
		if strings.Contains(v, s) {
			out = append(out, s)
		}
	}
	return out
}

func TestFeedIsOnByDefault(t *testing.T) {
	m := setup(t, newFake())
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 60}) // room for the banner, the messages and the notices
	var saved bool
	if found, _ := m.opt.Store.State(feedKey, &saved); found || !m.feed {
		t.Fatalf("on with nothing stored: feed=%v stored=%v", m.feed, found)
	}
	if got := showsAny(body(m), chatLines...); len(got) != len(chatLines) {
		t.Fatalf("window 0 interleaves every chat: shows %q:\n%s", got, body(m))
	}
	enter(m, "/feed")
	if m.isErr || m.status != "feed on: window 0 shows every chat's messages as they arrive; /feed off" {
		t.Fatalf("no argument shows the setting: %q", m.status)
	}
	if found, _ := m.opt.Store.State(feedKey, &saved); found {
		t.Fatal("showing the setting keeps nothing")
	}
}

// TestFeedUnreadableMeansOn: a stored value that does not read as on or
// off starts the feed on, as nothing stored does.
func TestFeedUnreadableMeansOn(t *testing.T) {
	st := memStore(t)
	if err := st.SetState(feedKey, "nonsense"); err != nil {
		t.Fatal(err)
	}
	m := startWith(t, st, newFake())
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 60}) // room for the banner, the messages and the notices
	if !m.feed {
		t.Fatal("an unreadable stored value means on")
	}
	for _, n := range m.wins[0].notes {
		if n.text == feedOffNotice {
			t.Fatal("no feed notice while the feed is on")
		}
	}
	if got := showsAny(ansi.Strip(body(m)), chatLines...); len(got) != len(chatLines) {
		t.Fatalf("window 0 interleaves every chat: shows %q:\n%s", got, body(m))
	}
}

func TestFeedOffShowsOnlyNoticesInWindow0(t *testing.T) {
	f := newFake()
	m := setup(t, f)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 80}) // room for the banner, the messages and the notices
	m.fail("sync: offline")                             // an error notice
	enter(m, "/feed off")
	if m.feed || m.isErr || m.status != "feed off: window 0 shows only notices; chat messages stay in their own windows" {
		t.Fatalf("off: feed=%v %q", m.feed, m.status)
	}
	v := ansi.Strip(body(m))
	if got := showsAny(v, chatLines...); len(got) != 0 {
		t.Fatalf("window 0 shows chat messages with /feed off: %q\n%s", got, v)
	}
	for _, want := range []string{"-!- /help lists commands", "-!- hint: /list", "-!- window 1 opened for #news", "-!- 4 chats, 1 with unread messages", "-!- error: sync: offline"} {
		if !strings.Contains(v, want) {
			t.Errorf("the notice %q stays:\n%s", want, v)
		}
	}
	enter(m, "/feed")
	if m.status != "feed off: window 0 shows only notices; chat messages stay in their own windows; /feed on" {
		t.Fatalf("no argument shows off: %q", m.status)
	}
	// The chat windows still show their messages.
	_, dev := m.winFor("c1")
	i, _ := m.winFor("c1")
	m.switchTo(i)
	if got := showsAny(body(m), "first dev line", "second dev line"); len(got) != 2 {
		t.Fatalf("#dev's window shows its messages: %q\n%s", got, body(m))
	}
	// Messages that arrive while off, from a group, a person and a
	// channel: counted, their windows open, the status says where they
	// went, they are live; window 0 shows none of them.
	m.switchTo(0)
	alice, _ := m.winFor("c2")
	m.closeWin(alice)
	_, news := m.winFor("c3")
	before, newsBefore := dev.unread, news.unread
	f.msgs = append(f.msgs, mk("c1", "3", "ada", "third dev line", 5), mk("c2", "8", "alice", "late hello", 6),
		mk("c3", "9", "wire", "late headline", 7))
	m.Update(m.syncCmd()())
	if dev.unread != before+1 || news.unread != newsBefore+1 || !strings.Contains(m.status, "3 new messages in") {
		t.Fatalf("arrival while off: unread %d (was %d), #news %d (was %d), status %q", dev.unread, before, news.unread, newsBefore, m.status)
	}
	if _, w := m.winFor("c2"); w == nil {
		t.Fatal("a window still opens for a chat that gets a message")
	}
	if !m.live[mk("c1", "3", "ada", "", 5).Key()] {
		t.Fatal("the message is recorded as live")
	}
	if got := showsAny(body(m), "third dev line", "late hello", "late headline"); len(got) != 0 {
		t.Fatalf("window 0 shows %q with /feed off", got)
	}
	// /search in window 0 still searches every chat.
	enter(m, "/search line")
	if got := showsAny(body(m), "first dev line", "third dev line"); len(got) != 2 {
		t.Fatalf("/search in window 0 finds every chat's messages: %q\n%s", got, body(m))
	}
	press(m, tea.KeyEsc)
	// On again: this session's messages, those that came while off included.
	enter(m, "/feed on")
	if !m.feed || m.isErr || !strings.HasPrefix(m.status, "feed on: ") {
		t.Fatalf("on: feed=%v %q", m.feed, m.status)
	}
	if got := showsAny(body(m), append(chatLines, "third dev line", "late hello", "late headline")...); len(got) != len(chatLines)+3 {
		t.Fatalf("window 0 shows the session's messages again: %q\n%s", got, body(m))
	}
}

func TestFeedOffIsKeptAndSaidAtStart(t *testing.T) {
	st := memStore(t)
	m := startWith(t, st, newFake())
	for _, n := range m.wins[0].notes {
		if n.text == feedOffNotice {
			t.Fatal("no feed notice while the feed is on")
		}
	}
	enter(m, "/feed off")
	var saved bool
	if found, err := st.State(feedKey, &saved); !found || err != nil || saved {
		t.Fatalf("off is kept: found=%v saved=%v err=%v", found, saved, err)
	}
	var sizes []int
	m2 := New(Options{Store: st, Provider: newFake(), Rand: lastPicks(&sizes)})
	m2.fx = false
	m2.revealAll()
	m2.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m2.Update(m2.chatsCmd()())
	m2.Update(m2.syncCmd()())
	if m2.feed {
		t.Fatal("a new model on the same store starts with the feed off")
	}
	// The notice follows the start hints, which keep their order.
	var texts []string
	for _, n := range m2.wins[0].notes {
		texts = append(texts, n.text)
	}
	at := -1
	for i, s := range texts {
		if s == feedOffNotice {
			if at >= 0 {
				t.Fatal("one feed notice per start")
			}
			at = i
		}
	}
	hints := hintsIn(m2.wins[0])
	if at < 0 || len(hints) != 4 || texts[at-1] != hints[3] || hints[0] != hintsFixed[0] || hints[1] != hintsFixed[1] {
		t.Fatalf("the notice comes right after the four hints: %q", texts)
	}
	if v := ansi.Strip(body(m2)); !strings.Contains(v, "-!- "+feedOffNotice) || len(showsAny(v, chatLines...)) != 0 {
		t.Fatalf("start with the feed off:\n%s", v)
	}
	enter(m2, "/feed on")
	m3 := startWith(t, st, newFake())
	if !m3.feed {
		t.Fatal("on again is kept too")
	}
}

func TestFeedTakesOnlyOnOrOff(t *testing.T) {
	m := setup(t, newFake())
	enter(m, "/feed maybe")
	if !m.isErr || m.status != "/feed takes on or off" || !m.feed {
		t.Fatalf("a bad argument is an error and changes nothing: feed=%v %q", m.feed, m.status)
	}
	var saved bool
	if found, _ := m.opt.Store.State(feedKey, &saved); found {
		t.Fatal("a bad argument keeps nothing")
	}
	enter(m, "/feed OFF")
	if m.feed || m.isErr {
		t.Fatalf("the argument is case-insensitive: %v %q", m.feed, m.status)
	}
}

func TestFeedKeepsWindow0ScrollValid(t *testing.T) {
	f := newFake()
	for i := range 60 {
		f.msgs = append(f.msgs, mk("c1", "x"+itoa(i), "ada", "busy line "+itoa(i), 10+i))
	}
	m := setup(t, f)
	limit := func() int { lines, _ := m.richLinesOf(m.wins[0]); return max(len(lines)-m.contentHeight(), 0) }
	for range 3 {
		press(m, tea.KeyPgUp)
	}
	m.View()
	if m.wins[0].scroll == 0 {
		t.Fatal("window 0 is scrolled back")
	}
	enter(m, "/feed off")
	if s := m.wins[0].scroll; s > limit() {
		t.Fatalf("off: scroll %d past the %d lines window 0 has", s, limit())
	}
	if v := m.View(); strings.Contains(v, "scrolled up") != (m.wins[0].scroll > 0) {
		t.Fatalf("the indicator follows the clamped offset %d:\n%s", m.wins[0].scroll, v)
	}
	// From a chat window too: window 0's own offset is kept valid.
	enter(m, "/feed on")
	m.wins[0].scroll = 1 << 20
	m.switchTo(2)
	enter(m, "/feed off")
	if s := m.wins[0].scroll; s > limit() {
		t.Fatalf("off from another window: scroll %d past %d", s, limit())
	}
	enter(m, "/feed on")
	if s := m.wins[0].scroll; s > limit() || s < 0 {
		t.Fatalf("on: scroll %d outside 0..%d", s, limit())
	}
}

func TestTabCompletesFeedSettings(t *testing.T) {
	m := setup(t, newFake())
	for typed, want := range map[string]string{
		"/feed of": "/feed off ",
		"/feed o":  "/feed o",  // on and off share the o
		"/feed al": "/feed al", // a setting, not a chat
	} {
		m.input, m.curBack = nil, 0
		typ(m, typed)
		press(m, tea.KeyTab)
		if got := string(m.input); got != want {
			t.Errorf("%q: got %q, want %q", typed, got, want)
		}
	}
	m.input, m.curBack = nil, 0
	typ(m, "/feed o")
	press(m, tea.KeyTab)
	press(m, tea.KeyTab)
	if v := body(m); !strings.Contains(v, "-!- off  on") {
		t.Errorf("the second Tab lists on and off:\n%s", v)
	}
}
