package demo

import (
	"bytes"
	"context"
	"image/png"
	"strings"
	"testing"
	"time"

	"lazygram/internal/domain"
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

func TestRecent(t *testing.T) {
	clock := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	p := newAt(func() time.Time { return clock })
	got, err := p.Recent(context.Background(), "demo-1", 2)
	if err != nil || len(got) != 2 || got[0].ChatID != "demo-1" || !got[0].Time.Before(got[1].Time) {
		t.Fatalf("%v %v", got, err)
	}
	for _, m := range got {
		if m.Time.After(clock) {
			t.Fatal("future lines must not be revealed")
		}
	}
	if got, _ := p.Recent(context.Background(), "nope", 5); len(got) != 0 {
		t.Fatal("unknown chat")
	}
}

func TestSendShowsUpOnNextSync(t *testing.T) {
	clock := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	p := newAt(func() time.Time { return clock })
	first, _ := p.Sync(context.Background(), time.Time{})
	if err := p.Send(context.Background(), "demo-3", "hello"); err != nil {
		t.Fatal(err)
	}
	if err := p.Send(context.Background(), "nope", "x"); err == nil {
		t.Fatal("unknown chat")
	}
	got, _ := p.Sync(context.Background(), first[len(first)-1].Time)
	if len(got) != 1 || got[0].Sender != "me" || got[0].Text != "hello" || got[0].ChatID != "demo-3" {
		t.Fatalf("%+v", got)
	}
}

func TestPressAnswersAndEditsTheMessage(t *testing.T) {
	clock := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	p := newAt(func() time.Time { return clock })
	all, _ := p.Sync(context.Background(), time.Time{})
	var bot domain.Message
	for _, m := range all {
		if m.ChatID == "demo-5" {
			bot = m
		}
	}
	if len(bot.Buttons) != 3 {
		t.Fatalf("bot message buttons: %+v", bot)
	}
	a, err := p.Press(context.Background(), "demo-5", bot.ID, bot.Buttons[0])
	if err != nil || a.Text == "" || a.Updated == nil || len(a.Updated.Buttons) != 0 || !strings.Contains(a.Updated.Text, "Deploy") {
		t.Fatalf("%+v %v", a, err)
	}
	if _, err := p.Press(context.Background(), "demo-5", bot.ID, bot.Buttons[1]); err == nil {
		t.Fatal("buttons are gone after the edit")
	}
}

func TestFetchReturnsAPNG(t *testing.T) {
	clock := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	p := newAt(func() time.Time { return clock })
	all, _ := p.Sync(context.Background(), time.Time{})
	var photo domain.Message
	for _, m := range all {
		if m.Media != nil {
			photo = m
		}
	}
	if photo.ID == "" || photo.Media.Kind != "photo" {
		t.Fatal("demo should carry one photo")
	}
	data, name, err := p.Fetch(context.Background(), photo.ChatID, photo.ID)
	if err != nil || !strings.HasSuffix(name, ".png") {
		t.Fatal(err, name)
	}
	if _, err := png.Decode(bytes.NewReader(data)); err != nil {
		t.Fatal("not a PNG:", err)
	}
	if _, _, err := p.Fetch(context.Background(), "demo-1", "1"); err == nil {
		t.Fatal("a message without media")
	}
}

func TestEditAndDeleteOwnLines(t *testing.T) {
	clock := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	p := newAt(func() time.Time { return clock })
	all, _ := p.Sync(context.Background(), time.Time{})
	var mine, theirs domain.Message
	for _, m := range all {
		if m.ChatID == "demo-3" && m.Sender == "me" {
			mine = m
		}
		if m.ChatID == "demo-3" && m.Sender != "me" {
			theirs = m
		}
	}
	if err := p.Edit(context.Background(), mine.ChatID, mine.ID, "changed"); err != nil {
		t.Fatal(err)
	}
	if err := p.Edit(context.Background(), theirs.ChatID, theirs.ID, "nope"); err == nil {
		t.Fatal("only own lines")
	}
	if err := p.Delete(context.Background(), mine.ChatID, mine.ID); err != nil {
		t.Fatal(err)
	}
	for _, m := range p.msgs {
		if m.ID == mine.ID {
			t.Fatal("deleted line still there")
		}
	}
}

func TestSendFileShowsUpAndFetchesBack(t *testing.T) {
	clock := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	p := newAt(func() time.Time { return clock })
	first, _ := p.Sync(context.Background(), time.Time{})
	if err := p.SendFile(context.Background(), "demo-3", "cat.png", []byte("PNGBYTES"), "look"); err != nil {
		t.Fatal(err)
	}
	got, _ := p.Sync(context.Background(), first[len(first)-1].Time)
	if len(got) != 1 || got[0].Media == nil || got[0].Media.Kind != "photo" || got[0].Text != "look" || got[0].Sender != "me" {
		t.Fatalf("%+v", got)
	}
	data, name, err := p.Fetch(context.Background(), "demo-3", got[0].ID)
	if err != nil || string(data) != "PNGBYTES" || name != "cat.png" {
		t.Fatalf("%q %q %v", data, name, err)
	}
	if err := p.SendFile(context.Background(), "nope", "a.txt", nil, ""); err == nil {
		t.Fatal("unknown chat")
	}
}
