package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"lazygram/internal/update"
)

// runUpdate implements "lazygram update": install the latest GitHub release
// the way this binary was installed (brew, or replacing the file). A binary
// a package manager installed is never replaced; the update is left to the
// package manager. check only reports; yes skips the question. executable
// resolves the running binary (update.Executable outside tests).
func runUpdate(c *update.Client, version string, executable func() (string, error), check, yes bool, in io.Reader, out io.Writer) error {
	latest, err := c.Latest()
	if err != nil {
		return err
	}
	current, release := update.Parse(version)
	if !release {
		_, _ = fmt.Fprintf(out, "lazygram %s is a build from source; the latest release is %s\n", version, latest)
		if check {
			return nil
		}
		return fmt.Errorf("not replacing a build from source; update it with git pull and make install")
	}
	if current.Compare(latest) >= 0 {
		_, _ = fmt.Fprintf(out, "lazygram %s is up to date\n", version)
		return nil
	}
	_, _ = fmt.Fprintf(out, "update available: %s -> %s\n", current, latest)
	if check {
		return nil
	}
	exe, err := executable()
	if err != nil {
		return err
	}
	method := update.Detect(exe)
	if method == update.Package {
		return fmt.Errorf("not replacing %s: %w", exe, update.ErrPackaged)
	}
	q := fmt.Sprintf("Update lazygram %s -> %s (%s)?", current, latest, method)
	if !yes && !confirm(out, bufio.NewReader(in), q) {
		_, _ = fmt.Fprintln(out, "skipped")
		return nil
	}
	if err := c.Install(method, latest, exe); err != nil {
		return err
	}
	if method == update.Binary {
		_, _ = fmt.Fprintf(out, "updated lazygram %s -> %s (%s)\n", current, latest, exe)
	}
	return nil
}

func confirm(out io.Writer, in *bufio.Reader, question string) bool {
	_, _ = fmt.Fprintf(out, "%s [y/N] ", question)
	s, _ := in.ReadString('\n')
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "y" || s == "yes"
}
