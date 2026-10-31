package telegram

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gotd/td/tg"

	"lazygram/internal/config"
	"lazygram/internal/domain"
)

// entity is a Telegram peer reduced to what lazygram needs.
type entity struct {
	id       int64 // Bot-API style: users positive, groups -id, channels -100id
	username string
	title    string
	peer     tg.InputPeerClass
	// raw is the peer's own numeric id, used to look up message senders.
	raw  int64
	kind peerKind
}

type peerKind int

const (
	kindUser peerKind = iota
	kindGroup
	kindChannel
)

// entities indexes the users and chats returned by the API.
type entities struct {
	byRef map[int64]entity // by Bot-API style id
	users map[int64]*tg.User
}

func collect(users []tg.UserClass, chats []tg.ChatClass) entities {
	e := entities{byRef: map[int64]entity{}, users: map[int64]*tg.User{}}
	for _, uc := range users {
		u, ok := uc.(*tg.User)
		if !ok {
			continue
		}
		e.users[u.ID] = u
		e.byRef[u.ID] = entity{
			id: u.ID, raw: u.ID, kind: kindUser, username: u.Username,
			title: userName(u),
			peer:  &tg.InputPeerUser{UserID: u.ID, AccessHash: u.AccessHash},
		}
	}
	for _, cc := range chats {
		switch c := cc.(type) {
		case *tg.Chat:
			e.byRef[-c.ID] = entity{id: -c.ID, raw: c.ID, kind: kindGroup, title: c.Title,
				peer: &tg.InputPeerChat{ChatID: c.ID}}
		case *tg.Channel:
			id := -1000000000000 - c.ID
			e.byRef[id] = entity{id: id, raw: c.ID, kind: kindChannel, username: c.Username, title: c.Title,
				peer: &tg.InputPeerChannel{ChannelID: c.ID, AccessHash: c.AccessHash}}
		}
	}
	return e
}

func userName(u *tg.User) string {
	name := strings.TrimSpace(u.FirstName + " " + u.LastName)
	switch {
	case name != "":
		return name
	case u.Username != "":
		return u.Username
	}
	return "user"
}

// match finds the entity a configured reference points at. A reference is a
// numeric Bot-API style id ("-100123", "42") or "@username".
func (e entities) match(ref string) (entity, bool) {
	ref = strings.TrimSpace(ref)
	if name, ok := strings.CutPrefix(ref, "@"); ok {
		for _, ent := range e.byRef {
			if ent.username != "" && strings.EqualFold(ent.username, name) {
				return ent, true
			}
		}
		return entity{}, false
	}
	id, err := strconv.ParseInt(ref, 10, 64)
	if err != nil {
		return entity{}, false
	}
	ent, ok := e.byRef[id]
	return ent, ok
}

// resolve maps configured refs to chats and their entities. Unknown refs are
// reported together so a typo does not silently drop a chat.
func (e entities) resolve(refs []config.ChatRef) ([]domain.Chat, map[string]entity, error) {
	var chats []domain.Chat
	byChat := map[string]entity{}
	var missing []string
	for _, r := range refs {
		ent, ok := e.match(r.Ref)
		if !ok {
			missing = append(missing, r.Alias)
			continue
		}
		c := domain.Chat{ID: chatID(ent), Service: "telegram", Alias: r.Alias, Title: ent.title}
		chats = append(chats, c)
		byChat[c.ID] = ent
	}
	if len(missing) > 0 {
		return chats, byChat, fmt.Errorf("telegram: no dialog found for alias(es): %s", strings.Join(missing, ", "))
	}
	return chats, byChat, nil
}

func chatID(e entity) string { return "telegram:" + strconv.FormatInt(e.id, 10) }
