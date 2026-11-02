package tui

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/rivo/uniseg"

	"lazygram/internal/domain"
)

// fuzzTexts seed every target here: the text other Telegram users send,
// and text meant to break a terminal or a layout.
var fuzzTexts = []string{
	"",
	" ",
	"hello world",
	"\u041f\u0440\u0438\u0432\u0435\u0442, \u043c\u0438\u0440! \u041a\u0430\u043a \u0434\u0435\u043b\u0430?", // Cyrillic
	"\U0001F600 \u2764\ufe0f \U0001F468\u200d\U0001F469\u200d\U0001F467 \U0001F1E7\U0001F1EC",                // emoji: plain, VS16, ZWJ family, flag
	"e\u0301 a\u0308\u0304 \u0301lone \u0915\u094d\u0937",                                                    // combining marks, one with no base, a conjunct
	"\u65e5\u672c\u8a9e\u306e\u30c6\u30ad\u30b9\u30c8 \ud55c\uad6d\uc5b4",                                    // wide CJK and Hangul
	"\x1b[31mred\x1b[0m \x1b]0;title\x07 \x1b]8;;https://evil.example\x1b\\x\x1b]8;;\x1b\\",                  // ANSI: SGR, OSC title, OSC 8 link
	"\x1b]52;c;cHduZWQ=\x07 \x1bP+q\x1b\\ \x1b[2J\x1b[H",                                                     // OSC 52 clipboard, DCS, clear screen
	"bell\a nul\x00 del\x7f cr\r tab\t vt\v ff\f",                                                            // C0 and DEL
	"\u0085nel \u009b31mcsi \u009d0;t\u009c",                                                                 // C1 as code points
	"\x9b31m \xff\xfe \xc3 \xe2\x82 \xed\xa0\x80",                                                            // invalid UTF-8: raw C1, stray bytes, a surrogate
	"a\nb\n\n\nc\n",
	"  /win N    switch to window N",
	"see https://example.org/a_(b)). and http://x.example/?q=1&r=2, ok",
	"\u202eevil\u202c \u2028line\u2029para \u00a0nbsp \u3000ideo", // bidi override, line and paragraph separators, Unicode spaces
	strings.Repeat("word ", 400),
	strings.Repeat("x", 3000),
	strings.Repeat("\u2764\ufe0f", 40),
	"https://" + strings.Repeat("a", 500) + ".example/" + strings.Repeat("\u0436", 200),
}

// hasControl reports whether s holds a control character other than a
// newline when nl is allowed.
func hasControl(s string, nl bool) bool {
	return strings.IndexFunc(s, func(r rune) bool { return unicode.IsControl(r) && (r != '\n' || !nl) }) >= 0
}

// FuzzSanitize: remote text cannot carry a control character to the
// terminal. The result is valid UTF-8 with no C0, DEL or C1 character (a
// newline only when keepNL), so no tab either; newlines survive as they
// were; the two modes differ only in newlines; sanitizing twice changes
// nothing; and text with no control character passes unchanged.
func FuzzSanitize(f *testing.F) {
	for _, s := range fuzzTexts {
		f.Add(s, true)
		f.Add(s, false)
	}
	f.Fuzz(func(t *testing.T, s string, keepNL bool) {
		out := sanitize(s, keepNL)
		if !utf8.ValidString(out) {
			t.Fatalf("%q (keepNL %t): invalid UTF-8 out %q", s, keepNL, out)
		}
		if hasControl(out, keepNL) {
			t.Fatalf("%q (keepNL %t): a control character survived in %q", s, keepNL, out)
		}
		if keepNL && strings.Count(out, "\n") != strings.Count(s, "\n") {
			t.Fatalf("%q: newlines not kept: %q", s, out)
		}
		if one, two := sanitize(s, false), strings.ReplaceAll(sanitize(s, true), "\n", ""); one != two {
			t.Fatalf("%q: the modes differ beyond newlines: %q vs %q", s, one, two)
		}
		if again := sanitize(out, keepNL); again != out {
			t.Fatalf("%q (keepNL %t): not idempotent: %q then %q", s, keepNL, out, again)
		}
		if utf8.ValidString(s) && !hasControl(s, keepNL) && out != s {
			t.Fatalf("%q (keepNL %t): clean text changed to %q", s, keepNL, out)
		}
	})
}

