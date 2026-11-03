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

func TestDelete(t *testing.T) {
	s := mustOpen(t, Memory, secret)
	mustAdd(t, s, m("a", "1", 1, "x"), m("a", "2", 2, "y"))
	if err := s.Delete("a", "1"); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Messages("a", 0)
	if len(got) != 1 || got[0].ID != "2" {
		t.Fatalf("%+v", got)
	}
	if err := s.Delete("a", "nope"); err != nil {
		t.Fatal("deleting an unknown message is fine")
	}
}

func TestRefreshBringsCopiesUpToDate(t *testing.T) {
	s := mustOpen(t, Memory, secret)
	mustAdd(t, s, m("c", "1", 0, "cached text"), m("c", "2", 1, "same"))
	edited := m("c", "1", 0, "edited elsewhere")
	edited.SenderID, edited.SenderNick = "u1", "nick"
	unknown := m("c", "9", 2, "not cached")
	same := m("c", "2", 1, "same")
	got, err := s.Refresh([]domain.Message{edited, unknown, same})
	if err != nil || len(got) != 1 || got[0].ID != "1" || got[0].Text != "edited elsewhere" || got[0].SenderNick != "nick" {
		t.Fatalf("refreshed %+v, %v", got, err)
	}
	if again, err := s.Refresh([]domain.Message{edited}); err != nil || len(again) != 0 {
		t.Fatalf("already up to date: %+v, %v", again, err)
	}
	all, err := s.Messages("c", 0)
	if err != nil || len(all) != 2 || all[0].Text != "edited elsewhere" || all[1].Text != "same" {
		t.Fatalf("stored: %+v, %v", all, err)
	}
}

func TestRefreshSkipsARowThatDoesNotOpen(t *testing.T) {
	s := mustOpen(t, Memory, secret)
	mustAdd(t, s, m("a", "1", 1, "x"), m("a", "2", 2, "y"))
	if _, err := s.db.Exec(`UPDATE messages SET data = (SELECT data FROM messages WHERE id = '2') WHERE id = '1'`); err != nil {
		t.Fatal(err)
	}
	one, two := m("a", "1", 1, "x2"), m("a", "2", 2, "y2")
	got, err := s.Refresh([]domain.Message{one, two})
	if err != nil || len(got) != 1 || got[0].ID != "2" {
		t.Fatalf("the bad row is skipped, the good one refreshed: %+v, %v", got, err)
	}
}

func TestPruneDropsWhatThePageProvesGone(t *testing.T) {
	s := mustOpen(t, Memory, secret)
	mustAdd(t, s, m("c", "5", 0, "old, outside the page"), m("c", "10", 1, "kept"), m("c", "11", 2, "deleted"),
		m("c", "12", 3, "kept"), m("c", "sent-1", 4, "non-numeric"), m("d", "11", 5, "another chat"))
	if err := s.PutMedia("c", "11", []byte("picture")); err != nil {
		t.Fatal(err)
	}
	gone, err := s.Prune("c", []domain.Message{m("c", "10", 1, "kept"), m("c", "12", 3, "kept")}, time.Time{})
	if err != nil || len(gone) != 1 || gone[0] != "11" {
		t.Fatalf("gone: %v %v", gone, err)
	}
	all, _ := s.Messages("", 0)
	if len(all) != 5 {
		t.Fatalf("only the one inside the range: %+v", all)
	}
	if _, found, _ := s.Media("c", "11"); found {
		t.Fatal("its attachment goes too")
	}
	if gone, err := s.Prune("c", nil, time.Now()); err != nil || len(gone) != 0 {
		t.Fatalf("an empty page proves nothing: %v %v", gone, err)
	}
}

