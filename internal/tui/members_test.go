package tui

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"lazygram/internal/domain"
	"lazygram/internal/provider"
	"lazygram/internal/store"
)

// memberFake is newFake with more people in #dev: Alice Smith, who has a
// username and a private chat (c2), next to ada and alan, who have neither,
// and grace_h, who never writes but alan mentions; and someone in #news
// whose name and username carry control characters.
func memberFake() *fake {
	f := newFake()
	a := mk("c1", "3", "Alice Smith", "third dev line", 4)
	a.SenderID, a.SenderNick = "c2", "alice_s"
	n := mk("c1", "5", "alan", "happy birthday @grace_h! ping @Alice_S", 5)
	e := mk("c3", "8", "Ev\x1be", "signed post", 5)
	e.SenderID, e.SenderNick = "u8", "ev\x1be_x"
	f.msgs = append(f.msgs, a, n, e)
	return f
}

// noticeLine is the last line of v that contains marker, trailing space cut.
func noticeLine(v, marker string) string {
	line := ""
	for _, l := range strings.Split(v, "\n") {
		if strings.Contains(l, marker) {
			line = strings.TrimRight(l, " ")
		}
	}
	return line
}

func tabbed(m *Model, typed string) string {
	m.input = nil
	typ(m, typed)
	press(m, tea.KeyTab)
	return string(m.input)
}

func TestWhoisCompletesMembersInAGroup(t *testing.T) {
	m := setup(t, memberFake())
	if got := tabbed(m, "/whois ali"); got != "/whois @alice " {
		t.Fatalf("server window completes chats: %q", got)
	}
	m.input = nil
	enter(m, "/win #dev")
	for typed, want := range map[string]string{
		"/whois ali":      "/whois @alice_s ", // the member, not the chat @alice
		"/wi Alice S":     "/wi @alice_s ",    // the whole argument, spaces and all: the name finds the nick
		"/whois gr":       "/whois @grace_h ", // mentioned only, still a nick in the channel
		"/whois ad":       "/whois ada ",      // no username: /whois offers the name, it only looks up
		"/whois ne":       "/whois ne",        // #news is a chat, not a member
		"/whois #ne":      "/whois #news ",    // a # asks for a chat
		"/win ne":         "/win #news ",      // /win completes every chat
		"/win al":         "/win @alice ",
		"thanks gr":       "thanks @grace_h ", // message text: @nick, always
		"hi @gr":          "hi @grace_h ",
		"hi @alice s":     "hi @alice_s ",
		"hi ad":           "hi ad",
		"hi ne":           "hi ne",
		"/me waves at gr": "/me waves at @grace_h ",
	} {
		if got := tabbed(m, typed); got != want {
			t.Errorf("%q: got %q, want %q", typed, got, want)
		}
	}
	tabbed(m, "hi @") // @alice_s and @grace_h: no common extension
	press(m, tea.KeyTab)
	if got := string(m.input); got != "hi @" {
		t.Fatalf("input changed: %q", got)
	}
	if v := body(m); !strings.Contains(v, "@alice_s (Alice Smith), @grace_h") || strings.Contains(v, "alan,") {
		t.Fatalf("second Tab lists the nicks, nobody without one:\n%s", v)
	}
	m.input = nil
	enter(m, "/win #news")
	if got := tabbed(m, "/whois ev"); got != "/whois @eve_x " {
		t.Fatalf("a member's nick is sanitised before it reaches the input: %q", got)
	}
	m.input = nil
	enter(m, "/win @alice")
	if got := tabbed(m, "hi ne"); got != "hi #news " {
		t.Fatalf("a private chat completes chats as before: %q", got)
	}
}

