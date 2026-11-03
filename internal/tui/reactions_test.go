package tui

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"lazygram/internal/domain"
	"lazygram/internal/provider"
	"lazygram/internal/provider/demo"
	"lazygram/internal/store"
)

// Reaction emoji as the service sends them, kept as escapes so this file
// stays plain ASCII.
const (
	upEmoji    = "\U0001F44D" // thumbs up
	heartEmoji = "\u2764"     // red heart, without the variation selector
	fireEmoji  = "\U0001F525" // fire
	partyEmoji = "\U0001F389" // party popper
)

// reactingFake is a fake whose service pushes reactions and answers polls.
type reactingFake struct {
	*fake
	ch      chan domain.Change
	polls   []string                   // PollReactions calls as "chat:id,id"
	answer  map[string][]domain.Change // what a poll of a chat returns
	pollErr error                      // returned by the next poll instead
}

func newReactingFake() *reactingFake {
	return &reactingFake{fake: newFake(), ch: make(chan domain.Change, 4), answer: map[string][]domain.Change{}}
}

func (f *reactingFake) Changes() <-chan domain.Change { return f.ch }

func (f *reactingFake) PollReactions(_ context.Context, chatID string, ids []string) ([]domain.Change, error) {
	f.polls = append(f.polls, chatID+":"+strings.Join(ids, ","))
	if err := f.pollErr; err != nil {
		f.pollErr = nil
		return nil, err
	}
	return f.answer[chatID], nil
}

// reactedChange is what the service reports: a message's reactions as
// they are now and who it names, pushed (own) or polled.
func reactedChange(chat, id string, own bool, rs []domain.Reaction, who ...domain.Reactor) domain.Change {
	return domain.Change{Kind: domain.Reacted, ChatID: chat, Msg: domain.Message{ID: id, ChatID: chat, Reactions: rs}, Reactors: who, Own: own}
}

func rx(emoji string, n int) domain.Reaction { return domain.Reaction{Emoji: emoji, Count: n} }
func rxMine(emoji string, n int) domain.Reaction {
	return domain.Reaction{Emoji: emoji, Count: n, Mine: true}
}
func by(who, id, emoji string) domain.Reactor   { return domain.Reactor{Who: who, ID: id, Emoji: emoji} }
func rs(r ...domain.Reaction) []domain.Reaction { return r }

// polled hands the model one poll's answer.
func polled(m *Model, c ...domain.Change) { m.Update(polledMsg{chatID: c[0].ChatID, changes: c}) }

// pushed hands the model one change from the service's feed.
func pushed(m *Model, c domain.Change) { m.Update(changeMsg{c: c}) }

// chatNotices are window 0's notices about the chats' activity.
func chatNotices(m *Model) []string {
	var out []string
	for _, n := range m.wins[0].notes {
		if n.chat {
			out = append(out, n.text)
		}
	}
	return out
}

// tickReactions delivers one reaction tick and, when it starts a poll,
// runs the poll and hands its answer to the model; the next tick it arms
// is not waited for. It reports whether a poll was made.
func tickReactions(m *Model) bool {
	_, cmd := m.Update(reactTickMsg{})
	if !m.react.running {
		return false // only the next tick, which would wait
	}
	msg := cmd()
	for {
		batch, ok := msg.(tea.BatchMsg)
		if !ok {
			break
		}
		msg = batch[len(batch)-1]() // the poll comes after the next tick
	}
	m.Update(msg)
	return true
}

// msgOf is the model's copy of a message.
func msgOf(m *Model, chat, id string) domain.Message {
	for _, x := range m.msgs {
		if x.ChatID == chat && x.ID == id {
			return x
		}
	}
	return domain.Message{}
}

// TestReactionsShowAfterTheText: the counts follow the text (after the
// caption, on its last line, before the buttons) in window 0 and in the
// chat windows, in the service's order, the account's own highlighted.
func TestReactionsShowAfterTheText(t *testing.T) {
	f := newFake()
	f.msgs[0].Reactions = rs(rx(upEmoji, 2), rxMine(heartEmoji, 1)) // ada's first dev line
	f.msgs[3].Reactions = rs(rx(fireEmoji, 1))                      // the bot's question with buttons
	photo := mk("c2", "9", "alice", "the view", 4)
	photo.Media = &domain.Media{Kind: "photo"}
	photo.Reactions = rs(rx(heartEmoji, 3))
	f.msgs = append(f.msgs, photo)
	m := setup(t, f)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	devLine := "<ada> first dev line  [" + upEmoji + " 2 " + heartEmoji + " 1]"
	photoLine := "<alice> [photo] the view  [" + heartEmoji + " 3]"
	botLine := "<bot> deploy?  [" + fireEmoji + " 1]"
	shows := func(where string, want ...string) string {
		t.Helper()
		v := ansi.Strip(body(m))
		for _, w := range want {
			if !strings.Contains(v, w) {
				t.Errorf("%s lacks %q:\n%s", where, w, v)
			}
		}
		return v
	}
	v := shows("window 0", devLine, photoLine, botLine)
	if b, buttons := strings.Index(v, botLine), strings.Index(v, "[1] Yes"); b < 0 || buttons < b {
		t.Errorf("the buttons come after the counts:\n%s", v)
	}
	if strings.Contains(v, "second dev line  [") || strings.Contains(v, "hi there  [") {
		t.Errorf("a message without reactions shows none:\n%s", v)
	}
	enter(m, "/win #dev")
	shows("#dev", devLine)
	m.input = nil
	enter(m, "/win @alice")
	shows("@alice", photoLine, botLine)

	// The account's own reaction is drawn in the highlight style, the
	// others' are not.
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(old) })
	m.input = nil
	enter(m, "/win #dev")
	m.rcache = &renderCache{} // drawn without colour so far
	v = body(m)
	if !strings.Contains(v, hiStyle.Render(heartEmoji+" 1")) || strings.Contains(v, hiStyle.Render(upEmoji+" 2")) {
		t.Errorf("only the account's own reaction is highlighted:\n%q", v)
	}
}

