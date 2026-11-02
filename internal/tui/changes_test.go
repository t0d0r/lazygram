package tui

import (
	"strings"
	"testing"
	"time"

	"lazygram/internal/domain"
)

// changingFake is a fake whose service reports edits and deletions.
type changingFake struct {
	*fake
	ch chan domain.Change
}

func (f *changingFake) Changes() <-chan domain.Change { return f.ch }

func TestLiveEditsAndDeletions(t *testing.T) {
	f := &changingFake{fake: newFake(), ch: make(chan domain.Change, 4)}
	f.chats[1].SharedIDs = true // @alice: a private chat, shared ids
	f.chats[0].SharedIDs = true // #dev: a basic group
	m := setupProvider(t, f)
	m.Init()
	if m.changeCh == nil {
		t.Fatal("the change feed is read")
	}
	enter(m, "/win #dev")
	m.Update(changeMsg{c: domain.Change{Kind: domain.Edited, ChatID: "c1",
		Msg: domain.Message{ID: "1", ChatID: "c1", Sender: "user", Text: "first dev line, fixed"}}})
	v := body(m)
	if !strings.Contains(v, "<ada> first dev line, fixed") {
		t.Fatalf("the edit shows, sender kept:\n%s", v)
	}
	cached, _ := m.opt.Store.Messages("c1", 0)
	if cached[0].Text != "first dev line, fixed" || cached[0].Sender != "ada" {
		t.Fatalf("the cache has it: %+v", cached[0])
	}
	m.Update(changeMsg{c: domain.Change{Kind: domain.Edited, ChatID: "c1", Msg: domain.Message{ID: "99", ChatID: "c1", Text: "x"}}})
	if strings.Contains(body(m), "x\n") {
		t.Fatal("an edit of a message not held adds nothing")
	}
	// A deletion the service names without a chat: the shared id space.
	if cmd := m.applyChange(domain.Change{Kind: domain.Deleted, IDs: []string{"2"}}); cmd != nil {
		cmd()
	}
	if strings.Contains(body(m), "second dev line") {
		t.Fatalf("deleted in #dev:\n%s", body(m))
	}
	if cached, _ := m.opt.Store.Messages("c1", 0); len(cached) != 1 {
		t.Fatalf("gone from the cache: %+v", cached)
	}
	m.input = nil
	enter(m, "/win @alice")
	if cmd := m.applyChange(domain.Change{Kind: domain.Deleted, ChatID: "c2", IDs: []string{"7"}}); cmd != nil {
		cmd()
	}
	if strings.Contains(body(m), "deploy?") {
		t.Fatalf("deleted in a named chat:\n%s", body(m))
	}
	if !strings.Contains(body(m), "hi there") {
		t.Fatalf("the rest stays:\n%s", body(m))
	}
	m.Update(changeDoneMsg{})
	if m.changeCh != nil {
		t.Fatal("a closed feed is let go")
	}
}

func TestFetchHealsEditsAndDeletions(t *testing.T) {
	f := newFake() // #dev holds 1 and 2
	f.recent["c1"] = []domain.Message{
		mk("c1", "1", "ada", "first dev line (edited)", 0),
		mk("c1", "3", "alan", "third", 3), // 2 was deleted meanwhile
	}
	m := setup(t, f)
	runAll(m, enter(m, "/win #dev"))
	v := body(m)
	if !strings.Contains(v, "first dev line (edited)") || strings.Contains(v, "second dev line") || !strings.Contains(v, "third") {
		t.Fatalf("the window reads as the service does:\n%s", v)
	}
	cached, _ := m.opt.Store.Messages("c1", 0)
	if len(cached) != 2 || cached[0].Text != "first dev line (edited)" {
		t.Fatalf("and so does the cache: %+v", cached)
	}
}

func TestDeletionTakesBackItsUnreadAndReachesOldChats(t *testing.T) {
	f := &changingFake{fake: newFake(), ch: make(chan domain.Change, 1)}
	f.chats[1].SharedIDs = true
	m := setupProvider(t, f)
	m.sharedChats["c-old"] = true // a private chat no longer among the recent dialogs
	if _, err := m.opt.Store.Add([]domain.Message{mk("c-old", "50", "ann", "old line", -100)}); err != nil {
		t.Fatal(err)
	}
	_, w := m.winFor("c2")
	w.unread = 2
	m.live[domain.Message{ChatID: "c2", ID: "1"}.Key()] = true // arrived this session
	if cmd := m.applyChange(domain.Change{Kind: domain.Deleted, IDs: []string{"1", "50"}}); cmd != nil {
		cmd()
	}
	if w.unread != 1 {
		t.Errorf("the deleted message is no longer unread: %d", w.unread)
	}
	if left, _ := m.opt.Store.Messages("c-old", 0); len(left) != 0 {
		t.Errorf("a kept shared chat is reached: %+v", left)
	}
}

func TestHealFetchesAShownWindowAgain(t *testing.T) {
	f := newFake()
	m := setup(t, f)
	step := pinClock(m)
	runAll(m, enter(m, "/win #dev"))
	calls := len(f.calls)
	_, cmd := m.Update(tickMsg(m.now()))
	_ = cmd
	if len(f.calls) != calls {
		t.Fatal("no heal before healEvery")
	}
	step(healEvery + time.Second)
	if cmd := m.healCmd(m.wins[m.cur]); cmd == nil {
		t.Fatal("a heal is due")
	} else {
		m.Update(cmd())
	}
	if len(f.calls) != calls+1 || strings.Count(body(m), "fetched 0 older") != 1 {
		t.Fatalf("one quiet fetch: %v", f.calls)
	}
	if cmd := m.applyChange(domain.Change{Kind: domain.Stale}); cmd == nil {
		t.Fatal("a reported gap heals the shown window at once")
	}
}
