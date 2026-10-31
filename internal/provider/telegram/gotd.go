package telegram

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	gotd "github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"

	"lazygram/internal/config"
	"lazygram/internal/domain"
)

const sessionFile = "session.json"

// ErrNotLoggedIn means no account is linked: there is no session, or the
// session is no longer authorized (for example after "Terminate session" on
// the phone). Linking is the separate, user-run Login.
var ErrNotLoggedIn = errors.New("not logged in")

// HasSession reports whether a session file exists. It does not prove the
// session is still authorized; Connect does.
func HasSession(cfg config.Telegram) bool {
	_, err := os.Stat(filepath.Join(cfg.SessionDir, sessionFile))
	return err == nil
}

// MTProto is the gotd/td backed implementation of Client. It connects lazily
// and only reads.
type MTProto struct {
	cfg config.Telegram

	once   sync.Once
	ready  chan struct{} // closed when connected and authorized, or failed
	cancel context.CancelFunc
	failed error

	mu  sync.Mutex // serializes use of svc
	svc *service
}

// NewClient prepares a client. No network connection is made until Connect
// or the first call to Dialogs or History.
func NewClient(cfg config.Telegram) *MTProto {
	return &MTProto{cfg: cfg, ready: make(chan struct{})}
}

// Connect establishes the connection and verifies the session is authorized.
// Without a session file it fails fast with ErrNotLoggedIn and no network use.
func (c *MTProto) Connect(ctx context.Context) error {
	if !HasSession(c.cfg) {
		return ErrNotLoggedIn
	}
	return c.wait(ctx)
}

// Close stops the connection, if any.
func (c *MTProto) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	return nil
}

func (c *MTProto) start() {
	runCtx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	go func() {
		err := c.run(runCtx)
		if c.svc == nil && c.failed == nil {
			c.failed = err
			close(c.ready)
		}
	}()
}

func (c *MTProto) run(ctx context.Context) error {
	cl, err := newGotdClient(c.cfg, gotd.Options{})
	if err != nil {
		return err
	}
	return cl.Run(ctx, func(ctx context.Context) error {
		st, err := cl.Auth().Status(ctx)
		if err != nil {
			return fmt.Errorf("telegram: auth status: %w", err)
		}
		if !st.Authorized {
			return ErrNotLoggedIn
		}
		c.svc = newService(floodAPI{api: tg.NewClient(cl), sleep: sleepCtx})
		close(c.ready)
		<-ctx.Done()
		return nil
	})
}

// newGotdClient builds a client whose session lives in cfg.SessionDir (0700).
func newGotdClient(cfg config.Telegram, opts gotd.Options) (*gotd.Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg.SessionDir, 0o700); err != nil {
		return nil, fmt.Errorf("telegram: session dir: %w", err)
	}
	opts.SessionStorage = &gotd.FileSessionStorage{Path: filepath.Join(cfg.SessionDir, sessionFile)}
	return gotd.NewClient(cfg.App.APIID, cfg.App.APIHash, opts), nil
}

func (c *MTProto) wait(ctx context.Context) error {
	c.once.Do(c.start)
	select {
	case <-c.ready:
		return c.failed
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Dialogs implements Client.
func (c *MTProto) Dialogs(ctx context.Context) ([]domain.Chat, error) {
	if err := c.wait(ctx); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.svc.Dialogs(ctx)
}

// History implements Client.
func (c *MTProto) History(ctx context.Context, since time.Time) ([]domain.Message, error) {
	if err := c.wait(ctx); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.svc.History(ctx, since)
}

// Send implements Client.
func (c *MTProto) Send(ctx context.Context, chatID, text string) error {
	if err := c.wait(ctx); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.svc.Send(ctx, chatID, text)
}

// Recent implements Client.
func (c *MTProto) Recent(ctx context.Context, chatID string, limit int) ([]domain.Message, error) {
	if err := c.wait(ctx); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.svc.Recent(ctx, chatID, limit)
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