// TestReactionsWrapLikeTheText: counts that do not fit the last row of
// the text continue on rows of their own, indented like the text, an
// emoji never parted from its count, and no row wider than the window.
func TestReactionsWrapLikeTheText(t *testing.T) {
	msg := mk("c1", "1", "ada", "a line of text that fills the row", 0)
	msg.Reactions = rs(rx(upEmoji, 12), rx(heartEmoji, 3), rx(fireEmoji, 7), rx(domain.CustomReaction, 2))
	units := []string{"[" + upEmoji + " 12", heartEmoji + " 3", fireEmoji + " 7", "* 2]"}
	for _, width := range []int{8, 10, 24, 30, 40, 100} {
		var rows []string
		for _, l := range renderLines([]domain.Message{msg}, nil, lineOpts{width: width}) {
			rows = append(rows, ansi.Strip(l))
		}
		all := strings.Join(rows, "\n")
		for i, r := range rows {
			if lipgloss.Width(r) > width && width >= 24 { // narrower, the text itself keeps 10 columns
				t.Errorf("%d columns: row %d is %d wide: %q", width, i, lipgloss.Width(r), r)
			}
			if strings.TrimSpace(r) == "" {
				t.Errorf("%d columns: row %d is blank", width, i)
			}
			if i > 1 && !strings.HasPrefix(r, "      ") { // after the day separator and the first line
				t.Errorf("%d columns: row %d is not indented like the text: %q", width, i, r)
			}
		}
		at := strings.Index(all, "row") // they follow the text's last word
		for _, u := range units {
			i := strings.Index(all[at:], u)
			if i < 0 {
				t.Fatalf("%d columns: %q is not drawn whole, in order:\n%s", width, u, all)
			}
			at += i + len(u)
		}
		if width == 100 && len(rows) != 2 { // the day separator and the one line
			t.Errorf("a wide window keeps them on the text's row:\n%s", all)
		}
	}
	// No text at all: still two spaces after the sender.
	bare := mk("c1", "2", "ada", "", 0)
	bare.Reactions = rs(rx(upEmoji, 1))
	if got := ansi.Strip(renderLines([]domain.Message{bare}, nil, lineOpts{width: 40})[1]); !strings.HasSuffix(got, "<ada>  ["+upEmoji+" 1]") {
		t.Errorf("a message without text: %q", got)
	}
	// Text from the service cannot style the screen: an emoji is cut to
	// its first character, control characters gone.
	for in, want := range map[string]string{"\x1b[31m" + upEmoji: "[", "": "?", " \t": "?", upEmoji + upEmoji: upEmoji, heartEmoji + "\ufe0f": heartEmoji + "\ufe0f"} {
		if got := emojiText(in); got != want {
			t.Errorf("emojiText(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestReactionCountsAreNeverStale: the render cache keys a message on its
// reactions, so a count that changed is drawn anew even when nothing else
// dropped the cache; a change from the service updates every window.
func TestReactionCountsAreNeverStale(t *testing.T) {
	m := setupProvider(t, newReactingFake())
	enter(m, "/win #dev")
	_ = body(m) // drawn and cached
	for _, n := range []int{4, 5} {
		for i := range m.msgs {
			if m.msgs[i].Key() == mk("c1", "1", "", "", 0).Key() {
				m.msgs[i].Reactions = rs(rx(upEmoji, n))
			}
		}
		if v := ansi.Strip(body(m)); !strings.Contains(v, "first dev line  ["+upEmoji+" "+strconv.Itoa(n)+"]") {
			t.Fatalf("the count %d is drawn:\n%s", n, v)
		}
	}
	polled(m, reactedChange("c1", "2", false, rs(rx(fireEmoji, 2))))
	if v := ansi.Strip(body(m)); !strings.Contains(v, "second dev line  ["+fireEmoji+" 2]") {
		t.Fatalf("a polled change shows in the window:\n%s", v)
	}
	if cached, _ := m.opt.Store.Messages("c1", 0); !slices.Equal(cached[1].Reactions, rs(rx(fireEmoji, 2))) || cached[1].Text != "second dev line" {
		t.Fatalf("and in the cache, text kept: %+v", cached[1])
	}
	m.switchTo(0)
	if v := ansi.Strip(body(m)); !strings.Contains(v, "second dev line  ["+fireEmoji+" 2]") {
		t.Fatalf("and in window 0:\n%s", v)
	}
}

// TestReactionNoticesInWindow0: a reaction someone added is told in
// window 0 with the chat's tag, naming who did it when the service names
// someone new, else with the counts; the message is quoted by its author
// and first words, "your" when it is the account's, and "a message" when
// lazygram does not hold it.
func TestReactionNoticesInWindow0(t *testing.T) {
	f := newReactingFake()
	photo := mk("c2", "9", "alice", "", 5)
	photo.Media = &domain.Media{Kind: "photo"}
	f.msgs = append(f.msgs, mk("c2", "8", "me", "see you at noon then", 4), photo,
		mk("c3", "8", "wire", "Breaking: a long headline that goes on and on", 6))
	m := setupProvider(t, f)
	grace, alan, ann := by("grace", "u-grace", upEmoji), by("alan", "u-alan", heartEmoji), by("ann", "u-ann", upEmoji)
	alice := func(e string) domain.Reactor { return by("Alice Smith", "c2", e) }
	steps := []struct {
		want  string
		apply func()
	}{
		{`#dev: @grace reacted ` + upEmoji + ` to ada "first dev line"`, func() {
			polled(m, reactedChange("c1", "1", false, rs(rx(upEmoji, 1)), grace))
		}},
		// Several new ones in one change, in the service's order; grace is known.
		{`#dev: @alan reacted ` + heartEmoji + `, @ann reacted ` + upEmoji + ` to ada "first dev line"`, func() {
			polled(m, reactedChange("c1", "1", false, rs(rx(upEmoji, 2), rx(heartEmoji, 1)), alan, ann, grace))
		}},
		// A count rose, nobody new is named: the counts, whole.
		{`#dev: ` + upEmoji + ` 3 ` + heartEmoji + ` 1 on ada "first dev line"`, func() {
			polled(m, reactedChange("c1", "1", false, rs(rx(upEmoji, 3), rx(heartEmoji, 1)), alan, ann, grace))
		}},
		// A channel names nobody; a long text is cut at a word.
		{`#news: ` + fireEmoji + ` 13 ` + heartEmoji + ` 4 on wire "Breaking: a long headline that..."`, func() {
			polled(m, reactedChange("c3", "8", false, rs(rx(fireEmoji, 13), rx(heartEmoji, 4))))
		}},
		// The account's own message, pushed; the reactor's chat is listed.
		{`@alice: @alice reacted ` + upEmoji + ` to your "see you at noon then"`, func() {
			pushed(m, reactedChange("c2", "8", true, rs(rx(upEmoji, 1)), alice(upEmoji)))
		}},
		{`@alice: ` + upEmoji + ` 2 on your "see you at noon then"`, func() {
			pushed(m, reactedChange("c2", "8", true, rs(rx(upEmoji, 2)), alice(upEmoji)))
		}},
		// One more, and the service lists two it had not named: the newest
		// is the one that came.
		{`@alice: @dora reacted ` + upEmoji + ` to your "see you at noon then"`, func() {
			pushed(m, reactedChange("c2", "8", true, rs(rx(upEmoji, 3)), by("dora", "u-dora", upEmoji), by("erin", "u-erin", upEmoji), alice(upEmoji)))
		}},
		// No text: the attachment token stands for it.
		{`@alice: @bob reacted ` + heartEmoji + ` to alice "[photo]"`, func() {
			polled(m, reactedChange("c2", "9", false, rs(rx(heartEmoji, 1)), by("bob", "u-bob", heartEmoji)))
		}},
		// A message lazygram does not hold, pushed: the account's own.
		{`@alice: @alice reacted ` + heartEmoji + ` to your message`, func() {
			pushed(m, reactedChange("c2", "999", true, rs(rx(heartEmoji, 1)), alice(heartEmoji)))
		}},
		{`@alice: ` + heartEmoji + ` 1 on your message`, func() {
			pushed(m, reactedChange("c2", "998", true, rs(rx(heartEmoji, 1))))
		}},
		// Polled twice and not held: the first word is a first sighting.
		{`#dev: @carol reacted ` + upEmoji + ` to a message`, func() {
			polled(m, reactedChange("c1", "997", false, rs(rx(upEmoji, 1)), by("bob", "u-bob", upEmoji)))
			polled(m, reactedChange("c1", "997", false, rs(rx(upEmoji, 2)), by("carol", "u-carol", upEmoji), by("bob", "u-bob", upEmoji)))
		}},
		// Reactors the service names without an id are told apart by name.
		{`#dev: @pat reacted ` + upEmoji + ` to alan "second dev line"`, func() {
			polled(m, reactedChange("c1", "2", false, rs(rx(upEmoji, 1)), by("pat", "", upEmoji)))
		}},
		{`#dev: @quinn reacted ` + upEmoji + ` to alan "second dev line"`, func() {
			polled(m, reactedChange("c1", "2", false, rs(rx(upEmoji, 2)), by("quinn", "", upEmoji), by("pat", "", upEmoji)))
		}},
		// /names by name names the chats and a listed reactor by title.
		{`@Alice Smith: @Alice Smith reacted ` + upEmoji + ` to alice "hi there"`, func() {
			enter(m, "/names")
			polled(m, reactedChange("c2", "1", false, rs(rx(upEmoji, 1)), alice(upEmoji)))
		}},
	}
	for i, s := range steps {
		before := len(chatNotices(m))
		s.apply()
		got := chatNotices(m)
		if len(got) != before+1 || got[len(got)-1] != s.want {
			t.Fatalf("step %d: want the notice %q, got %q", i, s.want, got[before:])
		}
	}
	if v := ansi.Strip(body(m)); !strings.Contains(v, "-!- @Alice Smith: @Alice Smith reacted "+upEmoji+" to alice") {
		t.Fatalf("a -!- notice in window 0:\n%s", v)
	}
}

// TestHelpTellsOfReactions: /help says how reactions show and when
// window 0 tells of them.
func TestHelpTellsOfReactions(t *testing.T) {
	m := setup(t, newFake())
	enter(m, "/help")
	drain(m)
	for _, n := range m.wins[0].notes {
		if strings.HasPrefix(n.text, "Reactions show after a message as [emoji count ...], yours highlighted; window 0 says when someone adds one (with /feed on).") {
			return
		}
	}
	t.Fatal("/help does not tell of reactions")
}

// TestReactionMemoryIsBounded: what the session remembers of reactors and
// of messages the cache does not hold starts over past maxReactors.
func TestReactionMemoryIsBounded(t *testing.T) {
	m := setupProvider(t, newReactingFake())
	r := &m.react
	for i := range maxReactors + 1 {
		r.remember("k", []domain.Reactor{{ID: strconv.Itoa(i), Emoji: upEmoji}})
	}
	if n := len(r.named); n > maxReactors || n == 0 {
		t.Fatalf("%d reactors remembered", n)
	}
	for i := range maxReactors + 1 {
		r.unheld[strconv.Itoa(i)] = nil
	}
	pushed(m, reactedChange("c2", "995", true, rs(rx(upEmoji, 1))))
	if len(r.unheld) != 1 {
		t.Fatalf("%d messages not held remembered", len(r.unheld))
	}
}

// TestFirstWordsQuoteTheStart: about 30 columns of the text, cut at a
// word, on one line.
func TestFirstWordsQuoteTheStart(t *testing.T) {
	for text, want := range map[string]string{
		"no problem, ordering for you":                      "no problem, ordering for you",
		"tip: /win 2 switches windows, Tab completes names": "tip: /win 2 switches windows...",
		"Breaking: a long headline that goes on and on":     "Breaking: a long headline that...",
		"two\nlines  of\ttext":                              "two lines of text",
		"averyveryverylongwordwithoutanyspaceatallinit":     "averyveryverylongwordwithoutan...",
		"a verylongwordthatgoesonandonforever":              "a verylongwordthatgoesonandonf...", // not cut back to "a"
		"\x1b[2Jclear":                                      "[2Jclear",
	} {
		if got := firstWords(domain.Message{Text: text}); got != want {
			t.Errorf("firstWords(%q) = %q, want %q", text, got, want)
		}
	}
}

// TestReactionsSayNothingUnlessSomeoneAddsOne: a first sighting (a cached
// message's first poll this session, a fetch, a heal, a fresh message), a
// removal, a lowered count, a change identical to what is known and the
// account's own reaction update the counts and say nothing.
func TestReactionsSayNothingUnlessSomeoneAddsOne(t *testing.T) {
	st := memStore(t)
	if _, err := st.Add([]domain.Message{mk("c1", "5", "alan", "cached last week", -600), mk("c1", "6", "ann", "also cached", -590)}); err != nil {
		t.Fatal(err)
	}
	f := newFake()
	m := startWith(t, st, f)
	step := pinClock(m)
	quiet := func(what string, n int) {
		t.Helper()
		if got := chatNotices(m); len(got) != n {
			t.Fatalf("%s: %d notices, want %d: %q", what, len(got), n, got)
		}
	}
	counts := func(chat, id string, want []domain.Reaction) {
		t.Helper()
		if got := msgOf(m, chat, id).Reactions; !slices.Equal(got, want) {
			t.Fatalf("%s/%s shows %v, want %v", chat, id, got, want)
		}
	}
	// A cached message's first word this session: it may be days old.
	polled(m, reactedChange("c1", "5", false, rs(rx(upEmoji, 3)), by("x", "u-x", upEmoji)))
	quiet("first sighting", 0)
	counts("c1", "5", rs(rx(upEmoji, 3)))
	polled(m, reactedChange("c1", "5", false, rs(rx(upEmoji, 4)), by("y", "u-y", upEmoji), by("x", "u-x", upEmoji)))
	quiet("then a new one", 1)
	// Removed and lowered: the counts follow.
	polled(m, reactedChange("c1", "5", false, rs(rx(upEmoji, 3))))
	polled(m, reactedChange("c1", "5", false, nil))
	quiet("removals", 1)
	counts("c1", "5", nil)
	enter(m, "/win #dev")
	if v := ansi.Strip(body(m)); !strings.Contains(v, "cached last week\n") {
		t.Fatalf("no counts left on the line:\n%s", v)
	}
	// The same change twice: one notice.
	c := reactedChange("c1", "1", false, rs(rx(upEmoji, 1)), by("x", "u-x", upEmoji))
	polled(m, c)
	polled(m, c)
	quiet("identical", 2)
	// The account's own reaction is no news, nor is it named; others with
	// it are.
	polled(m, reactedChange("c1", "1", false, rs(rxMine(upEmoji, 2)), by("x", "u-x", upEmoji)))
	quiet("own", 2)
	polled(m, reactedChange("c1", "1", false, rs(rxMine(upEmoji, 4)), by("me", "", upEmoji), by("z", "u-z", upEmoji), by("w", "u-w", upEmoji), by("x", "u-x", upEmoji)))
	if got := chatNotices(m); len(got) != 3 || got[2] != `#dev: @z reacted `+upEmoji+`, @w reacted `+upEmoji+` to ada "first dev line"` {
		t.Fatalf("two others with the account: %q", got)
	}
	// A window's first fetch and a heal bring reactions silently.
	m.input = nil
	enter(m, "/win 0")
	older, alice := mk("c2", "0", "alice", "older hello", -30), mk("c2", "1", "alice", "hi there", 1)
	alice.Reactions = rs(rx(heartEmoji, 9))
	f.recent["c2"] = []domain.Message{older, alice}
	m.input = nil
	runAll(m, enter(m, "/win @alice"))
	counts("c2", "1", rs(rx(heartEmoji, 9)))
	alice.Reactions = rs(rx(heartEmoji, 10))
	f.recent["c2"] = []domain.Message{older, alice}
	step(healEvery + time.Second)
	runAll(m, m.healCmd(m.wins[m.cur]))
	counts("c2", "1", rs(rx(heartEmoji, 10)))
	// A fresh message with reactions.
	fresh := mk("c2", "20", "alice", "brand new", 30)
	fresh.Reactions = rs(rx(fireEmoji, 5))
	f.msgs = append(f.msgs, fresh)
	m.Update(m.syncCmd()())
	counts("c2", "20", rs(rx(fireEmoji, 5)))
	quiet("fetch, heal and a fresh message", 3)
	// Learned by the fetch alone: the next word on it is compared.
	polled(m, reactedChange("c2", "0", false, rs(rx(heartEmoji, 1))))
	quiet("after a fetch", 4)
	// A message the cache does not hold is compared with its last report.
	gone := reactedChange("c2", "996", true, rs(rx(heartEmoji, 2)))
	pushed(m, gone)
	pushed(m, gone)
	pushed(m, reactedChange("c2", "996", true, rs(rx(heartEmoji, 1))))
	quiet("one push of three about a message not held", 5)
	// An edit the service reports carries the reactions too: learned.
	pushed(m, domain.Change{Kind: domain.Edited, ChatID: "c1", Msg: domain.Message{ID: "6", ChatID: "c1", Text: "also cached, edited"}})
	polled(m, reactedChange("c1", "6", false, rs(rx(upEmoji, 1))))
	quiet("after an edit", 6)
}

// TestAnUnchangedCopyTeachesItsReactions: a sync or a fetch that brings a
// cached message back as the cache has it still tells lazygram its
// reactions, so the next one someone adds to it is told.
func TestAnUnchangedCopyTeachesItsReactions(t *testing.T) {
	st := memStore(t)
	f := newFake()
	older := mk("c2", "0", "alice", "older hello", -30)
	older.Reactions = rs(rx(heartEmoji, 2))
	f.recent["c2"] = []domain.Message{older, f.msgs[1]}
	// Cached in an earlier session, as the service still has them.
	if _, err := st.Add([]domain.Message{f.msgs[0], older, f.msgs[1]}); err != nil {
		t.Fatal(err)
	}
	m := startWith(t, st, f) // the sync brings ada's line back the same
	polled(m, reactedChange("c1", "1", false, rs(rx(upEmoji, 1)), by("grace", "u-grace", upEmoji)))
	runAll(m, enter(m, "/win @alice")) // and the fetch alice's older line
	if got := msgOf(m, "c2", "0").Reactions; !slices.Equal(got, older.Reactions) {
		t.Fatalf("the fetch shows %v", got)
	}
	polled(m, reactedChange("c2", "0", false, rs(rx(heartEmoji, 3)), by("bob", "u-bob", heartEmoji)))
	want := []string{
		`#dev: @grace reacted ` + upEmoji + ` to ada "first dev line"`,
		`@alice: @bob reacted ` + heartEmoji + ` to alice "older hello"`,
	}
	if got := chatNotices(m); !slices.Equal(got, want) {
		t.Fatalf("notices %q, want %q", got, want)
	}
}

// TestReactionNoticesFollowTheFeed: with /feed off window 0 does not show
// them, as it shows no chat messages; the counts still change, and /feed
// on shows the notices again.
func TestReactionNoticesFollowTheFeed(t *testing.T) {
	m := setupProvider(t, newReactingFake())
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 60})
	enter(m, "/feed off")
	polled(m, reactedChange("c1", "1", false, rs(rx(upEmoji, 1)), by("grace", "u-grace", upEmoji)))
	notice := `-!- #dev: @grace reacted ` + upEmoji + ` to ada "first dev line"`
	if v := ansi.Strip(body(m)); strings.Contains(v, "reacted") {
		t.Fatalf("window 0 shows no chat activity with /feed off:\n%s", v)
	}
	if len(chatNotices(m)) != 1 {
		t.Fatal("the notice is kept for /feed on")
	}
	enter(m, "/win #dev")
	if v := ansi.Strip(body(m)); !strings.Contains(v, "first dev line  ["+upEmoji+" 1]") {
		t.Fatalf("the count changes all the same:\n%s", v)
	}
	m.input = nil
	enter(m, "/win 0")
	m.input = nil
	enter(m, "/feed on")
	if v := ansi.Strip(body(m)); !strings.Contains(v, notice) {
		t.Fatalf("/feed on shows it:\n%s", v)
	}
}

// TestReactionNoticesOpenNoWindowCountNothing: a notice is no message: no
// window opens, nothing counts as unread or activity, no status.
func TestReactionNoticesOpenNoWindowCountNothing(t *testing.T) {
	f := newReactingFake()
	f.msgs = append(f.msgs, mk("c2", "8", "me", "see you at noon then", 4))
	m := setupProvider(t, f)
	i, _ := m.winFor("c2")
	m.closeWin(i)
	wins, act, seen, status := len(m.wins), m.activity(), m.seen, m.status
	var unread []int
	for _, w := range m.wins {
		unread = append(unread, w.unread)
	}
	pushed(m, reactedChange("c2", "8", true, rs(rx(upEmoji, 1)), by("alice", "c2", upEmoji)))
	polled(m, reactedChange("c1", "1", false, rs(rx(upEmoji, 1)), by("grace", "u-grace", upEmoji)))
	if len(chatNotices(m)) != 2 {
		t.Fatalf("two notices: %q", chatNotices(m))
	}
	if _, w := m.winFor("c2"); w != nil || len(m.wins) != wins {
		t.Fatal("no window opens")
	}
	for k, w := range m.wins {
		if w.unread != unread[k] {
			t.Errorf("window %d: unread %d, was %d", k, w.unread, unread[k])
		}
	}
	if m.activity() != act || m.seen != seen || m.status != status {
		t.Errorf("activity %q (was %q), seen %d (was %d), status %q (was %q)", m.activity(), act, m.seen, seen, m.status, status)
	}
}

// TestPollAsksAboutTheMessagesOnScreen: a chat window's poll asks, in one
// call, about the messages it shows that others sent: not those scrolled
// off, not the account's own.
func TestPollAsksAboutTheMessagesOnScreen(t *testing.T) {
	f := newReactingFake()
	f.msgs = nil
	for i := range 40 {
		who := []string{"ada", "alan", "me", "grace", "ann"}[i%5]
		f.msgs = append(f.msgs, mk("c1", strconv.Itoa(100+i), who, fmt.Sprintf("dev line %02d", i), i))
	}
	f.recent["c1"] = domain.Tail(f.msgs, 10)
	m := setupProvider(t, f)
	runAll(m, enter(m, "/win #dev"))
	check := func(what string) {
		t.Helper()
		v := ansi.Strip(body(m)) + "\n"
		n := len(f.polls)
		if !tickReactions(m) || len(f.polls) != n+1 {
			t.Fatalf("%s: one call: %q", what, f.polls)
		}
		var want []string
		for _, x := range f.msgs {
			if x.Sender != "me" && strings.Contains(v, x.Text+"\n") {
				want = append(want, x.ID)
			}
		}
		if got := f.polls[n]; got != "c1:"+strings.Join(want, ",") || len(want) < 5 {
			t.Fatalf("%s: asked %q, on screen %v:\n%s", what, got, want, v)
		}
		if strings.Contains(v, "dev line 00\n") {
			t.Fatalf("%s: the window must not show everything:\n%s", what, v)
		}
	}
	check("at the bottom")
	press(m, tea.KeyPgUp)
	check("scrolled up")
	if strings.Contains(f.polls[1], ",139") {
		t.Fatalf("the newest line is off screen: %q", f.polls[1])
	}
}

// TestPollAsksOnlyAboutWhatHasRolledIn: while a window's first screen
// still rolls in, a message below the rows shown so far is not on screen.
func TestPollAsksOnlyAboutWhatHasRolledIn(t *testing.T) {
	f := newReactingFake()
	m := setupProvider(t, f)
	runAll(m, enter(m, "/win #dev"))
	w := m.wins[m.cur]
	w.revealing, w.revealed = true, 2 // the day and the first line
	m.revealTick = true               // its next step is not run here
	if v := ansi.Strip(body(m)); !strings.Contains(v, "first dev line") || strings.Contains(v, "second dev line") {
		t.Fatalf("one line rolled in:\n%s", v)
	}
	if !tickReactions(m) || !slices.Equal(f.polls, []string{"c1:1"}) {
		t.Fatalf("asked about %q", f.polls)
	}
}

// TestPollSkipsTheRowsOfNotices: a notice under a message is not the
// message: with only the notice on screen nothing is asked, whether the
// message was drawn anew or came from the render cache.
func TestPollSkipsTheRowsOfNotices(t *testing.T) {
	f := newReactingFake()
	m := setupProvider(t, f)
	runAll(m, enter(m, "/win #dev"))
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 5}) // one content row
	for _, how := range []string{"drawn", "from the cache"} {
		if v := ansi.Strip(body(m)); !strings.Contains(v, "-!- fetched 0 older") {
			t.Fatalf("%s: the last notice alone:\n%s", how, v)
		}
		if tickReactions(m) {
			t.Fatalf("%s: asked %q", how, f.polls)
		}
	}
}

