package tui

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"lazygram/internal/command"
	"lazygram/internal/domain"
)

func nameX(string) string { return "#dev" }

func TestWrapText(t *testing.T) {
	cases := []struct {
		in   string
		w    int
		want []string
	}{
		{"hello world", 20, []string{"hello world"}},
		{"hello world", 5, []string{"hello", "world"}},
		{"abcdefghij", 4, []string{"abcd", "efgh", "ij"}},
		{"a\nb", 10, []string{"a", "b"}},
		{"a\n\n\nb\n", 10, []string{"a", "b"}},
		{"", 10, []string{""}},
		{"  /win N    switch", 40, []string{"  /win N    switch"}},
	}
	for _, c := range cases {
		got := wrapText(c.in, c.w, c.w)
		if strings.Join(got, "|") != strings.Join(c.want, "|") {
			t.Errorf("wrap(%q,%d)=%q want %q", c.in, c.w, got, c.want)
		}
	}
	got := wrapText("one two three four five six", 9, 20)
	if strings.Join(got, "|") != "one two|three four five six" {
		t.Fatalf("first line narrower than the rest: %q", got)
	}
	got = wrapText("  /x   a long description that wraps", 20, 20)
	if len(got) != 3 || got[0] != "  /x a long" || got[1] != "  description that" || got[2] != "  wraps" {
		t.Fatalf("indent must be kept on wrapped pieces: %q", got)
	}
}

// entryRE splits a reference entry as specs/ui.md describes it: two
// spaces, the names (single spaces inside), two or more spaces, then the
// description. Kept apart from refEntry so the test does not grade the
// code by itself.
var entryRE = regexp.MustCompile(`^(  \S(?:.*?\S)?  +)(\S.*)$`)

// refRowsAt draws one line of a reference, as /help and /keys print it,
// in a window width columns wide, without colour.
func refRowsAt(l string, width int) []string {
	var rows []string
	for _, r := range renderLines(nil, []note{{at: time.Now(), text: l, plain: true}}, lineOpts{width: width}) {
		rows = append(rows, ansi.Strip(r))
	}
	return rows
}

// TestReferenceFollowsTheWidth: /help and /keys fill the window's width
// instead of a fixed 70 columns. An entry's description wraps under
// itself (a hanging indent) when there is room for the columns, and under
// the names, indented 4, when there is not; a note is a wrapped paragraph.
// No row is wider than the window and no word is lost or moved.
func TestReferenceFollowsTheWidth(t *testing.T) {
	for _, ref := range []string{command.HelpText, keyHelp} {
		for _, width := range []int{40, 80, 160} {
			wider := false
			for _, l := range strings.Split(ref, "\n") {
				rows := refRowsAt(l, width)
				for _, r := range rows {
					if lipgloss.Width(r) > width {
						t.Errorf("%d columns: row %d wide: %q", width, lipgloss.Width(r), r)
					}
					wider = wider || lipgloss.Width(r) > 100
				}
				if got := strings.Fields(strings.Join(rows, " ")); strings.Join(got, " ") != strings.Join(strings.Fields(l), " ") {
					t.Errorf("%d columns: words lost or moved in %q:\n%s", width, l, strings.Join(rows, "\n"))
				}
				sub := entryRE.FindStringSubmatch(l)
				if sub == nil {
					if strings.HasPrefix(l, " ") {
						t.Errorf("a reference line is an entry or a note, not a continuation: %q", l)
					}
					continue
				}
				lead, hang := sub[1], len(sub[1])
				switch width {
				case 40: // 15 columns beside the names: the description goes under them
					if rows[0] != strings.TrimRight(lead, " ") {
						t.Errorf("40 columns: the names alone on the first row: %q", rows[0])
					}
					for _, r := range rows[1:] {
						if !strings.HasPrefix(r, "    ") || r[4] == ' ' {
							t.Errorf("40 columns: the description indented 4 under %q: %q", lead, r)
						}
					}
				default:
					if !strings.HasPrefix(rows[0], lead) || rows[0] == lead {
						t.Errorf("%d columns: the first row holds the names and the description: %q", width, rows[0])
					}
					for _, r := range rows[1:] {
						if len(r) <= hang || strings.TrimLeft(r, " ") != r[hang:] {
							t.Errorf("%d columns: continuation of %q not at column %d: %q", width, lead, hang, r)
						}
					}
				}
			}
			if width == 160 && !wider {
				t.Errorf("160 columns: no row passes 100 columns, the reference ignores the width")
			}
		}
	}
	if rows := refRowsAt("  /win N                 switch to window N (0 is the server window)", 80); len(rows) != 1 || rows[0] != "  /win N                 switch to window N (0 is the server window)" {
		t.Fatalf("an entry that fits keeps its columns on one row: %q", rows)
	}
	long := "  /send <path> [caption]  send a local file; pictures go as photos (Tab completes the path); also /file, /upload"
	if rows := refRowsAt(long, 160); len(rows) != 1 || rows[0] != long {
		t.Fatalf("160 columns hold the whole entry: %q", rows)
	}
	want := []string{
		"  /send <path> [caption]  send a local file; pictures go as photos (Tab",
		"                          completes the path); also /file, /upload",
	}
	if rows := refRowsAt(long, 72); strings.Join(rows, "\n") != strings.Join(want, "\n") {
		t.Fatalf("hanging indent at the description column:\n%s", strings.Join(rows, "\n"))
	}
	want = []string{
		"  /send <path> [caption]",
		"    send a local file; pictures go as",
		"    photos (Tab completes the path);",
		"    also /file, /upload",
	}
	if rows := refRowsAt(long, 40); strings.Join(rows, "\n") != strings.Join(want, "\n") {
		t.Fatalf("too narrow for the columns: the description under the names:\n%s", strings.Join(rows, "\n"))
	}
	if rows := refRowsAt("Window 0 is the server window: every chat interleaved, plus notices; it stays at 0.", 40); len(rows) != 3 || rows[1] != "chat interleaved, plus notices; it stays" {
		t.Fatalf("a note is a paragraph wrapped to the width: %q", rows)
	}
}

