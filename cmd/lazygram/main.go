// Command lazygram is a terminal client that shows chats from messaging
// services as one IRC-style chronological feed.
//
//	lazygram          start with the linked Telegram account
//	lazygram login    link this device to Telegram by scanning a QR code
//	lazygram demo     start with invented offline data, no account needed
package main

import (
	"context"
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
)

var version = "0.2.0"

// buildAppID and buildAppHash embed a Telegram application registration at
// build time (make build TG_API_ID=... TG_API_HASH=...), so a personal build
// never asks for one. They are empty in a plain build.
var buildAppID, buildAppHash string

const notLinked = `lazygram is not linked to a Telegram account yet.

  lazygram login   link your account by scanning a QR code with the Telegram app
  lazygram demo    try lazygram with invented data, no account needed
`

type options struct {
	dataDir string
	smoke   bool
	ascii   bool
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
	showVersion := fs.Bool("version", false, "print the version and exit")
	fs.Usage = func() {
		_, _ = fmt.Fprint(fs.Output(), "usage: lazygram [flags] [login|demo]\n\n"+
			"  (none)  start with the linked Telegram account\n"+
			"  login   link this device to Telegram by scanning a QR code\n"+
			"  demo    start with invented offline data, no account needed\n\nflags:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return exitError{"", 2}
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
		return runTUI(o, demo.New(), 5*time.Second)
	case "login":
		return runLogin(o)
	default:
		fs.Usage()
		return exitError{fmt.Sprintf("lazygram: unknown command %q\n", cmd), 2}
	}
}

// telegramConfig loads the stored application registration, or the one
// embedded at build time. found is false when there is none.
func telegramConfig(dataDir string) (cfg config.Telegram, found bool, err error) {
	dir := config.TelegramDir(dataDir)
	app, found, err := config.LoadApp(dir)
	if err != nil {
		return cfg, found, err
	}
	if !found && buildAppID != "" {
		app, err = config.ParseApp(buildAppID, buildAppHash)
		if err != nil {
			return cfg, false, fmt.Errorf("embedded application registration: %w", err)
		}
		found = true
	}
	return config.Telegram{App: app, SessionDir: dir}, found, nil
}

func runTelegram(o options) error {
	cfg, found, err := telegramConfig(o.dataDir)
	if err != nil {
		return err
	}
	if !found || !telegram.HasSession(cfg) {
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
		return err
	}
	prov, err := telegram.New(cfg, client)
	if err != nil {
		return err
	}
	return runTUI(o, prov, 10*time.Second)
}

func runTUI(o options, prov provider.Provider, poll time.Duration) error {
	if c, ok := prov.(io.Closer); ok {
		defer func() { _ = c.Close() }()
	}
	st, err := store.Open(filepath.Join(o.dataDir, prov.Name()))
	if err != nil {
		return err
	}
	opt := tui.Options{Store: st, Provider: prov, Poll: poll}
	if o.smoke || !isTerminal(os.Stdout) || !isTerminal(os.Stdin) {
		fmt.Print(tui.Snapshot(opt, 100, 24))
		fmt.Println()
		return nil
	}
	_, err = tea.NewProgram(tui.New(opt), tea.WithAltScreen()).Run()
	return err
}

func isTerminal(f *os.File) bool { return term.IsTerminal(int(f.Fd())) }
