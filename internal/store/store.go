// Package store persists chats and message history locally as JSON files.
// An empty directory gives a memory-only store.
package store

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"lazygram/internal/domain"
)

const (
	messagesFile = "messages.jsonl"
	chatsFile    = "chats.json"
)

// Store is a concurrency-safe history cache.
type Store struct {
	dir string

	mu    sync.RWMutex
	msgs  []domain.Message // chronological, de-duplicated
	chats []domain.Chat
}

// Open loads the store from dir, creating it (mode 0700) when missing. An
// empty dir returns a memory-only store.
func Open(dir string) (*Store, error) {
	s := &Store{dir: dir}
	if dir == "" {
		return s, nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create store dir: %w", err)
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	if b, err := os.ReadFile(filepath.Join(s.dir, chatsFile)); err == nil {
		if err := json.Unmarshal(b, &s.chats); err != nil {
			return fmt.Errorf("read %s: %w", chatsFile, err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	f, err := os.Open(filepath.Join(s.dir, messagesFile))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	var loaded []domain.Message
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		var m domain.Message
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			continue // skip a torn or corrupt line instead of losing the cache
		}
		loaded = append(loaded, m)
	}
	if err := sc.Err(); err != nil {
		return err
	}
	s.msgs = domain.Merge(loaded)
	return nil
}

// Add stores new messages and returns how many were not already present.
func (s *Store) Add(msgs []domain.Message) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	known := make(map[string]struct{}, len(s.msgs))
	for _, m := range s.msgs {
		known[m.Key()] = struct{}{}
	}
	var fresh []domain.Message
	for _, m := range msgs {
		if _, ok := known[m.Key()]; ok {
			continue
		}
		known[m.Key()] = struct{}{}
		fresh = append(fresh, m)
	}
	if len(fresh) == 0 {
		return 0, nil
	}
	if s.dir != "" {
		f, err := os.OpenFile(filepath.Join(s.dir, messagesFile), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			return 0, err
		}
		enc := json.NewEncoder(f)
		for _, m := range fresh {
			if err := enc.Encode(m); err != nil {
				_ = f.Close()
				return 0, err
			}
		}
		if err := f.Close(); err != nil {
			return 0, err
		}
	}
	s.msgs = domain.Merge(s.msgs, fresh)
	return len(fresh), nil
}

// Messages returns a copy of all stored messages in chronological order.
func (s *Store) Messages() []domain.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]domain.Message(nil), s.msgs...)
}

// Chats returns the stored chat list.
func (s *Store) Chats() []domain.Chat {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]domain.Chat(nil), s.chats...)
}

// SetChats replaces the chat list.
func (s *Store) SetChats(chats []domain.Chat) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.chats = append([]domain.Chat(nil), chats...)
	if s.dir == "" {
		return nil
	}
	b, err := json.MarshalIndent(s.chats, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(s.dir, chatsFile+".tmp")
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(s.dir, chatsFile))
}
