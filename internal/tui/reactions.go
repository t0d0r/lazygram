package tui

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rivo/uniseg"

	"lazygram/internal/domain"
	"lazygram/internal/provider"
)

// Reactions show after a message's text in every window, "[<e> 3 <e> 1]"
// in the service's order, the account's own in the highlight style. The
// service reports changes two ways: pushed on the change feed (Telegram
// does so for the account's own messages only) and found by polling the
// messages on screen every reactionPollEvery, as Telegram asks apps to.
// Both go through applyChange (reacted): the cache and the windows show
// the new counts at once, and window 0 tells of a reaction someone added
// in a notice, which /feed off hides like the chats' messages. A first
// sighting, a removal and the account's own reaction say nothing.

const (
	// reactionPollEvery is how often the messages on screen are asked
	// about; Telegram asks apps to poll every 15 to 30 seconds.
	reactionPollEvery = 20 * time.Second
	// maxReactors bounds what is remembered of who reacted; past it the
	// memory starts over, which can name a reactor twice but never makes
	// a notice of a change that added nothing.
	maxReactors = 20000
	// firstWordsMax is about how many columns of a message a notice quotes.
	firstWordsMax = 30
)

type (
	reactTickMsg struct{} // a poll of the reactions on screen is due
	polledMsg    struct { // what a poll brought
		chatID  string
		changes []domain.Change
		err     error
	}
)

// reactState is what the reaction polls and notices go by.
type reactState struct {
	due      bool                         // a tick came while a sync ran: the poll follows its end
	running  bool                         // a poll is on its way
	wait     time.Time                    // the service asked for a pause: no poll before
	refused  map[string]bool              // chats the service will not be asked about, for the session
	last     string                       // the chat window 0 polled last; the next one on screen follows
	onScreen []string                     // the message on each row the focused window shows ("" for none), as of the last View
	learned  map[string]bool              // keys of the messages whose reactions came from the service this session
	named    map[string]bool              // who reacted with what to which message, as the service named them
	unheld   map[string][]domain.Reaction // the last reactions reported for messages the cache does not hold
}

func newReactState() reactState {
	return reactState{refused: map[string]bool{}, learned: map[string]bool{}, named: map[string]bool{},
		unheld: map[string][]domain.Reaction{}}
}

// learn records messages whose reactions the service just sent (a sync, a
// fetch, an edit): a poll's later word on them is compared with these.
func (r *reactState) learn(lists ...[]domain.Message) {
	for _, l := range lists {
		for _, x := range l {
			r.learned[x.Key()] = true
		}
	}
}

// remember records who the service named as reacting to a message, so a
// later change names only who is new.
func (r *reactState) remember(key string, rs []domain.Reactor) {
	if len(r.named)+len(rs) > maxReactors {
		r.named = map[string]bool{}
	}
	for _, x := range rs {
		r.named[reactorKey(key, x)] = true
	}
}

// reactorKey identifies one reactor's emoji on one message.
func reactorKey(key string, x domain.Reactor) string {
	who := x.ID
	if who == "" {
		who = "\x01" + x.Who
	}
	return key + "\x00" + who + "\x00" + x.Emoji
}

// reactInit starts the reaction polls when the service takes them and the
// background syncs run.
func (m *Model) reactInit() tea.Cmd {
	if _, ok := m.opt.Provider.(provider.ReactionPoller); !ok || m.opt.Poll <= 0 {
		return nil
	}
	return reactTick()
}

func reactTick() tea.Cmd {
	return tea.Tick(reactionPollEvery, func(time.Time) tea.Msg { return reactTickMsg{} })
}

// reactUpdate handles the poll's tick and answer; handled says whether
// msg was one of them. A refused chat is skipped for the session, a pause
// the service asks for stops every poll until it has passed, and nothing
// is said about either: a poll is never worth a notice.
func (m *Model) reactUpdate(msg tea.Msg) (cmd tea.Cmd, handled bool) {
	switch msg := msg.(type) {
	case reactTickMsg:
		m.react.due = !m.react.running // one call per tick: a poll still on its way takes this one
		return tea.Batch(reactTick(), m.pollReactions()), true
	case polledMsg:
		m.react.running = false
		var wait *provider.WaitError
		switch {
		case errors.Is(msg.err, provider.ErrNotPollable):
			m.react.refused[msg.chatID] = true
		case errors.As(msg.err, &wait):
			m.react.wait = m.now().Add(wait.Wait)
		}
		cmds := make([]tea.Cmd, 0, len(msg.changes))
		for _, c := range msg.changes { // applied like the pushed ones: one path
			cmds = append(cmds, m.applyChange(c))
		}
		return tea.Batch(cmds...), true
	}
	return nil, false
}

