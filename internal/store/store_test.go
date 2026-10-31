package store

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"lazygram/internal/domain"
)

var secret = bytes.Repeat([]byte{7}, 32)

func m(chat, id string, sec int, text string) domain.Message {
	return domain.Message{ID: id, ChatID: chat, Sender: "someone", Text: text, Time: time.Unix(int64(1000+sec), 0).UTC()}
}

func mustOpen(t *testing.T, path string, secret []byte) *Store {
	t.Helper()
	s, err := Open(path, secret)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func mustAdd(t *testing.T, s *Store, msgs ...domain.Message) []domain.Message {
	t.Helper()
	fresh, err := s.Add(msgs)
	if err != nil {
		t.Fatal(err)
	}
	return fresh
}

func TestPersistAndReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.sqlite")
	s := mustOpen(t, path, secret)
	fresh := mustAdd(t, s, m("a", "2", 5, "x"), m("b", "1", 1, "y"), m("a", "2", 5, "dup"))
	if len(fresh) != 2 || fresh[0].ChatID != "a" || fresh[1].ChatID != "b" {
		t.Fatalf("add: %v", fresh)
	}
	if fresh := mustAdd(t, s, m("b", "1", 1, "y")); len(fresh) != 0 {
		t.Fatalf("re-add should be a no-op, got %v", fresh)
	}
	if err := s.SetChats([]domain.Chat{{ID: "b", Alias: "beta", Kind: domain.ChatUser}, {ID: "a", Alias: "alpha", Title: "Alpha"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	r := mustOpen(t, path, secret)
	got, err := r.Messages("", 0)
	if err != nil || len(got) != 2 || got[0].ChatID != "b" || got[1].Text != "x" {
		t.Fatalf("reloaded: %+v %v", got, err)
	}
	c, err := r.Chats()
	if err != nil || len(c) != 2 || c[0].Alias != "beta" || c[1].Title != "Alpha" || c[0].Kind != domain.ChatUser {
		t.Fatalf("chats: %+v %v", c, err)
	}
	if n, _ := r.Count(""); n != 2 {
		t.Fatal("count")
	}
	if n, _ := r.Count("a"); n != 1 {
		t.Fatal("count a")
	}
	if latest, _ := r.Latest(); !latest.Equal(time.Unix(1005, 0)) {
		t.Fatalf("latest %v", latest)
	}
}

func TestPerChatLimitNewestFirstThenOrdered(t *testing.T) {
	s := mustOpen(t, Memory, secret)
	for i := 1; i <= 30; i++ {
		id := strconv.Itoa(i)
		mustAdd(t, s, m("c", id, i, "t"), m("other", id, i, "o"))
	}
	got, err := s.Messages("c", 10)
	if err != nil || len(got) != 10 {
		t.Fatalf("%d %v", len(got), err)
	}
	for i := 1; i < len(got); i++ {
		if got[i].ChatID != "c" || got[i].Time.Before(got[i-1].Time) {
			t.Fatalf("order: %+v", got)
		}
	}
	if !got[9].Time.Equal(time.Unix(1030, 0)) {
		t.Fatalf("newest should be included: %v", got[9].Time)
	}
	all, _ := s.Messages("", 0)
	if len(all) != 60 {
		t.Fatalf("all: %d", len(all))
	}
}

func TestNothingSensitiveInClearOnDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.sqlite")
	s := mustOpen(t, path, secret)
	mustAdd(t, s, m("telegram:4242", "1", 1, "the quick brown fox"))
	if err := s.SetChats([]domain.Chat{{ID: "telegram:4242", Alias: "fox-chat", Title: "Fox Chat Title"}}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"quick brown", "someone", "4242", "fox-chat", "Fox Chat", "telegram"} {
		if bytes.Contains(raw, []byte(leak)) {
			t.Errorf("%q is readable in the file", leak)
		}
	}
	st, _ := os.Stat(path)
	if st.Mode().Perm()&0o077 != 0 {
		t.Fatalf("cache must be owner-only, got %v", st.Mode())
	}
}

func TestWrongSecretIsRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.sqlite")
	s := mustOpen(t, path, secret)
	mustAdd(t, s, m("a", "1", 1, "x"))
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path, bytes.Repeat([]byte{9}, 32)); err != ErrWrongKey {
		t.Fatalf("got %v", err)
	}
	if _, err := Open(Memory, []byte("short")); err == nil {
		t.Fatal("short secret must be rejected")
	}
}

