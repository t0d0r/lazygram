package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"golang.org/x/term"

	"lazygram/internal/config"
	"lazygram/internal/provider/telegram"
	"lazygram/internal/qrcode"
)

const appHelp = `Telegram requires an application id and hash to connect. lazygram does not
ship one: get your own once at https://my.telegram.org (API development
tools). They are stored only in the local state directory and never shown
again.

`

// runLogin implements "lazygram login": a one-time, user-run link of this
// device to a Telegram account by QR code, as the official apps and nchat
// do. Nothing is read from flags, environment or files; nothing typed is
// echoed back or logged.
func runLogin(o options) error {
	if !isTerminal(os.Stdin) {
		return errors.New("login needs an interactive terminal")
	}
	p := &ttyPrompter{in: bufio.NewReader(os.Stdin), out: os.Stderr}
	cfg, found, err := telegramConfig(o.dataDir)
	if err != nil {
		return err
	}
	if !found {
		app, err := askApp(p)
		if err != nil {
			return err
		}
		if err := config.SaveApp(cfg.SessionDir, app); err != nil {
			return err
		}
		cfg.App = app
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	shown := 0
	show := func(url string) error {
		code, err := qrcode.Render(url, o.ascii)
		if err != nil {
			return err
		}
		if shown == 0 {
			p.say("\nOpen Telegram on your phone, go to Settings -> Devices,\n" +
				"tap Link Desktop Device and scan this code. Ctrl-C cancels.\n\n")
		} else {
			p.say("\nThe code expired; here is a new one.\n\n")
		}
		shown++
		p.say(code)
		return nil
	}
	password := func(context.Context) (string, error) {
		return p.secret("Two-step verification password (hidden): ")
	}
	already, err := telegram.Login(ctx, cfg, show, password)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return errors.New("login cancelled")
		}
		return fmt.Errorf("login failed: %w", err)
	}
	if already {
		fmt.Println("already linked; session left unchanged")
	} else {
		fmt.Println("linked; start lazygram to read your chats")
	}
	return nil
}

func askApp(p *ttyPrompter) (config.App, error) {
	p.say(appHelp)
	id, err := p.line("Application id: ")
	if err != nil {
		return config.App{}, err
	}
	hash, err := p.secret("Application hash (hidden): ")
	if err != nil {
		return config.App{}, err
	}
	return config.ParseApp(id, hash)
}

// ttyPrompter asks on the terminal. Secrets are read without echo.
type ttyPrompter struct {
	in  *bufio.Reader
	out io.Writer
}

func (t *ttyPrompter) say(s string) { _, _ = fmt.Fprint(t.out, s) }

func (t *ttyPrompter) line(prompt string) (string, error) {
	t.say(prompt)
	s, err := t.in.ReadString('\n')
	if err != nil && s == "" {
		return "", err
	}
	return strings.TrimSpace(s), nil
}

func (t *ttyPrompter) secret(prompt string) (string, error) {
	t.say(prompt)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	t.say("\n")
	return strings.TrimSpace(string(b)), err
}