// FuzzSanitizeName: whatever name an attachment claims, its copy lands
// directly in the private directory: one valid UTF-8 path element with
// no control character, no separator and no leading dot (so neither "."
// nor ".." nor a hidden file), taken from the name or else "attachment",
// and sanitizing it again changes nothing.
func FuzzSanitizeName(f *testing.F) {
	for _, s := range []string{".", "..", "...", "/", "//", "\\", "../../etc/passwd", "a/../../b", ".hidden", "..\\..\\x",
		"dir/", "x/.", "/tmp/x\x00.png", "\x1b[2J.jpg", "-rf.jpg", "~", ".\n./x", "\u2024\u2024/x"} {
		f.Add(s)
	}
	for _, s := range fuzzTexts {
		f.Add(s)
	}
	dir := filepath.Clean(f.TempDir()) // Join cleans, so compare with a clean path whatever TMPDIR is
	f.Fuzz(func(t *testing.T, name string) {
		out := sanitizeName(name)
		switch {
		case out == "":
			t.Fatalf("%q: empty name", name)
		case !utf8.ValidString(out):
			t.Fatalf("%q: invalid UTF-8 %q", name, out)
		case hasControl(out, false):
			t.Fatalf("%q: control character in %q", name, out)
		case strings.ContainsRune(out, '/') || strings.ContainsRune(out, filepath.Separator):
			t.Fatalf("%q: separator in %q", name, out)
		case strings.HasPrefix(out, "."):
			t.Fatalf("%q: leading dot in %q", name, out)
		}
		for _, elem := range []string{out, fmt.Sprintf("%d-%s", 1, out)} { // viewCopy numbers the copy
			if p := filepath.Join(dir, elem); filepath.Dir(p) != dir || filepath.Base(p) != elem {
				t.Fatalf("%q: %q is not one element of the directory: %q", name, elem, p)
			}
		}
		if out != "attachment" && !strings.Contains(sanitize(name, false), out) {
			t.Fatalf("%q: %q is not taken from the name", name, out)
		}
		if again := sanitizeName(out); again != out {
			t.Fatalf("%q: not idempotent: %q then %q", name, out, again)
		}
	})
}

