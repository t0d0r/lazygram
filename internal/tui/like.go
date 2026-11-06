package tui

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lazygram/internal/domain"
	"lazygram/internal/provider"
)

// /like sets the account's reaction on one message of the current chat
// window (specs/ui.md, Reactions, Sending a reaction): the newest message
// lazygram holds of that chat that someone else sent
// (domain.LastFromOthers). The reaction is named from a fixed vocabulary;
// each emoji is in Telegram's standard reaction set exactly as listed
// there (the Bot API's ReactionTypeEmoji: the heart has no U+FE0F).

// likeName is one reaction /like takes: its name, an alias ("" for
// none) and the emoji sent.
type likeName struct {
	name, also, emoji string
}

// likeNames is the vocabulary, in the order /help and the error list it;
// the first is what /like alone sends.
var likeNames = []likeName{
	{"like", "+1", "\U0001F44D"},    // thumbs up
	{"dislike", "-1", "\U0001F44E"}, // thumbs down
	{"love", "heart", "\u2764"},     // red heart
	{"fire", "", "\U0001F525"},
	{"laugh", "", "\U0001F601"}, // beaming face with smiling eyes
	{"sad", "", "\U0001F622"},   // crying face
	{"party", "tada", "\U0001F389"},
	{"clap", "", "\U0001F44F"},
	{"thanks", "pray", "\U0001F64F"}, // folded hands
	{"ok", "", "\U0001F44C"},
	{"think", "", "\U0001F914"},
	{"eyes", "", "\U0001F440"},
}

// likeWords are the names Tab offers after /like: not the aliases.
func likeWords() []string {
	out := make([]string, len(likeNames))
	for i, n := range likeNames {
		out[i] = n.name
	}
	return out
}

// likeNamesErr is /like's error for anything but a name: "/like takes
// like, dislike, ... think or eyes".
func likeNamesErr() string {
	w := likeWords()
	return "/like takes " + strings.Join(w[:len(w)-1], ", ") + " or " + w[len(w)-1]
}

// likeEmoji is the emoji a /like argument names, matched without regard
// to case; no argument is like. ok is false for anything else.
func likeEmoji(arg string) (string, bool) {
	if arg == "" {
		return likeNames[0].emoji, true
	}
	arg = strings.ToLower(arg)
	for _, n := range likeNames {
		if arg == n.name || (n.also != "" && arg == n.also) {
			return n.emoji, true
		}
	}
	return "", false
}

// likedMsg is the answer to /like: what was reacted to, as the status
// says it, and the reactions the service gave back.
type likedMsg struct {
	what    string
	changes []domain.Change
	err     error
}

// like runs /like. The checks go from the form to the state, and each
// sends nothing: an unknown name, window 0, a provider without
// reactions, no message from someone else. Then the request goes out in
// the background; nothing is focused or read.
func (m *Model) like(arg string) tea.Cmd {
	emoji, ok := likeEmoji(arg)
	if !ok {
		m.setErr(likeNamesErr())
		return nil
	}
	chatID := m.wins[m.cur].chatID
	if chatID == "" {
		m.setErr("the server window has no chat; /win to a chat first")
		return nil
	}
	r, ok := m.opt.Provider.(provider.Reactor)
	if !ok {
		m.setErr("reactions cannot be sent with " + m.opt.Provider.Name())
		return nil
	}
	target, ok := domain.LastFromOthers(m.msgs, chatID)
	if !ok {
		m.setErr("no message from someone else in " + m.nameOf(chatID) + " to react to")
		return nil
	}
	what := emojiText(emoji) + " to " + reactedTo(target, true, false)
	m.setInfo("reacting " + what + "...")
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		changes, err := r.React(ctx, target.ChatID, target.ID, emoji)
		return likedMsg{what: what, changes: changes, err: err}
	}
}

// liked applies /like's answer: a failure is told as every failed
// request is; the reactions the service gave back go the way a poll's do.
func (m *Model) liked(msg likedMsg) tea.Cmd {
	if msg.err != nil {
		m.fail("reaction failed: " + errText(msg.err))
		return nil
	}
	cmds := make([]tea.Cmd, 0, len(msg.changes))
	for _, c := range msg.changes {
		cmds = append(cmds, m.applyChange(c))
	}
	m.setInfo("reacted " + msg.what)
	return tea.Batch(cmds...)
}
