package tui

import (
	"errors"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"lazygram/internal/command"
	"lazygram/internal/domain"
	"lazygram/internal/provider"
)

// The reactions /like sends, as Telegram's standard set lists them.
const (
	eLike    = "\U0001F44D"
	eDislike = "\U0001F44E"
	eLove    = "\u2764"
	eFire    = "\U0001F525"
	eLaugh   = "\U0001F601"
	eSad     = "\U0001F622"
	eParty   = "\U0001F389"
	eClap    = "\U0001F44F"
	eThanks  = "\U0001F64F"
	eOK      = "\U0001F44C"
	eThink   = "\U0001F914"
	eEyes    = "\U0001F440"
)

const (
	wantLikeErr = "/like takes like, dislike, love, fire, laugh, sad, party, clap, thanks, ok, think or eyes"
	likeNoChat  = "the server window has no chat; /win to a chat first"
)

// likeModel is a model in #dev (window 2), whose messages are ada's
// "first dev line", alan's "second dev line" and the account's "my reply"
// after them.
func likeModel(t *testing.T) (*Model, *fake) {
	t.Helper()
	f := newFake()
	f.msgs = append(f.msgs, mk("c1", "3", "me", "my reply", 4))
	m := setup(t, f)
	enter(m, "/win 2")
	if m.winName(m.cur) != "#dev" {
		t.Fatalf("in %s", m.winName(m.cur))
	}
	return m, f
}

// TestLikeReactsToTheNewestMessageFromSomeoneElse: /like sends a thumbs
// up for alan's line, the newest someone else sent, skipping the
// account's own reply after it; the status says what goes and what went,
// the reactions the answer carries show at once, and nothing is focused
// or told in window 0.
func TestLikeReactsToTheNewestMessageFromSomeoneElse(t *testing.T) {
	m, f := likeModel(t)
	enter(m, "/read-receipts on")
	reads, notes, cur := len(f.read), len(m.wins[0].notes), m.cur
	unread := map[*win]int{}
	for _, w := range m.wins {
		unread[w] = w.unread
	}
	back := []domain.Reaction{{Emoji: eLike, Count: 1, Mine: true}}
	f.reactBack = []domain.Change{{Kind: domain.Reacted, ChatID: "c1", Msg: domain.Message{ID: "2", ChatID: "c1", Reactions: back}}}
	cmd := enter(m, "/like")
	if want := "reacting " + eLike + ` to alan "second dev line"...`; m.isErr || m.status != want || cmd == nil || len(f.reacted) != 0 {
		t.Fatalf("before the answer: %q (error %v), cmd %v, calls %v; want %q", m.status, m.isErr, cmd != nil, f.reacted, want)
	}
	runAll(m, cmd)
	if !slices.Equal(f.reacted, []string{"c1:2:" + eLike}) {
		t.Fatalf("one reaction to alan's line: %v", f.reacted)
	}
	if want := "reacted " + eLike + ` to alan "second dev line"`; m.isErr || m.status != want {
		t.Fatalf("after: %q (error %v); want %q", m.status, m.isErr, want)
	}
	if got := msgOf(m, "c1", "2").Reactions; !slices.Equal(got, back) {
		t.Fatalf("the answer's reactions show at once: %+v", got)
	}
	if len(m.wins[0].notes) != notes || len(f.read) != reads || m.cur != cur {
		t.Fatalf("nothing told, read or focused: notes %v, receipts %v, current %d", noteTexts(m.wins[0])[notes:], f.read[reads:], m.cur)
	}
	for _, w := range m.wins {
		if w.unread != unread[w] {
			t.Fatalf("unread count of %s changed", m.nameOf(w.chatID))
		}
	}
}

// TestLikeAppliesNothingWithoutAnAnswer: when the answer carries no
// reactions the message keeps what it had until a poll says more.
func TestLikeAppliesNothingWithoutAnAnswer(t *testing.T) {
	m, f := likeModel(t)
	runAll(m, enter(m, "/like fire"))
	if !slices.Equal(f.reacted, []string{"c1:2:" + eFire}) || m.isErr || len(msgOf(m, "c1", "2").Reactions) != 0 {
		t.Fatalf("%v, %q, %+v", f.reacted, m.status, msgOf(m, "c1", "2").Reactions)
	}
}