func TestTamperedRowIsRejected(t *testing.T) {
	s := mustOpen(t, Memory, secret)
	mustAdd(t, s, m("a", "1", 1, "x"), m("a", "2", 2, "y"))
	// Move row 2's ciphertext onto row 1: the AAD must catch it.
	if _, err := s.db.Exec(`UPDATE messages SET data = (SELECT data FROM messages WHERE id = '2') WHERE id = '1'`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Messages("a", 0); err == nil {
		t.Fatal("swapped ciphertext must not decrypt")
	}
}

func TestMemoryStore(t *testing.T) {
	s := mustOpen(t, Memory, secret)
	mustAdd(t, s, m("a", "1", 1, "x"))
	got, err := s.Messages("", 0)
	if err != nil || len(got) != 1 {
		t.Fatalf("%v %v", got, err)
	}
	if err := s.SetChats(nil); err != nil {
		t.Fatal(err)
	}
}

func TestStateRoundTripIsSealed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.sqlite")
	s := mustOpen(t, path, secret)
	type layout struct {
		Windows []string
		Current int
	}
	var got layout
	if found, err := s.State("windows", &got); found || err != nil {
		t.Fatalf("absent: %v %v", found, err)
	}
	want := layout{Windows: []string{"telegram:4242", "telegram:7"}, Current: 2}
	if err := s.SetState("windows", want); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	r := mustOpen(t, path, secret)
	found, err := r.State("windows", &got)
	if err != nil || !found || len(got.Windows) != 2 || got.Windows[0] != "telegram:4242" || got.Current != 2 {
		t.Fatalf("%+v %v %v", got, found, err)
	}
	raw, _ := os.ReadFile(path)
	if bytes.Contains(raw, []byte("4242")) || bytes.Contains(raw, []byte("telegram")) {
		t.Fatal("state must not be readable in the file")
	}
}

func TestReplaceKeepsButtonsRoundTrip(t *testing.T) {
	s := mustOpen(t, Memory, secret)
	orig := m("bot", "1", 1, "pick one")
	orig.Buttons = []domain.Button{{Label: "OK", Kind: domain.ButtonCallback, Data: []byte{1, 2}}, {Label: "Docs", Kind: domain.ButtonURL, URL: "https://example.invalid"}}
	mustAdd(t, s, orig)
	got, _ := s.Messages("bot", 0)
	if len(got) != 1 || len(got[0].Buttons) != 2 || string(got[0].Buttons[0].Data) != "\x01\x02" || got[0].Buttons[1].URL != "https://example.invalid" {
		t.Fatalf("%+v", got)
	}
	edited := m("bot", "1", 1, "confirmed")
	if err := s.Replace(edited); err != nil {
		t.Fatal(err)
	}
	got, _ = s.Messages("bot", 0)
	if len(got) != 1 || got[0].Text != "confirmed" || len(got[0].Buttons) != 0 {
		t.Fatalf("after replace: %+v", got)
	}
	if n, _ := s.Count("bot"); n != 1 {
		t.Fatal("replace must not add a row")
	}
}

func TestMediaVaultIsSealed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.sqlite")
	s := mustOpen(t, path, secret)
	if _, found, err := s.Media("telegram:1", "5"); found || err != nil {
		t.Fatalf("absent: %v %v", found, err)
	}
	photo := []byte("JFIF-THE-PHOTO-BYTES-telegram:1")
	if err := s.PutMedia("telegram:1", "5", photo); err != nil {
		t.Fatal(err)
	}
	got, found, err := s.Media("telegram:1", "5")
	if err != nil || !found || string(got) != string(photo) {
		t.Fatalf("%q %v %v", got, found, err)
	}
	entries, _ := os.ReadDir(filepath.Join(filepath.Dir(path), "media"))
	if len(entries) != 1 || !strings.HasSuffix(entries[0].Name(), ".enc") || strings.Contains(entries[0].Name(), "telegram") {
		t.Fatalf("vault entries: %v", entries)
	}
	raw, _ := os.ReadFile(filepath.Join(filepath.Dir(path), "media", entries[0].Name()))
	if bytes.Contains(raw, []byte("JFIF")) || bytes.Contains(raw, []byte("telegram:1")) {
		t.Fatal("attachment readable on disk")
	}
	info, _ := os.Stat(filepath.Join(filepath.Dir(path), "media", entries[0].Name()))
	dinfo, _ := os.Stat(filepath.Join(filepath.Dir(path), "media"))
	if info.Mode().Perm()&0o077 != 0 || dinfo.Mode().Perm()&0o077 != 0 {
		t.Fatal("vault must be owner-only")
	}
	mem := mustOpen(t, Memory, secret)
	if err := mem.PutMedia("a", "1", []byte("x")); err != nil {
		t.Fatal(err)
	}
	if got, found, _ := mem.Media("a", "1"); !found || string(got) != "x" {
		t.Fatal("memory vault")
	}
}
