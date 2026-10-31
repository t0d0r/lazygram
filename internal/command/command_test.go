package command

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want Command
		err  bool
	}{
		{"/help", Command{Help, ""}, false},
		{"  /Q ", Command{Quit, ""}, false},
		{"/sync", Command{Refresh, ""}, false},
		{"/list dev", Command{List, "dev"}, false},
		{"/ls", Command{List, ""}, false},
		{"/win", Command{Win, ""}, false},
		{"/win 2", Command{Win, "2"}, false},
		{"/w  @linus ", Command{Win, "@linus"}, false},
		{"/join #dev", Command{Win, "#dev"}, false},
		{"/close", Command{Close, ""}, false},
		{"/close 3", Command{Close, "3"}, false},
		{"/last", Command{Last, ""}, false},
		{"/history 20", Command{Last, "20"}, false},
		{"/search", Command{Search, ""}, false},
		{"/search needle", Command{Search, "needle"}, false},
		{"/stats", Command{Stats, ""}, false},
		{"/whois @ada", Command{Whois, "@ada"}, false},
		{"/me waves", Command{Me, "waves"}, false},
		{"/press 2", Command{Press, "2"}, false},
		{"/btn 1234 1", Command{Press, "1234 1"}, false},
		{"/open", Command{Open, ""}, false},
		{"/download 42", Command{Open, "42"}, false},
		{"/theme", Command{Theme, ""}, false},
		{"/colors nord", Command{Theme, "nord"}, false},
		{"/move 3", Command{Move, "3"}, false},
		{"/msg @ada hello there", Command{Msg, "@ada hello there"}, false},
		{"/about", Command{About, ""}, false},
		{"/fx", Command{Fx, ""}, false},
		{"/mouse", Command{Mouse, ""}, false},
		{"/clip osc52", Command{Clip, "osc52"}, false},
		{"/clipboard", Command{Clip, ""}, false},
		{"/edit new text", Command{Edit, "new text"}, false},
		{"/e", Command{Edit, ""}, false},
		{"/delete", Command{Delete, ""}, false},
		{"/rm", Command{Delete, ""}, false},
		{"/names", Command{Nicks, ""}, false},
		{"/send ~/pic.png a caption", Command{SendFile, "~/pic.png a caption"}, false},
		{"/redraw", Command{Redraw, ""}, false},
		{"/clear", Command{Redraw, ""}, false},
		{"/keys", Command{Keys, ""}, false},
		{"/shortcuts", Command{Keys, ""}, false},
		{"/KEYS extra", Command{Keys, "extra"}, false},
		{"/bogus", Command{}, true},
		{"/", Command{}, true},
		{"hello", Command{}, true},
		{"", Command{}, true},
	}
	for _, c := range cases {
		got, err := Parse(c.in)
		if (err != nil) != c.err {
			t.Errorf("%q: err=%v want err=%v", c.in, err, c.err)
			continue
		}
		if !c.err && got != c.want {
			t.Errorf("%q: got %+v want %+v", c.in, got, c.want)
		}
	}
}

func TestNamesSortedWithSlash(t *testing.T) {
	n := Names()
	if len(n) != len(table) || n[0][0] != '/' {
		t.Fatal(n)
	}
	for i := 1; i < len(n); i++ {
		if n[i-1] >= n[i] {
			t.Fatal("not sorted")
		}
	}
}

// hasSlashWord reports whether text holds "/"+name as a whole word: at
// the start or after a space, and followed by a space, comma, newline
// or the end. "/win" does not count for "/w", nor "/edit" for "/e".
func hasSlashWord(text, name string) bool {
	word := "/" + name
	for i := 0; ; {
		j := strings.Index(text[i:], word)
		if j < 0 {
			return false
		}
		at := i + j
		end := at + len(word)
		before := at == 0 || text[at-1] == ' '
		after := end == len(text) || strings.ContainsRune(" ,\n", rune(text[end]))
		if before && after {
			return true
		}
		i = end
	}
}

// TestHelpTextCoversEveryName keeps /help and the parser table in step:
// a spelling added to the table without a line in HelpText fails here.
func TestHelpTextCoversEveryName(t *testing.T) {
	for name := range table {
		if !hasSlashWord(HelpText, name) {
			t.Errorf("/%s is accepted by the parser but missing from HelpText", name)
		}
	}
	if hasSlashWord("/win /edit", "w") || hasSlashWord("/win /edit", "e") || !hasSlashWord("also /w, /e", "e") {
		t.Fatal("hasSlashWord must match whole words only")
	}
	for _, l := range strings.Split(HelpText, "\n") {
		if len(l) > 70 {
			t.Errorf("help line over 70 columns (%d): %q", len(l), l)
		}
	}
	if !strings.HasSuffix(HelpText, "type /keys.") || strings.Contains(HelpText, "Ctrl-") {
		t.Fatal("help names /keys for the shortcuts and holds no key listing")
	}
}
