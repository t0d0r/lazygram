package tui

import (
	"bytes"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lazygram/internal/domain"
	"lazygram/internal/store"
)

var typingT0 = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

// typingFake is the fake provider with a typing feed.
type typingFake struct {
	*fake
	ch chan domain.Typing
}

func (f *typingFake) Typing() <-chan domain.Typing { return f.ch }

// clockAt pins the model's clock, turns the lights on (setup leaves them
// off, so the running light is exercised here) and widens the screen so
// the full wording fits beside the other status segments.
func clockAt(m *Model, at time.Time) {
	m.now = func() time.Time { return at }
	m.fx = true
	m.width = 140
}

func report(m *Model, chatID, who string, kind domain.TypingKind, at time.Time) tea.Cmd {
	_, cmd := m.Update(typingMsg{ev: domain.Typing{ChatID: chatID, Who: who, Kind: kind, At: at}})
	return cmd
}

func TestTypingShowsInTheChatWindowAndTheServerWindow(t *testing.T) {
	f := newFake()
	m := setup(t, f)
	clockAt(m, typingT0)
	if cmd := report(m, "c1", "ada", domain.TypingText, typingT0); cmd == nil {
		t.Fatal("a report arms the expiry timer and the light")
	}
	if !m.typingExpiry || !m.typingAnim {
		t.Fatalf("timers: expiry=%v anim=%v", m.typingExpiry, m.typingAnim)
	}
	if v := m.View(); !strings.Contains(v, "ada (#dev) is typing") {
		t.Fatalf("server window names the chat:\n%s", v)
	}
	enter(m, "/win #dev")
	if v := m.View(); !strings.Contains(v, "ada is typing") || strings.Contains(v, "(#dev)") {
		t.Fatalf("chat window:\n%s", v)
	}
	report(m, "c1", "bob", domain.TypingText, typingT0.Add(time.Second))
	if v := m.View(); !strings.Contains(v, "ada and bob are typing") {
		t.Fatalf("two members:\n%s", v)
	}
	report(m, "c1", "ada", domain.TypingRecord, typingT0.Add(2*time.Second))
	if v := m.View(); !strings.Contains(v, "ada and bob are typing") {
		t.Fatalf("mixed kinds say typing:\n%s", v)
	}
	report(m, "c1", "bob", domain.TypingRecord, typingT0.Add(2*time.Second))
	if v := m.View(); !strings.Contains(v, "ada and bob are recording") {
		t.Fatalf("a common kind is named:\n%s", v)
	}
	report(m, "c1", "cy", domain.TypingRecord, typingT0.Add(3*time.Second))
	report(m, "c1", "dee", domain.TypingRecord, typingT0.Add(3*time.Second))
	if v := m.View(); !strings.Contains(v, "ada, bob and 2 others are recording") {
		t.Fatalf("many members:\n%s", v)
	}
	// Another chat's reports do not show in this window.
	report(m, "c2", "", domain.TypingText, typingT0.Add(3*time.Second))
	if v := m.View(); strings.Contains(v, "someone") {
		t.Fatalf("another chat leaks:\n%s", v)
	}
	// A stop report and a message from a member end their entries.
	report(m, "c1", "cy", domain.TypingStop, typingT0.Add(4*time.Second))
	report(m, "c1", "dee", domain.TypingStop, typingT0.Add(4*time.Second))
	m.arrived([]domain.Message{mk("c1", "9", "bob", "done", 9)})
	if v := m.View(); !strings.Contains(v, "ada is recording") {
		t.Fatalf("after a stop and a message:\n%s", v)
	}
	// The server window shows every listed chat, an unnamed member as
	// "someone"; a chat lazygram does not list stays out of sight.
	enter(m, "/win 0")
	report(m, "telegram:-1009", "", domain.TypingText, typingT0.Add(4*time.Second))
	if v := m.View(); !strings.Contains(v, "ada (#dev) and someone (@alice) are typing") || strings.Contains(v, "telegram:-1009") {
		t.Fatalf("server window shows every listed chat:\n%s", v)
	}
	// A message from a sender nobody reported by name ends the chat's
	// unnamed entry (the member was unknown at report time).
	m.arrived([]domain.Message{mk("c2", "10", "alice", "here", 10)})
	if v := m.View(); strings.Contains(v, "someone") || !strings.Contains(v, "ada (#dev) is recording") {
		t.Fatalf("after an unnamed member's message:\n%s", v)
	}
	// Handling reports and arrivals sends no message: the fake's Send was
	// never called (outbound typing does not exist: provider.Provider has
	// no such method and the connector never calls messages.setTyping).
	if len(f.sent) != 0 {
		t.Fatalf("sent: %v", f.sent)
	}
}

