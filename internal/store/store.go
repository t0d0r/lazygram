// Package store is the local message cache: an SQLite file whose sensitive
// columns are encrypted in Go before they reach the database, in the spirit
// of nchat's message cache but readable only with the key. Message ids and
// timestamps stay in clear so ordering and cursors work; chat ids are keyed
// hashes so lookups work; everything else (text, senders, titles) is
// AES-256-GCM ciphertext bound to its row.
package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"time"

	_ "modernc.org/sqlite" // pure Go SQLite driver

	"lazygram/internal/domain"
)

const (
	// Memory opens a private in-memory cache that disappears on Close.
	Memory = ":memory:"

	hkdfInfo   = "lazygram message cache v1"
	saltSize   = 32
	checkPlain = "lazygram"
)

// ErrWrongKey means the file was written with a key derived from another
// secret (for example an earlier login). The caller may discard it.
var ErrWrongKey = errors.New("cache was written with a different key")

// Store is a concurrency-safe encrypted cache.
type Store struct {
	mu   sync.Mutex
	db   *sql.DB
	aead cipher.AEAD
	mac  []byte

	mediaDir string            // sealed attachments next to the file; empty for Memory
	memMedia map[string][]byte // attachments of a Memory store

	// deleted remembers this session's deletions (chat key + id), so a
	// history page fetched before a deletion cannot bring the message back.
	deleted map[string]bool
}

// Open opens or creates the cache at path (Memory for none on disk). secret
// is the long-lived secret the keys are derived from; a random salt kept in
// the file makes the derived keys unique per cache. The file is created
// owner-only.
func Open(path string, secret []byte) (*Store, error) {
	if len(secret) < 16 {
		return nil, errors.New("store: secret too short")
	}
	if path != Memory {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, err
		}
		// SQLite gives journal files the mode of the main file, so make it
		// owner-only before the library creates it.
		f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			return nil, err
		}
		_ = f.Close()
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	s := &Store{db: db, memMedia: map[string][]byte{}, deleted: map[string]bool{}}
	if path != Memory {
		s.mediaDir = filepath.Join(filepath.Dir(path), "media")
	}
	if err := s.init(secret); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) init(secret []byte) error {
	for _, q := range []string{
		`PRAGMA journal_mode = DELETE`,
		`PRAGMA synchronous = NORMAL`,
		// A deleted or replaced row is overwritten on disk, not left in a
		// free page for someone with the key to read back.
		`PRAGMA secure_delete = ON`,
		`CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value BLOB NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS chats (chat_key BLOB PRIMARY KEY, pos INTEGER NOT NULL, data BLOB NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS messages (
			chat_key BLOB NOT NULL, id TEXT NOT NULL, time INTEGER NOT NULL, data BLOB NOT NULL,
			PRIMARY KEY (chat_key, id))`,
		`CREATE INDEX IF NOT EXISTS messages_time ON messages (time)`,
		`CREATE INDEX IF NOT EXISTS messages_chat_time ON messages (chat_key, time)`,
	} {
		if _, err := s.db.Exec(q); err != nil {
			return fmt.Errorf("store: schema: %w", err)
		}
	}
	salt, err := s.meta("salt")
	if err != nil {
		return err
	}
	fresh := salt == nil
	if fresh {
		salt = make([]byte, saltSize)
		if _, err := rand.Read(salt); err != nil {
			return err
		}
	}
	keys, err := hkdf.Key(sha256.New, secret, salt, hkdfInfo, 64)
	if err != nil {
		return err
	}
	block, err := aes.NewCipher(keys[:32])
	if err != nil {
		return err
	}
	if s.aead, err = cipher.NewGCM(block); err != nil {
		return err
	}
	s.mac = keys[32:]
	if fresh {
		if err := s.setMeta("salt", salt); err != nil {
			return err
		}
		return s.setMeta("check", s.seal([]byte(checkPlain), []byte("check")))
	}
	check, err := s.meta("check")
	if err != nil {
		return err
	}
	if plain, err := s.open(check, []byte("check")); err != nil || string(plain) != checkPlain {
		return ErrWrongKey
	}
	return nil
}

func (s *Store) meta(key string) ([]byte, error) {
	var v []byte
	err := s.db.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return v, err
}

