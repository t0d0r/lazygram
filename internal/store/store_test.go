package store

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"lazygram/internal/domain"
)

func m(chat, id string, sec int) domain.Message {
	return domain.Message{ID: id, ChatID: chat, Text: "t", Time: time.Unix(int64(1000+sec), 0).UTC()}
}

func TestPersistAndReload(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	n, err := s.Add([]domain.Message{m("a", "2", 5), m("b", "1", 1), m("a", "2", 5)})
	if err != nil || n != 2 {
		t.Fatalf("add: %d %v", n, err)
	}
	if n, _ := s.Add([]domain.Message{m("b", "1", 1)}); n != 0 {
		t.Fatalf("re-add should be a no-op, got %d", n)
	}
	if err := s.SetChats([]domain.Chat{{ID: "a", Alias: "alpha"}}); err != nil {
		t.Fatal(err)
	}

	r, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := r.Messages()
	if len(got) != 2 || got[0].ChatID != "b" || got[1].ChatID != "a" {
		t.Fatalf("reloaded order wrong: %+v", got)
	}
	if c := r.Chats(); len(c) != 1 || c[0].Alias != "alpha" {
		t.Fatalf("chats: %+v", c)
	}
}

func TestCorruptLineSkipped(t *testing.T) {
	dir := t.TempDir()
	s, _ := Open(dir)
	mustAdd(t, s, m("a", "1", 1))
	f, _ := os.OpenFile(filepath.Join(dir, messagesFile), os.O_APPEND|os.O_WRONLY, 0o600)
	if _, err := f.WriteString("{not json\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	r, err := Open(dir)
	if err != nil || len(r.Messages()) != 1 {
		t.Fatalf("reload: %v %d", err, len(r.Messages()))
	}
}

func TestMemoryOnly(t *testing.T) {
	s, err := Open("")
	if err != nil {
		t.Fatal(err)
	}
	mustAdd(t, s, m("a", "1", 1))
	if err := s.SetChats(nil); err != nil || len(s.Messages()) != 1 {
		t.Fatal("memory store")
	}
}

func TestPrivateFileMode(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	s, _ := Open(dir)
	mustAdd(t, s, m("a", "1", 1))
	st, err := os.Stat(filepath.Join(dir, messagesFile))
	if err != nil || st.Mode().Perm()&0o077 != 0 {
		t.Fatalf("history must be owner-only: %v %v", st, err)
	}
}

func mustAdd(t *testing.T, s *Store, msgs ...domain.Message) {
	t.Helper()
	if _, err := s.Add(msgs); err != nil {
		t.Fatal(err)
	}
}
