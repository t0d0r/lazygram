package telegram

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"

	"lazygram/internal/config"
)

// ErrSignUpUnsupported is returned when the phone number has no account.
var ErrSignUpUnsupported = errors.New("this number has no Telegram account; lazygram does not create accounts")

// Prompter supplies the interactive answers a login needs. Implementations
// must not log or echo the values they return.
type Prompter interface {
	Phone(ctx context.Context) (string, error)
	Code(ctx context.Context) (string, error)
	Password(ctx context.Context) (string, error)
}

// authenticator adapts a Prompter to gotd's auth.UserAuthenticator.
type authenticator struct{ p Prompter }

func (a authenticator) Phone(ctx context.Context) (string, error) {
	s, err := a.p.Phone(ctx)
	if err != nil {
		return "", err
	}
	return NormalizePhone(s)
}

func (a authenticator) Code(ctx context.Context, _ *tg.AuthSentCode) (string, error) {
	s, err := a.p.Code(ctx)
	return strings.TrimSpace(s), err
}

func (a authenticator) Password(ctx context.Context) (string, error) { return a.p.Password(ctx) }

func (authenticator) SignUp(context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, ErrSignUpUnsupported
}

func (authenticator) AcceptTermsOfService(context.Context, tg.HelpTermsOfService) error {
	return ErrSignUpUnsupported
}

// NormalizePhone accepts international format with optional spaces, dashes,
// dots or parentheses ("+49 170 1234567") and returns "+" and digits.
func NormalizePhone(s string) (string, error) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "+") {
		return "", errors.New("phone number must start with + and the country code")
	}
	var b strings.Builder
	b.WriteByte('+')
	for _, r := range s[1:] {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case strings.ContainsRune(" -.()", r):
		default:
			return "", errors.New("phone number may contain only digits, spaces, dashes, dots and parentheses")
		}
	}
	if n := b.Len() - 1; n < 7 || n > 15 {
		return "", errors.New("phone number must have 7 to 15 digits")
	}
	return b.String(), nil
}

// Login signs in once and stores the session in cfg.SessionDir. It returns
// already=true, without prompting, when an authorized session exists. It never
// creates accounts and never writes anything outside the session directory.
func Login(ctx context.Context, cfg config.Telegram, p Prompter) (already bool, err error) {
	if cfg.APIID <= 0 || cfg.APIHash == "" || cfg.SessionDir == "" {
		return false, fmt.Errorf("telegram: incomplete configuration")
	}
	cl, err := newGotdClient(cfg)
	if err != nil {
		return false, err
	}
	err = cl.Run(ctx, func(ctx context.Context) error {
		st, err := cl.Auth().Status(ctx)
		if err != nil {
			return fmt.Errorf("telegram: auth status: %w", err)
		}
		if st.Authorized {
			already = true
			return nil
		}
		return cl.Auth().IfNecessary(ctx, auth.NewFlow(authenticator{p}, auth.SendCodeOptions{}))
	})
	if err != nil {
		return false, err
	}
	// gotd writes 0600 already; enforce it in case an older file was looser.
	if err := os.Chmod(filepath.Join(cfg.SessionDir, sessionFile), 0o600); err != nil && !os.IsNotExist(err) {
		return already, err
	}
	return already, nil
}
