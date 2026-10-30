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

func TestResolveChat(t *testing.T) {
	chats := []Chat{
		{ID: "1", Alias: "dev", Title: "Dev Talk"},
		{ID: "2", Alias: "devops", Title: "Ops"},
		{ID: "3", Alias: "family", Title: "Home"},
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
