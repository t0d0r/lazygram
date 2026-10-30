package demo

import (
	"context"
	"testing"
	"time"
)

func TestSyncRevealsScriptOverTime(t *testing.T) {
	clock := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	p := newAt(func() time.Time { return clock })
	first, _ := p.Sync(context.Background(), time.Time{})
	if len(first) == 0 || len(first) >= len(script) {
		t.Fatalf("initial backlog should be partial, got %d of %d", len(first), len(script))
	}
	last := first[len(first)-1].Time
	again, _ := p.Sync(context.Background(), last)
	if len(again) != 0 {
		t.Fatalf("nothing new yet, got %d", len(again))
	}
	clock = clock.Add(10 * time.Minute)
	rest, _ := p.Sync(context.Background(), last)
	if len(first)+len(rest) != len(script) {
		t.Fatalf("all lines should be visible later: %d+%d != %d", len(first), len(rest), len(script))
	}
}

func TestChatsKnown(t *testing.T) {
	p := New()
	cs, err := p.Chats(context.Background())
	if err != nil || len(cs) != len(chats) {
		t.Fatal("chats")
	}
	known := map[string]bool{}
	for _, c := range cs {
		known[c.ID] = true
	}
	for _, m := range p.msgs {
		if !known[m.ChatID] {
			t.Fatalf("message references unknown chat %q", m.ChatID)
		}
	}
}
