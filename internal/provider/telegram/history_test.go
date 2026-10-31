package telegram

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
)

// fakeAPI serves invented dialogs and a descending message list per peer.
type fakeAPI struct {
	msgs    map[int64][]tg.MessageClass // by user id, newest first
	top     map[int64]int               // top message id per dialog
	dialogs int
	history int
	flood   int // FLOOD_WAIT errors still to return
	sent    []string
}

func newFakeAPI() *fakeAPI {
	return &fakeAPI{msgs: map[int64][]tg.MessageClass{}, top: map[int64]int{}}
}

func (f *fakeAPI) MessagesGetDialogs(context.Context, *tg.MessagesGetDialogsRequest) (tg.MessagesDialogsClass, error) {
	f.dialogs++
	if f.flood > 0 {
		f.flood--
		return nil, tgerr.New(420, "FLOOD_WAIT_2")
	}
	return &tg.MessagesDialogsSlice{
		Dialogs: []tg.DialogClass{
			&tg.Dialog{Peer: &tg.PeerUser{UserID: 7}, TopMessage: f.top[7], UnreadCount: 3},
			&tg.Dialog{Peer: &tg.PeerChat{ChatID: 5}, TopMessage: f.top[-5]},
			&tg.Dialog{Peer: &tg.PeerChannel{ChannelID: 9}, TopMessage: f.top[-9]},
			&tg.Dialog{Peer: &tg.PeerUser{UserID: 8}},
			&tg.Dialog{Peer: &tg.PeerUser{UserID: 1}},
			&tg.Dialog{Peer: &tg.PeerChannel{ChannelID: 12}},
			&tg.Dialog{Peer: &tg.PeerUser{UserID: 13}, TopMessage: 5},
			&tg.Dialog{Peer: &tg.PeerChannel{ChannelID: 99}}, // forbidden: no entity
			&tg.DialogFolder{},
		},
		Users: []tg.UserClass{
			&tg.User{ID: 7, AccessHash: 1, FirstName: "Test", LastName: "Person", Username: "testperson"},
			&tg.User{ID: 8, AccessHash: 2, Username: "nameless"},
			&tg.User{ID: 1, Self: true, FirstName: "Me"},
			&tg.User{ID: 13, Deleted: true},
		},
		Chats: []tg.ChatClass{
			&tg.Chat{ID: 5, Title: "Test Group"},
			&tg.Channel{ID: 9, AccessHash: 3, Title: "Test Channel", Username: "testchan"},
			&tg.Channel{ID: 12, AccessHash: 4, Title: "Super Group", Megagroup: true},
			&tg.ChannelForbidden{ID: 99},
		},
	}, nil
}

func (f *fakeAPI) MessagesGetHistory(_ context.Context, r *tg.MessagesGetHistoryRequest) (tg.MessagesMessagesClass, error) {
	f.history++
	var key int64
	switch p := r.Peer.(type) {
	case *tg.InputPeerUser:
		key = p.UserID
	case *tg.InputPeerChat:
		key = -p.ChatID
	case *tg.InputPeerChannel:
		key = -p.ChannelID
	}
	var page []tg.MessageClass
	for _, m := range f.msgs[key] {
		if r.OffsetID != 0 && m.GetID() >= r.OffsetID {
			continue
		}
		page = append(page, m)
		if len(page) == r.Limit {
			break
		}
	}
	return &tg.MessagesMessages{Messages: page, Users: []tg.UserClass{&tg.User{ID: 7, FirstName: "Test", LastName: "Person"}}}, nil
}

func (f *fakeAPI) MessagesSendMessage(_ context.Context, r *tg.MessagesSendMessageRequest) (tg.UpdatesClass, error) {
	f.sent = append(f.sent, r.Message)
	if r.RandomID == 0 {
		return nil, errors.New("random id missing")
	}
	return &tg.Updates{}, nil
}

var base = time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

func tmsg(id int, offset int, text string) *tg.Message {
	return &tg.Message{ID: id, Date: int(base.Add(time.Duration(offset) * time.Second).Unix()),
		Message: text, FromID: &tg.PeerUser{UserID: 7}}
}

// fill gives user 7 messages 1..n, newest first, and sets its top message.
func (f *fakeAPI) fill(n int) {
	f.msgs[7] = nil
	for id := n; id >= 1; id-- {
		f.msgs[7] = append(f.msgs[7], tmsg(id, id, "m"))
	}
	f.top[7] = n
}

