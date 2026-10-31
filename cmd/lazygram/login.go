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
)

// runLogin implements `lazygram login`: a one-time, user-run Telegram sign-in
// that stores the session under LAZYGRAM_TG_SESSION. Answers are typed at the
// prompt; nothing is read from files or flags, and nothing is printed back.
func runLogin() error {
	if !isTerminal(os.Stdin) {
		return errors.New("login needs an interactive terminal")
	}
	cfg, err := config.Load(func(k string) string {
		if k == config.EnvProvider {
			return config.ProviderTelegram
		}
		return os.Getenv(k)
	})
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	already, err := telegram.Login(ctx, cfg.Telegram, &ttyPrompter{in: bufio.NewReader(os.Stdin), out: os.Stderr})
	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}
	if already {
		fmt.Println("already logged in; session left unchanged")
	} else {
		fmt.Println("logged in; session saved to", cfg.Telegram.SessionDir)
	}
	return nil
}

// ttyPrompter asks on the terminal. The password is read without echo.
type ttyPrompter struct {
	in  *bufio.Reader
	out io.Writer
}

func (t *ttyPrompter) line(prompt string) (string, error) {
	_, _ = fmt.Fprint(t.out, prompt)
	s, err := t.in.ReadString('\n')
	if err != nil && s == "" {
		return "", err
	}
	return strings.TrimSpace(s), nil
}

func (t *ttyPrompter) Phone(context.Context) (string, error) {
	return t.line("Phone number (international format, e.g. +1...): ")
}

func (t *ttyPrompter) Code(context.Context) (string, error) {
	return t.line("Login code sent by Telegram: ")
}

func (t *ttyPrompter) Password(context.Context) (string, error) {
	_, _ = fmt.Fprint(t.out, "Two-step verification password (hidden): ")
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	_, _ = fmt.Fprintln(t.out)
	return string(b), err
}