func TestWhoisDescribesAMember(t *testing.T) {
	m := setup(t, memberFake())
	enter(m, "/win #dev")
	m.input = nil
	enter(m, "/whois alice")
	v := body(m)
	for _, want := range []string{"whois @alice_s in #dev: Alice Smith", "1 of the known messages here", "private chat @alice, window"} {
		if !strings.Contains(v, want) {
			t.Errorf("member whois lacks %q:\n%s", want, v)
		}
	}
	if v := body(m); !strings.Contains(v, "; mentioned in 1") {
		t.Errorf("alan mentions @Alice_S, whatever the case:\n%s", v)
	}
	enter(m, "/whois ada")
	if v := body(m); !strings.Contains(v, "whois ada in #dev: no username, so no @mention") || !strings.Contains(v, "no private chat with them") {
		t.Errorf("member without a username:\n%s", v)
	}
	enter(m, "/whois grace_h")
	if v := body(m); !strings.Contains(v, "whois @grace_h in #dev: mentioned, not seen writing") || !strings.Contains(v, "mentioned in 1 of the known messages here") {
		t.Errorf("a nick only mentioned:\n%s", v)
	}
	enter(m, "/whois a")
	if !m.isErr || !strings.Contains(m.status, "ambiguous member") {
		t.Errorf("three members start with a: %q", m.status)
	}
	enter(m, "/whois news")
	if v := body(m); !strings.Contains(v, "whois #news: News") {
		t.Errorf("no member: the chats as before:\n%s", v)
	}
	enter(m, "/whois")
	if v := body(m); !strings.Contains(v, "4 members known from the known messages, 2 with an @username") {
		t.Errorf("the group's own whois counts its members:\n%s", v)
	}
	enter(m, "/win @alice")
	enter(m, "/whois")
	if strings.Contains(body(m), "members seen") {
		t.Error("a person has no members")
	}
}

func TestMembersLeaveOutOwnAndUnsigned(t *testing.T) {
	f := newFake()
	f.msgs = append(f.msgs, mk("c1", "9", "me", "mine", 6), mk("c3", "9", "channel", "unsigned", 6))
	m := setup(t, f)
	var got []string
	for _, id := range []string{"c1", "c3"} {
		for _, mb := range m.members(id) {
			got = append(got, mb.Name)
		}
	}
	if strings.Join(got, ",") != "alan,ada" {
		t.Fatalf("members: %v", got)
	}
	if _, ok := m.privateChat(domain.Member{Name: "Alice Smith"}); !ok {
		t.Error("a person chat titled with the name is theirs")
	}
}

func TestMemberCompletionStaysInTheWindowInput(t *testing.T) {
	m := setup(t, memberFake())
	enter(m, "/win #dev")
	for _, open := range []string{"/list", "/win list", "/search"} {
		m.input = nil
		enter(m, open)
		typ(m, "ne")
		press(m, tea.KeyTab)
		if got := string(m.input); got != "#news " {
			t.Errorf("%s filter completes chats as anywhere else: %q", open, got)
		}
		press(m, tea.KeyEsc)
		press(m, tea.KeyEsc)
	}
	m.input = nil
	typ(m, "hello")
	press(m, tea.KeyCtrlA) // the cursor at the start of message text
	press(m, tea.KeyTab)
	press(m, tea.KeyTab)
	if v := body(m); strings.Contains(v, "#dev  #news") {
		t.Errorf("message text offers no chat names, cursor or not:\n%s", v)
	}
}

func TestMemberNamesWithSpacesComplete(t *testing.T) {
	f := memberFake()
	h, k := mk("c1", "6", "Grace Hopper", "a", 6), mk("c1", "7", "Grace Kelly", "b", 7)
	h.SenderID, h.SenderNick = "u6", "ghopper"
	k.SenderID, k.SenderNick = "u7", "gkelly"
	f.msgs = append(f.msgs, h, k)
	m := setup(t, f)
	enter(m, "/win #dev")
	if got := tabbed(m, "hi Grace "); got != "hi Grace " {
		t.Fatalf("no common extension: %q", got)
	}
	press(m, tea.KeyTab)
	if l := noticeLine(body(m), "-!- @ghopper"); !strings.HasSuffix(l, "-!- @ghopper (Grace Hopper), @gkelly (Grace Kelly)") {
		t.Fatalf("the second Tab lists the two Graces only: %q", l)
	}
	for typed, want := range map[string]string{
		"hi Grace K":      "hi @gkelly ", // the name, spaces and all, finds the nick
		"hi @grace h":     "hi @ghopper ",
		"/me waves at Gr": "/me waves at Gr", // three: @ghopper, @gkelly, @grace_h
		"grace said hi":   "grace said hi",   // nobody's name starts with "grace said" or "hi"
	} {
		if got := tabbed(m, typed); got != want {
			t.Errorf("%q: got %q, want %q", typed, got, want)
		}
	}
}

