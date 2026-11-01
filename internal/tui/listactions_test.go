package tui

import (
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestWinListActionsReadUnreadClose(t *testing.T) {
	m := setup(t, newFake())
	if m.wins[1].unread == 0 {
		t.Fatal("window 1 (#news) starts with unread messages")
	}
	enter(m, "/win")
	v := m.View()
	for _, want := range []string{"[focus]", "[read]", "[unread]", "[close]", "Left/Right pick focus/read/unread/close"} {
		if !strings.Contains(v, want) {
			t.Fatalf("the window list shows %q:\n%s", want, v)
		}
	}
	press(m, tea.KeyDown) // row 1: #news
	press(m, tea.KeyRight)
	if m.listAct != 1 || m.listCur != 1 {
		t.Fatalf("Right picks [read]: act=%d cur=%d", m.listAct, m.listCur)
	}
	press(m, tea.KeyEnter)
	if m.wins[1].unread != 0 || m.mode != modeList || m.listCur != 1 || m.listAct != 1 || !strings.Contains(m.status, "marked #news read") {
		t.Fatalf("read: unread=%d mode=%v cur=%d act=%d status=%q", m.wins[1].unread, m.mode, m.listCur, m.listAct, m.status)
	}
	press(m, tea.KeyRight)
	press(m, tea.KeyEnter)
	if m.wins[1].unread != 1 || !strings.Contains(m.activity(), "1") || !strings.Contains(m.status, "marked #news unread") {
		t.Fatalf("unread: unread=%d act=%q status=%q", m.wins[1].unread, m.activity(), m.status)
	}
	press(m, tea.KeyRight)
	press(m, tea.KeyRight) // past the last button: stays on [close]
	if m.listAct != 3 {
		t.Fatalf("clamped at the last button: %d", m.listAct)
	}
	n := len(m.wins)
	press(m, tea.KeyEnter)
	if len(m.wins) != n-1 || m.mode != modeList || m.listCur >= len(m.wins) || !strings.Contains(m.status, "closed #news") {
		t.Fatalf("close: wins=%d mode=%v cur=%d status=%q", len(m.wins), m.mode, m.listCur, m.status)
	}
	press(m, tea.KeyLeft)
	press(m, tea.KeyLeft)
	press(m, tea.KeyLeft)
	press(m, tea.KeyLeft) // past the first: stays on [focus]
	if m.listAct != 0 {
		t.Fatalf("clamped at the first button: %d", m.listAct)
	}
	cmd := press(m, tea.KeyEnter)
	if m.mode != modeWin || m.cur != 1 || cmd == nil {
		t.Fatalf("[focus] is the default: mode=%v cur=%d", m.mode, m.cur)
	}
	// The server window cannot be closed, from the list either.
	enter(m, "/win")
	press(m, tea.KeyUp)
	press(m, tea.KeyUp)
	n = len(m.wins)
	for i := 0; i < 3; i++ {
		press(m, tea.KeyRight)
	}
	press(m, tea.KeyEnter)
	if !m.isErr || len(m.wins) != n || m.mode != modeList {
		t.Fatalf("window 0 stays: err=%v wins=%d", m.isErr, len(m.wins))
	}
}

func TestListActionsReadUnreadKeepTheFilter(t *testing.T) {
	m := setup(t, newFake())
	enter(m, "/list new") // only #news matches; it has 4 unread
	if v := m.View(); !strings.Contains(v, "[open]") || !strings.Contains(v, "[unread]") || strings.Contains(v, "[close]") {
		t.Fatalf("the chat list has open, read and unread only:\n%s", v)
	}
	press(m, tea.KeyRight) // [read]
	press(m, tea.KeyEnter)
	c, _ := m.chat("c3")
	_, w := m.winFor("c3")
	if c.Unread != 0 || w == nil || w.unread != 0 || m.mode != modeList || !strings.Contains(m.status, "marked #news read") {
		t.Fatalf("read: chat=%d win=%v mode=%v status=%q", c.Unread, w, m.mode, m.status)
	}
	if v := m.View(); !strings.Contains(v, "news") || strings.Contains(v, "Alice Smith") {
		t.Fatalf("the filter still narrows the rows:\n%s", v)
	}
	// Unread on a chat without a window opens one, marked, without switching.
	press(m, tea.KeyEsc)
	enter(m, "/close 2") // #dev's window
	if _, w := m.winFor("c1"); w != nil {
		t.Fatal("#dev closed")
	}
	cur := m.cur
	enter(m, "/list dev")
	press(m, tea.KeyRight)
	press(m, tea.KeyRight) // [unread]
	press(m, tea.KeyEnter)
	i, w := m.winFor("c1")
	c, _ = m.chat("c1")
	if w == nil || w.unread != 1 || c.Unread < 1 || m.cur != cur || m.mode != modeList || !strings.Contains(m.activity(), strconv.Itoa(i)) {
		t.Fatalf("unread: i=%d win=%v chat=%d cur=%d mode=%v act=%q", i, w, c.Unread, m.cur, m.mode, m.activity())
	}
	// Typing still narrows after an action; an empty list has no buttons.
	typ(m, "xyz")
	if v := m.View(); !strings.Contains(v, "no chat matches") || strings.Contains(v, "[open]") {
		t.Fatalf("narrowed to nothing:\n%s", v)
	}
	press(m, tea.KeyEnter) // nothing to act on; the list closes as before
	if m.mode != modeWin {
		t.Fatalf("mode %v", m.mode)
	}
}

func TestLeftRightMoveTheCursorOutsideListsOnly(t *testing.T) {
	m := setup(t, newFake())
	typ(m, "abc")
	press(m, tea.KeyLeft)
	if m.curBack != 1 {
		t.Fatalf("Left moves the cursor in a window: %d", m.curBack)
	}
	press(m, tea.KeyRight)
	if m.curBack != 0 {
		t.Fatalf("Right moves it back: %d", m.curBack)
	}
	m.input, m.curBack = nil, 0
	enter(m, "/list")
	typ(m, "al")
	press(m, tea.KeyLeft)
	if m.curBack != 0 || m.listAct != 0 {
		t.Fatalf("in a list Left picks buttons, not the filter cursor: back=%d act=%d", m.curBack, m.listAct)
	}
	press(m, tea.KeyRight)
	if m.curBack != 0 || m.listAct != 1 || string(m.input) != "al" {
		t.Fatalf("Right: back=%d act=%d input=%q", m.curBack, m.listAct, string(m.input))
	}
	press(m, tea.KeyBackspace)
	if string(m.input) != "a" {
		t.Fatalf("the filter is still edited: %q", string(m.input))
	}
	m.input, m.curBack = nil, 0
	enter(m, "/win")
	if m.mode != modeList || m.listSrc != listWindows || m.listAct != 0 {
		t.Fatalf("a new list starts on the default button: mode=%v act=%d", m.mode, m.listAct)
	}
}

func TestListButtonsAreClickable(t *testing.T) {
	m := setup(t, newFake())
	enter(m, "/win")
	press(m, tea.KeyDown) // #news
	m.View()
	x, y := find(m, spanAct, "1:1") // [read]
	if x < 0 {
		t.Fatal("the read button is a click target")
	}
	click(m, x, y)
	if m.wins[1].unread != 0 || m.mode != modeList || m.listAct != 1 {
		t.Fatalf("click read: unread=%d mode=%v act=%d", m.wins[1].unread, m.mode, m.listAct)
	}
	m.View()
	if rx, ry := find(m, spanRow, "1"); rx < 0 || ry != y {
		t.Fatal("the row itself stays a click target on the same screen row")
	} else if rowSpan := m.clicks[ry][0]; rowSpan.kind != spanRow || rowSpan.x1 > x {
		t.Fatalf("the row target ends where the buttons start: %+v (button at %d)", rowSpan, x)
	}
	x, y = find(m, spanAct, "1:3") // [close]
	n := len(m.wins)
	click(m, x, y)
	if len(m.wins) != n-1 || m.mode != modeList {
		t.Fatalf("click close: wins=%d mode=%v", len(m.wins), m.mode)
	}
	m.View()
	x, y = find(m, spanRow, "1")
	cmd := click(m, x, y)
	if m.mode != modeWin || m.cur != 1 || cmd == nil {
		t.Fatalf("a click on the row focuses it: mode=%v cur=%d", m.mode, m.cur)
	}
}
