package telegram

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	gotd "github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth/qrlogin"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"

	"lazygram/internal/config"
)

// Login links this device to a Telegram account by QR code, the way nchat
// and the official desktop apps do: the phone app scans the code under
// Settings -> Devices -> Link Desktop Device. No phone number or SMS code is
// typed. show is called with the login URL to display as a QR code, again if
// the code expires before it is scanned. password is asked once if the
// account has two-step verification. The session is stored in
// cfg.SessionDir. It returns already=true, without showing anything, when an
// authorized session exists. Accounts are never created.
func Login(ctx context.Context, cfg config.Telegram,
	show func(url string) error, password func(ctx context.Context) (string, error),
) (already bool, err error) {
	if err := cfg.Validate(); err != nil {
		return false, err
	}
	if show == nil || password == nil {
		return false, errors.New("telegram: login needs show and password callbacks")
	}
	dispatcher := tg.NewUpdateDispatcher()
	loggedIn := qrlogin.OnLoginToken(dispatcher)
	cl, err := newGotdClient(cfg, gotd.Options{UpdateHandler: dispatcher})
	if err != nil {
		return false, err
	}
	linked := false
	err = cl.Run(ctx, func(ctx context.Context) error {
		st, err := cl.Auth().Status(ctx)
		if err != nil {
			return fmt.Errorf("telegram: auth status: %w", err)
		}
		if st.Authorized {
			already = true
			return nil
		}
		_, err = cl.QR().Auth(ctx, loggedIn, func(_ context.Context, token qrlogin.Token) error {
			return show(token.URL())
		})
		switch {
		case err == nil:
		case tgerr.Is(err, "SESSION_PASSWORD_NEEDED"):
			pw, err := password(ctx)
			if err != nil {
				return err
			}
			if _, err := cl.Auth().Password(ctx, pw); err != nil {
				return fmt.Errorf("telegram: two-step verification: %w", err)
			}
		default:
			return fmt.Errorf("telegram: qr login: %w", err)
		}
		linked = true
		return nil
	})
	// gotd's Run reports nil when the context was cancelled, so a Ctrl-C
	// must not read as success.
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	if err != nil {
		return false, err
	}
	if !already && !linked {
		return false, errors.New("telegram: login did not complete")
	}
	// gotd writes 0600 already; enforce it in case an older file was looser.
	if err := os.Chmod(filepath.Join(cfg.SessionDir, sessionFile), 0o600); err != nil && !os.IsNotExist(err) {
		return already, err
	}
	return already, nil
}
