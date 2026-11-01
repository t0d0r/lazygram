package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// One process per state directory: two lazygram processes on the same
// directory would share one Telegram authorization key from two MTProto
// sessions, which Telegram answers with AUTH_KEY_DUPLICATED (the login is
// invalidated and, with it, the encrypted cache), and would write the same
// cache and session file. The lock is advisory, held on <dir>/lock for the
// life of the process and released by the kernel when it dies, so a crash
// never leaves a stale lock behind.

// ErrLocked says another process holds the state directory.
var ErrLocked = errors.New("the state directory is in use by another lazygram")

// ErrNoFlock says the filesystem of the state directory cannot lock files
// (NFS without a lock daemon, some FUSE, CIFS and 9p mounts).
var ErrNoFlock = errors.New("the filesystem does not support file locks")

// Lock is a held state directory.
type Lock struct {
	f *os.File
	// Unlocked says the directory's filesystem cannot lock: the lock file
	// is held open but a second process would not be refused.
	Unlocked bool
}

// LockFile is the name of the lock file inside the state directory.
const LockFile = "lock"

// TryLock takes the state directory's lock without waiting. It creates the
// directory when missing. ErrLocked means another process holds it; on a
// filesystem without file locks it returns a Lock with Unlocked set, so the
// caller decides how to warn.
func TryLock(dir string) (*Lock, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, LockFile)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	switch err := flock(f); {
	case err == nil:
		return &Lock{f: f}, nil
	case errors.Is(err, ErrNoFlock):
		return &Lock{f: f, Unlocked: true}, nil
	case errors.Is(err, ErrLocked):
		_ = f.Close()
		return nil, ErrLocked
	default:
		_ = f.Close()
		return nil, fmt.Errorf("lock %s: %w", path, err)
	}
}

// Release gives the lock up; the file stays (empty) for the next process.
func (l *Lock) Release() error {
	if l == nil || l.f == nil {
		return nil
	}
	err := l.f.Close() // closing the description drops the lock
	l.f = nil
	return err
}
