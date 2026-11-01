package tui

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lazygram/internal/command"
	"lazygram/internal/domain"
	"lazygram/internal/provider"
)

// Members of groups and channels: the senders of the messages lazygram
// holds for the chat (this session's and the cached backlog), the
// usernames those messages mention, and a group's member list from the
// service when it shows one. Tab after /whois and in message text
// completes their @nicks in a group or channel window; /whois <name> there
// describes one.

// groupChat is the current window's chat when it is a group or channel.
func (m *Model) groupChat() (domain.Chat, bool) {
	w := m.wins[m.cur]
	if w.chatID == "" {
		return domain.Chat{}, false
	}
	c, ok := m.chat(w.chatID)
	return c, ok && c.Kind != domain.ChatUser
}

// members lists the chat's members, most recently active first, with
// their names and nicks stripped of control characters (they come from
// the service and end up in the input line and in notices). The chat's
// own username is no member: channels sign their posts with it.
func (m *Model) members(chatID string) []domain.Member {
	c, _ := m.chat(chatID)
	ms := domain.WithParticipants(domain.Members(m.msgs, chatID), m.listed[chatID])
	out := ms[:0]
	for _, mb := range ms {
		mb.Name = strings.TrimSpace(sanitize(mb.Name, false))
		mb.Nick = strings.TrimSpace(sanitize(mb.Nick, false))
		if strings.ContainsRune(mb.Nick, ' ') {
			mb.Nick = ""
		}
		self := mb.Nick != "" && (strings.EqualFold(mb.Nick, c.Username) ||
			c.Kind != domain.ChatUser && strings.EqualFold(mb.Nick, c.Alias))
		if self && mb.Name == "" {
			continue // only mentioned: the chat itself
		}
		if self {
			mb.Nick = ""
		}
		if mb.Name != "" || mb.Nick != "" {
			out = append(out, mb)
		}
	}
	return out
}

// matches reports whether typed (a leading @ aside) begins the member's
// username or display name, in either script.
func matches(mb domain.Member, typed string) bool {
	q := strings.TrimPrefix(typed, "@")
	return mb.Nick != "" && domain.PrefixMatch(mb.Nick, q) || mb.Name != "" && domain.PrefixMatch(mb.Name, q)
}

// memberCandidates are the completions of typed among the members: those
// whose username or display name starts with it (ignoring case and a
// leading @, in either script: "iv" finds the username of "Иван"), as
// @nick, the mention Telegram notifies. Members without a username are
// offered only when byName is set (the /whois argument, which only looks
// them up), by display name; message text never gets them, as a message
// cannot mention them by @ and @name would mention whoever holds that
// username. shown lists a candidate with the display name next to the nick.
func memberCandidates(ms []domain.Member, typed string, byName bool) (cands []string, shown func(string) string) {
	names := map[string]string{}
	for _, mb := range ms {
		if !matches(mb, typed) || mb.Nick == "" && (!byName || mb.Name == "") {
			continue
		}
		c := mb.Name
		if mb.Nick != "" {
			c = "@" + mb.Nick
		}
		if _, dup := names[strings.ToLower(c)]; !dup {
			names[strings.ToLower(c)] = mb.Name
			cands = append(cands, c)
		}
	}
	sort.Strings(cands)
	return cands, func(c string) string {
		if n := names[strings.ToLower(c)]; n != "" && n != strings.TrimPrefix(c, "@") {
			return c + " (" + n + ")"
		}
		return c
	}
}

// nameWords is how many words before the cursor a display name being
// typed may span ("Grace K", "Ana Maria L").
const nameWords = 4

// memberStart is where the member name being typed in input starts: the
// earliest of the last nameWords word starts, from from on, whose rest of
// the input begins some member's name or username (a display name may have
// spaces, "Grace K"), with or without a username; else last, the start of
// the last word. Anchoring on members without a username too keeps "Ada L"
// from completing the "L" to someone else's nick.
func memberStart(ms []domain.Member, input []rune, from, last int) int {
	var starts []int
	for i := from; i < last; i++ {
		if (i == from || input[i-1] == ' ') && input[i] != ' ' {
			starts = append(starts, i)
		}
	}
	if len(starts) > nameWords {
		starts = starts[len(starts)-nameWords:]
	}
	for _, i := range starts {
		typed := string(input[i:])
		for _, mb := range ms {
			if matches(mb, typed) {
				return i
			}
		}
	}
	return last
}

// firstFetch is how many messages a window's first show fetches: the last
// recentOnOpen, or maxRecent in a group whose newest maxRecent known
// messages include some from a sender without an id (cached by a lazygram
// that did not keep ids). Fetched again, those messages give
// Store.Identify their senders' ids and usernames, so Tab can offer the
// nicks without a /last first. Channels are left out: their posts never
// carry a sender id. Own messages and unsigned posts need none.
func (m *Model) firstFetch(chatID string) int {
	if c, ok := m.chat(chatID); !ok || c.Kind != domain.ChatGroup {
		return recentOnOpen
	}
	seen := 0
	for i := len(m.msgs) - 1; i >= 0 && seen < maxRecent; i-- {
		x := m.msgs[i]
		if x.ChatID != chatID {
			continue
		}
		seen++
		if x.SenderID == "" && x.Sender != "me" && x.Sender != "channel" && x.Sender != "" {
			return maxRecent
		}
	}
	return recentOnOpen
}

