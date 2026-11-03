package telegram

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"

	"lazygram/internal/domain"
	"lazygram/internal/provider"
)

const thumbsUp, heart = "\U0001F44D", "\u2764"

// count is one result of messageReactions; mine sets chosen_order.
func count(r tg.ReactionClass, n int, mine bool) tg.ReactionCount {
	rc := tg.ReactionCount{Reaction: r, Count: n}
	if mine {
		rc.SetChosenOrder(1)
	}
	return rc
}

func TestReactionsAreConvertedWithMessages(t *testing.T) {
	m := tmsg(1, 0, "liked")
	m.SetReactions(tg.MessageReactions{Results: []tg.ReactionCount{
		count(&tg.ReactionEmoji{Emoticon: thumbsUp}, 3, true),
		count(&tg.ReactionCustomEmoji{DocumentID: 11}, 2, false),
		count(&tg.ReactionPaid{}, 5, false),
		count(&tg.ReactionEmpty{}, 4, false),
		count(&tg.ReactionCustomEmoji{DocumentID: 12}, 1, true),
		count(&tg.ReactionEmoji{Emoticon: "\U0001F525"}, 0, false),
		count(&tg.ReactionEmoji{Emoticon: heart}, 1, false),
	}})
	got := convert("telegram:7", entity{kind: kindUser}, m, collect(nil, nil)).Reactions
	want := []domain.Reaction{
		{Emoji: thumbsUp, Count: 3, Mine: true, MineCount: 1},
		{Emoji: domain.CustomReaction, Count: 3, Mine: true, MineCount: 1}, // both custom emoji, summed where the first stood
		{Emoji: "\u2b50", Count: 5},                                        // paid: the star
		{Emoji: heart, Count: 1},                                           // empty and zero counts dropped
	}
	same := func(what string, got, want []domain.Reaction) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("%s: %+v", what, got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%s %d: got %+v want %+v", what, i, got[i], want[i])
			}
		}
	}
	same("converted", got, want)
	// The account's share: every own custom emoji summed, and its Stars,
	// which the paid count is of, from its entry among the top reactors.
	own := tmsg(5, 0, "starred")
	r := tg.MessageReactions{Results: []tg.ReactionCount{
		count(&tg.ReactionCustomEmoji{DocumentID: 11}, 2, true),
		count(&tg.ReactionPaid{}, 14, true),
		count(&tg.ReactionCustomEmoji{DocumentID: 12}, 1, true),
	}}
	r.SetTopReactors([]tg.MessageReactor{{Top: true, Count: 4}, {My: true, Count: 10}})
	own.SetReactions(r)
	same("own", convert("telegram:7", entity{kind: kindUser}, own, collect(nil, nil)).Reactions, []domain.Reaction{
		{Emoji: domain.CustomReaction, Count: 3, Mine: true, MineCount: 2},
		{Emoji: "\u2b50", Count: 14, Mine: true, MineCount: 10},
	})
	r.Results = []tg.ReactionCount{count(&tg.ReactionPaid{}, 14, false)} // listed among the top reactors alone
	own.SetReactions(r)
	same("stars without chosen_order", convert("telegram:7", entity{kind: kindUser}, own, collect(nil, nil)).Reactions, []domain.Reaction{
		{Emoji: "\u2b50", Count: 14, Mine: true, MineCount: 10},
	})
	if out := convert("telegram:7", entity{}, tmsg(2, 0, "plain"), collect(nil, nil)); out.Reactions != nil || out.MineUnknown {
		t.Fatalf("no reactions: %+v", out)
	}
	shared := tmsg(3, 0, "channel post")
	shared.SetReactions(tg.MessageReactions{Min: true, Results: []tg.ReactionCount{count(&tg.ReactionEmoji{Emoticon: heart}, 9, false)}})
	if out := convert("telegram:7", entity{}, shared, collect(nil, nil)); !out.MineUnknown || len(out.Reactions) != 1 {
		t.Fatalf("min reactions leave Mine unknown: %+v", out)
	}
	edited := tmsg(4, 0, "edited")
	edited.PeerID = &tg.PeerUser{UserID: 7}
	edited.SetReactions(tg.MessageReactions{Results: []tg.ReactionCount{count(&tg.ReactionEmoji{Emoticon: heart}, 2, false)}})
	if c, ok := changeEvent(&tg.UpdateEditMessage{Message: edited}, tg.Entities{}, nil); !ok || len(c.Msg.Reactions) != 1 || c.Msg.Reactions[0].Count != 2 {
		t.Fatalf("an edit carries its reactions: %+v", c.Msg)
	}
}

