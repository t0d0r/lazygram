package tui

import (
	"strings"
	"testing"
	"time"

	"lazygram/internal/domain"
)

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
		{"", 10, []string{""}},
	}
	for _, c := range cases {
		got := wrapText(c.in, c.w)
		if strings.Join(got, "|") != strings.Join(c.want, "|") {
			t.Errorf("wrap(%q,%d)=%q want %q", c.in, c.w, got, c.want)
		}
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

func TestRenderLinesShowsTimeAliasSenderText(t *testing.T) {
	ts := time.Date(2026, 5, 6, 14, 7, 0, 0, time.Local)
	msgs := []domain.Message{{ID: "1", ChatID: "c1", Sender: "ada", Text: "hello there", Time: ts}}
	lines := renderLines(msgs, lineOpts{width: 80, aliasW: 4, aliasOf: func(string) string { return "dev" }})
	if len(lines) != 2 || !strings.Contains(lines[0], "2026-05-06") {
		t.Fatalf("want day separator then message: %q", lines)
	}
	for _, want := range []string{"14:07", "#dev", "<ada>", "hello there"} {
		if !strings.Contains(lines[1], want) {
			t.Errorf("line %q lacks %q", lines[1], want)
		}
	}
}

func TestRenderWrapsWithHangingIndent(t *testing.T) {
	msgs := []domain.Message{{ID: "1", ChatID: "c", Sender: "s", Text: strings.Repeat("word ", 20), Time: time.Now()}}
	lines := renderLines(msgs, lineOpts{width: 40, aliasW: 3, aliasOf: func(string) string { return "x" }, fullDate: true})
	if len(lines) < 3 {
		t.Fatalf("expected wrapping, got %d lines", len(lines))
	}
	if !strings.HasPrefix(lines[1], "  ") {
		t.Fatalf("continuation should be indented: %q", lines[1])
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
	out := strings.Join(renderLines(msgs, lineOpts{width: 80, aliasW: 3, aliasOf: func(string) string { return "x" }, fullDate: true}), "\n")
	if strings.ContainsRune(out, 0x07) || strings.Contains(out, "\x1b]") || strings.Contains(out, "\x1b[31m<") {
		t.Fatalf("control characters leaked: %q", out)
	}
	if !strings.Contains(out, "next") {
		t.Fatal("newline-separated text should survive")
	}
}
