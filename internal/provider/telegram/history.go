package telegram

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"

	"lazygram/internal/domain"
)

const (
	editGrace    = 800 * time.Millisecond // time a bot gets to edit its message after a press
	maxDialogs   = 100                    // most recent dialogs listed
	pageSize     = 100
	maxPerChat   = 300 // messages fetched per chat per Sync call
	maxRecent    = 100 // cap for Recent
	maxFloodWait = 60 * time.Second
)

// api is the subset of *tg.Client lazygram uses; tests substitute a fake.
type api interface {
	MessagesGetDialogs(ctx context.Context, req *tg.MessagesGetDialogsRequest) (tg.MessagesDialogsClass, error)
	MessagesGetHistory(ctx context.Context, req *tg.MessagesGetHistoryRequest) (tg.MessagesMessagesClass, error)
	MessagesSendMessage(ctx context.Context, req *tg.MessagesSendMessageRequest) (tg.UpdatesClass, error)
	MessagesGetBotCallbackAnswer(ctx context.Context, req *tg.MessagesGetBotCallbackAnswerRequest) (*tg.MessagesBotCallbackAnswer, error)
	MessagesGetMessages(ctx context.Context, id []tg.InputMessageClass) (tg.MessagesMessagesClass, error)
	ChannelsGetMessages(ctx context.Context, req *tg.ChannelsGetMessagesRequest) (tg.MessagesMessagesClass, error)
	MessagesEditMessage(ctx context.Context, req *tg.MessagesEditMessageRequest) (tg.UpdatesClass, error)
	MessagesDeleteMessages(ctx context.Context, req *tg.MessagesDeleteMessagesRequest) (*tg.MessagesAffectedMessages, error)
	ChannelsDeleteMessages(ctx context.Context, req *tg.ChannelsDeleteMessagesRequest) (*tg.MessagesAffectedMessages, error)
}

// service implements Client on top of api. It is read-only. Every History
// call costs one getDialogs request plus one history page per chat whose top
// message changed, so polling stays cheap. The first call only records where
// each chat stands: like an IRC client, lazygram shows what arrives after it
// connected, and older messages come through Recent on demand.
type service struct {
	api api

	files downloader.Client                          // nil: attachments cannot be fetched
	wait  func(context.Context, time.Duration) error // sleeps; tests replace it

	known map[string]entity // chat id -> entity, filled by Dialogs
	top   map[string]int    // chat id -> top message id already fetched
}

func newService(a api) *service {
	return &service{api: a, known: map[string]entity{}, top: map[string]int{}, wait: sleepCtx}
}

// dialog is one row of getDialogs.
type dialog struct {
	chat domain.Chat
	ent  entity
	top  int
}

func (s *service) dialogs(ctx context.Context) ([]dialog, error) {
	res, err := s.api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
		OffsetPeer: &tg.InputPeerEmpty{}, Limit: maxDialogs,
	})
	if err != nil {
		return nil, fmt.Errorf("telegram: get dialogs: %w", err)
	}
	var rows []tg.DialogClass
	var ents entities
	switch d := res.(type) {
	case *tg.MessagesDialogs:
		rows, ents = d.Dialogs, collect(d.Users, d.Chats)
	case *tg.MessagesDialogsSlice:
		rows, ents = d.Dialogs, collect(d.Users, d.Chats)
	default:
		return nil, fmt.Errorf("telegram: unexpected dialogs response %T", res)
	}
	var out []dialog
	for _, dc := range rows {
		d, ok := dc.(*tg.Dialog)
		if !ok {
			continue // folders
		}
		ent, ok := ents.byPeer(d.Peer)
		if !ok {
			continue // forbidden or unknown peer
		}
		out = append(out, dialog{
			chat: domain.Chat{ID: chatID(ent), Service: "telegram", Title: ent.title,
				Kind: ent.chatKind(), Unread: d.UnreadCount, Deleted: ent.deleted},
			ent: ent, top: d.TopMessage,
		})
	}
	// Usernames are unique on Telegram and claim their names first, so a
	// chat whose title merely slugs to someone's username can never take it.
	taken := map[string]bool{}
	for pass := 0; pass < 2; pass++ {
		for i := range out {
			if (out[i].ent.username != "") == (pass == 0) {
				out[i].chat.Alias = aliasFor(out[i].ent, taken)
			}
		}
	}
	return out, nil
}

