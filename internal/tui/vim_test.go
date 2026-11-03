package tui

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
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
		{"привет мир", 0, "w", "привет |мир"},
		{"привет мир", 0, "e", "приве|т мир"},
		{"привет,мир", 0, "w", "привет|,мир"}, // Cyrillic letters are a word, the comma is not
		{"привет,мир", 0, "e", "приве|т,мир"},
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
	unmapped = append(unmapped, []rune("éжЖ€\u00a0")...)
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
		tea.KeyTab, tea.KeyTab, tea.KeyShiftTab, tea.KeyCtrlN, tea.KeyCtrlP} {
		keys = append(keys, tea.KeyMsg{Type: kt})
	}
	keys = append(keys, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("one\ntwo"), Paste: true},
		tea.KeyMsg{Type: tea.KeyUp}, tea.KeyMsg{Type: tea.KeyUp}, tea.KeyMsg{Type: tea.KeyDown},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("xyz")}, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}, Alt: true})
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