// nonSpace is s without its white space: what wrapping must keep.
func nonSpace(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

// wordClusters lists the grapheme clusters of the words of the lines,
// split at white space as wrapText splits them and each word segmented on
// its own. A line break inside a cluster leaves its two pieces in
// different words, so the list changes.
func wordClusters(lines ...string) []string {
	var out []string
	for _, l := range lines {
		for _, w := range strings.Fields(l) {
			for rest, state := w, -1; rest != ""; {
				var c string
				c, rest, _, state = uniseg.FirstGraphemeClusterInString(rest, state)
				out = append(out, c)
			}
		}
	}
	return out
}

// FuzzWrapText: message text from another user, sanitized as the
// renderer does, wraps to at least one line; the first is no wider than
// first and every later one no wider than width, measured with
// lipgloss.Width as the renderer measures; no line holds a newline or is
// blank (unless the text is); every line is valid UTF-8; nothing but
// white space is lost or reordered; no line ends inside a character (a
// grapheme cluster); and text that fits on one line keeps its spacing.
func FuzzWrapText(f *testing.F) {
	for _, s := range fuzzTexts {
		f.Add(s, uint8(0), uint8(0))
		f.Add(s, uint8(5), uint8(30))
	}
	// Found: splitAt counted rune by rune, so eight hearts with VS16 (16
	// columns) were one 10-column piece.
	f.Add(strings.Repeat("\u2764\ufe0f", 8), uint8(0), uint8(0))
	// Nine ZWJ families, one character in two columns each. Counted rune
	// by rune a family measured six, so at 10 columns splitAt cut the
	// second family after its second person, which the cluster check
	// catches. At first 13, width 14 that cut fell between families, so
	// the second seed only covers a hard break between ZWJ sequences.
	families := strings.Repeat("\U0001F468\u200d\U0001F469\u200d\U0001F467", 9)
	f.Add(families, uint8(0), uint8(0))
	f.Add(families, uint8(3), uint8(4))
	f.Fuzz(func(t *testing.T, raw string, a, b uint8) {
		// The renderer passes widths of at least 10 and a first line no
		// wider than the rest (the sender prefix is wider than the indent).
		width := 10 + int(b)
		first := 10 + int(a)%(width-9)
		text := sanitize(raw, true)
		lines := wrapText(text, first, width)
		if len(lines) == 0 {
			t.Fatalf("%q: no line", text)
		}
		for k, l := range lines {
			limit := width
			if k == 0 {
				limit = first
			}
			if w := lipgloss.Width(l); w > limit {
				t.Fatalf("%q (first %d, width %d): line %d is %d wide: %q", text, first, width, k, w, l)
			}
			if !utf8.ValidString(l) || strings.Contains(l, "\n") {
				t.Fatalf("%q: line %d is not one valid line: %q", text, k, l)
			}
			if strings.TrimSpace(l) == "" && nonSpace(text) != "" {
				t.Fatalf("%q: line %d is blank: %q", text, k, lines)
			}
		}
		if got, want := nonSpace(strings.Join(lines, "")), nonSpace(text); got != want {
			t.Fatalf("%q (first %d, width %d): text lost or reordered: %q", text, first, width, lines)
		}
		if got, want := wordClusters(lines...), wordClusters(text); !slices.Equal(got, want) {
			t.Fatalf("%q (first %d, width %d): a line break splits a character: %q", text, first, width, lines)
		}
		if !strings.Contains(text, "\n") && strings.TrimSpace(text) != "" && lipgloss.Width(text) <= first {
			if want := strings.TrimRight(text, " "); len(lines) != 1 || lines[0] != want {
				t.Fatalf("%q fits in %d but became %q", text, first, lines)
			}
		}
	})
}

// FuzzRefRows: any line laid out as /help and /keys lay out theirs,
// sanitized as the renderer does, in a window of at least 10 columns,
// gives at least one row; no row is wider than the window or holds a
// newline; and nothing but white space is lost or reordered.
func FuzzRefRows(f *testing.F) {
	for i, s := range fuzzTexts {
		f.Add(s, uint8(i*7))
		f.Add("  /x                     "+s, uint8(i*11))
		f.Add("  "+s+"  the description", uint8(i*13))
	}
	f.Add("  /msg <@user|#chan> <text>  send to that chat, opening its window; also /say, /tell", uint8(35))
	f.Fuzz(func(t *testing.T, raw string, w uint8) {
		width := 10 + int(w)
		text := sanitize(raw, true)
		rows := refRows(text, width)
		if len(rows) == 0 {
			t.Fatalf("%q: no row", text)
		}
		for k, r := range rows {
			if lw := lipgloss.Width(r); lw > width || strings.Contains(r, "\n") {
				t.Fatalf("%q in %d columns: row %d is %d wide: %q", text, width, k, lw, rows)
			}
		}
		if got, want := nonSpace(strings.Join(rows, "")), nonSpace(text); got != want {
			t.Fatalf("%q in %d columns: text lost or reordered: %q", text, width, rows)
		}
	})
}

// FuzzFitLine: a bar is exactly width columns whatever its text, made of
// a prefix of that text and padding spaces.
func FuzzFitLine(f *testing.F) {
	for _, s := range fuzzTexts {
		f.Add(s, uint8(0))
		f.Add(s, uint8(1))
		f.Add(s, uint8(80))
	}
	f.Fuzz(func(t *testing.T, raw string, w uint8) {
		s := sanitize(raw, false) // bar text is one line of sanitized text
		out := fitLine(s, int(w))
		if got := lipgloss.Width(out); got != int(w) {
			t.Fatalf("%q: fitLine to %d is %d wide: %q", s, w, got, out)
		}
		if kept := strings.TrimRight(out, " "); !strings.HasPrefix(s, kept) {
			t.Fatalf("%q: fitLine to %d is no prefix plus padding: %q", s, w, out)
		}
	})
}

// Where a link ends (specs/ui.md: a link ends as a double-clicked word
// does). linkCut is every character that may be cut from its end.
const (
	linkStop  = ".,;:!?" // sentence punctuation
	linkOpen  = "([{"
	linkClose = ")]}"
	linkCut   = linkStop + linkClose
)

// linkMayEnd reports whether a link may end in its last character: never
// sentence punctuation, and a closing bracket only when the link opens it
// at least as often as it closes it, so Go_(game) keeps its bracket.
func linkMayEnd(u string) bool {
	c := u[len(u)-1:]
	if strings.Contains(linkStop, c) {
		return false
	}
	if k := strings.Index(linkClose, c); k >= 0 {
		return strings.Count(u, linkOpen[k:k+1]) >= strings.Count(u, c)
	}
	return true
}

// runFrom is s from i up to the next white space, quote or angle
// bracket: as far as a link that reaches i could run.
func runFrom(s string, i int) string {
	if k := strings.IndexFunc(s[i:], func(r rune) bool { return unicode.IsSpace(r) || strings.ContainsRune("<>\"'`", r) }); k >= 0 {
		return s[i : i+k]
	}
	return s[i:]
}

// FuzzURLsOf: every link found is an http(s) link with more than its
// scheme that occurs in the text, in text order and without overlap; it
// holds no white space, quote, backtick or angle bracket; it ends as
// specs/ui.md ends a word (linkMayEnd), and what follows it up to the next
// blank is only characters that were cut, with no longer end it may end in;
// it is found again, whole, in itself; and every "http://" or "https://"
// in the text starts a link or lies in one, unless only such characters
// follow it. A link taken from sanitized text, as Tab offers after /open,
// has no control character and, when it is a web link, is a single word,
// so "/open <link>" takes it as a click would.
func FuzzURLsOf(f *testing.F) {
	for _, s := range []string{"https://", "http://.", "http://x.", "(https://example.org/a_(b)).", "<https://x.example>",
		"'https://x.example/q'", "https://x.example/\"q\"", "http://a http://b", "https://a.example/http://b.example",
		"HTTPS://UPPER.example", "https://xn--e1afmkfd.xn--p1ai/\u043f\u0443\u0442\u044c", "\u00a0see https://example.org/x",
		"https://example.org/\x1b]8;;x\x07", "https://example.org/\xff\xfe", "https://en.wikipedia.org/wiki/Go_(game)",
		"[https://x.example/a] {https://x.example/{b}}, https://x.example/(c]).", "https://x/)(a)", "xhttps://x.example"} {
		f.Add(s)
	}
	// Found: a link ran on over a no-break space (urlRE stopped only at
	// ASCII blanks), so Tab offered "http://\u00a0", which /open splits
	// and refuses; French puts one before "!", so a link took the space.
	f.Add("http://\u00a0")
	// Found: a scheme followed only by punctuation was cut to a bare
	// "https://" and then, with one character kept, to "https://!", which
	// the opener takes; it is no link at all.
	f.Add("https://!")
	f.Add("https://...")
	f.Add("http://)")
	f.Add("voir https://exemple.fr\u00a0! merci\u202fbien \u3000https://example.org/x\u2028y")
	for _, s := range fuzzTexts {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		locs := urlLocs(s)
		end := 0
		for _, loc := range locs {
			if loc[0] < end || loc[1] <= loc[0] {
				t.Fatalf("%q: links out of order or overlapping: %v", s, locs)
			}
			end = loc[1]
			u := s[loc[0]:loc[1]]
			if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
				t.Fatalf("%q: %q is no http(s) link", s, u)
			}
			if u == "http://" || u == "https://" {
				t.Fatalf("%q: %q is nothing but a scheme", s, u)
			}
			if strings.IndexFunc(u, unicode.IsSpace) >= 0 || strings.ContainsAny(u, "<>\"'`") {
				t.Fatalf("%q: %q holds a blank, quote, backtick or angle bracket", s, u)
			}
			if !linkMayEnd(u) {
				t.Fatalf("%q: %q ends in punctuation or a bracket it does not open", s, u)
			}
			cut := runFrom(s, loc[1])
			if strings.Trim(cut, linkCut) != "" {
				t.Fatalf("%q: %q is cut short of %q", s, u, cut)
			}
			for k := 1; k <= len(cut); k++ { // the longest end that may end a link is kept
				if linkMayEnd(u + cut[:k]) {
					t.Fatalf("%q: %q is cut short of %q", s, u, cut[:k])
				}
			}
			if again := urlsOf(u); len(again) != 1 || again[0] != u {
				t.Fatalf("%q: %q is found in itself as %q", s, u, again)
			}
		}
		for p := 0; p < len(s); p++ {
			scheme := ""
			for _, sc := range []string{"http://", "https://"} {
				if strings.HasPrefix(s[p:], sc) {
					scheme = sc
				}
			}
			if scheme == "" || strings.Trim(runFrom(s, p+len(scheme)), linkCut) == "" {
				continue
			}
			if !slices.ContainsFunc(locs, func(loc []int) bool { return p >= loc[0] && p < loc[1]+len(runFrom(s, loc[1])) }) {
				t.Fatalf("%q: the link at %d is missed: %v", s, p, locs)
			}
		}
		for _, u := range urlsOf(sanitize(s, true)) {
			if hasControl(u, false) {
				t.Fatalf("%q: control character in offered link %q", s, u)
			}
			if f := strings.Fields(u); isWebLink(u) && (len(f) != 1 || f[0] != u) {
				t.Fatalf("%q: Tab offers %q but /open reads it as %q", s, u, f)
			}
		}
	})
}

