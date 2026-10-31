package telegram

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/gotd/td/tg"

	"lazygram/internal/config"
	"lazygram/internal/domain"
)

const (
	pageSize   = 100
	maxPerChat = 300 // messages fetched per chat per Sync call
)

// api is the subset of *tg.Client lazygram uses; tests substitute a fake.
type api interface {
	MessagesGetDialogs(ctx context.Context, req *tg.MessagesGetDialogsRequest) (tg.MessagesDialogsClass, error)
	MessagesGetHistory(ctx context.Context, req *tg.MessagesGetHistoryRequest) (tg.MessagesMessagesClass, error)
}

// service implements Client on top of api. It is read-only.
type service struct {
	api api

	known map[string]entity // chat id -> entity, filled by Dialogs
}

func (s *service) Dialogs(ctx context.Context, refs []config.ChatRef) ([]domain.Chat, error) {
	if len(refs) == 0 {
		return nil, fmt.Errorf("telegram: no chats configured, set %s (alias=ref,...)", config.EnvTGChats)
	}
	res, err := s.api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
		OffsetPeer: &tg.InputPeerEmpty{}, Limit: pageSize,
	})
	if err != nil {
		return nil, fmt.Errorf("telegram: get dialogs: %w", err)
	}
	var ents entities
	switch d := res.(type) {
	case *tg.MessagesDialogs:
		ents = collect(d.Users, d.Chats)
	case *tg.MessagesDialogsSlice:
		ents = collect(d.Users, d.Chats)
	default:
		return nil, fmt.Errorf("telegram: unexpected dialogs response %T", res)
	}
	chats, known, err := ents.resolve(refs)
	s.known = known
	return chats, err
}

func (s *service) History(ctx context.Context, since time.Time) ([]domain.Message, error) {
	if s.known == nil {
		return nil, fmt.Errorf("telegram: chats not loaded yet")
	}
	var out []domain.Message
	for id, ent := range s.known {
		msgs, err := s.chatHistory(ctx, id, ent, since)
		if err != nil {
			return out, err
		}
		out = append(out, msgs...)
	}
	return out, nil
}

// chatHistory pages backwards from the newest message until it reaches
// messages not newer than since, or hits maxPerChat.
func (s *service) chatHistory(ctx context.Context, id string, ent entity, since time.Time) ([]domain.Message, error) {
	var out []domain.Message
	offsetID := 0
	for len(out) < maxPerChat {
		res, err := s.api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
			Peer: ent.peer, OffsetID: offsetID, Limit: pageSize,
		})
		if err != nil {
			return out, fmt.Errorf("telegram: get history: %w", err)
		}
		raw, users := historyParts(res)
		if len(raw) == 0 {
			break
		}
		ents := collect(users, nil)
		reachedOld := false
		lowest := raw[0].GetID()
		for _, mc := range raw {
			if mc.GetID() < lowest {
				lowest = mc.GetID()
			}
			m, ok := mc.(*tg.Message)
			if !ok {
				continue // service messages and empty placeholders
			}
			t := time.Unix(int64(m.Date), 0).UTC()
			if !since.IsZero() && !t.After(since) {
				reachedOld = true
				continue
			}
			out = append(out, convert(id, ent, m, ents))
		}
		if reachedOld || len(raw) < pageSize || lowest == offsetID {
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
	if p, ok := m.FromID.(*tg.PeerUser); ok {
		if u, ok := ents.users[p.UserID]; ok {
			return userName(u)
		}
		return "user"
	}
	if m.PostAuthor != "" {
		return m.PostAuthor
	}
	if chat.kind == kindUser { // private chat: outgoing or incoming without FromID
		return chat.title
	}
	return "channel"
}