// TestLikeTakesItsNames: every name and alias, in any case, and no name
// for like.
func TestLikeTakesItsNames(t *testing.T) {
	m, f := likeModel(t)
	for _, c := range []struct{ arg, emoji string }{
		{"", eLike}, {"like", eLike}, {"+1", eLike}, {"LIKE", eLike}, {"Like", eLike},
		{"dislike", eDislike}, {"-1", eDislike},
		{"love", eLove}, {"heart", eLove}, {"HEART", eLove},
		{"fire", eFire}, {"laugh", eLaugh}, {"sad", eSad},
		{"party", eParty}, {"tada", eParty}, {"clap", eClap},
		{"thanks", eThanks}, {"pray", eThanks},
		{"ok", eOK}, {"think", eThink}, {"eyes", eEyes},
	} {
		f.reacted = nil
		line := strings.TrimSpace("/like " + c.arg)
		runAll(m, enter(m, line))
		if !slices.Equal(f.reacted, []string{"c1:2:" + c.emoji}) || m.isErr {
			t.Errorf("%q sent %v (%q); want %q", line, f.reacted, m.status, c.emoji)
		}
	}
}

// TestLikeRefusesOtherNames: anything else is the error naming the
// vocabulary, in any window, and nothing is sent.
func TestLikeRefusesOtherNames(t *testing.T) {
	m, f := likeModel(t)
	for _, win := range []string{"/win 2", "/win 0"} {
		enter(m, win)
		for _, arg := range []string{"nope", "fire extra", eLike, "off", "remove", "+2", "likes", "thumbsup"} {
			cmd := enter(m, "/like "+arg)
			if !m.isErr || m.status != wantLikeErr || cmd != nil {
				t.Errorf("after %s, /like %s: %q (error %v, cmd %v)", win, arg, m.status, m.isErr, cmd != nil)
			}
		}
	}
	if len(f.reacted) != 0 {
		t.Fatalf("nothing sent: %v", f.reacted)
	}
}

// TestLikeRefusedInWindowZero: window 0 has no chat.
func TestLikeRefusedInWindowZero(t *testing.T) {
	m, f := likeModel(t)
	enter(m, "/win 0")
	for _, line := range []string{"/like", "/like fire"} {
		if cmd := enter(m, line); !m.isErr || m.status != likeNoChat || cmd != nil {
			t.Errorf("%s in window 0: %q (error %v)", line, m.status, m.isErr)
		}
	}
	if len(f.reacted) != 0 {
		t.Fatalf("nothing sent: %v", f.reacted)
	}
}

// TestLikeNeedsAReactor: a provider that cannot send reactions says so.
func TestLikeNeedsAReactor(t *testing.T) {
	f := newFake()
	m := setupProvider(t, struct{ provider.Provider }{f}) // no React
	enter(m, "/win 2")
	if cmd := enter(m, "/like"); !m.isErr || m.status != "reactions cannot be sent with fake" || cmd != nil {
		t.Fatalf("%q (error %v)", m.status, m.isErr)
	}
}

// TestLikeNeedsAMessageFromSomeoneElse: a chat whose messages are all the
// account's own, or that has none, has no target.
func TestLikeNeedsAMessageFromSomeoneElse(t *testing.T) {
	f := wideFake()
	f.msgs = append(f.msgs, mk("c6", "1", "me", "only mine", 5))
	m := setup(t, f)
	for _, c := range []struct{ win, want string }{
		{"/win #ops", "no message from someone else in #ops to react to"}, // none
		{"/win #qa", "no message from someone else in #qa to react to"},   // the account's own only
	} {
		enter(m, c.win)
		if cmd := enter(m, "/like"); !m.isErr || m.status != c.want || cmd != nil {
			t.Errorf("after %s: %q (error %v); want %q", c.win, m.status, m.isErr, c.want)
		}
	}
	if len(f.reacted) != 0 {
		t.Fatalf("nothing sent: %v", f.reacted)
	}
}

