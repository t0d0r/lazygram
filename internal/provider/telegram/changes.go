package telegram

import (
	"context"
	"strconv"
	"sync"

	"github.com/gotd/td/tg"

	"lazygram/internal/domain"
)

// Edits and deletions arrive as updates, like typing reports: the polled
// history only sees new messages, so without them a message edited or
// deleted on another device, or by its author, would stay as it was in
// lazygram. Four updates are used: updateEditMessage and
// updateEditChannelMessage (a message's text, media or buttons changed),
// updateDeleteMessages (private chats and basic groups, whose message ids
// are shared across the account, so it names no chat) and
// updateDeleteChannelMessages (a supergroup or channel).

// changeBuffer is how many changes can wait for the TUI before new ones
// are dropped; a dropped one is healed by the next fetch of the chat.
const changeBuffer = 256

// changeFeed carries changes from the update handlers to whoever reads
// the channel, with the same rules as typingFeed: emit never blocks, the
// feed is closed once Run has returned, a nil feed carries nothing.
type changeFeed struct {
	ch   chan domain.Change
	once sync.Once
}

func newChangeFeed() *changeFeed { return &changeFeed{ch: make(chan domain.Change, changeBuffer)} }

func (f *changeFeed) feed() <-chan domain.Change {
	if f == nil {
		return nil
	}
	return f.ch
}

func (f *changeFeed) emit(c domain.Change) bool {
	if f == nil {
		return false
	}
	select {
	case f.ch <- c:
		return true
	default:
		return false
	}
}

func (f *changeFeed) closeFeed() {
	if f == nil {
		return
	}
	f.once.Do(func() { close(f.ch) })
}

// register hooks the four edit and deletion updates up to the feed.
func (f *changeFeed) register(d tg.UpdateDispatcher) {
	if f == nil {
		return
	}
	handle := func(e tg.Entities, u tg.UpdateClass) {
		if c, ok := changeEvent(u, e); ok {
			f.emit(c)
		}
	}
	d.OnEditMessage(func(_ context.Context, e tg.Entities, u *tg.UpdateEditMessage) error {
		handle(e, u)
		return nil
	})
	d.OnEditChannelMessage(func(_ context.Context, e tg.Entities, u *tg.UpdateEditChannelMessage) error {
		handle(e, u)
		return nil
	})
	d.OnDeleteMessages(func(_ context.Context, e tg.Entities, u *tg.UpdateDeleteMessages) error {
		handle(e, u)
		return nil
	})
	d.OnDeleteChannelMessages(func(_ context.Context, e tg.Entities, u *tg.UpdateDeleteChannelMessages) error {
		handle(e, u)
		return nil
	})
}

// changeEvent maps one update to a change; ok is false for anything else
// and for an edit of something that is not a message (a service message).
func changeEvent(u tg.UpdateClass, e tg.Entities) (domain.Change, bool) {
	switch u := u.(type) {
	case *tg.UpdateEditMessage:
		return editEvent(u.Message, e)
	case *tg.UpdateEditChannelMessage:
		return editEvent(u.Message, e)
	case *tg.UpdateDeleteMessages:
		return domain.Change{Kind: domain.Deleted, IDs: ids(u.Messages)}, len(u.Messages) > 0
	case *tg.UpdateDeleteChannelMessages:
		return domain.Change{Kind: domain.Deleted, ChatID: chatID(entity{id: channelBase - u.ChannelID}), IDs: ids(u.Messages)},
			len(u.Messages) > 0
	}
	return domain.Change{}, false
}

// editEvent converts an edited message. Only its text, media and buttons
// are used: the TUI keeps who sent it from the copy it has.
func editEvent(mc tg.MessageClass, e tg.Entities) (domain.Change, bool) {
	m, ok := mc.(*tg.Message)
	if !ok {
		return domain.Change{}, false
	}
	var id int64
	switch p := m.PeerID.(type) {
	case *tg.PeerUser:
		id = p.UserID
	case *tg.PeerChat:
		id = -p.ChatID
	case *tg.PeerChannel:
		id = channelBase - p.ChannelID
	default:
		return domain.Change{}, false
	}
	users := make([]tg.UserClass, 0, len(e.Users))
	for _, u := range e.Users {
		users = append(users, u)
	}
	chat := entity{id: id}
	return domain.Change{Kind: domain.Edited, ChatID: chatID(chat), Msg: convert(chatID(chat), chat, m, collect(users, nil))}, true
}

func ids(ns []int) []string {
	out := make([]string, len(ns))
	for i, n := range ns {
		out[i] = strconv.Itoa(n)
	}
	return out
}
