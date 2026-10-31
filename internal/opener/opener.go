// Package opener hands a web link or a local file to the operating system's
// default application: open on macOS, xdg-open on Linux and the BSDs, the
// shell's start on Windows. Nothing is opened unless the user clicked or
// asked for it, and only http(s) links and existing absolute file paths are
// accepted.
package opener

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Open opens target with the default application and returns once the
// opener program has been started.
func Open(target string) error {
	if err := Check(target); err != nil {
		return err
	}
	name, args := command(runtime.GOOS, target)
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf("no opener program (%s) on this system", name)
	}
	cmd := exec.Command(name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// Check reports whether target is something Open accepts: an http or https
// URL with a host, or an absolute path to an existing regular file.
func Check(target string) error {
	if IsLink(target) {
		return nil
	}
	if strings.Contains(target, "://") || strings.HasPrefix(target, "-") {
		return errors.New("only http and https links can be opened")
	}
	if !filepath.IsAbs(target) {
		return errors.New("only absolute file paths can be opened")
	}
	st, err := os.Stat(target)
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() {
		return errors.New("not a regular file")
	}
	return nil
}

// IsLink reports whether s is an http or https URL with a host.
func IsLink(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// command is the opener program for an operating system.
func command(goos, target string) (string, []string) {
	switch goos {
	case "darwin":
		return "open", []string{target}
	case "windows":
		return "rundll32", []string{"url.dll,FileProtocolHandler", target}
	}
	return "xdg-open", []string{target}
}