// TestPollWindow0TakesTheChatsInTurn: window 0 asks about one chat per
// tick, the chats on screen in turn; a chat the service refuses is
// skipped for the session.
func TestPollWindow0TakesTheChatsInTurn(t *testing.T) {
	f := newReactingFake()
	f.msgs = append(f.msgs, mk("c2", "8", "me", "my own line", 4))
	m := setupProvider(t, f)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 60})
	body(m)
	notes := len(m.wins[0].notes)
	for range 3 {
		tickReactions(m)
	}
	f.pollErr = provider.ErrNotPollable // @alice's turn
	tickReactions(m)
	for range 2 {
		tickReactions(m)
	}
	want := []string{"c1:1,2", "c2:1,7", "c1:1,2", "c2:1,7", "c1:1,2", "c1:1,2"}
	if !slices.Equal(f.polls, want) {
		t.Fatalf("polls %q, want %q", f.polls, want)
	}
	if m.isErr || len(m.wins[0].notes) != notes {
		t.Fatalf("a refusal says nothing: %q", m.status)
	}
}

// TestPollWaitsWhenTheServiceAsks: after a FLOOD_WAIT nothing is polled
// until the wait has passed; errors say nothing.
func TestPollWaitsWhenTheServiceAsks(t *testing.T) {
	f := newReactingFake()
	m := setupProvider(t, f)
	step := pinClock(m)
	body(m)
	notes, status := len(m.wins[0].notes), m.status
	f.pollErr = &provider.WaitError{Wait: time.Minute}
	if !tickReactions(m) {
		t.Fatal("the first poll is made")
	}
	step(59 * time.Second)
	if tickReactions(m) || len(f.polls) != 1 {
		t.Fatalf("no poll while the service waits: %q", f.polls)
	}
	step(2 * time.Second)
	f.pollErr = errors.New("connection reset")
	if !tickReactions(m) || len(f.polls) != 2 {
		t.Fatalf("polls again once it has passed: %q", f.polls)
	}
	if !tickReactions(m) || len(f.polls) != 3 {
		t.Fatalf("another error stops nothing: %q", f.polls)
	}
	if len(m.wins[0].notes) != notes || m.status != status || m.isErr {
		t.Fatalf("nothing is said about a poll: %q", m.status)
	}
}