// Dialogs lists the most recent dialogs as chats.
func (s *service) Dialogs(ctx context.Context) ([]domain.Chat, error) {
	rows, err := s.dialogs(ctx)
	if err != nil {
		return nil, err
	}
	chats := make([]domain.Chat, 0, len(rows))
	for _, d := range rows {
		s.known[d.chat.ID] = d.ent
		chats = append(chats, d.chat)
	}
	return chats, nil
}

// History fetches messages that arrived since the last call. The first call
// records each chat's top message and returns nothing. Chats whose top
// message is unchanged, deleted accounts and empty chats cost no request.
// since is ignored: the cursor is the message id. On error it returns the
// messages fetched so far together with the error; cursors advanced only
// for the chats whose messages are returned, so callers must store a
// partial result.
func (s *service) History(ctx context.Context, _ time.Time) ([]domain.Message, error) {
	rows, err := s.dialogs(ctx)
	if err != nil {
		return nil, err
	}
	first := len(s.top) == 0
	var out []domain.Message
	for _, d := range rows {
		s.known[d.chat.ID] = d.ent
		seen, tracked := s.top[d.chat.ID]
		if first || !tracked {
			s.top[d.chat.ID] = d.top // a chat seen for the first time: start from here
			continue
		}
		if d.top == 0 || d.top <= seen || d.ent.deleted {
			continue
		}
		msgs, err := s.chatHistory(ctx, d.chat.ID, d.ent, seen, maxPerChat)
		if err != nil {
			return out, err
		}
		out = append(out, msgs...)
		s.top[d.chat.ID] = d.top
	}
	return out, nil
}

// entity finds a chat's entity, loading the dialogs first when the chat
// is not known yet (a window restored at start-up asks before the first
// dialog list arrived).
func (s *service) entity(ctx context.Context, chatID string) (entity, error) {
	if ent, ok := s.known[chatID]; ok {
		return ent, nil
	}
	if _, err := s.Dialogs(ctx); err != nil {
		return entity{}, err
	}
	if ent, ok := s.known[chatID]; ok {
		return ent, nil
	}
	return entity{}, fmt.Errorf("telegram: unknown chat %s (not among the recent dialogs)", chatID)
}

// Recent returns the last limit messages of a chat, oldest first.
func (s *service) Recent(ctx context.Context, chatID string, limit int) ([]domain.Message, error) {
	ent, err := s.entity(ctx, chatID)
	if err != nil {
		return nil, err
	}
	limit = min(max(limit, 1), maxRecent)
	msgs, err := s.chatHistory(ctx, chatID, ent, 0, limit)
	if err != nil {
		return nil, err
	}
	return domain.Merge(msgs), nil
}

// Send posts a text message. The sent message is picked up by the next
// History call because the chat's top message moves.
func (s *service) Send(ctx context.Context, chatID, text string) error {
	ent, err := s.entity(ctx, chatID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(text) == "" {
		return errors.New("telegram: empty message")
	}
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return err
	}
	_, err = s.api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
		Peer: ent.peer, Message: text, RandomID: int64(binary.LittleEndian.Uint64(b[:])),
	})
	if err != nil {
		return fmt.Errorf("telegram: send: %w", err)
	}
	return nil
}

