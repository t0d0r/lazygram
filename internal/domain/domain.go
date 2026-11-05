// Package domain holds the service-neutral model: chats, messages, and the
// pure functions that merge, filter, and select them.
package domain

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ChatKind says what kind of conversation a chat is. It decides the IRC
// sigil: people are @alias, groups and channels are #alias.
type ChatKind int

// Chat kinds.
const (
	ChatUser    ChatKind = iota // a person or bot
	ChatGroup                   // a group conversation
	ChatChannel                 // a broadcast channel
)

// Chat is one conversation on some service.
type Chat struct {
	ID      string   `json:"id"`      // stable, service-scoped identifier
	Service string   `json:"service"` // connector name, e.g. "telegram"
	Alias   string   `json:"alias"`   // short name without sigil
	Title   string   `json:"title,omitempty"`
	Kind    ChatKind `json:"kind"`
	Unread  int      `json:"unread,omitempty"`  // unread count reported by the service
	Deleted bool     `json:"deleted,omitempty"` // the account behind the chat no longer exists
	// Username is a group's or channel's own public username, without @;
	// empty for people and for chats without one. Unlike Alias it is
	// exact: posts that sign with it mention the chat itself.
	Username string `json:"username,omitempty"`
	// SharedIDs says the chat's message ids are unique across all the
	// account's chats that have it (Telegram: private chats and basic
	// groups), so a deletion the service reports without a chat names a
	// message of one of them.
	SharedIDs bool `json:"shared_ids,omitempty"`
}

// Name is the IRC-style name: @alias for people, #alias otherwise.
func (c Chat) Name() string {
	if c.Kind == ChatUser {
		return "@" + c.Alias
	}
	return "#" + c.Alias
}

// ChangeKind says what a Change does.
type ChangeKind int

// Change kinds.
const (
	Edited  ChangeKind = iota // Msg is the message as it reads now
	Deleted                   // IDs are gone from ChatID
	Stale                     // ChatID, or every chat when "", may have missed changes: fetch again
	Reacted                   // Msg.Reactions are the reactions message Msg.ID of ChatID has now
)

// Change is an edit, a deletion or new reactions the service reports
// while lazygram runs, made here or on another device, by the account or
// someone else.
type Change struct {
	Kind ChangeKind
	// ChatID is the chat; for a deletion it may be "", meaning the one of
	// the chats with SharedIDs that has the message.
	ChatID string
	IDs    []string // Deleted: the message ids
	// Msg is, for Edited, the message as it is now: its text, media,
	// buttons and reactions. For Reacted it carries the message's ID,
	// ChatID and Reactions (with MineUnknown) only.
	Msg Message
	// Reactors are, for Reacted, the recent reactions the service names
	// with who sent them, in its order; empty when it names nobody (a
	// broadcast channel, whose reactors are anonymous).
	Reactors []Reactor
	// Own says, for Reacted, that the service pushed the change unasked,
	// which Telegram does only for the account's own messages: a message
	// lazygram does not hold is then one of the account's.
	Own bool
}

// CustomReaction is the Emoji of all a message's custom-emoji reactions
// together: their emoji is a document of its own, which lazygram does not
// fetch, so they are counted as one.
const CustomReaction = "*"

// Reaction is one kind of reaction under a message and how many sent it.
type Reaction struct {
	Emoji string `json:"emoji"`          // the emoji, or CustomReaction
	Count int    `json:"count"`          // more than zero
	Mine  bool   `json:"mine,omitempty"` // the account sent it too
	// MineCount is how much of Count is the account's own when the service
	// says: more than one for a paid reaction, whose count is of Stars, and
	// for CustomReaction when several of the account's are summed in it.
	// Zero when not known: Own then counts Mine as one.
	MineCount int `json:"mine_count,omitempty"`
}

// Own is how much of Count the account sent: MineCount, else one when
// Mine.
func (r Reaction) Own() int {
	switch {
	case r.MineCount > 0:
		return r.MineCount
	case r.Mine:
		return 1
	}
	return 0
}