// TestPollWaitsForSyncsAndPolls: a tick during a sync polls when the sync
// ends; a tick while a poll is on its way makes none, then or later.
func TestPollWaitsForSyncsAndPolls(t *testing.T) {
	f := newReactingFake()
	m := setupProvider(t, f)
	body(m)
	m.syncing = true
	m.Update(reactTickMsg{})
	if m.react.running || len(f.polls) != 0 {
		t.Fatal("no poll beside a sync")
	}
	_, cmd := m.Update(syncedMsg{at: time.Now()})
	if !m.react.running || cmd == nil {
		t.Fatal("the poll follows the sync's end")
	}
	m.Update(cmd())
	if len(f.polls) != 1 || m.react.running {
		t.Fatalf("one poll: %q", f.polls)
	}
	if _, cmd := m.Update(syncedMsg{at: time.Now()}); cmd != nil {
		t.Fatal("one call per tick: the next sync's end makes none")
	}
	// A poll on its way takes the next tick.
	_, cmd = m.Update(reactTickMsg{})
	poll := cmd().(tea.BatchMsg)[1]
	_, cmd = m.Update(reactTickMsg{})
	if pollIn(cmd) {
		t.Fatal("a tick while a poll is on its way makes none")
	}
	m.Update(poll())
	if len(f.polls) != 2 {
		t.Fatalf("one call per tick: %q", f.polls)
	}
	if _, cmd := m.Update(syncedMsg{at: time.Now()}); cmd != nil || m.react.running {
		t.Fatal("the tick that came during the poll is not made up for")
	}
}

