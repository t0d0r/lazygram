package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lazygram/internal/domain"
	"lazygram/internal/provider"
)

// Typing indicators: the service's reports land in domain.TypingState,
// the status line shows who is doing what in the current window (or, in
// the server window, in any listed chat), with a running light over the
// verb. Two timers serve it, each armed only while needed: the expiry
// timer while a report is pending, the light while the current window
// shows someone and the lights are on (/fx). Update arms them after every
// message, so whatever changed the window or the state is followed.

// typingTick paces the running light.
const typingTick = 150 * time.Millisecond

type (
	typingMsg       struct{ ev domain.Typing } // one report from the service
	typingDoneMsg   struct{}                   // the service closed its feed
	typingExpireMsg struct{}                   // time to drop stale reports
	typingAnimMsg   struct{}                   // advance the running light
)

// typingInit subscribes to the service's feed when it has one.
func (m *Model) typingInit() tea.Cmd {
	ts, ok := m.opt.Provider.(provider.TypingSource)
	if !ok {
		return nil
	}
	ch := ts.Typing()
	if ch == nil {
		return nil
	}
	m.typingCh = ch
	return waitTyping(ch)
}

// waitTyping delivers the next report, or typingDoneMsg once the feed is
// closed. One goroutine per read, re-armed after each report, and none
// left behind: the service closes the feed when it stops.
func waitTyping(ch <-chan domain.Typing) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return typingDoneMsg{}
		}
		return typingMsg{ev: ev}
	}
}

// typingUpdate handles the typing messages; handled says whether msg was
// one of them. The timers are armed by Update afterwards.
func (m *Model) typingUpdate(msg tea.Msg) (cmd tea.Cmd, handled bool) {
	switch msg := msg.(type) {
	case typingMsg:
		m.typing.Apply(msg.ev)
		if m.typingCh != nil {
			return waitTyping(m.typingCh), true
		}
		return nil, true
	case typingDoneMsg:
		m.typingCh = nil
		return nil, true
	case typingExpireMsg:
		m.typingExpiry = false
		m.typing.Expire(m.now())
		return nil, true
	case typingAnimMsg:
		m.typingAnim = false
		if m.fx && len(m.typingShown()) > 0 {
			m.typingStep++
		}
		return nil, true // with nothing to light the clock stops here
	}
	return nil, false
}

// typingTimers arms what the state needs and nothing more. Each timer is
// armed once; its message clears the flag, and Update arms it again while
// it is still needed.
func (m *Model) typingTimers() []tea.Cmd {
	var cmds []tea.Cmd
	if next, ok := m.typing.Next(); ok && !m.typingExpiry {
		m.typingExpiry = true
		d := max(next.Sub(m.now()), time.Millisecond)
		cmds = append(cmds, tea.Tick(d, func(time.Time) tea.Msg { return typingExpireMsg{} }))
	}
	if m.fx && !m.typingAnim && len(m.typingShown()) > 0 {
		m.typingAnim = true
		cmds = append(cmds, tea.Tick(typingTick, func(time.Time) tea.Msg { return typingAnimMsg{} }))
	}
	return cmds
}

// typingSettled ends the reports of members whose messages just arrived.
// A sender who was not reported by name most likely is the chat's unnamed
// member (unknown at report time, or a channel posting as itself), so
// their message ends that entry; a still-typing unnamed member renews it.
func (m *Model) typingSettled(fresh []domain.Message) {
	for _, msg := range fresh {
		if msg.Sender == "me" {
			continue
		}
		if !m.typing.Clear(msg.ChatID, msg.Sender) {
			m.typing.Clear(msg.ChatID, domain.Unknown)
		}
	}
}

// shownTypist is one entry of the indicator; chatID is set in the server
// window, where every listed chat's reports show.
type shownTypist struct {
	domain.Typist
	chatID string
}

// typingShown is what the current window reports: in a chat window its
// own typists, in the server window everyone in a chat lazygram lists,
// the earliest report first. A report for a chat not among the listed
// dialogs has no name to show and waits out its expiry unseen.
func (m *Model) typingShown() []shownTypist {
	var out []shownTypist
	w := m.wins[m.cur]
	if w.chatID != "" {
		for _, t := range m.typing.Typists(w.chatID) {
			out = append(out, shownTypist{Typist: t})
		}
		return out
	}
	for _, id := range m.typing.Chats() {
		if _, ok := m.chat(id); !ok {
			continue
		}
		for _, t := range m.typing.Typists(id) {
			out = append(out, shownTypist{Typist: t, chatID: id})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if !a.Since.Equal(b.Since) {
			return a.Since.Before(b.Since)
		}
		if a.chatID != b.chatID {
			return a.chatID < b.chatID
		}
		return a.Who < b.Who
	})
	return out
}

// typingText words the indicator: "ada is", "ada and bob are", "ada, bob
// and 2 others are", each name with its chat in the server window ("ada
// (#dev) is"), and the verb: the common kind, "typing" when they differ.
func typingText(shown []shownTypist, nameOf func(string) string) (names, verb string) {
	if len(shown) == 0 {
		return "", ""
	}
	label := func(t shownTypist) string {
		if t.chatID == "" {
			return t.Who
		}
		return t.Who + " (" + nameOf(t.chatID) + ")"
	}
	verb = shown[0].Kind.String()
	for _, t := range shown[1:] {
		if t.Kind != shown[0].Kind {
			verb = domain.TypingText.String()
			break
		}
	}
	switch n := len(shown); n {
	case 1:
		names = label(shown[0]) + " is"
	case 2:
		names = label(shown[0]) + " and " + label(shown[1]) + " are"
	default:
		others := "others"
		if n == 3 {
			others = "other"
		}
		names = fmt.Sprintf("%s, %s and %d %s are", label(shown[0]), label(shown[1]), n-2, others)
	}
	return names, verb
}

// bounce is where the running light is in a word of n cells at step: it
// runs to the right, turns at the last cell and comes back, and so on.
// dir is +1 on the way right and -1 on the way back.
func bounce(step, n int) (pos, dir int) {
	if n <= 1 {
		return 0, 1
	}
	period := 2 * (n - 1)
	s := step % period
	if s < 0 {
		s += period
	}
	if s <= n-1 {
		return s, 1
	}
	return period - s, -1
}

// typingSegment is the status line's indicator fitted into room cells:
// the full wording, else the count and the verb ("2 typing"), else
// nothing. "" when the current window shows nobody. The light runs only
// with the lights on (/fx).
func (m *Model) typingSegment(room int) string {
	shown := m.typingShown()
	if len(shown) == 0 {
		return ""
	}
	names, verb := typingText(shown, m.nameOf)
	if lipgloss.Width(names)+1+len(verb) > room {
		names = strconv.Itoa(len(shown))
		if len(names)+1+len(verb) > room {
			return ""
		}
	}
	pos := -1
	if m.fx {
		pos, _ = bounce(m.typingStep, len(verb))
	}
	var b strings.Builder
	b.WriteString(seg(names+" ", 4, false))
	for i, r := range verb {
		if i == pos {
			b.WriteString(sparkStyle.Background(statusBG()).Render(string(r)))
		} else {
			b.WriteString(seg(string(r), 4, false))
		}
	}
	return b.String()
}
