package telegram

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"

	"lazygram/internal/domain"
	"lazygram/internal/provider"
)

// Reactions come with every message (message.reactions, in history and
// edits). Changes to them arrive two ways: updateMessageReactions is
// pushed for the account's own messages only, and for everyone else's
// messages Telegram asks apps to poll the ones on screen every 15 to 30
// seconds with messages.getMessagesReactions, whose answer is the same
// update, one per message that has reactions.

const (
	// paidEmoji shows a paid reaction (a Telegram Star): the star emoji.
	paidEmoji = "\u2b50"
	// maxPollIDs caps the messages one poll asks about.
	maxPollIDs = 100
)

// reactions converts a message's reactions, in the service's order: an
// emoji reaction by its emoticon, all custom-emoji reactions summed into
// one domain.CustomReaction (their emoji is a document lazygram does not
// fetch), a paid one as the star; empty reactions and zero counts are
// dropped. MineCount is how many of the results summed into an entry have
// chosen_order set, but for a paid reaction, whose count is of Stars, the
// Stars the account sent (myStars); Mine is MineCount above zero.
// mineUnknown is min: the object is the same for every user and lacks the
// account's own reactions.
func reactions(r tg.MessageReactions) (out []domain.Reaction, mineUnknown bool) {
	at := map[string]int{}
	for _, rc := range r.Results {
		emoji, ok := reactionEmoji(rc.Reaction)
		if !ok || rc.Count <= 0 {
			continue
		}
		mine := 0
		if _, chosen := rc.GetChosenOrder(); chosen {
			mine = 1
		}
		if _, paid := rc.Reaction.(*tg.ReactionPaid); paid {
			mine = max(mine, myStars(r.TopReactors))
		}
		if i, seen := at[emoji]; seen {
			out[i].Count += rc.Count
			out[i].MineCount += mine
			out[i].Mine = out[i].MineCount > 0
			continue
		}
		at[emoji] = len(out)
		out = append(out, domain.Reaction{Emoji: emoji, Count: rc.Count, Mine: mine > 0, MineCount: mine})
	}
	return out, r.Min
}

// myStars is how many Stars the account sent as paid reactions: the count
// of its own entry (my) among the top reactors, which lists it whether it
// is among the top or not; 0 when it is not listed.
func myStars(top []tg.MessageReactor) int {
	for _, x := range top {
		if x.My {
			return x.Count
		}
	}
	return 0
}

// reactionEmoji is how a reaction is shown; ok is false for reactionEmpty
// and kinds this does not know.
func reactionEmoji(r tg.ReactionClass) (string, bool) {
	switch r := r.(type) {
	case *tg.ReactionEmoji:
		return r.Emoticon, r.Emoticon != ""
	case *tg.ReactionCustomEmoji:
		return domain.CustomReaction, true
	case *tg.ReactionPaid:
		return paidEmoji, true
	}
	return "", false
}

// reactors names the recent reactions of r: who sent them, named as
// message senders are ("me" for the account), with the emoji and when.
// who names users the update does not carry (the names registry).
func reactors(r tg.MessageReactions, e tg.Entities, who func(int64) string) []domain.Reactor {
	var out []domain.Reactor
	for _, pr := range r.RecentReactions {
		emoji, ok := reactionEmoji(pr.Reaction)
		if !ok {
			continue
		}
		name, id := reactorName(pr.PeerID, e, who)
		if pr.My { // sent by the account, perhaps as one of its channels
			name, id = "me", ""
		}
		var at time.Time
		if pr.Date > 0 {
			at = time.Unix(int64(pr.Date), 0).UTC()
		}
		out = append(out, domain.Reactor{Who: name, ID: id, Emoji: emoji, At: at})
	}
	return out
}

// reactorName names the peer that reacted and gives its chat id: a user
// by name ("user" when never seen), a channel by its title.
func reactorName(p tg.PeerClass, e tg.Entities, who func(int64) string) (name, id string) {
	n, ok := peerID(p)
	if !ok {
		return "", ""
	}
	id = chatID(entity{id: n})
	switch p := p.(type) {
	case *tg.PeerUser:
		if u, ok := e.Users[p.UserID]; ok {
			if u.Self {
				return "me", ""
			}
			return userName(u), id
		}
		if who != nil {
			if name := who(p.UserID); name != "" {
				return name, id
			}
		}
		return "user", id
	case *tg.PeerChannel:
		if c, ok := e.Channels[p.ChannelID]; ok && c.Title != "" {
			return c.Title, id
		}
		return "channel", id
	case *tg.PeerChat:
		if c, ok := e.Chats[p.ChatID]; ok && c.Title != "" {
			return c.Title, id
		}
	}
	return "chat", id
}