// pollIn reports whether cmd holds a poll, running what it holds; a tick
// in it, which would wait for its time, is given up on after a moment.
func pollIn(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	got := make(chan tea.Msg, 1)
	go func() { got <- cmd() }()
	select {
	case msg := <-got:
		if batch, ok := msg.(tea.BatchMsg); ok {
			return slices.ContainsFunc(batch, pollIn)
		}
		_, ok := msg.(polledMsg)
		return ok
	case <-time.After(200 * time.Millisecond):
		return false
	}
}

// TestPollNeedsMessagesOnScreen: nothing is asked when the window shows
// no message, only own ones, or a list instead of the window.
func TestPollNeedsMessagesOnScreen(t *testing.T) {
	f := newReactingFake()
	f.msgs = append(f.msgs, mk("c3", "8", "me", "my own post", 4))
	m := setupProvider(t, f)
	enter(m, "/win #news") // only the account's post, the fetch not run
	body(m)
	if tickReactions(m) {
		t.Fatalf("own messages are pushed, not polled: %q", f.polls)
	}
	m.input = nil
	enter(m, "/win 0")
	body(m) // messages of others on screen, then a list over them
	m.input = nil
	enter(m, "/list")
	body(m)
	if tickReactions(m) {
		t.Fatalf("a list shows no messages: %q", f.polls)
	}
	press(m, tea.KeyEsc)
	m.input = nil
	enter(m, "/redraw")
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 6}) // the bot's buttons and the account's post
	body(m)
	if !tickReactions(m) || !slices.Equal(f.polls, []string{"c2:7"}) {
		t.Fatalf("the window again, what it shows: %q", f.polls)
	}
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 60})
	enter(m, "/feed off")
	enter(m, "/redraw")
	body(m)
	if tickReactions(m) {
		t.Fatalf("window 0 with /feed off shows no messages: %q", f.polls)
	}
}

