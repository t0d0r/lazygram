package telegram

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gotd/td/tg"

	"lazygram/internal/config"
)

// fakeAPI serves invented dialogs and a descending message list per peer.
type fakeAPI struct {
	msgs  []tg.MessageClass // newest first
	calls int
}

func (f *fakeAPI) MessagesGetDialogs(context.Context, *tg.MessagesGetDialogsRequest) (tg.MessagesDialogsClass, error) {
	return &tg.MessagesDialogs{
		Users: []tg.UserClass{
			&tg.User{ID: 7, AccessHash: 1, FirstName: "Test", LastName: "Person", Username: "testperson"},
			&tg.User{ID: 8, AccessHash: 2, Username: "nameless"},
		},
		Chats: []tg.ChatClass{
			&tg.Chat{ID: 5, Title: "Test Group"},
			&tg.Channel{ID: 9, AccessHash: 3, Title: "Test Channel", Username: "testchan"},
		},
	}, nil
}

func (f *fakeAPI) MessagesGetHistory(_ context.Context, r *tg.MessagesGetHistoryRequest) (tg.MessagesMessagesClass, error) {
	f.calls++
	var page []tg.MessageClass
	for _, m := range f.msgs {
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

func TestDialogsResolvesRefs(t *testing.T) {
	s := &service{api: &fakeAPI{}}
	chats, err := s.Dialogs(context.Background(), []config.ChatRef{
		{Alias: "p", Ref: "@TestPerson"}, {Alias: "g", Ref: "-5"}, {Alias: "c", Ref: "-1000000000009"}, {Alias: "n", Ref: "8"},
	})
	if err != nil || len(chats) != 4 {
		t.Fatalf("%v %v", chats, err)
	}
	if chats[0].ID != "telegram:7" || chats[0].Title != "Test Person" || chats[0].Service != "telegram" {
		t.Fatalf("%+v", chats[0])
	}
	if chats[3].Title != "nameless" {
		t.Fatalf("username fallback: %+v", chats[3])
	}
}

func TestDialogsReportsUnknownAndMissingConfig(t *testing.T) {
	s := &service{api: &fakeAPI{}}
	_, err := s.Dialogs(context.Background(), []config.ChatRef{{Alias: "ghost", Ref: "@nobody"}, {Alias: "bad", Ref: "x"}})
	if err == nil || !strings.Contains(err.Error(), "ghost") || !strings.Contains(err.Error(), "bad") {
		t.Fatalf("got %v", err)
	}
	if _, err := s.Dialogs(context.Background(), nil); err == nil || !strings.Contains(err.Error(), config.EnvTGChats) {
		t.Fatalf("got %v", err)
	}
}

func loaded(t *testing.T, f *fakeAPI) *service {
	t.Helper()
	s := &service{api: f}
	if _, err := s.Dialogs(context.Background(), []config.ChatRef{{Alias: "p", Ref: "7"}}); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestHistoryBeforeDialogsFails(t *testing.T) {
	if _, err := (&service{api: &fakeAPI{}}).History(context.Background(), time.Time{}); err == nil {
		t.Fatal("want error")
	}
}

func TestHistoryConvertsAndSkipsServiceMessages(t *testing.T) {
	f := &fakeAPI{msgs: []tg.MessageClass{
		&tg.Message{ID: 4, Date: int(base.Unix()) + 3, Media: &tg.MessageMediaEmpty{}, FromID: &tg.PeerUser{UserID: 7}},
		&tg.MessageService{ID: 3, Date: int(base.Unix()) + 2},
		&tg.Message{ID: 2, Date: int(base.Unix()) + 1, Message: "hi", PostAuthor: "editor"},
		tmsg(1, 0, "first"),
	}}
	got, err := loaded(t, f).History(context.Background(), time.Time{})
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
	if byID["4"] != "Test Person|[media]" || byID["2"] != "editor|hi" || byID["1"] != "Test Person|first" {
		t.Fatalf("%v", byID)
	}
}

func TestHistoryPagesAndStopsAtSince(t *testing.T) {
	var msgs []tg.MessageClass
	for id := 250; id >= 1; id-- {
		msgs = append(msgs, tmsg(id, id, "m"))
	}
	f := &fakeAPI{msgs: msgs}
	s := loaded(t, f)

	all, err := s.History(context.Background(), time.Time{})
	if err != nil || len(all) != 250 || f.calls != 3 {
		t.Fatalf("all: n=%d calls=%d err=%v", len(all), f.calls, err)
	}

	f.calls = 0
	recent, _ := s.History(context.Background(), base.Add(240*time.Second))
	if len(recent) != 10 || f.calls != 1 {
		t.Fatalf("since: n=%d calls=%d", len(recent), f.calls)
	}

	f.calls = 0
	none, _ := s.History(context.Background(), base.Add(250*time.Second))
	if len(none) != 0 || f.calls != 1 {
		t.Fatalf("up to date: n=%d calls=%d", len(none), f.calls)
	}
}

func TestHistoryCapsPerChat(t *testing.T) {
	var msgs []tg.MessageClass
	for id := 1000; id >= 1; id-- {
		msgs = append(msgs, tmsg(id, id, "m"))
	}
	got, _ := loaded(t, &fakeAPI{msgs: msgs}).History(context.Background(), time.Time{})
	if len(got) != maxPerChat {
		t.Fatalf("got %d, want cap %d", len(got), maxPerChat)
	}
}