func TestWindow(t *testing.T) {
	lines := []string{"1", "2", "3", "4", "5"}
	v, off := window(lines, 3, 0)
	if strings.Join(v, "") != "345" || off != 0 {
		t.Fatalf("bottom: %q %d", v, off)
	}
	v, off = window(lines, 3, 1)
	if strings.Join(v, "") != "234" || off != 1 {
		t.Fatalf("up1: %q %d", v, off)
	}
	v, off = window(lines, 3, 99)
	if strings.Join(v, "") != "123" || off != 2 {
		t.Fatalf("clamped: %q %d", v, off)
	}
	v, _ = window(lines[:2], 4, 0)
	if len(v) != 4 || v[0] != "" || v[3] != "2" {
		t.Fatalf("pad: %q", v)
	}
}

func TestRenderLinesServerWindowShowsTimeChatSenderText(t *testing.T) {
	ts := time.Date(2026, 5, 6, 14, 7, 0, 0, time.Local)
	msgs := []domain.Message{{ID: "1", ChatID: "c1", Sender: "ada", Text: "hello there", Time: ts}}
	lines := renderLines(msgs, nil, lineOpts{width: 80, nameW: 4, nameOf: nameX, showChat: true})
	if len(lines) != 2 || !strings.Contains(lines[0], "2026-05-06") {
		t.Fatalf("want day separator then message: %q", lines)
	}
	for _, want := range []string{"14:07", "#dev", "<ada>", "hello there"} {
		if !strings.Contains(lines[1], want) {
			t.Errorf("line %q lacks %q", lines[1], want)
		}
	}
	chatOnly := renderLines(msgs, nil, lineOpts{width: 80, nameW: 4, nameOf: nameX})
	if strings.Contains(chatOnly[1], "#dev") || !strings.Contains(chatOnly[1], "<ada>") {
		t.Fatalf("chat window must omit the chat column: %q", chatOnly[1])
	}
}

func TestRenderInterleavesNotesByTime(t *testing.T) {
	t0 := time.Date(2026, 5, 6, 14, 0, 0, 0, time.Local)
	msgs := []domain.Message{
		{ID: "1", ChatID: "c", Sender: "a", Text: "first", Time: t0},
		{ID: "2", ChatID: "c", Sender: "a", Text: "third", Time: t0.Add(2 * time.Minute)},
	}
	notes := []note{{at: t0.Add(time.Minute), text: "second"}, {at: t0.Add(3 * time.Minute), text: "fourth"}}
	out := strings.Join(renderLines(msgs, notes, lineOpts{width: 80, nameW: 4, nameOf: nameX}), "\n")
	a, b, c, d := strings.Index(out, "first"), strings.Index(out, "-!- second"), strings.Index(out, "third"), strings.Index(out, "-!- fourth")
	if a < 0 || b < 0 || c < 0 || d < 0 || a >= b || b >= c || c >= d {
		t.Fatalf("order wrong:\n%s", out)
	}
}

