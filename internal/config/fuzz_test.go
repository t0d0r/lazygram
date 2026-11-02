package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// rot moves every ASCII letter to the next one of its kind (a-f, g-z,
// A-F, G-Z) and, with digits set, every digit to the next one, and leaves
// every other byte alone. Length, blanks, UTF-8, case and hex-ness stay,
// only the value changes, so an error that never prints the value reads
// the same for both.
func rot(s string, digits bool) string {
	b := []byte(s)
	for i, c := range b {
		for _, r := range []string{"abcdef", "ghijklmnopqrstuvwxyz", "ABCDEF", "GHIJKLMNOPQRSTUVWXYZ", "0123456789"} {
			if j := strings.IndexByte(r, c); j >= 0 && (digits || r[0] != '0') {
				b[i] = r[(j+1)%len(r)]
			}
		}
	}
	return string(b)
}

// rotID changes an application id without changing whether it is valid:
// letters move as in rot, digits 1-9 move to the next one of 1-9 and 0
// stays, so the syntax, the sign and whether the number is 0 stay. Only
// its value changes, not its length; when that would move it across the
// int range (9223372036854775808 would become 1334483047965886909), only
// the letters move. An id whose digits are all 0 keeps its digits.
func rotID(id string) string {
	b := []byte(rot(id, false))
	for i, c := range b {
		if '1' <= c && c <= '9' {
			b[i] = '1' + (c-'1'+1)%9
		}
	}
	fits := func(s string) bool { _, err := strconv.Atoi(strings.TrimSpace(s)); return err == nil }
	if fits(id) != fits(string(b)) {
		return rot(id, false)
	}
	return string(b)
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// appSeeds are typed or exported id and hash pairs.
var appSeeds = [][2]string{
	{"42", goodHash}, {" 42 ", " " + goodHash + "\n"}, {"x", goodHash}, {"0", goodHash},
	{"-1", goodHash}, {"+7", goodHash}, {"0042", goodHash}, {"4 2", goodHash},
	{"9223372036854775807", goodHash}, {"9223372036854775808", goodHash}, {"１２", goodHash},
	{"٤٢", goodHash}, {"0x2a", goodHash}, {"12", ""}, {"12", "short"}, {"", ""},
	{"12", "has space " + goodHash}, {"12", goodHash + "\ttab"}, {"12", goodHash + "\r"},
	{"12", "\x1b[31m" + goodHash + "\x1b[0m"}, {"12", "\u00a0" + goodHash + "\u0085"},
	{"12", goodHash[:16] + "\u00a0" + goodHash}, {"12", strings.Repeat("ж", 16)},
	{"12", strings.Repeat("🎉", 8)}, {"12", strings.Repeat("e\u0301", 16)},
	{"12", strings.Repeat("\x00", 16)}, {"12", strings.Repeat("\u009b", 16)},
	{"12", strings.Repeat("ab", 4096)},
	// Invalid UTF-8 (found by FuzzParseApp): Validate accepted it, and
	// SaveApp's JSON stored U+FFFD in its place, so LoadApp read back
	// another hash than the one validated. Validate now refuses it.
	{"12", strings.Repeat("\xff", 16)},
}

// FuzzParseApp feeds ParseApp the id and hash typed at `lazygram login`.
// Errors name what is wrong, never the value (specs/security.md), so
// changing the letters and digits of the id (rotID, which keeps it valid
// or invalid) and of the hash gives the same error. An accepted
// registration holds the id as typed and the hash trimmed, parses again
// from its own values, and is what SaveApp stores and LoadApp reads back.
func FuzzParseApp(f *testing.F) {
	for _, s := range appSeeds {
		f.Add(s[0], s[1])
	}
	dir := f.TempDir() // reused: a worker runs one input at a time
	f.Fuzz(func(t *testing.T, id, hash string) {
		a, err := ParseApp(id, hash)
		if _, err2 := ParseApp(rotID(id), rot(hash, true)); errText(err) != errText(err2) {
			t.Fatalf("the error depends on the value: %q, then %q", errText(err), errText(err2))
		}
		if err != nil {
			return
		}
		if n, err := strconv.Atoi(strings.TrimSpace(id)); err != nil || n != a.APIID || a.APIHash != strings.TrimSpace(hash) {
			t.Fatalf("ParseApp(%q, %q) = %+v", id, hash, a)
		}
		if b, err := ParseApp(strconv.Itoa(a.APIID), a.APIHash); err != nil || b != a {
			t.Fatalf("%+v parses again as %+v, %v", a, b, err)
		}
		if err := SaveApp(dir, a); err != nil {
			t.Fatalf("SaveApp(%+v): %v", a, err)
		}
		if b, found, err := LoadApp(dir); err != nil || !found || b != a {
			t.Fatalf("saved %+v, loaded %+v, %v, %v", a, b, found, err)
		}
	})
}

// FuzzAppFromEnv feeds AppFromEnv lazygram's and nchat's variables. The
// registration is found when any of them is set; lazygram's own name wins
// over nchat's; an error names a variable and never a value
// (specs/providers.md), so it reads the same when the values change as
// in FuzzParseApp.
func FuzzAppFromEnv(f *testing.F) {
	for _, s := range appSeeds {
		f.Add(s[0], "", s[1], "")
		f.Add("", s[0], "", s[1])
	}
	f.Add("42", "7", goodHash, "short")
	f.Add(" ", "7", "\t", goodHash)
	f.Add("42", "", "", "")
	f.Fuzz(func(t *testing.T, id, nchatID, hash, nchatHash string) {
		get := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
		app, found, err := AppFromEnv(get(map[string]string{"TG_API_ID": id, "TG_APIID": nchatID, "TG_API_HASH": hash, "TG_APIHASH": nchatHash}))
		_, _, err2 := AppFromEnv(get(map[string]string{"TG_API_ID": rotID(id), "TG_APIID": rotID(nchatID), "TG_API_HASH": rot(hash, true), "TG_APIHASH": rot(nchatHash, true)}))
		if errText(err) != errText(err2) {
			t.Fatalf("the error depends on the values: %q, then %q", errText(err), errText(err2))
		}
		blank := func(s string) bool { return strings.TrimSpace(s) == "" }
		if found == (blank(id) && blank(nchatID) && blank(hash) && blank(nchatHash)) {
			t.Fatalf("found = %v for %q %q %q %q", found, id, nchatID, hash, nchatHash)
		}
		if err != nil {
			if !strings.Contains(err.Error(), "TG_") {
				t.Fatalf("the error names no variable: %v", err)
			}
			return
		}
		if !found {
			if app != (App{}) {
				t.Fatalf("nothing set but %+v", app)
			}
			return
		}
		pick := func(own, nchat string) string {
			if blank(own) {
				return nchat
			}
			return own
		}
		if want, err := ParseApp(pick(id, nchatID), pick(hash, nchatHash)); err != nil || app != want {
			t.Fatalf("AppFromEnv = %+v, want %+v, %v", app, want, err)
		}
	})
}

// FuzzLoadApp feeds LoadApp an app.json as it may be found on disk,
// hand-edited or damaged. A file that is there is always found. A loaded
// registration is valid and survives SaveApp and LoadApp unchanged; a
// refused one is the zero App, and the error never quotes a number stored
// under api_id or api_hash (specs/security.md: errors never print values).
func FuzzLoadApp(f *testing.F) {
	saved, _ := json.MarshalIndent(App{APIID: 12345, APIHash: goodHash}, "", "  ")
	for _, s := range []string{
		string(saved), `{"api_id":7,"api_hash":"` + goodHash + `"}`, `{"API_ID":7,"Api_Hash":"` + goodHash + `"}`,
		`{"api_id":7,"api_hash":"` + goodHash + `","api_id":8}`, `{"api_id":"7","api_hash":"` + goodHash + `"}`,
		`{"api_id":0,"api_hash":""}`, `{"api_id":-7,"api_hash":"` + goodHash + `"}`, `{"api_id":7,"api_hash":true}`,
		`{"api_id":7,"api_hash":"\u001b[31m` + goodHash + `"}`, `{"api_id":7,"api_hash":"` + strings.Repeat("ж", 16) + `"}`,
		`{"api_id":7,"api_hash":"` + strings.Repeat("🎉", 8) + `"}`, `{"api_id":7,"api_hash":"` + strings.Repeat("e\u0301", 16) + `"}`,
		`{"api_id":7,"api_hash":"` + strings.Repeat("\xff", 16) + `"}`, `{"api_id":7,"api_hash":"\ud800` + goodHash + `"}`,
		"\xef\xbb\xbf" + string(saved), "{nope", "", "null", "[]", `""`, "{}", `{"api_id":7}`, "5", "-5.5",
		`{"api_id":[1.5]}`, `{"api_id":7,"api_hash":31415926535897932384}`,
		// A number that is no int (found by FuzzLoadApp's seeds): the json
		// error quoted it, and LoadApp passed that on. It now names the field.
		`{"api_id":1.5,"api_hash":"` + goodHash + `"}`, `{"api_id":99999999999999999999}`, `{"API_ID":1e400}`,
	} {
		f.Add([]byte(s))
	}
	dir, again := f.TempDir(), f.TempDir() // reused: a worker runs one input at a time
	f.Fuzz(func(t *testing.T, data []byte) {
		if err := os.WriteFile(filepath.Join(dir, appFile), data, 0o600); err != nil {
			t.Fatal(err)
		}
		app, found, err := LoadApp(dir)
		if !found {
			t.Fatalf("app.json is there but not found (%v)", err)
		}
		if err != nil {
			if app != (App{}) {
				t.Fatalf("refused with %v but returned %+v", err, app)
			}
			var m map[string]json.RawMessage
			if json.Unmarshal(data, &m) != nil {
				return
			}
			for k, raw := range m { // json matches the keys ignoring case
				v := string(raw)
				if (strings.EqualFold(k, "api_id") || strings.EqualFold(k, "api_hash")) && len(v) > 1 &&
					strings.ContainsAny(v[:1], "-0123456789") && strings.Contains(err.Error(), v) {
					t.Fatalf("the error prints the %s value %s: %v", k, v, err)
				}
			}
			return
		}
		if err := app.Validate(); err != nil {
			t.Fatalf("loaded an invalid %+v: %v", app, err)
		}
		if err := SaveApp(again, app); err != nil {
			t.Fatal(err)
		}
		if b, _, err := LoadApp(again); err != nil || b != app {
			t.Fatalf("loaded %+v, saved and loaded again %+v, %v", app, b, err)
		}
	})
}
