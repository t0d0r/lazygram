package telegram

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"

	"lazygram/internal/domain"
)

// fakeAPI serves invented dialogs and a descending message list per peer.
type fakeAPI struct {
	msgs    map[int64][]tg.MessageClass // by user id, newest first
	top     map[int64]int               // top message id per dialog
	dialogs int
	history int
	flood   int // FLOOD_WAIT errors still to return
	sent    []string
	pressed []string // callback data pressed
	fetched []string // "messages" or "channels" per refetch
	edited  []string // "id:text"
	deleted []string // "messages:[ids]:revoke" or "channels:[ids]"

	extraDialogs []tg.DialogClass // listed before the fixed dialogs
	extraChats   []tg.ChatClass

	file      []byte // served by UploadGetFile
	downloads int
}

func (f *fakeAPI) UploadGetFile(_ context.Context, r *tg.UploadGetFileRequest) (tg.UploadFileClass, error) {
	f.downloads++
	off := int(r.Offset)
	if off >= len(f.file) {
		return &tg.UploadFile{Type: &tg.StorageFileJpeg{}}, nil
	}
	end := min(off+r.Limit, len(f.file))
	return &tg.UploadFile{Type: &tg.StorageFileJpeg{}, Bytes: f.file[off:end]}, nil
}

func (f *fakeAPI) UploadGetFileHashes(context.Context, *tg.UploadGetFileHashesRequest) ([]tg.FileHash, error) {
	return nil, errors.New("not in the fake")
}

func (f *fakeAPI) UploadReuploadCDNFile(context.Context, *tg.UploadReuploadCDNFileRequest) ([]tg.FileHash, error) {
	return nil, errors.New("not in the fake")
}

func (f *fakeAPI) UploadGetCDNFileHashes(context.Context, *tg.UploadGetCDNFileHashesRequest) ([]tg.FileHash, error) {
	return nil, errors.New("not in the fake")
}

