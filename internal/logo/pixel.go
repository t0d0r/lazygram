package logo

import (
	"math/rand/v2"
	"strings"
)

// The exit banner comes in several faces, one picked at random: the
// figlet block face, the word drawn from the pixel font below in three
// styles (solid, bracketed, standing on a shadow), the compact figlet
// face and a spaced wordmark. All plain
// ASCII, at most 80 columns.

// pixelFont is a 5-row bitmap font for the letters of "lazygram"; # is a
// set pixel. Letters are four columns wide, L three and M five.
var pixelFont = map[rune][]string{
	'l': {"#..", "#..", "#..", "#..", "###"},
	'a': {".##.", "#..#", "####", "#..#", "#..#"},
	'z': {"####", "...#", ".##.", "#...", "####"},
	'y': {"#..#", "#..#", ".##.", ".##.", ".##."},
	'g': {".###", "#...", "#.##", "#..#", ".###"},
	'r': {"###.", "#..#", "###.", "#.#.", "#..#"},
	'm': {"#...#", "##.##", "#.#.#", "#...#", "#...#"},
}

const pixelRows = 5

// pixels draws the word from the pixel font with two characters per
// pixel, so the letters keep their proportions in a terminal font, a
// blank column between letters, and an optional shadow under every
// pixel that has nothing below it, so the letters seem to stand on it.
func pixels(word, on, shadow string) string {
	type glyph struct {
		x    int
		rows []string
	}
	var glyphs []glyph
	x := 0
	for _, r := range word {
		g, ok := pixelFont[r]
		if !ok {
			continue
		}
		if x > 0 {
			x++ // a blank column between letters
		}
		glyphs = append(glyphs, glyph{x, g})
		x += len(g[0]) * 2
	}
	width, height := x, pixelRows+1
	grid := make([][]byte, height)
	for y := range grid {
		grid[y] = []byte(strings.Repeat(" ", width))
	}
	put := func(y, x int, s string) {
		for i := 0; i < len(s) && x+i < width; i++ {
			grid[y][x+i] = s[i]
		}
	}
	each := func(f func(y, x int)) {
		for _, g := range glyphs {
			for y, row := range g.rows {
				for i := range row {
					if row[i] == '#' {
						f(y, g.x+i*2)
					}
				}
			}
		}
	}
	if shadow != "" {
		each(func(y, x int) { put(y+1, x, shadow) })
	}
	each(func(y, x int) { put(y, x, on) })
	lines := make([]string, 0, height)
	for y := range grid {
		if l := strings.TrimRight(string(grid[y]), " "); l != "" || y < pixelRows {
			lines = append(lines, l)
		}
	}
	return strings.Join(lines, "\n")
}

// wordmark spaces the letters out over a rule, the quiet version.
func wordmark(word string) string {
	letters := make([]string, 0, len(word))
	for _, r := range word {
		letters = append(letters, string(r))
	}
	w := strings.Join(letters, "   ")
	return w + "\n" + strings.Repeat("=", len(w))
}

// exits are the faces the exit banner is drawn in.
var exits = []string{
	Wide,
	pixels("lazygram", "##", ""),
	pixels("lazygram", "[]", ""),
	pixels("lazygram", "##", "::"),
	Narrow,
	wordmark("lazygram"),
}

// Variants returns the exit faces that fit in width columns, or the
// narrow banner alone when none does.
func Variants(width int) []string {
	var out []string
	for _, e := range exits {
		if Width(e) <= width {
			out = append(out, e)
		}
	}
	if len(out) == 0 {
		return []string{Narrow}
	}
	return out
}

// Random picks one of the exit faces that fit in width columns.
func Random(width int) string {
	v := Variants(width)
	return v[rand.IntN(len(v))]
}
