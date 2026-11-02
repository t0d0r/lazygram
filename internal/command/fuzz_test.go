package command

import (
	"strings"
	"testing"
)

// FuzzParse feeds Parse whatever can be typed or pasted on the input line.
// A line that does not start with "/" (after its surrounding blanks) is
// never a command, so text is never run by mistake. Blanks around the
// line change nothing. A parsed command's Arg is trimmed and taken from
// the line, and writing the command back as "/name Arg", with any
// spelling of its kind in either letter case, parses to the same command.
func FuzzParse(f *testing.F) {
	for _, s := range []string{
		"", "/", "//", "/ ", "/ help", "\t/help\t", "/help", "  /Q ", "/KEYS extra",
		"/w  @linus ", "/msg @ada hello there", "/send ~/pic.png a caption",
		"/help\targ", "/\thelp x", "/me\u00a0hi", "/me\u3000hi", "hello", " hello /help",
		"/ме привет", "/win Ада", "/whois @Добрин", "/me 👋🏽 hi", "/me e\u0301",
		"/me \x1b[31mred\x1b[0m", "/\x1b[2Jquit", "/me \x07\x00\x7f", "/me \u0085x\u0085",
		"/me \u009b31m", "/me \xff\xfe", "/\xffhelp", "/he\xc3lp", "/İnfo", "/\u212Aeys",
		"/msg @a " + strings.Repeat("ж", 4096), "/" + strings.Repeat("a", 4096),
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, line string) {
		c, err := Parse(line)
		if !strings.HasPrefix(strings.TrimSpace(line), "/") {
			if err == nil {
				t.Fatalf("%q does not start with / but parsed as %+v", line, c)
			}
			return
		}
		again, err2 := Parse(" \t" + line + "\n ")
		if again != c || (err2 == nil) != (err == nil) {
			t.Fatalf("%q: blanks around the line changed %+v, %v to %+v, %v", line, c, err, again, err2)
		}
		if err != nil {
			if c != (Command{}) {
				t.Fatalf("%q: error %v with a command %+v", line, err, c)
			}
			return
		}
		if c.Arg != strings.TrimSpace(c.Arg) || !strings.Contains(line, c.Arg) {
			t.Fatalf("%q: Arg %q is not a trimmed part of the line", line, c.Arg)
		}
		for name, k := range table {
			if k != c.Kind {
				continue
			}
			for _, spelled := range []string{name, strings.ToUpper(name)} {
				if got, err := Parse("/" + spelled + " " + c.Arg); err != nil || got != c {
					t.Fatalf("%q = %+v, but /%s %q = %+v, %v", line, c, spelled, c.Arg, got, err)
				}
			}
		}
	})
}
