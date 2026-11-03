package tui

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"lazygram/internal/domain"
)

// vimChat is @alice's window (chatOn, where Enter adds a line break and
// Ctrl-X sends) with vim on, in Insert.
func vimChat(t *testing.T) *Model {
	t.Helper()
	m := chatOn(t, 100, 30)
	enter(m, "/vim on")
	if !m.vi.on || m.vi.normal || m.cur != 3 {
		t.Fatalf("vim on, in Insert, in @alice's window: on=%v normal=%v cur=%d", m.vi.on, m.vi.normal, m.cur)
	}
	return m
}

// normalAt puts text in the input with the cursor on rune p, in Normal
// mode, with nothing waiting, nothing killed and nothing to undo.
func normalAt(m *Model, text string, p int) {
	m.input = []rune(text)
	m.curBack = len(m.input) - p
	m.vi.normal, m.vi.op, m.vi.kill, m.vi.undo = true, 0, nil, nil
}

// curText is the input with a | before the cursor: on the character after
// it, or at the end.
func curText(m *Model) string {
	p := m.pos()
	return string(m.input[:p]) + "|" + string(m.input[p:])
}

// alt presses Alt with a character.
func alt(m *Model, r rune) { m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}, Alt: true}) }

// statusRow is the status line of the last frame, for a one-row input.
func statusRow(m *Model) string {
	rows := strings.Split(ansi.Strip(m.View()), "\n")
	return rows[len(rows)-3]
}

func TestVimIsOffByDefault(t *testing.T) {
	m := chatOn(t, 100, 30)
	var saved bool
	if found, _ := m.opt.Store.State(vimKey, &saved); found || m.vi.on {
		t.Fatalf("off with nothing stored: on=%v stored=%v", m.vi.on, found)
	}
	enter(m, "/vim")
	if m.isErr || m.status != "vim off: the input keys are the usual ones; /vim on for vi keys" {
		t.Fatalf("no argument shows the setting: %q", m.status)
	}
	if found, _ := m.opt.Store.State(vimKey, &saved); found {
		t.Fatal("showing the setting keeps nothing")
	}
	typ(m, "hjkl")
	press(m, tea.KeyEsc)
	if len(m.input) != 0 || m.vi.normal || strings.Contains(statusRow(m), "NORMAL") {
		t.Fatalf("off: letters type, Esc clears, no Normal mode: %q normal=%v", string(m.input), m.vi.normal)
	}
}

func TestVimIsKeptAndStartsInInsert(t *testing.T) {
	st := memStore(t)
	m := startWith(t, st, newFake())
	enter(m, "/vim on")
	var saved bool
	if found, err := st.State(vimKey, &saved); !found || err != nil || !saved {
		t.Fatalf("on is kept: found=%v saved=%v err=%v", found, saved, err)
	}
	if !m.vi.on || m.vi.normal || m.isErr ||
		m.status != "vim on: typing works as before; Esc enters Normal mode (vi keys), i returns to typing" {
		t.Fatalf("on: Insert at once: on=%v normal=%v %q", m.vi.on, m.vi.normal, m.status)
	}
	typ(m, "hi")
	if string(m.input) != "hi" {
		t.Fatalf("typing works at once: %q", string(m.input))
	}
	press(m, tea.KeyEsc)
	m.input, m.curBack = nil, 0
	typ(m, ":vim") // from Normal: ':' starts a command line
	press(m, tea.KeyEnter)
	if m.status != "vim on: Esc enters Normal mode (vi keys), i returns to typing; /vim off" {
		t.Fatalf("no argument shows on: %q", m.status)
	}
	again := startWith(t, st, newFake())
	if !again.vi.on || again.vi.normal {
		t.Fatalf("a new start keeps it on and begins in Insert: on=%v normal=%v", again.vi.on, again.vi.normal)
	}
	// Off from Normal mode, with ':': the usual keys at once.
	press(m, tea.KeyEsc)
	typ(m, ":vim off")
	press(m, tea.KeyEnter)
	if m.vi.on || m.vi.normal || m.status != "vim off: the input keys are the usual ones" {
		t.Fatalf("off: on=%v normal=%v %q", m.vi.on, m.vi.normal, m.status)
	}
	if found, _ := st.State(vimKey, &saved); !found || saved || startWith(t, st, newFake()).vi.on {
		t.Fatal("off is kept too")
	}
	// On again, from Normal state left behind: it begins in Insert again.
	m.vi.normal = true
	m.setVim("on")
	if m.vi.normal {
		t.Fatal("each /vim on begins in Insert")
	}
	// On again while on, with something killed and something to undo:
	// Insert, with nothing to put and nothing to undo.
	normalAt(m, "hello", 0)
	typ(m, "x")
	if string(m.vi.kill) != "h" || m.vi.undo == nil {
		t.Fatalf("x kills and can be undone: kill %q undo %v", string(m.vi.kill), m.vi.undo)
	}
	m.setVim("on")
	if m.vi.normal || m.vi.kill != nil || m.vi.undo != nil {
		t.Fatalf("/vim on again empties the kill buffer and the undo: normal=%v kill %q undo %v", m.vi.normal, string(m.vi.kill), m.vi.undo)
	}
	press(m, tea.KeyEsc)
	for _, k := range "pu" {
		typ(m, string(k))
		if curText(m) != "|ello" || !m.vi.normal {
			t.Fatalf("%c: nothing to put, nothing to undo: %q", k, curText(m))
		}
	}
}

func TestVimUnreadableMeansOff(t *testing.T) {
	st := memStore(t)
	if err := st.SetState(vimKey, "nonsense"); err != nil {
		t.Fatal(err)
	}
	if m := startWith(t, st, newFake()); m.vi.on {
		t.Fatal("an unreadable stored value means off")
	}
}

func TestVimTakesOnlyOnOrOff(t *testing.T) {
	m := setup(t, newFake())
	enter(m, "/vim maybe")
	if !m.isErr || m.status != "/vim takes on or off" || m.vi.on {
		t.Fatalf("a bad argument is an error and changes nothing: on=%v %q", m.vi.on, m.status)
	}
	var saved bool
	if found, _ := m.opt.Store.State(vimKey, &saved); found {
		t.Fatal("a bad argument keeps nothing")
	}
	enter(m, "/vim ON")
	if !m.vi.on || m.isErr {
		t.Fatalf("the argument is case-insensitive: %v %q", m.vi.on, m.status)
	}
}

func TestVimSettingsRow(t *testing.T) {
	m := setup(t, newFake())
	s := settingNamed(t, "vim")
	if s.key != vimKey || s.desc != "vi keys: Esc for Normal, i for Insert" || !reflect.DeepEqual(s.values(), []string{"on", "off"}) {
		t.Fatalf("the row: %+v %q", s, s.values())
	}
	openAt(t, m, "vim")
	if menuValue(m, "vim") != "off" {
		t.Fatalf("off at first: %q", menuValue(m, "vim"))
	}
	press(m, tea.KeyEnter)
	var saved bool
	if found, _ := m.opt.Store.State(vimKey, &saved); !m.vi.on || m.vi.normal || !found || !saved || menuValue(m, "vim") != "on" ||
		!strings.HasPrefix(m.status, "vim on: ") {
		t.Fatalf("Enter turns it on, kept, in Insert: on=%v normal=%v %q", m.vi.on, m.vi.normal, m.status)
	}
	press(m, tea.KeyLeft)
	if m.vi.on || menuValue(m, "vim") != "off" {
		t.Fatal("Left turns it off")
	}
	press(m, tea.KeyEsc)
	enter(m, "/vim on")
	enter(m, "/settings")
	if menuValue(m, "vim") != "on" {
		t.Fatalf("the row follows /vim: %q", menuValue(m, "vim"))
	}
}

func TestTabCompletesVimSettings(t *testing.T) {
	m := setup(t, newFake())
	for typed, want := range map[string]string{
		"/vim of": "/vim off ",
		"/vim o":  "/vim o",  // on and off share the o
		"/vim al": "/vim al", // a setting, not a chat
		"/vi":     "/vim ",
	} {
		m.input, m.curBack = nil, 0
		typ(m, typed)
		press(m, tea.KeyTab)
		if got := string(m.input); got != want {
			t.Errorf("%q: got %q, want %q", typed, got, want)
		}
	}
}

func TestVimEscEntersNormalKeepingTheText(t *testing.T) {
	m := vimChat(t)
	typ(m, "hello")
	press(m, tea.KeyEsc)
	if !m.vi.normal || curText(m) != "hell|o" {
		t.Fatalf("Esc: Normal, the text kept, the cursor back on the last character: %v %q", m.vi.normal, curText(m))
	}
	typ(m, "i")
	press(m, tea.KeyLeft)
	press(m, tea.KeyEsc)
	if curText(m) != "he|llo" {
		t.Fatalf("Esc steps back onto the character before the cursor: %q", curText(m))
	}
	normalAt(m, "one\ntwo", 4)
	m.vi.normal = false
	press(m, tea.KeyEsc)
	if curText(m) != "one\n|two" {
		t.Fatalf("at a line's start it stays: %q", curText(m))
	}
	press(m, tea.KeyEsc) // Esc in Normal changes nothing
	if !m.vi.normal || curText(m) != "one\n|two" {
		t.Fatalf("Esc in Normal: %q", curText(m))
	}
}

// TestVimEscThenDigitSwitchesWindows: Esc then a digit still switches
// windows, the input kept as it is; 0 is the line start only when it is
// not the key right after Esc.
func TestVimEscThenDigitSwitchesWindows(t *testing.T) {
	m := vimChat(t) // 0 server, 1 #news, 2 #dev, 3 @alice
	typ(m, "draft")
	press(m, tea.KeyEsc)
	typ(m, "2")
	if m.cur != 2 || curText(m) != "draf|t" || !m.vi.normal {
		t.Fatalf("Esc 2 from Insert: cur=%d %q normal=%v", m.cur, curText(m), m.vi.normal)
	}
	press(m, tea.KeyEsc) // in Normal too
	typ(m, "0")
	if m.cur != 0 || curText(m) != "draf|t" {
		t.Fatalf("Esc 0 in Normal: window 0: cur=%d %q", m.cur, curText(m))
	}
	typ(m, "0")
	if m.cur != 0 || curText(m) != "|draft" {
		t.Fatalf("0 not after Esc: the line start: cur=%d %q", m.cur, curText(m))
	}
	typ(m, "$")
	for _, d := range "123456789" {
		typ(m, string(d))
		if m.cur != 0 || curText(m) != "draf|t" || m.isErr {
			t.Fatalf("%c not after Esc: nothing: cur=%d %q %q", d, m.cur, curText(m), m.status)
		}
	}
	press(m, tea.KeyEsc)
	press(m, tea.KeyEsc)
	typ(m, "1")
	if m.cur != 1 {
		t.Fatalf("the last Esc arms the digit: cur=%d", m.cur)
	}
	alt(m, '3')
	if m.cur != 3 || string(m.input) != "draft" {
		t.Fatalf("Alt-3 in Normal: cur=%d %q", m.cur, string(m.input))
	}
	typ(m, "d") // a two-key command waiting
	press(m, tea.KeyEsc)
	typ(m, "2")
	if m.cur != 2 || string(m.input) != "draft" {
		t.Fatalf("Esc drops the waiting d, the digit switches: cur=%d %q", m.cur, string(m.input))
	}
	typ(m, "d")
	press(m, tea.KeyCtrlN)
	typ(m, "w")
	if m.cur != 3 || string(m.input) != "draft" {
		t.Fatalf("any key ends the wait: d, Ctrl-N, w deletes nothing: cur=%d %q", m.cur, string(m.input))
	}
}

