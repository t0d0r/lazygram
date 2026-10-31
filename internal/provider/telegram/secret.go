package telegram

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/gotd/td/session"
	gotd "github.com/gotd/td/telegram"

	"lazygram/internal/config"
)

// SessionSecret returns the session's MTProto authorization key, the
// long-lived secret the message cache key is derived from. Nothing else on
// disk needs protecting then: without session.json the cache is unreadable,
// and a new login (new key) starts a fresh cache. It fails with
// ErrNotLoggedIn when there is no session.
func SessionSecret(ctx context.Context, cfg config.Telegram) ([]byte, error) {
	l := session.Loader{Storage: &gotd.FileSessionStorage{Path: filepath.Join(cfg.SessionDir, sessionFile)}}
	d, err := l.Load(ctx)
	if errors.Is(err, session.ErrNotFound) {
		return nil, ErrNotLoggedIn
	}
	if err != nil {
		return nil, err
	}
	if len(d.AuthKey) == 0 {
		return nil, ErrNotLoggedIn
	}
	return d.AuthKey, nil
}
