package domain

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Member is someone in a group or channel: the sender of one of its known
// messages, a username its messages mention, or an entry of its member
// list from the service (WithParticipants).
type Member struct {
	Name     string    // display name, as the messages or the list show it; empty when only mentioned
	Nick     string    // username without @; empty when they have none
	ID       string    // their own chat id; empty when unknown
	Count    int       // known messages by them in the chat
	Mentions int       // known messages in the chat that mention @Nick
	Last     time.Time // their latest known message; the latest mention when they never wrote
	Listed   bool      // in the member list from the service
}

// Participant is one entry of a group's member list from the service.
type Participant struct {
	ID   string // the person's own chat id
	Name string // display name
	Nick string // username without @; empty when they have none
}

// unsignedPost is the sender the Telegram connector gives a channel post
// without an author signature: the chat itself, not a member.
const unsignedPost = "channel"

// Members lists the chat's members in msgs, the most recently active
// first: the distinct senders of its messages and the usernames its
// messages mention (MentionsIn).
//
// A sender with an ID is one member however their name changed: the
// newest message gives the name and the nick, an empty nick included (the
// username was dropped). A message without an ID is an own message ("me"),
// an unsigned channel post, or one from a sender the service did not
// identify (cached before IDs were kept, a signed channel post); the first
// two are left out, the last counts for the one member with that name, or
// else under the name alone.
//
// A mention counts for the sender whose nick it is, ignoring case, and
// otherwise makes a member known by the nick alone, with no name. Own
// messages and channel posts mention people too.
func Members(msgs []Message, chatID string) []Member {
	byKey := map[string]*Member{}
	add := func(key string, m Message) {
		mb := byKey[key]
		if mb == nil {
			mb = &Member{ID: m.SenderID}
			byKey[key] = mb
		}
		mb.Count++
		if mb.Count == 1 || m.Time.After(mb.Last) { // the newest message names them
			mb.Last, mb.Name, mb.Nick = m.Time, m.Sender, m.SenderNick
		}
	}
	var anon []Message
	for _, m := range msgs {
		switch {
		case m.ChatID != chatID || m.Sender == "":
		case m.SenderID != "":
			add("id:"+m.SenderID, m)
		case m.Sender == "me" || m.Sender == unsignedPost: // own, or the chat itself
		default:
			anon = append(anon, m)
		}
	}
	byName := map[string][]*Member{}
	for _, mb := range byKey {
		byName[mb.Name] = append(byName[mb.Name], mb)
	}
	for _, m := range anon {
		if ms := byName[m.Sender]; len(ms) == 1 {
			ms[0].Count++
			if m.Time.After(ms[0].Last) {
				ms[0].Last = m.Time
			}
			continue
		}
		add("name:"+m.Sender, m)
	}
	// A username moves between people: the member who wrote with it last
	// keeps it, the others lose it (they had it once). Ties go to the
	// smaller key, so the result does not depend on map order.
	byNick := map[string]*Member{}
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		mb := byKey[k]
		if mb.Nick == "" {
			continue
		}
		n := strings.ToLower(mb.Nick)
		switch held := byNick[n]; {
		case held == nil:
			byNick[n] = mb
		case mb.Last.After(held.Last):
			held.Nick, byNick[n] = "", mb
		default:
			mb.Nick = ""
		}
	}
	for _, m := range msgs {
		if m.ChatID != chatID {
			continue
		}
		for _, nick := range MentionsIn(m.Text) {
			k := strings.ToLower(nick)
			mb := byNick[k]
			if mb == nil {
				mb = &Member{Nick: nick}
				byNick[k], byKey["nick:"+k] = mb, mb
			}
			mb.Mentions++
			if mb.Count == 0 && m.Time.After(mb.Last) {
				mb.Last, mb.Nick = m.Time, nick // the newest spelling of a nick only mentioned
			}
		}
	}
	out := make([]Member, 0, len(byKey))
	for _, mb := range byKey {
		out = append(out, *mb)
	}
	sortMembers(out)
	return out
}

// sortMembers orders members the most recently active first, then by name
// and nick, so the order never depends on map iteration.
func sortMembers(ms []Member) {
	sort.Slice(ms, func(i, j int) bool {
		if !ms[i].Last.Equal(ms[j].Last) {
			return ms[i].Last.After(ms[j].Last)
		}
		if ms[i].Name != ms[j].Name {
			return ms[i].Name < ms[j].Name
		}
		return ms[i].Nick < ms[j].Nick
	})
}

