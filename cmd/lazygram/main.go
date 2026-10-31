// Command lazygram is a terminal client that shows chats from messaging
// services as one IRC-style chronological feed.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"lazygram/internal/config"
	"lazygram/internal/provider"
	"lazygram/internal/provider/demo"
	"lazygram/internal/provider/telegram"
	"lazygram/internal/store"
	"lazygram/internal/tui"
)

var version = "0.1.0"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "lazygram:", err)
		os.Exit(1)
	}
}

func run() error {
	smoke := flag.Bool("smoke", false, "sync once, print one screen, and exit (no terminal needed)")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println("lazygram", version)
		return nil
	}

	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	prov, err := newProvider(cfg)
	if err != nil {
		return err
	}
	if c, ok := prov.(io.Closer); ok {
		defer func() { _ = c.Close() }()
	}
	st, err := store.Open(filepath.Join(cfg.DataDir, prov.Name()))
	if err != nil {
		return err
	}
	opt := tui.Options{Store: st, Provider: prov, Poll: 5 * time.Second}

	if *smoke || !isTerminal(os.Stdout) || !isTerminal(os.Stdin) {
		fmt.Print(tui.Snapshot(opt, 100, 24))
		fmt.Println()
		return nil
	}
	_, err = tea.NewProgram(tui.New(opt), tea.WithAltScreen()).Run()
	return err
}

func newProvider(cfg config.Config) (provider.Provider, error) {
	switch cfg.Provider {
	case config.ProviderTelegram:
		return telegram.New(cfg.Telegram, telegram.NewClient(cfg.Telegram))
	default:
		return demo.New(), nil
	}
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