func TestTypingServerWindowOrdersByFirstReport(t *testing.T) {
	m := setup(t, newFake())
	clockAt(m, typingT0)
	report(m, "c2", "alice", domain.TypingText, typingT0)
	report(m, "c1", "ada", domain.TypingText, typingT0.Add(time.Second))
	if v := m.View(); !strings.Contains(v, "alice (@alice) and ada (#dev) are typing") {
		t.Fatalf("the earlier report comes first, whatever the chat ids:\n%s", v)
	}
	// A chat lazygram does not list neither shows nor runs the light.
	m2 := setup(t, newFake())
	clockAt(m2, typingT0)
	report(m2, "telegram:-1009", "x", domain.TypingText, typingT0)
	if m2.typingAnim || !m2.typingExpiry || strings.Contains(m2.View(), "x (") {
		t.Fatalf("unlisted chat: anim=%v expiry=%v", m2.typingAnim, m2.typingExpiry)
	}
}

func TestTypingSegmentFitsTheRoom(t *testing.T) {
	m := setup(t, newFake())
	clockAt(m, typingT0)
	enter(m, "/win #dev")
	report(m, "c1", "Bartholomew Montgomery-Smith", domain.TypingText, typingT0)
	report(m, "c1", "Alexandra Konstantinopoulou", domain.TypingText, typingT0.Add(time.Second))
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 30})
	if v := m.statusLine(); !strings.Contains(v, "Bartholomew Montgomery-Smith and Alexandra Konstantinopoulou are typing") || !strings.Contains(v, "msgs") {
		t.Fatalf("wide: %s", v)
	}
	m.Update(tea.WindowSizeMsg{Width: 70, Height: 30})
	if v := m.statusLine(); !strings.Contains(v, "2 typing") || strings.Contains(v, "Bartholomew") || !strings.Contains(v, "msgs") || !strings.Contains(v, "synced") {
		t.Fatalf("narrow: the short form and the counters stay: %s", v)
	}
	if m.typingSegment(0) != "" || m.typingSegment(5) != "" {
		t.Fatal("no room, no indicator")
	}
}

func TestTypingLightFollowsFx(t *testing.T) {
	m := setup(t, newFake())
	clockAt(m, typingT0) // lights on
	enter(m, "/fx")      // lights off
	if m.fx {
		t.Fatal("fx off expected")
	}
	if cmd := report(m, "c1", "ada", domain.TypingText, typingT0); cmd == nil || m.typingAnim || !m.typingExpiry {
		t.Fatalf("lights off: expiry only: anim=%v expiry=%v", m.typingAnim, m.typingExpiry)
	}
	if v := m.View(); !strings.Contains(v, "ada (#dev) is typing") {
		t.Fatalf("the indicator still reads, plain:\n%s", v)
	}
	if _, cmd := m.Update(typingAnimMsg{}); cmd != nil || m.typingAnim || m.typingStep != 0 {
		t.Fatal("no light while the lights are off")
	}
	enter(m, "/fx") // lights on: the light starts with the next message
	if !m.fx || !m.typingAnim {
		t.Fatalf("lights on: fx=%v anim=%v", m.fx, m.typingAnim)
	}
}

