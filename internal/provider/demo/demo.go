// Package demo is an offline provider with synthetic data, so lazygram runs
// with no account. All names and text here are invented.
package demo

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"sync"
	"time"

	"lazygram/internal/domain"
	"lazygram/internal/provider"
)

// Provider serves a scripted conversation. Early lines are already in the
// past; the rest "arrive" over the next minutes so new windows open while
// you watch.
type Provider struct {
	mu    sync.Mutex
	chats []domain.Chat
	msgs  []domain.Message
	now   func() time.Time
	sent  int
	files map[string][]byte // attachments sent from here, by message id

	start    time.Time
	reacted  int                         // how many of later have been sent
	reactors map[string][]domain.Reactor // who reacted while the demo ran, newest first, by message key
	changes  chan domain.Change          // reactions to the account's own lines, pushed
}

// Reaction emoji the demo uses, as Telegram sends them.
const (
	thumbsUp = "\U0001F44D"
	heart    = "\u2764"
	fire     = "\U0001F525"
	party    = "\U0001F389"
	star     = "\u2b50" // a paid reaction
)

// startReactions are the reactions lines already carry, by the line's
// text.
var startReactions = map[string][]domain.Reaction{
	"pushed a fix for the scrollback off-by-one": {{Emoji: thumbsUp, Count: 2}},
	"release candidate tagged":                   {{Emoji: party, Count: 3}, {Emoji: thumbsUp, Count: 1, Mine: true}},
	"the place looks like this today":            {{Emoji: heart, Count: 1, Mine: true}},
	"Demo headline: IRC-style feeds are still popular with power users": {
		{Emoji: fire, Count: 12}, {Emoji: heart, Count: 4}, {Emoji: domain.CustomReaction, Count: 2}, {Emoji: star, Count: 1}},
}

// reaction is one reaction sent while the demo runs.
type reaction struct {
	after time.Duration // after start
	text  string        // the line it is sent to
	who   string
	emoji string
}

// later are the reactions sent while the demo runs, in time order: one to
// the account's own line, which the service pushes on the change feed as
// Telegram does, and one to someone else's, which only a poll finds.
var later = []reaction{
	{25 * time.Second, "no problem, ordering for you", "linus", thumbsUp},
	{100 * time.Second, "tip: /win 2 switches windows, Tab completes names", "grace", thumbsUp},
}

// changeBuffer is how many pushed reactions wait for the reader.
const changeBuffer = 16

type line struct {
	chat   int
	offset time.Duration // relative to start; negative is the past
	sender string
	text   string
}

var chats = []domain.Chat{
	{ID: "demo-1", Service: "demo", Alias: "dev", Title: "Demo Developers", Kind: domain.ChatGroup},
	{ID: "demo-2", Service: "demo", Alias: "ops", Title: "Demo Operations", Kind: domain.ChatGroup},
	{ID: "demo-3", Service: "demo", Alias: "linus", Title: "Linus Demo", Kind: domain.ChatUser},
	{ID: "demo-4", Service: "demo", Alias: "news", Title: "Demo Newsroom", Kind: domain.ChatChannel, Unread: 2},
	{ID: "demo-5", Service: "demo", Alias: "demobot", Title: "Demo Bot", Kind: domain.ChatUser},
}

// botButtons is the inline keyboard under the demo bot's question.
var botButtons = []domain.Button{
	{Label: "Deploy", Kind: domain.ButtonCallback, Data: []byte("deploy")},
	{Label: "Hold", Kind: domain.ButtonCallback, Data: []byte("hold")},
	{Label: "Runbook", Kind: domain.ButtonURL, URL: "https://example.invalid/runbook"},
}

