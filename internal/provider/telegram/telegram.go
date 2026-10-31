// Package telegram is the live Telegram connector boundary.
//
// The provider talks to Telegram through the Client interface. MTProto
// (gotd/td) is the real implementation; with a nil client the provider is
// inert. It only reads, never logs in, and connects only when selected by
// environment configuration. Credentials come from the environment only; see
// package config.
package telegram

import (
	"context"
	"fmt"
	"io"
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

// Close releases the underlying client when it has one.
func (p *Provider) Close() error {
	if c, ok := p.client.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// Name implements provider.Provider.
func (p *Provider) Name() string { return config.ProviderTelegram }

// Chats implements provider.Provider.
func (p *Provider) Chats(ctx context.Context) ([]domain.Chat, error) {
	if p.client == nil {
		return nil, fmt.Errorf("telegram: %w (no client configured; use the demo provider)", provider.ErrNotImplemented)
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
