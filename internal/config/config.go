// Package config reads and validates settings from environment variables.
// Nothing here is read from, or written to, tracked files.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Environment variable names.
const (
	EnvProvider  = "LAZYGRAM_PROVIDER"    // "demo" (default) or "telegram"
	EnvDataDir   = "LAZYGRAM_DATA_DIR"    // local runtime state directory
	EnvTGAPIID   = "LAZYGRAM_TG_API_ID"   // Telegram application id (integer)
	EnvTGAPIHash = "LAZYGRAM_TG_API_HASH" // Telegram application hash
	EnvTGSession = "LAZYGRAM_TG_SESSION"  // directory for the local session state
	EnvTGChats   = "LAZYGRAM_TG_CHATS"    // comma list of alias=chat-ref entries
)

// Provider names.
const (
	ProviderDemo     = "demo"
	ProviderTelegram = "telegram"
)

// Config is the validated runtime configuration.
type Config struct {
	Provider string
	DataDir  string // root for per-provider history caches
	Telegram Telegram
}

// Telegram holds the live connector settings. Zero unless provider=telegram.
type Telegram struct {
	APIID      int
	APIHash    string
	SessionDir string
	Chats      []ChatRef
}

// ChatRef maps a display alias to a Telegram chat reference.
type ChatRef struct {
	Alias string
	Ref   string
}

// Getenv abstracts os.Getenv for tests.
type Getenv func(string) string

// Load builds a Config from the environment, applying defaults and validating.
func Load(getenv Getenv) (Config, error) {
	c := Config{Provider: strings.ToLower(strings.TrimSpace(getenv(EnvProvider)))}
	if c.Provider == "" {
		c.Provider = ProviderDemo
	}
	c.DataDir = strings.TrimSpace(getenv(EnvDataDir))
	if c.DataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return c, fmt.Errorf("%s not set and home directory unknown: %w", EnvDataDir, err)
		}
		c.DataDir = filepath.Join(home, ".local", "state", "lazygram")
	}
	switch c.Provider {
	case ProviderDemo:
		return c, nil
	case ProviderTelegram:
		tg, err := loadTelegram(getenv)
		c.Telegram = tg
		return c, err
	default:
		return c, fmt.Errorf("%s: unknown provider %q (want %s or %s)", EnvProvider, c.Provider, ProviderDemo, ProviderTelegram)
	}
}

func loadTelegram(getenv Getenv) (Telegram, error) {
	var tg Telegram
	var errs []error
	if raw := strings.TrimSpace(getenv(EnvTGAPIID)); raw == "" {
		errs = append(errs, fmt.Errorf("%s is required", EnvTGAPIID))
	} else if id, err := strconv.Atoi(raw); err != nil || id <= 0 {
		errs = append(errs, fmt.Errorf("%s must be a positive integer", EnvTGAPIID))
	} else {
		tg.APIID = id
	}
	tg.APIHash = strings.TrimSpace(getenv(EnvTGAPIHash))
	if tg.APIHash == "" {
		errs = append(errs, fmt.Errorf("%s is required", EnvTGAPIHash))
	}
	tg.SessionDir = strings.TrimSpace(getenv(EnvTGSession))
	if tg.SessionDir == "" {
		errs = append(errs, fmt.Errorf("%s is required (a local, git-ignored directory)", EnvTGSession))
	}
	chats, err := ParseChats(getenv(EnvTGChats))
	if err != nil {
		errs = append(errs, err)
	}
	tg.Chats = chats
	return tg, errors.Join(errs...)
}

// ParseChats parses "alias=ref,alias2=ref2". Aliases must be unique, non-empty
// and free of spaces. An empty string means no chats were configured.
func ParseChats(s string) ([]ChatRef, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	var out []ChatRef
	seen := map[string]bool{}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		alias, ref, ok := strings.Cut(part, "=")
		alias, ref = strings.TrimSpace(alias), strings.TrimSpace(ref)
		switch {
		case !ok || alias == "" || ref == "":
			return nil, fmt.Errorf("%s: entry %q is not alias=ref", EnvTGChats, part)
		case strings.ContainsAny(alias, " \t#/"):
			return nil, fmt.Errorf("%s: alias %q must not contain spaces, # or /", EnvTGChats, alias)
		case seen[strings.ToLower(alias)]:
			return nil, fmt.Errorf("%s: duplicate alias %q", EnvTGChats, alias)
		}
		seen[strings.ToLower(alias)] = true
		out = append(out, ChatRef{Alias: alias, Ref: ref})
	}
	return out, nil
}