// Reactor is one recent reaction the service names: who sent which emoji
// and when.
type Reactor struct {
	Who   string    // named like a message sender: a person, a channel's title, "me" for the account
	ID    string    // their chat id, like Message.SenderID; empty for the account and when unknown
	Emoji string    // as in Reaction.Emoji
	At    time.Time // zero when unknown
}

// KeepMine returns now with the Mine flags and counts of known, for
// reactions the service sent without the account's own
// (Message.MineUnknown): each emoji keeps whether, and how much of it,
// the account had sent.
func KeepMine(known, now []Reaction) []Reaction {
	if len(now) == 0 {
		return now
	}
	mine := map[string]Reaction{}
	for _, r := range known {
		if r.Mine {
			mine[r.Emoji] = r
		}
	}
	out := make([]Reaction, len(now))
	for i, r := range now {
		k := mine[r.Emoji]
		r.Mine, r.MineCount = k.Mine, k.MineCount
		out[i] = r
	}
	return out
}

// CurrentReactions returns the reactions of a message known as known,
// given later, a copy of it fetched or reported after: reactions carry no
// edit time, so later's are current, with known's Mine flags and counts
// when later's are unknown.
func CurrentReactions(known, later Message) []Reaction {
	if later.MineUnknown {
		return KeepMine(known.Reactions, later.Reactions)
	}
	return later.Reactions
}

// ButtonKind says what pressing a button does.
type ButtonKind int

// Button kinds.
const (
	ButtonCallback ButtonKind = iota // sends Data to the bot, which answers
	ButtonURL                        // opens URL; lazygram only prints it
	ButtonText                       // a reply-keyboard button: sends Label as a message
	ButtonOther                      // games, payments, web apps, ...: not supported
)

// Button is a button a bot attached to a message.
type Button struct {
	Label string     `json:"label"`
	Kind  ButtonKind `json:"kind"`
	Data  []byte     `json:"data,omitempty"`
	URL   string     `json:"url,omitempty"`
}

// Media is an attachment a provider can fetch to a local file.
type Media struct {
	Kind string `json:"kind"`           // "photo", "image" (a file that is an image) or "file"
	Name string `json:"name,omitempty"` // file name, when the service gives one
	// ID is the service's id of the attachment itself (Telegram: the photo
	// or document id), so an edit that swaps it for another is told apart
	// from one that keeps it; empty when unknown.
	ID string `json:"id,omitempty"`
}

// IsImageName reports whether a file name has a raster image extension.
func IsImageName(name string) bool {
	switch strings.ToLower(filepathExt(name)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		return true
	}
	return false
}

func filepathExt(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 && i > strings.LastIndexAny(name, "/\\") {
		return name[i:]
	}
	return ""
}

// Image reports whether the attachment is a picture a viewer can show.
func (m *Media) Image() bool { return m != nil && (m.Kind == "photo" || m.Kind == "image") }

// Message is one message in a chat.
type Message struct {
	ID     string `json:"id"` // unique within the chat
	ChatID string `json:"chat_id"`
	Sender string `json:"sender"`
	// SenderID is the sender's own chat id (a person), SenderNick their
	// username without @; both empty when unknown, for own messages and
	// for posts signed by a channel. They tell group members apart.
	SenderID   string    `json:"sender_id,omitempty"`
	SenderNick string    `json:"sender_nick,omitempty"`
	Text       string    `json:"text"` // the caption, for a message with Media
	Time       time.Time `json:"time"`
	Buttons    []Button  `json:"buttons,omitempty"` // numbered from 1 in this order
	Media      *Media    `json:"media,omitempty"`
	// Edited is when the message was last edited at the service; zero
	// when never (or not known). Of two copies the later edit wins, but
	// for Reactions, which the later copy carries as they are now.
	Edited time.Time `json:"edited,omitempty"`
	// Reactions are the reactions under the message, in the service's
	// order; none when nobody reacted.
	Reactions []Reaction `json:"reactions,omitempty"`
	// MineUnknown says Reactions came without the account's own
	// (Telegram's min reactions, the same for every user), so each keeps
	// the Mine flag and count already known for its emoji
	// (CurrentReactions). It is never stored.
	MineUnknown bool `json:"-"`
}

