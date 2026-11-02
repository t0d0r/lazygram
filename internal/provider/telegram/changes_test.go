package telegram

import (
	"context"
	"testing"

	"github.com/gotd/td/tg"

	"lazygram/internal/domain"
)

func TestChangeEvents(t *testing.T) {
	e := tg.Entities{Users: map[int64]*tg.User{7: {ID: 7, FirstName: "Test"}}}
	edit := func(peer tg.PeerClass) *tg.Message {
		return &tg.Message{ID: 42, PeerID: peer, FromID: &tg.PeerUser{UserID: 7}, Message: "now this"}
	}
	for _, c := range []struct {
		u    tg.UpdateClass
		want domain.Change
	}{
		{&tg.UpdateEditMessage{Message: edit(&tg.PeerUser{UserID: 7})}, domain.Change{Kind: domain.Edited, ChatID: "telegram:7"}},
		{&tg.UpdateEditMessage{Message: edit(&tg.PeerChat{ChatID: 5})}, domain.Change{Kind: domain.Edited, ChatID: "telegram:-5"}},
		{&tg.UpdateEditChannelMessage{Message: edit(&tg.PeerChannel{ChannelID: 9})}, domain.Change{Kind: domain.Edited, ChatID: "telegram:-1000000000009"}},
		{&tg.UpdateDeleteMessages{Messages: []int{3, 4}}, domain.Change{Kind: domain.Deleted, IDs: []string{"3", "4"}}},
		{&tg.UpdateDeleteChannelMessages{ChannelID: 9, Messages: []int{8}}, domain.Change{Kind: domain.Deleted, ChatID: "telegram:-1000000000009", IDs: []string{"8"}}},
	} {
		got, ok := changeEvent(c.u, e)
		if !ok || got.Kind != c.want.Kind || got.ChatID != c.want.ChatID || len(got.IDs) != len(c.want.IDs) {
			t.Errorf("%T: %+v %v", c.u, got, ok)
			continue
		}
		for i := range got.IDs {
			if got.IDs[i] != c.want.IDs[i] {
				t.Errorf("%T ids: %v", c.u, got.IDs)
			}
		}
		if got.Kind == domain.Edited && (got.Msg.ID != "42" || got.Msg.Text != "now this" || got.Msg.ChatID != c.want.ChatID) {
			t.Errorf("%T message: %+v", c.u, got.Msg)
		}
	}
	for _, u := range []tg.UpdateClass{
		&tg.UpdateEditMessage{Message: &tg.MessageService{ID: 1}},
		&tg.UpdateDeleteMessages{},
		&tg.UpdateUserTyping{UserID: 7},
	} {
		if _, ok := changeEvent(u, e); ok {
			t.Errorf("%T is no change", u)
		}
	}
}

func TestChangeFeedDropsWhenFullAndCloses(t *testing.T) {
	f := newChangeFeed()
	for i := 0; i < changeBuffer; i++ {
		if !f.emit(domain.Change{}) {
			t.Fatalf("change %d did not fit", i)
		}
	}
	if f.emit(domain.Change{}) {
		t.Fatal("a full feed drops instead of blocking")
	}
	f.closeFeed()
	f.closeFeed() // twice is fine
	var nilFeed *changeFeed
	if nilFeed.emit(domain.Change{}) || nilFeed.feed() != nil {
		t.Fatal("a nil feed carries nothing")
	}
}

func TestDialogsSayWhichChatsShareIDs(t *testing.T) {
	chats, err := newService(newFakeAPI()).Dialogs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range chats {
		want := c.Title != "Test Channel" && c.Title != "Super Group"
		if c.SharedIDs != want {
			t.Errorf("%s: shared ids %v, want %v", c.Title, c.SharedIDs, want)
		}
	}
}

func TestGapsAreReportedStale(t *testing.T) {
	if c, ok := changeEvent(&tg.UpdateChannelTooLong{ChannelID: 9}, tg.Entities{}); !ok || c.Kind != domain.Stale || c.ChatID != "telegram:-1000000000009" {
		t.Fatalf("channel too long: %+v %v", c, ok)
	}
	f := newChangeFeed()
	h := gapHandler{next: tg.NewUpdateDispatcher(), changes: f}
	if err := h.Handle(context.Background(), &tg.UpdatesTooLong{}); err != nil {
		t.Fatal(err)
	}
	select {
	case c := <-f.feed():
		if c.Kind != domain.Stale || c.ChatID != "" {
			t.Fatalf("updates too long: %+v", c)
		}
	default:
		t.Fatal("updatesTooLong reports every chat stale")
	}
}

func TestEditCarriesTheEditTimeAndAttachmentID(t *testing.T) {
	m := &tg.Message{ID: 1, PeerID: &tg.PeerUser{UserID: 7}, Message: "x",
		Media: &tg.MessageMediaPhoto{Photo: &tg.Photo{ID: 555}}}
	m.SetEditDate(1700000000)
	c, ok := changeEvent(&tg.UpdateEditMessage{Message: m}, tg.Entities{})
	if !ok || c.Msg.Edited.Unix() != 1700000000 || c.Msg.Media == nil || c.Msg.Media.ID != "555" {
		t.Fatalf("%+v", c.Msg)
	}
}