func TestEditKeepsTheSender(t *testing.T) {
	s := mustOpen(t, Memory, secret)
	orig := m("c", "1", 0, "before")
	orig.SenderID, orig.SenderNick = "u1", "nick"
	mustAdd(t, s, orig)
	upd := domain.Message{ID: "1", ChatID: "c", Sender: "user", Text: "after", Media: &domain.Media{Kind: "photo"}}
	got, ok, err := s.Edit(upd)
	if err != nil || !ok || got.Text != "after" || got.Sender != "someone" || got.SenderNick != "nick" || got.Media == nil || !got.Time.Equal(orig.Time) {
		t.Fatalf("edited: %+v %v %v", got, ok, err)
	}
	if _, ok, err := s.Edit(domain.Message{ID: "9", ChatID: "c", Text: "x"}); ok || err != nil {
		t.Fatalf("not stored: %v %v", ok, err)
	}
}

func TestDeleteMessagesAcrossChats(t *testing.T) {
	s := mustOpen(t, Memory, secret)
	mustAdd(t, s, m("a", "7", 0, "x"), m("b", "8", 1, "y"), m("c", "7", 2, "a channel's own 7"))
	_ = s.PutMedia("b", "8", []byte("pic"))
	refs, err := s.DeleteMessages([]string{"a", "b"}, []string{"7", "8", "99"})
	if err != nil || len(refs) != 2 {
		t.Fatalf("refs %+v %v", refs, err)
	}
	all, _ := s.Messages("", 0)
	if len(all) != 1 || all[0].ChatID != "c" {
		t.Fatalf("left: %+v", all)
	}
	if _, found, _ := s.Media("b", "8"); found {
		t.Fatal("attachments go with their messages")
	}
	mustAdd(t, s, m("c", "9", 3, "z"))
	_ = s.PutMedia("c", "9", []byte("pic"))
	if err := s.Delete("c", "9"); err != nil {
		t.Fatal(err)
	}
	if _, found, _ := s.Media("c", "9"); found {
		t.Fatal("Delete removes the attachment too")
	}
}

func TestPruneAboveThePageOnlyWhatIsOld(t *testing.T) {
	s := mustOpen(t, Memory, secret)
	mustAdd(t, s, m("c", "10", 1, "on the page"), m("c", "11", 2, "deleted newest"), m("c", "12", 500, "arrived after the page"))
	before := time.Unix(1000+100, 0) // between the two
	gone, err := s.Prune("c", []domain.Message{m("c", "10", 1, "on the page")}, before)
	if err != nil || len(gone) != 1 || gone[0] != "11" {
		t.Fatalf("gone: %v %v", gone, err)
	}
	if fresh := mustAdd(t, s, m("c", "11", 2, "the stale page again")); len(fresh) != 0 {
		t.Fatal("a deletion this session is not undone by a stale page")
	}
}

func TestEditsKeepTheirOrderAndDropAReplacedAttachment(t *testing.T) {
	s := mustOpen(t, Memory, secret)
	orig := m("c", "1", 0, "v0")
	orig.Media = &domain.Media{Kind: "photo", ID: "p1"}
	mustAdd(t, s, orig)
	_ = s.PutMedia("c", "1", []byte("first picture"))
	v2 := domain.Message{ID: "1", ChatID: "c", Text: "v2", Media: &domain.Media{Kind: "photo", ID: "p1"}, Edited: time.Unix(2000, 0)}
	if got, _, _ := s.Edit(v2); got.Text != "v2" {
		t.Fatalf("applied: %+v", got)
	}
	if _, found, _ := s.Media("c", "1"); !found {
		t.Fatal("the same attachment keeps its vault copy")
	}
	v1 := v2
	v1.Text, v1.Edited = "v1", time.Unix(1500, 0)
	if got, _, _ := s.Edit(v1); got.Text != "v2" {
		t.Fatalf("an older edit arriving late changes nothing: %+v", got)
	}
	if got, _ := s.Refresh([]domain.Message{v1}); len(got) != 0 {
		t.Fatalf("nor does an older page copy: %+v", got)
	}
	v3 := v2
	v3.Text, v3.Edited, v3.Media = "v3", time.Unix(3000, 0), &domain.Media{Kind: "photo", ID: "p2"}
	_, _, _ = s.Edit(v3)
	if _, found, _ := s.Media("c", "1"); found {
		t.Fatal("a swapped attachment drops the old vault copy")
	}
}

