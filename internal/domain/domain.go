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
}

// Name is the IRC-style name: @alias for people, #alias otherwise.
func (c Chat) Name() string {
	if c.Kind == ChatUser {
		return "@" + c.Alias
	}
	return "#" + c.Alias
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
}

// Image reports whether the attachment is a picture a viewer can show.
func (m *Media) Image() bool { return m != nil && (m.Kind == "photo" || m.Kind == "image") }

// Message is one message in a chat.
type Message struct {
	ID      string    `json:"id"` // unique within the chat
	ChatID  string    `json:"chat_id"`
	Sender  string    `json:"sender"`
	Text    string    `json:"text"` // the caption, for a message with Media
	Time    time.Time `json:"time"`
	Buttons []Button  `json:"buttons,omitempty"` // numbered from 1 in this order
	Media   *Media    `json:"media,omitempty"`
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
	needle := strings.ToLower(strings.TrimSpace(q.Text))
	var out []Message
	for _, m := range msgs {
		if q.ChatID != "" && m.ChatID != q.ChatID {
			continue
		}
		if !q.Since.IsZero() && m.Time.Before(q.Since) {
			continue
		}
		if needle != "" &&
			!strings.Contains(strings.ToLower(m.Text), needle) &&
			!strings.Contains(strings.ToLower(m.Sender), needle) {
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

// ResolveChat finds one chat by exact ID or alias, then by unique
// case-insensitive prefix, then by unique substring of alias or title. A
// leading @ restricts the search to people, a leading # to groups and
// channels.
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
	for _, match := range []func(Chat) bool{
		func(c Chat) bool { return strings.ToLower(c.ID) == q || strings.ToLower(c.Alias) == q },
		func(c Chat) bool { return strings.HasPrefix(strings.ToLower(c.Alias), q) },
		func(c Chat) bool {
			return strings.Contains(strings.ToLower(c.Alias), q) ||
				strings.Contains(strings.ToLower(c.Title), q)
		},
	} {
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