// WithParticipants adds a group's member list from the service to the
// members its messages show, in a new slice. A listed person is the member
// with their id, else the one known only by the username they hold (a nick
// only mentioned), else the one sender known by their display name alone
// (a sender cached without an id) when no other listed person has that
// name; else they join with no messages. A sender known by that name alone
// is folded into a person matched by id or username too, so one person is
// one member. The list is current, so its name and username win, an empty
// username included; anyone else who had that username loses it, and the
// mentions of a nick only mentioned go to its listed holder.
func WithParticipants(ms []Member, ps []Participant) []Member {
	if len(ps) == 0 {
		return ms
	}
	out := append([]Member(nil), ms...)
	gone := make([]bool, len(out))
	byID, byNick, byName := map[string]int{}, map[string]int{}, map[string][]int{}
	for i, mb := range out {
		switch {
		case mb.ID != "":
			byID[mb.ID] = i
		case mb.Nick != "":
			byNick[strings.ToLower(mb.Nick)] = i
		default:
			byName[mb.Name] = append(byName[mb.Name], i)
		}
	}
	listedNames := map[string]int{}
	for _, p := range ps {
		listedNames[p.Name]++
	}
	// nameOnly is the one unclaimed sender known by name alone, or -1.
	nameOnly := func(name string, except int) int {
		if name == "" || listedNames[name] != 1 {
			return -1
		}
		hit, n := -1, 0
		for _, j := range byName[name] {
			if !gone[j] && j != except {
				hit, n = j, n+1
			}
		}
		if n != 1 {
			return -1
		}
		return hit
	}
	holder := map[string]int{} // username -> index of the listed member holding it
	for _, p := range ps {
		i, ok := byID[p.ID]
		if !ok && p.Nick != "" {
			if j, found := byNick[strings.ToLower(p.Nick)]; found && !gone[j] {
				i, ok = j, true
			}
		}
		if !ok {
			if j := nameOnly(p.Name, -1); j >= 0 {
				i, ok = j, true
			}
		}
		if !ok {
			out, gone = append(out, Member{}), append(gone, false)
			i = len(out) - 1
		}
		if j := nameOnly(p.Name, i); j >= 0 { // their messages from before ids were kept
			a, b := &out[i], out[j]
			if a.Count == 0 || b.Last.After(a.Last) {
				a.Last = b.Last
			}
			a.Count += b.Count
			gone[j] = true
		}
		mb := &out[i]
		mb.ID, mb.Nick, mb.Listed = p.ID, p.Nick, true
		if p.Name != "" {
			mb.Name = p.Name
		}
		byID[mb.ID] = i
		if p.Nick != "" {
			holder[strings.ToLower(p.Nick)] = i
		}
		listedNames[p.Name] = -1 // its name-only sender, if any, is decided
	}
	for k := range out {
		mb := &out[k]
		h, held := holder[strings.ToLower(mb.Nick)]
		if gone[k] || mb.Nick == "" || !held || h == k {
			continue
		}
		if mb.Name == "" && mb.ID == "" { // a nick only mentioned: its mentions are the holder's
			hd := &out[h]
			hd.Mentions += mb.Mentions
			if hd.Count == 0 && mb.Last.After(hd.Last) {
				hd.Last = mb.Last
			}
			gone[k] = true
			continue
		}
		mb.Nick = "" // the username is someone else's now
	}
	kept := make([]Member, 0, len(out))
	for k, mb := range out {
		if !gone[k] && (mb.Name != "" || mb.Nick != "") {
			kept = append(kept, mb)
		}
	}
	sortMembers(kept)
	return kept
}

