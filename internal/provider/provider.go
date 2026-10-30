// Package provider defines the connector interface. Each messaging service
// implements Provider; the core UI and domain never import a connector.
package provider

import (
	"context"
	"errors"
	"time"

	"lazygram/internal/domain"
)

// ErrNotImplemented is returned by connectors that are present but inert.
var ErrNotImplemented = errors.New("live connector not implemented")

// Provider is a source of chats and messages.
type Provider interface {
	// Name is the service name, e.g. "demo" or "telegram".
	Name() string
	// Chats lists the configured chats.
	Chats(ctx context.Context) ([]domain.Chat, error)
	// Sync returns messages newer than since (zero means full backlog).
	Sync(ctx context.Context, since time.Time) ([]domain.Message, error)
}