// reactionsUpdate is updateMessageReactions for message 42 of channel 9
// with recent reactions from user 7 (in the update), user 8 (known from
// earlier responses), user 6 (never seen), the channel itself, the
// account itself, and one sent by the account as some channel.
func reactionsUpdate() *tg.UpdateMessageReactions {
	recent := func(p tg.PeerClass, emoji string, my bool) tg.MessagePeerReaction {
		return tg.MessagePeerReaction{PeerID: p, Date: 1700000000, Reaction: &tg.ReactionEmoji{Emoticon: emoji}, My: my}
	}
	r := tg.MessageReactions{Results: []tg.ReactionCount{count(&tg.ReactionEmoji{Emoticon: thumbsUp}, 4, true), count(&tg.ReactionEmoji{Emoticon: heart}, 2, false)}}
	r.SetRecentReactions([]tg.MessagePeerReaction{
		recent(&tg.PeerUser{UserID: 7}, thumbsUp, false),
		recent(&tg.PeerUser{UserID: 8}, heart, false),
		recent(&tg.PeerUser{UserID: 6}, heart, false),
		recent(&tg.PeerChannel{ChannelID: 9}, thumbsUp, false),
		recent(&tg.PeerUser{UserID: 1}, thumbsUp, false),
		recent(&tg.PeerChannel{ChannelID: 30}, thumbsUp, true),
		{PeerID: &tg.PeerUser{UserID: 7}, Reaction: &tg.ReactionEmpty{}},
	})
	return &tg.UpdateMessageReactions{Peer: &tg.PeerChannel{ChannelID: 9}, MsgID: 42, Reactions: r}
}

func checkReacted(t *testing.T, c domain.Change, own bool) {
	t.Helper()
	ch := "telegram:-1000000000009"
	if c.Kind != domain.Reacted || c.ChatID != ch || c.Msg.ID != "42" || c.Msg.ChatID != ch || c.Own != own {
		t.Fatalf("change: %+v", c)
	}
	if rs := c.Msg.Reactions; len(rs) != 2 || rs[0] != (domain.Reaction{Emoji: thumbsUp, Count: 4, Mine: true, MineCount: 1}) || rs[1] != (domain.Reaction{Emoji: heart, Count: 2}) {
		t.Fatalf("reactions: %+v", rs)
	}
	var got []string
	for _, r := range c.Reactors {
		got = append(got, r.Who+"|"+r.ID+"|"+r.Emoji)
		if r.At.Unix() != 1700000000 {
			t.Fatalf("when: %+v", r)
		}
	}
	want := []string{
		"Test Person|telegram:7|" + thumbsUp,
		"Quiet One|telegram:8|" + heart,
		"user|telegram:6|" + heart,
		"Test Channel|telegram:-1000000000009|" + thumbsUp,
		"me||" + thumbsUp,
		"me||" + thumbsUp,
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("reactors:\n got %v\nwant %v", got, want)
	}
}

