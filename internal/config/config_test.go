package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// goodHash is a well-formed placeholder, built so no literal looks like a secret.
var goodHash = strings.Repeat("ab", 16)

func TestAppValidate(t *testing.T) {
	if err := (App{APIID: 12345, APIHash: goodHash}).Validate(); err != nil {
		t.Fatal(err)
	}
	bad := []App{{}, {APIID: 0, APIHash: goodHash}, {APIID: 1, APIHash: ""}, {APIID: 1, APIHash: "short"}, {APIID: 1, APIHash: "has space " + goodHash}}
	for _, a := range bad {
		if a.Validate() == nil {
			t.Errorf("%+v should fail", a)
		}
	}
}

func TestParseApp(t *testing.T) {
	a, err := ParseApp(" 42 ", " "+goodHash+"\n")
	if err != nil || a.APIID != 42 || a.APIHash != goodHash {
		t.Fatalf("%+v %v", a, err)
	}
	if _, err := ParseApp("x", goodHash); err == nil {
		t.Fatal("non-numeric id")
	}
}

func TestSaveLoadApp(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tg")
	if _, found, err := LoadApp(dir); found || err != nil {
		t.Fatalf("missing: %v %v", found, err)
	}
	want := App{APIID: 7, APIHash: goodHash}
	if err := SaveApp(dir, want); err != nil {
		t.Fatal(err)
	}
	got, found, err := LoadApp(dir)
	if err != nil || !found || got != want {
		t.Fatalf("%+v %v %v", got, found, err)
	}
	st, _ := os.Stat(filepath.Join(dir, appFile))
	if st.Mode().Perm()&0o077 != 0 {
		t.Fatalf("app file must be owner-only, got %v", st.Mode())
	}
	if SaveApp(dir, App{}) == nil {
		t.Fatal("invalid app must not be saved")
	}
}

func TestLoadAppRejectsCorrupt(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, appFile), []byte("{nope"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, found, err := LoadApp(dir); !found || err == nil {
		t.Fatalf("%v %v", found, err)
	}
	if err := os.WriteFile(filepath.Join(dir, appFile), []byte(`{"api_id":0,"api_hash":""}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := LoadApp(dir); err == nil || !strings.Contains(err.Error(), appFile) {
		t.Fatalf("invalid content: %v", err)
	}
}

func TestTelegramValidate(t *testing.T) {
	if err := (Telegram{App: App{APIID: 1, APIHash: goodHash}, SessionDir: "/s"}).Validate(); err != nil {
		t.Fatal(err)
	}
	if (Telegram{App: App{APIID: 1, APIHash: goodHash}}).Validate() == nil {
		t.Fatal("session dir required")
	}
	if (Telegram{SessionDir: "/s"}).Validate() == nil {
		t.Fatal("app required")
	}
}

func TestTelegramDirUnderData(t *testing.T) {
	if TelegramDir("/d") != filepath.Join("/d", "telegram") {
		t.Fatal(TelegramDir("/d"))
	}
}

func TestAppFromEnv(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	if _, found, err := AppFromEnv(env(nil)); found || err != nil {
		t.Fatalf("unset: %v %v", found, err)
	}
	app, found, err := AppFromEnv(env(map[string]string{"TG_API_ID": " 42 ", "TG_API_HASH": goodHash}))
	if err != nil || !found || app.APIID != 42 || app.APIHash != goodHash {
		t.Fatalf("%+v %v %v", app, found, err)
	}
	app, _, err = AppFromEnv(env(map[string]string{"TG_APIID": "7", "TG_APIHASH": goodHash}))
	if err != nil || app.APIID != 7 {
		t.Fatalf("nchat names: %+v %v", app, err)
	}
	if _, found, err := AppFromEnv(env(map[string]string{"TG_API_ID": "42"})); !found || err == nil || !strings.Contains(err.Error(), "TG_API_HASH") {
		t.Fatalf("half set: %v %v", found, err)
	}
	_, _, err = AppFromEnv(env(map[string]string{"TG_API_ID": "nope", "TG_API_HASH": goodHash}))
	if err == nil || strings.Contains(err.Error(), "nope") || strings.Contains(err.Error(), goodHash) || !strings.Contains(err.Error(), "TG_API_ID") {
		t.Fatalf("invalid: %v", err)
	}
}
