package telegram

import (
	"strconv"
	"strings"

	"github.com/gotd/td/tg"

	"lazygram/internal/domain"
)

// entity is a Telegram peer reduced to what lazygram needs.
type entity struct {
	id       int64 // Bot-API style: users positive, groups -id, channels -100<id>
	username string
	title    string
	peer     tg.InputPeerClass
	channel  *tg.InputChannel // set for channels and supergroups
	kind     peerKind
	deleted  bool // a user account that no longer exists
}

func (e entity) chatKind() domain.ChatKind {
	switch e.kind {
	case kindGroup:
		return domain.ChatGroup
	case kindChannel:
		return domain.ChatChannel
	}
	return domain.ChatUser
}

type peerKind int

const (
	kindUser peerKind = iota
	kindGroup
	kindChannel
)

const channelBase = -1000000000000

// entities indexes the users and chats returned by the API.
type entities struct {
	byID  map[int64]entity // by Bot-API style id
	users map[int64]*tg.User
}

func collect(users []tg.UserClass, chats []tg.ChatClass) entities {
	e := entities{byID: map[int64]entity{}, users: map[int64]*tg.User{}}
	for _, uc := range users {
		u, ok := uc.(*tg.User)
		if !ok {
			continue
		}
		e.users[u.ID] = u
		ent := entity{id: u.ID, kind: kindUser, username: u.Username, title: userName(u), deleted: u.Deleted,
			peer: &tg.InputPeerUser{UserID: u.ID, AccessHash: u.AccessHash}}
		switch {
		case u.Self:
			ent.username, ent.title = "saved", "Saved Messages"
		case u.Deleted:
			ent.username = "deleted-" + strconv.FormatInt(u.ID, 10)
		}
		e.byID[u.ID] = ent
	}
	for _, cc := range chats {
		switch c := cc.(type) {
		case *tg.Chat:
			e.byID[-c.ID] = entity{id: -c.ID, kind: kindGroup, title: c.Title,
				peer: &tg.InputPeerChat{ChatID: c.ID}}
		case *tg.Channel:
			id := channelBase - c.ID
			kind := kindChannel
			if c.Megagroup || c.Gigagroup {
				kind = kindGroup // supergroups are conversations, not broadcasts
			}
			e.byID[id] = entity{id: id, kind: kind, username: c.Username, title: c.Title,
				peer:    &tg.InputPeerChannel{ChannelID: c.ID, AccessHash: c.AccessHash},
				channel: &tg.InputChannel{ChannelID: c.ID, AccessHash: c.AccessHash}}
		}
	}
	return e
}

// byPeer finds the entity a dialog's peer refers to.
func (e entities) byPeer(p tg.PeerClass) (entity, bool) {
	var id int64
	switch p := p.(type) {
	case *tg.PeerUser:
		id = p.UserID
	case *tg.PeerChat:
		id = -p.ChatID
	case *tg.PeerChannel:
		id = channelBase - p.ChannelID
	default:
		return entity{}, false
	}
	ent, ok := e.byID[id]
	return ent, ok
}

func userName(u *tg.User) string {
	name := strings.TrimSpace(u.FirstName + " " + u.LastName)
	switch {
	case u.Deleted:
		return "Deleted Account"
	case name != "":
		return name
	case u.Username != "":
		return u.Username
	}
	return "user"
}

func chatID(e entity) string { return "telegram:" + strconv.FormatInt(e.id, 10) }
