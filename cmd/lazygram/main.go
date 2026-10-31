// Command lazygram is a terminal client that shows chats from messaging
// services as one IRC-style chronological feed.
//
//	lazygram          start with the linked Telegram account
//	lazygram login    link this device to Telegram by scanning a QR code
//	lazygram demo     start with invented offline data, no account needed
//	lazygram update   install the latest release
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"lazygram/internal/config"
	"lazygram/internal/provider"
	"lazygram/internal/provider/demo"
	"lazygram/internal/provider/telegram"
	"lazygram/internal/store"
	"lazygram/internal/tui"
	"lazygram/internal/update"
)

// version is set at build time with -ldflags "-X main.version=..." (git
// describe output, or the release version in Homebrew builds).
var version = "dev"

// buildAppID and buildAppHash carry a Telegram application registration in
// the binary, like nchat's built-in one, so a personal build never asks for
// it. They are set by the git-ignored app_local.go that "make embed" writes.
// They are empty in a plain build.
var buildAppID, buildAppHash string

const notLinked = `lazygram is not linked to a Telegram account yet.

  lazygram login   link your account by scanning a QR code with the Telegram app
  lazygram demo    try lazygram with invented data, no account needed
`

type options struct {
	dataDir string
	smoke   bool
	ascii   bool
	check   bool // update: only report
	yes     bool // update: do not ask
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		var ec exitError
		if errors.As(err, &ec) {
			fmt.Fprint(os.Stderr, ec.msg)
			os.Exit(ec.code)
		}
		fmt.Fprintln(os.Stderr, "lazygram:", err)
		os.Exit(1)
	}
}

// exitError carries a full message and status for expected exits.
type exitError struct {
	msg  string
	code int
}

func (e exitError) Error() string { return e.msg }

func run(args []string) error {
	fs := flag.NewFlagSet("lazygram", flag.ContinueOnError)
	var o options
	fs.StringVar(&o.dataDir, "data", "", "state directory (default ~/.local/state/lazygram)")
	fs.BoolVar(&o.smoke, "smoke", false, "sync once, print one screen, and exit (no terminal needed)")
	fs.BoolVar(&o.ascii, "ascii", false, "draw the login QR code with # characters instead of block characters")
	fs.BoolVar(&o.check, "check", false, "update: only report whether a newer release exists")
	fs.BoolVar(&o.yes, "yes", false, "update: install without asking")
	fs.BoolVar(&o.yes, "y", false, "same as -yes")
	showVersion := fs.Bool("version", false, "print the version and exit")
	fs.Usage = func() {
		_, _ = fmt.Fprint(fs.Output(), "usage: lazygram [flags] [login|demo|update]\n\n"+
			"  (none)  start with the linked Telegram account\n"+
			"  login   link this device to Telegram by scanning a QR code\n"+
			"  demo    start with invented offline data, no account needed\n"+
			"  update  install the latest release (-check only reports, -yes skips the question)\n\nflags:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return exitError{"", 2}
	}
	if fs.NArg() > 1 { // flags may follow the subcommand: lazygram login -ascii
		rest := fs.Args()[1:]
		cmd := fs.Arg(0)
		if err := fs.Parse(rest); err != nil {
			return exitError{"", 2}
		}
		if fs.NArg() > 0 {
			fs.Usage()
			return exitError{fmt.Sprintf("lazygram: unexpected argument %q\n", fs.Arg(0)), 2}
		}
		if err := fs.Parse([]string{cmd}); err != nil {
			return exitError{"", 2}
		}
	}
	if *showVersion {
		fmt.Println("lazygram", version)
		return nil
	}
	if o.dataDir == "" {
		d, err := config.DefaultDataDir()
		if err != nil {
			return err
		}
		o.dataDir = d
	}
	switch cmd := fs.Arg(0); cmd {
	case "":
		return runTelegram(o)
	case "demo":
		secret := make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			return err
		}
		st, err := store.Open(store.Memory, secret) // the demo keeps nothing on disk
		if err != nil {
			return err
		}
		return runTUI(o, demo.New(), st, 5*time.Second)
	case "login":
		return runLogin(o)
	case "update":
		return runUpdate(o.check, o.yes, os.Stdin, os.Stdout)
	default:
		fs.Usage()
		return exitError{fmt.Sprintf("lazygram: unknown command %q\n", cmd), 2}
	}
}

// appSource says where the application registration came from.
type appSource int

const (
	appNone   appSource = iota
	appStored           // app.json in the state directory
	appEnv              // TG_API_ID / TG_API_HASH in the environment
	appBuilt            // embedded at build time
)