func TestVimNormalMotions(t *testing.T) {
	m := vimChat(t)
	words := "foo bar.baz  qux"
	xcy := "x" + coder + "y"
	for _, c := range []struct {
		text string
		p    int
		keys string
		want string
	}{
		{"hello", 2, "h", "h|ello"},
		{"hello", 0, "h", "|hello"},
		{"ab\ncd", 3, "h", "ab\n|cd"}, // not onto the line above
		{"hello", 3, "l", "hell|o"},
		{"hello", 4, "l", "hell|o"},   // never past the last character
		{"ab\ncd", 1, "l", "a|b\ncd"}, // nor onto the next line
		{words, 0, "w", "foo |bar.baz  qux"},
		{words, 0, "ww", "foo bar|.baz  qux"},
		{words, 0, "www", "foo bar.|baz  qux"},
		{words, 0, "wwww", "foo bar.baz  |qux"},
		{words, 0, "wwwww", "foo bar.baz  qu|x"}, // no word after: the last character
		{words, 15, "b", "foo bar.baz  |qux"},
		{words, 15, "bb", "foo bar.|baz  qux"},
		{words, 15, "bbb", "foo bar|.baz  qux"},
		{words, 15, "bbbb", "foo |bar.baz  qux"},
		{words, 15, "bbbbbb", "|foo bar.baz  qux"},
		{words, 0, "e", "fo|o bar.baz  qux"},
		{words, 0, "ee", "foo ba|r.baz  qux"},
		{words, 0, "eee", "foo bar|.baz  qux"},
		{words, 0, "eeee", "foo bar.ba|z  qux"},
		{words, 0, "eeeee", "foo bar.baz  qu|x"},
		{words, 0, "eeeeee", "foo bar.baz  qu|x"},
		{"foo bar", 5, "b", "foo |bar"},
		{"foo bar", 5, "e", "foo ba|r"},
		{"ab\ncd", 0, "w", "ab\n|cd"},
		{"ab\ncd", 3, "b", "|ab\ncd"},
		{"ab\ncd", 1, "e", "ab\nc|d"},
		{"abc\n", 0, "w", "abc\n|"}, // no word after, an empty last line: on it, as vi
		{"abc\n", 2, "e", "abc\n|"},
		{"abc\n", 4, "w", "abc\n|"},
		{"\u043f\u0440\u0438\u0432\u0435\u0442 \u043c\u0438\u0440", 0, "w", "\u043f\u0440\u0438\u0432\u0435\u0442 |\u043c\u0438\u0440"},
		{"\u043f\u0440\u0438\u0432\u0435\u0442 \u043c\u0438\u0440", 0, "e", "\u043f\u0440\u0438\u0432\u0435|\u0442 \u043c\u0438\u0440"},
		{"\u043f\u0440\u0438\u0432\u0435\u0442,\u043c\u0438\u0440", 0, "w", "\u043f\u0440\u0438\u0432\u0435\u0442|,\u043c\u0438\u0440"}, // Cyrillic letters are a word, the comma is not
		{"\u043f\u0440\u0438\u0432\u0435\u0442,\u043c\u0438\u0440", 0, "e", "\u043f\u0440\u0438\u0432\u0435|\u0442,\u043c\u0438\u0440"},
		{"snake_case2 x", 0, "w", "snake_case2 |x"},
		{"  two words", 6, "0", "|  two words"},
		{"  two words", 6, "^", "  |two words"},
		{"  two words", 0, "$", "  two word|s"},
		{"one\n  two", 7, "0", "one\n|  two"},
		{"one\n  two", 8, "^", "one\n  |two"},
		{"one\n  two", 4, "$", "one\n  tw|o"},
		{"one\ntwo", 1, "$", "on|e\ntwo"},
		{xcy, 0, "l", "x|" + coder + "y"}, // an emoji sequence is one character
		{xcy, 0, "ll", "x" + coder + "|y"},
		{xcy, 4, "h", "x|" + coder + "y"},
		{xcy, 2, "l", "x" + coder + "|y"}, // from inside it, as drawn: on it
		{"e" + acute + "z", 0, "l", "e" + acute + "|z"},
		{"", 0, "hlwbe0^$", "|"},
	} {
		normalAt(m, c.text, c.p)
		typ(m, c.keys)
		if got := curText(m); got != c.want || !m.vi.normal || string(m.input) != c.text {
			t.Errorf("%q at %d, %s: %q, want %q (normal=%v)", c.text, c.p, c.keys, got, c.want, m.vi.normal)
		}
	}
}

// TestVimJKMoveRowsAndHistory: j and k are Down and Up: between the input
// rows, keeping the column, and through the history at the first and the
// last row; the cursor stays on a character.
func TestVimJKMoveRowsAndHistory(t *testing.T) {
	m := chatOn(t, 30, 30) // after the prompt "[@alice] " a row holds 20 cells
	enter(m, "/vim on")
	a25 := strings.Repeat("a", 25)
	paste(m, "the first line\n2nd\n"+a25+"\nthe last line of all")
	draft := string(m.input)
	press(m, tea.KeyEsc)
	at := func(row, col int) {
		t.Helper()
		m.View()
		rows, cur, _, _ := m.inputLayout(m.prompt())
		if cur != row || rows[cur].col(m.pos()) != col {
			t.Fatalf("cursor on row %d col %d, want row %d col %d", cur, rows[cur].col(m.pos()), row, col)
		}
	}
	at(4, 19)
	typ(m, "k")
	at(3, 4) // the rest of the wrapped line: its last character
	typ(m, "k")
	at(2, 19) // the column kept
	typ(m, "k")
	at(1, 2)
	typ(m, "k")
	at(0, 13)
	typ(m, "k") // the first row: the history
	if string(m.input) != "/vim on" || curText(m) != "/vim o|n" || !m.vi.normal {
		t.Fatalf("k on the first row recalls: %q", curText(m))
	}
	typ(m, "k")
	if string(m.input) != "/win @alice" {
		t.Fatalf("k again: %q", string(m.input))
	}
	typ(m, "jj")
	if string(m.input) != draft {
		t.Fatalf("j brings the draft back: %q", string(m.input))
	}
	at(4, 19)
	typ(m, "0")
	for row := 3; row >= 0; row-- {
		typ(m, "k")
		at(row, 0)
	}
	for row := 1; row <= 4; row++ {
		typ(m, "j")
		at(row, 0)
	}
	typ(m, "j") // the last row: nothing newer, the draft stays
	if string(m.input) != draft {
		t.Fatalf("j past the last row keeps the draft: %q", string(m.input))
	}
	// The same as Down and Up: compared key for key with the arrows.
	arrows := chatOn(t, 30, 30)
	enter(arrows, "/vim on")
	paste(arrows, draft)
	press(arrows, tea.KeyEsc)
	normalAt(m, draft, len(draft)-1)
	m.history, m.histIdx = slices.Clone(arrows.history), len(arrows.history)
	for i, k := range "kkkjjkkkkkjjjjjj" {
		typ(m, string(k))
		press(arrows, map[rune]tea.KeyType{'j': tea.KeyDown, 'k': tea.KeyUp}[k])
		if curText(m) != curText(arrows) {
			t.Fatalf("key %d (%c): %q; Up/Down give %q", i, k, curText(m), curText(arrows))
		}
	}
}

func TestVimEnterInsert(t *testing.T) {
	m := vimChat(t)
	for _, c := range []struct {
		text string
		p    int
		key  string
		want string
	}{
		{"abc", 1, "i", "aX|bc"},
		{"abc", 1, "a", "abX|c"},
		{"abc", 2, "a", "abcX|"},
		{"", 0, "a", "X|"},
		{"  abc", 3, "I", "  X|abc"},
		{"abc", 1, "A", "abcX|"},
		{"one\ntwo", 1, "A", "oneX|\ntwo"},
		{"one\ntwo", 1, "o", "one\nX|\ntwo"},
		{"one", 1, "o", "one\nX|"},
		{"one\ntwo", 5, "O", "one\nX|\ntwo"},
		{"one\ntwo", 1, "O", "X|\none\ntwo"},
	} {
		normalAt(m, c.text, c.p)
		typ(m, c.key)
		if m.vi.normal {
			t.Errorf("%q at %d, %s: Insert", c.text, c.p, c.key)
		}
		typ(m, "X")
		if got := curText(m); got != c.want {
			t.Errorf("%q at %d, %s then X: %q, want %q", c.text, c.p, c.key, got, c.want)
		}
	}
}

func TestVimEdits(t *testing.T) {
	m := vimChat(t)
	xcy := "x" + coder + "y"
	for _, c := range []struct {
		text   string
		p      int
		keys   string
		want   string
		kill   string
		insert bool
	}{
		{"hello", 1, "x", "h|llo", "e", false},
		{"abc", 2, "x", "a|b", "c", false}, // the last one: back onto the new last
		{"", 0, "x", "|", "", false},
		{xcy, 1, "x", "x|y", coder, false}, // a whole emoji
		{"hello", 2, "X", "h|llo", "e", false},
		{"hello", 0, "X", "|hello", "", false},
		{"ab\ncd", 3, "X", "ab\n|cd", "", false}, // not the line break
		{"hello world", 5, "D", "hell|o", " world", false},
		{"one\ntwo", 1, "D", "|o\ntwo", "ne", false},
		{"two\nlines", 5, "dd", "|", "two\nlines", false},
		{"foo bar baz", 4, "dw", "foo |baz", "bar ", false},
		{"foo bar", 4, "dw", "foo| ", "bar", false},
		{"foo\nbar", 0, "dw", "|\nbar", "foo", false}, // the line break stays
		{"foo  bar", 3, "dw", "foo|bar", "  ", false},
		{"foo bar", 0, "cw", "| bar", "foo", true}, // as vi: the blank after the word stays
		{"foo bar", 2, "cw", "fo| bar", "o", true},
		{"foo  bar", 3, "cw", "foo|bar", "  ", true},
		{"abc\ndef", 1, "cc", "|", "abc\ndef", true},
		{"abc", 1, "S", "|", "abc", true},
		{"hello world", 6, "C", "hello |", "world", true},
		{"cat", 0, "rb", "|bat", "", false},
		{"cat", 2, "r ", "ca| ", "", false},
		{xcy, 1, "rz", "x|zy", "", false},
		{"", 0, "rz", "|", "", false},
		{"cat", 1, "dz", "c|at", "", false}, // pairs it does not know: nothing
		{"cat", 1, "cz", "c|at", "", false},
		{"cat", 1, "gz", "c|at", "", false},
	} {
		normalAt(m, c.text, c.p)
		typ(m, c.keys)
		if got := curText(m); got != c.want || string(m.vi.kill) != c.kill || m.vi.normal == c.insert || m.vi.op != 0 {
			t.Errorf("%q at %d, %s: %q kill %q insert %v; want %q kill %q insert %v",
				c.text, c.p, c.keys, got, string(m.vi.kill), !m.vi.normal, c.want, c.kill, c.insert)
		}
	}
	// cw then typing replaces the word, as in vi.
	normalAt(m, "foo bar", 0)
	typ(m, "cwbaz")
	if string(m.input) != "baz bar" {
		t.Fatalf("cw baz: %q", string(m.input))
	}
	// r then Esc replaces nothing.
	normalAt(m, "cat", 0)
	typ(m, "r")
	press(m, tea.KeyEsc)
	typ(m, "l")
	if curText(m) != "c|at" {
		t.Fatalf("r Esc: %q", curText(m))
	}
}