// pollReactions asks the service about the reactions on screen when a
// tick made a poll due (none does while a poll is on its way), unless a
// sync runs (the poll follows its end; on Telegram both need the one
// client) or the service asked for a pause. Nothing on screen, nothing
// asked.
func (m *Model) pollReactions() tea.Cmd {
	r := &m.react
	if !r.due || m.syncing {
		return nil
	}
	r.due = false
	p, ok := m.opt.Provider.(provider.ReactionPoller)
	if !ok || m.now().Before(r.wait) {
		return nil
	}
	chatID, ids := m.pollTarget()
	if len(ids) == 0 {
		return nil
	}
	r.running = true
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		changes, err := p.PollReactions(ctx, chatID, ids)
		return polledMsg{chatID: chatID, changes: changes, err: err}
	}
}

// pollTarget picks what one poll asks about: the messages on screen in
// the focused window that others sent (the service pushes the account's
// own), in a chat that did not refuse. A chat window asks about all of
// them; window 0 about those of one chat, the chats on screen in turn.
func (m *Model) pollTarget() (chatID string, ids []string) {
	r := &m.react
	if len(r.onScreen) == 0 {
		return "", nil
	}
	shown := make(map[string]bool, len(r.onScreen))
	for _, k := range r.onScreen {
		shown[k] = true
	}
	byChat := map[string][]string{}
	for _, x := range m.msgs {
		if shown[x.Key()] && x.Sender != "me" && !r.refused[x.ChatID] {
			byChat[x.ChatID] = append(byChat[x.ChatID], x.ID)
		}
	}
	if len(byChat) == 0 {
		return "", nil
	}
	chats := slices.Sorted(maps.Keys(byChat))
	chatID = chats[0]
	for _, c := range chats {
		if c > r.last {
			chatID = c
			break
		}
	}
	r.last = chatID
	return chatID, byChat[chatID]
}

// reacted applies the reactions a message has now, pushed or polled: the
// cache and the windows show them at once (Store.React keeps everything
// else), and window 0 tells of one someone added (reactionNotice). A
// poll's first word on a message whose reactions lazygram has not had
// from the service this session is a first sighting and says nothing: a
// cached copy may be days old, and a cache from before reactions knows
// none. A push is news whatever was known.
func (m *Model) reacted(c domain.Change) {
	r := &m.react
	msg := c.Msg
	msg.ChatID = c.ChatID
	key := msg.Key()
	before, now, held, err := m.opt.Store.React(msg)
	if err != nil {
		return
	}
	switch {
	case !held: // not cached: the last report stands for what was known
		before = domain.Message{ID: msg.ID, ChatID: msg.ChatID, Reactions: r.unheld[key]}
		now = before
		now.Reactions = domain.CurrentReactions(before, msg)
		if len(r.unheld) > maxReactors {
			r.unheld = map[string][]domain.Reaction{}
		}
		r.unheld[key] = now.Reactions
	case !slices.Equal(before.Reactions, now.Reactions):
		m.identify([]domain.Message{now})
	}
	if c.Own || r.learned[key] {
		if text := m.reactionNotice(c, before, now, held); text != "" {
			w := m.wins[0]
			m.note(w, text)
			w.notes[len(w.notes)-1].chat = true // chat activity: /feed off hides it
		}
	}
	r.learned[key] = true
	r.remember(key, c.Reactors)
}

// reactionNotice is what window 0 says about a message's reactions going
// from before to now, "" unless someone other than the account added one
// (a count of others rose, or a new emoji came). It names who reacted
// when the change names them and they are new here, at most as many per
// emoji as its count rose:
//
//	#dev: @grace reacted <e>, @alan reacted <e> to ada "first words"
//
// and otherwise gives the counts whole:
//
//	#news: <e> 13 <e> 4 on wire "first words"
func (m *Model) reactionNotice(c domain.Change, before, now domain.Message, held bool) string {
	rose := map[string]int{}
	for _, x := range now.Reactions {
		if n := others(x) - others(reactionOf(before.Reactions, x.Emoji)); n > 0 {
			rose[x.Emoji] = n
		}
	}
	if len(rose) == 0 {
		return ""
	}
	var named []string
	for _, x := range c.Reactors {
		if rose[x.Emoji] == 0 || x.Who == "me" || m.react.named[reactorKey(now.Key(), x)] {
			continue
		}
		rose[x.Emoji]--
		named = append(named, m.reactorName(x)+" reacted "+emojiText(x.Emoji))
	}
	where := m.nameOf(c.ChatID) + ": "
	if len(named) > 0 {
		return where + strings.Join(named, ", ") + " to " + reactedTo(now, held, c.Own)
	}
	return where + reactionCounts(now.Reactions) + " on " + reactedTo(now, held, c.Own)
}

