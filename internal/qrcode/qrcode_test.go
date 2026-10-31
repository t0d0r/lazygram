package qrcode

import (
	"strings"
	"testing"
)

// checker is a 2x2 matrix: dark at (0,0) and (1,1).
func checker(x, y int) bool { return x == y }

func TestRenderHalfBlocks(t *testing.T) {
	out := render(2, checker, false)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != (2+2*quietZone)/2 {
		t.Fatalf("want %d rows, got %d", (2+2*quietZone)/2, len(lines))
	}
	row := []rune(lines[quietZone/2]) // the row holding both matrix lines
	if len(row) != 2+2*quietZone {
		t.Fatalf("row width %d", len(row))
	}
	if row[0] != '█' {
		t.Fatalf("quiet zone must be light (full block), got %q", row[0])
	}
	if row[quietZone] != '▄' || row[quietZone+1] != '▀' {
		t.Fatalf("checker rendered as %q %q", row[quietZone], row[quietZone+1])
	}
}

func TestRenderASCII(t *testing.T) {
	out := render(2, checker, true)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2+2*quietZone {
		t.Fatalf("rows %d", len(lines))
	}
	if l := lines[quietZone]; !strings.HasPrefix(l, strings.Repeat("##", quietZone)+"  ##") {
		t.Fatalf("row %q", l)
	}
	if strings.ContainsFunc(out, func(r rune) bool { return r > 127 }) {
		t.Fatal("ascii mode must be pure ASCII")
	}
}

func TestRenderEncodesLoginURL(t *testing.T) {
	out, err := Render("tg://login?token=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", false)
	if err != nil || len(out) == 0 {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	w := len([]rune(lines[0]))
	for _, l := range lines {
		if len([]rune(l)) != w {
			t.Fatal("ragged output")
		}
	}
	if w < 25 || w > 80 {
		t.Fatalf("unexpected width %d", w)
	}
}