func (s *Store) setMeta(key string, v []byte) error {
	_, err := s.db.Exec(`INSERT OR REPLACE INTO meta (key, value) VALUES (?, ?)`, key, v)
	return err
}

// Close releases the database.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Close()
}

// seal encrypts plain with a fresh nonce; aad binds the blob to its row.
func (s *Store) seal(plain, aad []byte) []byte {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		panic("store: no randomness: " + err.Error())
	}
	return s.aead.Seal(nonce, nonce, plain, aad)
}

func (s *Store) open(blob, aad []byte) ([]byte, error) {
	n := s.aead.NonceSize()
	if len(blob) < n {
		return nil, errors.New("store: blob too short")
	}
	return s.aead.Open(nil, blob[:n], blob[n:], aad)
}

// chatKey is the keyed hash under which a chat's rows are filed.
func (s *Store) chatKey(chatID string) []byte {
	h := hmac.New(sha256.New, s.mac)
	h.Write([]byte(chatID))
	return h.Sum(nil)
}

func rowAAD(chatKey []byte, id string) []byte {
	return append(append([]byte("msg:"), chatKey...), id...)
}

// Add stores new messages and returns those not already present, in input
// order.
func (s *Store) Add(msgs []domain.Message) ([]domain.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.Prepare(`INSERT OR IGNORE INTO messages (chat_key, id, time, data) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = stmt.Close() }()
	var fresh []domain.Message
	for _, m := range msgs {
		ck := s.chatKey(m.ChatID)
		if s.deleted[string(ck)+"\x00"+m.ID] { // deleted this session: a stale page brings it back
			continue
		}
		plain, err := json.Marshal(m)
		if err != nil {
			return nil, err
		}
		res, err := stmt.Exec(ck, m.ID, m.Time.UnixNano(), s.seal(plain, rowAAD(ck, m.ID)))
		if err != nil {
			return nil, err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			fresh = append(fresh, m)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return fresh, nil
}

// storedLocked reads one stored message; ok is false when there is none
// or it does not open or decode (a bad row is left alone). s.mu is held.
func (s *Store) storedLocked(q interface {
	QueryRow(string, ...any) *sql.Row
}, chatID, msgID string) (m domain.Message, ok bool, err error) {
	ck := s.chatKey(chatID)
	var data []byte
	switch err := q.QueryRow(`SELECT data FROM messages WHERE chat_key = ? AND id = ?`, ck, msgID).Scan(&data); {
	case errors.Is(err, sql.ErrNoRows):
		return m, false, nil
	case err != nil:
		return m, false, err
	}
	plain, err := s.open(data, rowAAD(ck, msgID))
	if err != nil {
		return m, false, nil
	}
	if json.Unmarshal(plain, &m) != nil {
		return m, false, nil
	}
	return m, true, nil
}

// putLocked writes m over its stored row. s.mu is held.
func (s *Store) putLocked(q interface {
	Exec(string, ...any) (sql.Result, error)
}, m domain.Message) error {
	ck := s.chatKey(m.ChatID)
	plain, err := json.Marshal(m)
	if err != nil {
		return err
	}
	_, err = q.Exec(`INSERT OR REPLACE INTO messages (chat_key, id, time, data) VALUES (?, ?, ?, ?)`,
		ck, m.ID, m.Time.UnixNano(), s.seal(plain, rowAAD(ck, m.ID)))
	return err
}

// Refresh brings stored messages up to date with copies fetched again:
// each message in msgs whose stored copy differs (edited text, media or
// buttons, the sender's id, username or name, the reactions) replaces it,
// and the replacements are returned. A copy older than the stored edit
// brings its reactions only: they have no edit time, and the copy fetched
// later carries them as they are now. Messages not stored are left to
// Add; a stored row that does not open or decode is left alone, so one
// bad row never fails the fetch around it.
func (s *Store) Refresh(msgs []domain.Message) ([]domain.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var out []domain.Message
	for _, m := range msgs {
		old, ok, err := s.storedLocked(tx, m.ChatID, m.ID)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		m.Reactions, m.MineUnknown = domain.CurrentReactions(old, m), false
		if old.Edited.After(m.Edited) { // a live edit newer than this page's copy: only the reactions are newer
			if slices.Equal(old.Reactions, m.Reactions) {
				continue
			}
			reactions := m.Reactions
			m = old
			m.Reactions = reactions
		}
		a, _ := json.Marshal(old)
		b, _ := json.Marshal(m)
		if string(a) == string(b) {
			continue
		}
		if err := s.putLocked(tx, m); err != nil {
			return nil, err
		}
		if mediaChanged(old.Media, m.Media) {
			s.deleteMediaLocked(m.ChatID, m.ID)
		}
		out = append(out, m)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

// Prune deletes the stored messages of chatID that a fetched page proves
// gone: the page is a chat's newest history without gaps, so a stored
// message whose numeric id lies between the page's lowest and highest and
// that the page lacks was deleted at the service, and so was one above
// the page's highest that is older than before (the time the page was
// asked for: a message newer than that may simply have arrived after the
// page was taken). Their attachments go with them, and the deletions are
// remembered for the session. It returns the deleted ids. Non-numeric ids
// are never pruned, nor anything below the page: a page shorter than
// asked for may only have skipped service messages.
func (s *Store) Prune(chatID string, page []domain.Message, before time.Time) ([]string, error) {
	in := map[int64]bool{}
	lo, hi := int64(0), int64(0)
	for _, m := range page {
		if m.ChatID != chatID {
			continue
		}
		n, err := strconv.ParseInt(m.ID, 10, 64)
		if err != nil {
			continue
		}
		if len(in) == 0 || n < lo {
			lo = n
		}
		if len(in) == 0 || n > hi {
			hi = n
		}
		in[n] = true
	}
	if len(in) == 0 {
		return nil, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ck := s.chatKey(chatID)
	rows, err := s.db.Query(`SELECT id, time FROM messages WHERE chat_key = ?`, ck)
	if err != nil {
		return nil, err
	}
	var gone []string
	for rows.Next() {
		var id string
		var at int64
		if err := rows.Scan(&id, &at); err != nil {
			_ = rows.Close()
			return nil, err
		}
		n, err := strconv.ParseInt(id, 10, 64)
		if err != nil || in[n] {
			continue
		}
		inside := n > lo && n < hi
		above := n > hi && !before.IsZero() && time.Unix(0, at).Before(before)
		if inside || above {
			gone = append(gone, id)
		}
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, id := range gone {
		if _, err := s.db.Exec(`DELETE FROM messages WHERE chat_key = ? AND id = ?`, ck, id); err != nil {
			return nil, err
		}
		s.deleteMediaLocked(chatID, id)
		s.deleted[string(ck)+"\x00"+id] = true
	}
	return gone, nil
}

// Edit applies an edit the service reported to the stored message: its
// text, attachment, buttons, edit time and reactions become m's; who sent
// it, when and its ids stay. An edit older than the stored copy's
// (updates can arrive out of order) changes the reactions only, which the
// copy reported later carries as they are now. When the attachment was
// swapped or removed, the vault's copy of the old one is dropped. It
// returns the message as stored now, or ok false when it is not stored.
func (s *Store) Edit(m domain.Message) (domain.Message, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, ok, err := s.storedLocked(s.db, m.ChatID, m.ID)
	if err != nil || !ok {
		return domain.Message{}, false, err
	}
	reactions := domain.CurrentReactions(old, m)
	if old.Edited.After(m.Edited) { // an older edit arriving late
		if slices.Equal(old.Reactions, reactions) {
			return old, true, nil
		}
		old.Reactions = reactions
		if err := s.putLocked(s.db, old); err != nil {
			return domain.Message{}, false, err
		}
		return old, true, nil
	}
	swapped := mediaChanged(old.Media, m.Media)
	old.Text, old.Media, old.Buttons, old.Edited, old.Reactions = m.Text, m.Media, m.Buttons, m.Edited, reactions
	if err := s.putLocked(s.db, old); err != nil {
		return domain.Message{}, false, err
	}
	if swapped { // the vault's copy is of the attachment that was replaced
		s.deleteMediaLocked(m.ChatID, m.ID)
	}
	return old, true, nil
}

// React applies reactions the service reported (a Reacted change) to the
// stored message: m's Reactions become its own, with the Mine flags
// already known when m's are unknown; its text and everything else stay.
// It returns the stored copy before and as stored now (the same when
// nothing changed), or ok false when the message is not stored.
func (s *Store) React(m domain.Message) (before, now domain.Message, ok bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	before, ok, err = s.storedLocked(s.db, m.ChatID, m.ID)
	if err != nil || !ok {
		return domain.Message{}, domain.Message{}, false, err
	}
	now = before
	now.Reactions = domain.CurrentReactions(before, m)
	if slices.Equal(before.Reactions, now.Reactions) {
		return before, before, true, nil
	}
	if err := s.putLocked(s.db, now); err != nil {
		return domain.Message{}, domain.Message{}, false, err
	}
	return before, now, true, nil
}

// mediaChanged reports whether an edit took the attachment away or put
// another in its place: a different id, kind or name.
func mediaChanged(old, now *domain.Media) bool {
	switch {
	case old == nil:
		return false // nothing in the vault for it
	case now == nil:
		return true
	}
	return old.ID != now.ID || old.Kind != now.Kind || old.Name != now.Name
}

// MessageRef names one stored message.
type MessageRef struct{ ChatID, ID string }

// DeleteMessages deletes the messages with the given ids from any of the
// given chats, with their attachments, and returns which were stored. It
// serves deletions the service reports without naming the chat: ids that
// are unique across a set of chats.
func (s *Store) DeleteMessages(chatIDs, ids []string) ([]MessageRef, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	stmt, err := tx.Prepare(`DELETE FROM messages WHERE chat_key = ? AND id = ?`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = stmt.Close() }()
	var out []MessageRef
	for _, chatID := range chatIDs {
		ck := s.chatKey(chatID)
		for _, id := range ids {
			res, err := stmt.Exec(ck, id)
			if err != nil {
				return nil, err
			}
			if n, _ := res.RowsAffected(); n > 0 {
				out = append(out, MessageRef{chatID, id})
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	for _, r := range out {
		s.deleteMediaLocked(r.ChatID, r.ID)
	}
	// Remember every pair, stored or not: a page fetched before the
	// deletion may still be on its way. Ids are unique across the chats
	// of a shared id space, so the pairs in the other chats name nothing.
	for _, chatID := range chatIDs {
		ck := string(s.chatKey(chatID))
		for _, id := range ids {
			s.deleted[ck+"\x00"+id] = true
		}
	}
	return out, nil
}

// Replace stores a message, overwriting the stored one with the same chat
// and id (an edited bot message after a button press).
func (s *Store) Replace(m domain.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ck := s.chatKey(m.ChatID)
	plain, err := json.Marshal(m)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT OR REPLACE INTO messages (chat_key, id, time, data) VALUES (?, ?, ?, ?)`,
		ck, m.ID, m.Time.UnixNano(), s.seal(plain, rowAAD(ck, m.ID)))
	return err
}

