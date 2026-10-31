package tui

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"  // decoders for the attachments lazygram shows
	_ "image/jpeg" //
	_ "image/png"  //
	"strings"

	"github.com/charmbracelet/lipgloss"
	_ "golang.org/x/image/webp" // Telegram stickers and image files
)

// maxImageRows is the height of an in-terminal picture; each text row
// holds two pixel rows.
const maxImageRows = 24

// renderImage decodes a picture and paints it as text: every cell is an
// upper half block whose foreground is the top pixel and background the
// bottom one, so a row of text holds two rows of pixels. The picture is
// scaled to fit cols x rows cells, keeping its aspect ratio. Nothing is
// written anywhere.
func renderImage(data []byte, cols, rows int) ([]string, image.Point, error) {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, image.Point{}, fmt.Errorf("not a picture lazygram can show (%w)", err)
	}
	b := src.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 || cols < 2 || rows < 1 {
		return nil, image.Point{}, errors.New("empty picture")
	}
	// A cell is about twice as tall as it is wide, and holds two pixels
	// vertically, so pixels map one to one horizontally and vertically.
	w, h := cols, 2*rows
	if b.Dx()*h > b.Dy()*w {
		h = max(b.Dy()*w/b.Dx(), 2)
	} else {
		w = max(b.Dx()*h/b.Dy(), 1)
	}
	h += h % 2
	px := func(x, y int) color.RGBA { // box filter over the source block
		x0, x1 := b.Min.X+x*b.Dx()/w, b.Min.X+(x+1)*b.Dx()/w
		y0, y1 := b.Min.Y+y*b.Dy()/h, b.Min.Y+(y+1)*b.Dy()/h
		if x1 <= x0 {
			x1 = x0 + 1
		}
		if y1 <= y0 {
			y1 = y0 + 1
		}
		var r, g, bl, n uint64
		for yy := y0; yy < y1; yy++ {
			for xx := x0; xx < x1; xx++ {
				cr, cg, cb, _ := src.At(xx, yy).RGBA()
				r, g, bl, n = r+uint64(cr>>8), g+uint64(cg>>8), bl+uint64(cb>>8), n+1
			}
		}
		return color.RGBA{uint8(r / n), uint8(g / n), uint8(bl / n), 255}
	}
	hex := func(c color.RGBA) lipgloss.Color { return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)) }
	lines := make([]string, 0, h/2)
	for y := 0; y < h; y += 2 {
		var sb strings.Builder
		for x := 0; x < w; x++ {
			top, bottom := px(x, y), px(x, y+1)
			sb.WriteString(lipgloss.NewStyle().Foreground(hex(top)).Background(hex(bottom)).Render("▀"))
		}
		lines = append(lines, sb.String())
	}
	return lines, image.Point{X: b.Dx(), Y: b.Dy()}, nil
}
