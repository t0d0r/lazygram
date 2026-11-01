package telegram

import (
	"context"
	"sync"
	"time"

	"github.com/gotd/td/tg"

	"lazygram/internal/domain"
)

// Typing reports arrive as updates, not through the polled history, so the
// connector listens to the update stream for these three updates only:
// updateUserTyping (a private chat), updateChatUserTyping (a basic group)
// and updateChannelUserTyping (a supergroup or channel). Nothing else from
// the stream is used, and lazygram never sends a typing report of its own.

// typingBuffer is how many reports can wait for the TUI before new ones
// are dropped; a report is best effort and the next one renews it anyway.
const typingBuffer = 64

// names remembers the display names of the users seen in API responses,
// so that a typing update, which carries only an id, can be reported by
// the name messages use. Safe for concurrent use: the update handlers read
// and learn while syncs write. A nil *names learns and knows nothing.
type names struct {
	mu   sync.Mutex
	byID map[int64]string
}

func newNames() *names { return &names{byID: map[int64]string{}} }

// learn takes the users of one response in.
func (n *names) learn(e entities) { n.learnUsers(e.users) }

// learnUsers takes users in, keyed by id; the update dispatcher passes the
// ones an update container carries (an updateShort carries none).
func (n *names) learnUsers(users map[int64]*tg.User) {
	if n == nil || len(users) == 0 {
		return
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	for id, u := range users {
		n.byID[id] = userName(u)
	}
}

// get is the display name of a user, or "" when the user was never seen.
func (n *names) get(id int64) string {
	if n == nil {
		return ""
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.byID[id]
}

// typingFeed carries typing reports from the update handlers to whoever
// reads the channel. gotd calls the update handler concurrently: its read
// loop hands every received frame to a new goroutine, and the updates
// packed in one container are handled in order on that goroutine. emit is
// a non-blocking send and the names registry is locked, so that is safe;
// a handler must still never block, because the read loop waits for every
// handler goroutine before the connection shuts down. Two reports can
// reach the feed in the other order than they were received (typing
// updates carry no sequence number), so a stop can occasionally be applied
// before the start it follows; the TTL then ends it. emit drops a report
// when the reader is behind; closeFeed ends the channel once the
// connection has stopped delivering updates, which is after the client's
// Run has returned. A nil *typingFeed (a client built without NewClient)
// carries nothing.
type typingFeed struct {
	ch   chan domain.Typing
	once sync.Once
}

func newTypingFeed() *typingFeed {
	return &typingFeed{ch: make(chan domain.Typing, typingBuffer)}
}

// feed is the channel to read, nil for a nil feed.
func (f *typingFeed) feed() <-chan domain.Typing {
	if f == nil {
		return nil
	}
	return f.ch
}

// emit queues a report without waiting; it reports whether it fit.
func (f *typingFeed) emit(ev domain.Typing) bool {
	if f == nil {
		return false
	}
	select {
	case f.ch <- ev:
		return true
	default:
		return false
	}
}

// closeFeed closes the channel. It must run only after the last emit, that
// is after the client's Run has returned.
func (f *typingFeed) closeFeed() {
	if f == nil {
		return
	}
	f.once.Do(func() { close(f.ch) })
}

// register hooks the three typing updates up to the feed. The users an
// update container carries are learned first, so a member seen for the
// first time in the update is still named; now stamps the reports.
func (f *typingFeed) register(d tg.UpdateDispatcher, n *names, now func() time.Time) {
	if f == nil {
		return
	}
	handle := func(e tg.Entities, u tg.UpdateClass) {
		n.learnUsers(e.Users)
		if ev, ok := typingEvent(u, n.get, now()); ok {
			f.emit(ev)
		}
	}
	d.OnUserTyping(func(_ context.Context, e tg.Entities, u *tg.UpdateUserTyping) error {
		handle(e, u)
		return nil
	})
	d.OnChatUserTyping(func(_ context.Context, e tg.Entities, u *tg.UpdateChatUserTyping) error {
		handle(e, u)
		return nil
	})
	d.OnChannelUserTyping(func(_ context.Context, e tg.Entities, u *tg.UpdateChannelUserTyping) error {
		handle(e, u)
		return nil
	})
}

// typingEvent maps one typing update to a report. ok is false for updates
// that say nothing about a member composing a message (an import, speaking
// in a group call, an emoji interaction) and for update types this does
// not know.
func typingEvent(u tg.UpdateClass, who func(int64) string, at time.Time) (domain.Typing, bool) {
	var ev domain.Typing
	var action tg.SendMessageActionClass
	switch u := u.(type) {
	case *tg.UpdateUserTyping:
		ev.ChatID = chatID(entity{id: u.UserID})
		ev.Who = who(u.UserID)
		action = u.Action
	case *tg.UpdateChatUserTyping:
		ev.ChatID = chatID(entity{id: -u.ChatID})
		ev.Who = typistName(u.FromID, who)
		action = u.Action
	case *tg.UpdateChannelUserTyping:
		ev.ChatID = chatID(entity{id: channelBase - u.ChannelID})
		ev.Who = typistName(u.FromID, who)
		action = u.Action
	default:
		return domain.Typing{}, false
	}
	kind, ok := typingKind(action)
	if !ok {
		return domain.Typing{}, false
	}
	ev.Kind, ev.At = kind, at
	return ev, true
}

// typistName names the member a group update is about: a user by the
// registry, anything else (a channel posting as itself) unnamed.
func typistName(p tg.PeerClass, who func(int64) string) string {
	if u, ok := p.(*tg.PeerUser); ok {
		return who(u.UserID)
	}
	return ""
}

// typingKind folds the service's actions into the kinds lazygram shows.
// A live draft (a bot or a user streaming a reply) is typing and its end
// a stop. ok is false for actions that are not a member composing.
func typingKind(a tg.SendMessageActionClass) (domain.TypingKind, bool) {
	switch a.(type) {
	case *tg.SendMessageTypingAction, *tg.SendMessageTextDraftAction, *tg.SendMessageRichMessageDraftAction:
		return domain.TypingText, true
	case *tg.SendMessageCancelAction, *tg.SendMessageStopDraftAction:
		return domain.TypingStop, true
	case *tg.SendMessageRecordAudioAction, *tg.SendMessageRecordVideoAction, *tg.SendMessageRecordRoundAction:
		return domain.TypingRecord, true
	case *tg.SendMessageUploadAudioAction, *tg.SendMessageUploadVideoAction, *tg.SendMessageUploadPhotoAction,
		*tg.SendMessageUploadDocumentAction, *tg.SendMessageUploadRoundAction:
		return domain.TypingUpload, true
	case *tg.SendMessageChooseStickerAction, *tg.SendMessageChooseContactAction, *tg.SendMessageGeoLocationAction,
		*tg.SendMessageGamePlayAction:
		return domain.TypingChoose, true
	}
	return 0, false
}