// others is how many besides the account sent a reaction.
func others(x domain.Reaction) int {
	if x.Mine {
		return x.Count - 1
	}
	return x.Count
}

// reactionOf finds the reaction with the emoji; zero when there is none.
func reactionOf(rs []domain.Reaction, emoji string) domain.Reaction {
	for _, x := range rs {
		if x.Emoji == emoji {
			return x
		}
	}
	return domain.Reaction{}
}

// reactorName is how a notice names who reacted: as /names shows their
// chat when lazygram lists it, else by the name the service gave, with a
// person's @.
func (m *Model) reactorName(x domain.Reactor) string {
	if c, ok := m.chat(x.ID); ok && x.ID != "" {
		return m.display(c)
	}
	if who := strings.TrimSpace(sanitize(x.Who, false)); who != "" {
		return "@" + who
	}
	return "someone"
}

// reactedTo names the message a notice is about: its author, as lines
// name senders, and its first words ("your" for the account's own), or
// "a message" when the cache does not hold it ("your message" when the
// service pushed the change, which it does for the account's own).
func reactedTo(msg domain.Message, held, own bool) string {
	switch {
	case !held && own:
		return "your message"
	case !held:
		return "a message"
	}
	who := sanitize(msg.Sender, false)
	if msg.Sender == "me" {
		who = "your"
	}
	words := firstWords(msg)
	switch {
	case words != "":
		return who + ` "` + words + `"`
	case msg.Sender == "me":
		return "your message"
	}
	return "a message by " + who
}

// firstWords quotes a message in a notice: the start of its text, or its
// attachment token when it has none, on one line, cut at a word to about
// firstWordsMax columns with "...".
func firstWords(msg domain.Message) string {
	s := strings.Join(strings.Fields(sanitize(msg.Text, true)), " ")
	if s == "" && msg.Media != nil {
		s = mediaToken(msg.Media)
	}
	if lipgloss.Width(s) <= firstWordsMax {
		return s
	}
	head, rest := splitAt(s, firstWordsMax)
	if !strings.HasPrefix(rest, " ") { // cut inside a word: end at the one before
		if i := strings.LastIndex(head, " "); i > len(head)/2 {
			head = head[:i]
		}
	}
	return strings.TrimRight(head, " ,.;:!?") + "..."
}

// reactionCounts lists reactions for a notice: "<e> 13 <e> 4".
func reactionCounts(rs []domain.Reaction) string {
	parts := make([]string, 0, len(rs))
	for _, x := range rs {
		parts = append(parts, fmt.Sprintf("%s %d", emojiText(x.Emoji), x.Count))
	}
	return strings.Join(parts, " ")
}

// emojiText is how an emoji from the service is drawn: stripped of
// control characters and cut to its first character (a whole emoji
// sequence), so whatever the service sends takes one place; "?" when
// nothing visible is left.
func emojiText(e string) string {
	c, _, _, _ := uniseg.FirstGraphemeClusterInString(strings.TrimSpace(sanitize(e, false)), -1)
	if strings.TrimSpace(c) == "" {
		return "?"
	}
	return c
}

// withReactions puts a message's reactions after the last line of its
// text, which is used columns wide: sep, then "[<e> 3 <e> 1]" in the
// service's order, each emoji kept with its count and the account's own
// in the highlight style. What does not fit the width goes on rows of
// their own after indent, as the text wraps.
func withReactions(line string, used, width int, sep, indent string, rs []domain.Reaction) []string {
	out := []string{line}
	for i, x := range rs {
		open, shut := "", ""
		if i == 0 {
			open = "["
		}
		if i == len(rs)-1 {
			shut = "]"
		}
		core := emojiText(x.Emoji) + " " + strconv.Itoa(x.Count)
		w := lipgloss.Width(open + core + shut)
		if used+len(sep)+w > width { // a fresh row then takes it, whatever its width
			out = append(out, indent)
			used, sep = len(indent), ""
		}
		if x.Mine {
			core = hiStyle.Render(core)
		}
		out[len(out)-1] += sep + open + core + shut
		used += len(sep) + w
		sep = " "
	}
	return out
}
