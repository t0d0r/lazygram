package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"lazygram/internal/domain"
)

func TestCtrlLRedraws(t *testing.T) {
	m := setup(t, newFake())
	enter(m, "/win #dev")
	w := m.wins[m.cur]
	m.note(w, "a notice to drop")
	if len(w.notes) == 0 || !strings.Contains(m.View(), "a notice to drop") {
		t.Fatal("the notice shows")
	}
	press(m, tea.KeyCtrlL)
	if len(w.notes) != 0 || strings.Contains(m.View(), "a notice to drop") || !strings.Contains(m.View(), "first dev line") {
		t.Fatalf("Ctrl-L drops the notices and keeps the messages:\n%s", m.View())
	}
}

func TestOpenLinkGoesToTheBrowser(t *testing.T) {
	m := setup(t, newFake())
	var opened []string
	m.opt.Open = func(target string) error { opened = append(opened, target); return nil }
	cmd := enter(m, "/open https://example.invalid/doc?x=1")
	if cmd == nil || m.isErr || !strings.Contains(m.status, "opening https://example.invalid/doc?x=1") {
		t.Fatalf("server window too: cmd=%v status=%q", cmd != nil, m.status)
	}
	m.Update(cmd())
	if len(opened) != 1 || opened[0] != "https://example.invalid/doc?x=1" {
		t.Fatalf("opened %v", opened)
	}
	for _, bad := range []string{"/open http://", "/open HTTPS:///nohost", "/open http:/x",
		"/open https://example.invalid/x extra words", "/open https://example.invalid extra"} {
		if cmd := enter(m, bad); cmd != nil || !m.isErr || !strings.Contains(m.status, "only one http or https link with a host") {
			t.Fatalf("%s: cmd=%v status=%q", bad, cmd != nil, m.status)
		}
	}
	if len(opened) != 1 {
		t.Fatalf("a bad link opened nothing: %v", opened)
	}
	// Without a link, /open still means an attachment and needs a chat.
	enter(m, "/open")
	if !m.isErr || !strings.Contains(m.status, "server window has no chat") {
		t.Fatalf("attachment path kept: %q", m.status)
	}
	if !isWebLink("http://h/p") || !isWebLink("HTTPS://H") || isWebLink("ftp://h") || isWebLink("https://") || isWebLink("mailto:x@y") {
		t.Fatal("isWebLink: http(s) with a host only")
	}
}

func TestOpenCompletesTheRecentLinks(t *testing.T) {
	m := setup(t, newFake())
	enter(m, "/win @alice")
	var msgs []domain.Message
	for i := 0; i < 12; i++ { // twelve distinct links, the newest last
		msgs = append(msgs, mk("c2", "5"+string(rune('a'+i)), "alice", "see https://l.example/"+string(rune('a'+i)), 20+i))
	}
	msgs = append(msgs, mk("c2", "70", "alice", "again https://l.example/l and ftp://no.example/x", 40))
	msgs = append(msgs, mk("c2", "71", "alice", "read https://l.example/l.", 41)) // the full stop is not part of the link
	m.absorb(msgs)
	links := m.recentLinks("c2", recentLinkCount)
	if len(links) != recentLinkCount || links[0] != "https://l.example/l" || links[1] != "https://l.example/k" {
		t.Fatalf("newest first, distinct, ten at most, http(s) only, punctuation cut: %v", links)
	}
	for _, l := range links {
		if strings.HasSuffix(l, ".") {
			t.Fatalf("a candidate is what a click opens: %v", links)
		}
	}
	for _, l := range links {
		if l == "https://l.example/a" || l == "https://l.example/b" {
			t.Fatalf("the oldest links fall off: %v", links)
		}
	}
	typ(m, "/open https://l.example/k")
	press(m, tea.KeyTab)
	if got := string(m.input); got != "/open https://l.example/k " {
		t.Fatalf("a single candidate completes with a space: %q", got)
	}
	m.input, m.curBack = nil, 0
	typ(m, "/open o")
	press(m, tea.KeyTab)
	if got := string(m.input); got != "/open out " {
		t.Fatalf("out is still offered: %q", got)
	}
	m.input, m.curBack = nil, 0
	typ(m, "/open ht")
	press(m, tea.KeyTab)
	if got := string(m.input); got != "/open https://l.example/" {
		t.Fatalf("the common prefix of the links: %q", got)
	}
	// Nothing after /open out, and nothing in the server window.
	m.input, m.curBack = nil, 0
	typ(m, "/open out ht")
	press(m, tea.KeyTab)
	if got := string(m.input); got != "/open out ht" {
		t.Fatalf("links are not offered after out: %q", got)
	}
	// A link with capitals completes from a typed prefix with capitals and
	// is filled in with its own case.
	m.absorb([]domain.Message{mk("c2", "72", "alice", "repo https://github.example/Owner/Repo", 42)})
	m.input, m.curBack = nil, 0
	typ(m, "/open https://github.example/O")
	press(m, tea.KeyTab)
	if got := string(m.input); got != "/open https://github.example/Owner/Repo " {
		t.Fatalf("mixed-case link: %q", got)
	}
	// Two links that differ only in case stay apart: the typed case picks.
	m.absorb([]domain.Message{
		mk("c2", "73", "alice", "old https://d.example/Path", 43),
		mk("c2", "74", "alice", "new https://d.example/path", 44),
	})
	m.input, m.curBack = nil, 0
	typ(m, "/open https://d.example/P")
	press(m, tea.KeyTab)
	if got := string(m.input); got != "/open https://d.example/Path " {
		t.Fatalf("the typed capital picks the older link: %q", got)
	}
	m.input, m.curBack = nil, 0
	typ(m, "/open https://d.example/p")
	press(m, tea.KeyTab)
	if got := string(m.input); got != "/open https://d.example/path " {
		t.Fatalf("the typed lower case picks the newer link: %q", got)
	}
	// Control characters from a sender never reach the input line: links
	// are taken from the sanitised text, as a click takes them.
	m.absorb([]domain.Message{
		mk("c2", "75", "alice", "csi https://x.example/a\u009b2Jb", 45),
		mk("c2", "76", "alice", "soh https://y.example/a\x01b", 46),
	})
	for _, l := range m.recentLinks("c2", recentLinkCount) {
		if strings.ContainsAny(l, "\u009b\x01") {
			t.Fatalf("a control character survived: %q", l)
		}
	}
	m.input, m.curBack = nil, 0
	typ(m, "/open https://x")
	press(m, tea.KeyTab)
	if got := string(m.input); got != "/open https://x.example/a2Jb " || strings.ContainsRune(m.View(), '\u009b') {
		t.Fatalf("sanitised candidate: %q", got)
	}
	m.input, m.curBack = nil, 0
	typ(m, "/open https://y")
	press(m, tea.KeyTab)
	if got := string(m.input); got != "/open https://y.example/ab " {
		t.Fatalf("the C0 link is offered as a click would open it: %q", got)
	}
	m.input, m.curBack = nil, 0
	enter(m, "/win 0")
	typ(m, "/open ht")
	press(m, tea.KeyTab)
	if got := string(m.input); got != "/open ht" {
		t.Fatalf("server window: %q", got)
	}
}

