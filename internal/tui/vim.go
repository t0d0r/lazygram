package tui

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
)

// Vim mode (/vim, off by default and kept; specs/ui.md, Vim mode) gives
// the input line vi's two modes. Insert is the usual keys except Esc,
// which enters Normal and keeps the text; Normal reads the typed
// characters as vi commands and never inserts one, so a stray key there
// cannot type or send anything. The keys Normal does not take (Enter,
// Ctrl-X, Tab, the arrows, Backspace, Home/End, PgUp/PgDn, the other Ctrl
// keys, a paste) do what they do in Insert, and Esc then a digit still
// switches windows, so 0 is the line start only when it does not follow
// an Esc. Each start and each /vim on begins in Insert, since a chat
// client is mostly typing. One level of everything: no counts, one kill
// buffer for p and P (lazygram's own, never the clipboard), one undo.
//
// The lists (/list, /win, /settings) and the pager have the two modes
// too, each its own: they open in Normal, where j and k move, h and l are
// Left and Right, gg and G go to the ends, Ctrl-D and Ctrl-U move half a
// page and q closes; / or i enter their Insert mode, where typing narrows
// the filter (the pager: edits the search) as with vim off, until Esc.
// The window's mode stays as it was while a list is open.
//
// key hands vim mode every key first (vimBegin: the first key of a
// two-key command waiting for it is taken, j and k become Down and Up, in
// a list h and l Left and Right too), then, once Ctrl-C and the window
// keys have had it, the rest (vimKey, vimList for the lists and the
// pager); vimSettle keeps Normal's cursor on a character when the key is
// done.

const vimKey = "vim" // store state key for vim mode; absent or unreadable means off

// vimState is vim mode's state.
type vimState struct {
	on      bool     // /vim: vi keys on the input, the lists and the pager (kept); off: the usual keys only
	normal  bool     // the input is in Normal mode; false is Insert
	listIns bool     // the open list or pager is in its Insert mode; false is Normal, as each one opens
	op      rune     // the first key of a two-key command waiting for its second: g, d, c or r; 0 none
	kill    []rune   // what the last delete took, for p and P
	undo    *vimSnap // the input before the last Normal-mode change, for u
}

// vimSnap is the input and its cursor at one moment.
type vimSnap struct {
	input   []rune
	curBack int
}

// setVim implements /vim: no argument shows the setting, on or off sets
// it, takes effect at once and keeps it. Either way the input is in
// Insert after it, with nothing waiting, so typing goes on as before.
func (m *Model) setVim(arg string) {
	switch strings.ToLower(strings.TrimSpace(arg)) {
	case "":
		if m.vi.on {
			m.setInfo("vim on: Esc enters Normal mode (vi keys), i returns to typing; /vim off")
		} else {
			m.setInfo("vim off: the input keys are the usual ones; /vim on for vi keys")
		}
	case "on":
		m.vi = vimState{on: true}
		_ = m.opt.Store.SetState(vimKey, true)
		m.setInfo("vim on: typing works as before; Esc enters Normal mode (vi keys), i returns to typing")
	case "off":
		m.vi = vimState{}
		_ = m.opt.Store.SetState(vimKey, false)
		m.setInfo("vim off: the input keys are the usual ones")
	default:
		m.setErr("/vim takes on or off")
	}
}

// inNormal reports whether keys go to Normal mode now: vim on, Normal,
// and a window's own input, not a list's filter or the pager's search.
func (m *Model) inNormal() bool { return m.vi.on && m.vi.normal && m.mode == modeWin }

// listNormal reports whether a list or the pager is open in its Normal
// mode, where typed keys move and act instead of narrowing it.
func (m *Model) listNormal() bool { return m.vi.on && m.mode != modeWin && !m.vi.listIns }

// vimTag is the status line's mode tag: NORMAL while the input, or the
// open list or pager, is in Normal mode; nothing in Insert or with vim
// off.
func (m *Model) vimTag() string {
	if m.inNormal() || m.listNormal() {
		return "NORMAL"
	}
	return ""
}

// vimTypeTo is how the rule of a list or the pager says to narrow it or
// search: "type to", or "/ to" in its Normal mode, where typing does not.
func (m *Model) vimTypeTo() string {
	if m.listNormal() {
		return "/ to"
	}
	return "type to"
}

