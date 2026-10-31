// Package config holds runtime settings. Nothing is read from environment
// variables: the data directory comes from a flag (default under the home
// directory) and the Telegram application registration is stored once in
// local, git-ignored state.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	stateDirName = "lazygram"
	appFile      = "app.json"
)

// Config is the validated runtime configuration.
type Config struct {
	DataDir string // root for per-provider state and history caches
}

// DefaultDataDir is ~/.local/state/lazygram.
func DefaultDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home directory unknown, pass -data: %w", err)
	}
	return filepath.Join(home, ".local", "state", stateDirName), nil
}

// TelegramDir is where the Telegram connector keeps its session, application
// registration and history cache.
func TelegramDir(dataDir string) string { return filepath.Join(dataDir, "telegram") }

// App is a Telegram application registration (id and hash issued by
// https://my.telegram.org). MTProto cannot connect without one. It is
// private to the person who registered it and is never committed.
type App struct {
	APIID   int    `json:"api_id"`
	APIHash string `json:"api_hash"`
}

// Validate checks that both parts are present and plausible.
func (a App) Validate() error {
	var errs []error
	if a.APIID <= 0 {
		errs = append(errs, errors.New("application id must be a positive integer"))
	}
	if h := strings.TrimSpace(a.APIHash); h == "" {
		errs = append(errs, errors.New("application hash is required"))
	} else if len(h) < 16 || strings.ContainsAny(h, " \t\n") {
		errs = append(errs, errors.New("application hash looks wrong (expected a long hex string)"))
	}
	return errors.Join(errs...)
}

// ParseApp builds an App from typed input.
func ParseApp(id, hash string) (App, error) {
	n, err := strconv.Atoi(strings.TrimSpace(id))
	if err != nil {
		return App{}, errors.New("application id must be a positive integer")
	}
	a := App{APIID: n, APIHash: strings.TrimSpace(hash)}
	return a, a.Validate()
}

// LoadApp reads the stored registration from dir. found is false when none
// has been stored yet; a stored but invalid file is an error.
func LoadApp(dir string) (app App, found bool, err error) {
	b, err := os.ReadFile(filepath.Join(dir, appFile))
	if os.IsNotExist(err) {
		return App{}, false, nil
	}
	if err != nil {
		return App{}, false, err
	}
	if err := json.Unmarshal(b, &app); err != nil {
		return App{}, true, fmt.Errorf("%s is not valid JSON: %w", appFile, err)
	}
	if err := app.Validate(); err != nil {
		return App{}, true, fmt.Errorf("%s: %w", appFile, err)
	}
	return app, true, nil
}

// SaveApp stores the registration in dir (0700) as an owner-only file.
func SaveApp(dir string, a App) error {
	if err := a.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, appFile+".tmp")
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, appFile))
}

// Telegram is what the connector needs.
type Telegram struct {
	App        App
	SessionDir string // holds session.json; must be a private, git-ignored directory
}

// Validate checks the connector configuration.
func (t Telegram) Validate() error {
	if t.SessionDir == "" {
		return errors.New("telegram: session directory is required")
	}
	if err := t.App.Validate(); err != nil {
		return fmt.Errorf("telegram: %w", err)
	}
	return nil
}
