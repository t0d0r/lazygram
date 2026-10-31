package main

import (
	"bufio"
	"bytes"
	"errors"
	"strings"
	"testing"

	"lazygram/internal/config"
)

func TestTTYPrompterReadsLines(t *testing.T) {
	var out bytes.Buffer
	p := &ttyPrompter{in: bufio.NewReader(strings.NewReader(" 12345 \nlast")), out: &out}
	if v, err := p.line("id: "); err != nil || v != "12345" {
		t.Fatalf("%q %v", v, err)
	}
	if v, err := p.line("x: "); err != nil || v != "last" {
		t.Fatalf("last line without newline: %q %v", v, err)
	}
	if _, err := p.line("x: "); err == nil {
		t.Fatal("EOF must be an error")
	}
	if strings.Contains(out.String(), "12345") {
		t.Fatal("answers must never be echoed back")
	}
}

func TestLoginRefusesNonTerminal(t *testing.T) {
	// go test runs with stdin not attached to a terminal.
	err := runLogin(options{dataDir: t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "interactive") {
		t.Fatalf("got %v", err)
	}
}

func TestLoginAppResolution(t *testing.T) {
	dir := t.TempDir()
	tg := config.TelegramDir(dir)
	app := config.App{APIID: 5, APIHash: strings.Repeat("ab", 16)}
	var said []string
	say := func(s string) { said = append(said, s) }
	asked := 0
	ask := func() (config.App, error) { asked++; return app, nil }

	// nothing known: ask and store
	cfg, err := loginApp(config.Telegram{SessionDir: tg}, appNone, ask, say)
	if err != nil || asked != 1 || cfg.App != app {
		t.Fatalf("%v %d %+v", err, asked, cfg)
	}
	if stored, found, _ := config.LoadApp(tg); !found || stored != app {
		t.Fatal("asked registration must be stored")
	}

	// from the environment: store and say so, never ask
	dir2 := config.TelegramDir(t.TempDir())
	cfg, err = loginApp(config.Telegram{App: app, SessionDir: dir2}, appEnv, ask, say)
	if err != nil || asked != 1 || len(said) != 1 || !strings.Contains(said[0], "environment") {
		t.Fatalf("%v %d %v", err, asked, said)
	}
	if _, found, _ := config.LoadApp(dir2); !found {
		t.Fatal("environment registration must be stored")
	}

	// stored or built in: use as is, write nothing, say nothing
	for _, src := range []appSource{appStored, appBuilt} {
		dir3 := config.TelegramDir(t.TempDir())
		if _, err := loginApp(config.Telegram{App: app, SessionDir: dir3}, src, ask, say); err != nil || asked != 1 || len(said) != 1 {
			t.Fatalf("src %v: %v %d %v", src, err, asked, said)
		}
		if _, found, _ := config.LoadApp(dir3); found {
			t.Fatalf("src %v must not write app.json", src)
		}
	}

	// ask failing
	if _, err := loginApp(config.Telegram{SessionDir: tg}, appNone, func() (config.App, error) { return config.App{}, errors.New("eof") }, say); err == nil {
		t.Fatal("ask error must surface")
	}
}