// FuzzHighlight: the search hits in a line are where it holds the needle,
// both lower-cased rune by rune: as many as strings.Count finds in the
// lower-cased strings, left to right without overlap, each one
// lower-casing to the needle and, in valid text, cut at character
// boundaries; and highlighting a sanitized line keeps its text.
func FuzzHighlight(f *testing.F) {
	for _, s := range fuzzTexts {
		f.Add(s, "e")
		f.Add(s, "\u0416") // Cyrillic capital zhe
	}
	// Regression: highlight sliced the line at offsets into its lower-cased
	// copy. U+0130 lower-cases a byte shorter and U+023A a byte longer, so
	// the lengths agreed but the hit on "abc" started a byte early, inside
	// the e-acute.
	f.Add("\u0130\u00e9abc\u023a", "abc")
	f.Add("\u212a k K", "\u212a") // the Kelvin sign lower-cases to k
	f.Fuzz(func(t *testing.T, s, needle string) {
		hits := hitSpans(s, needle)
		want := 0
		if needle != "" {
			want = strings.Count(strings.ToLower(s), strings.ToLower(needle))
		}
		if len(hits) != want {
			t.Fatalf("%q in %q: %d hits %v, want %d", needle, s, len(hits), hits, want)
		}
		end := 0
		lowNeedle := strings.ToLower(needle) // ToLower, not EqualFold: the same folding as the search
		for _, h := range hits {
			if h[0] < end || h[1] <= h[0] || h[1] > len(s) {
				t.Fatalf("%q in %q: hits out of order, overlapping or out of the line: %v", needle, s, hits)
			}
			end = h[1]
			if got := s[h[0]:h[1]]; strings.ToLower(got) != lowNeedle {
				t.Fatalf("%q in %q: hit %v is %q", needle, s, h, got)
			}
			if utf8.ValidString(s) && (!utf8.ValidString(s[:h[0]]) || !utf8.ValidString(s[h[0]:h[1]])) {
				t.Fatalf("%q in %q: hit %v cuts a character", needle, s, h)
			}
		}
	})
}