func TestPushedReactionsAreAReactedChange(t *testing.T) {
	n := newNames()
	n.learnUsers(map[int64]*tg.User{8: {ID: 8, FirstName: "Quiet", LastName: "One"}})
	f := newChangeFeed()
	d := tg.NewUpdateDispatcher()
	f.register(d, n)
	err := d.Handle(context.Background(), &tg.Updates{
		Updates: []tg.UpdateClass{reactionsUpdate()},
		Users:   []tg.UserClass{&tg.User{ID: 7, FirstName: "Test", LastName: "Person"}, &tg.User{ID: 1, Self: true, FirstName: "Me"}},
		Chats:   []tg.ChatClass{&tg.Channel{ID: 9, Title: "Test Channel"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case c := <-f.feed():
		checkReacted(t, c, true) // pushed: about one of the account's own messages
	default:
		t.Fatal("updateMessageReactions reaches the change feed")
	}
	if n.get(7) != "Test Person" {
		t.Fatal("the users of the update are learned")
	}
	if _, ok := changeEvent(&tg.UpdateMessageReactions{Peer: &tg.PeerChannel{ChannelID: 9}}, tg.Entities{}, nil); ok {
		t.Fatal("no message, no change")
	}
}

// pollService is a service that knows the fake's dialogs, whose clock the
// test moves.
func pollService(t *testing.T, f *fakeAPI, clock *time.Time) *service {
	t.Helper()
	s := newService(f)
	s.names = newNames()
	s.now = func() time.Time { return *clock }
	if _, err := s.Dialogs(context.Background()); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestPollReactionsAsksForTheMessagesOnScreen(t *testing.T) {
	f := newFakeAPI()
	clock := base
	s := pollService(t, f, &clock)
	s.names.learnUsers(map[int64]*tg.User{8: {ID: 8, FirstName: "Quiet", LastName: "One"}})
	f.reactions = &tg.Updates{
		Updates: []tg.UpdateClass{reactionsUpdate(), &tg.UpdateUserTyping{UserID: 7}},
		Users:   []tg.UserClass{&tg.User{ID: 7, FirstName: "Test", LastName: "Person"}, &tg.User{ID: 1, Self: true, FirstName: "Me"}, &tg.User{ID: 21, FirstName: "Fresh"}},
		Chats:   []tg.ChatClass{&tg.Channel{ID: 9, Title: "Test Channel"}},
	}
	got, err := s.PollReactions(context.Background(), "telegram:-1000000000009", []string{"42", "sent-1", "3", "42"})
	if err != nil || len(f.polls) != 1 || f.polls[0] != "*tg.InputPeerChannel:[3 42]" {
		t.Fatalf("one call for the chat's numeric ids: %v %v", f.polls, err)
	}
	if len(got) != 2 {
		t.Fatalf("one change per asked message: %+v", got)
	}
	checkReacted(t, got[0], false) // polled: not known to be the account's
	if s.names.get(21) != "Fresh" {
		t.Fatal("the users of the answer are learned")
	}
	if c := got[1]; c.Kind != domain.Reacted || c.ChatID != "telegram:-1000000000009" || c.Msg.ID != "3" || c.Msg.Reactions != nil || c.Own {
		t.Fatalf("a message the answer leaves out has no reactions: %+v", c)
	}

	f.reactions = &tg.UpdateShort{Update: reactionsUpdate()}
	got, _ = s.PollReactions(context.Background(), "telegram:-1000000000009", []string{"42", "3"})
	if len(got) != 1 || got[0].Msg.ID != "42" {
		t.Fatalf("only a full answer says what it left out: %+v", got)
	}

	var many []string
	for i := 1; i <= 150; i++ {
		many = append(many, strconv.Itoa(i))
	}
	if _, err := s.PollReactions(context.Background(), "telegram:7", many); err != nil {
		t.Fatal(err)
	}
	if last := f.polls[len(f.polls)-1]; !strings.HasPrefix(last, "*tg.InputPeerUser:[51 52 ") || !strings.HasSuffix(last, " 150]") {
		t.Fatalf("the newest %d at most: %s", maxPollIDs, last)
	}

	calls, dialogs := len(f.polls), f.dialogs
	if got, err := s.PollReactions(context.Background(), "telegram:7", []string{"x", "0", "-4"}); err != nil || got != nil {
		t.Fatalf("no message ids: %+v %v", got, err)
	}
	if _, err := s.PollReactions(context.Background(), "telegram:404", []string{"1"}); err == nil || errors.Is(err, provider.ErrNotPollable) {
		t.Fatalf("an unknown chat: %v", err)
	}
	if len(f.polls) != calls || f.dialogs != dialogs {
		t.Fatalf("neither costs a request: polls %v dialogs %d", f.polls, f.dialogs)
	}
}

func TestPollRefusalsSkipTheChat(t *testing.T) {
	for _, code := range []string{"CHANNEL_PRIVATE", "CHANNEL_INVALID", "MSG_ID_INVALID"} {
		f := newFakeAPI()
		clock := base
		s := pollService(t, f, &clock)
		f.reactErr = tgerr.New(400, code)
		if _, err := s.PollReactions(context.Background(), "telegram:-1000000000009", []string{"1"}); !errors.Is(err, provider.ErrNotPollable) {
			t.Fatalf("%s: %v", code, err)
		}
		f.reactErr = nil
		if _, err := s.PollReactions(context.Background(), "telegram:-1000000000009", []string{"1"}); !errors.Is(err, provider.ErrNotPollable) || len(f.polls) != 1 {
			t.Fatalf("%s: the chat is not asked again: %v %v", code, err, f.polls)
		}
		if _, err := s.PollReactions(context.Background(), "telegram:7", []string{"1"}); err != nil || len(f.polls) != 2 {
			t.Fatalf("%s: other chats are: %v %v", code, err, f.polls)
		}
	}
	f := newFakeAPI()
	clock := base
	s := pollService(t, f, &clock)
	f.reactErr = errors.New("network down")
	if _, err := s.PollReactions(context.Background(), "telegram:7", []string{"1"}); err == nil || errors.Is(err, provider.ErrNotPollable) {
		t.Fatalf("other errors pass: %v", err)
	}
	f.reactErr = nil
	if _, err := s.PollReactions(context.Background(), "telegram:7", []string{"1"}); err != nil || len(f.polls) != 2 {
		t.Fatalf("and do not stop the chat: %v %v", err, f.polls)
	}
}

func TestPollFloodWaitStopsPolling(t *testing.T) {
	f := newFakeAPI()
	clock := base
	s := pollService(t, f, &clock)
	s.api = floodAPI{api: f, sleep: func(context.Context, time.Duration) error {
		t.Fatal("a poll never sleeps through a FLOOD_WAIT")
		return nil
	}}
	f.reactErr = tgerr.New(420, "FLOOD_WAIT_30")
	var wait *provider.WaitError
	if _, err := s.PollReactions(context.Background(), "telegram:7", []string{"1"}); !errors.As(err, &wait) || wait.Wait != 30*time.Second {
		t.Fatalf("flood: %v", err)
	}
	f.reactErr = nil
	clock = clock.Add(20 * time.Second)
	if _, err := s.PollReactions(context.Background(), "telegram:-5", []string{"1"}); !errors.As(err, &wait) || wait.Wait != 10*time.Second || len(f.polls) != 1 {
		t.Fatalf("no poll of any chat until it has passed: %v %v", err, f.polls)
	}
	clock = clock.Add(10 * time.Second)
	if _, err := s.PollReactions(context.Background(), "telegram:-5", []string{"1"}); err != nil || len(f.polls) != 2 {
		t.Fatalf("then polls go on: %v %v", err, f.polls)
	}
}
