package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lazygram/internal/config"
)

func TestNotLinkedExitsWithHint(t *testing.T) {
	err := run([]string{"-data", t.TempDir()})
	var ec exitError
	if !errors.As(err, &ec) || ec.code != 2 || !strings.Contains(ec.msg, "lazygram login") || !strings.Contains(ec.msg, "lazygram demo") {
		t.Fatalf("got %v", err)
	}
}

func TestAppStoredButNoSessionStillAsks(t *testing.T) {
	dir := t.TempDir()
	if err := config.SaveApp(config.TelegramDir(dir), config.App{APIID: 1, APIHash: strings.Repeat("ab", 16)}); err != nil {
		t.Fatal(err)
	}
	err := run([]string{"-data", dir})
	var ec exitError
	if !errors.As(err, &ec) || ec.code != 2 {
		t.Fatalf("got %v", err)
	}
}

func TestCorruptAppFileIsReported(t *testing.T) {
	dir := t.TempDir()
	tg := config.TelegramDir(dir)
	if err := os.MkdirAll(tg, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tg, "app.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run([]string{"-data", dir}); err == nil || !strings.Contains(err.Error(), "app.json") {
		t.Fatalf("got %v", err)
	}
}

func TestUnknownCommand(t *testing.T) {
	err := run([]string{"-data", t.TempDir(), "bogus"})
	var ec exitError
	if !errors.As(err, &ec) || ec.code != 2 {
		t.Fatalf("got %v", err)
	}
}

func TestDemoSmokePrintsScreen(t *testing.T) {
	// stdin is not a terminal under go test, so demo falls back to one screen.
	if err := run([]string{"-data", t.TempDir(), "-smoke", "demo"}); err != nil {
		t.Fatal(err)
	}
}

func TestEmbeddedAppIsUsed(t *testing.T) {
	buildAppID, buildAppHash = "42", strings.Repeat("ab", 16)
	defer func() { buildAppID, buildAppHash = "", "" }()
	cfg, found, err := telegramConfig(t.TempDir())
	if err != nil || !found || cfg.App.APIID != 42 {
		t.Fatalf("%+v %v %v", cfg, found, err)
	}
	buildAppID = "x"
	if _, _, err := telegramConfig(t.TempDir()); err == nil {
		t.Fatal("bad embedded id must be reported")
	}
}
