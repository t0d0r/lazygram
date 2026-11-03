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

// A Latin spelling is as decisive as the Cyrillic one once no plain tier
// matches: an exact alias or title in either script resolves before a
// prefix, and a prefix before a substring, so "ivan" is иван although
// иванка starts with it too.
func TestResolveChatExactInEitherScript(t *testing.T) {
	chats := []Chat{
		{ID: "c1", Alias: "иван", Title: "Иван", Kind: ChatUser},
		{ID: "c2", Alias: "иванка", Title: "Иванка", Kind: ChatUser},
		{ID: "c3", Alias: "новини", Title: "Новини", Kind: ChatChannel},
		{ID: "c4", Alias: "новини-спорт", Title: "Новини Спорт", Kind: ChatChannel},
		{ID: "c5", Alias: "stuckoff", Title: "Камен", Kind: ChatUser},
		{ID: "c6", Alias: "каменов", Title: "Каменов", Kind: ChatUser},
	}
	for q, want := range map[string]string{
		"иван": "c1", "ivan": "c1", "@ivan": "c1", "IVAN": "c1",
		"новини": "c3", "novini": "c3", "#novini": "c3",
		"kamen": "c5", "@kamen": "c5", "kamenov": "c6",
		"ivanka": "c2", "novini sport": "c4",
	} {
		if c, err := ResolveChat(chats, q); err != nil || c.ID != want {
			t.Errorf("%q: got %q, %v; want %s", q, c.ID, err, want)
		}
	}
	if _, err := ResolveChat(chats, "iva"); !errors.Is(err, ErrAmbiguous) {
		t.Errorf("iva begins both: %v", err)
	}
	// The alias alone is exact when the title spells something else, as
	// stuckoff's title alone is above.
	chats = []Chat{
		{ID: "c1", Alias: "иван", Title: "Ванката", Kind: ChatUser},
		{ID: "c2", Alias: "иванка", Title: "Иванка", Kind: ChatUser},
	}
	if c, err := ResolveChat(chats, "ivan"); err != nil || c.ID != "c1" {
		t.Errorf("ivan, the exact alias: got %q, %v", c.ID, err)
	}
}

// The plain tiers come before those in either script: a query that begins
// an alias or is inside one as typed finds that chat even when it is
// another chat's exact name in either script. So a Latin alias kamenov
// takes "kamen" from the chat titled Камен, which the Cyrillic spelling
// still finds by its exact title.
func TestResolveChatPlainTiersComeFirst(t *testing.T) {
	for _, tc := range []struct {
		chats []Chat
		want  map[string]string
	}{
		{[]Chat{
			{ID: "c1", Alias: "stuckoff", Title: "Камен", Kind: ChatUser},
			{ID: "c2", Alias: "kamenov", Title: "Каменов", Kind: ChatUser},
		}, map[string]string{"kamen": "c2", "@kamen": "c2", "камен": "c1", "@Камен": "c1"}},
		{[]Chat{
			{ID: "c1", Alias: "the-ivan-show", Title: "Show", Kind: ChatChannel},
			{ID: "c2", Alias: "иван", Title: "Иван", Kind: ChatUser},
		}, map[string]string{"ivan": "c1", "иван": "c2", "@ivan": "c2"}},
	} {
		for q, want := range tc.want {
			if c, err := ResolveChat(tc.chats, q); err != nil || c.ID != want {
				t.Errorf("%q: got %q, %v; want %s", q, c.ID, err, want)
			}
		}
	}
}

// A unique prefix in either script resolves where the substring would be
// ambiguous: "kalo" begins one title and is inside another.
func TestResolveChatPrefixInEitherScript(t *testing.T) {
	chats := []Chat{
		{ID: "c1", Alias: "калоян-gps", Title: "Калоян GPS", Kind: ChatUser},
		{ID: "c2", Alias: "свети-калоян", Title: "Свети Калоян", Kind: ChatGroup},
	}
	for _, q := range []string{"kalo", "@kalo", "kaloyan g"} {
		if c, err := ResolveChat(chats, q); err != nil || c.ID != "c1" {
			t.Errorf("%q: got %q, %v", q, c.ID, err)
		}
	}
	if c, err := ResolveChat(chats, "#kalo"); err != nil || c.ID != "c2" {
		t.Errorf("#kalo: the sigil still picks the kind: %q, %v", c.ID, err)
	}
	// Either half alone: the title of a chat whose alias is an unrelated
	// username, and the alias of a chat whose title is unrelated.
	for _, chats := range [][]Chat{
		{
			{ID: "c1", Alias: "stuckoff", Title: "Калоян GPS", Kind: ChatUser},
			{ID: "c2", Alias: "sveti", Title: "Свети Калоян", Kind: ChatGroup},
		},
		{
			{ID: "c1", Alias: "калоян-gps", Title: "GPS", Kind: ChatUser},
			{ID: "c2", Alias: "свети-калоян", Title: "Church", Kind: ChatGroup},
		},
	} {
		if c, err := ResolveChat(chats, "kalo"); err != nil || c.ID != "c1" {
			t.Errorf("kalo begins %s %q only: got %q, %v", chats[0].Alias, chats[0].Title, c.ID, err)
		}
	}
}

func TestMatcherExactAndPrefix(t *testing.T) {
	mt := NewMatcher(" Kamen ")
	for s, want := range map[string][2]bool{
		"Камен": {true, true}, "kamen": {true, true}, "Каменов": {false, true},
		"Ками": {false, false}, "Стар Камен": {false, false},
	} {
		if got := [2]bool{mt.MatchExact(s), mt.MatchPrefix(s)}; got != want {
			t.Errorf("%q: exact, prefix = %v, want %v", s, got, want)
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

func TestPrefixMatch(t *testing.T) {
	for _, c := range []struct {
		s, prefix string
		want      bool
	}{
		{"Иван Петров", "iv", true},
		{"Иван Петров", "ИВ", true},
		{"Ivan", "ив", true},
		{"#новини", "#nov", true},
		{"Жельо", "zh", true},
		{"Иван Петров", "pet", false}, // a prefix, not a substring
		{"Gracelyn", "grace ", false},
		{"Grace Kelly", "grace ", true},
		{"anything", "", true},
	} {
		if got := PrefixMatch(c.s, c.prefix); got != c.want {
			t.Errorf("PrefixMatch(%q, %q) = %v", c.s, c.prefix, got)
		}
	}
}
