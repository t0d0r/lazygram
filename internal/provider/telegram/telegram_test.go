package telegram

import (
	"context"
	"errors"
	"testing"
	"time"

	"lazygram/internal/config"
	"lazygram/internal/provider"
)

func TestNewValidates(t *testing.T) {
	if _, err := New(config.Telegram{}, nil); err == nil {
		t.Fatal("empty config must be rejected")
	}
}

func TestInertWithoutClient(t *testing.T) {
	p, err := New(config.Telegram{APIID: 1, APIHash: "h", SessionDir: "/s"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Chats(context.Background()); !errors.Is(err, provider.ErrNotImplemented) {
		t.Fatalf("chats: %v", err)
	}
	if _, err := p.Sync(context.Background(), time.Time{}); !errors.Is(err, provider.ErrNotImplemented) {
		t.Fatalf("sync: %v", err)
	}
	var _ provider.Provider = p
}

func TestMTProtoMakesNoConnectionOnConstruction(t *testing.T) {
	c := NewClient(config.Telegram{APIID: 1, APIHash: "h", SessionDir: t.TempDir()})
	if c.cancel != nil {
		t.Fatal("client must connect lazily")
	}
	p, err := New(config.Telegram{APIID: 1, APIHash: "h", SessionDir: "/s"}, c)
	if err != nil || p.Close() != nil {
		t.Fatalf("%v", err)
	}
}
