package tui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 255 / max(w-1, 1)), uint8(y * 255 / max(h-1, 1)), 90, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestRenderImageFitsAndKeepsAspect(t *testing.T) {
	lines, size, err := renderImage(testPNG(t, 400, 100), 40, 24)
	if err != nil || size != (image.Point{400, 100}) {
		t.Fatal(err, size)
	}
	if len(lines) != 5 { // 40 cells wide -> 10 pixel rows -> 5 text rows
		t.Fatalf("rows %d", len(lines))
	}
	for _, l := range lines {
		if strings.Count(l, "▀") != 40 {
			t.Fatalf("40 cells per row: %q", l)
		}
	}
	lines, _, err = renderImage(testPNG(t, 100, 400), 40, 10)
	if err != nil || len(lines) != 10 || strings.Count(lines[0], "▀") != 5 {
		t.Fatalf("tall picture: rows=%d cells=%d err=%v", len(lines), strings.Count(lines[0], "▀"), err)
	}
	if _, _, err := renderImage([]byte("not an image"), 40, 10); err == nil {
		t.Fatal("garbage must fail")
	}
}
