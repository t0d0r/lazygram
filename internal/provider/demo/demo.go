// Package demo is an offline provider with synthetic data, so lazygram runs
// with no account. All names and text here are invented.
package demo

import (
	"context"
	"fmt"
	"time"

	"lazygram/internal/domain"
)

// Provider serves a scripted conversation. Early lines are already in the
// past; the rest "arrive" over the next minutes so refresh shows new lines.
type Provider struct {
	chats []domain.Chat
	msgs  []domain.Message
	now   func() time.Time
}

type line struct {
	chat   int
	offset time.Duration // relative to start; negative is the past
	sender string
	text   string
}

var chats = []domain.Chat{
	{ID: "demo-1", Service: "demo", Alias: "dev", Title: "Demo Developers"},
	{ID: "demo-2", Service: "demo", Alias: "ops", Title: "Demo Operations"},
	{ID: "demo-3", Service: "demo", Alias: "friends", Title: "Demo Friends"},
	{ID: "demo-4", Service: "demo", Alias: "news", Title: "Demo Newsroom"},
}

var script = []line{
	{0, -95 * time.Minute, "ada", "morning, the build is green again"},
	{1, -92 * time.Minute, "grace", "nightly backup finished in 14m"},
	{2, -90 * time.Minute, "linus", "anyone up for lunch later?"},
	{0, -88 * time.Minute, "alan", "I will review the parser change after standup"},
	{3, -85 * time.Minute, "wire", "Demo headline: local terminal clients make a comeback"},
	{1, -80 * time.Minute, "ken", "disk usage on the demo host is at 71%"},
	{2, -78 * time.Minute, "margaret", "yes, 12:30 at the usual place"},
	{0, -70 * time.Minute, "ada", "pushed a fix for the scrollback off-by-one"},
	{0, -69 * time.Minute, "grace", "nice, tests pass here too"},
	{1, -60 * time.Minute, "dennis", "rotating the demo logs now"},
	{3, -55 * time.Minute, "wire", "Demo headline: IRC-style feeds are still popular with power users"},
	{2, -50 * time.Minute, "linus", "running 5 minutes late"},
	{0, -40 * time.Minute, "alan", "left two comments on the PR, mostly naming"},
	{1, -35 * time.Minute, "grace", "alert: demo queue depth above threshold"},
	{1, -34 * time.Minute, "ken", "looking into the queue depth alert"},
	{1, -30 * time.Minute, "ken", "false alarm, a batch job finished late"},
	{2, -25 * time.Minute, "margaret", "photos from the weekend are in the shared album"},
	{0, -20 * time.Minute, "ada", "who owns the history pager spec?"},
	{0, -19 * time.Minute, "alan", "I do, draft is in specs/"},
	{3, -12 * time.Minute, "wire", "Demo headline: terminal UIs gain search-first pagers"},
	{2, -8 * time.Minute, "linus", "back from lunch, what did I miss"},
	{0, -5 * time.Minute, "grace", "release candidate tagged"},
	{1, -2 * time.Minute, "dennis", "deploy window opens in 30 minutes"},
	{0, 20 * time.Second, "ada", "tip: press Tab to cycle through chats"},
	{2, 45 * time.Second, "margaret", "try /history to page through everything"},
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
		p.msgs = append(p.msgs, domain.Message{
			ID:     fmt.Sprintf("%d", i+1),
			ChatID: chats[l.chat].ID,
			Sender: l.sender,
			Text:   l.text,
			Time:   start.Add(l.offset),
		})
	}
	return p
}

// Name implements provider.Provider.
func (p *Provider) Name() string { return "demo" }

// Chats implements provider.Provider.
func (p *Provider) Chats(context.Context) ([]domain.Chat, error) {
	return append([]domain.Chat(nil), p.chats...), nil
}

// Sync implements provider.Provider: messages after since and not in the future.
func (p *Provider) Sync(_ context.Context, since time.Time) ([]domain.Message, error) {
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
