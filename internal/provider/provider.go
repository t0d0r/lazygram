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
	// Sync returns messages that arrived since the last call. since is the
	// newest time already cached; a provider may use it or ignore it.
	Sync(ctx context.Context, since time.Time) ([]domain.Message, error)
	// Recent returns the last limit messages of one chat, oldest first.
	Recent(ctx context.Context, chatID string, limit int) ([]domain.Message, error)
	// Send posts a text message to a chat. The sent message is not returned;
	// it arrives through Sync like any other.
	Send(ctx context.Context, chatID, text string) error
	// Press presses a callback button of a message and returns the bot's
	// answer, with the message as it is afterwards when it changed.
	Press(ctx context.Context, chatID, msgID string, b domain.Button) (domain.PressAnswer, error)
	// Fetch downloads a message's attachment and returns its bytes with a
	// file name (extension at least) for the viewer. Nothing is written to
	// disk by the provider; the caller keeps it encrypted.
	Fetch(ctx context.Context, chatID, msgID string) (data []byte, name string, err error)
	// SendFile uploads data as a photo (when name has an image extension)
	// or as a file, with an optional caption.
	SendFile(ctx context.Context, chatID, name string, data []byte, caption string) error
	// Edit replaces the text of one of this account's messages.
	Edit(ctx context.Context, chatID, msgID, text string) error
	// Delete removes one of this account's messages for everyone.
	Delete(ctx context.Context, chatID, msgID string) error
}
