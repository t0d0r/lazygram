package demo

import (
	"bytes"
	"context"
	"errors"
	"image/png"
	"slices"
	"strings"
	"testing"
	"time"

	"lazygram/internal/domain"
	"lazygram/internal/provider"
)

func TestSyncRevealsScriptOverTime(t *testing.T) {
	clock := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	p := NewAt(func() time.Time { return clock })
	first, _ := p.Sync(context.Background(), time.Time{})
	if len(first) == 0 || len(first) >= len(script) {
		t.Fatalf("initial backlog should be partial, got %d of %d", len(first), len(script))
	}
	last := first[len(first)-1].Time
	again, _ := p.Sync(context.Background(), last)
	if len(again) != 0 {
		t.Fatalf("nothing new yet, got %d", len(again))
	}
	clock = clock.Add(10 * time.Minute)
	rest, _ := p.Sync(context.Background(), last)
	if len(first)+len(rest) != len(script) {
		t.Fatalf("all lines should be visible later: %d+%d != %d", len(first), len(rest), len(script))
	}
}

func TestChatsKnown(t *testing.T) {
	p := New()
	cs, err := p.Chats(context.Background())
	if err != nil || len(cs) != len(chats) {
		t.Fatal("chats")
	}
	known := map[string]bool{}
	for _, c := range cs {
		known[c.ID] = true
	}
	for _, m := range p.msgs {
		if !known[m.ChatID] {
			t.Fatalf("message references unknown chat %q", m.ChatID)
		}
	}
}

func TestRecent(t *testing.T) {
	clock := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	p := NewAt(func() time.Time { return clock })
	got, err := p.Recent(context.Background(), "demo-1", 2)
	if err != nil || len(got) != 2 || got[0].ChatID != "demo-1" || !got[0].Time.Before(got[1].Time) {
		t.Fatalf("%v %v", got, err)
	}
	for _, m := range got {
		if m.Time.After(clock) {
			t.Fatal("future lines must not be revealed")
		}
	}
	if got, _ := p.Recent(context.Background(), "nope", 5); len(got) != 0 {
		t.Fatal("unknown chat")
	}
}

func TestSendShowsUpOnNextSync(t *testing.T) {
	clock := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	p := NewAt(func() time.Time { return clock })
	first, _ := p.Sync(context.Background(), time.Time{})
	if err := p.Send(context.Background(), "demo-3", "hello"); err != nil {
		t.Fatal(err)
	}
	if err := p.Send(context.Background(), "nope", "x"); err == nil {
		t.Fatal("unknown chat")
	}
	got, _ := p.Sync(context.Background(), first[len(first)-1].Time)
	if len(got) != 1 || got[0].Sender != "me" || got[0].Text != "hello" || got[0].ChatID != "demo-3" {
		t.Fatalf("%+v", got)
	}
}

func TestPressAnswersAndEditsTheMessage(t *testing.T) {
	clock := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	p := NewAt(func() time.Time { return clock })
	all, _ := p.Sync(context.Background(), time.Time{})
	var bot domain.Message
	for _, m := range all {
		if m.ChatID == "demo-5" {
			bot = m
		}
	}
	if len(bot.Buttons) != 3 {
		t.Fatalf("bot message buttons: %+v", bot)
	}
	a, err := p.Press(context.Background(), "demo-5", bot.ID, bot.Buttons[0])
	if err != nil || a.Text == "" || a.Updated == nil || len(a.Updated.Buttons) != 0 || !strings.Contains(a.Updated.Text, "Deploy") {
		t.Fatalf("%+v %v", a, err)
	}
	if _, err := p.Press(context.Background(), "demo-5", bot.ID, bot.Buttons[1]); err == nil {
		t.Fatal("buttons are gone after the edit")
	}
}

func TestFetchReturnsAPNG(t *testing.T) {
	clock := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	p := NewAt(func() time.Time { return clock })
	all, _ := p.Sync(context.Background(), time.Time{})
	var photo domain.Message
	for _, m := range all {
		if m.Media != nil {
			photo = m
		}
	}
	if photo.ID == "" || photo.Media.Kind != "photo" {
		t.Fatal("demo should carry one photo")
	}
	data, name, err := p.Fetch(context.Background(), photo.ChatID, photo.ID)
	if err != nil || !strings.HasSuffix(name, ".png") {
		t.Fatal(err, name)
	}
	if _, err := png.Decode(bytes.NewReader(data)); err != nil {
		t.Fatal("not a PNG:", err)
	}
	if _, _, err := p.Fetch(context.Background(), "demo-1", "1"); err == nil {
		t.Fatal("a message without media")
	}
}