func TestDialogsListsAllWithAliases(t *testing.T) {
	s := newService(newFakeAPI())
	chats, err := s.Dialogs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, c := range chats {
		got[c.ID] = c.Name() + "|" + c.Title
		if c.Service != "telegram" {
			t.Fatalf("%+v", c)
		}
		switch c.ID {
		case "telegram:7":
			if c.Unread != 3 {
				t.Errorf("unread: %+v", c)
			}
		case "telegram:13":
			if !c.Deleted {
				t.Errorf("deleted flag: %+v", c)
			}
		default:
			if c.Deleted || c.Unread != 0 {
				t.Errorf("%+v", c)
			}
		}
	}
	want := map[string]string{
		"telegram:7":              "@testperson|Test Person",
		"telegram:-5":             "#test-group|Test Group",
		"telegram:-1000000000009": "#testchan|Test Channel",
		"telegram:-1000000000012": "#super-group|Super Group",
		"telegram:8":              "@nameless|nameless",
		"telegram:1":              "@saved|Saved Messages",
		"telegram:13":             "@deleted-13|Deleted Account",
	}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for id, w := range want {
		if got[id] != w {
			t.Errorf("%s: got %q want %q", id, got[id], w)
		}
	}
}

func TestRecentConvertsAndSkipsServiceMessages(t *testing.T) {
	f := newFakeAPI()
	f.msgs[7] = []tg.MessageClass{
		&tg.Message{ID: 4, Date: int(base.Unix()) + 3, Media: &tg.MessageMediaEmpty{}, FromID: &tg.PeerUser{UserID: 7}},
		&tg.MessageService{ID: 3, Date: int(base.Unix()) + 2},
		&tg.Message{ID: 2, Date: int(base.Unix()) + 1, Message: "hi", PostAuthor: "editor"},
		&tg.Message{ID: 1, Date: int(base.Unix()), Message: "mine", Out: true},
	}
	f.top[7] = 4
	s := newService(f)
	if _, err := s.Recent(context.Background(), "telegram:7", 10); err == nil {
		t.Fatal("Recent before Dialogs must fail")
	}
	if _, err := s.Dialogs(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, err := s.Recent(context.Background(), "telegram:7", 10)
	if err != nil || len(got) != 3 || !got[0].Time.Before(got[2].Time) {
		t.Fatalf("%v %v", got, err)
	}
	byID := map[string]string{}
	for _, m := range got {
		byID[m.ID] = m.Sender + "|" + m.Text
		if m.ChatID != "telegram:7" || m.Time.Location() != time.UTC {
			t.Fatalf("%+v", m)
		}
	}
	if byID["4"] != "Test Person|[media]" || byID["2"] != "editor|hi" || byID["1"] != "me|mine" {
		t.Fatalf("%v", byID)
	}
}

func TestHistoryTakesNoBacklogThenOnlyChangedChats(t *testing.T) {
	f := newFakeAPI()
	f.fill(10)
	s := newService(f)

	first, err := s.History(context.Background(), time.Time{})
	if err != nil || len(first) != 0 || f.dialogs != 1 || f.history != 0 {
		t.Fatalf("first call must only record cursors: n=%d dialogs=%d history=%d err=%v", len(first), f.dialogs, f.history, err)
	}

	none, _ := s.History(context.Background(), time.Time{})
	if len(none) != 0 || f.history != 0 {
		t.Fatalf("unchanged: n=%d history=%d", len(none), f.history)
	}

	f.fill(12)
	more, _ := s.History(context.Background(), time.Time{})
	if len(more) != 2 || f.history != 1 || more[0].ID != "12" {
		t.Fatalf("changed: n=%d history=%d %+v", len(more), f.history, more)
	}
	// The deleted account's dialog (top 5) is never fetched.
	if f.history != 1 {
		t.Fatalf("deleted account fetched: history=%d", f.history)
	}
}

func TestRecentIsCappedAndOldestFirst(t *testing.T) {
	f := newFakeAPI()
	f.fill(500)
	s := newService(f)
	if _, err := s.Dialogs(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, err := s.Recent(context.Background(), "telegram:7", 10)
	if err != nil || len(got) != 10 || got[0].ID != "491" || got[9].ID != "500" || f.history != 1 {
		t.Fatalf("n=%d first=%s err=%v history=%d", len(got), got[0].ID, err, f.history)
	}
	got, _ = s.Recent(context.Background(), "telegram:7", 100000)
	if len(got) != maxRecent {
		t.Fatalf("cap: %d", len(got))
	}
}

func TestHistoryPagesUntilKnownMessage(t *testing.T) {
	f := newFakeAPI()
	f.fill(1)
	s := newService(f)
	if _, err := s.History(context.Background(), time.Time{}); err != nil {
		t.Fatal(err)
	}
	f.fill(260) // 259 new messages: three pages
	got, _ := s.History(context.Background(), time.Time{})
	if len(got) != 259 || f.history != 3 {
		t.Fatalf("n=%d history=%d", len(got), f.history)
	}
}

func TestFloodWaitRetriesOnce(t *testing.T) {
	f := newFakeAPI()
	f.flood = 1
	var slept time.Duration
	s := newService(floodAPI{api: f, sleep: func(_ context.Context, d time.Duration) error { slept = d; return nil }})
	if _, err := s.Dialogs(context.Background()); err != nil || slept != 2*time.Second || f.dialogs != 2 {
		t.Fatalf("err=%v slept=%v calls=%d", err, slept, f.dialogs)
	}

	f.flood = 2
	s = newService(floodAPI{api: f, sleep: func(context.Context, time.Duration) error { return nil }})
	if _, err := s.Dialogs(context.Background()); err == nil {
		t.Fatal("second flood wait must surface")
	}

	f.flood = 1
	s = newService(floodAPI{api: f, sleep: func(context.Context, time.Duration) error { return context.Canceled }})
	if _, err := s.Dialogs(context.Background()); !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

// failingAPI fails the Nth history request.
type failingAPI struct {
	*fakeAPI
	failAt int
}

func (f *failingAPI) MessagesGetHistory(ctx context.Context, r *tg.MessagesGetHistoryRequest) (tg.MessagesMessagesClass, error) {
	if f.history+1 == f.failAt {
		f.history++
		return nil, errors.New("network down")
	}
	return f.fakeAPI.MessagesGetHistory(ctx, r)
}

func TestHistoryReturnsPartialResultAndKeepsUnfetchedChatsPending(t *testing.T) {
	f := newFakeAPI()
	s := newService(&failingAPI{fakeAPI: f, failAt: 2})
	if _, err := s.History(context.Background(), time.Time{}); err != nil {
		t.Fatal(err)
	}
	// Two chats move: user 7 and group -5.
	f.fill(3)
	f.msgs[-5] = []tg.MessageClass{tmsg(2, 2, "g"), tmsg(1, 1, "g")}
	f.top[-5] = 2
	got, err := s.History(context.Background(), time.Time{})
	if err == nil || len(got) != 3 {
		t.Fatalf("partial: n=%d err=%v", len(got), err)
	}
	if s.top["telegram:7"] != 3 || s.top["telegram:-5"] != 0 {
		t.Fatalf("cursors: %v", s.top)
	}
	again, err := s.History(context.Background(), time.Time{})
	if err != nil || len(again) != 2 || again[0].ChatID != "telegram:-5" {
		t.Fatalf("retry must fetch the pending chat only: n=%d err=%v", len(again), err)
	}
}

func TestSendRequiresKnownChatAndText(t *testing.T) {
	f := newFakeAPI()
	s := newService(f)
	if err := s.Send(context.Background(), "telegram:7", "hi"); err == nil {
		t.Fatal("unknown chat before Dialogs")
	}
	if _, err := s.Dialogs(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.Send(context.Background(), "telegram:7", "  "); err == nil {
		t.Fatal("empty text")
	}
	if err := s.Send(context.Background(), "telegram:7", "hello"); err != nil || len(f.sent) != 1 || f.sent[0] != "hello" {
		t.Fatalf("%v %v", err, f.sent)
	}
}

func TestOwnMessagesAreFromMe(t *testing.T) {
	ents := collect([]tg.UserClass{&tg.User{ID: 7, FirstName: "Test"}}, nil)
	m := &tg.Message{ID: 1, Out: true, FromID: &tg.PeerUser{UserID: 7}, Message: "x"}
	if got := sender(entity{kind: kindGroup}, m, ents); got != "me" {
		t.Fatal(got)
	}
}
