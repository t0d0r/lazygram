package command

import (
	"fmt"
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
		{"/close 2,3-5", Command{Close, "2,3-5"}, false},
		{"/wc  2, 3-5 ", Command{Close, "2, 3-5"}, false},
		{"/part 2 3 5", Command{Close, "2 3 5"}, false},
		{"/close-others", Command{CloseOthers, ""}, false},
		{"  /CLOSE-OTHERS ", Command{CloseOthers, ""}, false},
		{"/close-others 3", Command{CloseOthers, "3"}, false}, // the TUI rejects it
		{"/closeothers", Command{}, true},                     // one spelling only
		{"/close-other", Command{}, true},
		{"/wc-others", Command{}, true},
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
		{"/read-receipts on", Command{Receipts, "on"}, false},
		{"/receipts", Command{Receipts, ""}, false},
		{"/mark-as-read", Command{MarkRead, ""}, false},
		{"/autojoin off", Command{AutoJoin, "off"}, false},
		{"/autojoin", Command{AutoJoin, ""}, false},
		{"/markread", Command{}, true}, // one spelling only
		{"/feed", Command{Feed, ""}, false},
		{"/FEED off", Command{Feed, "off"}, false},
		{"/feed  on ", Command{Feed, "on"}, false},
		{"/settings", Command{Settings, ""}, false},
		{"/SETTINGS  mouse ", Command{Settings, "mouse"}, false},
		{"/settings read receipts", Command{Settings, "read receipts"}, false},
		{"/set", Command{}, true}, // one spelling only: no aliases
		{"/options", Command{}, true},
		{"/SET mouse", Command{}, true},
		{"/setting", Command{}, true},
		{"/vim", Command{Vim, ""}, false},
		{"/vim on", Command{Vim, "on"}, false},
		{"/VIM  off ", Command{Vim, "off"}, false},
		{"/vi", Command{}, true}, // one spelling only
		{"/unread", Command{Unread, ""}, false},
		{"/UNREAD ", Command{Unread, ""}, false},
		{"/unr", Command{}, true}, // one spelling only
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
// a spelling added to the table without an entry in HelpText fails here.
func TestHelpTextCoversEveryName(t *testing.T) {
	for name := range table {
		if !hasSlashWord(HelpText, name) {
			t.Errorf("/%s is accepted by the parser but missing from HelpText", name)
		}
	}
	if hasSlashWord("/win /edit", "w") || hasSlashWord("/win /edit", "e") || !hasSlashWord("also /w, /e", "e") {
		t.Fatal("hasSlashWord must match whole words only")
	}
	if !strings.HasSuffix(HelpText, "type /keys.") || strings.Contains(HelpText, "Ctrl-") {
		t.Fatal("help names /keys for the shortcuts and holds no key listing")
	}
}

// TestHelpTextIsOneLinePerEntry keeps the reference unwrapped: the window
// wraps it to its own width when drawn (specs/ui.md, /help), so a line
// wrapped here by hand would come out as a stray row of its own.
func TestHelpTextIsOneLinePerEntry(t *testing.T) {
	for _, l := range strings.Split(HelpText, "\n") {
		if p := entryProblem(l); p != "" {
			t.Errorf("%s: %q", p, l)
		}
	}
}

// entryProblem says what is wrong with one line of HelpText, or "" for a
// well-formed entry, heading or note. It checks the length before
// indexing, so a malformed line fails the test instead of panicking it.
func entryProblem(l string) string {
	if !strings.HasPrefix(l, " ") {
		return "" // a heading or a note
	}
	if strings.TrimSpace(l) == "" {
		return "a blank line holds spaces"
	}
	if !strings.HasPrefix(l, "  ") || l[2] == ' ' {
		return "an entry starts with exactly two spaces, no continuation lines"
	}
	names, rest, ok := strings.Cut(l[2:], "  ")
	desc := strings.TrimLeft(rest, " ")
	if !ok || desc == "" {
		return "no description after the names"
	}
	col := len(l) - len(desc)
	if want := max(25, 2+len(names)+2); col != want {
		return fmt.Sprintf("description at column %d, want %d", col, want)
	}
	return ""
}

// TestEntryProblem covers the malformed lines HelpText does not hold today,
// so the check above reports them rather than panicking on a short line.
func TestEntryProblem(t *testing.T) {
	ok := "  /help" + strings.Repeat(" ", 25-7) + "this list"
	for _, c := range []struct {
		line, want string
	}{
		{"", ""},
		{"Commands", ""},
		{ok, ""},
		{"  /a-very-long-command-name  its description", ""},
		{" ", "a blank line holds spaces"},
		{"  ", "a blank line holds spaces"},
		{"     ", "a blank line holds spaces"},
		{" /help  one space", "an entry starts with exactly two spaces, no continuation lines"},
		{"    wrapped continuation", "an entry starts with exactly two spaces, no continuation lines"},
		{"  /help", "no description after the names"},
		{"  /help   ", "no description after the names"},
		{"  /help  this list", "description at column 9, want 25"},
	} {
		if got := entryProblem(c.line); got != c.want {
			t.Errorf("entryProblem(%q) = %q, want %q", c.line, got, c.want)
		}
	}
}