func TestEditAndDeleteOwnLines(t *testing.T) {
	clock := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	p := NewAt(func() time.Time { return clock })
	all, _ := p.Sync(context.Background(), time.Time{})
	var mine, theirs domain.Message
	for _, m := range all {
		if m.ChatID == "demo-3" && m.Sender == "me" {
			mine = m
		}
		if m.ChatID == "demo-3" && m.Sender != "me" {
			theirs = m
		}
	}
	if err := p.Edit(context.Background(), mine.ChatID, mine.ID, "changed"); err != nil {
		t.Fatal(err)
	}
	if err := p.Edit(context.Background(), theirs.ChatID, theirs.ID, "nope"); err == nil {
		t.Fatal("only own lines")
	}
	if err := p.Delete(context.Background(), mine.ChatID, mine.ID); err != nil {
		t.Fatal(err)
	}
	for _, m := range p.msgs {
		if m.ID == mine.ID {
			t.Fatal("deleted line still there")
		}
	}
}

func TestSendFileShowsUpAndFetchesBack(t *testing.T) {
	clock := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	p := NewAt(func() time.Time { return clock })
	first, _ := p.Sync(context.Background(), time.Time{})
	if err := p.SendFile(context.Background(), "demo-3", "cat.png", []byte("PNGBYTES"), "look"); err != nil {
		t.Fatal(err)
	}
	got, _ := p.Sync(context.Background(), first[len(first)-1].Time)
	if len(got) != 1 || got[0].Media == nil || got[0].Media.Kind != "photo" || got[0].Text != "look" || got[0].Sender != "me" {
		t.Fatalf("%+v", got)
	}
	data, name, err := p.Fetch(context.Background(), "demo-3", got[0].ID)
	if err != nil || string(data) != "PNGBYTES" || name != "cat.png" {
		t.Fatalf("%q %q %v", data, name, err)
	}
	if err := p.SendFile(context.Background(), "nope", "a.txt", nil, ""); err == nil {
		t.Fatal("unknown chat")
	}
}

func TestGroupMembersHaveNicks(t *testing.T) {
	p := NewAt(func() time.Time { return time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC) })
	nicks := map[string]bool{}
	for _, mb := range domain.Members(p.msgs, "demo-1") {
		nicks[mb.Nick] = true
	}
	if !nicks["ada"] || !nicks["alan"] || !nicks["grace"] || len(nicks) != 3 {
		t.Fatalf("#dev members: %v", nicks)
	}
	if ms := domain.Members(p.msgs, "demo-4"); len(ms) != 1 || ms[0].Nick != "" || ms[0].Name != "wire" {
		t.Fatalf("#news has a signed author without a nick: %+v", ms)
	}
}

func TestMembersList(t *testing.T) {
	p := New()
	dev, err := p.Members(context.Background(), "demo-1")
	if err != nil || len(dev) != 5 || dev[3].Nick != "margaret_h" || dev[4].Nick != "" {
		t.Fatalf("#dev: %+v %v", dev, err)
	}
	if _, err := p.Members(context.Background(), "demo-4"); !errors.Is(err, provider.ErrMembersHidden) {
		t.Fatalf("the channel hides its subscribers: %v", err)
	}
}

func TestSomeLinesCarryReactions(t *testing.T) {
	p := NewAt(func() time.Time { return time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC) })
	all, _ := p.Sync(context.Background(), time.Time{})
	var with, mine int
	for _, m := range all {
		if len(m.Reactions) > 0 {
			with++
		}
		for _, r := range m.Reactions {
			if r.Count <= 0 || r.Emoji == "" {
				t.Fatalf("%+v", m)
			}
			if r.Mine {
				mine++
			}
		}
	}
	if with < 3 || mine == 0 {
		t.Fatalf("lines with reactions %d, of the account's own %d", with, mine)
	}
}