func TestWhoisMemberKeepsPeopleApart(t *testing.T) {
	f := newFake()
	other := mk("c1", "6", "Alice Smith", "not the one in c2", 8)
	other.SenderID = "u77" // same name as c2's Alice, another person
	f.msgs = append(f.msgs, other)
	m := setup(t, f)
	enter(m, "/win #dev")
	enter(m, "/whois Alice Smith")
	if v := body(m); !strings.Contains(v, "no private chat with them") {
		t.Errorf("a known id finds its own chat or none:\n%s", v)
	}
	m.input = nil
	enter(m, "/win 0")
	enter(m, "/whois #dev")
	if strings.Contains(body(m), "members seen") {
		t.Error("the member count belongs to /whois without a name")
	}
}

func TestCachedMessagesLearnTheirSender(t *testing.T) {
	f := newFake() // ada's line in #dev is cached without an id or username
	again := mk("c1", "1", "ada", "first dev line", 0)
	again.SenderID, again.SenderNick = "u5", "ada_l"
	f.recent["c1"] = []domain.Message{again}
	m := setup(t, f)
	runAll(m, enter(m, "/win #dev")) // the window's first show fetches it again
	nick := ""
	for _, mb := range m.members("c1") {
		if mb.Name == "ada" {
			nick = mb.Nick
		}
	}
	if nick != "ada_l" {
		t.Fatalf("the fetched copy's username reaches the member: %q", nick)
	}
	cached, err := m.opt.Store.Messages("c1", 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range cached {
		if x.ID == "1" && (x.SenderID != "u5" || x.SenderNick != "ada_l" || x.Text != "first dev line") {
			t.Fatalf("the cache keeps it: %+v", x)
		}
	}
	if got := tabbed(m, "thanks ad"); got != "thanks @ada_l " {
		t.Fatalf("completion uses the username: %q", got)
	}
}

func TestCompletionInEitherScript(t *testing.T) {
	f := memberFake()
	f.chats = append(f.chats, domain.Chat{ID: "c9", Alias: "иван-петров", Title: "Иван Петров", Kind: domain.ChatUser})
	iv := mk("c1", "9", "Иван Петров", "здравей", 9)
	iv.SenderID, iv.SenderNick = "c9", "ivan_p"
	f.msgs = append(f.msgs, iv)
	m := setup(t, f)
	for typed, want := range map[string]string{
		"/win iv":   "/win @иван-петров ", // a Latin prefix completes a Cyrillic chat
		"/win не":   "/win #news ",        // and a Cyrillic one a Latin chat
		"/whois iv": "/whois @иван-петров ",
	} {
		if got := tabbed(m, typed); got != want {
			t.Errorf("server window %q: got %q, want %q", typed, got, want)
		}
	}
	m.input = nil
	enter(m, "/win #dev")
	for typed, want := range map[string]string{
		"/whois iv": "/whois @ivan_p ", // a Latin prefix of a Cyrillic name finds the nick
		"hi iva":    "hi @ivan_p ",
		"hi ива":    "hi @ivan_p ",
		"hi али":    "hi @alice_s ", // a Cyrillic prefix of a Latin nick
		"hi pet":    "hi pet",       // a prefix of the name, not of a later word
	} {
		if got := tabbed(m, typed); got != want {
			t.Errorf("#dev %q: got %q, want %q", typed, got, want)
		}
	}
	m.input = nil
	enter(m, "/whois iv")
	if v := body(m); !strings.Contains(v, "whois @ivan_p in #dev: Иван Петров") || !strings.Contains(v, "private chat @иван-петров") {
		t.Errorf("/whois resolves what Tab completes:\n%s", v)
	}
}

func TestExtends(t *testing.T) {
	for _, c := range []struct {
		prefix, typed string
		want          bool
	}{
		{"Живк", "zhiv", true}, // longer in transliteration, not in runes
		{"#ne", "ne", true},    // the sigil comes with it
		{"@van", "ив", false},  // a username prefix of members matched by name
		{"ne", "ne", false},
		{"~/Documents/", "~/Doc", true},
		{"Grace ", "gr", true},
	} {
		if got := extends(c.prefix, c.typed); got != c.want {
			t.Errorf("extends(%q, %q) = %v", c.prefix, c.typed, got)
		}
	}
}

func TestCommonPrefixKeepsWhatWasTyped(t *testing.T) {
	f := newFake()
	add := func(id, name, nick string, min int) {
		x := mk("c1", id, name, "line", min)
		x.SenderID, x.SenderNick = "u"+id, nick
		f.msgs = append(f.msgs, x)
	}
	add("20", "Иван Петров", "vanko_p", 10)
	add("21", "Ивайло Колев", "vanyo88", 11)
	add("22", "Vanessa", "vanessa", 12)
	add("23", "Живко", "zhivko1", 13)
	add("24", "Живка", "zhivka2", 14)
	m := setup(t, f)
	enter(m, "/win #dev")
	if got := tabbed(m, "hi ив"); got != "hi ив" {
		t.Fatalf("not replaced by the usernames' prefix @van: %q", got)
	}
	press(m, tea.KeyTab)
	if v := body(m); !strings.Contains(v, "@vanko_p (Иван Петров), @vanyo88 (Ивайло Колев)") || strings.Contains(v, "@vanessa") {
		t.Fatalf("the second Tab lists who matched, nobody else:\n%s", v)
	}
	if got := tabbed(m, "hi жив"); got != "hi @zhivk" {
		t.Fatalf("a common prefix in another script extends the word: %q", got)
	}
}

func TestRecentShowsWhatWasStoredDespiteAnError(t *testing.T) {
	m := setup(t, newFake())
	_, w := m.winFor("c1")
	m.Update(recentMsg{chatID: "c1", fresh: []domain.Message{mk("c1", "77", "ada", "kept", 9)}, err: errors.New("disk trouble")})
	if !m.isErr || !strings.Contains(m.status, "disk trouble") {
		t.Errorf("the error shows: %q", m.status)
	}
	found := false
	for _, x := range m.msgs {
		found = found || x.ChatID == "c1" && x.ID == "77"
	}
	if !found || w.loaded {
		t.Fatalf("the stored message shows, the window still counts as not loaded: found=%v loaded=%v", found, w.loaded)
	}
}

func TestWhoisMemberFollowsNames(t *testing.T) {
	m := setup(t, memberFake())
	enter(m, "/win #dev")
	m.byName = true // /names: chats by title
	enter(m, "/whois alice")
	if v := body(m); !strings.Contains(v, "whois @alice_s in #Dev: Alice Smith") || !strings.Contains(v, "private chat @Alice Smith, window") {
		t.Errorf("chats named by title, as /names shows them:\n%s", v)
	}
}

func TestTheChatsOwnUsernameIsNoMember(t *testing.T) {
	f := newFake()
	f.chats[2].Username = "news_daily" // #news signs its posts
	f.msgs = append(f.msgs, mk("c3", "9", "channel", "Breaking: something. Subscribe @news_daily", 6),
		mk("c1", "9", "ada", "read @news_daily", 7))
	m := setup(t, f)
	for _, mb := range m.members("c3") {
		t.Errorf("#news has no member %+v", mb)
	}
	enter(m, "/win #news")
	m.input = nil
	enter(m, "/whois news")
	if v := body(m); !strings.Contains(v, "whois #news: News") {
		t.Errorf("the channel's own name still describes the channel:\n%s", v)
	}
	m.input = nil
	enter(m, "/win #dev")
	if got := tabbed(m, "hi @new"); got != "hi @news_daily " {
		t.Errorf("in another chat the channel is a nick like any: %q", got)
	}
}

func TestNameWithoutUsernameAnchorsTheStart(t *testing.T) {
	f := newFake()
	ada := mk("c1", "8", "Ada Lovelace", "x", 6)
	ada.SenderID = "u8" // no username
	lena := mk("c1", "9", "Lena", "y", 7)
	lena.SenderID, lena.SenderNick = "u9", "lena_k"
	f.msgs = append(f.msgs, ada, lena)
	m := setup(t, f)
	enter(m, "/win #dev")
	if got := tabbed(m, "hi Ada L"); got != "hi Ada L" {
		t.Fatalf("Ada L is Ada, who has no @nick, not @lena_k: %q", got)
	}
	if got := tabbed(m, "hi L"); got != "hi @lena_k " {
		t.Fatalf("L alone is Lena: %q", got)
	}
}

func TestWhoisListsEveryMemberTextOnlyNicks(t *testing.T) {
	m := setup(t, memberFake())
	enter(m, "/win #dev")
	tabbed(m, "/whois @")
	press(m, tea.KeyTab)
	if l := noticeLine(body(m), "-!- @alice_s"); !strings.HasSuffix(l, "-!- @alice_s (Alice Smith), @grace_h, ada, alan") {
		t.Fatalf("/whois lists everyone, nicks first: %q", l)
	}
	tabbed(m, "hi @")
	press(m, tea.KeyTab)
	if l := noticeLine(body(m), "-!- @alice_s"); !strings.HasSuffix(l, "-!- @alice_s (Alice Smith), @grace_h") {
		t.Fatalf("message text lists only @nicks: %q", l)
	}
}

func TestFirstFetchCatchesUpOnSenders(t *testing.T) {
	f := newFake() // #dev's lines are cached without sender ids
	m := setup(t, f)
	if n := m.firstFetch("c1"); n != maxRecent {
		t.Errorf("a group with unidentified senders fetches %d, want %d", n, maxRecent)
	}
	if n := m.firstFetch("c3"); n != recentOnOpen {
		t.Errorf("a channel's posts carry no sender id: %d", n)
	}
	if n := m.firstFetch("c2"); n != recentOnOpen {
		t.Errorf("a person: %d", n)
	}
	for i := range m.msgs {
		if m.msgs[i].ChatID == "c1" {
			m.msgs[i].SenderID = "u" + m.msgs[i].ID
		}
	}
	if n := m.firstFetch("c1"); n != recentOnOpen {
		t.Errorf("every sender known: %d", n)
	}
	f2 := newFake()
	f2.msgs = append([]domain.Message{}, mk("c1", "1", "me", "mine", 0), mk("c1", "2", "channel", "sent as the group", 1))
	if n := setup(t, f2).firstFetch("c1"); n != recentOnOpen {
		t.Errorf("own messages and unsigned posts need no id: %d", n)
	}
}

// listingFake is a fake that also lists group members.
type listingFake struct {
	*fake
	lists   map[string][]domain.Participant
	listErr error
	asked   []string
}

func (f *listingFake) Members(_ context.Context, chatID string) ([]domain.Participant, error) {
	f.asked = append(f.asked, chatID)
	return f.lists[chatID], f.listErr
}

// setupProvider is setup for any provider.
func setupProvider(t *testing.T, p provider.Provider) *Model {
	t.Helper()
	st, err := store.Open(store.Memory, bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	m := New(Options{Store: st, Provider: p})
	m.fx = false
	m.revealAll()
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	m.Update(m.chatsCmd()())
	m.Update(m.syncCmd()())
	return m
}

func TestMemberListAddsSilentMembers(t *testing.T) {
	f := &listingFake{fake: memberFake(), lists: map[string][]domain.Participant{"c1": {
		{ID: "c2", Name: "Alice Smith", Nick: "alice_s"},
		{ID: "u40", Name: "Margaret", Nick: "margaret_h"}, // never writes
		{ID: "u41", Name: "Barbara"},                      // no username
		{ID: "u42", Name: "Grace", Nick: "grace_h"},       // only mentioned so far
	}}}
	m := setupProvider(t, f)
	runAll(m, enter(m, "/win #dev"))
	if len(f.asked) != 1 || f.asked[0] != "c1" {
		t.Fatalf("asked once, for the group: %v", f.asked)
	}
	if v := body(m); !strings.Contains(v, "member list: 4 members") {
		t.Errorf("the window says what came:\n%s", v)
	}
	for typed, want := range map[string]string{
		"hi mar":    "hi @margaret_h ",
		"hi bar":    "hi bar", // no username: not in text
		"/whois ba": "/whois Barbara ",
	} {
		if got := tabbed(m, typed); got != want {
			t.Errorf("%q: got %q, want %q", typed, got, want)
		}
	}
	m.input = nil
	enter(m, "/whois margaret_h")
	if v := body(m); !strings.Contains(v, "whois @margaret_h in #dev: Margaret") || !strings.Contains(v, "no known messages of theirs here; in the member list") {
		t.Errorf("a listed member who never wrote:\n%s", v)
	}
	enter(m, "/whois grace_h")
	if v := body(m); !strings.Contains(v, "whois @grace_h in #dev: Grace") || !strings.Contains(v, "mentioned in 1 of the known messages here") {
		t.Errorf("the list names a nick only mentioned:\n%s", v)
	}
	enter(m, "/whois")
	if v := body(m); !strings.Contains(v, "members known from the member list and the known messages") {
		t.Errorf("the count says where members come from:\n%s", v)
	}
	m.input = nil
	runAll(m, enter(m, "/win #news"))
	runAll(m, enter(m, "/win #dev"))
	if len(f.asked) != 1 {
		t.Errorf("a channel is not asked, a group once a session: %v", f.asked)
	}
}

func TestHiddenMemberListKeepsTheSeen(t *testing.T) {
	f := &listingFake{fake: memberFake(), listErr: provider.ErrMembersHidden}
	m := setupProvider(t, f)
	runAll(m, enter(m, "/win #dev"))
	if v := body(m); !strings.Contains(v, "member list hidden here") {
		t.Errorf("the refusal is said:\n%s", v)
	}
	if got := tabbed(m, "hi gr"); got != "hi @grace_h " {
		t.Errorf("the members seen still complete: %q", got)
	}
	f.listErr = errors.New("network down")
	m2 := setupProvider(t, f)
	runAll(m2, enter(m2, "/win #dev"))
	if v := body(m2); !strings.Contains(v, "member list: network down") || m2.listAsked["c1"] {
		t.Errorf("another error is said and asked again later:\n%s", v)
	}
}

func TestMemberListNoticeNamesTheGroupElsewhere(t *testing.T) {
	f := &listingFake{fake: memberFake()}
	m := setupProvider(t, f)
	if i, w := m.winFor("c1"); w != nil {
		m.closeWin(i)
	}
	m.cur = 0
	m.Update(memberListMsg{chatID: "c1", list: []domain.Participant{{ID: "u40", Name: "Margaret", Nick: "margaret_h"}}})
	m.Update(memberListMsg{chatID: "c1", err: provider.ErrMembersHidden})
	v := body(m)
	if !strings.Contains(v, "member list of #dev: 1 members") || !strings.Contains(v, "member list hidden in #dev") {
		t.Errorf("the server window names the group:\n%s", v)
	}
	runAll(m, enter(m, "/win #news")) // a broadcast channel: not asked, hidden
	m.input = nil
	enter(m, "/whois")
	if v := body(m); !strings.Contains(v, "the member list is hidden") || len(f.asked) != 0 {
		t.Errorf("a channel's list is hidden without asking: %v\n%s", f.asked, v)
	}
}

func TestServerWindowShowsOnlyThisSession(t *testing.T) {
	st, err := store.Open(store.Memory, bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if _, err := st.Add([]domain.Message{mk("c1", "90", "ada", "cached from yesterday", -600)}); err != nil {
		t.Fatal(err)
	}
	f := newFake() // its Sync brings the session's messages
	m := New(Options{Store: st, Provider: f})
	m.fx = false
	m.revealAll()
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	m.Update(m.chatsCmd()())
	m.Update(m.syncCmd()())
	v := body(m)
	if strings.Contains(v, "cached from yesterday") || !strings.Contains(v, "first dev line") {
		t.Fatalf("window 0: what arrived this session, not the cache:\n%s", v)
	}
	enter(m, "/win #dev")
	if !strings.Contains(body(m), "cached from yesterday") {
		t.Fatalf("the chat window keeps its history:\n%s", body(m))
	}
}