func TestPastedTextKeepsItsLineBreaks(t *testing.T) {
	m := setup(t, newFake())
	enter(m, "/win @alice")
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("line one\r\nline two\nend\r"), Paste: true})
	if got := string(m.input); got != "line one\nline two\nend\n" {
		t.Fatalf("line breaks kept, CR LF as one: %q", got)
	}
	if !strings.Contains(m.View(), "line one↵line two↵end↵") {
		t.Fatal("the input shows the breaks as return marks")
	}
	m.input, m.curBack = nil, 0
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("tab\there\x01x"), Paste: true})
	if got := string(m.input); got != "tabherex" {
		t.Fatalf("other control characters are still dropped: %q", got)
	}
	m.input, m.curBack = nil, 0
	enter(m, "/list")
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("al\nice"), Paste: true})
	if got := string(m.input); got != "alice" || m.mode != modeList {
		t.Fatalf("a list filter takes no line breaks: %q mode=%v", got, m.mode)
	}
	press(m, tea.KeyEsc)
	enter(m, "/win @alice")
	enter(m, "/search")
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("hi\r\nthere"), Paste: true})
	if got := string(m.input); got != "hithere" || m.mode != modePager {
		t.Fatalf("the pager search takes no line breaks: %q mode=%v", got, m.mode)
	}
}

func TestListAndSearchMatchEitherScript(t *testing.T) {
	m := setup(t, newFake())
	m.chats = append(m.chats, domain.Chat{ID: "c9", Alias: "ivan", Title: "Иван Петров", Kind: domain.ChatUser})
	enter(m, "/list")
	typ(m, "петров")
	if v := m.View(); !strings.Contains(v, "Иван Петров") || strings.Contains(v, "Alice Smith") {
		t.Fatalf("a Cyrillic filter:\n%s", v)
	}
	m.input, m.curBack = nil, 0
	typ(m, "petrov")
	if v := m.View(); !strings.Contains(v, "Иван Петров") {
		t.Fatalf("a Latin filter finds the Cyrillic title:\n%s", v)
	}
	m.input, m.curBack = nil, 0
	typ(m, "иван")
	if v := m.View(); !strings.Contains(v, "Иван Петров") {
		t.Fatalf("a Cyrillic filter finds the Latin alias:\n%s", v)
	}
	press(m, tea.KeyEsc)
	// /whois and /win resolve a chat name in either script too.
	enter(m, "/whois petrov")
	if v := m.View(); m.isErr || !strings.Contains(v, "whois @ivan") {
		t.Fatalf("whois in either script: %q\n%s", m.status, v)
	}
	enter(m, "/win петров")
	if m.wins[m.cur].chatID != "c9" {
		t.Fatalf("/win in either script: %q chat=%q", m.status, m.wins[m.cur].chatID)
	}
	enter(m, "/win @alice")
	m.absorb([]domain.Message{mk("c2", "80", "alice", "Среща в София утре", 30)})
	enter(m, "/search sreshta")
	if v := m.View(); m.mode != modePager || !strings.Contains(v, "1 of") || !strings.Contains(v, "Среща") {
		t.Fatalf("the pager finds Cyrillic text with a Latin query:\n%s", v)
	}
}
