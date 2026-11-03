package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"lazygram/internal/domain"
)

// titleFake adds people whose aliases (Telegram usernames) say nothing of
// the titles the window bar shows by name, and a deleted account whose
// title begins like one of them.
func titleFake() *fake {
	f := newFake()
	f.chats = append(f.chats,
		domain.Chat{ID: "c11", Alias: "stuckoff", Title: "Камен", Kind: domain.ChatUser},
		domain.Chat{ID: "c12", Alias: "krasi-varna", Title: "Краси Варна", Kind: domain.ChatUser},
		domain.Chat{ID: "c13", Alias: "калоян-gps", Title: "Калоян GPS", Kind: domain.ChatUser},
		domain.Chat{ID: "c14", Alias: "калоян-симеонов", Title: "Калоян Симеонов", Kind: domain.ChatUser},
		domain.Chat{ID: "c15", Alias: "tsvetan_i", Title: "Цветан Иванов", Kind: domain.ChatUser},
		domain.Chat{ID: "c16", Alias: "deleted-16", Title: "Камелия", Kind: domain.ChatUser, Deleted: true},
	)
	return f
}

// The window bar shows @Камен by name; Tab offers it for what was typed of
// that title, in either script, fills it in as shown, and Enter finds it.
func TestTabCompletesTheTitleTheBarShows(t *testing.T) {
	m := setup(t, titleFake())
	enter(m, "/names")
	if got := tabbed(m, "/win @K"); got != "/win @K" {
		t.Fatalf("four people begin with K: %q", got)
	}
	press(m, tea.KeyTab)
	if l := noticeLine(body(m), "-!- @Калоян GPS"); !strings.HasSuffix(l, "-!- @Калоян GPS, @Калоян Симеонов, @Камен, @Краси Варна") {
		t.Fatalf("the second Tab lists the titles the bar shows: %q", l)
	}
	if got := tabbed(m, "/win @Ка"); got != "/win @Ка" {
		t.Fatalf("no common extension past @Ка: %q", got)
	}
	press(m, tea.KeyTab)
	if l := noticeLine(body(m), "-!- @Калоян GPS"); !strings.HasSuffix(l, "-!- @Калоян GPS, @Калоян Симеонов, @Камен") {
		t.Fatalf("listed: %q", l)
	}
	if got := tabbed(m, "/win @Кам"); got != "/win @Камен " { // not the deleted Камелия
		t.Fatalf("one title: %q", got)
	}
	press(m, tea.KeyEnter)
	if m.wins[m.cur].chatID != "c11" || m.winName(m.cur) != "@Камен" || m.isErr {
		t.Fatalf("Enter focuses the chat filled in: %q %q", m.wins[m.cur].chatID, m.status)
	}
	for typed, want := range map[string]string{
		"/win @kam":   "/win @Камен ", // Latin finds the Cyrillic title
		"/win kam":    "/win @Камен ", // the sigil is added
		"/win @stu":   "/win @Камен ", // the alias still matches; the name as shown is filled in
		"/whois @kam": "/whois @Камен ",
		"hi kam":      "hi @Камен ",
	} {
		if got := tabbed(m, typed); got != want {
			t.Errorf("%q: got %q, want %q", typed, got, want)
		}
	}
}

// By nick the alias is filled in, and the second Tab shows the title of a
// chat that matched by its title alone, as the member listing does.
func TestTabCompletesTitlesByNick(t *testing.T) {
	m := setup(t, titleFake())
	for typed, want := range map[string]string{
		"/win @kam": "/win @stuckoff ",
		"/win Кам":  "/win @stuckoff ",
		"/msg kam":  "/msg @stuckoff ",
		"hi кам":    "hi @stuckoff ",
		"/win #kam": "/win #kam", // a person, not a group
	} {
		if got := tabbed(m, typed); got != want {
			t.Errorf("%q: got %q, want %q", typed, got, want)
		}
	}
	if got := tabbed(m, "/win @k"); got != "/win @k" {
		t.Fatalf("several: %q", got)
	}
	press(m, tea.KeyTab)
	if l := noticeLine(body(m), "-!- @krasi-varna"); !strings.HasSuffix(l, "-!- @krasi-varna, @stuckoff (Камен), @калоян-gps, @калоян-симеонов") {
		t.Fatalf("the title after the alias it matched by: %q", l)
	}
	if got := tabbed(m, "/win #"); got != "/win #" {
		t.Fatalf("#dev, #news: %q", got)
	}
	press(m, tea.KeyTab)
	if l := noticeLine(body(m), "-!- #dev"); !strings.HasSuffix(l, "-!- #dev  #news") {
		t.Fatalf("no title shown, the old separator: %q", l)
	}
}

