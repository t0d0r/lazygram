package tui

// Start-up hints in the server window. A new user's first steps are a
// chat and a window, so those two hints show at every start; the rest is
// discovered one tip at a time, hintPicks of them per start, drawn at
// random from hintPool. Each line must stay true of the current code
// (specs/ui.md) and fit one row of an 80-column window after the
// "HH:MM -!- " stamp (TestHintsAreWellFormed).

// hintsFixed always show, first.
var hintsFixed = []string{
	"hint: /list shows the chats, @people and #channels; Enter opens one",
	"hint: /win lists the open windows; /win N or Alt-N switches to one",
}

// hintPool holds the hints drawn at random after the fixed ones.
var hintPool = []string{
	"hint: Tab completes commands, chats and /send paths; Tab again lists",
	"hint: in a chat window Ctrl-X sends the message and Enter adds a line",
	"hint: Ctrl-X twice makes Enter send in this window, twice again undoes",
	"hint: Ctrl-N and Ctrl-P, or Tab on an empty prompt, walk the windows",
	"hint: /search <text> pages through this chat's local history",
	"hint: double click copies a word, triple click a row, a drag a range",
	"hint: /theme lists the colour themes; /theme <name> switches to one",
	"hint: /autojoin off stops opening a window per unread chat at start",
	"hint: /read-receipts on reports a chat read when you focus its window",
	"hint: /whois <name> says what is known about a chat or a group member",
	"hint: /send <path> sends a file to this chat; Tab completes the path",
	"hint: click a link, or type /open <link>, to open it in the browser",
	"hint: Ctrl-L or /redraw drops this window's notices; messages stay",
	"hint: /about shows the version, the build and the libraries",
	"hint: /mark-as-read reports this chat read now, receipts on or off",
	"hint: /names switches chats between nicks (@alias) and titles (@Title)",
	"hint: /last N fetches the last N messages of this chat, up to 100",
	"hint: Up / Down recall the commands and messages entered this session",
	"hint: /mouse off gives the terminal its own drag-select back",
	"hint: PgUp / PgDn scroll a page; End jumps back to the newest line",
	"hint: /move N puts this window at position N; /close closes it",
}

// hintPicks is how many pool hints one start shows.
const hintPicks = 2

// startHints is the hint block of one start: the fixed hints, then
// hintPicks pool hints drawn without repeats. intn returns a number in
// [0, n), as rand.IntN does.
func startHints(intn func(n int) int) []string {
	out := append([]string(nil), hintsFixed...)
	pool := append([]string(nil), hintPool...)
	for range hintPicks {
		i := intn(len(pool))
		out = append(out, pool[i])
		pool = append(pool[:i], pool[i+1:]...) // drawn hints leave the pool: no repeats
	}
	return out
}