var script = []line{
	{0, -95 * time.Minute, "ada", "morning, the build is green again"},
	{1, -92 * time.Minute, "grace", "nightly backup finished in 14m"},
	{2, -90 * time.Minute, "linus", "anyone up for lunch later?"},
	{0, -88 * time.Minute, "alan", "I will review the parser change after standup"},
	{3, -85 * time.Minute, "wire", "Demo headline: local terminal clients make a comeback"},
	{1, -80 * time.Minute, "ken", "disk usage on the demo host is at 71%"},
	{2, -78 * time.Minute, "me", "yes, 12:30 at the usual place"},
	{0, -70 * time.Minute, "ada", "pushed a fix for the scrollback off-by-one"},
	{0, -69 * time.Minute, "grace", "nice, tests pass here too"},
	{1, -60 * time.Minute, "dennis", "rotating the demo logs now"},
	{3, -55 * time.Minute, "wire", "Demo headline: IRC-style feeds are still popular with power users"},
	{2, -50 * time.Minute, "linus", "running 5 minutes late"},
	{0, -40 * time.Minute, "alan", "left two comments on the PR, mostly naming"},
	{1, -35 * time.Minute, "grace", "alert: demo queue depth above threshold"},
	{1, -34 * time.Minute, "ken", "looking into the queue depth alert"},
	{1, -30 * time.Minute, "ken", "false alarm, a batch job finished late"},
	{2, -25 * time.Minute, "me", "no problem, ordering for you"},
	{2, -24 * time.Minute, "linus", "the place looks like this today"},
	{0, -20 * time.Minute, "ada", "who owns the history pager spec?"},
	{0, -19 * time.Minute, "alan", "I do, draft is in specs/"},
	{3, -12 * time.Minute, "wire", "Demo headline: terminal UIs gain search-first pagers"},
	{2, -8 * time.Minute, "linus", "back from lunch, what did I miss"},
	{0, -5 * time.Minute, "grace", "release candidate tagged"},
	{1, -2 * time.Minute, "dennis", "deploy window opens in 30 minutes"},
	{4, -90 * time.Second, "demobot", "Release candidate is green. Deploy it now? Use /press 1 or /press 2."},
	{3, -1 * time.Minute, "wire", "Demo digest: three stories today.\n\n1. Terminal clients keep their users.\n2. Encrypted local caches become the norm.\n3. Nobody misses the gap under the nick.\n\nThat is all for this hour, back at the top of the next one with more invented news for the demo feed."},
	{0, 20 * time.Second, "ada", "tip: /win 2 switches windows, Tab completes names"},
	{2, 45 * time.Second, "linus", "try /win @linus to open my window"},
	{3, 90 * time.Second, "wire", "Demo headline: new lines keep arriving while you read"},
	{1, 150 * time.Second, "ken", "all quiet on the demo host"},
	{0, 210 * time.Second, "alan", "try /search release to find a line in history"},
}

// New returns a demo provider anchored at the current time.
func New() *Provider { return newAt(time.Now) }

func newAt(now func() time.Time) *Provider {
	start := now().Truncate(time.Second)
	p := &Provider{chats: chats, now: now, files: map[string][]byte{}, start: start,
		reactors: map[string][]domain.Reactor{}, changes: make(chan domain.Change, changeBuffer)}
	for i, l := range script {
		m := domain.Message{
			ID:     fmt.Sprintf("%d", i+1),
			ChatID: chats[l.chat].ID,
			Sender: l.sender,
			Text:   l.text,
			Time:   start.Add(l.offset),
		}
		if chats[l.chat].Kind == domain.ChatGroup { // group members have ids and usernames, like most on Telegram
			m.SenderID, m.SenderNick = "demo-user-"+l.sender, l.sender
		}
		if l.sender == "demobot" {
			m.Buttons = append([]domain.Button(nil), botButtons...)
		}
		if l.text == "the place looks like this today" {
			m.Media = &domain.Media{Kind: "photo"}
		}
		m.Reactions = append([]domain.Reaction(nil), startReactions[l.text]...)
		p.msgs = append(p.msgs, m)
	}
	return p
}

// Changes implements provider.ChangeSource: the reactions to the
// account's own lines, pushed when they are sent. The feed never closes.
func (p *Provider) Changes() <-chan domain.Change { return p.changes }

// reactLocked sends the reactions of later whose time has come: each one
// counts on its line (in a new slice, so copies already handed out stay as
// they were) and is remembered with who sent it; one to the account's own
// line is pushed on the change feed, dropped when the reader is behind.
// The demo's clock moves when it is asked something, so Sync, Recent and
// PollReactions call this first. p.mu is held.
func (p *Provider) reactLocked(now time.Time) {
	for p.reacted < len(later) && !p.start.Add(later[p.reacted].after).After(now) {
		r := later[p.reacted]
		p.reacted++
		for i := range p.msgs {
			m := &p.msgs[i]
			if m.Text != r.text {
				continue
			}
			m.Reactions = withReaction(m.Reactions, r.emoji)
			who := domain.Reactor{Who: r.who, ID: "demo-user-" + r.who, Emoji: r.emoji, At: p.start.Add(r.after)}
			p.reactors[m.Key()] = append([]domain.Reactor{who}, p.reactors[m.Key()]...)
			if m.Sender == "me" {
				select {
				case p.changes <- p.reactedLocked(*m, true):
				default:
				}
			}
			break
		}
	}
}

