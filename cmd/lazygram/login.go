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
	lock, err := lockState(o.dataDir, os.Stderr)
	if err != nil {
		return err
	}
	defer func() { _ = lock.Release() }()
	if !isTerminal(os.Stdin) {
		return errors.New("login needs an interactive terminal")
	}
	p := &ttyPrompter{in: bufio.NewReader(os.Stdin), out: os.Stderr}
	cfg, src, err := telegramConfig(o.dataDir)
	if err != nil {
		return err
	}
	cfg, err = loginApp(cfg, src, func() (config.App, error) { return askApp(p) }, p.say)
	if err != nil {
		return err
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
	p.say("connecting to Telegram... (Ctrl-C cancels)\n")
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

// loginApp makes sure a registration is in place before the QR login:
// stored and built-in values are used as they are, a value from the
// environment is stored so later starts need no environment, and when there
// is none ask supplies one, which is stored too.
func loginApp(cfg config.Telegram, src appSource, ask func() (config.App, error), say func(string)) (config.Telegram, error) {
	switch src {
	case appNone:
		app, err := ask()
		if err != nil {
			return cfg, err
		}
		if err := config.SaveApp(cfg.SessionDir, app); err != nil {
			return cfg, err
		}
		cfg.App = app
	case appEnv:
		if err := config.SaveApp(cfg.SessionDir, cfg.App); err != nil {
			return cfg, err
		}
		say("using the application registration from the environment; stored in the state directory\n")
	}
	return cfg, nil
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
	return config.ParseApp(id, strings.TrimSpace(hash))
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

// secret reads without echo and returns the input untouched: a password
// may begin or end with spaces.
func (t *ttyPrompter) secret(prompt string) (string, error) {
	t.say(prompt)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	t.say("\n")
	return string(b), err
}