// Edit replaces the text of one of this account's messages.
func (s *service) Edit(ctx context.Context, chatID, msgID, text string) error {
	ent, err := s.entity(ctx, chatID)
	if err != nil {
		return err
	}
	id, err := strconv.Atoi(msgID)
	if err != nil {
		return fmt.Errorf("telegram: bad message id %q", msgID)
	}
	if strings.TrimSpace(text) == "" {
		return errors.New("telegram: empty message")
	}
	req := &tg.MessagesEditMessageRequest{Peer: ent.peer, ID: id}
	req.SetMessage(text)
	if _, err := s.api.MessagesEditMessage(ctx, req); err != nil {
		return fmt.Errorf("telegram: edit: %w", err)
	}
	return nil
}

// Delete removes one of this account's messages for everyone.
func (s *service) Delete(ctx context.Context, chatID, msgID string) error {
	ent, err := s.entity(ctx, chatID)
	if err != nil {
		return err
	}
	id, err := strconv.Atoi(msgID)
	if err != nil {
		return fmt.Errorf("telegram: bad message id %q", msgID)
	}
	if ent.channel != nil {
		_, err = s.api.ChannelsDeleteMessages(ctx, &tg.ChannelsDeleteMessagesRequest{Channel: ent.channel, ID: []int{id}})
	} else {
		_, err = s.api.MessagesDeleteMessages(ctx, &tg.MessagesDeleteMessagesRequest{Revoke: true, ID: []int{id}})
	}
	if err != nil {
		return fmt.Errorf("telegram: delete: %w", err)
	}
	return nil
}

// Press presses a callback button: messages.getBotCallbackAnswer, then the
// message is fetched again because bots usually edit it in response.
func (s *service) Press(ctx context.Context, chatID, msgID string, b domain.Button) (domain.PressAnswer, error) {
	ent, err := s.entity(ctx, chatID)
	if err != nil {
		return domain.PressAnswer{}, err
	}
	id, err := strconv.Atoi(msgID)
	if err != nil {
		return domain.PressAnswer{}, fmt.Errorf("telegram: bad message id %q", msgID)
	}
	if b.Kind != domain.ButtonCallback {
		return domain.PressAnswer{}, errors.New("telegram: only callback buttons can be pressed")
	}
	req := &tg.MessagesGetBotCallbackAnswerRequest{Peer: ent.peer, MsgID: id}
	req.SetData(b.Data)
	res, err := s.api.MessagesGetBotCallbackAnswer(ctx, req)
	var a domain.PressAnswer
	switch {
	case err == nil:
		if res != nil {
			a.Text, _ = res.GetMessage()
			a.URL, _ = res.GetURL()
		}
	case tgerr.Is(err, "BOT_RESPONSE_TIMEOUT"):
		// Many bots never answer the callback query but still act on it.
		a.Text = "(the bot gave no answer in time)"
	default:
		return domain.PressAnswer{}, fmt.Errorf("telegram: press: %w", err)
	}
	// Bots edit their message right after answering; give them a moment.
	if err := s.wait(ctx, editGrace); err != nil {
		return a, err
	}
	a.Updated, err = s.message(ctx, chatID, ent, id)
	if err != nil {
		return a, fmt.Errorf("telegram: pressed, but could not refetch the message: %w", err)
	}
	return a, nil
}

// message fetches one message by id as it is now.
func (s *service) message(ctx context.Context, chatID string, ent entity, id int) (*domain.Message, error) {
	m, ents, err := s.rawMessage(ctx, ent, id)
	if err != nil {
		return nil, err
	}
	out := convert(chatID, ent, m, ents)
	return &out, nil
}

// rawMessage fetches one message by id in the service's own form, with the
// users it mentions. It is also how a fresh file reference is obtained.
func (s *service) rawMessage(ctx context.Context, ent entity, id int) (*tg.Message, entities, error) {
	ids := []tg.InputMessageClass{&tg.InputMessageID{ID: id}}
	var res tg.MessagesMessagesClass
	var err error
	if ent.channel != nil {
		res, err = s.api.ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{Channel: ent.channel, ID: ids})
	} else {
		res, err = s.api.MessagesGetMessages(ctx, ids)
	}
	if err != nil {
		return nil, entities{}, err
	}
	raw, users := historyParts(res)
	ents := collect(users, nil)
	for _, mc := range raw {
		if m, ok := mc.(*tg.Message); ok && m.ID == id {
			return m, ents, nil
		}
	}
	return nil, ents, errors.New("message not found")
}

