package domain

import (
	"testing"
	"time"
)

var typingT0 = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

func TestTypingStateApplyExpireAndClear(t *testing.T) {
	s := NewTypingState(0)
	if s.TTL != DefaultTypingTTL || !s.Empty() {
		t.Fatalf("defaults: ttl=%v empty=%v", s.TTL, s.Empty())
	}
	if !s.Apply(Typing{ChatID: "c1", Who: "ada", Kind: TypingText, At: typingT0}) {
		t.Fatal("a new member is a change")
	}
	if s.Apply(Typing{ChatID: "c1", Who: "ada", Kind: TypingText, At: typingT0.Add(2 * time.Second)}) {
		t.Fatal("a renewal of the same kind is not a change")
	}
	if !s.Apply(Typing{ChatID: "c1", Who: "ada", Kind: TypingRecord, At: typingT0.Add(3 * time.Second)}) {
		t.Fatal("a changed kind is a change")
	}
	if got := s.Typists("c1"); len(got) != 1 || got[0] != (Typist{Who: "ada", Kind: TypingRecord, Since: typingT0}) {
		t.Fatalf("typists keep the first report's time: %v", got)
	}
	// The renewal moved the expiry: typingT0+3s+6s.
	if next, ok := s.Next(); !ok || !next.Equal(typingT0.Add(9*time.Second)) {
		t.Fatalf("next: %v %v", next, ok)
	}
	if s.Expire(typingT0.Add(8 * time.Second)) {
		t.Fatal("nothing expires before the renewed deadline")
	}
	if !s.Expire(typingT0.Add(9*time.Second)) || !s.Empty() {
		t.Fatalf("expired at the deadline: empty=%v", s.Empty())
	}
	if _, ok := s.Next(); ok {
		t.Fatal("nothing pending")
	}

	s.Apply(Typing{ChatID: "c1", Who: "ada", Kind: TypingText, At: typingT0})
	if !s.Apply(Typing{ChatID: "c1", Who: "ada", Kind: TypingStop, At: typingT0.Add(time.Second)}) || !s.Empty() {
		t.Fatal("a stop report ends it")
	}
	if s.Apply(Typing{ChatID: "c1", Who: "ada", Kind: TypingStop, At: typingT0}) {
		t.Fatal("a stop for nobody is not a change")
	}

	s.Apply(Typing{ChatID: "c1", Who: "ada", Kind: TypingText, At: typingT0})
	if !s.Clear("c1", "ada") || s.Clear("c1", "ada") || !s.Empty() {
		t.Fatal("Clear ends the report once")
	}
}

func TestTypingStateSeveralMembersAndChats(t *testing.T) {
	s := NewTypingState(6 * time.Second)
	s.Apply(Typing{ChatID: "c1", Who: "bob", Kind: TypingText, At: typingT0.Add(time.Second)})
	s.Apply(Typing{ChatID: "c1", Who: "ada", Kind: TypingUpload, At: typingT0})
	s.Apply(Typing{ChatID: "c1", Who: "cy", Kind: TypingText, At: typingT0.Add(time.Second)})
	s.Apply(Typing{ChatID: "c2", Who: "", Kind: TypingText, At: typingT0.Add(2 * time.Second)})
	got := s.Typists("c1")
	want := []Typist{{"ada", TypingUpload, typingT0}, {"bob", TypingText, typingT0.Add(time.Second)}, {"cy", TypingText, typingT0.Add(time.Second)}}
	if len(got) != len(want) {
		t.Fatalf("typists: %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order: %v, want %v", got, want)
		}
	}
	if got := s.Typists("c2"); len(got) != 1 || got[0].Who != Unknown {
		t.Fatalf("an unnamed member is %q: %v", Unknown, got)
	}
	if chats := s.Chats(); len(chats) != 2 || chats[0] != "c1" || chats[1] != "c2" {
		t.Fatalf("chats: %v", chats)
	}
	if next, ok := s.Next(); !ok || !next.Equal(typingT0.Add(6*time.Second)) {
		t.Fatalf("next is ada's: %v", next)
	}
	// Members expire one by one; the chat goes when its last one does.
	if !s.Expire(typingT0.Add(6*time.Second)) || len(s.Typists("c1")) != 2 {
		t.Fatalf("ada expired: %v", s.Typists("c1"))
	}
	s.Expire(typingT0.Add(7 * time.Second))
	if len(s.Typists("c1")) != 0 || len(s.Chats()) != 1 || s.Chats()[0] != "c2" {
		t.Fatalf("c1 gone: %v %v", s.Typists("c1"), s.Chats())
	}
	if s.Typists("nowhere") != nil {
		t.Fatal("an unknown chat has no typists")
	}
}

func TestTypingKindVerbs(t *testing.T) {
	for k, want := range map[TypingKind]string{TypingText: "typing", TypingRecord: "recording", TypingUpload: "uploading", TypingChoose: "choosing", TypingStop: "stopped", TypingKind(99): "typing"} {
		if got := k.String(); got != want {
			t.Errorf("%d: %q, want %q", k, got, want)
		}
	}
}