// memberListCmd asks the service for a group's member list, once a
// session, when the provider can list members; nil otherwise. A broadcast
// channel is not asked, its subscribers are for admins only: it counts as
// hidden at once, without a notice.
func (m *Model) memberListCmd(chatID string) tea.Cmd {
	lister, ok := m.opt.Provider.(provider.MemberLister)
	c, known := m.chat(chatID)
	if !ok || !known || m.listAsked[chatID] {
		return nil
	}
	if c.Kind == domain.ChatChannel {
		m.listHidden[chatID] = true
	}
	if c.Kind != domain.ChatGroup {
		return nil
	}
	m.listAsked[chatID] = true
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		list, err := lister.Members(ctx, chatID)
		return memberListMsg{chatID: chatID, list: list, err: err}
	}
}

// gotMemberList keeps a member list and says in the group's window what
// came of it; a refusal leaves the members seen in messages.
func (m *Model) gotMemberList(msg memberListMsg) {
	_, w := m.winFor(msg.chatID)
	where, of := "here", "" // in the group's window it goes without saying
	if w == nil {           // closed meanwhile: the server window names the group
		w = m.wins[0]
		where, of = "in "+m.nameOf(msg.chatID), " of "+m.nameOf(msg.chatID)
	}
	switch {
	case errors.Is(msg.err, provider.ErrMembersHidden):
		m.listHidden[msg.chatID] = true
		m.note(w, "member list hidden "+where+": the members are the people seen writing or mentioned")
	case msg.err != nil:
		m.listAsked[msg.chatID] = false // try again on the next first show
		m.note(w, "member list"+of+": "+errText(msg.err))
	default:
		m.listed[msg.chatID] = msg.list
		m.note(w, fmt.Sprintf("member list%s: %d members", of, len(msg.list)))
	}
}

// commandArg reports whether the input is the command kind followed by a
// space, and where its argument starts.
func commandArg(input []rune, kind command.Kind) (int, bool) {
	s := string(input)
	if !strings.HasPrefix(s, "/") {
		return 0, false
	}
	i := strings.IndexByte(s, ' ')
	if i < 0 {
		return 0, false
	}
	if c, err := command.Parse(s[:i]); err != nil || c.Kind != kind {
		return 0, false
	}
	for i < len(s) && s[i] == ' ' {
		i++
	}
	return len([]rune(s[:i])), true
}

// whoisMember describes a member of the window's group or channel by the
// name given to /whois; chats are named as /names shows them. It reports
// false when nobody there matches, so
// the name is looked up among the chats instead; a # in front always
// means a chat.
func (m *Model) whoisMember(w *win, arg string) bool {
	g, ok := m.groupChat()
	if !ok || strings.HasPrefix(arg, "#") {
		return false
	}
	mb, err := domain.ResolveMember(m.members(g.ID), arg)
	switch {
	case errors.Is(err, domain.ErrNoMember):
		return false
	case err != nil:
		m.setErr(err.Error() + " here; type more of the name")
		return true
	}
	when := mb.Last.Local().Format("2006-01-02 15:04")
	switch {
	case mb.Name == "":
		m.note(w, fmt.Sprintf("whois @%s in %s: mentioned, not seen writing", mb.Nick, m.display(g)))
	case mb.Nick != "":
		m.note(w, fmt.Sprintf("whois @%s in %s: %s", mb.Nick, m.display(g), mb.Name))
	default:
		m.note(w, fmt.Sprintf("whois %s in %s: no username, so no @mention", mb.Name, m.display(g)))
	}
	var line string
	switch {
	case mb.Count > 0:
		line = fmt.Sprintf("  %d of the known messages here, the last %s", mb.Count, when)
		if mb.Mentions > 0 {
			line += fmt.Sprintf("; mentioned in %d", mb.Mentions)
		}
	case mb.Mentions > 0:
		line = fmt.Sprintf("  mentioned in %d of the known messages here, the last %s; none of theirs known", mb.Mentions, when)
	default:
		line = "  no known messages of theirs here"
	}
	if mb.Listed {
		line += "; in the member list"
	}
	m.note(w, line)
	if c, ok := m.privateChat(mb); ok {
		win := "no window"
		if i, _ := m.winFor(c.ID); i > 0 {
			win = fmt.Sprintf("window %d", i)
		}
		m.note(w, fmt.Sprintf("  private chat %s, %s", m.display(c), win))
	} else {
		m.note(w, "  no private chat with them")
	}
	return true
}

// privateChat finds the member's own chat. With an id it is the chat with
// that id or none (a private chat has the person's id, so another chat
// with their name or username is someone else); without one, a person chat
// whose alias is their nick, else the one person chat titled with their
// name.
func (m *Model) privateChat(mb domain.Member) (domain.Chat, bool) {
	if mb.ID != "" {
		c, ok := m.chat(mb.ID)
		return c, ok && !c.Deleted && c.Kind == domain.ChatUser
	}
	var byTitle []domain.Chat
	for _, c := range m.chats {
		if c.Deleted || c.Kind != domain.ChatUser {
			continue
		}
		if mb.Nick != "" && strings.EqualFold(c.Alias, mb.Nick) {
			return c, true
		}
		if mb.Name != "" && strings.EqualFold(c.Title, mb.Name) {
			byTitle = append(byTitle, c)
		}
	}
	if len(byTitle) == 1 {
		return byTitle[0], true
	}
	return domain.Chat{}, false
}