func TestRenderWrapsPastTheTimeColumn(t *testing.T) {
	msgs := []domain.Message{{ID: "1", ChatID: "c", Sender: "s", Text: strings.Repeat("word ", 20) + "\n\nsecond paragraph", Time: time.Now()}}
	lines := renderLines(msgs, nil, lineOpts{width: 40, nameW: 3, nameOf: nameX})
	if len(lines) < 4 {
		t.Fatalf("expected wrapping, got %d lines", len(lines))
	}
	for _, l := range lines[2:] {
		if !strings.HasPrefix(l, "      ") || strings.HasPrefix(l, "       ") {
			t.Fatalf("continuation should sit just past the time column: %q", l)
		}
		if strings.TrimSpace(l) == "" {
			t.Fatal("blank lines inside a message must be dropped")
		}
	}
}

func TestHighlight(t *testing.T) {
	if got := highlight("plain", ""); got != "plain" {
		t.Fatal(got)
	}
	if got := highlight("no match", "zzz"); got != "no match" {
		t.Fatal(got)
	}
	if got := highlight("Find FIND", "find"); !strings.Contains(got, "Find") || !strings.Contains(got, "FIND") {
		t.Fatal(got)
	}
}

// The hits are byte ranges of the line itself, found rune by rune, also
// where lower-casing changes a letter's byte length (U+0130 and U+212A
// shrink, U+023A grows), so a hit never starts inside a character.
func TestHitSpans(t *testing.T) {
	cases := []struct {
		s, needle string
		want      [][2]int
	}{
		{"Find FIND find", "find", [][2]int{{0, 4}, {5, 9}, {10, 14}}},
		{"aaa", "aa", [][2]int{{0, 2}}}, // no overlap
		{"abc", "", nil},
		{"abc", "x", nil},
		{"\u0130\u00e9abc\u023a", "abc", [][2]int{{4, 7}}},
		{"\u212a k", "K", [][2]int{{0, 3}, {4, 5}}},
		{"x\u0130", "i", [][2]int{{1, 3}}},
		{"\u023a", "\u2c65", [][2]int{{0, 2}}},
	}
	for _, c := range cases {
		got := hitSpans(c.s, c.needle)
		if len(got) != len(c.want) {
			t.Fatalf("%q in %q: %v, want %v", c.needle, c.s, got, c.want)
		}
		for k := range got {
			if got[k] != c.want[k] {
				t.Fatalf("%q in %q: %v, want %v", c.needle, c.s, got, c.want)
			}
		}
	}
}

// A link ends as a double-clicked word does (specs/ui.md): sentence
// punctuation and a closing bracket the link does not open are not part
// of it, a bracket it opens is, so a click opens exactly what a double
// click on it copies; a scheme with nothing but punctuation after it is
// no link.
func TestLinksEndAsWordsDo(t *testing.T) {
	for _, c := range []struct{ row, link string }{
		{"see (https://example.invalid/a_(b)). ok", "https://example.invalid/a_(b)"},
		{"read https://en.wikipedia.org/wiki/Go_(game), then", "https://en.wikipedia.org/wiki/Go_(game)"},
		{"[https://example.invalid/x]: done", "https://example.invalid/x"},
		{"{https://example.invalid/y}!", "https://example.invalid/y"},
		{"https://example.invalid/(z)?!", "https://example.invalid/(z)"},
	} {
		got := urlsOf(c.row)
		if len(got) != 1 || got[0] != c.link {
			t.Fatalf("%q: links %q, want %q", c.row, got, c.link)
		}
		sp := linkSpans(c.row, 0, got)
		if len(sp) != 1 || sp[0].target != c.link {
			t.Fatalf("%q: a click opens %+v, want %q", c.row, sp, c.link)
		}
		from, to, ok := wordAt(c.row, sp[0].x0+1)
		if !ok || c.row[from:to+1] != c.link { // ASCII rows: a column is a byte
			t.Fatalf("%q: a double click copies %q, a click opens %q", c.row, c.row[from:to+1], c.link)
		}
	}
	if got := urlsOf("wow https://! and http://... or (https://)"); len(got) != 0 {
		t.Fatalf("a bare scheme is no link: %q", got)
	}
}

