//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd

package config

import (
	"errors"
	"os"
	"syscall"
)

// flockFn takes an exclusive, non-blocking flock(2); tests replace it.
var flockFn = func(fd int) error { return syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB) }

// flock locks f. Each open() has its own description, so a second TryLock
// in the same process fails too. A filesystem that cannot lock answers
// ENOLCK or ENOTSUP (EOPNOTSUPP; the two differ on darwin).
func flock(f *os.File) error {
	err := flockFn(int(f.Fd()))
	switch {
	case err == nil:
		return nil
	case errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN):
		return ErrLocked
	case errors.Is(err, syscall.ENOLCK) || errors.Is(err, syscall.ENOTSUP) || errors.Is(err, syscall.EOPNOTSUPP):
		return ErrNoFlock
	}
	return err
}