// sharedFake adds two people with the same title, a title that is another
// chat's alias, and a title with a control character, which /names shows
// without it.
func sharedFake() *fake {
	f := titleFake()
	f.chats = append(f.chats,
		domain.Chat{ID: "c21", Alias: "vanko", Title: "Иван", Kind: domain.ChatUser},
		domain.Chat{ID: "c22", Alias: "ivan_p", Title: "Иван", Kind: domain.ChatUser},
		domain.Chat{ID: "c23", Alias: "bob", Title: "Robert", Kind: domain.ChatUser},
		domain.Chat{ID: "c24", Alias: "r1", Title: "bob", Kind: domain.ChatUser},
		domain.Chat{ID: "c25", Alias: "bell1", Title: "Ri\x07ng", Kind: domain.ChatUser},
		domain.Chat{ID: "c26", Alias: "devguy", Title: "Dev", Kind: domain.ChatUser},
	)
	return f
}

// By name a chat whose shown name would find another chat, or none, is
// offered by its alias, so what Tab fills in always finds it.
func TestTitlesThatResolveElsewhereCompleteByAlias(t *testing.T) {
	m := setup(t, sharedFake())
	enter(m, "/names")
	if got := tabbed(m, "/win @Ив"); got != "/win @Ив" {
		t.Fatalf("two Ivans: %q", got)
	}
	press(m, tea.KeyTab)
	if l := noticeLine(body(m), "-!- @ivan_p"); !strings.HasSuffix(l, "-!- @ivan_p, @vanko (Иван)") {
		t.Fatalf("a shared title is offered by alias: %q", l)
	}
	if got := tabbed(m, "/win @bo"); got != "/win @bo" {
		t.Fatalf("two: %q", got)
	}
	press(m, tea.KeyTab)
	if l := noticeLine(body(m), "-!- @Robert"); !strings.HasSuffix(l, "-!- @Robert, @r1 (bob)") {
		t.Fatalf("@bob is the alias of @Robert, so the chat titled bob is @r1: %q", l)
	}
	for typed, want := range map[string]string{
		"/win @Ri":  "/win @bell1 ", // shown as @Ring, which is not its title
		"/win @Dev": "/win @Dev ",   // a person; #Dev is the group
		"/win #De":  "/win #Dev ",
	} {
		if got := tabbed(m, typed); got != want {
			t.Errorf("%q: got %q, want %q", typed, got, want)
		}
	}
}

// Whatever Tab fills in for a chat resolves to that chat, by nick and by
// name, for every prefix of every alias and title in both scripts, with
// and without a sigil; deleted accounts are never offered.
func TestEveryCompletedChatNameResolves(t *testing.T) {
	m := setup(t, sharedFake())
	deleted := map[string]bool{}
	var typed []string
	for _, c := range m.chats {
		deleted[c.ID] = c.Deleted
		for _, s := range []string{c.Alias, shownTitle(c), domain.Translit(shownTitle(c))} {
			rs := []rune(s)
			for i := 0; i <= len(rs); i++ {
				typed = append(typed, string(rs[:i]), "@"+string(rs[:i]), "#"+string(rs[:i]))
			}
		}
	}
	for _, byName := range []bool{false, true} {
		m.byName = byName
		offered := 0
		for _, q := range typed {
			offers := map[string]string{} // what is filled in -> the chat it is offered for
			for _, cc := range m.chatCompletions(q) {
				offered++
				offers[cc.fill] = cc.chatID
				c, err := domain.ResolveChat(m.chats, cc.fill)
				if err != nil || c.ID != cc.chatID {
					t.Fatalf("by name %v, %q: %q is offered for %s but resolves to %q, %v", byName, q, cc.fill, cc.chatID, c.ID, err)
				}
				if deleted[cc.chatID] {
					t.Fatalf("by name %v, %q: the deleted %s is offered", byName, q, cc.chatID)
				}
			}
			// The same through Tab: one candidate is filled in with a space
			// and leaves nothing to list (a common prefix may end in a space
			// too, but waits for the second Tab).
			m.comp = nil
			got := tabbed(m, "/win "+q)
			name, one := strings.CutSuffix(strings.TrimPrefix(got, "/win "), " ")
			if one && m.comp == nil && got != "/win "+q && name != "list" {
				if c, err := domain.ResolveChat(m.chats, name); err != nil || c.ID != offers[name] {
					t.Fatalf("by name %v: Tab filled in %q for %q, which resolves to %q, %v", byName, name, q, c.ID, err)
				}
			}
			m.comp = nil
		}
		if offered < len(typed) {
			t.Fatalf("by name %v: only %d offers for %d prefixes", byName, offered, len(typed))
		}
	}
}

