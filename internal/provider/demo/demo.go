// Package demo is an offline provider with synthetic data, so lazygram runs
// with no account. All names and text here are invented.
package demo

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"time"

	"lazygram/internal/domain"
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
}

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
	p := &Provider{chats: chats, now: now}
	for i, l := range script {
		m := domain.Message{
			ID:     fmt.Sprintf("%d", i+1),
			ChatID: chats[l.chat].ID,
			Sender: l.sender,
			Text:   l.text,
			Time:   start.Add(l.offset),
		}
		if l.sender == "demobot" {
			m.Buttons = append([]domain.Button(nil), botButtons...)
		}
		if l.text == "the place looks like this today" {
			m.Media = &domain.Media{Kind: "photo"}
		}
		p.msgs = append(p.msgs, m)
	}
	return p
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

// Fetch implements provider.Provider: it draws a small PNG into a private
// temporary directory, so the preview flow works without a service.
func (p *Provider) Fetch(_ context.Context, chatID, msgID string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, m := range p.msgs {
		if m.ChatID != chatID || m.ID != msgID {
			continue
		}
		if m.Media == nil {
			return "", fmt.Errorf("demo: message %s has no attachment", msgID)
		}
		dir := filepath.Join(os.TempDir(), "lazygram-demo")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return "", err
		}
		path := filepath.Join(dir, msgID+".png")
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
		img := image.NewRGBA(image.Rect(0, 0, 160, 100))
		for y := 0; y < 100; y++ {
			for x := 0; x < 160; x++ {
				img.Set(x, y, color.RGBA{uint8(x * 255 / 160), uint8(y * 255 / 100), 160, 255})
			}
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			return "", err
		}
		if err := png.Encode(f, img); err != nil {
			_ = f.Close()
			return "", err
		}
		return path, f.Close()
	}
	return "", fmt.Errorf("demo: no message %s in %s", msgID, chatID)
}