func TestAReactionIsPushedAfterStart(t *testing.T) {
	clock := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	p := NewAt(func() time.Time { return clock })
	var cs provider.ChangeSource = p
	before, _ := p.Recent(context.Background(), "demo-3", 50)
	select {
	case c := <-cs.Changes():
		t.Fatalf("nothing yet: %+v", c)
	default:
	}
	clock = clock.Add(30 * time.Second)
	if _, err := p.Sync(context.Background(), clock); err != nil {
		t.Fatal(err)
	}
	var c domain.Change
	select {
	case c = <-cs.Changes():
	default:
		t.Fatal("the reaction to the account's own line is pushed")
	}
	if c.Kind != domain.Reacted || c.ChatID != "demo-3" || !c.Own || len(c.Msg.Reactions) != 1 ||
		c.Msg.Reactions[0] != (domain.Reaction{Emoji: thumbsUp, Count: 1}) ||
		len(c.Reactors) != 1 || c.Reactors[0].Who != "linus" || c.Reactors[0].Emoji != thumbsUp {
		t.Fatalf("%+v", c)
	}
	after, _ := p.Recent(context.Background(), "demo-3", 50)
	for i, m := range after {
		if m.ID == c.Msg.ID {
			if m.Sender != "me" || len(m.Reactions) != 1 || len(before[i].Reactions) != 0 {
				t.Fatalf("the line counts it now, a copy handed out before does not: %+v / %+v", m, before[i])
			}
		}
	}
	select {
	case c := <-cs.Changes():
		t.Fatalf("pushed once: %+v", c)
	default:
	}
}

func TestPollFindsReactionsToOthersLines(t *testing.T) {
	clock := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	p := NewAt(func() time.Time { return clock })
	var poller provider.ReactionPoller = p
	tip, fix := "", ""
	for _, m := range p.msgs {
		switch m.Text {
		case "tip: /win 2 switches windows, Tab completes names":
			tip = m.ID
		case "pushed a fix for the scrollback off-by-one":
			fix = m.ID
		}
	}
	if got, _ := poller.PollReactions(context.Background(), "demo-1", []string{tip}); len(got) != 0 {
		t.Fatalf("a line not there yet is left out: %+v", got)
	}
	clock = clock.Add(2 * time.Minute)
	got, err := poller.PollReactions(context.Background(), "demo-1", []string{fix, tip, "404"})
	if err != nil || len(got) != 2 {
		t.Fatalf("one change per asked line: %+v %v", got, err)
	}
	for _, c := range got {
		if c.Kind != domain.Reacted || c.ChatID != "demo-1" || c.Own {
			t.Fatalf("%+v", c)
		}
	}
	if got[0].Msg.ID != fix || len(got[0].Msg.Reactions) != 1 || got[0].Msg.Reactions[0].Count != 2 || len(got[0].Reactors) != 0 {
		t.Fatalf("as it was: %+v", got[0])
	}
	if c := got[1]; c.Msg.ID != tip || len(c.Msg.Reactions) != 1 || c.Msg.Reactions[0] != (domain.Reaction{Emoji: thumbsUp, Count: 1}) ||
		len(c.Reactors) != 1 || c.Reactors[0].Who != "grace" {
		t.Fatalf("grace's reaction, found by the poll: %+v", c)
	}
	for len(p.Changes()) > 0 {
		if c := <-p.Changes(); c.Msg.ID == tip || !c.Own {
			t.Fatalf("only reactions to the account's own lines are pushed, got %+v", c)
		}
	}
}

func TestWithReactionLeavesItsInputAlone(t *testing.T) {
	rs := []domain.Reaction{{Emoji: "a", Count: 1}}
	got := withReaction(rs, "a")
	if got[0].Count != 2 || rs[0].Count != 1 {
		t.Fatalf("counted on a copy: %+v %+v", got, rs)
	}
	if got := withReaction(rs, "b"); len(got) != 2 || got[1] != (domain.Reaction{Emoji: "b", Count: 1}) || len(rs) != 1 {
		t.Fatalf("a new emoji goes last: %+v", got)
	}
}

// lineByText is the demo line with that text.
func lineByText(t *testing.T, p *Provider, text string) domain.Message {
	t.Helper()
	for _, m := range p.msgs {
		if m.Text == text {
			return m
		}
	}
	t.Fatalf("no demo line %q", text)
	return domain.Message{}
}

