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

	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"

	"lazygram/internal/domain"
)

const (
	maxDialogs   = 100 // most recent dialogs listed
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
}

// service implements Client on top of api. It is read-only. Every History
// call costs one getDialogs request plus one history page per chat whose top
// message changed, so polling stays cheap. The first call only records where
// each chat stands: like an IRC client, lazygram shows what arrives after it
// connected, and older messages come through Recent on demand.
type service struct {
	api api

	known map[string]entity // chat id -> entity, filled by Dialogs
	top   map[string]int    // chat id -> top message id already fetched
}

func newService(a api) *service {
	return &service{api: a, known: map[string]entity{}, top: map[string]int{}}
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

// Recent returns the last limit messages of a chat, oldest first.
func (s *service) Recent(ctx context.Context, chatID string, limit int) ([]domain.Message, error) {
	ent, ok := s.known[chatID]
	if !ok {
		return nil, fmt.Errorf("telegram: unknown chat %s", chatID)
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
	ent, ok := s.known[chatID]
	if !ok {
		return fmt.Errorf("telegram: unknown chat %s", chatID)
	}
	if strings.TrimSpace(text) == "" {
		return errors.New("telegram: empty message")
	}
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return err
	}
	_, err := s.api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
		Peer: ent.peer, Message: text, RandomID: int64(binary.LittleEndian.Uint64(b[:])),
	})
	if err != nil {
		return fmt.Errorf("telegram: send: %w", err)
	}
	return nil
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
	if text == "" && m.Media != nil {
		text = "[media]"
	}
	return domain.Message{
		ID:     strconv.Itoa(m.ID),
		ChatID: chatID,
		Sender: sender(chat, m, ents),
		Text:   text,
		Time:   time.Unix(int64(m.Date), 0).UTC(),
	}
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
