// Package logo holds the figlet banners shown when lazygram starts and
// exits, BitchX style. Generated with figlet (fonts slant and smslant);
// plain ASCII only. The lines are double-quoted because the slant font
// contains backticks.
package logo

import "strings"

var startLines = []string{
	"    __",
	"   / /___ _____  __  ______ __________ _____ ___",
	"  / / __ `/_  / / / / / __ `/ ___/ __ `/ __ `__ \\",
	" / / /_/ / / /_/ /_/ / /_/ / /  / /_/ / / / / / /",
	"/_/\\__,_/ /___/\\__, /\\__, /_/   \\__,_/_/ /_/ /_/",
	"              /____//____/",
}

var exitLines = []string{
	"   __",
	"  / /__ ____ __ _____ ________ ___ _",
	" / / _ `/_ // // / _ `/ __/ _ `/  ' \\",
	"/_/\\_,_//__/\\_, /\\_, /_/  \\_,_/_/_/_/",
	"           /___//___/",
}

// Start is the banner for the server window at start-up.
var Start = strings.Join(startLines, "\n")

// Exit is the banner printed to the terminal after the program ends.
var Exit = strings.Join(exitLines, "\n")

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