// sgrRE matches the colour and attribute sequences lipgloss writes; any
// other escape in a rendered line came from the message.
var sgrRE = regexp.MustCompile("\x1b\\[[0-9;]*m")

// FuzzRenderMessage: a message from another user, whatever its sender,
// text, attachment name and button label, renders to lines that hold no
// control character beyond lipgloss's own colour sequences and are valid
// UTF-8; every line after the first stays within the window; and a link
// a click would open occurs in the sanitized text and lies on its line.
func FuzzRenderMessage(f *testing.F) {
	for i, s := range fuzzTexts {
		f.Add(fuzzTexts[(i+3)%len(fuzzTexts)], s, fuzzTexts[(i+5)%len(fuzzTexts)], fuzzTexts[(i+7)%len(fuzzTexts)], uint8(i*13), uint8(i*37))
	}
	f.Add("ann", "* waves at https://example.org/a", "x.png", "OK", uint8(0), uint8(0xff))
	kinds := []string{"photo", "image", "file", "media"}
	f.Fuzz(func(t *testing.T, sender, text, name, label string, w, flags uint8) {
		m := domain.Message{ID: "42", ChatID: "c1", Sender: sender, Text: text, Time: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}
		if flags&1 != 0 {
			m.Media = &domain.Media{Kind: kinds[int(flags>>1)%len(kinds)], Name: name}
		}
		if flags&8 != 0 {
			m.Buttons = []domain.Button{{Label: label, Kind: domain.ButtonKind(int(flags>>4) % 4)}, {Label: name}}
		}
		// At least 30 columns, so the continuation width (the window less
		// the time column, at most 17 wide) is never raised to the floor
		// of 10.
		o := lineOpts{width: 30 + int(w), showChat: flags&0x40 != 0, nameOf: nameX, nameW: 6, fullDate: flags&0x80 != 0, phase: -1}
		var lines []string
		var spans [][]span
		renderMessage(m, o, func(l string, sp ...span) {
			lines = append(lines, l)
			spans = append(spans, sp)
		})
		if len(lines) == 0 {
			t.Fatal("no line")
		}
		plain := sanitize(text, true)
		for k, l := range lines {
			bare := sgrRE.ReplaceAllString(l, "")
			if !utf8.ValidString(bare) || hasControl(bare, false) {
				t.Fatalf("sender %q text %q: line %d carries a control character: %q", sender, text, k, l)
			}
			lw := lipgloss.Width(bare)
			if k > 0 && lw > o.width {
				t.Fatalf("text %q: line %d is %d wide in %d columns: %q", text, k, lw, o.width, bare)
			}
			for _, sp := range spans[k] {
				if sp.x0 < 0 || sp.x1 <= sp.x0 {
					t.Fatalf("text %q: line %d has an empty span %+v", text, k, sp)
				}
				if sp.kind == spanURL && (sp.x1 > lw || !strings.Contains(plain, sp.target)) {
					t.Fatalf("text %q: line %d (%d wide) links %+v", text, k, lw, sp)
				}
			}
		}
	})
}