func TestSecureDeleteIsOn(t *testing.T) {
	s := mustOpen(t, Memory, secret)
	var on int
	if err := s.db.QueryRow(`PRAGMA secure_delete`).Scan(&on); err != nil || on != 1 {
		t.Fatalf("secure_delete = %d, %v", on, err)
	}
}

func TestDeleteWithoutAChatRemembersEveryPair(t *testing.T) {
	s := mustOpen(t, Memory, secret)
	if _, err := s.DeleteMessages([]string{"a", "b"}, []string{"9"}); err != nil {
		t.Fatal(err)
	}
	if fresh := mustAdd(t, s, m("b", "9", 1, "a page from before the deletion")); len(fresh) != 0 {
		t.Fatal("not stored again, whichever shared chat it was in")
	}
}

func TestReactionsPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.sqlite")
	s := mustOpen(t, path, secret)
	x := m("c", "1", 0, "liked")
	x.Reactions = []domain.Reaction{{Emoji: "\U0001F44D", Count: 3, Mine: true}, {Emoji: domain.CustomReaction, Count: 3, Mine: true, MineCount: 2}}
	x.MineUnknown = true
	mustAdd(t, s, x, m("c", "2", 1, "nobody reacted"))
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := mustOpen(t, path, secret).Messages("c", 0)
	if err != nil || len(got) != 2 {
		t.Fatalf("%+v %v", got, err)
	}
	if rs := got[0].Reactions; len(rs) != 2 || rs[0] != x.Reactions[0] || rs[1] != x.Reactions[1] || got[0].MineUnknown {
		t.Fatalf("reactions come back, MineUnknown is never stored: %+v", got[0])
	}
	if got[1].Reactions != nil {
		t.Fatalf("none stay none: %+v", got[1])
	}
}

func TestRefreshTakesTheLaterCopysReactions(t *testing.T) {
	s := mustOpen(t, Memory, secret)
	thumbs := func(n int, mine bool) domain.Reaction {
		return domain.Reaction{Emoji: "\U0001F44D", Count: n, Mine: mine}
	}
	x := m("c", "1", 0, "same text")
	x.Reactions = []domain.Reaction{thumbs(1, true)}
	mustAdd(t, s, x)
	page := m("c", "1", 0, "same text")
	page.Reactions = []domain.Reaction{thumbs(2, false), {Emoji: "*", Count: 1}}
	page.MineUnknown = true // min: the account's own are not in it
	got, err := s.Refresh([]domain.Message{page})
	if err != nil || len(got) != 1 || len(got[0].Reactions) != 2 || got[0].Reactions[0] != thumbs(2, true) || got[0].MineUnknown {
		t.Fatalf("the text is unchanged, the reactions are the page's with the known Mine: %+v %v", got, err)
	}
	if again, _ := s.Refresh([]domain.Message{page}); len(again) != 0 {
		t.Fatalf("already up to date: %+v", again)
	}
	edit := domain.Message{ID: "1", ChatID: "c", Text: "edited live", Edited: time.Unix(2000, 0)}
	edit.Reactions = got[0].Reactions
	if _, _, err := s.Edit(edit); err != nil {
		t.Fatal(err)
	}
	older := m("c", "1", 0, "same text") // fetched before the edit, but its reactions are newer
	older.Edited = time.Unix(1500, 0)
	older.Reactions = []domain.Reaction{thumbs(5, true)}
	got, err = s.Refresh([]domain.Message{older})
	if err != nil || len(got) != 1 || got[0].Text != "edited live" || !got[0].Edited.Equal(edit.Edited) ||
		len(got[0].Reactions) != 1 || got[0].Reactions[0] != thumbs(5, true) {
		t.Fatalf("the later edit keeps its text, the later copy brings its reactions: %+v %v", got, err)
	}
	if again, _ := s.Refresh([]domain.Message{older}); len(again) != 0 {
		t.Fatalf("an older copy with the same reactions changes nothing: %+v", again)
	}
	all, _ := s.Messages("c", 0)
	if all[0].Text != "edited live" || all[0].Reactions[0] != thumbs(5, true) {
		t.Fatalf("stored: %+v", all[0])
	}
}