// buttons flattens a message's inline keyboard (or reply keyboard) into the
// domain form, row by row, left to right. Callback buttons that require the
// account password are reported as unsupported.
func buttons(markup tg.ReplyMarkupClass) []domain.Button {
	var out []domain.Button
	switch mk := markup.(type) {
	case *tg.ReplyInlineMarkup:
		for _, row := range mk.Rows {
			for _, b := range row.Buttons {
				switch t := b.Type.(type) {
				case *tg.InlineButtonTypeCallback:
					if t.RequiresPassword {
						out = append(out, domain.Button{Label: b.Text, Kind: domain.ButtonOther})
						continue
					}
					out = append(out, domain.Button{Label: b.Text, Kind: domain.ButtonCallback, Data: t.Data})
				case *tg.InlineButtonTypeURL:
					out = append(out, domain.Button{Label: b.Text, Kind: domain.ButtonURL, URL: t.URL})
				default:
					out = append(out, domain.Button{Label: b.Text, Kind: domain.ButtonOther})
				}
			}
		}
	case *tg.ReplyKeyboardMarkup:
		for _, row := range mk.Rows {
			for _, b := range row.Buttons {
				switch b.Type.(type) {
				case *tg.ButtonTypeDefault, nil:
					out = append(out, domain.Button{Label: b.Text, Kind: domain.ButtonText})
				default:
					out = append(out, domain.Button{Label: b.Text, Kind: domain.ButtonOther})
				}
			}
		}
	}
	return out
}

// chatHistory pages backwards from the newest message and stops at a message
// already fetched (id <= stopID) or at limit. Messages come newest first.
func (s *service) chatHistory(ctx context.Context, id string, ent entity, stopID int, limit int) ([]domain.Message, error) {
	var out []domain.Message
	offsetID := 0
	for len(out) < limit {
		res, err := s.api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
			Peer: ent.peer, OffsetID: offsetID, Limit: min(pageSize, limit-len(out)),
		})
		if err != nil {
			return out, fmt.Errorf("telegram: get history: %w", err)
		}
		raw, users := historyParts(res)
		if len(raw) == 0 {
			break
		}
		ents := collect(users, nil)
		done := false
		lowest := raw[0].GetID()
		for _, mc := range raw {
			lowest = min(lowest, mc.GetID())
			if mc.GetID() <= stopID {
				done = true
				break
			}
			m, ok := mc.(*tg.Message)
			if !ok {
				continue // service messages and empty placeholders
			}
			out = append(out, convert(id, ent, m, ents))
		}
		if done || len(raw) < pageSize || lowest == offsetID {
			break
		}
		offsetID = lowest
	}
	return out, nil
}

func historyParts(res tg.MessagesMessagesClass) ([]tg.MessageClass, []tg.UserClass) {
	switch r := res.(type) {
	case *tg.MessagesMessages:
		return r.Messages, r.Users
	case *tg.MessagesMessagesSlice:
		return r.Messages, r.Users
	case *tg.MessagesChannelMessages:
		return r.Messages, r.Users
	}
	return nil, nil
}

// convert maps a Telegram message to the domain model.
func convert(chatID string, chat entity, m *tg.Message, ents entities) domain.Message {
	text := m.Message
	media := mediaOf(m)
	if text == "" && m.Media != nil && media == nil {
		text = "[media]" // something lazygram cannot fetch: a poll, a location, ...
	}
	out := domain.Message{
		ID:     strconv.Itoa(m.ID),
		ChatID: chatID,
		Sender: sender(chat, m, ents),
		Text:   text,
		Time:   time.Unix(int64(m.Date), 0).UTC(),
	}
	if mk, ok := m.GetReplyMarkup(); ok {
		out.Buttons = buttons(mk)
	}
	out.Media = media
	return out
}