// reactedLocked is the Reacted change that reports m's reactions as they
// are now. p.mu is held.
func (p *Provider) reactedLocked(m domain.Message, own bool) domain.Change {
	return domain.Change{
		Kind: domain.Reacted, ChatID: m.ChatID,
		Msg:      domain.Message{ID: m.ID, ChatID: m.ChatID, Reactions: append([]domain.Reaction(nil), m.Reactions...)},
		Reactors: append([]domain.Reactor(nil), p.reactors[m.Key()]...),
		Own:      own,
	}
}

// withReaction returns rs with one more emoji, counted on its entry or
// added at the end, without touching rs.
func withReaction(rs []domain.Reaction, emoji string) []domain.Reaction {
	out := append([]domain.Reaction(nil), rs...)
	for i := range out {
		if out[i].Emoji == emoji {
			out[i].Count++
			return out
		}
	}
	return append(out, domain.Reaction{Emoji: emoji, Count: 1})
}

// PollReactions implements provider.ReactionPoller: one Reacted change per
// asked-for line that is already there, with its reactions as they are now
// and who reacted while the demo ran.
func (p *Provider) PollReactions(_ context.Context, chatID string, ids []string) ([]domain.Change, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.now()
	p.reactLocked(now)
	asked := map[string]bool{}
	for _, id := range ids {
		asked[id] = true
	}
	var out []domain.Change
	for _, m := range p.msgs {
		if m.ChatID == chatID && asked[m.ID] && !m.Time.After(now) {
			out = append(out, p.reactedLocked(m, false))
		}
	}
	return out, nil
}

// Name implements provider.Provider.
func (p *Provider) Name() string { return "demo" }

// Chats implements provider.Provider.
func (p *Provider) Chats(context.Context) ([]domain.Chat, error) {
	return append([]domain.Chat(nil), p.chats...), nil
}

// Sync implements provider.Provider: messages after since and not in the
// future. The demo replays its past on the first call so the server window
// has something to show.
func (p *Provider) Sync(_ context.Context, since time.Time) ([]domain.Message, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.now()
	p.reactLocked(now)
	var out []domain.Message
	for _, m := range p.msgs {
		if m.Time.After(now) || !m.Time.After(since) {
			continue
		}
		out = append(out, m)
	}
	return out, nil
}

// Recent implements provider.Provider.
func (p *Provider) Recent(_ context.Context, chatID string, limit int) ([]domain.Message, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := p.now()
	p.reactLocked(now)
	var out []domain.Message
	for _, m := range p.msgs {
		if m.ChatID == chatID && !m.Time.After(now) {
			out = append(out, m)
		}
	}
	return domain.Tail(out, limit), nil
}

// Send implements provider.Provider: the message shows up on the next Sync
// as a line from "me".
func (p *Provider) Send(_ context.Context, chatID, text string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	known := false
	for _, c := range p.chats {
		known = known || c.ID == chatID
	}
	if !known {
		return fmt.Errorf("demo: unknown chat %s", chatID)
	}
	p.sent++
	p.msgs = append(p.msgs, domain.Message{
		ID: fmt.Sprintf("sent-%d", p.sent), ChatID: chatID, Sender: "me", Text: text, Time: p.now(),
	})
	return nil
}

// Press implements provider.Provider: the bot answers and edits its message
// to show the choice, as real bots do.
func (p *Provider) Press(_ context.Context, chatID, msgID string, b domain.Button) (domain.PressAnswer, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.msgs {
		m := &p.msgs[i]
		if m.ChatID != chatID || m.ID != msgID {
			continue
		}
		if len(m.Buttons) == 0 {
			return domain.PressAnswer{}, fmt.Errorf("demo: message %s has no buttons any more", msgID)
		}
		m.Text = "Release candidate is green. You chose: " + b.Label + "."
		m.Buttons = nil
		updated := *m
		return domain.PressAnswer{Text: "Got it: " + string(b.Data), Updated: &updated}, nil
	}
	return domain.PressAnswer{}, fmt.Errorf("demo: no message %s in %s", msgID, chatID)
}