func TestVimKillBufferPutAndUndo(t *testing.T) {
	m := vimChat(t)
	normalAt(m, "hello world", 0)
	typ(m, "dw")
	if curText(m) != "|world" || string(m.vi.kill) != "hello " {
		t.Fatalf("dw: %q kill %q", curText(m), string(m.vi.kill))
	}
	typ(m, "p")
	if curText(m) != "whello| orld" {
		t.Fatalf("p puts after the cursor's character, the cursor on its last: %q", curText(m))
	}
	typ(m, "u")
	if curText(m) != "|world" {
		t.Fatalf("u takes the p back: %q", curText(m))
	}
	typ(m, "u")
	if curText(m) != "whello| orld" {
		t.Fatalf("u again takes the undo back, as in vi: %q", curText(m))
	}
	typ(m, "uP")
	if curText(m) != "hello| world" {
		t.Fatalf("P puts before the cursor: %q", curText(m))
	}
	normalAt(m, "ab", 0)
	typ(m, "xp")
	if curText(m) != "b|a" {
		t.Fatalf("xp swaps: %q", curText(m))
	}
	// The kill buffer outlives an empty delete and the input.
	typ(m, "dd")
	typ(m, "0x") // an empty input: nothing to delete
	if string(m.vi.kill) != "ba" {
		t.Fatalf("an empty delete keeps the kill buffer: %q", string(m.vi.kill))
	}
	typ(m, "p")
	if curText(m) != "b|a" {
		t.Fatalf("p into the empty input: %q", curText(m))
	}
	// u takes back what an Insert typed along with the key that began it.
	normalAt(m, "abc", 1)
	typ(m, "iXY")
	press(m, tea.KeyEsc)
	typ(m, "u")
	if curText(m) != "a|bc" {
		t.Fatalf("u after iXY Esc: %q", curText(m))
	}
	normalAt(m, "one two", 4)
	typ(m, "dd")
	typ(m, "u")
	if curText(m) != "one |two" {
		t.Fatalf("u after dd: %q", curText(m))
	}
	normalAt(m, "cat", 0)
	typ(m, "rbu")
	if curText(m) != "|cat" {
		t.Fatalf("u after r: %q", curText(m))
	}
	// Nothing to undo, nothing to put: nothing happens.
	normalAt(m, "abc", 1)
	typ(m, "upP")
	if curText(m) != "a|bc" {
		t.Fatalf("u, p and P with nothing: %q", curText(m))
	}
	// lazygram's own buffer: the clipboard is never touched.
	m.clipMode = clipOSC52
	normalAt(m, "secret", 0)
	for _, k := range "dduxP" {
		if cmd := func() tea.Cmd { _, c := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{k}}); return c }(); cmd != nil {
			t.Fatalf("%c returns a command: the clipboard is not used", k)
		}
	}
	if m.clipSeq != "" || m.kept != nil {
		t.Fatal("a kill never reaches the clipboard")
	}
}

func TestVimScrollKeys(t *testing.T) {
	f := newFake()
	for i := range 60 {
		f.msgs = append(f.msgs, mk("c1", "x"+itoa(i), "ada", "busy line "+itoa(i), 10+i))
	}
	m := setup(t, f) // window 0 interleaves every chat: 60 lines and more
	enter(m, "/vim on")
	press(m, tea.KeyEsc)
	w := m.wins[0]
	h := m.contentHeight()
	half, page := h/2, h-1
	for _, c := range []struct {
		key  tea.KeyType
		want int
	}{
		{tea.KeyCtrlY, 1}, {tea.KeyCtrlY, 2}, {tea.KeyCtrlE, 1}, {tea.KeyCtrlE, 0}, {tea.KeyCtrlE, 0},
		{tea.KeyCtrlU, half}, {tea.KeyCtrlU, 2 * half}, {tea.KeyCtrlD, half}, {tea.KeyCtrlD, 0},
		{tea.KeyCtrlB, page}, {tea.KeyCtrlB, 2 * page}, {tea.KeyCtrlF, page}, {tea.KeyCtrlF, 0},
	} {
		press(m, c.key)
		if w.scroll != c.want || len(m.input) != 0 { // as the key left it, before a frame clamps it
			t.Fatalf("%v: scroll %d, want %d", c.key, w.scroll, c.want)
		}
		m.View()
	}
	if page != max(m.contentHeight()-1, 1) {
		t.Fatal("a page is what PgUp scrolls")
	}
	typ(m, "gg")
	m.View()
	home := setup(t, f)
	press(home, tea.KeyHome)
	home.View()
	if w.scroll == 0 || w.scroll != home.wins[0].scroll {
		t.Fatalf("gg: the oldest line, as Home: %d, Home %d", w.scroll, home.wins[0].scroll)
	}
	typ(m, "G")
	if w.scroll != 0 {
		t.Fatalf("G: the newest: %d", w.scroll)
	}
}

// TestVimCtrlKeysInsertAndNormal: Ctrl-U, Ctrl-E and the other scrolling
// keys have their vi meaning only in Normal; in Insert they are what they
// were.
func TestVimCtrlKeysInsertAndNormal(t *testing.T) {
	m := vimChat(t)
	w := m.wins[m.cur]
	typ(m, "draft")
	press(m, tea.KeyCtrlU)
	if len(m.input) != 0 || w.scroll != 0 {
		t.Fatalf("Insert: Ctrl-U clears the input: %q scroll %d", string(m.input), w.scroll)
	}
	typ(m, "draft")
	press(m, tea.KeyCtrlA)
	press(m, tea.KeyCtrlE)
	if curText(m) != "draft|" || w.scroll != 0 {
		t.Fatalf("Insert: Ctrl-E is the line end: %q", curText(m))
	}
	for _, k := range []tea.KeyType{tea.KeyCtrlD, tea.KeyCtrlY, tea.KeyCtrlF, tea.KeyCtrlB} {
		press(m, k)
		if curText(m) != "draft|" || w.scroll != 0 {
			t.Fatalf("Insert: %v does nothing, as before: %q scroll %d", k, curText(m), w.scroll)
		}
	}
	press(m, tea.KeyEsc)
	press(m, tea.KeyCtrlU)
	if string(m.input) != "draft" || w.scroll != max(m.contentHeight()/2, 1) {
		t.Fatalf("Normal: Ctrl-U scrolls half a page, the input stays: %q scroll %d", string(m.input), w.scroll)
	}
	typ(m, "0")
	press(m, tea.KeyCtrlE)
	if curText(m) != "|draft" || w.scroll != max(m.contentHeight()/2, 1)-1 {
		t.Fatalf("Normal: Ctrl-E scrolls a line, the cursor stays: %q scroll %d", curText(m), w.scroll)
	}
	// The Ctrl keys Normal does not take keep their meaning.
	press(m, tea.KeyCtrlK)
	if len(m.input) != 0 {
		t.Fatalf("Normal: Ctrl-K deletes to the line end: %q", string(m.input))
	}
}

// keptState is what a key that keeps its meaning can change: a keyState,
// the notes of the current window (Ctrl-L drops them) and whether the key
// gave a command (Ctrl-R syncs, F1 prints the help).
type keptState struct {
	keyState
	Notes int
	Cmd   bool
}

func keptStateOf(m *Model, cmd tea.Cmd) keptState {
	return keptState{stateOf(m), len(m.wins[m.cur].notes), cmd != nil}
}

// TestVimNormalKeepsTheOtherKeys: the keys Normal mode does not take do
// in Normal what they do with vim off, compared on two models from an
// empty input and from "one two" with the cursor on the t, and Normal
// stays. Tab and Ctrl-W take the character under the cursor first
// (TestVimTabAndCtrlWAsAppend).
func TestVimNormalKeepsTheOtherKeys(t *testing.T) {
	for _, text := range []string{"", "one two"} {
		for _, keys := range [][]tea.KeyType{
			{tea.KeyShiftTab}, {tea.KeyPgUp}, {tea.KeyPgUp, tea.KeyPgDown}, {tea.KeyHome}, {tea.KeyHome, tea.KeyEnd},
			{tea.KeyCtrlA}, {tea.KeyLeft}, {tea.KeyRight}, {tea.KeyBackspace}, {tea.KeyDelete},
			{tea.KeyCtrlL}, {tea.KeyCtrlR}, {tea.KeyF1},
		} {
			off, on := chatOn(t, 100, 30), vimChat(t)
			normalAt(on, text, min(4, len(text)))
			off.input, off.curBack = []rune(text), on.curBack
			off.history, off.histIdx = slices.Clone(on.history), on.histIdx // /vim on is one more line
			for _, m := range []*Model{off, on} {
				m.note(m.wins[m.cur], "a note")
				m.status, m.isErr = "", false
			}
			var a, b keptState
			for _, k := range keys {
				a, b = keptStateOf(off, press(off, k)), keptStateOf(on, press(on, k))
			}
			if !reflect.DeepEqual(a, b) || !on.vi.normal {
				t.Errorf("%q, %v: with vim off %+v\nin Normal  %+v (normal=%v)", text, keys, a, b, on.vi.normal)
			}
		}
	}
}

