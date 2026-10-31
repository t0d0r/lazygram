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
			&tg.Dialog{Peer: &tg.PeerUser{UserID: 7}, TopMessage: f.top[7]},
			&tg.Dialog{Peer: &tg.PeerChat{ChatID: 5}, TopMessage: f.top[-5]},
			&tg.Dialog{Peer: &tg.PeerChannel{ChannelID: 9}, TopMessage: f.top[-9]},
			&tg.Dialog{Peer: &tg.PeerUser{UserID: 8}},
			&tg.Dialog{Peer: &tg.PeerUser{UserID: 1}},
			&tg.Dialog{Peer: &tg.PeerChannel{ChannelID: 99}}, // forbidden: no entity
			&tg.DialogFolder{},
		},
		Users: []tg.UserClass{
			&tg.User{ID: 7, AccessHash: 1, FirstName: "Test", LastName: "Person", Username: "testperson"},
			&tg.User{ID: 8, AccessHash: 2, Username: "nameless"},
			&tg.User{ID: 1, Self: true, FirstName: "Me"},
		},
		Chats: []tg.ChatClass{
			&tg.Chat{ID: 5, Title: "Test Group"},
			&tg.Channel{ID: 9, AccessHash: 3, Title: "Test Channel", Username: "testchan"},
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
		got[c.ID] = c.Alias + "|" + c.Title
		if c.Service != "telegram" {
			t.Fatalf("%+v", c)
		}
	}
	want := map[string]string{
		"telegram:7":              "testperson|Test Person",
		"telegram:-5":             "test-group|Test Group",
		"telegram:-1000000000009": "testchan|Test Channel",
		"telegram:8":              "nameless|nameless",
		"telegram:1":              "saved|Saved Messages",
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

func TestHistoryConvertsAndSkipsServiceMessages(t *testing.T) {
	f := newFakeAPI()
	f.msgs[7] = []tg.MessageClass{
		&tg.Message{ID: 4, Date: int(base.Unix()) + 3, Media: &tg.MessageMediaEmpty{}, FromID: &tg.PeerUser{UserID: 7}},
		&tg.MessageService{ID: 3, Date: int(base.Unix()) + 2},
		&tg.Message{ID: 2, Date: int(base.Unix()) + 1, Message: "hi", PostAuthor: "editor"},
		&tg.Message{ID: 1, Date: int(base.Unix()), Message: "mine", Out: true},
	}
	f.top[7] = 4
	got, err := newService(f).History(context.Background(), time.Time{})
	if err != nil || len(got) != 3 {
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

func TestHistoryFetchesOnlyChangedChats(t *testing.T) {
	f := newFakeAPI()
	f.fill(10)
	s := newService(f)

	first, err := s.History(context.Background(), time.Time{})
	if err != nil || len(first) != 10 {
		t.Fatalf("first: %d %v", len(first), err)
	}
	// Only the dialog with messages is fetched; empty chats cost nothing.
	if f.dialogs != 1 || f.history != 1 {
		t.Fatalf("first: dialogs=%d history=%d", f.dialogs, f.history)
	}

	f.history = 0
	none, _ := s.History(context.Background(), time.Time{})
	if len(none) != 0 || f.history != 0 {
		t.Fatalf("unchanged: n=%d history=%d", len(none), f.history)
	}

	f.fill(12)
	f.history = 0
	more, _ := s.History(context.Background(), time.Time{})
	if len(more) != 2 || f.history != 1 || more[0].ID != "12" {
		t.Fatalf("changed: n=%d history=%d %+v", len(more), f.history, more)
	}
}

func TestHistoryInitialBacklogIsCapped(t *testing.T) {
	f := newFakeAPI()
	f.fill(500)
	got, _ := newService(f).History(context.Background(), time.Time{})
	if len(got) != initialPerChat {
		t.Fatalf("empty cache: got %d want %d", len(got), initialPerChat)
	}
}

func TestHistoryAfterRestartStopsAtSince(t *testing.T) {
	f := newFakeAPI()
	f.fill(250)
	s := newService(f) // fresh process: nothing tracked yet
	got, _ := s.History(context.Background(), base.Add(240*time.Second))
	if len(got) != 10 || f.history != 1 {
		t.Fatalf("since: n=%d history=%d", len(got), f.history)
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
	f.history = 0
	got, _ := s.History(context.Background(), base)
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