// Fetch implements provider.Provider: it draws a small PNG in memory, so
// the preview flow works without a service and without writing anything.
func (p *Provider) Fetch(_ context.Context, chatID, msgID string) ([]byte, string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, m := range p.msgs {
		if m.ChatID != chatID || m.ID != msgID {
			continue
		}
		if m.Media == nil {
			return nil, "", fmt.Errorf("demo: message %s has no attachment", msgID)
		}
		if data, ok := p.files[msgID]; ok { // something sent from here
			return data, m.Media.Name, nil
		}
		img := image.NewRGBA(image.Rect(0, 0, 160, 100))
		for y := 0; y < 100; y++ {
			for x := 0; x < 160; x++ {
				img.Set(x, y, color.RGBA{uint8(x * 255 / 160), uint8(y * 255 / 100), 160, 255})
			}
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return nil, "", err
		}
		return buf.Bytes(), msgID + ".png", nil
	}
	return nil, "", fmt.Errorf("demo: no message %s in %s", msgID, chatID)
}

// Edit implements provider.Provider for the account's own lines.
func (p *Provider) Edit(_ context.Context, chatID, msgID, text string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.msgs {
		if p.msgs[i].ChatID == chatID && p.msgs[i].ID == msgID {
			if p.msgs[i].Sender != "me" {
				return fmt.Errorf("demo: message %s is not yours", msgID)
			}
			p.msgs[i].Text = text
			return nil
		}
	}
	return fmt.Errorf("demo: no message %s in %s", msgID, chatID)
}

// Members implements provider.MemberLister: who is in the demo groups,
// with margaret, who never writes, and barbara, who has no username, in
// #dev; the channel hides its subscribers like any broadcast channel.
func (p *Provider) Members(_ context.Context, chatID string) ([]domain.Participant, error) {
	person := func(nick string) domain.Participant {
		return domain.Participant{ID: "demo-user-" + nick, Name: nick, Nick: nick}
	}
	switch chatID {
	case "demo-1":
		return []domain.Participant{person("ada"), person("alan"), person("grace"),
			{ID: "demo-user-margaret", Name: "margaret", Nick: "margaret_h"},
			{ID: "demo-user-barbara", Name: "barbara"}}, nil
	case "demo-2":
		return []domain.Participant{person("grace"), person("ken"), person("dennis")}, nil
	case "demo-4":
		return nil, provider.ErrMembersHidden
	}
	return nil, fmt.Errorf("demo: %s is not a group", chatID)
}

// Delete implements provider.Provider for the account's own lines.
// MarkRead implements provider.Provider: the demo has nobody to tell.
func (p *Provider) MarkRead(context.Context, string, string) error { return nil }

// MarkUnread implements provider.Provider: the demo keeps no marks.
func (p *Provider) MarkUnread(context.Context, string, bool) error { return nil }

func (p *Provider) Delete(_ context.Context, chatID, msgID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.msgs {
		if p.msgs[i].ChatID == chatID && p.msgs[i].ID == msgID {
			if p.msgs[i].Sender != "me" {
				return fmt.Errorf("demo: message %s is not yours", msgID)
			}
			p.msgs = append(p.msgs[:i], p.msgs[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("demo: no message %s in %s", msgID, chatID)
}

// SendFile implements provider.Provider: the attachment becomes a line
// from "me" that the next Sync returns and Fetch serves back.
func (p *Provider) SendFile(_ context.Context, chatID, name string, data []byte, caption string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	known := false
	for _, c := range p.chats {
		known = known || c.ID == chatID
	}
	if !known {
		return fmt.Errorf("demo: unknown chat %s", chatID)
	}
	p.sent++
	id := fmt.Sprintf("sent-%d", p.sent)
	kind := "file"
	if domain.IsImageName(name) {
		kind = "photo"
	}
	p.files[id] = data
	p.msgs = append(p.msgs, domain.Message{
		ID: id, ChatID: chatID, Sender: "me", Text: caption, Time: p.now(),
		Media: &domain.Media{Kind: kind, Name: name},
	})
	return nil
}
