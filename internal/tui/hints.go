package tui

// Start-up hints in the server window. A new user's first steps are a
// chat and a window, so those two hints show at every start; the rest is
// discovered one tip at a time, hintPicks of them per start, drawn at
// random from hintPool. Each line must stay true of the current code
// (specs/ui.md) and fit one row of an 80-column window after the
// "HH:MM -!- " stamp (TestHintsAreWellFormed). They are read in window 0,
// which has no chat: a hint about an action that needs one names a chat
// window, never "this" chat or window, and a setting word follows only a
// command that takes it (/mouse toggles and takes none).

// hintsFixed always show, first.
var hintsFixed = []string{
	"hint: /list shows the chats, @people and #channels; Enter opens one",
	"hint: /win lists the open windows; /win N or Alt-N switches to one",
}

// hintPool holds the hints drawn at random after the fixed ones.
var hintPool = []string{
	"hint: Tab completes commands, chat names and paths; Tab again lists",
	"hint: in a chat window Ctrl-X sends the message and Enter adds a line",
	"hint: Ctrl-X twice in a chat window makes Enter send; again undoes",
	"hint: Ctrl-N and Ctrl-P, or Tab on an empty prompt, walk the windows",
	"hint: /search <text> pages through a chat's history; all in window 0",
	"hint: double click copies a word, triple click a row, a drag a range",
	"hint: /theme lists the colour themes; /theme <name> switches to one",
	"hint: /autojoin off stops opening a window per unread chat at start",
	"hint: /read-receipts on reports a chat read when you focus its window",
	"hint: /whois <name> describes a chat, or in a group window a member",
	"hint: /send <path> in a chat window sends a file; Tab completes paths",
	"hint: click a link, or type /open <link>, to open it in the browser",
	"hint: Ctrl-L or /redraw drops the window's notices; its messages stay",
	"hint: /about shows the version, the build and the libraries",
	"hint: /mark-as-read in a chat window reports it read, receipts or not",
	"hint: /names switches chats between nicks (@alias) and titles (@Title)",
	"hint: /last N in a chat window fetches its last N messages, up to 100",
	"hint: Up / Down recall the commands and messages entered this session",
	"hint: /mouse toggles clicks; off, the terminal's own drag-select works",
	"hint: PgUp / PgDn scroll a page; End jumps back to the newest line",
	"hint: /move N moves a chat window to position N; /close closes it",
	"hint: /feed off keeps chat messages out of window 0; notices stay",
	"hint: /settings lists every setting; Enter or Left/Right change one",
	"hint: /win last, or Tab twice fast, jumps back to the last window",
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
