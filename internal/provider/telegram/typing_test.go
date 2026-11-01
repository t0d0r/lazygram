package telegram

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/gotd/td/tg"

	"lazygram/internal/domain"
)

var typingAt = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

func TestTypingEventMapsEveryUpdateAndAction(t *testing.T) {
	who := func(id int64) string {
		if id == 7 {
			return "Ada Lovelace"
		}
		return ""
	}
	cases := []struct {
		name string
		upd  tg.UpdateClass
		want domain.Typing
		ok   bool
	}{
		{"private chat, typing", &tg.UpdateUserTyping{UserID: 7, Action: &tg.SendMessageTypingAction{}},
			domain.Typing{ChatID: "telegram:7", Who: "Ada Lovelace", Kind: domain.TypingText, At: typingAt}, true},
		{"private chat, unknown user", &tg.UpdateUserTyping{UserID: 8, Action: &tg.SendMessageRecordAudioAction{}},
			domain.Typing{ChatID: "telegram:8", Who: "", Kind: domain.TypingRecord, At: typingAt}, true},
		{"basic group, cancel", &tg.UpdateChatUserTyping{ChatID: 55, FromID: &tg.PeerUser{UserID: 7}, Action: &tg.SendMessageCancelAction{}},
			domain.Typing{ChatID: "telegram:-55", Who: "Ada Lovelace", Kind: domain.TypingStop, At: typingAt}, true},
		{"supergroup, upload", &tg.UpdateChannelUserTyping{ChannelID: 1234, FromID: &tg.PeerUser{UserID: 7}, Action: &tg.SendMessageUploadPhotoAction{}},
			domain.Typing{ChatID: "telegram:-1000000001234", Who: "Ada Lovelace", Kind: domain.TypingUpload, At: typingAt}, true},
		{"channel posting as itself", &tg.UpdateChannelUserTyping{ChannelID: 1234, FromID: &tg.PeerChannel{ChannelID: 1234}, Action: &tg.SendMessageChooseStickerAction{}},
			domain.Typing{ChatID: "telegram:-1000000001234", Who: "", Kind: domain.TypingChoose, At: typingAt}, true},
		{"history import is not composing", &tg.UpdateUserTyping{UserID: 7, Action: &tg.SendMessageHistoryImportAction{}}, domain.Typing{}, false},
		{"speaking in a group call is not composing", &tg.UpdateChannelUserTyping{ChannelID: 1234, FromID: &tg.PeerUser{UserID: 7}, Action: &tg.SpeakingInGroupCallAction{}}, domain.Typing{}, false},
		{"a live draft is composing", &tg.UpdateUserTyping{UserID: 7, Action: &tg.SendMessageTextDraftAction{}},
			domain.Typing{ChatID: "telegram:7", Who: "Ada Lovelace", Kind: domain.TypingText, At: typingAt}, true},
		{"another update type", &tg.UpdateNewMessage{}, domain.Typing{}, false},
	}
	for _, c := range cases {
		got, ok := typingEvent(c.upd, who, typingAt)
		if ok != c.ok || got != c.want {
			t.Errorf("%s: got %+v %v, want %+v %v", c.name, got, ok, c.want, c.ok)
		}
	}
	// Every composing action maps; the kinds are the ones the TUI shows.
	kinds := map[tg.SendMessageActionClass]domain.TypingKind{
		&tg.SendMessageTypingAction{}:           domain.TypingText,
		&tg.SendMessageRecordVideoAction{}:      domain.TypingRecord,
		&tg.SendMessageRecordRoundAction{}:      domain.TypingRecord,
		&tg.SendMessageUploadAudioAction{}:      domain.TypingUpload,
		&tg.SendMessageUploadVideoAction{}:      domain.TypingUpload,
		&tg.SendMessageUploadDocumentAction{}:   domain.TypingUpload,
		&tg.SendMessageUploadRoundAction{}:      domain.TypingUpload,
		&tg.SendMessageChooseContactAction{}:    domain.TypingChoose,
		&tg.SendMessageGeoLocationAction{}:      domain.TypingChoose,
		&tg.SendMessageGamePlayAction{}:         domain.TypingChoose,
		&tg.SendMessageCancelAction{}:           domain.TypingStop,
		&tg.SendMessageRichMessageDraftAction{}: domain.TypingText,
		&tg.SendMessageStopDraftAction{}:        domain.TypingStop,
	}
	for a, want := range kinds {
		if got, ok := typingKind(a); !ok || got != want {
			t.Errorf("%T: %v %v, want %v", a, got, ok, want)
		}
	}
}

