// Package logo holds the figlet banners shown when lazygram starts and
// exits, BitchX style. Generated with figlet (fonts slant, lean and
// smslant); plain ASCII only. The lines are double-quoted because the
// fonts contain backticks.
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

var exitWideLines = []string{
	"    _/",
	"   _/    _/_/_/  _/_/_/_/  _/    _/    _/_/_/  _/  _/_/    _/_/_/  _/_/_/  _/_/",
	"  _/  _/    _/      _/    _/    _/  _/    _/  _/_/      _/    _/  _/    _/    _/",
	" _/  _/    _/    _/      _/    _/  _/    _/  _/        _/    _/  _/    _/    _/",
	"_/    _/_/_/  _/_/_/_/    _/_/_/    _/_/_/  _/          _/_/_/  _/    _/    _/",
	"                             _/        _/",
	"                        _/_/      _/_/",
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

// ExitWide is the banner printed to a terminal of 80 columns or more
// after the program ends.
var ExitWide = strings.Join(exitWideLines, "\n")

// Exit is the narrower exit banner.
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
