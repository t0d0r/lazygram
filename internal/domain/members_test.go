package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func said(chat, id string, min int, sender, senderID, nick string) Message {
	return Message{ID: id, ChatID: chat, Sender: sender, SenderID: senderID, SenderNick: nick,
		Time: time.Date(2026, 1, 2, 3, 0, 0, 0, time.UTC).Add(time.Duration(min) * time.Minute)}
}

func TestMembersAreTheDistinctSenders(t *testing.T) {
	ms := Members([]Message{
		said("g", "1", 0, "Al", "u1", ""),
		said("g", "2", 5, "Alice", "u1", "alice"), // renamed, got a username: one member
		said("g", "3", 1, "Alice", "", ""),        // cached without an id: counts for u1
		said("g", "4", 2, "Bob", "", ""),          // no id at all: by name
		said("g", "5", 3, "me", "", ""),           // own messages are no member
		said("g", "6", 4, "channel", "", ""),      // an unsigned channel post neither
		said("h", "7", 9, "Bob", "u9", "bobby"),   // another chat
	}, "g")
	if len(ms) != 2 {
		t.Fatalf("members: %+v", ms)
	}
	a, b := ms[0], ms[1]
	if a.ID != "u1" || a.Name != "Alice" || a.Nick != "alice" || a.Count != 3 || a.Last.Minute() != 5 {
		t.Errorf("first, most recent: %+v", a)
	}
	if b.Name != "Bob" || b.ID != "" || b.Nick != "" || b.Count != 1 {
		t.Errorf("second: %+v", b)
	}
}

func TestMembersKeepNamesakesApart(t *testing.T) {
	ms := Members([]Message{
		said("g", "1", 0, "Sam", "u1", ""),
		said("g", "2", 1, "Sam", "u2", ""),
		said("g", "3", 2, "Sam", "", ""), // which Sam? neither
		said("g", "4", 3, "channel", "u3", "chan"),
	}, "g")
	if len(ms) != 4 {
		t.Fatalf("two Sams by id, one by name, a user named channel: %+v", ms)
	}
	if ms := Members(nil, "g"); len(ms) != 0 {
		t.Fatalf("no messages, no members: %+v", ms)
	}
}

func TestResolveMember(t *testing.T) {
	ms := []Member{
		{Name: "Alice Smith", Nick: "alice_s", ID: "u1"},
		{Name: "Alan", Nick: "", ID: "u2"},
		{Name: "Иван Петров", Nick: "", ID: "u3"},
		{Name: "Bob", Nick: "alan_b"},
	}
	for q, want := range map[string]string{
		"@alice_s":    "u1",
		"ALICE SMITH": "u1",
		"alic":        "u1", // unique prefix of the nick and the name
		"alan":        "u2", // exact name beats the nick prefix alan_b
		"alan_":       "",   // Bob, by the nick prefix
		"smith":       "u1", // substring of the name
		"ivan":        "u3", // either script
		"петр":        "u3",
	} {
		got, err := ResolveMember(ms, q)
		if err != nil || got.ID != want {
			t.Errorf("%q: got %+v, %v; want %q", q, got, err, want)
		}
	}
	if _, err := ResolveMember(ms, "al"); !errors.Is(err, ErrAmbiguousMember) {
		t.Errorf("al is three members: %v", err)
	}
	for _, q := range []string{"zed", "", "@", "#dev"} {
		if _, err := ResolveMember(ms, q); !errors.Is(err, ErrNoMember) {
			t.Errorf("%q: %v", q, err)
		}
	}
}

func TestMembersFollowTheNewestMessage(t *testing.T) {
	ms := Members([]Message{
		said("g", "2", 5, "Alice", "u1", ""), // dropped the username since
		said("g", "1", 0, "Alice", "u1", "alice_old"),
	}, "g")
	if len(ms) != 1 || ms[0].Nick != "" || ms[0].Name != "Alice" || ms[0].Count != 2 {
		t.Fatalf("the newest message has no username: %+v", ms)
	}
}

func TestMembersTellSentinelsFromPeopleByID(t *testing.T) {
	ms := Members([]Message{
		said("g", "1", 0, "me", "u42", ""),     // a person called me
		said("g", "2", 1, "channel", "u3", ""), // a person called channel
		said("g", "3", 2, "channel", "", ""),   // an unsigned post: not theirs
		said("g", "4", 3, "me", "", ""),        // own message
	}, "g")
	if len(ms) != 2 {
		t.Fatalf("two people, no sentinels: %+v", ms)
	}
	for _, m := range ms {
		if m.Count != 1 {
			t.Errorf("%q counts %d messages", m.Name, m.Count)
		}
	}
}

