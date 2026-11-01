package domain

import (
	"sort"
	"time"
)

// TypingKind says what a chat member is doing, as far as the service tells.
type TypingKind int

// Typing kinds. TypingStop is a report that the member stopped.
const (
	TypingText   TypingKind = iota // typing a message
	TypingRecord                   // recording a voice or video note
	TypingUpload                   // uploading a photo, video or file
	TypingChoose                   // choosing a sticker, contact, location or game
	TypingStop                     // stopped, says the service
)

// String is the verb shown to the user.
func (k TypingKind) String() string {
	switch k {
	case TypingRecord:
		return "recording"
	case TypingUpload:
		return "uploading"
	case TypingChoose:
		return "choosing"
	case TypingStop:
		return "stopped"
	}
	return "typing"
}

// DefaultTypingTTL is how long one report counts without a renewal; the
// official clients use six seconds and renew every five while typing.
const DefaultTypingTTL = 6 * time.Second

// Typing is one report from a service: a member of a chat is doing
// something (or stopped). Who is the member's display name as messages
// show it; a service that cannot name the member leaves it empty.
type Typing struct {
	ChatID string
	Who    string
	Kind   TypingKind
	At     time.Time // when the service reported it
}

// Typist is a member currently doing something in a chat; Since is their
// first report.
type Typist struct {
	Who   string
	Kind  TypingKind
	Since time.Time
}

// Unknown names a member the service could not name.
const Unknown = "someone"

// TypingState keeps who is doing what in which chat. A report is good for
// TTL and renewed by the next one; a stop report, a message from the
// member or the expiry ends it. Not safe for concurrent use: one goroutine
// owns it (the TUI's).
type TypingState struct {
	TTL    time.Duration
	byChat map[string]map[string]*typist
}

type typist struct {
	kind         TypingKind
	since, until time.Time
}

// NewTypingState makes an empty state; ttl <= 0 means DefaultTypingTTL.
func NewTypingState(ttl time.Duration) *TypingState {
	if ttl <= 0 {
		ttl = DefaultTypingTTL
	}
	return &TypingState{TTL: ttl, byChat: map[string]map[string]*typist{}}
}

// Apply takes one report in. It reports whether what is shown changed: a
// new member, a changed kind or a stop; a renewal of the same kind only
// moves the expiry.
func (s *TypingState) Apply(ev Typing) bool {
	who := ev.Who
	if who == "" {
		who = Unknown
	}
	if ev.Kind == TypingStop {
		return s.Clear(ev.ChatID, who)
	}
	chat := s.byChat[ev.ChatID]
	if chat == nil {
		chat = map[string]*typist{}
		s.byChat[ev.ChatID] = chat
	}
	until := ev.At.Add(s.TTL)
	if t, ok := chat[who]; ok {
		changed := t.kind != ev.Kind
		t.kind = ev.Kind
		if until.After(t.until) {
			t.until = until
		}
		return changed
	}
	chat[who] = &typist{kind: ev.Kind, since: ev.At, until: until}
	return true
}

// Expire drops every report older than TTL as of now and reports whether
// anything was dropped.
func (s *TypingState) Expire(now time.Time) bool {
	changed := false
	for chatID, chat := range s.byChat {
		for who, t := range chat {
			if !t.until.After(now) {
				delete(chat, who)
				changed = true
			}
		}
		if len(chat) == 0 {
			delete(s.byChat, chatID)
		}
	}
	return changed
}

// Clear ends who's report in a chat, for example because their message
// arrived, and reports whether there was one.
func (s *TypingState) Clear(chatID, who string) bool {
	chat := s.byChat[chatID]
	if _, ok := chat[who]; !ok {
		return false
	}
	delete(chat, who)
	if len(chat) == 0 {
		delete(s.byChat, chatID)
	}
	return true
}

// Typists lists who is doing what in a chat, the earliest report first,
// names breaking ties.
func (s *TypingState) Typists(chatID string) []Typist {
	chat := s.byChat[chatID]
	if len(chat) == 0 {
		return nil
	}
	out := make([]Typist, 0, len(chat))
	for who, t := range chat {
		out = append(out, Typist{Who: who, Kind: t.kind, Since: t.since})
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Since.Equal(out[j].Since) {
			return out[i].Since.Before(out[j].Since)
		}
		return out[i].Who < out[j].Who
	})
	return out
}

// Chats lists the chats with activity, sorted by id.
func (s *TypingState) Chats() []string {
	out := make([]string, 0, len(s.byChat))
	for id := range s.byChat {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// Next is the earliest expiry, for whoever schedules Expire; ok is false
// when nothing is pending.
func (s *TypingState) Next() (next time.Time, ok bool) {
	for _, chat := range s.byChat {
		for _, t := range chat {
			if !ok || t.until.Before(next) {
				next, ok = t.until, true
			}
		}
	}
	return next, ok
}

// Empty reports whether nobody is doing anything anywhere.
func (s *TypingState) Empty() bool { return len(s.byChat) == 0 }