// PressAnswer is what a provider reports after a button was pressed.
type PressAnswer struct {
	Text    string   // the bot's answer, if any
	URL     string   // a URL the bot asked to open, if any
	Updated *Message // the message as it is after the press, when it changed
}

// Key identifies a message across chats, for de-duplication.
func (m Message) Key() string { return m.ChatID + "\x00" + m.ID }

// Merge combines message lists into one chronological list. Duplicates (same
// chat and message ID) are dropped, keeping the first seen. Ties on time are
// broken by chat ID then message ID so the order is deterministic.
func Merge(lists ...[]Message) []Message {
	seen := make(map[string]struct{})
	var out []Message
	for _, l := range lists {
		for _, m := range l {
			k := m.Key()
			if _, dup := seen[k]; dup {
				continue
			}
			seen[k] = struct{}{}
			out = append(out, m)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if !a.Time.Equal(b.Time) {
			return a.Time.Before(b.Time)
		}
		if a.ChatID != b.ChatID {
			return a.ChatID < b.ChatID
		}
		return a.ID < b.ID
	})
	return out
}

// Query selects messages. Zero fields match everything.
type Query struct {
	ChatID string    // exact chat; empty means all chats
	Text   string    // case-insensitive substring of text or sender
	Since  time.Time // inclusive lower bound; zero means none
}

// Replace swaps the message with the same key in msgs, or appends it.
func Replace(msgs []Message, m Message) []Message {
	for i := range msgs {
		if msgs[i].Key() == m.Key() {
			msgs[i] = m
			return msgs
		}
	}
	return Merge(msgs, []Message{m})
}

// LastOwn returns the newest message of chatID sent by this account.
func LastOwn(msgs []Message, chatID string) (Message, bool) {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].ChatID == chatID && msgs[i].Sender == "me" {
			return msgs[i], true
		}
	}
	return Message{}, false
}

// LastFromOthers returns the newest message of chatID that someone
// other than this account sent and that has an id: the target of /like.
func LastFromOthers(msgs []Message, chatID string) (Message, bool) {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].ChatID == chatID && msgs[i].Sender != "me" && msgs[i].ID != "" {
			return msgs[i], true
		}
	}
	return Message{}, false
}

// Remove drops the message with the given chat and id, if present.
func Remove(msgs []Message, chatID, msgID string) []Message {
	for i := range msgs {
		if msgs[i].ChatID == chatID && msgs[i].ID == msgID {
			return append(append([]Message{}, msgs[:i]...), msgs[i+1:]...)
		}
	}
	return msgs
}

// LatestWithMedia returns the newest message of chatID with an attachment.
func LatestWithMedia(msgs []Message, chatID string) (Message, bool) {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].ChatID == chatID && msgs[i].Media != nil {
			return msgs[i], true
		}
	}
	return Message{}, false
}

// LatestWithButtons returns the newest message of chatID that has buttons.
func LatestWithButtons(msgs []Message, chatID string) (Message, bool) {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].ChatID == chatID && len(msgs[i].Buttons) > 0 {
			return msgs[i], true
		}
	}
	return Message{}, false
}

// Filter returns the messages matching q, preserving order.
func Filter(msgs []Message, q Query) []Message {
	mt := NewMatcher(q.Text) // normalised once for every message
	var out []Message
	for _, m := range msgs {
		if q.ChatID != "" && m.ChatID != q.ChatID {
			continue
		}
		if !q.Since.IsZero() && m.Time.Before(q.Since) {
			continue
		}
		if !mt.Empty() && !mt.Match(m.Text) && !mt.Match(m.Sender) {
			continue
		}
		out = append(out, m)
	}
	return out
}

// Tail returns at most the last n messages.
func Tail(msgs []Message, n int) []Message {
	if n < 0 || len(msgs) <= n {
		return msgs
	}
	return msgs[len(msgs)-n:]
}

