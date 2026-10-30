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

// Chat is one conversation on some service.
type Chat struct {
	ID      string `json:"id"`      // stable, service-scoped identifier
	Service string `json:"service"` // connector name, e.g. "telegram"
	Alias   string `json:"alias"`   // short display name used in the feed
	Title   string `json:"title,omitempty"`
}

// Message is one message in a chat.
type Message struct {
	ID     string    `json:"id"` // unique within the chat
	ChatID string    `json:"chat_id"`
	Sender string    `json:"sender"`
	Text   string    `json:"text"`
	Time   time.Time `json:"time"`
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
// case-insensitive prefix, then by unique substring of alias or title.
func ResolveChat(chats []Chat, query string) (Chat, error) {
	q := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(query), "#"))
	if q == "" {
		return Chat{}, ErrNoChat
	}
	for _, c := range chats {
		if strings.ToLower(c.ID) == q || strings.ToLower(c.Alias) == q {
			return c, nil
		}
	}
	for _, match := range []func(Chat) bool{
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