// vimRune is the one character a key types, Space included, or false for
// any other key: a special key, an Alt chord, a run of several or a paste.
func vimRune(k tea.KeyMsg) (rune, bool) {
	switch {
	case k.Alt || k.Paste:
		return 0, false
	case k.Type == tea.KeySpace:
		return ' ', true
	case k.Type == tea.KeyRunes && len(k.Runes) == 1:
		return k.Runes[0], true
	}
	return 0, false
}

// The keys Normal mode reads as arrows, so they do exactly what those do:
// on the input j and k (Down and Up: between its rows, keeping the column,
// then through the history at the first and the last row); in a list h
// and l too (Left and Right: the action button, the setting's value; in
// the pager the search's cursor).
var (
	vimArrows     = map[rune]tea.KeyType{'j': tea.KeyDown, 'k': tea.KeyUp}
	vimListArrows = map[rune]tea.KeyType{'j': tea.KeyDown, 'k': tea.KeyUp, 'h': tea.KeyLeft, 'l': tea.KeyRight}
)

// vimBegin is vim mode's first look at a key, before key reads it. The
// first key of a two-key command waiting for this one is taken off and
// returned (any key ends the wait; vimKey reads this one as the second),
// and in Normal mode with nothing waiting the arrow letters become the
// arrows. A digit right after an Esc in a list's Insert mode closes the
// list before the digit switches windows: with vim off that Esc closed
// it, and its filter never follows into a window's input.
func (m *Model) vimBegin(k tea.KeyMsg) (tea.KeyMsg, rune) {
	op := m.vi.op
	m.vi.op = 0
	r, ok := vimRune(k)
	if ok && r >= '0' && r <= '9' && m.escPending && m.mode != modeWin {
		m.mode, m.input, m.curBack = modeWin, nil, 0
	}
	if ok && op == 0 {
		arrows := map[rune]tea.KeyType(nil)
		switch {
		case m.inNormal():
			arrows = vimArrows
		case m.listNormal():
			arrows = vimListArrows
		}
		if t, ok := arrows[r]; ok {
			k = tea.KeyMsg{Type: t}
		}
	}
	return k, op
}

