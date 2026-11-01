//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package config

import (
	"errors"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// withFlock replaces the flock call for one test.
func withFlock(t *testing.T, fn func(int) error) {
	t.Helper()
	old := flockFn
	flockFn = fn
	t.Cleanup(func() { flockFn = old })
}

func TestTryLockDegradesWithoutFlock(t *testing.T) {
	for _, errno := range []syscall.Errno{syscall.ENOLCK, syscall.ENOTSUP, syscall.EOPNOTSUPP} {
		withFlock(t, func(int) error { return errno })
		l, err := TryLock(filepath.Join(t.TempDir(), "state"))
		if err != nil || l == nil || !l.Unlocked {
			t.Fatalf("%v: a filesystem without locks is used, unlocked: %v %+v", errno, err, l)
		}
		_ = l.Release()
	}
}

func TestTryLockNamesTheFileOnOtherErrors(t *testing.T) {
	withFlock(t, func(int) error { return syscall.EBADF })
	dir := filepath.Join(t.TempDir(), "state")
	l, err := TryLock(dir)
	if l != nil || err == nil || errors.Is(err, ErrLocked) || !strings.Contains(err.Error(), filepath.Join(dir, LockFile)) {
		t.Fatalf("other errors stop with the path: %v %+v", err, l)
	}
}
