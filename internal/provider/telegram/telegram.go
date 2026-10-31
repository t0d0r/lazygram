// Package telegram is the Telegram connector.
//
// The provider talks to Telegram through the Client interface. MTProto
// (gotd/td) is the real implementation; with a nil client the provider is
// inert. The running client reads, and sends only what the user types and
// confirms with Ctrl-X. Linking an account is the separate, user-run QR
// login (see Login). Application credentials and the session
// live in local state only; see package config.
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

// Client is what a Telegram transport must implement.
type Client interface {
	Dialogs(ctx context.Context) ([]domain.Chat, error)
	History(ctx context.Context, since time.Time) ([]domain.Message, error)
	Recent(ctx context.Context, chatID string, limit int) ([]domain.Message, error)
	Send(ctx context.Context, chatID, text string) error
}

// Provider adapts a Client to provider.Provider.
type Provider struct {
	client Client
}

// New validates cfg and returns the provider. With a nil client the provider
// is inert and every call returns provider.ErrNotImplemented.
func New(cfg config.Telegram, client Client) (*Provider, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Provider{client: client}, nil
}

// Close releases the underlying client when it has one.
func (p *Provider) Close() error {
	if c, ok := p.client.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// Name implements provider.Provider.
func (p *Provider) Name() string { return "telegram" }

// Chats implements provider.Provider.
func (p *Provider) Chats(ctx context.Context) ([]domain.Chat, error) {
	if p.client == nil {
		return nil, fmt.Errorf("telegram: %w (no client configured)", provider.ErrNotImplemented)
	}
	return p.client.Dialogs(ctx)
}

// Sync implements provider.Provider.
func (p *Provider) Sync(ctx context.Context, since time.Time) ([]domain.Message, error) {
	if p.client == nil {
		return nil, fmt.Errorf("telegram: %w", provider.ErrNotImplemented)
	}
	return p.client.History(ctx, since)
}

// Send implements provider.Provider.
func (p *Provider) Send(ctx context.Context, chatID, text string) error {
	if p.client == nil {
		return fmt.Errorf("telegram: %w", provider.ErrNotImplemented)
	}
	return p.client.Send(ctx, chatID, text)
}

// Recent implements provider.Provider.
func (p *Provider) Recent(ctx context.Context, chatID string, limit int) ([]domain.Message, error) {
	if p.client == nil {
		return nil, fmt.Errorf("telegram: %w", provider.ErrNotImplemented)
	}
	return p.client.Recent(ctx, chatID, limit)
}
