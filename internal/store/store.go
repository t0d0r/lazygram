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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	s := &Store{db: db}
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
