package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"lazygram/internal/config"
)

func TestNotLinkedExitsWithHint(t *testing.T) {
	clearEnv(t)
	err := run([]string{"-data", t.TempDir()})
	var ec exitError
	if !errors.As(err, &ec) || ec.code != 2 || !strings.Contains(ec.msg, "lazygram login") || !strings.Contains(ec.msg, "lazygram demo") {
		t.Fatalf("got %v", err)
	}
}

func TestAppStoredButNoSessionStillAsks(t *testing.T) {
	clearEnv(t)
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
	clearEnv(t)
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

func clearEnv(t *testing.T) {
	t.Helper()
	for _, n := range []string{"TG_API_ID", "TG_API_HASH", "TG_APIID", "TG_APIHASH"} {
		t.Setenv(n, "")
	}
}

func TestEmbeddedAppIsUsed(t *testing.T) {
	clearEnv(t)
	buildAppID, buildAppHash = "42", strings.Repeat("ab", 16)
	defer func() { buildAppID, buildAppHash = "", "" }()
	cfg, src, err := telegramConfig(t.TempDir())
	if err != nil || src != appBuilt || cfg.App.APIID != 42 {
		t.Fatalf("%+v %v %v", cfg, src, err)
	}
	buildAppID = "x"
	if _, _, err := telegramConfig(t.TempDir()); err == nil {
		t.Fatal("bad embedded id must be reported")
	}
}

func TestEnvironmentAppIsUsedAndStoredFileWins(t *testing.T) {
	clearEnv(t)
	t.Setenv("TG_API_ID", "7")
	t.Setenv("TG_API_HASH", strings.Repeat("cd", 16))
	dir := t.TempDir()
	cfg, src, err := telegramConfig(dir)
	if err != nil || src != appEnv || cfg.App.APIID != 7 {
		t.Fatalf("%+v %v %v", cfg, src, err)
	}
	if err := config.SaveApp(config.TelegramDir(dir), config.App{APIID: 1, APIHash: strings.Repeat("ab", 16)}); err != nil {
		t.Fatal(err)
	}
	cfg, src, err = telegramConfig(dir)
	if err != nil || src != appStored || cfg.App.APIID != 1 {
		t.Fatalf("stored must win: %+v %v %v", cfg, src, err)
	}
}

func TestBrokenEnvironmentIsReportedWithoutValue(t *testing.T) {
	clearEnv(t)
	t.Setenv("TG_API_ID", "notanumber")
	t.Setenv("TG_API_HASH", strings.Repeat("cd", 16))
	_, _, err := telegramConfig(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "TG_API_ID") || strings.Contains(err.Error(), "notanumber") {
		t.Fatalf("got %v", err)
	}
}

func TestRemoveLegacyCache(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"messages.jsonl", "chats.json"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	removeLegacyCache(dir)
	if _, err := os.Stat(filepath.Join(dir, "messages.jsonl")); !os.IsNotExist(err) {
		t.Fatal("legacy cache must be removed")
	}
	removeLegacyCache(dir) // idempotent
}

func TestFlagsAfterSubcommand(t *testing.T) {
	clearEnv(t)
	if err := run([]string{"demo", "-data", t.TempDir(), "-smoke"}); err != nil {
		t.Fatalf("flags after the subcommand: %v", err)
	}
	err := run([]string{"-data", t.TempDir(), "demo", "extra"})
	var ec exitError
	if !errors.As(err, &ec) || ec.code != 2 || !strings.Contains(ec.msg, "unexpected argument") {
		t.Fatalf("got %v", err)
	}
	err = run([]string{"-data", t.TempDir(), "demo", "-bogus"})
	if !errors.As(err, &ec) || ec.code != 2 {
		t.Fatalf("unknown flag after subcommand: %v", err)
	}
}

func TestUpdateHint(t *testing.T) {
	if updateHint("dev", time.Second) != "" || updateHint("v0.3.0-2-gabc", time.Second) != "" {
		t.Fatal("builds from source get no hint")
	}
	// A release build with no network answers quietly within the timeout.
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:9") // nothing listens there
	start := time.Now()
	if h := updateHint("v0.1.0", 500*time.Millisecond); h != "" {
		t.Fatalf("unexpected hint: %q", h)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("the check must stay brief")
	}
}