func TestRenderStripsControlCharacters(t *testing.T) {
	msgs := []domain.Message{{ID: "1", ChatID: "c", Sender: "a\x1b[31m", Text: "hi\x1b]0;pwn\x07 there\tx\nnext", Time: time.Now()}}
	notes := []note{{at: time.Now(), text: "evil\x1b[2Jnote"}}
	out := strings.Join(renderLines(msgs, notes, lineOpts{width: 80, nameW: 3, nameOf: nameX, fullDate: true}), "\n")
	if strings.ContainsRune(out, 0x07) || strings.Contains(out, "\x1b]") || strings.Contains(out, "\x1b[31m<") || strings.Contains(out, "\x1b[2J") {
		t.Fatalf("control characters leaked: %q", out)
	}
	if !strings.Contains(out, "next") {
		t.Fatal("newline-separated text should survive")
	}
}

func TestTopWindow(t *testing.T) {
	lines := []string{"1", "2", "3", "4", "5", "6", "7"}
	if v := topWindow(lines, 3, 0); strings.Join(v, "") != "123" {
		t.Fatalf("top: %q", v)
	}
	if v := topWindow(lines, 3, 1); strings.Join(v, "") != "123" {
		t.Fatalf("cursor moves to the middle first: %q", v)
	}
	if v := topWindow(lines, 3, 3); strings.Join(v, "") != "345" {
		t.Fatalf("then the list scrolls under a centred cursor: %q", v)
	}
	if v := topWindow(lines, 3, 6); strings.Join(v, "") != "567" {
		t.Fatalf("cursor reaches the bottom at the end: %q", v)
	}
	if v := topWindow(lines, 3, 5); strings.Join(v, "") != "567" {
		t.Fatalf("near the end the cursor leaves the middle: %q", v)
	}
	if v := topWindow(lines[:2], 4, 0); len(v) != 4 || v[0] != "1" || v[3] != "" {
		t.Fatalf("pad bottom: %q", v)
	}
	if v := topWindow(nil, 2, 0); len(v) != 2 {
		t.Fatalf("empty: %q", v)
	}
}

func TestRenderActionLines(t *testing.T) {
	msgs := []domain.Message{{ID: "1", ChatID: "c", Sender: "ada", Text: "* waves at everyone", Time: time.Now()}}
	out := renderLines(msgs, nil, lineOpts{width: 80, nameW: 4, nameOf: nameX, fullDate: true})[0]
	if !strings.Contains(out, "* ada") || strings.Contains(out, "<ada>") || !strings.Contains(out, "waves at everyone") {
		t.Fatalf("action line: %q", out)
	}
	plain := renderLines([]domain.Message{{ID: "2", ChatID: "c", Sender: "ada", Text: "*", Time: time.Now()}}, nil, lineOpts{width: 80, nameW: 4, nameOf: nameX, fullDate: true})[0]
	if !strings.Contains(plain, "<ada>") {
		t.Fatalf("a lone star is not an action: %q", plain)
	}
}

func TestRenderButtonsUnderTheMessage(t *testing.T) {
	msgs := []domain.Message{{ID: "42", ChatID: "c", Sender: "bot", Text: "deploy?", Time: time.Now(), Buttons: []domain.Button{
		{Label: "Yes", Kind: domain.ButtonCallback}, {Label: "Docs\x1b[2J", Kind: domain.ButtonURL, URL: "https://x"}, {Label: "Pay", Kind: domain.ButtonOther},
	}}}
	lines := renderLines(msgs, nil, lineOpts{width: 80, nameW: 4, nameOf: nameX, fullDate: true})
	if len(lines) != 2 {
		t.Fatalf("want message plus button line: %q", lines)
	}
	for _, want := range []string{"[1] Yes", "[2] Docs", "(link)", "[3] Pay (unsupported)", "(msg 42)"} {
		if !strings.Contains(lines[1], want) {
			t.Errorf("button line lacks %q: %q", want, lines[1])
		}
	}
	if strings.Contains(lines[1], "\x1b[2J") {
		t.Fatal("button labels must be sanitized")
	}
}

func spanAt(sp []span, x int) (span, bool) {
	for _, s := range sp {
		if s.hit(x) {
			return s, true
		}
	}
	return span{}, false
}