// TestVimTabAndCtrlWAsAppend: in Normal Tab and Ctrl-W work on the text up
// to and including the character under the cursor, as a, the key and Esc
// do in vi. Compared with vim off and its cursor after that character,
// every press leaves the same text, notes, status and window, and
// Normal's cursor one character back within its line; want is the input
// in Normal at the end.
func TestVimTabAndCtrlWAsAppend(t *testing.T) {
	tab, ctrlW := tea.KeyTab, tea.KeyCtrlW
	for _, c := range []struct {
		text string
		p    int // the character under Normal's cursor
		keys []tea.KeyType
		want string
	}{
		{"hi @ali", 6, []tea.KeyType{tab}, "hi @alice| "},
		{"hi @ali", 6, []tea.KeyType{tab, tab}, "hi @alice| "}, // the second Tab, on an empty word, does nothing
		{"/wi", 2, []tea.KeyType{tab, tab}, "/w|i"},            // the second Tab lists the commands
		{"hi @ali there", 6, []tea.KeyType{tab}, "hi @alice|  there"},
		{"hi @ali there", 5, []tea.KeyType{tab}, "hi @alice| i there"},
		{"hello world", 10, []tea.KeyType{ctrlW}, "hello| "},
		{"hello world", 8, []tea.KeyType{ctrlW}, "hello| ld"},
		{"hello world", 10, []tea.KeyType{ctrlW, ctrlW}, "|"},
		{"hello", 0, []tea.KeyType{ctrlW}, "|ello"},
		{"ab\ncd", 3, []tea.KeyType{ctrlW}, "|d"},
		{"ab\n", 3, []tea.KeyType{ctrlW}, "|"}, // an empty line has no character to take
		{"ab\n", 3, []tea.KeyType{tab}, "ab\n|"},
		{"ab\n\ncd", 3, []tea.KeyType{ctrlW}, "|\ncd"}, // an empty line inside: the break after it stays
		{"ab\n\ncd", 3, []tea.KeyType{tab}, "ab\n|\ncd"},
		{"", 0, []tea.KeyType{ctrlW}, "|"},
		{"", 0, []tea.KeyType{tab}, "|"}, // an empty prompt: Tab walks the windows
	} {
		off, on := chatOn(t, 100, 30), vimChat(t)
		normalAt(on, c.text, c.p)
		after := c.p // past the character under the cursor, within its line
		if after < len(c.text) && c.text[after] != '\n' {
			after++
		}
		off.input, off.curBack = []rune(c.text), len(c.text)-after
		off.history, off.histIdx = slices.Clone(on.history), on.histIdx // /vim on is one more line
		for _, m := range []*Model{off, on} {
			m.status, m.isErr = "", false
		}
		for i, k := range c.keys {
			a, b := keptStateOf(off, press(off, k)), keptStateOf(on, press(on, k))
			if p := a.Pos; p > 0 && c.text != "" && string(off.input)[p-1] != '\n' {
				a.Pos-- // the step back of Esc, within the line
			}
			if !reflect.DeepEqual(a, b) || !on.vi.normal {
				t.Fatalf("%q on %d, key %d (%v): with vim off, stepped back %+v\nin Normal %+v (normal=%v)",
					c.text, c.p, i, k, a, b, on.vi.normal)
			}
		}
		if curText(on) != c.want {
			t.Errorf("%q on %d, %v: %q, want %q", c.text, c.p, c.keys, curText(on), c.want)
		}
	}
	// A character is a grapheme cluster: Ctrl-W takes the whole e with its
	// accent, and Tab steps back onto the whole emoji, skin tone and all.
	m := vimChat(t)
	normalAt(m, "café", 3)
	press(m, ctrlW)
	if curText(m) != "|" || !m.vi.normal {
		t.Fatalf("Ctrl-W on an accented e: %q", curText(m))
	}
	normalAt(m, "hi \U0001F44D\U0001F3FD", 3)
	press(m, tab)
	if curText(m) != "hi |\U0001F44D\U0001F3FD" || !m.vi.normal {
		t.Fatalf("Tab on an emoji with a skin tone: %q", curText(m))
	}
}

func TestVimGtGTSwitchWindows(t *testing.T) {
	m := vimChat(t) // window 3 of 0..3
	press(m, tea.KeyEsc)
	for _, c := range []struct {
		keys string
		want int
	}{{"gt", 0}, {"gt", 1}, {"gT", 0}, {"gT", 3}, {"gT", 2}} {
		typ(m, c.keys)
		if m.cur != c.want || !m.vi.normal {
			t.Fatalf("%s: window %d, want %d", c.keys, m.cur, c.want)
		}
	}
	normalAt(m, "draft", 2) // with text typed too, unlike Tab
	typ(m, "gt")
	if m.cur != 3 || curText(m) != "dr|aft" {
		t.Fatalf("gt with a draft: cur=%d %q", m.cur, curText(m))
	}
}

func TestVimColonSlashQuestion(t *testing.T) {
	m := vimChat(t)
	for _, c := range []struct{ key, want string }{{":", "/|"}, {"/", "/search |"}, {"?", "/search |"}} {
		normalAt(m, "", 0)
		typ(m, c.key)
		if curText(m) != c.want || m.vi.normal {
			t.Fatalf("%s in an empty input: %q normal=%v", c.key, curText(m), m.vi.normal)
		}
		normalAt(m, "draft", 1)
		typ(m, c.key)
		if curText(m) != "draft|" || m.vi.normal {
			t.Fatalf("%s with a draft: Insert at its end, nothing overwritten: %q", c.key, curText(m))
		}
	}
	normalAt(m, "", 0)
	typ(m, ":stats")
	press(m, tea.KeyEnter)
	if v := ansi.Strip(m.View()); !strings.Contains(v, "-!- stats: fake") {
		t.Fatalf(":stats runs /stats:\n%s", v)
	}
	normalAt(m, "", 0)
	typ(m, "/hi")
	press(m, tea.KeyEnter)
	if m.mode != modePager || string(m.input) != "hi" {
		t.Fatalf("/hi opens the pager on hi: %v %q", m.mode, string(m.input))
	}
}

// TestVimNormalNeverInsertsText: no typed character that Normal does not
// map changes the input, and neither does one after the first key of a
// two-key command it does not complete; nothing is sent.
func TestVimNormalNeverInsertsText(t *testing.T) {
	f := newFake()
	m := setup(t, f)
	enter(m, "/win @alice")
	enter(m, "/vim on")
	const mapped = "hjklwbe0^$iaIAoOxXDCSdcgrupPG:/?"
	var unmapped []rune
	for r := rune(' '); r <= '~'; r++ {
		if !strings.ContainsRune(mapped, r) {
			unmapped = append(unmapped, r)
		}
	}
	unmapped = append(unmapped, []rune("\u00e9\u0436\u0416\u20ac\u00a0")...)
	sent := len(f.sent)
	check := func(what string) {
		t.Helper()
		if curText(m) != "ab|c def" || !m.vi.normal || m.cur != 3 || m.mode != modeWin || m.vi.op != 0 || m.pending != nil {
			t.Fatalf("%s: %q normal=%v cur=%d mode=%v op=%q", what, curText(m), m.vi.normal, m.cur, m.mode, m.vi.op)
		}
	}
	for _, r := range unmapped {
		normalAt(m, "abc def", 2)
		typ(m, string(r))
		check(string(r))
		for _, first := range "dcg" {
			if first == 'g' && (r == 't' || r == 'T') { // gt and gT switch windows
				continue
			}
			normalAt(m, "abc def", 2)
			typ(m, string(first)+string(r))
			check(string(first) + string(r))
		}
		if r < '1' || r > '9' { // Alt-1 .. Alt-9 switch windows
			alt(m, r)
			check("Alt-" + string(r))
		}
	}
	normalAt(m, "abc def", 2)
	typ(m, coder) // a run of several runes, one character
	check("an emoji")
	if len(f.sent) != sent {
		t.Fatalf("nothing is sent: %q", f.sent[sent:])
	}
}

// TestVimRunsOfKeysAndPaste: several characters that arrive as one key
// are read one by one in Normal mode; a paste goes in as in Insert, and
// Insert takes a run as it always did.
func TestVimRunsOfKeysAndPaste(t *testing.T) {
	m := vimChat(t)
	normalAt(m, "foo bar", 0)
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("dw")})
	if curText(m) != "|bar" {
		t.Fatalf("dw as one key: %q", curText(m))
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ixy")})
	if curText(m) != "xy|bar" || m.vi.normal {
		t.Fatalf("i then xy as one key: Insert takes the rest: %q", curText(m))
	}
	normalAt(m, "ab", 1)
	paste(m, "XY")
	if curText(m) != "aXY|b" || !m.vi.normal {
		t.Fatalf("a paste in Normal goes in: %q normal=%v", curText(m), m.vi.normal)
	}
}

// TestVimSendingKeepsTheMode: Enter and Ctrl-X keep their meaning in
// Normal mode, and sending leaves the mode as it was.
func TestVimSendingKeepsTheMode(t *testing.T) {
	f := newFake()
	m := setup(t, f)
	enter(m, "/win @alice")
	enter(m, "/vim on")
	typ(m, "hello")
	press(m, tea.KeyEsc)
	press(m, tea.KeyCtrlX)
	_, cmd := m.Update(sendLaterMsg{seq: m.pendingSeq}) // the double Ctrl-X window has passed
	runAll(m, cmd)
	if !slices.Contains(f.sent, "c2:hello") || len(m.input) != 0 || !m.vi.normal {
		t.Fatalf("Ctrl-X in Normal sends; Normal stays: sent %q input %q normal=%v", f.sent, string(m.input), m.vi.normal)
	}
	normalAt(m, "/stats", 0)
	n := m.cmdsN
	press(m, tea.KeyEnter)
	if m.cmdsN != n+1 || !m.vi.normal || len(m.input) != 0 {
		t.Fatalf("Enter in Normal runs the command; Normal stays: %d normal=%v", m.cmdsN, m.vi.normal)
	}
	m.wins[m.cur].enterSends = true
	normalAt(m, "hi there", 3)
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	runAll(m, cmd)
	if !slices.Contains(f.sent, "c2:hi there") || !m.vi.normal {
		t.Fatalf("Enter sends in Normal: %q normal=%v", f.sent, m.vi.normal)
	}
	m.wins[m.cur].enterSends = false
	normalAt(m, "line", 1)
	press(m, tea.KeyEnter) // Ctrl-X sends here: Enter adds a line, as in Insert
	if string(m.input) != "line\n" || !m.vi.normal {
		t.Fatalf("Enter adds a line in Normal: %q", string(m.input))
	}
	// Insert stays Insert across a send too.
	typ(m, "i")
	m.input, m.curBack = []rune("bye"), 0
	m.lastCtrlX = time.Time{} // a single Ctrl-X, not the second of a double
	press(m, tea.KeyCtrlX)
	if m.vi.normal || m.pending == nil {
		t.Fatalf("Ctrl-X in Insert: Insert stays: normal=%v", m.vi.normal)
	}
}

// TestVimCtrlXLineBreakAsInInsert: with "Enter sends" on, the line break
// a Ctrl-X adds in Normal goes after the character under the cursor,
// where Insert's cursor stood before Esc, so the input is what Insert
// gives, and Ctrl-X twice leaves the draft as typed and toggles.
func TestVimCtrlXLineBreakAsInInsert(t *testing.T) {
	m := vimChat(t)
	w := m.wins[m.cur]
	for _, c := range []struct {
		esc   bool
		twice bool
		want  string
		sends bool
	}{
		{false, false, "hello\n|", true},
		{true, false, "hello\n|", true},
		{false, true, "hello|", false},
		{true, true, "hell|o", false}, // the same text; Normal's cursor stands on its last character
	} {
		w.enterSends, m.lastCtrlX, m.vi.normal = true, time.Time{}, false
		m.input, m.curBack = nil, 0
		typ(m, "hello")
		if c.esc {
			press(m, tea.KeyEsc)
		}
		press(m, tea.KeyCtrlX)
		if !c.twice && m.status != "line added; Enter sends the message" {
			t.Fatalf("Esc %v: one Ctrl-X adds a line: %q", c.esc, m.status)
		}
		if c.twice {
			press(m, tea.KeyCtrlX)
		}
		if curText(m) != c.want || w.enterSends != c.sends || m.vi.normal != c.esc || m.pending != nil {
			t.Fatalf("Esc %v, twice %v: %q enterSends=%v normal=%v; want %q enterSends=%v",
				c.esc, c.twice, curText(m), w.enterSends, m.vi.normal, c.want, c.sends)
		}
	}
	// Elsewhere in the line the break follows the character under the
	// cursor, as Ctrl-X at Insert's cursor after a does.
	w.enterSends, m.lastCtrlX = true, time.Time{}
	normalAt(m, "hello", 1)
	press(m, tea.KeyCtrlX)
	if curText(m) != "he\n|llo" || !m.vi.normal {
		t.Fatalf("Ctrl-X on the e: %q", curText(m))
	}
}