func TestTypingFeedNeverBlocksAndClosesOnce(t *testing.T) {
	f := newTypingFeed()
	for i := 0; i < typingBuffer; i++ {
		if !f.emit(domain.Typing{ChatID: "c", Who: "x", At: typingAt}) {
			t.Fatalf("report %d did not fit", i)
		}
	}
	done := make(chan bool, 1)
	go func() { done <- f.emit(domain.Typing{ChatID: "c", Who: "y", At: typingAt}) }()
	select {
	case fit := <-done:
		if fit {
			t.Fatal("a full feed must drop, not queue")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("emit blocked on a full feed")
	}
	f.closeFeed()
	f.closeFeed() // twice is fine
	n := 0
	for range f.ch {
		n++
	}
	if n != typingBuffer {
		t.Fatalf("drained %d reports, want %d", n, typingBuffer)
	}
}

func TestTypingDispatcherFeedsTheChannel(t *testing.T) {
	f := newTypingFeed()
	d := tg.NewUpdateDispatcher()
	n := newNames()
	n.learn(collect([]tg.UserClass{&tg.User{ID: 7, FirstName: "Ada"}}, nil))
	f.register(d, n, func() time.Time { return typingAt })
	upd := &tg.UpdateShort{Update: &tg.UpdateUserTyping{UserID: 7, Action: &tg.SendMessageTypingAction{}}, Date: int(typingAt.Unix())}
	if err := d.Handle(context.Background(), upd); err != nil {
		t.Fatal(err)
	}
	select {
	case ev := <-f.ch:
		if ev != (domain.Typing{ChatID: "telegram:7", Who: "Ada", Kind: domain.TypingText, At: typingAt}) {
			t.Fatalf("got %+v", ev)
		}
	default:
		t.Fatal("the dispatcher delivered nothing")
	}
	// An update the feed does not care about leaves it empty.
	other := &tg.UpdateShort{Update: &tg.UpdateUserTyping{UserID: 7, Action: &tg.SendMessageHistoryImportAction{}}, Date: int(typingAt.Unix())}
	_ = d.Handle(context.Background(), other)
	if len(f.ch) != 0 {
		t.Fatal("an import action is not a report")
	}
	// The users an update container carries name a member never seen before.
	boxed := &tg.Updates{Users: []tg.UserClass{&tg.User{ID: 9, FirstName: "Bob"}},
		Updates: []tg.UpdateClass{&tg.UpdateChatUserTyping{ChatID: 55, FromID: &tg.PeerUser{UserID: 9}, Action: &tg.SendMessageTypingAction{}}}}
	if err := d.Handle(context.Background(), boxed); err != nil {
		t.Fatal(err)
	}
	select {
	case ev := <-f.ch:
		if ev.Who != "Bob" || ev.ChatID != "telegram:-55" || n.get(9) != "Bob" {
			t.Fatalf("learned from the container: %+v, registry %q", ev, n.get(9))
		}
	default:
		t.Fatal("the container's update was not delivered")
	}
}

func TestNilFeedCarriesNothing(t *testing.T) {
	var f *typingFeed
	if f.feed() != nil || f.emit(domain.Typing{}) {
		t.Fatal("a nil feed has no channel and takes nothing")
	}
	f.closeFeed()
	f.register(tg.NewUpdateDispatcher(), newNames(), time.Now)
	c := &MTProto{} // built without NewClient, as some tests do
	if c.Typing() != nil {
		t.Fatal("a client without a feed reports nil")
	}
}

func TestNamesAreSafeToReadWhileLearning(t *testing.T) {
	n := newNames()
	var nilNames *names
	nilNames.learn(entities{})
	if nilNames.get(1) != "" {
		t.Fatal("a nil registry knows nothing")
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				n.learn(collect([]tg.UserClass{&tg.User{ID: int64(i), FirstName: "u"}}, nil))
			}
		}(i)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = n.get(int64(i))
			}
		}(i)
	}
	wg.Wait()
	if n.get(0) != "u" || n.get(99) != "" {
		t.Fatalf("names: %q %q", n.get(0), n.get(99))
	}
}
