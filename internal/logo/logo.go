// Package logo holds the figlet banners shown when lazygram starts and
// exits, BitchX style: a solid block face (figlet font banner3) where the
// terminal has room and a compact one (font chunky) below that. Plain
// ASCII only. The lines are double-quoted because the fonts contain
// backticks and backslashes.
package logo

import "strings"

var wideLines = []string{
	"##          ###    ######## ##    ##  ######   ########     ###    ##     ##",
	"##         ## ##        ##   ##  ##  ##    ##  ##     ##   ## ##   ###   ###",
	"##        ##   ##      ##     ####   ##        ##     ##  ##   ##  #### ####",
	"##       ##     ##    ##       ##    ##   #### ########  ##     ## ## ### ##",
	"##       #########   ##        ##    ##    ##  ##   ##   ######### ##     ##",
	"##       ##     ##  ##         ##    ##    ##  ##    ##  ##     ## ##     ##",
	"######## ##     ## ########    ##     ######   ##     ## ##     ## ##     ##",
}

var narrowLines = []string{
	" __",
	"|  |.---.-.-----.--.--.-----.----.---.-.--------.",
	"|  ||  _  |-- __|  |  |  _  |   _|  _  |        |",
	"|__||___._|_____|___  |___  |__| |___._|__|__|__|",
	"                |_____|_____|",
}

// Wide is the banner for terminals of about 80 columns or more.
var Wide = strings.Join(wideLines, "\n")

// Narrow is the compact banner for anything smaller.
var Narrow = strings.Join(narrowLines, "\n")

// Pick returns the widest banner that fits in width columns.
func Pick(width int) string {
	if width >= Width(Wide) {
		return Wide
	}
	return Narrow
}

// Lines splits a banner into its lines without a trailing empty one.
func Lines(banner string) []string {
	return strings.Split(strings.TrimRight(banner, "\n"), "\n")
}

// Width is the widest line of a banner.
func Width(banner string) int {
	w := 0
	for _, l := range Lines(banner) {
		w = max(w, len(l))
	}
	return w
}