// Latest returns the newest message time, or the zero time when empty.
func Latest(msgs []Message) time.Time {
	var t time.Time
	for _, m := range msgs {
		if m.Time.After(t) {
			t = m.Time
		}
	}
	return t
}

// Errors returned by ResolveChat.
var (
	ErrNoChat    = errors.New("no matching chat")
	ErrAmbiguous = errors.New("ambiguous chat")
)

// ResolveChat finds one chat by name, case-insensitively, in tiers, the
// first tier with any hit deciding (one hit resolves, several are
// ambiguous): the exact ID or alias, the exact title (the name /names
// shows resolves even when another title contains it), a prefix of the
// alias, a substring of the alias or title; then in either script (a Latin
// query finds a title written in Cyrillic and the other way round): the
// exact alias or title, a prefix of either, a substring of either. The
// either-script tiers make a Latin spelling as decisive as the Cyrillic
// one once no plain tier matches, and come after the plain ones, so they
// never turn a plain match ambiguous; a plain match still wins over them
// ("kamen" begins the alias kamenov as typed, so it finds that chat and
// not the one titled Kamen in Cyrillic, which the Cyrillic spelling
// finds by its exact title). A leading @ restricts the search to people,
// a leading # to groups and channels.
func ResolveChat(chats []Chat, query string) (Chat, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	switch {
	case strings.HasPrefix(q, "@"):
		chats = ofKind(chats, func(c Chat) bool { return c.Kind == ChatUser })
		q = q[1:]
	case strings.HasPrefix(q, "#"):
		chats = ofKind(chats, func(c Chat) bool { return c.Kind != ChatUser })
		q = q[1:]
	}
	if q == "" {
		return Chat{}, ErrNoChat
	}
	title := func(c Chat) string { return strings.TrimSpace(c.Title) }
	tiers := []func(Chat) bool{
		func(c Chat) bool { return strings.ToLower(c.ID) == q || strings.ToLower(c.Alias) == q },
		func(c Chat) bool { return strings.ToLower(title(c)) == q },
		func(c Chat) bool { return strings.HasPrefix(strings.ToLower(c.Alias), q) },
		func(c Chat) bool {
			return strings.Contains(strings.ToLower(c.Alias), q) ||
				strings.Contains(strings.ToLower(c.Title), q)
		},
	}
	// A query that transliterates to nothing (only combining marks) would
	// match every chat in either script; it gets no such tier.
	if either := NewMatcher(q); !either.Empty() {
		tiers = append(tiers,
			func(c Chat) bool { return either.MatchExact(c.Alias) || either.MatchExact(title(c)) },
			func(c Chat) bool { return either.MatchPrefix(c.Alias) || either.MatchPrefix(title(c)) },
			func(c Chat) bool { return either.Match(c.Alias) || either.Match(c.Title) })
	}
	for _, match := range tiers {
		var hits []Chat
		for _, c := range chats {
			if match(c) {
				hits = append(hits, c)
			}
		}
		switch len(hits) {
		case 1:
			return hits[0], nil
		case 0:
		default:
			return Chat{}, fmt.Errorf("%w: %q matches %d chats", ErrAmbiguous, query, len(hits))
		}
	}
	return Chat{}, fmt.Errorf("%w: %q", ErrNoChat, query)
}

func ofKind(chats []Chat, keep func(Chat) bool) []Chat {
	var out []Chat
	for _, c := range chats {
		if keep(c) {
			out = append(out, c)
		}
	}
	return out
}

// FilterChats returns chats whose alias, title, or ID contains text.
func FilterChats(chats []Chat, text string) []Chat {
	needle := strings.ToLower(strings.TrimSpace(text))
	if needle == "" {
		return chats
	}
	var out []Chat
	for _, c := range chats {
		if strings.Contains(strings.ToLower(c.Alias), needle) ||
			strings.Contains(strings.ToLower(c.Title), needle) ||
			strings.Contains(strings.ToLower(c.ID), needle) {
			out = append(out, c)
		}
	}
	return out
}
