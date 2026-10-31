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

// ErrNotLoggedIn means the local session state holds no authorized account.
// lazygram never logs in by itself; see specs/providers.md.
var ErrNotLoggedIn = errors.New("no authorized Telegram session found in " + config.EnvTGSession)

// MTProto is the gotd/td (MTProto) backed implementation of the package's
// Client interface. It connects lazily on first use and only reads.
type MTProto struct {
	cfg config.Telegram

	once   sync.Once
	ready  chan struct{} // closed when connected and authorized, or failed
	cancel context.CancelFunc
	failed error

	mu  sync.Mutex // serializes use of svc
	svc *service
}

// NewClient prepares an MTProto client. It makes no network connection until the first
// call to Dialogs or History.
func NewClient(cfg config.Telegram) *MTProto {
	return &MTProto{cfg: cfg, ready: make(chan struct{})}
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
	if err := os.MkdirAll(c.cfg.SessionDir, 0o700); err != nil {
		return fmt.Errorf("telegram: session dir: %w", err)
	}
	cl := gotd.NewClient(c.cfg.APIID, c.cfg.APIHash, gotd.Options{
		SessionStorage: &gotd.FileSessionStorage{Path: filepath.Join(c.cfg.SessionDir, "session.json")},
	})
	return cl.Run(ctx, func(ctx context.Context) error {
		st, err := cl.Auth().Status(ctx)
		if err != nil {
			return fmt.Errorf("telegram: auth status: %w", err)
		}
		if !st.Authorized {
			return ErrNotLoggedIn
		}
		c.svc = &service{api: tg.NewClient(cl)}
		close(c.ready)
		<-ctx.Done()
		return nil
	})
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
func (c *MTProto) Dialogs(ctx context.Context, refs []config.ChatRef) ([]domain.Chat, error) {
	if err := c.wait(ctx); err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.svc.Dialogs(ctx, refs)
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
