//go:build !(darwin || dragonfly || freebsd || linux || netbsd || openbsd)

package config

import "os"

// flock reports ErrNoFlock where lazygram has no flock(2) (Windows,
// Solaris, AIX, plan9, ...): the lock file is created but a second process
// is not refused. Releases ship for Linux and macOS, where the real lock
// applies.
func flock(*os.File) error { return ErrNoFlock }