// vimEach reads a run of typed characters that came as one key in Normal
// mode one key at a time, as vi would: the terminal reader groups what it
// finds waiting, so keys typed fast (or held) can arrive so, and "dw" must
// still delete a word. A paste is not split: it goes in as in Insert.
func (m *Model) vimEach(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	for _, r := range k.Runes {
		_, cmd := m.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
}

// vimSettle keeps the cursor on a character in Normal mode, as vi does: a
// key that left it past the text of a line (a recalled line, a delete at
// the end, l or $, the arrows) puts it back on the line's last character.
// An empty line has none, so there it stays.
func (m *Model) vimSettle() {
	if !m.inNormal() {
		return
	}
	if p := m.pos(); p == m.lineEnd(p) && p > m.lineStart(p) {
		m.vimMove(m.vimText().prev(p, m.lineStart(p)))
	}
}

// vimKey reads a key in vim mode once Ctrl-C and the window keys (Alt-N,
// Esc then a digit, Ctrl-N, Ctrl-P) have passed it by; op is the first key
// of a two-key command this one completes. It reports false for a key it
// leaves to the usual code: in Insert every key but Esc, in Normal every
// key that types nothing and is not Esc or a scrolling key. The lists and
// the pager have their own Normal mode (vimList).
func (m *Model) vimKey(k tea.KeyMsg, op rune) (bool, tea.Cmd) {
	if m.mode != modeWin {
		return m.vimList(k, op)
	}
	if !m.vi.normal {
		if k.Type != tea.KeyEsc {
			return false, nil
		}
		// Esc leaves Insert as in vi: the text stays and the cursor steps
		// back onto the character before it. A digit next still switches
		// windows, the input kept as it is (no escSaved: nothing cleared).
		m.vi.normal = true
		m.vimStepBack()
		m.escPending, m.escSaved = true, nil
		return true, nil
	}
	r, ok := vimRune(k)
	if !ok {
		return m.normalSpecial(k)
	}
	if op != 0 {
		return true, m.vimSecond(op, r)
	}
	return true, m.normalRune(r)
}

// normalSpecial reads a key that is not one typed character in Normal
// mode: Esc, the scrolling keys, Tab and Ctrl-W on text, and the Alt
// chords and runs that type nothing here; a paste and every other key go
// on to the usual code.
func (m *Model) normalSpecial(k tea.KeyMsg) (bool, tea.Cmd) {
	w := m.wins[m.cur]
	h := m.contentHeight()
	half, page := max(h/2, 1), max(h-1, 1) // a page as PgUp and PgDn scroll it
	switch k.Type {
	case tea.KeyEsc: // drops a two-key command waiting; a digit next switches windows
		m.escPending, m.escSaved = true, nil
	case tea.KeyTab, tea.KeyCtrlW:
		if len(m.input) == 0 {
			return false, nil // Tab on an empty prompt walks the windows
		}
		m.vimAppendKey(func() {
			if k.Type == tea.KeyTab {
				m.complete()
			} else {
				m.cutWord()
			}
		})
	case tea.KeyCtrlE: // one line toward the newest
		w.scroll = max(w.scroll-1, 0)
	case tea.KeyCtrlY: // one line toward the oldest
		w.scroll++
	case tea.KeyCtrlD:
		w.scroll = max(w.scroll-half, 0)
	case tea.KeyCtrlU:
		w.scroll += half
	case tea.KeyCtrlF:
		w.scroll = max(w.scroll-page, 0)
	case tea.KeyCtrlB:
		w.scroll += page
	case tea.KeyRunes, tea.KeySpace:
		if k.Paste {
			return false, nil
		}
		// An Alt chord: Normal types nothing (Alt-N switched windows already).
	default:
		return false, nil
	}
	return true, nil
}

// normalRune reads a typed character in Normal mode as a vi command; one
// it does not know does nothing, so Normal never types.
func (m *Model) normalRune(r rune) tea.Cmd {
	t := m.vimText()
	p := t.charAt(m.pos())
	ls, le := m.lineStart(p), m.lineEnd(p)
	switch r {
	case 'h':
		m.vimMove(t.prev(p, ls))
	case 'l':
		m.vimMove(t.next(p, le)) // vimSettle keeps it off the end
	case 'w':
		m.vimMove(t.wordNext(p))
	case 'b':
		m.vimMove(t.wordPrev(p))
	case 'e':
		m.vimMove(t.wordEnd(p))
	case '0':
		m.vimMove(ls)
	case '^':
		m.vimMove(m.firstNonBlank(p))
	case '$':
		m.vimMove(le)
	case 'i':
		m.vimInsert(p)
	case 'a':
		m.vimInsert(t.next(p, le))
	case 'I':
		m.vimInsert(m.firstNonBlank(p))
	case 'A':
		m.vimInsert(le)
	case 'o': // a new input row below, as a line break typed at the line's end
		m.vimInsert(le)
		m.insert('\n')
	case 'O':
		m.vimInsert(ls)
		m.insert('\n')
		m.vimMove(ls)
	case 'x':
		m.vimCut(p, t.next(p, le))
	case 'X':
		m.vimCut(t.prev(p, ls), p)
	case 'D':
		m.vimCut(p, le)
	case 'C':
		m.vimSave()
		m.vimCut(p, le)
		m.vi.normal = false
	case 'S':
		m.vimChange()
	case 'd', 'c', 'g', 'r':
		m.vi.op = r
	case 'u':
		m.vimUndo()
	case 'p':
		m.vimPut(t.next(p, le))
	case 'P':
		m.vimPut(p)
	case 'G': // the newest line: the end, as End
		m.wins[m.cur].scroll = 0
	case ':': // a command line
		m.vimLine("/")
	case '/', '?': // a search, in the pager
		m.vimLine("/search ")
	}
	return nil
}

// vimSecond completes a two-key command: op is its first key, r the
// second. A pair it does not know does nothing.
func (m *Model) vimSecond(op, r rune) tea.Cmd {
	t := m.vimText()
	p := t.charAt(m.pos())
	le := m.lineEnd(p)
	switch string([]rune{op, r}) {
	case "gt": // the next window, as Tab on an empty prompt
		return m.switchTo((m.cur + 1) % len(m.wins))
	case "gT":
		return m.switchTo((m.cur + len(m.wins) - 1) % len(m.wins))
	case "gg": // the oldest loaded line, as Home
		m.wins[m.cur].scroll = 1 << 30
	case "dd": // the whole input, the draft of several lines too
		m.vimCut(0, len(m.input))
	case "dw": // to the next word, the line break kept
		m.vimCut(p, min(t.wordNext(p), le))
	case "cc":
		m.vimChange()
	case "cw":
		// As in vi, on a word to its end, keeping the blank after it; on
		// blanks to the next word, as dw.
		to := min(t.wordNext(p), le)
		if t.classAt(p) != vimBlank {
			to = t.runEnd(p)
		}
		m.vimSave()
		m.vimCut(p, to)
		m.vi.normal = false
	default:
		if op == 'r' {
			m.vimReplace(p, t.next(p, le), r)
		}
	}
	return nil
}

// vimList reads a key in a list or the pager once the window keys have
// passed it by; op is a g waiting for its second. In Normal mode (vimBegin
// made j, k, h and l the arrows): gg and G go to the first and the last
// row (the pager: the oldest and the newest line), Ctrl-D and Ctrl-U move
// half a page, q closes as Esc does but arms no digit, since it is no Esc,
// i enters Insert at the cursor, / at the end of the filter, and : as well
// with "/" in an empty one, to type a command as in a window. Any other
// typed character does nothing. In Insert every key but Esc is the usual
// one, and Esc returns to Normal, the filter kept, arming Esc then a digit
// as Esc does everywhere. It reports false for a key it leaves to the
// usual code: Esc in Normal (it closes), Enter, a paste, the other keys.
func (m *Model) vimList(k tea.KeyMsg, op rune) (bool, tea.Cmd) {
	if m.vi.listIns {
		if k.Type != tea.KeyEsc {
			return false, nil
		}
		m.vi.listIns = false
		m.escPending, m.escSaved = true, nil
		return true, nil
	}
	r, ok := vimRune(k)
	if !ok {
		half := max(m.contentHeight()/2, 1)
		switch {
		case k.Type == tea.KeyCtrlD:
			m.vimListBy(half)
		case k.Type == tea.KeyCtrlU:
			m.vimListBy(-half)
		case (k.Type == tea.KeyRunes || k.Type == tea.KeySpace) && !k.Paste:
			// An Alt chord: Normal types nothing (Alt-N switched windows already).
		default:
			return false, nil
		}
		return true, nil
	}
	switch {
	case op == 'g':
		if r == 'g' {
			m.vimListEnd(true)
		}
	case r == 'g':
		m.vi.op = r
	case r == 'G':
		m.vimListEnd(false)
	case r == 'q':
		m.mode, m.input, m.curBack = modeWin, nil, 0
	case r == 'i':
		m.vi.listIns = true
	case r == '/':
		m.vi.listIns, m.curBack = true, 0
	case r == ':':
		m.vi.listIns, m.curBack = true, 0
		if len(m.input) == 0 {
			m.input = []rune("/")
		}
	}
	return true, nil
}

// vimListBy moves the list's selection d rows down (up when d < 0), within
// its rows; the pager scrolls d lines toward the newest (Down's way).
func (m *Model) vimListBy(d int) {
	if m.mode == modePager {
		w := m.wins[m.cur]
		w.scroll = max(w.scroll-d, 0)
		return
	}
	m.listCur = min(max(m.listCur+d, 0), max(m.listLen()-1, 0))
}

// vimListEnd selects the list's first row or its last; the pager goes to
// the top, the oldest line, as Home, or to the bottom, as End.
func (m *Model) vimListEnd(first bool) {
	switch {
	case m.mode == modePager && first:
		m.wins[m.cur].scroll = 1 << 30
	case m.mode == modePager:
		m.wins[m.cur].scroll = 0
	case first:
		m.listCur = 0
	default:
		m.listCur = max(m.listLen()-1, 0)
	}
}

// listLen is the number of rows the open list shows.
func (m *Model) listLen() int {
	switch m.listSrc {
	case listWindows:
		return len(m.wins)
	case listSettings:
		return len(m.settingRows())
	}
	return len(m.listItems())
}

// vimMove puts the cursor before rune p.
func (m *Model) vimMove(p int) { m.curBack = len(m.input) - p }

// vimBreakPoint readies the cursor for the line break Ctrl-X adds with
// "Enter sends" on. In Normal mode it goes after the character under the
// cursor, where Insert's cursor stood before Esc stepped back onto it, so
// a draft left with Esc gets the break at its end, as in Insert, and a
// fast second Ctrl-X finds it there and removes it. Elsewhere the break
// goes in at the cursor, as with vim off.
func (m *Model) vimBreakPoint() {
	if m.inNormal() {
		m.vimPastChar()
	}
}

// vimAppendKey runs key, the usual code of a key that works on the text
// before the cursor (Tab completes its last word, Ctrl-W deletes it), as
// a, that key and Esc do in vi. Normal's cursor stands on a character,
// after Esc the last one typed, so that character is part of the text
// the key works on: "hi @ali", Esc, Tab completes @ali as Insert does.
// The cursor then steps back onto a character, and a second Tab, which
// steps past it again, finds the text where the first left it.
func (m *Model) vimAppendKey(key func()) {
	m.vimPastChar()
	key()
	m.vimStepBack()
}

// vimPastChar puts the cursor after the character under it, within its
// line, where a puts Insert's cursor; on an empty line it stays.
func (m *Model) vimPastChar() {
	t := m.vimText()
	p := t.charAt(m.pos())
	m.vimMove(t.next(p, m.lineEnd(p)))
}

// vimStepBack puts the cursor on the character before it, within its
// line, as Esc does on leaving Insert; at a line's start it stays.
func (m *Model) vimStepBack() {
	if p := m.pos(); p > m.lineStart(p) {
		m.vimMove(m.vimText().prev(p, m.lineStart(p)))
	}
}

// vimSave keeps the input as it is now for u: the next change is the one
// u takes back.
func (m *Model) vimSave() {
	m.vi.undo = &vimSnap{input: append([]rune(nil), m.input...), curBack: m.curBack}
}

// vimInsert enters Insert with the cursor before rune at. What the Insert
// types is part of the change u takes back.
func (m *Model) vimInsert(at int) {
	m.vimSave()
	m.vimMove(at)
	m.vi.normal = false
}

// vimCut deletes the runes [from, to) into the kill buffer as a change u
// takes back, and leaves the cursor at from. Nothing to delete changes
// nothing, the kill buffer and u's input included.
func (m *Model) vimCut(from, to int) {
	if from >= to {
		return
	}
	m.vimSave()
	m.vi.kill = append([]rune(nil), m.input[from:to]...)
	m.cut(from, to)
	m.vimMove(from)
}

// vimChange clears the whole input into the kill buffer and enters Insert
// (cc and S).
func (m *Model) vimChange() {
	m.vimSave()
	m.vimCut(0, len(m.input))
	m.vi.normal = false
}

// vimReplace puts r in place of the character [p, to) (vi's r); on an
// empty line, or for a character that is not printable, nothing.
func (m *Model) vimReplace(p, to int, r rune) {
	if p >= to || r < ' ' || r == 0x7f {
		return
	}
	m.vimSave()
	m.input = append(m.input[:p], append([]rune{r}, m.input[to:]...)...)
	m.vimMove(p)
}

// vimPut puts the kill buffer before rune at and leaves the cursor on its
// last character, as vi's p (at: after the cursor's character) and P (at:
// the cursor) do. An empty kill buffer puts nothing.
func (m *Model) vimPut(at int) {
	if len(m.vi.kill) == 0 {
		return
	}
	m.vimSave()
	m.vimMove(at)
	m.insert(m.vi.kill...)
	m.vimMove(m.vimText().charAt(at + len(m.vi.kill) - 1))
}

// vimUndo takes back the last Normal-mode change; a second u takes back
// the undo, as in vi, where u is one level.
func (m *Model) vimUndo() {
	if m.vi.undo == nil {
		return
	}
	was := m.vi.undo
	m.vimSave()
	m.input, m.curBack = was.input, was.curBack
}

// vimLine enters Insert at the end of the input to type a command: text
// ("/", or "/search ") in an empty input; one with text keeps it as it is.
func (m *Model) vimLine(text string) {
	m.vimInsert(len(m.input))
	if len(m.input) == 0 {
		m.input, m.curBack = []rune(text), 0
	}
}

// firstNonBlank is the first character of the line holding p that is not
// a blank, or the line's end when it is all blanks (vi's ^ and I).
func (m *Model) firstNonBlank(p int) int {
	s, e := m.lineStart(p), m.lineEnd(p)
	for s < e && unicode.IsSpace(m.input[s]) {
		s++
	}
	return s
}

// The classes of characters for the word motions, as in vi: a word is a
// run of letters, digits and underscores, or a run of other characters
// that are not blank; blanks (line breaks too) separate them.
const (
	vimBlank = iota
	vimWord
	vimOther
)

// vimText is the input's characters as the input area draws them
// (grapheme clusters, so an emoji or a letter with an accent is one; a
// line break is one): where each starts and its class, and the input's
// length after the last.
type vimText struct {
	at, class []int
	n         int
}

func (m *Model) vimText() vimText {
	t := vimText{n: len(m.input)}
	at := 0
	for _, g := range glyphs(string(m.input)) {
		r, _ := utf8.DecodeRuneInString(g.text)
		c := vimOther
		switch {
		case unicode.IsSpace(r):
			c = vimBlank
		case r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r):
			c = vimWord
		}
		t.at, t.class = append(t.at, at), append(t.class, c)
		at += utf8.RuneCountInString(g.text)
	}
	return t
}