// TestReactionPollsRunWithTheSyncs: the ticks start when the service takes
// polls and background syncs run, at Telegram's 15 to 30 seconds.
func TestReactionPollsRunWithTheSyncs(t *testing.T) {
	st := func() *store.Store { return memStore(t) }
	if New(Options{Store: st(), Provider: newFake(), Poll: time.Second}).reactInit() != nil {
		t.Error("a service that takes no polls gets none")
	}
	if New(Options{Store: st(), Provider: newReactingFake()}).reactInit() != nil {
		t.Error("no background syncs, no polls")
	}
	if New(Options{Store: st(), Provider: newReactingFake(), Poll: time.Second}).reactInit() == nil {
		t.Error("polls start with the syncs")
	}
	if reactionPollEvery < 15*time.Second || reactionPollEvery > 30*time.Second {
		t.Errorf("every %s", reactionPollEvery)
	}
}

// TestDemoShowsReactions: lazygram demo shows the counts its lines carry,
// a reaction to the account's own line pushed 25 s after start as a count
// and a notice, and one to someone else's line found by a poll.
func TestDemoShowsReactions(t *testing.T) {
	clock := time.Now().Truncate(time.Second)
	p := demo.NewAt(func() time.Time { return clock })
	m := setupProvider(t, p)
	m.now = func() time.Time { return clock }
	m.changesInit()
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 80})
	if v := ansi.Strip(body(m)); !strings.Contains(v, "release candidate tagged  ["+partyEmoji+" 3 "+upEmoji+" 1]") {
		t.Fatalf("the start counts show:\n%s", v)
	}
	clock = clock.Add(26 * time.Second)
	m.Update(m.syncCmd()())
	select {
	case c := <-m.changeCh:
		pushed(m, c)
	default:
		t.Fatal("the reaction to the account's line is pushed")
	}
	if got := chatNotices(m); len(got) != 1 || got[0] != `@linus: @linus reacted `+upEmoji+` to your "no problem, ordering for you"` {
		t.Fatalf("the push is told: %q", got)
	}
	runAll(m, enter(m, "/win @linus"))
	if v := ansi.Strip(body(m)); !strings.Contains(v, "no problem, ordering for you  ["+upEmoji+" 1]") {
		t.Fatalf("and counted:\n%s", v)
	}
	m.input = nil
	runAll(m, enter(m, "/win #dev"))
	clock = clock.Add(75 * time.Second)
	body(m)
	if !tickReactions(m) {
		t.Fatal("#dev is polled")
	}
	if got := chatNotices(m); len(got) != 2 || got[1] != `#dev: @grace reacted `+upEmoji+` to ada "tip: /win 2 switches windows..."` {
		t.Fatalf("the poll finds grace's reaction: %q", got)
	}
}