func TestTypingExpiresOnTheClock(t *testing.T) {
	m := setup(t, newFake())
	clockAt(m, typingT0)
	report(m, "c1", "ada", domain.TypingText, typingT0)
	report(m, "c1", "bob", domain.TypingText, typingT0.Add(2*time.Second))
	if !m.typingExpiry {
		t.Fatal("expiry armed")
	}
	// The timer fires at ada's deadline: she goes, bob stays, a new timer is armed.
	clockAt(m, typingT0.Add(domain.DefaultTypingTTL))
	_, cmd := m.Update(typingExpireMsg{})
	if cmd == nil || !m.typingExpiry {
		t.Fatal("bob's deadline re-arms the timer")
	}
	if got := m.typing.Typists("c1"); len(got) != 1 || got[0].Who != "bob" {
		t.Fatalf("after ada's deadline: %v", got)
	}
	// A renewal pushes bob's deadline; the old timer finds nothing and re-arms.
	report(m, "c1", "bob", domain.TypingText, typingT0.Add(7*time.Second))
	clockAt(m, typingT0.Add(8*time.Second))
	if _, cmd := m.Update(typingExpireMsg{}); cmd == nil || len(m.typing.Typists("c1")) != 1 {
		t.Fatal("renewed: still there, timer re-armed")
	}
	clockAt(m, typingT0.Add(13*time.Second))
	if _, cmd := m.Update(typingExpireMsg{}); cmd != nil || m.typingExpiry || !m.typing.Empty() {
		t.Fatalf("all gone: cmd=%v expiry=%v empty=%v", cmd != nil, m.typingExpiry, m.typing.Empty())
	}
	if v := m.View(); strings.Contains(v, "typing") {
		t.Fatalf("nothing shown:\n%s", v)
	}
}

func TestTypingLightBouncesAndRunsOnlyWhenShown(t *testing.T) {
	// The light runs right, turns at the last cell and comes back.
	var seq []int
	var dirs []int
	for step := 0; step < 11; step++ {
		p, d := bounce(step, 6)
		seq, dirs = append(seq, p), append(dirs, d)
	}
	wantSeq := []int{0, 1, 2, 3, 4, 5, 4, 3, 2, 1, 0}
	wantDir := []int{1, 1, 1, 1, 1, 1, -1, -1, -1, -1, 1}
	for i := range wantSeq {
		if seq[i] != wantSeq[i] || dirs[i] != wantDir[i] {
			t.Fatalf("bounce: pos %v dir %v, want %v %v", seq, dirs, wantSeq, wantDir)
		}
	}
	if p, d := bounce(5, 1); p != 0 || d != 1 {
		t.Fatalf("a one-cell word: %d %d", p, d)
	}

	m := setup(t, newFake())
	clockAt(m, typingT0)
	if _, cmd := m.Update(typingAnimMsg{}); cmd != nil || m.typingAnim {
		t.Fatal("no report, no light")
	}
	report(m, "c1", "ada", domain.TypingText, typingT0)
	if !m.typingAnim {
		t.Fatal("the light runs in the server window, which shows the report")
	}
	step := m.typingStep
	if _, cmd := m.Update(typingAnimMsg{}); cmd == nil || m.typingStep != step+1 || !m.typingAnim {
		t.Fatal("a tick advances the light and re-arms")
	}
	// The light lands on the verb: with step 1 the second letter of "typing".
	m.typingStep = 1
	if v := m.View(); !strings.Contains(v, "ada (#dev) is typing") {
		t.Fatalf("the lit verb still reads as a word:\n%s", v)
	}
	// A window without reports: the next tick stops the light; coming back re-arms it.
	enter(m, "/win @alice")
	if _, cmd := m.Update(typingAnimMsg{}); cmd != nil || m.typingAnim {
		t.Fatal("nothing shown here: the light stops")
	}
	dev, _ := m.winFor("c1")
	if cmd := m.switchTo(dev); cmd == nil || !m.typingAnim {
		t.Fatal("switching to a window that shows the report starts the light")
	}
	// The expiry timer is separate: it keeps running while a report is pending.
	if !m.typingExpiry {
		t.Fatal("expiry stays armed")
	}
	// Closing the current window shows another one: the light follows.
	enter(m, "/win @alice")
	m.Update(typingAnimMsg{}) // nothing shown in @alice: the light stops
	if m.typingAnim {
		t.Fatal("stopped")
	}
	typ(m, "/close")
	if cmd := press(m, tea.KeyEnter); cmd == nil || !m.typingAnim || m.wins[m.cur].chatID != "c1" {
		t.Fatalf("closing @alice shows #dev and starts the light: anim=%v chat=%q", m.typingAnim, m.wins[m.cur].chatID)
	}
}