func TestRenderRichSpans(t *testing.T) {
	ts := time.Date(2026, 5, 6, 14, 7, 0, 0, time.Local)
	long := "https://example.invalid/" + strings.Repeat("abc/", 20) + "end"
	msgs := []domain.Message{{ID: "1", ChatID: "c1", Sender: "ada", Text: "see https://example.invalid/x and " + long, Time: ts}}
	lines, spans := renderRich(msgs, nil, lineOpts{width: 60, nameW: 4, nameOf: nameX, showChat: true})
	if len(lines) != len(spans) || len(lines) < 3 {
		t.Fatalf("lines=%d spans=%d", len(lines), len(spans))
	}
	first := spans[1] // after the day separator
	if s, ok := spanAt(first, 7); !ok || s.kind != spanChat || s.target != "c1" {
		t.Fatalf("chat column: %+v", first)
	}
	if s, ok := spanAt(first, 12); !ok || s.kind != spanNick || s.target != "ada" {
		t.Fatalf("nick: %+v", first)
	}
	var urls []string
	for _, row := range spans {
		for _, s := range row {
			if s.kind == spanURL {
				urls = append(urls, s.target)
			}
		}
	}
	if len(urls) < 2 || urls[0] != "https://example.invalid/x" {
		t.Fatalf("urls: %v", urls)
	}
	for _, u := range urls[1:] {
		if u != long {
			t.Fatalf("a wrapped link must map to the full URL: %q", u)
		}
	}
	_, sp := renderRich(msgs, nil, lineOpts{width: 60, nameW: 4, nameOf: nameX})
	for _, s := range sp[1] {
		if s.kind == spanChat {
			t.Fatal("no chat column in a chat window")
		}
	}
	if s, ok := spanAt(sp[1], 7); !ok || s.kind != spanNick {
		t.Fatalf("nick right after the time in a chat window: %+v", sp[1])
	}
}

func TestWindowGenericPadsAtTop(t *testing.T) {
	sp := [][]span{{{0, 1, spanURL, "a"}}, {{0, 1, spanURL, "b"}}}
	got, off := window(sp, 4, 0)
	if off != 0 || len(got) != 4 || got[0] != nil || got[3][0].target != "b" {
		t.Fatalf("%v %d", got, off)
	}
}

func TestLinksTrimPunctuationAndPreferExactMatches(t *testing.T) {
	if got := urlsOf("see https://a.example/x. and (https://b.example/y), ok"); len(got) != 2 || got[0] != "https://a.example/x" || got[1] != "https://b.example/y" {
		t.Fatalf("%v", got)
	}
	full := []string{"https://a.example/x", "https://a.example/xyz"}
	if wholeURL("https://a.example/x", false, full) != "https://a.example/x" {
		t.Fatal("an exact entry wins over a longer one")
	}
	if wholeURL("https://a.exam", true, full) != "https://a.example/x" {
		t.Fatal("a fragment at the end of a line maps to the first URL starting with it")
	}
	if wholeURL("https://a.exam", false, full) != "https://a.exam" {
		t.Fatal("a fragment inside a line is left alone")
	}
	note := []note{{at: time.Now(), text: "open https://example.invalid/" + strings.Repeat("abc/", 30) + "end"}}
	_, spans := renderRich(nil, note, lineOpts{width: 60, nameW: 4, nameOf: nameX})
	for _, row := range spans {
		for _, s := range row {
			if s.kind == spanURL && !strings.HasSuffix(s.target, "/end") {
				t.Fatalf("a wrapped link in a notice must map to the whole URL: %q", s.target)
			}
		}
	}
}

func TestBouncePos(t *testing.T) {
	// A full pass takes travelMs whatever the length: the end is reached at
	// travelMs, the start again at 2*travelMs.
	for _, n := range []int{2, 5, 40} {
		if bouncePos(0, n) != 0 || bouncePos(travelMs, n) != n-1 || bouncePos(2*travelMs, n) != 0 {
			t.Fatalf("n=%d: %d %d %d", n, bouncePos(0, n), bouncePos(travelMs, n), bouncePos(2*travelMs, n))
		}
	}
	if bouncePos(travelMs/2, 5) != 2 || bouncePos(travelMs+travelMs/2, 5) != 2 {
		t.Fatal("halfway, both directions")
	}
	if bouncePos(5, 1) != 0 || bouncePos(5, 0) != 0 {
		t.Fatal("short elements")
	}
	a, b := clickable("abcd", linkStyle, 0), clickable("abcd", linkStyle, 1)
	if a == b && lipgloss.Width(a) > 0 && strings.Contains(a, "\x1b") {
		t.Fatal("the light must move")
	}
}