// Delete removes a message from the cache, and its attachment from the
// vault.
func (s *Store) Delete(chatID, msgID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.Exec(`DELETE FROM messages WHERE chat_key = ? AND id = ?`, s.chatKey(chatID), msgID)
	if err == nil {
		s.deleteMediaLocked(chatID, msgID)
		s.deleted[string(s.chatKey(chatID))+"\x00"+msgID] = true
	}
	return err
}

// deleteMediaLocked removes a message's attachment from the vault, if
// there is one. s.mu is held.
func (s *Store) deleteMediaLocked(chatID, msgID string) {
	key := s.mediaKey(chatID, msgID)
	if s.mediaDir == "" {
		delete(s.memMedia, key)
		return
	}
	_ = os.Remove(filepath.Join(s.mediaDir, key+".enc"))
}

// Messages returns the newest limit messages of a chat (every chat when
// chatID is empty), oldest first. limit <= 0 means all.
func (s *Store) Messages(chatID string, limit int) ([]domain.Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 {
		limit = -1
	}
	var rows *sql.Rows
	var err error
	if chatID == "" {
		rows, err = s.db.Query(`SELECT chat_key, id, data FROM messages ORDER BY time DESC, id DESC LIMIT ?`, limit)
	} else {
		rows, err = s.db.Query(`SELECT chat_key, id, data FROM messages WHERE chat_key = ? ORDER BY time DESC, id DESC LIMIT ?`, s.chatKey(chatID), limit)
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []domain.Message
	for rows.Next() {
		var ck, data []byte
		var id string
		if err := rows.Scan(&ck, &id, &data); err != nil {
			return nil, err
		}
		plain, err := s.open(data, rowAAD(ck, id))
		if err != nil {
			return nil, fmt.Errorf("store: message %s: %w", id, err)
		}
		var m domain.Message
		if err := json.Unmarshal(plain, &m); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return domain.Merge(out), nil
}

// Count returns how many messages a chat has (every chat when empty).
func (s *Store) Count(chatID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int
	var err error
	if chatID == "" {
		err = s.db.QueryRow(`SELECT COUNT(*) FROM messages`).Scan(&n)
	} else {
		err = s.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE chat_key = ?`, s.chatKey(chatID)).Scan(&n)
	}
	return n, err
}

// Latest returns the newest message time, or the zero time when empty.
func (s *Store) Latest() (time.Time, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var ns sql.NullInt64
	if err := s.db.QueryRow(`SELECT MAX(time) FROM messages`).Scan(&ns); err != nil {
		return time.Time{}, err
	}
	if !ns.Valid {
		return time.Time{}, nil
	}
	return time.Unix(0, ns.Int64).UTC(), nil
}

// SetState stores a small JSON document (for example the window layout)
// under key, encrypted like everything else.
func (s *Store) SetState(key string, v any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	plain, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return s.setMeta("state:"+key, s.seal(plain, []byte("state:"+key)))
}

// State loads a document stored with SetState. found is false when there
// is none.
func (s *Store) State(key string, v any) (found bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	blob, err := s.meta("state:" + key)
	if err != nil || blob == nil {
		return false, err
	}
	plain, err := s.open(blob, []byte("state:"+key))
	if err != nil {
		return true, fmt.Errorf("store: state %s: %w", key, err)
	}
	return true, json.Unmarshal(plain, v)
}

// mediaKey names an attachment's sealed file: a keyed hash, so the file
// names reveal nothing.
func (s *Store) mediaKey(chatID, msgID string) string {
	h := hmac.New(sha256.New, s.mac)
	h.Write([]byte("media:" + chatID + "\x00" + msgID))
	return hex.EncodeToString(h.Sum(nil))[:32]
}

// PutMedia seals an attachment into the media vault (0700 directory, 0600
// files, names that are keyed hashes).
func (s *Store) PutMedia(chatID, msgID string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := s.mediaKey(chatID, msgID)
	blob := s.seal(data, []byte("media:"+key))
	if s.mediaDir == "" {
		s.memMedia[key] = blob
		return nil
	}
	if err := os.MkdirAll(s.mediaDir, 0o700); err != nil {
		return err
	}
	path := filepath.Join(s.mediaDir, key+".enc")
	if err := os.WriteFile(path+".part", blob, 0o600); err != nil {
		return err
	}
	return os.Rename(path+".part", path)
}

// Media opens an attachment from the vault. found is false when it was
// never fetched.
func (s *Store) Media(chatID, msgID string) (data []byte, found bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := s.mediaKey(chatID, msgID)
	var blob []byte
	if s.mediaDir == "" {
		blob, found = s.memMedia[key]
		if !found {
			return nil, false, nil
		}
	} else {
		blob, err = os.ReadFile(filepath.Join(s.mediaDir, key+".enc"))
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, err
		}
	}
	data, err = s.open(blob, []byte("media:"+key))
	if err != nil {
		return nil, true, fmt.Errorf("store: attachment: %w", err)
	}
	return data, true, nil
}

// Chats returns the stored chat list in its stored order.
func (s *Store) Chats() ([]domain.Chat, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.db.Query(`SELECT chat_key, data FROM chats ORDER BY pos`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []domain.Chat
	for rows.Next() {
		var ck, data []byte
		if err := rows.Scan(&ck, &data); err != nil {
			return nil, err
		}
		plain, err := s.open(data, append([]byte("chat:"), ck...))
		if err != nil {
			return nil, fmt.Errorf("store: chat: %w", err)
		}
		var c domain.Chat
		if err := json.Unmarshal(plain, &c); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SetChats replaces the chat list.
func (s *Store) SetChats(chats []domain.Chat) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`DELETE FROM chats`); err != nil {
		return err
	}
	for i, c := range chats {
		ck := s.chatKey(c.ID)
		plain, err := json.Marshal(c)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO chats (chat_key, pos, data) VALUES (?, ?, ?)`, ck, i, s.seal(plain, append([]byte("chat:"), ck...))); err != nil {
			return err
		}
	}
	return tx.Commit()
}
