package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// flockPlatform says whether this platform has the real lock.
func flockPlatform() bool {
	switch runtime.GOOS {
	case "darwin", "dragonfly", "freebsd", "linux", "netbsd", "openbsd":
		return true
	}
	return false
}

func TestTryLockRefusesASecondHolder(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	first, err := TryLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(filepath.Join(dir, LockFile)); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("lock file: %v %v", fi, err)
	}
	if di, err := os.Stat(dir); err != nil || di.Mode().Perm() != 0o700 {
		t.Fatalf("state dir mode: %v %v", di, err)
	}
	second, err := TryLock(dir)
	if flockPlatform() {
		if !errors.Is(err, ErrLocked) || second != nil || first.Unlocked {
			t.Fatalf("second holder: %v %v unlocked=%v", second, err, first.Unlocked)
		}
	} else {
		if err != nil || !second.Unlocked {
			t.Fatalf("no flock here: a second lock is granted, marked unlocked: %v", err)
		}
		_ = second.Release()
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	if err := first.Release(); err != nil {
		t.Fatal("a second release is harmless")
	}
	again, err := TryLock(dir)
	if err != nil {
		t.Fatalf("after release: %v", err)
	}
	_ = again.Release()
	var none *Lock
	if err := none.Release(); err != nil {
		t.Fatal("nil lock")
	}
}