func TestTypingFeedIsReadUntilClosed(t *testing.T) {
	st, _ := store.Open(store.Memory, bytes.Repeat([]byte{7}, 32))
	defer func() { _ = st.Close() }()
	plain := New(Options{Store: st, Provider: newFake()})
	if plain.typingInit() != nil || plain.typingCh != nil {
		t.Fatal("a service without a feed is left alone")
	}
	none := &typingFake{fake: newFake()} // a nil feed: nothing to wait on
	m := New(Options{Store: st, Provider: none})
	if m.typingInit() != nil || m.typingCh != nil {
		t.Fatal("a nil feed is left alone")
	}
	tf := &typingFake{fake: newFake(), ch: make(chan domain.Typing, 1)}
	m = New(Options{Store: st, Provider: tf})
	m.revealAll()
	clockAt(m, typingT0)
	m.Update(chatsMsg{chats: tf.chats}) // the server window shows listed chats only
	cmd := m.typingInit()
	if cmd == nil || m.typingCh == nil {
		t.Fatal("subscribed")
	}
	tf.ch <- domain.Typing{ChatID: "c2", Who: "alice", Kind: domain.TypingText, At: typingT0}
	msg := cmd()
	ev, ok := msg.(typingMsg)
	if !ok || ev.ev.Who != "alice" {
		t.Fatalf("got %T %+v", msg, msg)
	}
	_, next := m.Update(msg)
	if next == nil || len(m.typing.Typists("c2")) != 1 {
		t.Fatal("the report is applied and the feed read again")
	}
	close(tf.ch)
	if msg := waitTyping(tf.ch)(); msg != (typingDoneMsg{}) {
		t.Fatalf("a closed feed ends the reads: %T", msg)
	}
	m.Update(typingDoneMsg{})
	if m.typingCh != nil {
		t.Fatal("no more reads after the feed closed")
	}
	// Reports still expire and show after the feed closed: with both
	// timers fired, a report arms them again, without another read.
	m.typingExpiry, m.typingAnim = false, false
	_, cmd = m.Update(typingMsg{ev: domain.Typing{ChatID: "c2", Who: "alice", Kind: domain.TypingText, At: typingT0.Add(time.Second)}})
	if cmd == nil || !m.typingExpiry || !m.typingAnim || m.typingCh != nil {
		t.Fatalf("timers: cmd=%v expiry=%v anim=%v", cmd != nil, m.typingExpiry, m.typingAnim)
	}
}

func TestTypingTextWording(t *testing.T) {
	nameOf := func(id string) string { return "#" + id }
	one := []shownTypist{{Typist: domain.Typist{Who: "ada", Kind: domain.TypingUpload}}}
	if n, v := typingText(one, nameOf); n != "ada is" || v != "uploading" {
		t.Fatalf("%q %q", n, v)
	}
	two := []shownTypist{{Typist: domain.Typist{Who: "ada", Kind: domain.TypingChoose}, chatID: "x"}, {Typist: domain.Typist{Who: "bob", Kind: domain.TypingChoose}, chatID: "y"}}
	if n, v := typingText(two, nameOf); n != "ada (#x) and bob (#y) are" || v != "choosing" {
		t.Fatalf("%q %q", n, v)
	}
	three := append(two, shownTypist{Typist: domain.Typist{Who: "cy", Kind: domain.TypingText}})
	if n, v := typingText(three, nameOf); n != "ada (#x), bob (#y) and 1 other are" || v != "typing" {
		t.Fatalf("%q %q", n, v)
	}
	if n, v := typingText(nil, nameOf); n != "" || v != "" {
		t.Fatalf("%q %q", n, v)
	}
}
