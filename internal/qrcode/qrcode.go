// Package qrcode renders a QR code as terminal text, the way nchat does it:
// two modules per character row using half-block characters, drawn for a
// dark terminal background (light modules are the foreground). An ASCII-only
// mode uses two characters per module instead.
package qrcode

import (
	"strings"

	"rsc.io/qr"
)

const quietZone = 4 // modules of border required by the QR spec

// Render encodes text with medium error correction and returns the lines.
func Render(text string, ascii bool) (string, error) {
	code, err := qr.Encode(text, qr.M)
	if err != nil {
		return "", err
	}
	return render(code.Size, code.Black, ascii), nil
}

// render draws a size x size matrix; dark reports whether a module is dark.
// Coordinates outside the matrix are the light quiet zone.
func render(size int, dark func(x, y int) bool, ascii bool) string {
	at := func(x, y int) bool {
		return x >= 0 && y >= 0 && x < size && y < size && dark(x, y)
	}
	var b strings.Builder
	if ascii {
		for y := -quietZone; y < size+quietZone; y++ {
			for x := -quietZone; x < size+quietZone; x++ {
				if at(x, y) {
					b.WriteString("  ")
				} else {
					b.WriteString("##")
				}
			}
			b.WriteByte('\n')
		}
		return b.String()
	}
	for y := -quietZone; y < size+quietZone; y += 2 {
		for x := -quietZone; x < size+quietZone; x++ {
			top, bottom := at(x, y), at(x, y+1)
			switch {
			case top && bottom:
				b.WriteByte(' ')
			case top:
				b.WriteString("▄") // lower half block: bottom is light
			case bottom:
				b.WriteString("▀") // upper half block: top is light
			default:
				b.WriteString("█") // full block: both light
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}