// index is the index of the character holding rune p, len(at) at the end
// of the input.
func (t vimText) index(p int) int {
	if p >= t.n {
		return len(t.at)
	}
	return max(sort.SearchInts(t.at, p+1)-1, 0)
}

// start is where character i starts, the input's length past the last.
func (t vimText) start(i int) int {
	if i >= len(t.at) {
		return t.n
	}
	return t.at[max(i, 0)]
}

// charAt is the start of the character holding rune p.
func (t vimText) charAt(p int) int { return t.start(t.index(p)) }

// classAt is the class of the character holding rune p; the end of the
// input counts as a blank.
func (t vimText) classAt(p int) int {
	if i := t.index(p); i < len(t.at) {
		return t.class[i]
	}
	return vimBlank
}

// next and prev are the starts of the characters after and before the one
// at p, never past end or before start, the ends of p's line.
func (t vimText) next(p, end int) int { return min(t.start(t.index(p)+1), end) }

func (t vimText) prev(p, start int) int {
	if i := t.index(p); i > 0 {
		return max(t.start(i-1), start)
	}
	return 0
}

// wordNext is where the next word starts after the one at p (vi's w),
// across line breaks; the end of the input when none follows.
func (t vimText) wordNext(p int) int {
	i, n := t.index(p), len(t.at)
	if i < n && t.class[i] != vimBlank {
		for c := t.class[i]; i < n && t.class[i] == c; i++ {
		}
	}
	for i < n && t.class[i] == vimBlank {
		i++
	}
	return t.start(i)
}

// wordPrev is where the word before p starts, or the one p is inside
// (vi's b).
func (t vimText) wordPrev(p int) int {
	i := t.index(p) - 1
	for i >= 0 && t.class[i] == vimBlank {
		i--
	}
	if i < 0 {
		return 0
	}
	for c := t.class[i]; i > 0 && t.class[i-1] == c; i-- {
	}
	return t.start(i)
}

// wordEnd is the last character of the word p is inside, or of the next
// one when p is on its end already (vi's e); the end of the input when no
// word follows.
func (t vimText) wordEnd(p int) int {
	i, n := t.index(p)+1, len(t.at)
	for i < n && t.class[i] == vimBlank {
		i++
	}
	if i >= n {
		return t.n
	}
	for c := t.class[i]; i+1 < n && t.class[i+1] == c; i++ {
	}
	return t.start(i)
}

// runEnd is where the run of characters of p's class that p is in ends.
func (t vimText) runEnd(p int) int {
	i, n := t.index(p), len(t.at)
	if i < n {
		for c := t.class[i]; i < n && t.class[i] == c; i++ {
		}
	}
	return t.start(i)
}