func TestResolveMemberMatchesTheCompletionCandidates(t *testing.T) {
	ms := []Member{
		{Name: "Alan Turing", Nick: "alan", ID: "u1"},
		{Name: "alan", ID: "u2"}, // shown as alan, no username
	}
	for q, want := range map[string]string{"@alan": "u1", "alan": "u2", "Alan T": "u1", "@ALAN": "u1"} {
		if got, err := ResolveMember(ms, q); err != nil || got.ID != want {
			t.Errorf("%q: got %+v, %v; want %s", q, got, err, want)
		}
	}
}

func TestResolveMemberPrefixInEitherScript(t *testing.T) {
	ms := []Member{
		{Name: "Иван Петров", ID: "u1"},
		{Name: "Dobri Ivanov", ID: "u2"}, // "iva" inside, not at the start
		{Name: "Stoyan", Nick: "stoyan_k", ID: "u3"},
	}
	for q, want := range map[string]string{
		"iva":  "u2", // the plain substring comes before either script
		"ива":  "u1",
		"сто":  "u3",
		"@сто": "u3",
		"petr": "u1", // a substring in either script
		"dob":  "u2",
	} {
		if got, err := ResolveMember(ms, q); err != nil || got.ID != want {
			t.Errorf("%q: got %+v, %v; want %s", q, got, err, want)
		}
	}
}

func TestResolveMemberEitherScriptNeverBeatsPlain(t *testing.T) {
	ms := []Member{{Name: "Olivia", ID: "u1"}, {Name: "Иван", ID: "u2"}, {Name: "Ивайло", ID: "u3"}}
	if got, err := ResolveMember(ms, "iv"); err != nil || got.ID != "u1" {
		t.Errorf("iv is Olivia, as typed: %+v, %v", got, err)
	}
	if _, err := ResolveMember(ms, "́"); !errors.Is(err, ErrNoMember) {
		t.Errorf("combining marks only resolve nobody: %v", err)
	}
}

func TestMembersCountMentions(t *testing.T) {
	text := func(m Message, s string) Message { m.Text = s; return m }
	ms := Members([]Message{
		text(said("g", "1", 0, "Alice", "u1", "alice_s"), "hi"),
		text(said("g", "2", 1, "Bob", "u2", ""), "happy birthday @Grace_H and @ALICE_S"),
		text(said("g", "3", 2, "me", "", ""), "@grace_h, @grace_h!"), // own messages mention too; once each
		text(said("h", "4", 3, "Bob", "u2", ""), "@elsewhere in another chat"),
	}, "g")
	byNick := map[string]Member{}
	for _, m := range ms {
		byNick[strings.ToLower(m.Nick)] = m
	}
	if len(ms) != 3 || byNick["elsewhere"].Mentions != 0 {
		t.Fatalf("Alice, Bob and grace_h: %+v", ms)
	}
	if a := byNick["alice_s"]; a.Name != "Alice" || a.Count != 1 || a.Mentions != 1 || a.Last.Minute() != 0 {
		t.Errorf("a sender mentioned keeps her name and her own last message: %+v", a)
	}
	if g := byNick["grace_h"]; g.Name != "" || g.Count != 0 || g.Mentions != 2 || g.Nick != "grace_h" || g.Last.Minute() != 2 {
		t.Errorf("a nick only mentioned, the newest spelling: %+v", g)
	}
}

func TestMentionsIn(t *testing.T) {
	for text, want := range map[string]string{
		"@petar_1 happy birthday!":                                    "petar_1",
		"thanks, @Ada_L. And @ada_l again":                            "Ada_L",
		"mail@example.com and @ab and @1abc":                          "",
		"(@grace_h) @@lead_x":                                         "grace_h", // not after another @
		"new video https://www.youtube.com/@Some_Channel":             "",
		"see medium.com/@writer_x/post and mastodon.social/@user_one": "",
		"www.example.com/@path_one":                                   "",
		"pishi na имейл@primer.com":                                   "",
		"@nickя, \u00ab@quoted_one\u00bb, \U0001F389@party_one":       "quoted_one,party_one",
		"@" + strings.Repeat("a", 33):                                 "", // too long
		"":                                                            "",
	} {
		if got := strings.Join(MentionsIn(text), ","); got != want {
			t.Errorf("MentionsIn(%q) = %q, want %q", text, got, want)
		}
	}
}

func TestMembersGiveAMovedUsernameToItsNewestHolder(t *testing.T) {
	text := func(m Message, s string) Message { m.Text = s; return m }
	for run := 0; run < 20; run++ { // map order must not matter
		ms := Members([]Message{
			said("g", "1", 0, "Old Owner", "u1", "Robin_X"),
			said("g", "2", 5, "New Owner", "u2", "robin_x"),
			text(said("g", "3", 6, "Bob", "u3", ""), "hey @robin_x"),
		}, "g")
		byID := map[string]Member{}
		for _, m := range ms {
			byID[m.ID] = m
		}
		if byID["u1"].Nick != "" || byID["u2"].Nick != "robin_x" || byID["u2"].Mentions != 1 || len(ms) != 3 {
			t.Fatalf("the newest holder keeps the nick and its mentions: %+v", ms)
		}
		if got, err := ResolveMember(ms, "@robin_x"); err != nil || got.ID != "u2" {
			t.Fatalf("the nick resolves to one person: %+v, %v", got, err)
		}
	}
}