// A title has spaces: the name being typed may span words, after /win and
// /msg and in text, and the common prefix may end in a space.
func TestTitleWithSpacesCompletesAcrossWords(t *testing.T) {
	m := setup(t, titleFake())
	enter(m, "/names")
	if got := tabbed(m, "/win @Кал"); got != "/win @Калоян " {
		t.Fatalf("the common prefix, space and all: %q", got)
	}
	press(m, tea.KeyTab)
	if got := string(m.input); got != "/win @Калоян " {
		t.Fatalf("the second Tab keeps the words: %q", got)
	}
	if l := noticeLine(body(m), "-!- @Калоян GPS"); !strings.HasSuffix(l, "-!- @Калоян GPS, @Калоян Симеонов") {
		t.Fatalf("and lists the two: %q", l)
	}
	for typed, want := range map[string]string{
		"/win @Калоян С":        "/win @Калоян Симеонов ",
		"/win kaloyan s":        "/win @Калоян Симеонов ",
		"/msg @Калоян G":        "/msg @Калоян GPS ",
		"/msg @Калоян GPS зд":   "/msg @Калоян GPS зд", // the text after the name is free
		"/msg @Калоян GPS ":     "/msg @Калоян GPS ",
		"hi Калоян G":           "hi @Калоян GPS ",
		"/me waves at Калоян G": "/me waves at @Калоян GPS ",
	} {
		if got := tabbed(m, typed); got != want {
			t.Errorf("%q: got %q, want %q", typed, got, want)
		}
	}
	m.input = nil
	enter(m, "/names")
	if got := tabbed(m, "/win Калоян С"); got != "/win @калоян-симеонов " {
		t.Fatalf("by nick the alias: %q", got)
	}
}

// /msg takes a name with spaces, as /names shows it or as its title, in
// both modes; otherwise the first word is the name, as before.
func TestMsgToATitleWithSpaces(t *testing.T) {
	f := titleFake()
	m := setup(t, f)
	for i, byName := range []bool{false, true} {
		m.byName = byName
		m.input = nil
		runAll(m, enter(m, "/msg @Цветан Иванов здрасти"))
		if want := "c15:здрасти"; len(f.sent) != 2*i+1 || f.sent[2*i] != want || m.wins[m.cur].chatID != "c15" {
			t.Fatalf("by name %v: sent %v, want %q in window %q", byName, f.sent, want, m.wins[m.cur].chatID)
		}
		m.input = nil
		runAll(m, enter(m, "/msg цветан иванов как си"))
		if want := "c15:как си"; len(f.sent) != 2*i+2 || f.sent[2*i+1] != want {
			t.Fatalf("by name %v: the title without the sigil: sent %v", byName, f.sent)
		}
	}
	m.input = nil
	runAll(m, enter(m, "/msg @alice hi there"))
	if got := f.sent[len(f.sent)-1]; got != "c2:hi there" {
		t.Fatalf("the first word: %q", got)
	}
	m.input = nil
	runAll(m, enter(m, "/msg @Цветан hi"))
	if got := f.sent[len(f.sent)-1]; got != "c15:hi" {
		t.Fatalf("no title is @Цветан, so the first word names the chat: %q", got)
	}
	m.input = nil
	enter(m, "/msg @Цветан Иванов")
	if !m.isErr || !strings.Contains(m.status, "usage") || len(f.sent) != 6 {
		t.Fatalf("a name without text sends nothing, not its last word: %q %v", m.status, f.sent)
	}
}