// reactedEvent converts updateMessageReactions: the message's reactions
// as they are now and who reacted lately. own says the update was pushed,
// which Telegram does only for the account's own messages.
func reactedEvent(u *tg.UpdateMessageReactions, e tg.Entities, who func(int64) string, own bool) (domain.Change, bool) {
	n, ok := peerID(u.Peer)
	if !ok || u.MsgID <= 0 {
		return domain.Change{}, false
	}
	chat := chatID(entity{id: n})
	rs, mineUnknown := reactions(u.Reactions)
	return domain.Change{
		Kind: domain.Reacted, ChatID: chat,
		Msg:      domain.Message{ID: strconv.Itoa(u.MsgID), ChatID: chat, Reactions: rs, MineUnknown: mineUnknown},
		Reactors: reactors(u.Reactions, e, who),
		Own:      own,
	}, true
}

// PollReactions asks Telegram for the reactions of messages of one chat,
// messages.getMessagesReactions, and returns them as Reacted changes. Ids
// that are not message numbers are left out, and one call asks about the
// newest maxPollIDs at most. As in Telegram's own client library, an
// asked-for message the answer leaves out has no reactions. A chat not
// among the known dialogs costs no request. CHANNEL_PRIVATE,
// CHANNEL_INVALID and MSG_ID_INVALID make the chat provider.ErrNotPollable
// for the session. A FLOOD_WAIT is not slept like other calls' (a poll is
// never worth holding the client for): until it has passed no poll is
// sent and the answer is a *provider.WaitError.
func (s *service) PollReactions(ctx context.Context, chat string, msgIDs []string) ([]domain.Change, error) {
	if s.unpollable[chat] {
		return nil, provider.ErrNotPollable
	}
	if left := s.pollUntil.Sub(s.now()); left > 0 {
		return nil, &provider.WaitError{Wait: left}
	}
	ent, ok := s.known[chat]
	if !ok {
		return nil, fmt.Errorf("telegram: unknown chat %s", chat)
	}
	var nums []int
	for _, id := range msgIDs {
		if n, err := strconv.Atoi(id); err == nil && n > 0 {
			nums = append(nums, n)
		}
	}
	slices.Sort(nums)
	nums = slices.Compact(nums)
	if len(nums) > maxPollIDs {
		nums = nums[len(nums)-maxPollIDs:]
	}
	if len(nums) == 0 {
		return nil, nil
	}
	res, err := s.api.MessagesGetMessagesReactions(ctx, &tg.MessagesGetMessagesReactionsRequest{Peer: ent.peer, ID: nums})
	if d, flood := tgerr.AsFloodWait(err); flood {
		s.pollUntil = s.now().Add(d)
		return nil, &provider.WaitError{Wait: d}
	}
	switch {
	case tgerr.Is(err, "CHANNEL_PRIVATE", "CHANNEL_INVALID", "MSG_ID_INVALID"):
		s.unpollable[chat] = true
		return nil, provider.ErrNotPollable
	case err != nil:
		return nil, fmt.Errorf("telegram: reactions: %w", err)
	}
	updates, e := updatesParts(res)
	s.names.learnUsers(e.Users)
	var out []domain.Change
	answered := map[string]bool{}
	for _, uc := range updates {
		u, ok := uc.(*tg.UpdateMessageReactions)
		if !ok {
			continue
		}
		if c, ok := reactedEvent(u, e, s.names.get, false); ok {
			out = append(out, c)
			if c.ChatID == chat {
				answered[c.Msg.ID] = true
			}
		}
	}
	if _, full := res.(*tg.Updates); full {
		for _, n := range nums {
			if id := strconv.Itoa(n); !answered[id] {
				out = append(out, domain.Change{Kind: domain.Reacted, ChatID: chat, Msg: domain.Message{ID: id, ChatID: chat}})
			}
		}
	}
	return out, nil
}

// updatesParts takes the updates out of a response, with the users and
// chats it carries, the way the update dispatcher does.
func updatesParts(res tg.UpdatesClass) ([]tg.UpdateClass, tg.Entities) {
	e := tg.Entities{Users: map[int64]*tg.User{}, Chats: map[int64]*tg.Chat{}, Channels: map[int64]*tg.Channel{}}
	switch r := res.(type) {
	case *tg.Updates:
		e.Users, e.Chats, e.Channels = r.MapUsers().NotEmptyToMap(), r.MapChats().ChatToMap(), r.MapChats().ChannelToMap()
		return r.Updates, e
	case *tg.UpdatesCombined:
		e.Users, e.Chats, e.Channels = r.MapUsers().NotEmptyToMap(), r.MapChats().ChatToMap(), r.MapChats().ChannelToMap()
		return r.Updates, e
	case *tg.UpdateShort:
		return []tg.UpdateClass{r.Update}, e
	}
	return nil, e
}

// peerID is a peer's Bot-API style id: users positive, groups -id,
// channels -100<id>.
func peerID(p tg.PeerClass) (int64, bool) {
	switch p := p.(type) {
	case *tg.PeerUser:
		return p.UserID, true
	case *tg.PeerChat:
		return -p.ChatID, true
	case *tg.PeerChannel:
		return channelBase - p.ChannelID, true
	}
	return 0, false
}