// telegramConfig resolves the application registration: the stored file
// wins, then the environment, then the build-time value.
func telegramConfig(dataDir string) (cfg config.Telegram, src appSource, err error) {
	dir := config.TelegramDir(dataDir)
	cfg = config.Telegram{SessionDir: dir}
	app, found, err := config.LoadApp(dir)
	if err != nil {
		return cfg, appNone, err
	}
	if found {
		cfg.App = app
		return cfg, appStored, nil
	}
	app, found, err = config.AppFromEnv(os.Getenv)
	if err != nil {
		return cfg, appNone, err
	}
	if found {
		cfg.App = app
		return cfg, appEnv, nil
	}
	if buildAppID != "" {
		app, err = config.ParseApp(buildAppID, buildAppHash)
		if err != nil {
			return cfg, appNone, fmt.Errorf("embedded application registration: %w", err)
		}
		cfg.App = app
		return cfg, appBuilt, nil
	}
	return cfg, appNone, nil
}

func runTelegram(o options) error {
	cfg, src, err := telegramConfig(o.dataDir)
	if err != nil {
		return err
	}
	if src == appNone || !telegram.HasSession(cfg) {
		return exitError{notLinked, 2}
	}
	client := telegram.NewClient(cfg)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	err = client.Connect(ctx)
	cancel()
	switch {
	case errors.Is(err, telegram.ErrNotLoggedIn):
		_ = client.Close()
		return exitError{"the stored Telegram session is no longer authorized.\n\n" +
			"  lazygram login   link your account again\n  lazygram demo    use invented data instead\n", 2}
	case errors.Is(err, context.DeadlineExceeded):
		_ = client.Close()
		return errors.New("could not reach Telegram within 45s; check the network and try again")
	case err != nil:
		_ = client.Close()
		return sessionHint(cfg, err)
	}
	prov, err := telegram.New(cfg, client)
	if err != nil {
		return err
	}
	st, err := openCache(cfg)
	if err != nil {
		_ = prov.Close()
		return err
	}
	return runTUI(o, prov, st, 10*time.Second)
}

// updateHint checks GitHub for a newer release, briefly and quietly: a
// release build gets a one-line hint when a newer version exists, a build
// from source or an unreachable network gets nothing.
func updateHint(version string, timeout time.Duration) string {
	current, release := update.Parse(version)
	if !release {
		return ""
	}
	c := update.New()
	c.HTTP.Timeout = timeout
	latest, err := c.Latest()
	if err != nil || current.Compare(latest) >= 0 {
		return ""
	}
	return fmt.Sprintf("a newer release is available: %s -> %s (run: lazygram update)", current, latest)
}

// sessionHint adds the way out of a session file that cannot be used.
func sessionHint(cfg config.Telegram, err error) error {
	return fmt.Errorf("%w\n  if this persists, remove %s and run lazygram login again",
		err, filepath.Join(cfg.SessionDir, "session.json"))
}

// openCache opens the encrypted message cache next to the session. Its key
// derives from the session's auth key, so a cache left by an earlier login
// cannot be read and is discarded. Plain-text caches from older versions are
// removed.
func openCache(cfg config.Telegram) (*store.Store, error) {
	secret, err := telegram.SessionSecret(context.Background(), cfg)
	if err != nil {
		return nil, sessionHint(cfg, err)
	}
	removeLegacyCache(cfg.SessionDir)
	path := filepath.Join(cfg.SessionDir, "cache.sqlite")
	st, err := store.Open(path, secret)
	if errors.Is(err, store.ErrWrongKey) {
		fmt.Fprintln(os.Stderr, "lazygram: the message cache belongs to an earlier login; starting a fresh one")
		for _, f := range []string{path, path + "-journal"} {
			_ = os.Remove(f)
		}
		st, err = store.Open(path, secret)
	}
	return st, err
}

// removeLegacyCache deletes the unencrypted cache files of earlier versions.
func removeLegacyCache(dir string) {
	for _, f := range []string{"messages.jsonl", "chats.json"} {
		if err := os.Remove(filepath.Join(dir, f)); err == nil {
			fmt.Fprintf(os.Stderr, "lazygram: removed unencrypted %s from an earlier version\n", f)
		}
	}
}

func runTUI(o options, prov provider.Provider, st *store.Store, poll time.Duration) error {
	if c, ok := prov.(io.Closer); ok {
		defer func() { _ = c.Close() }()
	}
	defer func() { _ = st.Close() }()
	opt := tui.Options{Store: st, Provider: prov, Poll: poll, Version: version}
	if o.smoke || !isTerminal(os.Stdout) || !isTerminal(os.Stdin) {
		fmt.Print(tui.Snapshot(opt, 100, 24))
		fmt.Println()
		return nil
	}
	final, err := tea.NewProgram(tui.New(opt), tea.WithAltScreen(), tea.WithMouseAllMotion()).Run()
	if err != nil {
		return err
	}
	width, summary := 80, ""
	if m, ok := final.(*tui.Model); ok {
		width, summary = m.Width(), m.Summary()
	}
	fmt.Print(tui.ExitBanner(version, width, summary))
	if hint := updateHint(version, 3*time.Second); hint != "" {
		fmt.Println(hint)
	}
	return nil
}

func isTerminal(f *os.File) bool { return term.IsTerminal(int(f.Fd())) }
