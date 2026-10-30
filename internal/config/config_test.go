package config

import (
	"strings"
	"testing"
)

func env(m map[string]string) Getenv { return func(k string) string { return m[k] } }

func TestDefaultsToDemo(t *testing.T) {
	c, err := Load(env(map[string]string{EnvDataDir: "/x"}))
	if err != nil || c.Provider != ProviderDemo || c.DataDir != "/x" {
		t.Fatalf("%+v %v", c, err)
	}
}

func TestUnknownProvider(t *testing.T) {
	_, err := Load(env(map[string]string{EnvProvider: "irc", EnvDataDir: "/x"}))
	if err == nil || !strings.Contains(err.Error(), "unknown provider") {
		t.Fatalf("got %v", err)
	}
}

func TestTelegramMissingEverything(t *testing.T) {
	_, err := Load(env(map[string]string{EnvProvider: "telegram", EnvDataDir: "/x"}))
	if err == nil {
		t.Fatal("want error")
	}
	for _, name := range []string{EnvTGAPIID, EnvTGAPIHash, EnvTGSession} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error should name %s: %v", name, err)
		}
	}
}

func TestTelegramErrorsNeverEchoSecrets(t *testing.T) {
	_, err := Load(env(map[string]string{
		EnvProvider: "telegram", EnvDataDir: "/x",
		EnvTGAPIID: "abc", EnvTGAPIHash: "s3cret-hash", EnvTGSession: "/s",
	}))
	if err == nil || strings.Contains(err.Error(), "s3cret-hash") || strings.Contains(err.Error(), "abc") {
		t.Fatalf("got %v", err)
	}
}

func TestTelegramValid(t *testing.T) {
	c, err := Load(env(map[string]string{
		EnvProvider: "Telegram", EnvDataDir: "/x",
		EnvTGAPIID: "12345", EnvTGAPIHash: "h", EnvTGSession: "/s",
		EnvTGChats: "dev=@somegroup, ops=-100200",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.Telegram.APIID != 12345 || len(c.Telegram.Chats) != 2 || c.Telegram.Chats[1].Alias != "ops" {
		t.Fatalf("%+v", c.Telegram)
	}
}

func TestParseChats(t *testing.T) {
	bad := []string{"novalue", "a=", "=b", "a b=c", "#a=c", "a=1,A=2"}
	for _, s := range bad {
		if _, err := ParseChats(s); err == nil {
			t.Errorf("%q should fail", s)
		}
	}
	if c, err := ParseChats(" , "); err != nil || len(c) != 0 {
		t.Errorf("blank entries: %v %v", c, err)
	}
}
