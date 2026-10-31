package telegram

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"lazygram/internal/config"
	"lazygram/internal/provider"
)

var testCfg = config.Telegram{App: config.App{APIID: 1, APIHash: strings.Repeat("ab", 16)}, SessionDir: "/s"}

func TestNewValidates(t *testing.T) {
	if _, err := New(config.Telegram{}, nil); err == nil {
		t.Fatal("empty config must be rejected")
	}
}

func TestInertWithoutClient(t *testing.T) {
	p, err := New(testCfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Chats(context.Background()); !errors.Is(err, provider.ErrNotImplemented) {
		t.Fatalf("chats: %v", err)
	}
	if _, err := p.Sync(context.Background(), time.Time{}); !errors.Is(err, provider.ErrNotImplemented) {
		t.Fatalf("sync: %v", err)
	}
	if _, err := p.Recent(context.Background(), "x", 5); !errors.Is(err, provider.ErrNotImplemented) {
		t.Fatalf("recent: %v", err)
	}
	if err := p.Send(context.Background(), "x", "y"); !errors.Is(err, provider.ErrNotImplemented) {
		t.Fatalf("send: %v", err)
	}
	var _ provider.Provider = p
}

func TestMTProtoMakesNoConnectionOnConstruction(t *testing.T) {
	cfg := testCfg
	cfg.SessionDir = t.TempDir()
	c := NewClient(cfg)
	if c.cancel != nil {
		t.Fatal("client must connect lazily")
	}
	if HasSession(cfg) {
		t.Fatal("no session file was written")
	}
	p, err := New(cfg, c)
	if err != nil || p.Close() != nil {
		t.Fatalf("%v", err)
	}
}

func TestLoginRejectsIncompleteConfig(t *testing.T) {
	_, err := Login(context.Background(), config.Telegram{}, nil, nil)
	if err == nil {
		t.Fatal("want error")
	}
}

func TestSessionSecretWithoutSession(t *testing.T) {
	cfg := testCfg
	cfg.SessionDir = t.TempDir()
	if _, err := SessionSecret(context.Background(), cfg); !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("got %v", err)
	}
}