// TestVimNormalTag: the status line says NORMAL, styled as its segments,
// only while the input is in Normal mode.
func TestVimNormalTag(t *testing.T) {
	trueColour(t)
	m := vimChat(t)
	if strings.Contains(statusRow(m), "NORMAL") {
		t.Fatal("no tag in Insert")
	}
	press(m, tea.KeyEsc)
	if row := statusRow(m); !strings.Contains(row, "[3:@alice]  NORMAL  ") {
		t.Fatalf("the tag after the window's: %q", row)
	}
	if line := m.statusLine(); !strings.Contains(line, seg("NORMAL", 4, true)) {
		t.Fatalf("styled like the other segments: %q", line)
	}
	typ(m, "a")
	if strings.Contains(statusRow(m), "NORMAL") {
		t.Fatal("gone in Insert again")
	}
	press(m, tea.KeyEsc)
	typ(m, ":vim off")
	press(m, tea.KeyEnter)
	if strings.Contains(statusRow(m), "NORMAL") || m.vimTag() != "" {
		t.Fatal("no tag with vim off")
	}
}

// keyState is what a key can change, for comparing two models.
type keyState struct {
	Input   string
	Pos     int
	Cur     int
	Mode    mode
	Scroll  []int
	Status  string
	IsErr   bool
	Hist    []string
	HistIdx int
}

func stateOf(m *Model) keyState {
	s := keyState{Input: string(m.input), Pos: m.pos(), Cur: m.cur, Mode: m.mode, Status: m.status, IsErr: m.isErr,
		Hist: slices.Clone(m.history), HistIdx: m.histIdx}
	for _, w := range m.wins {
		s.Scroll = append(s.Scroll, w.scroll)
	}
	return s
}

// todaysKeys are keys with their effect from "one two" with the cursor
// on the t, in @alice's window, as lazygram had them before vim mode.
var todaysKeys = []struct {
	name string
	keys []tea.KeyMsg
	want func(s *keyState)
}{
	{"Esc clears", []tea.KeyMsg{{Type: tea.KeyEsc}}, func(s *keyState) { s.Input, s.Pos = "", 0 }},
	{"Esc 2 switches", []tea.KeyMsg{{Type: tea.KeyEsc}, {Type: tea.KeyRunes, Runes: []rune{'2'}}},
		func(s *keyState) { s.Pos, s.Cur, s.Status = 7, 2, "" }},
	{"Ctrl-U clears", []tea.KeyMsg{{Type: tea.KeyCtrlU}}, func(s *keyState) { s.Input, s.Pos = "", 0 }},
	{"Ctrl-E line end", []tea.KeyMsg{{Type: tea.KeyCtrlE}}, func(s *keyState) { s.Pos = 7 }},
	{"Ctrl-A line start", []tea.KeyMsg{{Type: tea.KeyCtrlA}}, func(s *keyState) { s.Pos = 0 }},
	{"Ctrl-W word before", []tea.KeyMsg{{Type: tea.KeyCtrlW}}, func(s *keyState) { s.Input, s.Pos = "two", 0 }},
	{"Ctrl-K to the end", []tea.KeyMsg{{Type: tea.KeyCtrlK}}, func(s *keyState) { s.Input = "one " }},
	{"Ctrl-D nothing", []tea.KeyMsg{{Type: tea.KeyCtrlD}}, func(*keyState) {}},
	{"Ctrl-Y nothing", []tea.KeyMsg{{Type: tea.KeyCtrlY}}, func(*keyState) {}},
	{"Ctrl-F nothing", []tea.KeyMsg{{Type: tea.KeyCtrlF}}, func(*keyState) {}},
	{"Ctrl-B nothing", []tea.KeyMsg{{Type: tea.KeyCtrlB}}, func(*keyState) {}},
	{"Left", []tea.KeyMsg{{Type: tea.KeyLeft}}, func(s *keyState) { s.Pos = 3 }},
	{"Right", []tea.KeyMsg{{Type: tea.KeyRight}}, func(s *keyState) { s.Pos = 5 }},
	{"Backspace", []tea.KeyMsg{{Type: tea.KeyBackspace}}, func(s *keyState) { s.Input, s.Pos = "onetwo", 3 }},
	{"Delete", []tea.KeyMsg{{Type: tea.KeyDelete}}, func(s *keyState) { s.Input = "one wo" }},
	{"Up recalls", []tea.KeyMsg{{Type: tea.KeyUp}}, func(s *keyState) {
		s.Input, s.Pos, s.HistIdx = s.Hist[len(s.Hist)-1], len(s.Hist[len(s.Hist)-1]), len(s.Hist)-1
	}},
	{"Down keeps the draft", []tea.KeyMsg{{Type: tea.KeyDown}}, func(*keyState) {}},
	{"Home", []tea.KeyMsg{{Type: tea.KeyHome}}, func(s *keyState) { s.Scroll[3] = 1 << 30 }},
	{"Space types", []tea.KeyMsg{{Type: tea.KeySpace}}, func(s *keyState) { s.Input, s.Pos = "one  two", 5 }},
	{"Enter adds a line", []tea.KeyMsg{{Type: tea.KeyEnter}}, func(s *keyState) {
		s.Input, s.Pos, s.Status = "one two\n", 5, "line added; Ctrl-X sends the message" // at the end; the cursor keeps its distance from it
	}},
	{"Alt-1", []tea.KeyMsg{{Type: tea.KeyRunes, Runes: []rune{'1'}, Alt: true}}, func(s *keyState) { s.Cur, s.Status = 1, "" }},
	{"Ctrl-N", []tea.KeyMsg{{Type: tea.KeyCtrlN}}, func(s *keyState) { s.Cur, s.Status = 0, "" }},
	{"a run types", []tea.KeyMsg{{Type: tea.KeyRunes, Runes: []rune("dw")}}, func(s *keyState) { s.Input, s.Pos = "one dwtwo", 6 }},
}

// TestVimOffKeepsTodaysKeys: with vim off, from the start or turned off
// again from Normal mode, every key does what it did before vim mode, and
// every typed character types itself.
func TestVimOffKeepsTodaysKeys(t *testing.T) {
	fresh := func() *Model { return vimChat(t) }
	for _, how := range []string{"never on", "turned off from Normal"} {
		start := func() *Model {
			m := chatOn(t, 100, 30)
			if how == "turned off from Normal" {
				m = fresh()
				press(m, tea.KeyEsc)
				typ(m, ":vim off")
				press(m, tea.KeyEnter)
				if m.vi.on {
					t.Fatal("off")
				}
			}
			m.input, m.curBack = []rune("one two"), 3
			m.status, m.isErr = "", false
			return m
		}
		for _, c := range todaysKeys {
			m := start()
			want := stateOf(m)
			c.want(&want)
			for _, k := range c.keys {
				m.Update(k)
			}
			if got := stateOf(m); !reflect.DeepEqual(got, want) {
				t.Errorf("%s: %s:\n got %+v\nwant %+v", how, c.name, got, want)
			}
		}
		for r := rune('!'); r <= '~'; r++ {
			m := start()
			typ(m, string(r))
			if want := "one " + string(r) + "|two"; curText(m) != want || m.vi.normal {
				t.Errorf("%s: %c: %q, want %q", how, r, curText(m), want)
			}
		}
	}
}

// TestVimInsertIsTodaysKeys: in Insert every key but Esc does exactly
// what it does with vim off, compared key for key on two models.
func TestVimInsertIsTodaysKeys(t *testing.T) {
	off, on := chatOn(t, 30, 30), chatOn(t, 30, 30)
	on.setVim("on")
	keys := []tea.KeyMsg{}
	for _, r := range "hjkl x:/?0GgduipPa" {
		if r == ' ' {
			keys = append(keys, tea.KeyMsg{Type: tea.KeySpace})
		} else {
			keys = append(keys, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		}
	}
	for _, kt := range []tea.KeyType{tea.KeyCtrlA, tea.KeyCtrlE, tea.KeyCtrlD, tea.KeyCtrlY, tea.KeyCtrlF, tea.KeyCtrlB,
		tea.KeyLeft, tea.KeyLeft, tea.KeyRight, tea.KeyBackspace, tea.KeyDelete, tea.KeyCtrlW, tea.KeyEnter,
		tea.KeyUp, tea.KeyDown, tea.KeyUp, tea.KeyHome, tea.KeyPgDown, tea.KeyEnd, tea.KeyPgUp, tea.KeyCtrlK, tea.KeyCtrlU,
		tea.KeyTab, tea.KeyTab, tea.KeyShiftTab, tea.KeyCtrlN, tea.KeyCtrlP, tea.KeyCtrlL, tea.KeyCtrlR} {
		keys = append(keys, tea.KeyMsg{Type: kt})
	}
	keys = append(keys, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("one\ntwo"), Paste: true},
		tea.KeyMsg{Type: tea.KeyUp}, tea.KeyMsg{Type: tea.KeyUp}, tea.KeyMsg{Type: tea.KeyDown},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("xyz")}, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}, Alt: true},
		tea.KeyMsg{Type: tea.KeyF1}, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	for i, k := range keys {
		off.Update(k)
		on.Update(k)
		a, b := stateOf(off), stateOf(on)
		a.Status, b.Status = "", "" // /vim on said something the other did not
		if !reflect.DeepEqual(a, b) || on.vi.normal {
			t.Fatalf("key %d %v: with vim off %+v\nin Insert %+v", i, k, a, b)
		}
	}
}

// busyFake is newFake with 40 more chats, so /list is longer than a page,
// and 60 more lines in #dev, so window 0 and its pager are too.
func busyFake() *fake {
	f := newFake()
	for i := range 40 {
		f.chats = append(f.chats, domain.Chat{ID: "r" + itoa(i), Alias: "room" + itoa(i), Title: "Room " + itoa(i), Kind: domain.ChatGroup})
	}
	for i := range 60 {
		f.msgs = append(f.msgs, mk("c1", "x"+itoa(i), "ada", "busy line "+itoa(i), 10+i))
	}
	return f
}

// vimOn is setup with vim on, in window 0, its input in Insert.
func vimOn(t *testing.T, f *fake) *Model {
	t.Helper()
	m := setup(t, f)
	enter(m, "/vim on")
	if !m.vi.on || m.vi.normal || m.mode != modeWin {
		t.Fatalf("vim on, in Insert: on=%v normal=%v mode=%v", m.vi.on, m.vi.normal, m.mode)
	}
	return m
}

// ruleRow is the rule of the last frame, for a one-row input.
func ruleRow(m *Model) string {
	rows := strings.Split(ansi.Strip(m.View()), "\n")
	return rows[len(rows)-4]
}

