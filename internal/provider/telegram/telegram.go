// Package telegram is the live Telegram connector boundary.
//
// v0.1 ships only the validated construction path and an inert provider: no
// Telegram client library is linked and no network connection is made. A real
// client (for example an MTProto library) plugs in behind the Client
// interface once application credentials and an approved login flow exist.
// Credentials come from the environment only; see package config.
package telegram

import (
	"context"
	"fmt"
	"time"

	"lazygram/internal/config"
	"lazygram/internal/domain"
	"lazygram/internal/provider"
)

// Client is what a real Telegram library adapter must implement.
type Client interface {
	Dialogs(ctx context.Context, refs []config.ChatRef) ([]domain.Chat, error)
	History(ctx context.Context, since time.Time) ([]domain.Message, error)
}

// Provider adapts a Client to provider.Provider.
type Provider struct {
	cfg    config.Telegram
	client Client
}

// New validates cfg and returns the provider. With a nil client the provider
// is inert and every call returns provider.ErrNotImplemented.
func New(cfg config.Telegram, client Client) (*Provider, error) {
	if cfg.APIID <= 0 || cfg.APIHash == "" || cfg.SessionDir == "" {
		return nil, fmt.Errorf("telegram: incomplete configuration")
	}
	return &Provider{cfg: cfg, client: client}, nil
}

// Name implements provider.Provider.
func (p *Provider) Name() string { return config.ProviderTelegram }

// Chats implements provider.Provider.
func (p *Provider) Chats(ctx context.Context) ([]domain.Chat, error) {
	if p.client == nil {
		return nil, fmt.Errorf("telegram: %w (no client library linked in v0.1; use the demo provider)", provider.ErrNotImplemented)
	}
	return p.client.Dialogs(ctx, p.cfg.Chats)
}

// Sync implements provider.Provider.
func (p *Provider) Sync(ctx context.Context, since time.Time) ([]domain.Message, error) {
	if p.client == nil {
		return nil, fmt.Errorf("telegram: %w", provider.ErrNotImplemented)
	}
	return p.client.History(ctx, since)
}
