package domain

import (
	"errors"
	"testing"
	"time"
)

var t0 = time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC)

func msg(chat, id string, min int, sender, text string) Message {
	return Message{ID: id, ChatID: chat, Sender: sender, Text: text, Time: t0.Add(time.Duration(min) * time.Minute)}
}

func ids(ms []Message) string {
	s := ""
	for _, m := range ms {
		s += m.ChatID + m.ID + " "
	}
	return s
}

func TestMergeInterleavesChronologically(t *testing.T) {
	a := []Message{msg("a", "1", 0, "x", ""), msg("a", "2", 4, "x", "")}
	b := []Message{msg("b", "1", 2, "y", ""), msg("b", "2", 6, "y", "")}
	got := ids(Merge(a, b))
	if want := "a1 b1 a2 b2 "; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestMergeDedupesAndBreaksTies(t *testing.T) {
	a := []Message{msg("b", "1", 1, "x", "first"), msg("a", "1", 1, "x", "")}
	b := []Message{msg("b", "1", 1, "x", "dup")}
	got := Merge(a, b)
	if len(got) != 2 {
		t.Fatalf("want 2 messages, got %d", len(got))
	}
	if ids(got) != "a1 b1 " {
		t.Fatalf("tie order: %q", ids(got))
	}
	if got[1].Text != "first" {
		t.Fatalf("duplicate should keep first seen, got %q", got[1].Text)
	}
}

func TestFilter(t *testing.T) {
	ms := []Message{
		msg("a", "1", 0, "alice", "Deploy done"),
		msg("b", "1", 1, "bob", "lunch?"),
		msg("a", "2", 2, "carol", "deploy failed"),
	}
	cases := []struct {
		name string
		q    Query
		want string
	}{
		{"all", Query{}, "a1 b1 a2 "},
		{"chat", Query{ChatID: "a"}, "a1 a2 "},
		{"text ci", Query{Text: "DEPLOY"}, "a1 a2 "},
		{"sender", Query{Text: "bob"}, "b1 "},
		{"chat+text", Query{ChatID: "a", Text: "failed"}, "a2 "},
		{"since", Query{Since: t0.Add(time.Minute)}, "b1 a2 "},
		{"none", Query{Text: "zzz"}, ""},
	}
	for _, c := range cases {
		if got := ids(Filter(ms, c.q)); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestTailAndLatest(t *testing.T) {
	ms := []Message{msg("a", "1", 0, "", ""), msg("a", "2", 5, "", ""), msg("a", "3", 3, "", "")}
	if got := ids(Tail(ms, 2)); got != "a2 a3 " {
		t.Fatalf("tail: %q", got)
	}
	if len(Tail(ms, 10)) != 3 || len(Tail(ms, 0)) != 0 {
		t.Fatal("tail bounds")
	}
	if !Latest(ms).Equal(t0.Add(5 * time.Minute)) {
		t.Fatal("latest")
	}
	if !Latest(nil).IsZero() {
		t.Fatal("latest of empty")
	}
}

func TestChatName(t *testing.T) {
	if (Chat{Alias: "ada", Kind: ChatUser}).Name() != "@ada" || (Chat{Alias: "dev", Kind: ChatGroup}).Name() != "#dev" ||
		(Chat{Alias: "news", Kind: ChatChannel}).Name() != "#news" {
		t.Fatal("names")
	}
}

func TestResolveChatSigils(t *testing.T) {
	chats := []Chat{
		{ID: "u", Alias: "dev", Kind: ChatUser, Title: "Dev Person"},
		{ID: "g", Alias: "dev", Kind: ChatGroup, Title: "Dev Group"},
	}
	if _, err := ResolveChat(chats, "dev"); !errors.Is(err, ErrAmbiguous) {
		t.Fatalf("no sigil: %v", err)
	}
	if c, err := ResolveChat(chats, "@dev"); err != nil || c.ID != "u" {
		t.Fatalf("@: %v %v", c, err)
	}
	if c, err := ResolveChat(chats, "#DEV"); err != nil || c.ID != "g" {
		t.Fatalf("#: %v %v", c, err)
	}
	if _, err := ResolveChat(chats, "@"); !errors.Is(err, ErrNoChat) {
		t.Fatalf("bare sigil: %v", err)
	}
}

func TestResolveChat(t *testing.T) {
	chats := []Chat{
		{ID: "1", Alias: "dev", Title: "Dev Talk", Kind: ChatGroup},
		{ID: "2", Alias: "devops", Title: "Ops", Kind: ChatGroup},
		{ID: "3", Alias: "family", Title: "Home", Kind: ChatGroup},
	}
	ok := map[string]string{"dev": "1", "#DEVOPS": "2", "3": "3", "fam": "family", "home": "family"}
	for q, want := range ok {
		c, err := ResolveChat(chats, q)
		if err != nil || (c.ID != want && c.Alias != want) {
			t.Errorf("%q: got %v, %v", q, c, err)
		}
	}
	if _, err := ResolveChat(chats, "de"); !errors.Is(err, ErrAmbiguous) {
		t.Errorf("want ambiguous, got %v", err)
	}
	if _, err := ResolveChat(chats, "nope"); !errors.Is(err, ErrNoChat) {
		t.Errorf("want no chat, got %v", err)
	}
	if _, err := ResolveChat(chats, " "); !errors.Is(err, ErrNoChat) {
		t.Errorf("empty query: %v", err)
	}
}

func TestFilterChats(t *testing.T) {
	chats := []Chat{{ID: "1", Alias: "dev"}, {ID: "2", Alias: "ops", Title: "Developers"}}
	if len(FilterChats(chats, "dev")) != 2 || len(FilterChats(chats, "")) != 2 || len(FilterChats(chats, "ops")) != 1 {
		t.Fatal("FilterChats")
	}
}

func TestReplaceAndLatestWithButtons(t *testing.T) {
	ms := []Message{msg("a", "1", 0, "bot", "pick"), msg("b", "1", 1, "x", "")}
	ms[0].Buttons = []Button{{Label: "OK", Kind: ButtonCallback}}
	got, ok := LatestWithButtons(ms, "a")
	if !ok || got.ID != "1" {
		t.Fatal("latest with buttons")
	}
	if _, ok := LatestWithButtons(ms, "b"); ok {
		t.Fatal("chat b has none")
	}
	edited := msg("a", "1", 0, "bot", "done")
	ms = Replace(ms, edited)
	if len(ms) != 2 || ms[0].Text != "done" || len(ms[0].Buttons) != 0 {
		t.Fatalf("replace: %+v", ms)
	}
	ms = Replace(ms, msg("c", "9", 5, "y", "new"))
	if len(ms) != 3 || ms[2].ChatID != "c" {
		t.Fatalf("replace appends unknown: %+v", ms)
	}
}

func TestLastOwnAndRemove(t *testing.T) {
	ms := []Message{msg("a", "1", 0, "me", "mine"), msg("a", "2", 1, "x", "theirs"), msg("a", "3", 2, "me", "mine too"), msg("b", "1", 3, "me", "other chat")}
	got, ok := LastOwn(ms, "a")
	if !ok || got.ID != "3" {
		t.Fatalf("%+v %v", got, ok)
	}
	if _, ok := LastOwn(ms, "c"); ok {
		t.Fatal("none")
	}
	ms = Remove(ms, "a", "3")
	if len(ms) != 3 || ms[2].ChatID != "b" {
		t.Fatalf("remove: %+v", ms)
	}
	if len(Remove(ms, "a", "nope")) != 3 {
		t.Fatal("remove of unknown is a no-op")
	}
}