// MentionsIn lists the usernames text mentions, without the @, each once
// (ignoring case), in order. A mention is an @ at the start of the text or
// after a character that is no letter, digit, underscore or one of / . @ = &
// (so mail@example.com, имейл@example.com and youtube.com/@name are none),
// then a letter and 3 to 31 more letters, digits or underscores (Telegram
// usernames are 5 to 32 long, collectible ones 4), followed by the end or a
// character that cannot continue a username or a link path (not /). An @
// in a word that holds :// or starts with www. is part of a link, never a
// mention: Telegram reads the whole link as one.
func MentionsIn(text string) []string {
	word := func(c byte) bool {
		return c == '_' || '0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
	}
	letter := func(c byte) bool { return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' }
	var out []string
	seen := map[string]bool{}
	for i := 0; i < len(text); i++ {
		if text[i] != '@' {
			continue
		}
		if i > 0 {
			r, _ := utf8.DecodeLastRuneInString(text[:i])
			if r == '_' || r == '/' || r == '.' || r == '@' || r == '=' || r == '&' ||
				unicode.IsLetter(r) || unicode.IsDigit(r) {
				continue
			}
		}
		j := i + 1
		for j < len(text) && word(text[j]) {
			j++
		}
		nick := text[i+1 : j]
		if len(nick) < 4 || len(nick) > 32 || !letter(nick[0]) {
			continue
		}
		if j < len(text) {
			if r, _ := utf8.DecodeRuneInString(text[j:]); r == '/' || unicode.IsLetter(r) || unicode.IsDigit(r) {
				continue
			}
		}
		if inLink(text, i) {
			continue
		}
		if k := strings.ToLower(nick); !seen[k] {
			seen[k] = true
			out = append(out, nick)
		}
		i = j - 1
	}
	return out
}

// inLink reports whether the space-separated word around text[i] is a
// link: it holds :// or starts with www.
func inLink(text string, i int) bool {
	start := 0
	if k := strings.LastIndexFunc(text[:i], unicode.IsSpace); k >= 0 {
		_, n := utf8.DecodeRuneInString(text[k:]) // a space may take more than a byte
		start = k + n
	}
	end := len(text)
	if k := strings.IndexFunc(text[i:], unicode.IsSpace); k >= 0 {
		end = i + k
	}
	w := strings.ToLower(text[start:end])
	return strings.Contains(w, "://") || strings.HasPrefix(w, "www.")
}

// Member lookup errors.
var (
	ErrNoMember        = errors.New("no such member")
	ErrAmbiguousMember = errors.New("ambiguous member")
)

// ResolveMember finds one member by nick or display name, ignoring case:
// exact name, then exact nick (with a leading @ the other way round), then
// a unique prefix of either, then a unique substring of the name, then the
// same in either script: a unique prefix of either (what Tab completes:
// "iv" finds "Иван"), then a unique substring of the name (as
// ResolveChat). The either-script tiers come after the plain ones, so they
// never make a plain match ambiguous. The order makes @nick resolve to the
// member with that username and a name to the one shown with it, even when
// one's name is the other's nick. A member known only by a mentioned nick
// has no name to match.
func ResolveMember(ms []Member, query string) (Member, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	at := strings.HasPrefix(q, "@")
	q = strings.TrimPrefix(q, "@")
	if q == "" {
		return Member{}, ErrNoMember
	}
	byName := func(m Member) bool { return strings.ToLower(m.Name) == q }
	byNick := func(m Member) bool { return m.Nick != "" && strings.ToLower(m.Nick) == q }
	tiers := []func(Member) bool{byName, byNick}
	if at {
		tiers = []func(Member) bool{byNick, byName}
	}
	tiers = append(tiers,
		func(m Member) bool {
			return m.Nick != "" && strings.HasPrefix(strings.ToLower(m.Nick), q) ||
				strings.HasPrefix(strings.ToLower(m.Name), q)
		},
		func(m Member) bool { return strings.Contains(strings.ToLower(m.Name), q) },
	)
	// The either-script tiers come last, so they never make a plain match
	// ambiguous, and a query that transliterates to nothing (combining
	// marks only) gets none: it would match everyone.
	if either := NewMatcher(q); !either.Empty() {
		tiers = append(tiers,
			func(m Member) bool { // what Tab completes: a prefix in either script
				return m.Nick != "" && PrefixMatch(m.Nick, q) || PrefixMatch(m.Name, q)
			},
			func(m Member) bool { return either.Match(m.Name) },
		)
	}
	for _, match := range tiers {
		var hits []Member
		for _, m := range ms {
			if match(m) {
				hits = append(hits, m)
			}
		}
		switch len(hits) {
		case 1:
			return hits[0], nil
		case 0:
		default:
			return Member{}, fmt.Errorf("%w: %q matches %d members", ErrAmbiguousMember, query, len(hits))
		}
	}
	return Member{}, fmt.Errorf("%w: %q", ErrNoMember, query)
}