func TestWithParticipants(t *testing.T) {
	text := func(m Message, s string) Message { m.Text = s; return m }
	seen := Members([]Message{
		said("g", "1", 0, "Alice", "u1", "alice_old"), // renamed her username since
		said("g", "2", 1, "Bob", "", ""),              // cached without an id
		text(said("g", "3", 2, "Carol", "u3", "carol_c"), "hi @dave_d"),
		said("g", "4", 3, "Erin", "u5", "taken_x"), // held taken_x once; Frank holds it now
	}, "g")
	ms := WithParticipants(seen, []Participant{
		{ID: "u1", Name: "Alice A.", Nick: "alice_new"},
		{ID: "u2", Name: "Bob", Nick: "bob_b"}, // the one sender known by this name alone
		{ID: "u4", Name: "Dave", Nick: "dave_d"},
		{ID: "u6", Name: "Frank", Nick: "taken_x"},
		{ID: "u7", Name: "Gina"}, // never wrote, no username
	})
	byID := map[string]Member{}
	for _, m := range ms {
		byID[m.ID] = m
	}
	if len(ms) != 7 {
		t.Fatalf("seven people: %+v", ms)
	}
	if a := byID["u1"]; a.Name != "Alice A." || a.Nick != "alice_new" || a.Count != 1 || !a.Listed {
		t.Errorf("the list's name and username win: %+v", a)
	}
	if b := byID["u2"]; b.Name != "Bob" || b.Nick != "bob_b" || b.Count != 1 {
		t.Errorf("a sender cached without an id gets it by name: %+v", b)
	}
	if d := byID["u4"]; d.Name != "Dave" || d.Mentions != 1 || d.Count != 0 {
		t.Errorf("a nick only mentioned gets its name: %+v", d)
	}
	if e := byID["u5"]; e.Nick != "" || e.Listed {
		t.Errorf("a username someone else holds now is gone: %+v", e)
	}
	if g := byID["u7"]; g.Name != "Gina" || g.Nick != "" || !g.Listed || ms[len(ms)-1].ID != "u7" && ms[len(ms)-2].ID != "u7" {
		t.Errorf("a listed member who never wrote comes last: %+v", ms)
	}
	if again := WithParticipants(seen, nil); len(again) != len(seen) {
		t.Errorf("no list, no change")
	}
}

func TestWithParticipantsKeepsPeopleWhole(t *testing.T) {
	text := func(m Message, s string) Message { m.Text = s; return m }
	// Two listed people share a name; the old line by that name is neither's for sure.
	ms := WithParticipants(Members([]Message{said("g", "1", 0, "Sam", "", "")}, "g"),
		[]Participant{{ID: "u1", Name: "Sam", Nick: "sam_one"}, {ID: "u2", Name: "Sam", Nick: "sam_two"}})
	nicks := map[string]bool{}
	for _, m := range ms {
		nicks[m.Nick] = true
	}
	if !nicks["sam_one"] || !nicks["sam_two"] {
		t.Errorf("both listed Sams stay: %+v", ms)
	}
	// One person seen two ways (an old line by name, a mention) and listed: one member.
	ms = WithParticipants(Members([]Message{
		said("g", "1", 0, "Bob", "", ""),
		text(said("g", "2", 1, "Carol", "u3", ""), "thanks @bob_b"),
	}, "g"), []Participant{{ID: "u2", Name: "Bob", Nick: "bob_b"}})
	bobs := 0
	for _, m := range ms {
		if m.Name == "Bob" {
			bobs++
			if m.ID != "u2" || m.Count != 1 || m.Mentions != 1 || m.Last.Minute() != 0 {
				t.Errorf("Bob whole: %+v", m)
			}
		}
	}
	if bobs != 1 {
		t.Errorf("one Bob: %+v", ms)
	}
	if got, err := ResolveMember(ms, "Bob"); err != nil || got.ID != "u2" {
		t.Errorf("/whois Bob finds him: %+v %v", got, err)
	}
	// The list gives Alice a username the messages only mention: the mentions are hers.
	ms = WithParticipants(Members([]Message{
		said("g", "1", 0, "Alice", "u1", "alice_old"),
		text(said("g", "2", 1, "Carol", "u3", ""), "@alice_new hi"),
		text(said("g", "3", 2, "Carol", "u3", ""), "@alice_new again"),
	}, "g"), []Participant{{ID: "u1", Name: "Alice", Nick: "alice_new"}})
	for _, m := range ms {
		if m.Nick == "alice_new" && (m.ID != "u1" || m.Mentions != 2 || m.Count != 1) {
			t.Errorf("Alice keeps her mentions: %+v", m)
		}
		if m.Name == "" {
			t.Errorf("no nick-only leftover: %+v", ms)
		}
	}
}