// TestVimListsOpenInNormal: with vim on /list, /win, /settings and the
// pager open in Normal mode, say so (NORMAL, "/ to narrow"), and i, then
// Esc, then Esc go to Insert, back to Normal and close; the window's own
// mode is as it was.
func TestVimListsOpenInNormal(t *testing.T) {
	m := vimOn(t, newFake())
	for _, c := range []struct{ line, normal, insert string }{
		{"/list", "chats (3): Up/Down, Left/Right pick open/read/unread, Enter does it, Esc returns, / to narrow",
			"chats (3): Up/Down, Left/Right pick open/read/unread, Enter does it, Esc returns, type to narrow"},
		{"/win", "windows (4): Up/Down, Left/Right pick focus/read/unread/close, Enter does it, Esc returns",
			"windows (4): Up/Down, Left/Right pick focus/read/unread/close, Enter does it, Esc returns"},
		{"/settings", "settings (11): Up/Down, Enter or Right next value, Left previous, Esc closes, / to narrow",
			"settings (11): Up/Down, Enter or Right next value, Left previous, Esc closes, type to narrow"},
		{"/search", "history of server: 4 of 4 messages; / to search, Esc returns",
			"history of server: 4 of 4 messages; type to search, Esc returns"},
	} {
		enter(m, c.line)
		if m.mode == modeWin || !m.listNormal() || m.vi.listIns {
			t.Fatalf("%s opens in Normal: mode=%v ins=%v", c.line, m.mode, m.vi.listIns)
		}
		if rule, status := ruleRow(m), statusRow(m); !strings.Contains(rule, c.normal) || !strings.Contains(status, "[0:server]  NORMAL  ") {
			t.Fatalf("%s in Normal: rule %q status %q", c.line, rule, status)
		}
		typ(m, "i")
		if rule, status := ruleRow(m), statusRow(m); m.listNormal() || !strings.Contains(rule, c.insert) || strings.Contains(status, "NORMAL") {
			t.Fatalf("%s: i enters Insert: rule %q status %q", c.line, rule, status)
		}
		press(m, tea.KeyEsc)
		if m.mode == modeWin || !m.listNormal() || !strings.Contains(statusRow(m), "NORMAL") {
			t.Fatalf("%s: Esc in Insert returns to Normal, open: mode=%v", c.line, m.mode)
		}
		press(m, tea.KeyEsc)
		if m.mode != modeWin || m.vi.normal || len(m.input) != 0 || strings.Contains(statusRow(m), "NORMAL") {
			t.Fatalf("%s: Esc in Normal closes; the window is in Insert as before: mode=%v normal=%v", c.line, m.mode, m.vi.normal)
		}
	}
}

// TestVimListJKAndEnds: j and k are Down and Up, gg and G the first and
// the last row; in the pager they scroll, gg as Home, G as End.
func TestVimListJKAndEnds(t *testing.T) {
	m := vimOn(t, busyFake())
	for _, c := range []struct {
		line string
		n    int // the rows
	}{{"/list", 43}, {"/list room1", 10}, {"/win", 4}, {"/settings", 11}, {"/settings mo", 2}} {
		enter(m, c.line)
		m.View()
		if m.listCur != 0 || m.listLen() != c.n { // /win opens on window 0, the current one
			t.Fatalf("%s: cursor %d of %d rows", c.line, m.listCur, m.listLen())
		}
		filter := string(m.input)
		for i := 1; i <= 3; i++ {
			typ(m, "j")
			m.View() // a frame keeps Down within the rows
			if m.listCur != min(i, c.n-1) {
				t.Fatalf("%s: j %d times: row %d", c.line, i, m.listCur)
			}
		}
		typ(m, "k")
		if m.listCur != min(3, c.n-1)-1 {
			t.Fatalf("%s: k: %d", c.line, m.listCur)
		}
		typ(m, "G")
		if m.listCur != c.n-1 {
			t.Fatalf("%s: G is the last row: %d of %d", c.line, m.listCur, c.n)
		}
		typ(m, "gg")
		if m.listCur != 0 {
			t.Fatalf("%s: gg is the first: %d", c.line, m.listCur)
		}
		typ(m, "k")
		if m.listCur != 0 || string(m.input) != filter || !m.listNormal() {
			t.Fatalf("%s: k on the first row stays; nothing typed: %d %q", c.line, m.listCur, string(m.input))
		}
		press(m, tea.KeyEsc)
	}
	// The pager, key for key with the arrows, Home and End.
	keys, vi := setup(t, busyFake()), vimOn(t, busyFake())
	enter(keys, "/search")
	enter(vi, "/search")
	for _, c := range []struct {
		vim string
		key tea.KeyType
	}{{"k", tea.KeyUp}, {"k", tea.KeyUp}, {"k", tea.KeyUp}, {"j", tea.KeyDown}, {"gg", tea.KeyHome}, {"j", tea.KeyDown}, {"G", tea.KeyEnd}, {"j", tea.KeyDown}} {
		typ(vi, c.vim)
		press(keys, c.key)
		vi.View()
		keys.View()
		if a, b := vi.wins[0].scroll, keys.wins[0].scroll; a != b || len(vi.input) != 0 || vi.mode != modePager {
			t.Fatalf("pager %s: scroll %d, %v gives %d; input %q", c.vim, a, c.key, b, string(vi.input))
		}
	}
	if vi.wins[0].scroll != 0 {
		t.Fatal("the pager is at the bottom")
	}
	typ(vi, "gg")
	vi.View()
	if vi.wins[0].scroll < 40 {
		t.Fatalf("gg: the top of the pager: %d", vi.wins[0].scroll)
	}
}

// TestVimListHL: h and l are Left and Right: the action button in /list
// and /win, the setting's value in /settings, the search's cursor in the
// pager.
func TestVimListHL(t *testing.T) {
	m := vimOn(t, newFake())
	enter(m, "/list")
	for _, c := range []struct {
		keys string
		act  int
	}{{"l", 1}, {"l", 2}, {"l", 2}, {"h", 1}, {"hh", 0}} {
		typ(m, c.keys)
		if m.listAct != c.act || len(m.input) != 0 {
			t.Fatalf("/list %s: button %d, want %d; input %q", c.keys, m.listAct, c.act, string(m.input))
		}
	}
	press(m, tea.KeyEsc)
	enter(m, "/win")
	typ(m, "llll")
	if m.listAct != 3 {
		t.Fatalf("/win: the close button: %d", m.listAct)
	}
	press(m, tea.KeyEsc)
	enter(m, "/settings")
	typ(m, "jjjjj") // fx
	if m.fx {
		t.Fatal("the lights start off")
	}
	typ(m, "l")
	if !m.fx || menuValue(m, "fx") != "on" || !m.settingsOpen() {
		t.Fatalf("l: the next value: fx=%v", m.fx)
	}
	typ(m, "h")
	if m.fx || !m.settingsOpen() {
		t.Fatalf("h: the previous value: fx=%v", m.fx)
	}
	press(m, tea.KeyEsc)
	enter(m, "/search line")
	typ(m, "hh")
	if curText(m) != "li|ne" || m.mode != modePager {
		t.Fatalf("pager hh: the search's cursor: %q", curText(m))
	}
	typ(m, "l")
	if curText(m) != "lin|e" {
		t.Fatalf("pager l: %q", curText(m))
	}
}

// TestVimListCtrlDU: in Normal Ctrl-D and Ctrl-U move half a page within
// the rows (the pager scrolls); in Insert they are the usual keys.
func TestVimListCtrlDU(t *testing.T) {
	m := vimOn(t, busyFake())
	half := max(m.contentHeight()/2, 1)
	enter(m, "/list")
	n := m.listLen()
	for _, c := range []struct {
		key  tea.KeyType
		want int
	}{{tea.KeyCtrlD, half}, {tea.KeyCtrlD, 2 * half}, {tea.KeyCtrlU, half}, {tea.KeyCtrlU, 0}, {tea.KeyCtrlU, 0},
		{tea.KeyCtrlD, half}, {tea.KeyCtrlD, 2 * half}, {tea.KeyCtrlD, 3 * half}, {tea.KeyCtrlD, 4 * half}, {tea.KeyCtrlD, 5 * half},
		{tea.KeyCtrlD, n - 1}, {tea.KeyCtrlD, n - 1}, {tea.KeyCtrlU, n - 1 - half}} {
		press(m, c.key) // no frame between: the key clamps by itself
		if m.listCur != c.want || len(m.input) != 0 {
			t.Fatalf("%v: row %d, want %d (half %d, %d rows)", c.key, m.listCur, c.want, half, n)
		}
	}
	// Insert: Ctrl-U clears the filter, Ctrl-D does nothing, as with vim off.
	typ(m, "/r")
	m.listCur = 3
	press(m, tea.KeyCtrlD)
	if m.listCur != 3 || string(m.input) != "r" {
		t.Fatalf("Insert Ctrl-D: row %d input %q", m.listCur, string(m.input))
	}
	press(m, tea.KeyCtrlU)
	if m.listCur != 3 || len(m.input) != 0 || m.mode != modeList {
		t.Fatalf("Insert Ctrl-U clears the filter: row %d input %q", m.listCur, string(m.input))
	}
	press(m, tea.KeyEsc)
	press(m, tea.KeyEsc)
	// The pager scrolls half a page: Ctrl-U toward the oldest, Ctrl-D back.
	enter(m, "/search")
	w := m.wins[0]
	w.scroll = 0
	for _, c := range []struct {
		key  tea.KeyType
		want int
	}{{tea.KeyCtrlU, half}, {tea.KeyCtrlU, 2 * half}, {tea.KeyCtrlD, half}, {tea.KeyCtrlD, 0}, {tea.KeyCtrlD, 0}} {
		press(m, c.key)
		if w.scroll != c.want || len(m.input) != 0 {
			t.Fatalf("pager %v: scroll %d, want %d", c.key, w.scroll, c.want)
		}
	}
}

// TestVimListEnterQAndEsc: Enter acts as with vim off; q closes without
// arming Esc then a digit, Esc closes and arms it; the window's mode is
// what it was before the list opened.
func TestVimListEnterQAndEsc(t *testing.T) {
	m := vimOn(t, newFake())
	enter(m, "/list")
	typ(m, "j")
	press(m, tea.KeyEnter)
	if m.mode != modeWin || m.winName(m.cur) != "@alice" {
		t.Fatalf("j, Enter opens @alice: mode=%v %s", m.mode, m.winName(m.cur))
	}
	enter(m, "/win")
	typ(m, "lll") // [close]
	press(m, tea.KeyEnter)
	if len(m.wins) != 3 || m.mode != modeList {
		t.Fatalf("Enter runs the chosen button: %d windows, mode %v", len(m.wins), m.mode)
	}
	typ(m, "q")
	if m.mode != modeWin || m.escPending || len(m.input) != 0 {
		t.Fatalf("q closes, no digit armed: mode=%v esc=%v", m.mode, m.escPending)
	}
	cur := m.cur
	if cur == 1 { // a 1 typed below must name another window than the current one
		t.Fatalf("current window %d", cur)
	}
	typ(m, "2")
	if m.cur != cur || string(m.input) != "2" {
		t.Fatalf("after q a digit types, in Insert: cur=%d %q", m.cur, string(m.input))
	}
	m.input, m.curBack = nil, 0
	enter(m, "/list")
	typ(m, "/ali")
	press(m, tea.KeyEsc)
	typ(m, "q")
	if m.mode != modeWin || len(m.input) != 0 || m.escPending {
		t.Fatalf("q drops the filter, as Esc does: mode=%v %q", m.mode, string(m.input))
	}
	// From Normal: the window is in Normal again when the list closes.
	m.input, m.curBack = []rune("/list"), 0
	press(m, tea.KeyEsc)
	press(m, tea.KeyEnter) // Enter runs the line in Normal too
	if !m.listNormal() || !m.vi.normal {
		t.Fatalf("the list in Normal, the window's Normal kept: %v %v", m.listNormal(), m.vi.normal)
	}
	typ(m, "q")
	if m.mode != modeWin || !m.vi.normal || !strings.Contains(statusRow(m), "NORMAL") {
		t.Fatalf("q: back to the window in Normal: mode=%v normal=%v", m.mode, m.vi.normal)
	}
	typ(m, "1")
	if m.cur != cur {
		t.Fatal("a digit after q switches nothing")
	}
	m.input, m.curBack = []rune("/settings"), 0
	press(m, tea.KeyEnter)
	press(m, tea.KeyEsc)
	if m.mode != modeWin || !m.escPending || !m.vi.normal {
		t.Fatalf("Esc closes and arms the digit: mode=%v esc=%v", m.mode, m.escPending)
	}
	typ(m, "1")
	if m.cur != 1 || !m.vi.normal {
		t.Fatalf("Esc 1: window 1: %d", m.cur)
	}
}