// TestReactSetsTheAccountsReaction: /like on a demo line takes the
// account's earlier reaction off (a count that drops to zero goes) and
// adds the new one as the account's; the answer and later polls report
// the line that way.
func TestReactSetsTheAccountsReaction(t *testing.T) {
	clock := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	p := NewAt(func() time.Time { return clock })
	var r provider.Reactor = p
	fix := lineByText(t, p, "pushed a fix for the scrollback off-by-one") // thumbs up 2, none the account's
	got, err := r.React(context.Background(), fix.ChatID, fix.ID, heart)
	want := []domain.Reaction{{Emoji: thumbsUp, Count: 2}, {Emoji: heart, Count: 1, Mine: true}}
	if err != nil || len(got) != 1 || got[0].Kind != domain.Reacted || got[0].Own || got[0].Msg.ID != fix.ID || got[0].ChatID != fix.ChatID ||
		!slices.Equal(got[0].Msg.Reactions, want) {
		t.Fatalf("a new reaction of the account's goes last: %+v %v", got, err)
	}
	if polled, _ := p.PollReactions(context.Background(), fix.ChatID, []string{fix.ID}); len(polled) != 1 || !slices.Equal(polled[0].Msg.Reactions, want) {
		t.Fatalf("later polls report it: %+v", polled)
	}
	rc := lineByText(t, p, "release candidate tagged") // party 3, the account's thumbs up 1
	got, _ = r.React(context.Background(), rc.ChatID, rc.ID, fire)
	if want := []domain.Reaction{{Emoji: party, Count: 3}, {Emoji: fire, Count: 1, Mine: true}}; !slices.Equal(got[0].Msg.Reactions, want) {
		t.Fatalf("the account's earlier reaction is replaced: %+v", got[0].Msg.Reactions)
	}
	got, _ = r.React(context.Background(), rc.ChatID, rc.ID, party)
	if want := []domain.Reaction{{Emoji: party, Count: 4, Mine: true}}; !slices.Equal(got[0].Msg.Reactions, want) {
		t.Fatalf("on an entry already there: %+v", got[0].Msg.Reactions)
	}
	got, _ = r.React(context.Background(), rc.ChatID, rc.ID, party)
	if want := []domain.Reaction{{Emoji: party, Count: 4, Mine: true}}; !slices.Equal(got[0].Msg.Reactions, want) {
		t.Fatalf("the same reaction again changes nothing: %+v", got[0].Msg.Reactions)
	}
}

// TestReactKeepsThePaidStar: Telegram sets paid reactions apart, so the
// demo keeps the account's star when it reacts.
func TestReactKeepsThePaidStar(t *testing.T) {
	rs := []domain.Reaction{{Emoji: star, Count: 5, Mine: true, MineCount: 2}, {Emoji: domain.CustomReaction, Count: 3, Mine: true, MineCount: 2}}
	got := withMine(rs, thumbsUp)
	want := []domain.Reaction{{Emoji: star, Count: 5, Mine: true, MineCount: 2}, {Emoji: domain.CustomReaction, Count: 1}, {Emoji: thumbsUp, Count: 1, Mine: true}}
	if !slices.Equal(got, want) || rs[1].Count != 3 {
		t.Fatalf("withMine: %+v (input %+v)", got, rs)
	}
}

// TestReactRefusesWhatTheDemoLacks: a chat or line the demo does not have,
// a line not there yet and an empty emoji are errors and change nothing.
func TestReactRefusesWhatTheDemoLacks(t *testing.T) {
	clock := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	p := NewAt(func() time.Time { return clock })
	fix := lineByText(t, p, "pushed a fix for the scrollback off-by-one")
	tip := lineByText(t, p, "tip: /win 2 switches windows, Tab completes names") // arrives later
	for _, c := range []struct{ chat, id, emoji string }{
		{fix.ChatID, "404", heart}, {"demo-404", fix.ID, heart}, {tip.ChatID, tip.ID, heart}, {fix.ChatID, fix.ID, ""},
	} {
		if got, err := p.React(context.Background(), c.chat, c.id, c.emoji); err == nil || got != nil {
			t.Errorf("React(%q, %q, %q): %+v %v", c.chat, c.id, c.emoji, got, err)
		}
	}
	if m := lineByText(t, p, "pushed a fix for the scrollback off-by-one"); len(m.Reactions) != 1 || m.Reactions[0].Count != 2 {
		t.Fatalf("nothing changed: %+v", m.Reactions)
	}
}