func sender(chat entity, m *tg.Message, ents entities) string {
	if m.Out {
		return "me"
	}
	if p, ok := m.FromID.(*tg.PeerUser); ok {
		if u, ok := ents.users[p.UserID]; ok {
			return userName(u)
		}
		return "user"
	}
	switch {
	case m.PostAuthor != "":
		return m.PostAuthor
	case chat.kind == kindUser:
		return chat.title
	}
	return "channel"
}

// floodAPI retries a request once after a short FLOOD_WAIT.
type floodAPI struct {
	api   api
	sleep func(context.Context, time.Duration) error
}

func (f floodAPI) MessagesGetDialogs(ctx context.Context, req *tg.MessagesGetDialogsRequest) (tg.MessagesDialogsClass, error) {
	return retryFlood(ctx, f.sleep, func() (tg.MessagesDialogsClass, error) { return f.api.MessagesGetDialogs(ctx, req) })
}

func (f floodAPI) MessagesGetHistory(ctx context.Context, req *tg.MessagesGetHistoryRequest) (tg.MessagesMessagesClass, error) {
	return retryFlood(ctx, f.sleep, func() (tg.MessagesMessagesClass, error) { return f.api.MessagesGetHistory(ctx, req) })
}

func (f floodAPI) MessagesSendMessage(ctx context.Context, req *tg.MessagesSendMessageRequest) (tg.UpdatesClass, error) {
	return retryFlood(ctx, f.sleep, func() (tg.UpdatesClass, error) { return f.api.MessagesSendMessage(ctx, req) })
}

func (f floodAPI) MessagesGetBotCallbackAnswer(ctx context.Context, req *tg.MessagesGetBotCallbackAnswerRequest) (*tg.MessagesBotCallbackAnswer, error) {
	return retryFlood(ctx, f.sleep, func() (*tg.MessagesBotCallbackAnswer, error) { return f.api.MessagesGetBotCallbackAnswer(ctx, req) })
}

func (f floodAPI) MessagesGetMessages(ctx context.Context, id []tg.InputMessageClass) (tg.MessagesMessagesClass, error) {
	return retryFlood(ctx, f.sleep, func() (tg.MessagesMessagesClass, error) { return f.api.MessagesGetMessages(ctx, id) })
}

func (f floodAPI) ChannelsGetMessages(ctx context.Context, req *tg.ChannelsGetMessagesRequest) (tg.MessagesMessagesClass, error) {
	return retryFlood(ctx, f.sleep, func() (tg.MessagesMessagesClass, error) { return f.api.ChannelsGetMessages(ctx, req) })
}

func (f floodAPI) MessagesEditMessage(ctx context.Context, req *tg.MessagesEditMessageRequest) (tg.UpdatesClass, error) {
	return retryFlood(ctx, f.sleep, func() (tg.UpdatesClass, error) { return f.api.MessagesEditMessage(ctx, req) })
}

func (f floodAPI) MessagesDeleteMessages(ctx context.Context, req *tg.MessagesDeleteMessagesRequest) (*tg.MessagesAffectedMessages, error) {
	return retryFlood(ctx, f.sleep, func() (*tg.MessagesAffectedMessages, error) { return f.api.MessagesDeleteMessages(ctx, req) })
}

func (f floodAPI) ChannelsDeleteMessages(ctx context.Context, req *tg.ChannelsDeleteMessagesRequest) (*tg.MessagesAffectedMessages, error) {
	return retryFlood(ctx, f.sleep, func() (*tg.MessagesAffectedMessages, error) { return f.api.ChannelsDeleteMessages(ctx, req) })
}

func retryFlood[T any](ctx context.Context, sleep func(context.Context, time.Duration) error, call func() (T, error)) (T, error) {
	res, err := call()
	d, ok := tgerr.AsFloodWait(err)
	if !ok || d > maxFloodWait {
		return res, err
	}
	if err := sleep(ctx, d); err != nil {
		var zero T
		return zero, err
	}
	return call()
}