func (f *fakeAPI) UploadGetWebFile(context.Context, *tg.UploadGetWebFileRequest) (*tg.UploadWebFile, error) {
	return nil, errors.New("not in the fake")
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
		Dialogs: append(append([]tg.DialogClass{}, f.extraDialogs...), []tg.DialogClass{
			&tg.Dialog{Peer: &tg.PeerUser{UserID: 7}, TopMessage: f.top[7], UnreadCount: 3},
			&tg.Dialog{Peer: &tg.PeerChat{ChatID: 5}, TopMessage: f.top[-5]},
			&tg.Dialog{Peer: &tg.PeerChannel{ChannelID: 9}, TopMessage: f.top[-9]},
			&tg.Dialog{Peer: &tg.PeerUser{UserID: 8}},
			&tg.Dialog{Peer: &tg.PeerUser{UserID: 1}},
			&tg.Dialog{Peer: &tg.PeerChannel{ChannelID: 12}},
			&tg.Dialog{Peer: &tg.PeerUser{UserID: 13}, TopMessage: 5},
			&tg.Dialog{Peer: &tg.PeerChannel{ChannelID: 99}}, // forbidden: no entity
			&tg.DialogFolder{},
		}...),
		Users: []tg.UserClass{
			&tg.User{ID: 7, AccessHash: 1, FirstName: "Test", LastName: "Person", Username: "testperson"},
			&tg.User{ID: 8, AccessHash: 2, Username: "nameless"},
			&tg.User{ID: 1, Self: true, FirstName: "Me"},
			&tg.User{ID: 13, Deleted: true},
		},
		Chats: append(append([]tg.ChatClass{}, f.extraChats...), []tg.ChatClass{
			&tg.Chat{ID: 5, Title: "Test Group"},
			&tg.Channel{ID: 9, AccessHash: 3, Title: "Test Channel", Username: "testchan"},
			&tg.Channel{ID: 12, AccessHash: 4, Title: "Super Group", Megagroup: true},
			&tg.ChannelForbidden{ID: 99},
		}...),
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

func (f *fakeAPI) MessagesGetBotCallbackAnswer(_ context.Context, r *tg.MessagesGetBotCallbackAnswerRequest) (*tg.MessagesBotCallbackAnswer, error) {
	data, _ := r.GetData()
	f.pressed = append(f.pressed, string(data))
	if string(data) == "boom" {
		return nil, errors.New("BOT_RESPONSE_FAILED")
	}
	if string(data) == "slow" {
		return nil, tgerr.New(400, "BOT_RESPONSE_TIMEOUT")
	}
	// The bot edits its message in response, as bots do.
	for key, msgs := range f.msgs {
		for i, mc := range msgs {
			if m, ok := mc.(*tg.Message); ok && m.ID == r.MsgID {
				edited := *m
				edited.Message = "you pressed " + string(data)
				edited.ReplyMarkup = nil
				edited.Flags.Unset(6)
				f.msgs[key][i] = &edited
			}
		}
	}
	a := &tg.MessagesBotCallbackAnswer{}
	a.SetMessage("thanks for " + string(data))
	return a, nil
}

func (f *fakeAPI) find(id int) tg.MessageClass {
	for _, msgs := range f.msgs {
		for _, mc := range msgs {
			if mc.GetID() == id {
				return mc
			}
		}
	}
	return nil
}

func (f *fakeAPI) MessagesGetMessages(_ context.Context, ids []tg.InputMessageClass) (tg.MessagesMessagesClass, error) {
	f.fetched = append(f.fetched, "messages")
	var out []tg.MessageClass
	for _, ic := range ids {
		if m := f.find(ic.(*tg.InputMessageID).ID); m != nil {
			out = append(out, m)
		}
	}
	return &tg.MessagesMessages{Messages: out, Users: []tg.UserClass{&tg.User{ID: 7, FirstName: "Test", LastName: "Person"}}}, nil
}

func (f *fakeAPI) ChannelsGetMessages(_ context.Context, r *tg.ChannelsGetMessagesRequest) (tg.MessagesMessagesClass, error) {
	f.fetched = append(f.fetched, "channels")
	var out []tg.MessageClass
	for _, ic := range r.ID {
		if m := f.find(ic.(*tg.InputMessageID).ID); m != nil {
			out = append(out, m)
		}
	}
	return &tg.MessagesChannelMessages{Messages: out}, nil
}

func (f *fakeAPI) MessagesEditMessage(_ context.Context, r *tg.MessagesEditMessageRequest) (tg.UpdatesClass, error) {
	msg, _ := r.GetMessage()
	f.edited = append(f.edited, fmt.Sprintf("%d:%s", r.ID, msg))
	return &tg.Updates{}, nil
}

func (f *fakeAPI) MessagesDeleteMessages(_ context.Context, r *tg.MessagesDeleteMessagesRequest) (*tg.MessagesAffectedMessages, error) {
	f.deleted = append(f.deleted, fmt.Sprintf("messages:%v:%v", r.ID, r.Revoke))
	return &tg.MessagesAffectedMessages{}, nil
}

func (f *fakeAPI) ChannelsDeleteMessages(_ context.Context, r *tg.ChannelsDeleteMessagesRequest) (*tg.MessagesAffectedMessages, error) {
	f.deleted = append(f.deleted, fmt.Sprintf("channels:%v", r.ID))
	return &tg.MessagesAffectedMessages{}, nil
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
	// Recent before Dialogs loads the dialogs itself.
	if _, err := s.Recent(context.Background(), "telegram:7", 10); err != nil || f.dialogs != 1 {
		t.Fatalf("auto-load: %v dialogs=%d", err, f.dialogs)
	}
	if _, err := s.Recent(context.Background(), "telegram:404", 10); err == nil || !strings.Contains(err.Error(), "unknown chat") {
		t.Fatalf("still unknown after a refresh: %v", err)
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
	if err := s.Send(context.Background(), "telegram:999", "hi"); err == nil {
		t.Fatal("unknown chat")
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

func TestUsernamesClaimNamesBeforeTitles(t *testing.T) {
	f := newFakeAPI()
	// A group titled "testperson" is listed before the person @testperson.
	f.extraChats = []tg.ChatClass{&tg.Chat{ID: 50, Title: "testperson"}}
	f.extraDialogs = []tg.DialogClass{&tg.Dialog{Peer: &tg.PeerChat{ChatID: 50}, TopMessage: 1}}
	chats, err := newService(f).Dialogs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]string{}
	for _, c := range chats {
		names[c.ID] = c.Name()
	}
	if names["telegram:7"] != "@testperson" || names["telegram:-50"] != "#testperson-2" {
		t.Fatalf("%v", names)
	}
}

func withButtons(m *tg.Message) *tg.Message {
	m.SetReplyMarkup(&tg.ReplyInlineMarkup{Rows: []tg.KeyboardInlineButtonRow{
		{Buttons: []tg.KeyboardInlineButton{
			{Text: "OK", Type: &tg.InlineButtonTypeCallback{Data: []byte("ok")}},
			{Text: "Docs", Type: &tg.InlineButtonTypeURL{URL: "https://example.invalid/docs"}},
		}},
		{Buttons: []tg.KeyboardInlineButton{
			{Text: "Share", Type: &tg.InlineButtonTypeSwitchInline{Query: "q"}},
			{Text: "Secure", Type: &tg.InlineButtonTypeCallback{Data: []byte("x"), RequiresPassword: true}},
		}},
	}})
	return m
}

func TestButtonsAreConvertedRowByRow(t *testing.T) {
	m := withButtons(tmsg(1, 0, "pick"))
	out := convert("telegram:7", entity{kind: kindUser}, m, collect(nil, nil))
	if len(out.Buttons) != 4 {
		t.Fatalf("%+v", out.Buttons)
	}
	if b := out.Buttons[0]; b.Kind != domain.ButtonCallback || b.Label != "OK" || string(b.Data) != "ok" {
		t.Fatalf("%+v", b)
	}
	if b := out.Buttons[1]; b.Kind != domain.ButtonURL || b.URL != "https://example.invalid/docs" {
		t.Fatalf("%+v", b)
	}
	if b := out.Buttons[2]; b.Kind != domain.ButtonOther || b.Label != "Share" {
		t.Fatalf("%+v", b)
	}
	if b := out.Buttons[3]; b.Kind != domain.ButtonOther {
		t.Fatalf("password-protected callbacks are unsupported: %+v", b)
	}
	reply := tmsg(2, 0, "keyboard")
	reply.SetReplyMarkup(&tg.ReplyKeyboardMarkup{Rows: []tg.KeyboardButtonRow{{Buttons: []tg.KeyboardButton{{Text: "Yes", Type: &tg.ButtonTypeDefault{}}}}}})
	if out := convert("telegram:7", entity{}, reply, collect(nil, nil)); len(out.Buttons) != 1 || out.Buttons[0].Kind != domain.ButtonText {
		t.Fatalf("reply keyboard: %+v", out.Buttons)
	}
	if out := convert("telegram:7", entity{}, tmsg(3, 0, "plain"), collect(nil, nil)); len(out.Buttons) != 0 {
		t.Fatal("no markup, no buttons")
	}
}

func TestPressAnswersAndRefetches(t *testing.T) {
	f := newFakeAPI()
	f.msgs[7] = []tg.MessageClass{withButtons(tmsg(5, 5, "deploy?"))}
	f.msgs[-9] = []tg.MessageClass{withButtons(tmsg(8, 8, "channel post"))}
	s := newService(f)
	waited := 0
	s.wait = func(context.Context, time.Duration) error { waited++; return nil }
	if _, err := s.Press(context.Background(), "telegram:999", "5", domain.Button{Kind: domain.ButtonCallback, Data: []byte("ok")}); err == nil {
		t.Fatal("unknown chat")
	}
	a, err := s.Press(context.Background(), "telegram:7", "5", domain.Button{Kind: domain.ButtonCallback, Data: []byte("ok")})
	if err != nil || a.Text != "thanks for ok" || a.Updated == nil || a.Updated.Text != "you pressed ok" || len(a.Updated.Buttons) != 0 {
		t.Fatalf("%+v %v", a, err)
	}
	if f.pressed[0] != "ok" || f.fetched[0] != "messages" {
		t.Fatalf("pressed=%v fetched=%v", f.pressed, f.fetched)
	}
	// Channels use channels.getMessages for the refetch.
	if _, err := s.Press(context.Background(), "telegram:-1000000000009", "8", domain.Button{Kind: domain.ButtonCallback, Data: []byte("ok")}); err != nil || f.fetched[1] != "channels" {
		t.Fatalf("%v %v", err, f.fetched)
	}
	if _, err := s.Press(context.Background(), "telegram:7", "5", domain.Button{Kind: domain.ButtonURL}); err == nil {
		t.Fatal("url buttons are not pressed through the API")
	}
	if _, err := s.Press(context.Background(), "telegram:7", "x", domain.Button{Kind: domain.ButtonCallback}); err == nil {
		t.Fatal("bad id")
	}
	if _, err := s.Press(context.Background(), "telegram:7", "5", domain.Button{Kind: domain.ButtonCallback, Data: []byte("boom")}); err == nil {
		t.Fatal("bot error must surface")
	}
	if waited == 0 {
		t.Fatal("a press waits for the bot's edit before refetching")
	}
	// A bot that never answers the query still gets its edit picked up.
	a, err = s.Press(context.Background(), "telegram:7", "5", domain.Button{Kind: domain.ButtonCallback, Data: []byte("slow")})
	if err != nil || a.Updated == nil || !strings.Contains(a.Text, "no answer") {
		t.Fatalf("timeout: %+v %v", a, err)
	}
}

func withPhoto(m *tg.Message) *tg.Message {
	m.SetMedia(&tg.MessageMediaPhoto{Photo: &tg.Photo{ID: 77, AccessHash: 8, FileReference: []byte("ref"), Sizes: []tg.PhotoSizeClass{
		&tg.PhotoSize{Type: "m", W: 320, H: 200},
		&tg.PhotoSizeProgressive{Type: "y", W: 1280, H: 800, Sizes: []int{1000, 20000}},
		&tg.PhotoSize{Type: "x", W: 800, H: 500},
	}}})
	return m
}

func TestMediaConversion(t *testing.T) {
	photo := convert("telegram:7", entity{}, withPhoto(tmsg(1, 0, "")), collect(nil, nil))
	if photo.Media == nil || photo.Media.Kind != "photo" || photo.Text != "" {
		t.Fatalf("%+v", photo)
	}
	doc := tmsg(2, 0, "look")
	doc.SetMedia(&tg.MessageMediaDocument{Document: &tg.Document{ID: 5, MimeType: "image/png", Attributes: []tg.DocumentAttributeClass{&tg.DocumentAttributeFilename{FileName: "C:\\\\pics\\\\cat.png"}}}})
	img := convert("telegram:7", entity{}, doc, collect(nil, nil))
	if img.Media == nil || img.Media.Kind != "image" || img.Media.Name != "cat.png" || img.Text != "look" {
		t.Fatalf("%+v", img.Media)
	}
	// A sender-chosen name never decides what gets opened: the mime type does.
	trap := tmsg(6, 0, "")
	trap.SetMedia(&tg.MessageMediaDocument{Document: &tg.Document{ID: 8, MimeType: "image/png", Attributes: []tg.DocumentAttributeClass{&tg.DocumentAttributeFilename{FileName: "photo.html"}}}})
	if _, ext, err := location(trap); err != nil || ext != ".png" {
		t.Fatalf("image extension must follow the mime type: %q %v", ext, err)
	}
	svg := tmsg(7, 0, "")
	svg.SetMedia(&tg.MessageMediaDocument{Document: &tg.Document{ID: 9, MimeType: "image/svg+xml", Attributes: []tg.DocumentAttributeClass{&tg.DocumentAttributeFilename{FileName: "a.svg"}}}})
	if g := convert("telegram:7", entity{}, svg, collect(nil, nil)); g.Media == nil || g.Media.Kind != "file" {
		t.Fatalf("svg is a file, never auto-opened: %+v", g.Media)
	}
	pdf := tmsg(3, 0, "")
	pdf.SetMedia(&tg.MessageMediaDocument{Document: &tg.Document{ID: 6, MimeType: "application/pdf"}})
	if f := convert("telegram:7", entity{}, pdf, collect(nil, nil)); f.Media == nil || f.Media.Kind != "file" || f.Text != "" {
		t.Fatalf("%+v", f)
	}
	poll := tmsg(4, 0, "")
	poll.SetMedia(&tg.MessageMediaGeo{})
	if g := convert("telegram:7", entity{}, poll, collect(nil, nil)); g.Media != nil || g.Text != "[media]" {
		t.Fatalf("%+v", g)
	}
	loc, ext, err := location(withPhoto(tmsg(1, 0, "")))
	if err != nil || ext != ".jpg" || loc.(*tg.InputPhotoFileLocation).ThumbSize != "y" {
		t.Fatalf("largest size: %v %s %v", loc, ext, err)
	}
	if _, _, err := location(tmsg(5, 0, "plain")); err == nil {
		t.Fatal("no media")
	}
}

func TestFetchStreamsIntoMemory(t *testing.T) {
	f := newFakeAPI()
	f.file = []byte("JPEGDATA-JPEGDATA")
	f.msgs[7] = []tg.MessageClass{withPhoto(tmsg(5, 5, ""))}
	s := newService(f)
	if _, _, err := s.Fetch(context.Background(), "telegram:7", "5"); err == nil {
		t.Fatal("no downloader configured")
	}
	s.files = f
	data, name, err := s.Fetch(context.Background(), "telegram:7", "5")
	if err != nil || string(data) != "JPEGDATA-JPEGDATA" || name != "5.jpg" {
		t.Fatalf("%q %q %v", data, name, err)
	}
	if _, _, err := s.Fetch(context.Background(), "telegram:7", "404"); err == nil {
		t.Fatal("unknown message")
	}
	lw := &limitWriter{w: io.Discard, left: 3}
	if _, err := lw.Write([]byte("abcd")); err == nil {
		t.Fatal("the size cap must hold")
	}
}

func TestEditAndDelete(t *testing.T) {
	f := newFakeAPI()
	s := newService(f)
	if err := s.Edit(context.Background(), "telegram:7", "5", "new text"); err != nil || f.edited[0] != "5:new text" {
		t.Fatalf("%v %v", err, f.edited)
	}
	if err := s.Edit(context.Background(), "telegram:7", "5", "  "); err == nil {
		t.Fatal("empty edit")
	}
	if err := s.Delete(context.Background(), "telegram:7", "5"); err != nil || f.deleted[0] != "messages:[5]:true" {
		t.Fatalf("%v %v", err, f.deleted)
	}
	if err := s.Delete(context.Background(), "telegram:-1000000000009", "8"); err != nil || f.deleted[1] != "channels:[8]" {
		t.Fatalf("channel delete: %v %v", err, f.deleted)
	}
	if err := s.Delete(context.Background(), "telegram:7", "x"); err == nil {
		t.Fatal("bad id")
	}
}

func TestSendFileUsesTheUploader(t *testing.T) {
	f := newFakeAPI()
	c := &MTProto{cfg: testCfg, ready: make(chan struct{})}
	c.svc = newService(f)
	var got []string
	c.svc.upload = func(_ context.Context, peer tg.InputPeerClass, name string, data []byte, caption string, photo bool) error {
		got = append(got, fmt.Sprintf("%T:%s:%d:%s:%v", peer, name, len(data), caption, photo))
		return nil
	}
	close(c.ready)
	if err := c.SendFile(context.Background(), "telegram:7", "cat.JPG", []byte("abc"), "hi"); err != nil || len(got) != 1 || got[0] != "*tg.InputPeerUser:cat.JPG:3:hi:true" {
		t.Fatalf("%v %v", err, got)
	}
	if err := c.SendFile(context.Background(), "telegram:-5", "notes.txt", []byte("abc"), ""); err != nil || got[1] != "*tg.InputPeerChat:notes.txt:3::false" {
		t.Fatalf("%v %v", err, got)
	}
	c.svc.upload = nil
	if err := c.SendFile(context.Background(), "telegram:7", "a.png", []byte("x"), ""); err == nil {
		t.Fatal("no uploader")
	}
}
