package opener

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheck(t *testing.T) {
	ok := filepath.Join(t.TempDir(), "img.png")
	if err := os.WriteFile(ok, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, good := range []string{"https://example.invalid/a.png", "http://h/x", ok} {
		if err := Check(good); err != nil {
			t.Errorf("%q: %v", good, err)
		}
	}
	for _, bad := range []string{"javascript:alert(1)", "file:///etc/passwd", "ftp://h/x", "relative/path", "-flag", t.TempDir(), "https://", "mailto:x@y"} {
		if err := Check(bad); err == nil {
			t.Errorf("%q should be refused", bad)
		}
	}
}

func TestCommand(t *testing.T) {
	if n, a := command("darwin", "https://x"); n != "open" || a[0] != "https://x" {
		t.Fatal(n, a)
	}
	if n, _ := command("linux", "https://x"); n != "xdg-open" {
		t.Fatal(n)
	}
	if n, a := command("windows", "https://x"); n != "rundll32" || a[1] != "https://x" {
		t.Fatal(n, a)
	}
}
