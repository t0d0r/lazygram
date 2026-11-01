package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestTranslitBulgarianRussianUkrainian(t *testing.T) {
	cases := map[string]string{
		"Иван":                "ivan",
		"Щастие":              "shtastie",
		"Българско Юнско":     "balgarsko yunsko",
		"Ёлка и щи":           "yolka i shti",
		"Київ Їжак Ґанок":     "kiyiv yizhak ganok",
		"Latin stays 42":      "latin stays 42",
		"ЦЯЛ ДЕН":             "tsyal den",
		"Быт Экономика Євген": "byt ekonomika yevgen",
		"ѝ":                   "i",
		"Ива́н":               "ivan",  // a stress mark is dropped
		"Ўзбек":               "ўzbek", // a letter outside the table passes through
	}
	for in, want := range cases {
		if got := Translit(in); got != want {
			t.Errorf("Translit(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMatchTextCrossesScripts(t *testing.T) {
	cyr := "Среща утре в София с Иван"
	for _, q := range []string{"иван", "Ivan", "sofiya", "СОФИЯ", "sreshta utre", "утре в", ""} {
		if !MatchText(cyr, q) {
			t.Errorf("%q should match %q", q, cyr)
		}
	}
	for _, q := range []string{"petar", "Петър", "sofia", "sofiya x"} { // sofia: ya is the only spelling of the Cyrillic ya
		if MatchText(cyr, q) {
			t.Errorf("%q should not match %q", q, cyr)
		}
	}
	lat := "Meeting with Ivan tomorrow"
	if !MatchText(lat, "иван") || !MatchText(lat, "Ivan") || MatchText(lat, "петър") {
		t.Error("a Cyrillic query finds Latin text")
	}
}

func TestMatcherAgreesWithTheSlowPath(t *testing.T) {
	hays := []string{"Среща утре в София", "Meeting with Ivan", "mixed Иван and Ivan", "", "Ива́н"}
	queries := []string{"", "ivan", "Иван", "sreshta", "ivan and", "x", "софия", "иван"}
	for _, q := range queries {
		mt := NewMatcher(q)
		for _, h := range hays {
			slow := q == "" || strings.Contains(strings.ToLower(h), strings.ToLower(strings.TrimSpace(q))) ||
				strings.Contains(Translit(h), Translit(strings.TrimSpace(q)))
			if got := mt.Match(h); got != slow {
				t.Errorf("Match(%q, %q) = %v, the full comparison says %v", h, q, got, slow)
			}
		}
	}
}

func TestResolveChatInEitherScript(t *testing.T) {
	chats := []Chat{
		{ID: "c1", Alias: "alice", Title: "Alice Smith", Kind: ChatUser},
		{ID: "c9", Alias: "ivan", Title: "Иван Петров", Kind: ChatUser},
		{ID: "c8", Alias: "anna", Title: "Анна", Kind: ChatUser},
	}
	for _, q := range []string{"petrov", "@petrov", "петров", "иван"} {
		if c, err := ResolveChat(chats, q); err != nil || c.ID != "c9" {
			t.Errorf("%q: %v %v", q, c.ID, err)
		}
	}
	// A plain substring match wins and is not made ambiguous by the
	// transliterated tier: "a" is in "alice" and "anna" as typed already,
	// so it stays ambiguous as before; "smith" is plain and unique.
	if c, err := ResolveChat(chats, "smith"); err != nil || c.ID != "c1" {
		t.Errorf("smith: %v %v", c.ID, err)
	}
	if _, err := ResolveChat(chats, "a"); !errors.Is(err, ErrAmbiguous) {
		t.Errorf("a: %v", err)
	}
	// A Cyrillic query finds a Latin title when its transliteration spells
	// it ("смит" is "smit", in "Smith"); a phonetic spelling that does not
	// ("елис" is "elis", not in "Alice") is no match, since Latin is never
	// transliterated back.
	if c, err := ResolveChat(chats, "смит"); err != nil || c.ID != "c1" {
		t.Errorf("смит: %v %v", c.ID, err)
	}
	if _, err := ResolveChat(chats, "елис"); !errors.Is(err, ErrNoChat) {
		t.Errorf("елис: %v", err)
	}
	// A query of combining marks only transliterates to nothing; it must
	// not match every chat of its kind.
	for _, q := range []string{"́", "@́", "#́"} {
		if c, err := ResolveChat(chats, q); !errors.Is(err, ErrNoChat) {
			t.Errorf("%q: %v %v", q, c.ID, err)
		}
	}
}

func TestFilterSearchesBothScripts(t *testing.T) {
	at := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	msgs := []Message{
		{ChatID: "c1", ID: "1", Sender: "Иван", Text: "Добро утро", Time: at},
		{ChatID: "c1", ID: "2", Sender: "ada", Text: "good morning", Time: at.Add(time.Minute)},
	}
	if got := Filter(msgs, Query{Text: "dobro"}); len(got) != 1 || got[0].ID != "1" {
		t.Fatalf("Latin query over Cyrillic text: %v", got)
	}
	if got := Filter(msgs, Query{Text: "ivan"}); len(got) != 1 || got[0].ID != "1" {
		t.Fatalf("Latin query over a Cyrillic sender: %v", got)
	}
	if got := Filter(msgs, Query{Text: "гуд"}); len(got) != 0 {
		t.Fatalf("a Cyrillic spelling of a Latin word is not a transliteration match: %v", got)
	}
	if got := Filter(msgs, Query{Text: "morning"}); len(got) != 1 || got[0].ID != "2" {
		t.Fatalf("plain match still works: %v", got)
	}
}