// TestVimListQAndSlashEverywhere: in every list and the pager q closes
// without arming a digit, and / enters Insert, where typing narrows the
// filter (the pager: edits the search).
func TestVimListQAndSlashEverywhere(t *testing.T) {
	m := vimOn(t, newFake())
	for _, line := range []string{"/list", "/win", "/settings", "/search"} {
		enter(m, line)
		open := m.mode
		typ(m, "q")
		if m.mode != modeWin || m.escPending || len(m.input) != 0 || m.cur != 0 || m.vi.normal {
			t.Fatalf("%s: q closes, no digit armed: mode=%v esc=%v %q", line, m.mode, m.escPending, string(m.input))
		}
		enter(m, line)
		typ(m, "/x")
		if m.mode != open || m.listNormal() || string(m.input) != "x" {
			t.Fatalf("%s: / then x: Insert, x typed: mode=%v normal=%v %q", line, m.mode, m.listNormal(), string(m.input))
		}
		press(m, tea.KeyEsc)
		press(m, tea.KeyEsc)
		if m.mode != modeWin {
			t.Fatalf("%s: Esc Esc closes: %v", line, m.mode)
		}
	}
}

// TestVimListInsertNarrows: / and i enter the list's Insert mode, where
// typing narrows the filter; Esc returns to Normal with the filter kept,
// and a digit right after that Esc closes the list, dropping the filter,
// and switches windows, as Esc then a digit does with vim off.
func TestVimListInsertNarrows(t *testing.T) {
	m := vimOn(t, newFake())
	enter(m, "/list")
	typ(m, "al")
	if len(m.input) != 0 || len(m.listItems()) != 3 || !m.listNormal() {
		t.Fatalf("Normal: a and l narrow nothing: %q", string(m.input))
	}
	typ(m, "/ali")
	if string(m.input) != "ali" || len(m.listItems()) != 1 || m.listNormal() {
		t.Fatalf("/ then ali: %q, %d rows", string(m.input), len(m.listItems()))
	}
	press(m, tea.KeyEsc)
	if string(m.input) != "ali" || m.mode != modeList || !m.listNormal() {
		t.Fatalf("Esc: Normal, the filter kept: %q mode %v", string(m.input), m.mode)
	}
	typ(m, "xc")
	if string(m.input) != "ali" {
		t.Fatalf("Normal types nothing: %q", string(m.input))
	}
	m.curBack = 2
	typ(m, "ic")
	if curText(m) != "ac|li" {
		t.Fatalf("i: Insert at the cursor: %q", curText(m))
	}
	press(m, tea.KeyEsc)
	typ(m, "/e")
	if curText(m) != "aclie|" {
		t.Fatalf("/: Insert at the end: %q", curText(m))
	}
	press(m, tea.KeyEsc)
	press(m, tea.KeyEsc)
	if m.mode != modeWin || len(m.input) != 0 {
		t.Fatalf("a second Esc closes: mode=%v %q", m.mode, string(m.input))
	}
	enter(m, "/list")
	typ(m, "/ali")
	press(m, tea.KeyEsc)
	typ(m, "2")
	if m.cur != 2 || m.mode != modeWin || len(m.input) != 0 {
		t.Fatalf("Esc 2 from Insert: window 2, the list closed, no filter: cur=%d mode=%v %q", m.cur, m.mode, string(m.input))
	}
	enter(m, "/list")
	typ(m, "/x")
	press(m, tea.KeyEsc)
	typ(m, "9")
	if !m.isErr || m.cur != 2 || m.mode != modeWin || len(m.input) != 0 {
		t.Fatalf("Esc 9: no window 9, the list closed: cur=%d mode=%v %q %q", m.cur, m.mode, string(m.input), m.status)
	}
	// A digit not right after Esc does nothing in Normal.
	enter(m, "/list")
	typ(m, "1")
	if m.cur != 2 || m.mode != modeList || len(m.input) != 0 {
		t.Fatalf("1 in Normal: cur=%d mode=%v %q", m.cur, m.mode, string(m.input))
	}
}

// TestVimListColonAndCommands: : starts a command line in the filter; a
// command line leaves Insert, so the list it opens, or the one it leaves
// open, is in Normal.
func TestVimListColonAndCommands(t *testing.T) {
	m := vimOn(t, newFake())
	enter(m, "/settings")
	typ(m, ":")
	if curText(m) != "/|" || m.listNormal() {
		t.Fatalf(": in an empty filter: %q", curText(m))
	}
	typ(m, "feed off")
	press(m, tea.KeyEnter)
	if m.feed || !m.settingsOpen() || !m.listNormal() || len(m.input) != 0 {
		t.Fatalf(":feed off runs it; the menu stays, in Normal: feed=%v open=%v normal=%v", m.feed, m.settingsOpen(), m.listNormal())
	}
	typ(m, "/fe")
	press(m, tea.KeyEsc)
	m.curBack = 1
	typ(m, ":")
	if curText(m) != "fe|" || m.listNormal() {
		t.Fatalf(": with a filter: Insert at its end, nothing overwritten: %q", curText(m))
	}
	m.input, m.curBack = []rune("/nope"), 0
	press(m, tea.KeyEnter)
	if !m.isErr || !m.settingsOpen() || !m.listNormal() {
		t.Fatalf("an unknown command leaves Insert too: err=%v normal=%v", m.isErr, m.listNormal())
	}
	typ(m, "i")
	m.input, m.curBack = []rune("/list"), 0
	press(m, tea.KeyEnter)
	if m.mode != modeList || m.listSrc != listChats || !m.listNormal() {
		t.Fatalf("a list opened from another's Insert mode opens in Normal: src=%v normal=%v", m.listSrc, m.listNormal())
	}
	typ(m, "i")
	press(m, tea.KeyDown)
	press(m, tea.KeyEnter) // a row opened: Insert lasts until the next list opens
	if m.mode != modeWin {
		t.Fatalf("Enter opened the row: %v", m.mode)
	}
	enter(m, "/win")
	if !m.listNormal() {
		t.Fatal("the next list opens in Normal")
	}
}

// TestVimListNormalNeverTypes: in a list's or the pager's Normal mode no
// typed character that is not mapped, no pair g with another key and no
// Alt chord changes the filter, the selection or the view.
func TestVimListNormalNeverTypes(t *testing.T) {
	f := newFake()
	m := vimOn(t, f)
	const mapped = "jkhlgGqi/:"
	var unmapped []rune
	for r := rune(' '); r <= '~'; r++ {
		if !strings.ContainsRune(mapped, r) {
			unmapped = append(unmapped, r)
		}
	}
	unmapped = append(unmapped, []rune("\u00e9\u0436\u20ac\u00a0")...)
	sent := len(f.sent)
	for _, line := range []string{"/list", "/win", "/settings", "/search"} {
		enter(m, line)
		mode, src := m.mode, m.listSrc
		reset := func() {
			m.mode, m.listSrc, m.listCur, m.listAct, m.input, m.curBack = mode, src, 1, 1, nil, 0
			m.vi.listIns, m.vi.op, m.wins[0].scroll = false, 0, 1
		}
		check := func(what string) {
			t.Helper()
			if m.mode != mode || m.listSrc != src || m.listCur != 1 || m.listAct != 1 || len(m.input) != 0 ||
				m.vi.listIns || m.vi.op != 0 || m.wins[0].scroll != 1 || m.cur != 0 || m.vi.normal {
				t.Fatalf("%s %s: mode=%v cur=%d act=%d input=%q ins=%v op=%q scroll=%d", line, what, m.mode, m.listCur, m.listAct,
					string(m.input), m.vi.listIns, m.vi.op, m.wins[0].scroll)
			}
		}
		for _, r := range unmapped {
			reset()
			typ(m, string(r))
			check(string(r))
			if r != 'g' {
				reset()
				typ(m, "g"+string(r))
				check("g" + string(r))
			}
			if r < '0' || r > '9' {
				reset()
				alt(m, r)
				check("Alt-" + string(r))
			}
		}
		for _, r := range mapped {
			reset()
			alt(m, r)
			check("Alt-" + string(r))
			if r != 'g' { // g then a mapped key is no pair either: both keys do nothing
				reset()
				typ(m, "g"+string(r))
				check("g" + string(r))
			}
		}
		press(m, tea.KeyEsc)
	}
	if len(f.sent) != sent {
		t.Fatalf("nothing is sent: %q", f.sent[sent:])
	}
}

// TestVimListRunsAndPaste: keys that arrive as one run in a list's Normal
// mode are read one at a time; a paste narrows the filter, as with vim off.
func TestVimListRunsAndPaste(t *testing.T) {
	m := vimOn(t, newFake())
	enter(m, "/list")
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("jj")})
	if m.listCur != 2 || len(m.input) != 0 {
		t.Fatalf("jj as one key: row %d input %q", m.listCur, string(m.input))
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("gg")})
	if m.listCur != 0 {
		t.Fatalf("gg as one key: row %d", m.listCur)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/al")})
	if string(m.input) != "al" || m.listNormal() {
		t.Fatalf("/al as one key: Insert takes the rest: %q", string(m.input))
	}
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ic")})
	if string(m.input) != "alic" {
		t.Fatalf("Insert takes a run as it always did: %q", string(m.input))
	}
	press(m, tea.KeyEsc)
	m.input, m.curBack = nil, 0
	paste(m, "new")
	if string(m.input) != "new" || !m.listNormal() || len(m.listItems()) != 1 {
		t.Fatalf("a paste in Normal narrows: %q normal=%v", string(m.input), m.listNormal())
	}
}

// TestVimSettingsMenuTurnsVimOn: the vim row turns vim on in the open
// menu, which is in Normal at once; Left, Right and Enter go on stepping.
func TestVimSettingsMenuTurnsVimOn(t *testing.T) {
	m := setup(t, newFake())
	openAt(t, m, "vim")
	press(m, tea.KeyEnter)
	if !m.vi.on || !m.listNormal() || !m.settingsOpen() {
		t.Fatalf("on: the menu in Normal: on=%v normal=%v", m.vi.on, m.listNormal())
	}
	typ(m, "kh")                                                    // clip, its previous value
	if m.listCur != 7 || m.clipMode != "off" || len(m.input) != 0 { // auto wraps back to off
		t.Fatalf("k h: clip %q row %d input %q", m.clipMode, m.listCur, string(m.input))
	}
	typ(m, "jl")
	if m.vi.on || m.listNormal() || !m.settingsOpen() {
		t.Fatal("j l: vim off again; the menu's keys are the usual ones")
	}
	typ(m, "k")
	if string(m.input) != "k" {
		t.Fatalf("with vim off typing narrows: %q", string(m.input))
	}
}

