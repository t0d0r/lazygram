// Package provider defines the connector interface. Each messaging service
// implements Provider; the core UI and domain never import a connector.
package provider

import (
	"context"
	"errors"
	"fmt"
	"time"

	"lazygram/internal/domain"
)

// ErrNotImplemented is returned by connectors that are present but inert.
var ErrNotImplemented = errors.New("live connector not implemented")

// TypingSource is implemented by a service that reports who is typing.
// Typing returns the feed of reports; the service closes it when it shuts
// down. A nil feed means the service reports nothing, and the caller must
// not wait on it. Nothing is ever sent back: lazygram never reports its
// own typing.
type TypingSource interface {
	Typing() <-chan domain.Typing
}

// ChangeSource is implemented by a service that reports edits, deletions
// and reactions as they happen. Changes returns the feed; the service
// closes it when it shuts down; a nil feed reports nothing. A change may
// be dropped when the reader is behind: the next fetch of the chat's
// history brings the cache up to date anyway.
type ChangeSource interface {
	Changes() <-chan domain.Change
}

// ReactionPoller is implemented by a service whose reactions to other
// people's messages must be asked for: Telegram pushes reactions only for
// the account's own messages and asks apps to poll the messages on screen
// every 15 to 30 seconds. PollReactions asks for the reactions of the
// given messages of one chat and returns one domain.Reacted change per
// message it learned about, to be applied like those of the change feed;
// a message the service leaves out of its answer has no reactions. It
// returns ErrNotPollable when the chat cannot be polled (the caller skips
// it for the session) and a *WaitError when the service asked for a pause.
type ReactionPoller interface {
	PollReactions(ctx context.Context, chatID string, ids []string) ([]domain.Change, error)
}

// ErrNotPollable says a chat's reactions cannot be asked for (Telegram: a
// private or invalid channel, or message ids it does not take).
var ErrNotPollable = errors.New("reactions cannot be polled in this chat")

// WaitError says the service asked lazygram to wait (Telegram's
// FLOOD_WAIT) before it polls again; nothing is polled until Wait has
// passed.
type WaitError struct{ Wait time.Duration }

func (e *WaitError) Error() string { return fmt.Sprintf("the service asks to wait %s", e.Wait) }

// ErrMembersHidden says a chat does not show its member list to this
// account: a broadcast channel, whose subscribers only admins see, or a
// group whose admins hid its members.
var ErrMembersHidden = errors.New("member list not visible")

// MemberLister is implemented by a service that can list a group's
// members. Members returns them, without this account and deleted
// accounts, or ErrMembersHidden. It may stop at a cap on large groups.
type MemberLister interface {
	Members(ctx context.Context, chatID string) ([]domain.Participant, error)
}

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
	// MarkRead tells the service that the chat was read up to and
	// including message msgID, or up to its newest message at the service
	// when msgID is empty: a read receipt. It is called only when the user
	// turned read receipts on (off is the default) or asked for one.
	MarkRead(ctx context.Context, chatID, msgID string) error
	// MarkUnread sets (unread) or clears the chat's unread mark at the
	// service, the "mark as unread" of the official apps. Called only when
	// the user asks, once per request.
	MarkUnread(ctx context.Context, chatID string, unread bool) error
}