func TestReactChangesOnlyTheReactions(t *testing.T) {
	s := mustOpen(t, Memory, secret)
	x := m("c", "1", 0, "keep me")
	x.SenderID, x.Edited = "u1", time.Unix(1500, 0)
	x.Reactions = []domain.Reaction{{Emoji: "a", Count: 1, Mine: true, MineCount: 1}}
	mustAdd(t, s, x)
	rep := domain.Message{ID: "1", ChatID: "c", MineUnknown: true,
		Reactions: []domain.Reaction{{Emoji: "a", Count: 2}, {Emoji: "b", Count: 1}}}
	before, now, ok, err := s.React(rep)
	if err != nil || !ok || len(before.Reactions) != 1 || before.Reactions[0].Count != 1 {
		t.Fatalf("before: %+v %v %v", before, ok, err)
	}
	if now.Text != "keep me" || now.SenderID != "u1" || !now.Edited.Equal(x.Edited) || now.Sender != "someone" ||
		len(now.Reactions) != 2 || now.Reactions[0] != (domain.Reaction{Emoji: "a", Count: 2, Mine: true, MineCount: 1}) {
		t.Fatalf("only the reactions change, Mine and its count kept: %+v", now)
	}
	all, _ := s.Messages("c", 0)
	if len(all[0].Reactions) != 2 || all[0].Text != "keep me" {
		t.Fatalf("stored: %+v", all[0])
	}
	before, now, ok, _ = s.React(rep)
	if !ok || len(before.Reactions) != 2 || len(now.Reactions) != 2 || before.Reactions[1] != now.Reactions[1] {
		t.Fatalf("the same again: before and now alike: %+v %+v", before, now)
	}
	if _, _, ok, err := s.React(domain.Message{ID: "9", ChatID: "c"}); ok || err != nil {
		t.Fatalf("not stored: %v %v", ok, err)
	}
	if _, now, _, _ := s.React(domain.Message{ID: "1", ChatID: "c"}); len(now.Reactions) != 0 {
		t.Fatalf("all taken back: %+v", now)
	}
}

func TestALateEditBringsItsReactions(t *testing.T) {
	s := mustOpen(t, Memory, secret)
	mustAdd(t, s, m("c", "1", 0, "v0"))
	v2 := domain.Message{ID: "1", ChatID: "c", Text: "v2", Edited: time.Unix(2000, 0),
		Reactions: []domain.Reaction{{Emoji: "a", Count: 1}}}
	if got, _, _ := s.Edit(v2); got.Text != "v2" || len(got.Reactions) != 1 {
		t.Fatalf("an edit carries its reactions: %+v", got)
	}
	v1 := domain.Message{ID: "1", ChatID: "c", Text: "v1", Edited: time.Unix(1500, 0),
		Reactions: []domain.Reaction{{Emoji: "a", Count: 4}}}
	if got, _, _ := s.Edit(v1); got.Text != "v2" || len(got.Reactions) != 1 || got.Reactions[0].Count != 4 {
		t.Fatalf("an older edit reported later keeps the text, brings the reactions: %+v", got)
	}
	if all, _ := s.Messages("c", 0); all[0].Text != "v2" || all[0].Reactions[0].Count != 4 {
		t.Fatalf("stored: %+v", all[0])
	}
}
