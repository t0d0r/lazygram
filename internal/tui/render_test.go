package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

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