// TestLikeFailureIsReported: a refusal is a failed request: the status
// in red and window 0's error notice; the message keeps its reactions.
func TestLikeFailureIsReported(t *testing.T) {
	for _, c := range []struct {
		err  error
		want string
	}{
		{errors.New("telegram: this chat does not allow that reaction (REACTION_INVALID)"), "reaction failed: telegram: this chat does not allow that reaction (REACTION_INVALID)"},
		{provider.ErrNotImplemented, "reaction failed: live connector not available; run lazygram demo"},
	} {
		m, f := likeModel(t)
		f.reactErr = c.err
		f.reactBack = []domain.Change{{Kind: domain.Reacted, ChatID: "c1", Msg: domain.Message{ID: "2", ChatID: "c1", Reactions: []domain.Reaction{{Emoji: eLove, Count: 1, Mine: true}}}}}
		runAll(m, enter(m, "/like love"))
		if !m.isErr || m.status != c.want {
			t.Errorf("status %q (error %v); want %q", m.status, m.isErr, c.want)
		}
		if n := noteTexts(m.wins[0]); n[len(n)-1] != "error: "+c.want {
			t.Errorf("window 0: %q", n[len(n)-1])
		}
		if got := msgOf(m, "c1", "2").Reactions; len(got) != 0 {
			t.Errorf("nothing applied: %+v", got)
		}
	}
}

// TestTabCompletesLikeNames: Tab after /like offers the twelve names, not
// chat names.
func TestTabCompletesLikeNames(t *testing.T) {
	m, _ := likeModel(t)
	for typed, want := range map[string]string{
		"/like f":   "/like fire ",
		"/LIKE lo":  "/LIKE love ",
		"/like th":  "/like th", // thanks, think
		"/like al":  "/like al", // no chat names
		"/like e":   "/like eyes ",
		"/like +":   "/like +", // aliases are not offered
		"/like fir": "/like fire ",
	} {
		m.input, m.comp = nil, nil
		typ(m, typed)
		press(m, tea.KeyTab)
		if got := string(m.input); got != want {
			t.Errorf("%q: got %q, want %q", typed, got, want)
		}
	}
	m.input, m.comp = nil, nil
	typ(m, "/like ")
	press(m, tea.KeyTab)
	press(m, tea.KeyTab)
	if v := body(m); !strings.Contains(v, "clap  dislike  eyes  fire  laugh  like  love  ok  party  sad  thanks  think") {
		t.Errorf("the second Tab lists the names:\n%s", v)
	}
}

// TestHelpNamesLike: /help's /like line names every name and alias and
// the target; the Reactions note names /like.
func TestHelpNamesLike(t *testing.T) {
	var line string
	for _, l := range strings.Split(command.HelpText, "\n") {
		if strings.HasPrefix(l, "  /like ") {
			line = l
		}
	}
	for _, want := range []string{"/like [name]", "newest message someone else sent", "like (the default", "+1", "dislike (-1)", "love (heart)",
		"fire", "laugh", "sad", "party (tada)", "clap", "thanks (pray)", "ok", "think", "eyes", "replaces your reaction"} {
		if !strings.Contains(line, want) {
			t.Errorf("/help's /like line lacks %q: %q", want, line)
		}
	}
	var note string
	for _, l := range strings.Split(helpNotes, "\n") {
		if strings.HasPrefix(l, "  Reactions ") {
			note = l
		}
	}
	if !strings.Contains(note, "/like") {
		t.Errorf("the Reactions note does not name /like: %q", note)
	}
}

// TestHintNamesLike: a start hint makes /like known; window 0 refuses
// it, so it says "chat window" (TestHintsAreWellFormed checks the form).
func TestHintNamesLike(t *testing.T) {
	for _, h := range hintPool {
		if strings.Contains(h, "/like") && strings.Contains(h, "chat window") {
			return
		}
	}
	t.Fatal("no start hint names /like")
}