// listState is what a key can change in a list or the pager.
type listState struct {
	keyState
	ListCur, ListAct int
	Src              listSrc
	EscPending       bool
}

func listStateOf(m *Model) listState {
	return listState{stateOf(m), m.listCur, m.listAct, m.listSrc, m.escPending}
}

// TestVimOffListsKeepTodaysKeys: with vim off, never on or turned off
// again, the lists' keys are what they were before vim mode: every typed
// character narrows, Ctrl-U clears the filter, Esc closes.
func TestVimOffListsKeepTodaysKeys(t *testing.T) {
	for _, how := range []string{"never on", "turned off"} {
		start := func() *Model {
			m := setup(t, newFake())
			if how == "turned off" {
				enter(m, "/vim on")
				enter(m, "/list")
				typ(m, ":vim off")
				press(m, tea.KeyEnter)
				press(m, tea.KeyEsc)
				if m.vi.on {
					t.Fatal("off")
				}
			}
			enter(m, "/list e") // all three chats
			m.listCur = 1
			m.status, m.isErr, m.escPending = "", false, false
			return m
		}
		for _, c := range []struct {
			name string
			keys []tea.KeyMsg
			want func(s *listState)
		}{
			{"Space types", []tea.KeyMsg{{Type: tea.KeySpace}}, func(s *listState) { s.Input, s.Pos = "e ", 2 }},
			{"Ctrl-U clears", []tea.KeyMsg{{Type: tea.KeyCtrlU}}, func(s *listState) { s.Input, s.Pos = "", 0 }},
			{"Ctrl-D nothing", []tea.KeyMsg{{Type: tea.KeyCtrlD}}, func(*listState) {}},
			{"Down", []tea.KeyMsg{{Type: tea.KeyDown}}, func(s *listState) { s.ListCur = 2 }},
			{"Up", []tea.KeyMsg{{Type: tea.KeyUp}}, func(s *listState) { s.ListCur = 0 }},
			{"Right", []tea.KeyMsg{{Type: tea.KeyRight}}, func(s *listState) { s.ListAct = 1 }},
			{"Esc closes", []tea.KeyMsg{{Type: tea.KeyEsc}}, func(s *listState) { s.Mode, s.Input, s.Pos, s.EscPending = modeWin, "", 0, true }},
			{"Esc 2", []tea.KeyMsg{{Type: tea.KeyEsc}, {Type: tea.KeyRunes, Runes: []rune{'2'}}},
				func(s *listState) { s.Mode, s.Input, s.Pos, s.Cur = modeWin, "", 0, 2 }},
			{"Alt-x types", []tea.KeyMsg{{Type: tea.KeyRunes, Runes: []rune{'x'}, Alt: true}}, func(s *listState) { s.Input, s.Pos, s.ListCur = "ex", 2, 0 }},
			{"a run types", []tea.KeyMsg{{Type: tea.KeyRunes, Runes: []rune("gg")}}, func(s *listState) { s.Input, s.Pos, s.ListCur = "egg", 3, 0 }},
			{"a paste types", []tea.KeyMsg{{Type: tea.KeyRunes, Runes: []rune("a\nb"), Paste: true}}, func(s *listState) { s.Input, s.Pos, s.ListCur = "eab", 3, 0 }},
			{"Enter opens", []tea.KeyMsg{{Type: tea.KeyEnter}}, func(s *listState) { s.Mode, s.Input, s.Pos, s.Cur = modeWin, "", 0, 3 }},
		} {
			m := start()
			want := listStateOf(m)
			c.want(&want)
			for _, k := range c.keys {
				m.Update(k)
			}
			if got := listStateOf(m); !reflect.DeepEqual(got, want) {
				t.Errorf("%s: %s:\n got %+v\nwant %+v", how, c.name, got, want)
			}
		}
		for r := rune('!'); r <= '~'; r++ {
			m := start()
			typ(m, string(r))
			if string(m.input) != "e"+string(r) || m.listCur != 0 || m.mode != modeList || m.vimTag() != "" {
				t.Errorf("%s: %c narrows: %q row %d mode %v", how, r, string(m.input), m.listCur, m.mode)
			}
		}
		m := start()
		if v := ruleRow(m); !strings.Contains(v, "type to narrow") || strings.Contains(statusRow(m), "NORMAL") {
			t.Errorf("%s: the rule says type to narrow: %q", how, v)
		}
	}
}

// settingValues is every setting's value but vim's, name=value, in the
// menu's order: what Enter and Left or Right in the menu change.
func settingValues(m *Model) string {
	var out []string
	for _, s := range settings {
		if s.key != vimKey {
			out = append(out, s.name+"="+s.value(m))
		}
	}
	return strings.Join(out, " ")
}

// TestVimListNormalKeepsTheOtherKeys: in a list's or the pager's Normal
// mode the keys it does not take do what they do with vim off, compared
// key for key on two models, and Normal stays while it is open.
func TestVimListNormalKeepsTheOtherKeys(t *testing.T) {
	for _, line := range []string{"/list e", "/list al", "/win", "/settings", "/search line"} {
		for _, keys := range [][]tea.KeyType{
			{tea.KeyPgUp}, {tea.KeyPgUp, tea.KeyPgDown}, {tea.KeyHome}, {tea.KeyHome, tea.KeyEnd},
			{tea.KeyTab}, {tea.KeyShiftTab}, {tea.KeyLeft}, {tea.KeyRight}, {tea.KeyDown}, {tea.KeyDown, tea.KeyUp},
			{tea.KeyBackspace}, {tea.KeyCtrlW}, {tea.KeyCtrlA}, {tea.KeyCtrlA, tea.KeyCtrlE}, {tea.KeyCtrlA, tea.KeyDelete},
			{tea.KeyCtrlA, tea.KeyCtrlK}, {tea.KeyEnter}, {tea.KeyCtrlL}, {tea.KeyCtrlR}, {tea.KeyF1},
		} {
			off, on := setup(t, newFake()), vimOn(t, newFake())
			off.history, off.histIdx = slices.Clone(on.history), on.histIdx // /vim on is one more line
			for _, m := range []*Model{off, on} {
				enter(m, line)
				m.note(m.wins[m.cur], "a note")
				m.status, m.isErr, m.escPending = "", false, false
			}
			if !on.listNormal() {
				t.Fatalf("%s opens in Normal", line)
			}
			type state struct {
				listState
				Notes    int
				Cmd      bool
				Settings string
			}
			var a, b state
			for _, k := range keys {
				ca, cb := press(off, k), press(on, k)
				a = state{listStateOf(off), len(off.wins[off.cur].notes), ca != nil, settingValues(off)}
				b = state{listStateOf(on), len(on.wins[on.cur].notes), cb != nil, settingValues(on)}
			}
			if !reflect.DeepEqual(a, b) || (on.mode != modeWin && !on.listNormal()) {
				t.Errorf("%s, %v: with vim off %+v\nin Normal  %+v", line, keys, a, b)
			}
		}
	}
}

// TestVimListInsertIsTodaysKeys: in a list's or the pager's Insert mode
// every key but Esc does exactly what it does with vim off, compared key
// for key on two models.
func TestVimListInsertIsTodaysKeys(t *testing.T) {
	var keys []tea.KeyMsg
	for _, r := range "jkhlgGqi/:x0 " {
		if r == ' ' {
			keys = append(keys, tea.KeyMsg{Type: tea.KeySpace})
		} else {
			keys = append(keys, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		}
	}
	for _, kt := range []tea.KeyType{tea.KeyLeft, tea.KeyRight, tea.KeyRight, tea.KeyDown, tea.KeyDown, tea.KeyUp,
		tea.KeyCtrlA, tea.KeyCtrlE, tea.KeyBackspace, tea.KeyDelete, tea.KeyCtrlW, tea.KeyCtrlD, tea.KeyCtrlY,
		tea.KeyCtrlF, tea.KeyCtrlB, tea.KeyPgUp, tea.KeyPgDown, tea.KeyHome, tea.KeyEnd, tea.KeyTab, tea.KeyCtrlK,
		tea.KeyCtrlL, tea.KeyCtrlX, tea.KeyShiftTab} {
		keys = append(keys, tea.KeyMsg{Type: kt})
	}
	keys = append(keys, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}, Alt: true}, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("ab")},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c\nd"), Paste: true}, tea.KeyMsg{Type: tea.KeyCtrlU},
		tea.KeyMsg{Type: tea.KeyTab}, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}}, tea.KeyMsg{Type: tea.KeyEnter})
	// F1 closes the list (the help goes to window 0), so it has a run of
	// its own, after one key that types.
	for _, keys := range [][]tea.KeyMsg{keys, {{Type: tea.KeyRunes, Runes: []rune{'e'}}, {Type: tea.KeyF1}}} {
		for _, line := range []string{"/list", "/win", "/settings", "/search"} {
			off, on := setup(t, busyFake()), setup(t, busyFake())
			on.setVim("on")
			enter(off, line)
			enter(on, line)
			typ(on, "i")
			off.status, on.status = "", ""
			for i, k := range keys {
				off.Update(k)
				on.Update(k)
				if a, b := listStateOf(off), listStateOf(on); !reflect.DeepEqual(a, b) || (on.mode != modeWin && !on.vi.listIns) {
					t.Fatalf("%s key %d %v: with vim off %+v\nin Insert %+v", line, i, k, a, b)
				}
			}
		}
	}
}

// TestVimHelpAndKeysDescribeVimMode: /keys and /help name vim mode, and a
// start hint offers it.
func TestVimHelpAndKeysDescribeVimMode(t *testing.T) {
	printed := func(line string) string {
		m := setup(t, newFake())
		enter(m, line)
		drain(m)
		var out []string
		for _, n := range m.wins[0].notes {
			out = append(out, n.text)
		}
		return strings.Join(out, "\n")
	}
	keys, help := printed("/keys"), printed("/help")
	for _, want := range []string{
		"\n  vim: Normal mode       /vim on: Esc enters it, i a I A o O leave it; typed keys are commands, never text",
		"h l w b e 0 ^ $ move (Esc 0 is still window 0), j k as Down / Up, x X D dd dw cw cc S C r edit, u undoes, p P put back",
		"Ctrl-E Y D U F B and gg G scroll; gt gT switch windows; : / ? a command, a search",
		"\n  vim: lists, pager      /vim on: they open in Normal: j k move, h l as Left / Right, gg G ends, Ctrl-D / Ctrl-U half a page, q closes; / or i to type, Esc stops",
	} {
		if !strings.Contains(keys, want) {
			t.Errorf("/keys lacks %q", want)
		}
	}
	if want := "\n  /vim [on|off]          vi keys: Normal and Insert modes on the input, in the lists and the pager"; !strings.Contains(help, want) {
		t.Errorf("/help lacks %q", want)
	}
	if !slices.Contains(hintPool, "hint: /vim on gives the input vi keys; Esc for Normal mode, i to type") {
		t.Error("no start hint for /vim")
	}
}
