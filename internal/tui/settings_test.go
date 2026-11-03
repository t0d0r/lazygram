package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"lazygram/internal/command"
)

// menuOrder is the order of the settings menu's rows.
var menuOrder = []string{"feed", "autojoin", "read-receipts", "names", "theme", "fx", "mouse", "clip"}

// menuRows are the rows the menu shows, as name and value, in order.
func menuRows(m *Model) [][2]string {
	var out [][2]string
	for _, l := range strings.Split(ansi.Strip(body(m)), "\n") {
		if f := strings.Fields(l); len(f) >= 2 && slices.Contains(menuOrder, f[0]) {
			out = append(out, [2]string{f[0], f[1]})
		}
	}
	return out
}

// menuValue is the value the menu shows for the named setting, "" when the
// row is not shown.
func menuValue(m *Model, name string) string {
	for _, r := range menuRows(m) {
		if r[0] == name {
			return r[1]
		}
	}
	return ""
}

// settingNamed is the menu's row for the named setting.
func settingNamed(t *testing.T, name string) setting {
	t.Helper()
	for _, s := range settings {
		if s.name == name {
			return s
		}
	}
	t.Fatalf("no row %q", name)
	return setting{}
}

// openAt opens the menu and selects the named setting's row.
func openAt(t *testing.T, m *Model, name string) {
	t.Helper()
	m.input, m.curBack = nil, 0
	enter(m, "/settings")
	for range slices.Index(menuOrder, name) {
		press(m, tea.KeyDown)
	}
	m.View()
	if !m.settingsOpen() || m.listCur != slices.Index(menuOrder, name) {
		t.Fatalf("the menu is open on %s: mode=%v cur=%d", name, m.mode, m.listCur)
	}
}

// msgsOf runs a command and returns the messages it makes, batches
// opened. The commands here never wait.
func msgsOf(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if b, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, c := range b {
			out = append(out, msgsOf(c)...)
		}
		return out
	}
	return []tea.Msg{msg}
}

// makes reports whether the command makes a message of the same type as
// want's.
func makes(cmd tea.Cmd, want tea.Cmd) bool {
	w := reflect.TypeOf(want())
	for _, msg := range msgsOf(cmd) {
		if reflect.TypeOf(msg) == w {
			return true
		}
	}
	return false
}

func defaultTheme(t *testing.T) {
	applyTheme(themes["default"])
	t.Cleanup(func() { applyTheme(themes["default"]) })
}

func TestSettingsOpensInWindow0FromAChatWindow(t *testing.T) {
	defaultTheme(t)
	m := setup(t, newFake())
	enter(m, "/win 2") // #dev
	if m.cur != 2 {
		t.Fatalf("in a chat window: %d", m.cur)
	}
	enter(m, "/settings")
	if m.cur != 0 || !m.settingsOpen() || m.listCur != 0 {
		t.Fatalf("/settings focuses window 0 and opens the menu there: cur=%d mode=%v src=%v", m.cur, m.mode, m.listSrc)
	}
	v := ansi.Strip(m.View())
	rows := strings.Split(v, "\n")
	bar, input := rows[len(rows)-2], rows[len(rows)-1]
	if !strings.Contains(v, "[0:server]") || !strings.HasPrefix(strings.TrimSpace(bar), "[0:server]") {
		t.Fatalf("the window bar and the status show window 0:\n%s", v)
	}
	if !strings.HasPrefix(input, "settings > ") || m.prompt() != "settings > " {
		t.Fatalf("the prompt: %q", input)
	}
	if !strings.Contains(v, "settings (8): Up/Down, Enter or Right next value, Left previous, Esc closes, type to narrow") {
		t.Fatalf("the rule explains the keys:\n%s", v)
	}
	if strings.Contains(v, "[open]") || strings.Contains(v, "[focus]") || strings.Contains(v, "first dev line") {
		t.Fatalf("the menu has no buttons and covers window 0's lines:\n%s", v)
	}
	want := [][2]string{{"feed", "on"}, {"autojoin", "on"}, {"read-receipts", "off"}, {"names", "nick"},
		{"theme", "default"}, {"fx", "off"}, {"mouse", "on"}, {"clip", "auto"}}
	if got := menuRows(m); !reflect.DeepEqual(got, want) {
		t.Fatalf("the eight rows in order with their values:\n%q\nwant\n%q", got, want)
	}
	// The columns line up, whatever the values.
	var descAt []int
	for _, l := range strings.Split(ansi.Strip(body(m)), "\n") {
		for _, s := range settings {
			if f := strings.Fields(l); len(f) > 0 && f[0] == s.name {
				descAt = append(descAt, strings.Index(l, s.desc))
			}
		}
	}
	if len(descAt) != 8 || slices.Min(descAt) != slices.Max(descAt) || descAt[0] < 0 {
		t.Fatalf("the descriptions start in one column: %v", descAt)
	}
	// The selected row is highlighted, as in the other lists.
	if lines := strings.Split(body(m), "\n"); !strings.Contains(lines[0], hiStyle.Render(ansi.Strip(lines[0]))) {
		t.Fatalf("the first row is selected and highlighted: %q", lines[0])
	}
}

func TestSettingsEnterRightLeftStepWrapAndKeep(t *testing.T) {
	defaultTheme(t)
	st := memStore(t)
	m := startWith(t, st, newFake())
	kept := func(name string) string { // what a new model on the same store reads
		return settingNamed(t, name).value(New(Options{Store: st, Provider: newFake()}))
	}
	for _, name := range menuOrder {
		s := settingNamed(t, name)
		vals := s.values()
		openAt(t, m, name)
		start := menuValue(m, name)
		i := slices.Index(vals, start)
		if i < 0 || start != s.value(m) {
			t.Fatalf("%s shows %q, the setting is %q, the values %q", name, start, s.value(m), vals)
		}
		next, prev := vals[(i+1)%len(vals)], vals[(i+len(vals)-1)%len(vals)]
		press(m, tea.KeyEnter)
		if got := menuValue(m, name); got != next || s.value(m) != next || kept(name) != next {
			t.Fatalf("%s: Enter from %q: shown %q, set %q, kept %q; want %q", name, start, got, s.value(m), kept(name), next)
		}
		if !m.settingsOpen() || m.listCur != slices.Index(menuOrder, name) {
			t.Fatalf("%s: the menu stays open on the row: mode=%v cur=%d", name, m.mode, m.listCur)
		}
		press(m, tea.KeyLeft)
		if got := menuValue(m, name); got != start || kept(name) != start {
			t.Fatalf("%s: Left goes back to %q: %q, kept %q", name, start, got, kept(name))
		}
		press(m, tea.KeyLeft)
		if got := menuValue(m, name); got != prev || kept(name) != prev {
			t.Fatalf("%s: Left from %q wraps to %q: %q", name, start, prev, got)
		}
		press(m, tea.KeyRight)
		if got := menuValue(m, name); got != start || kept(name) != start {
			t.Fatalf("%s: Right goes forward to %q: %q", name, start, got)
		}
		if len(vals) == 2 && next != prev {
			t.Fatalf("%s: for two values both directions toggle", name)
		}
		press(m, tea.KeyEsc)
	}
	// Every value of every row, forward round the whole cycle.
	for _, name := range menuOrder {
		s := settingNamed(t, name)
		openAt(t, m, name)
		start := s.value(m)
		for range s.values() {
			press(m, tea.KeyRight)
			if kept(name) != s.value(m) || menuValue(m, name) != s.value(m) {
				t.Fatalf("%s: %q kept as %q, shown as %q", name, s.value(m), kept(name), menuValue(m, name))
			}
		}
		if s.value(m) != start {
			t.Fatalf("%s: a whole cycle comes back to %q: %q", name, start, s.value(m))
		}
		press(m, tea.KeyEsc)
	}
}

func TestSettingsChangeAsTheirCommandsDo(t *testing.T) {
	defaultTheme(t)
	commandFor := map[string]func(v string) string{
		"feed": func(v string) string { return "/feed " + v }, "autojoin": func(v string) string { return "/autojoin " + v },
		"read-receipts": func(v string) string { return "/read-receipts " + v }, "names": func(string) string { return "/names" },
		"theme": func(v string) string { return "/theme " + v }, "fx": func(string) string { return "/fx" },
		"mouse": func(string) string { return "/mouse" }, "clip": func(v string) string { return "/clip " + v },
	}
	for _, name := range menuOrder {
		menu, typed := setup(t, newFake()), setup(t, newFake())
		s := settingNamed(t, name)
		openAt(t, menu, name)
		for _, key := range []tea.KeyType{tea.KeyEnter, tea.KeyLeft} {
			var c1 tea.Cmd
			_, c1 = menu.Update(tea.KeyMsg{Type: key})
			v := s.value(menu)
			c2 := enter(typed, commandFor[name](v))
			if s.value(typed) != v || menu.status != typed.status || menu.isErr != typed.isErr {
				t.Fatalf("%s %v: the menu set %q and said %q (err %v); %s says %q (err %v)",
					name, key, v, menu.status, menu.isErr, commandFor[name](v), typed.status, typed.isErr)
			}
			var k1, k2 bool
			if name == "mouse" {
				want := tea.Cmd(tea.EnableMouseAllMotion)
				if v == "off" {
					want = tea.DisableMouse
				}
				k1, k2 = makes(c1, want), makes(c2, want)
			}
			if k1 != k2 || name == "mouse" && !k1 {
				t.Fatalf("%s: the menu returns what the command returns: %v %v", name, k1, k2)
			}
			var a, b any
			if found, _ := menu.opt.Store.State(s.key, &a); !found {
				t.Fatalf("%s: kept under %q", name, s.key)
			}
			_, _ = typed.opt.Store.State(s.key, &b)
			if !reflect.DeepEqual(a, b) {
				t.Fatalf("%s: kept as %v, the command keeps %v", name, a, b)
			}
			// The rule shows what was said, until the next key.
			if v := ansi.Strip(menu.View()); !strings.Contains(v, ansi.Truncate(menu.status, 60, "")) || strings.Contains(v, "settings (8)") {
				t.Fatalf("%s: the rule shows the change's status %q:\n%s", name, menu.status, v)
			}
		}
		press(menu, tea.KeyDown)
		press(menu, tea.KeyUp)
		if v := ansi.Strip(menu.View()); !strings.Contains(v, "settings (8): Up/Down") {
			t.Fatalf("%s: after another key the rule explains the keys again:\n%s", name, v)
		}
	}
}

func TestSettingsMouseRowTurnsReportingOffAndOn(t *testing.T) {
	m := setup(t, newFake())
	openAt(t, m, "mouse")
	cmd := press(m, tea.KeyEnter)
	if m.mouseOn || !makes(cmd, tea.DisableMouse) || !strings.Contains(m.status, "drag selects text") {
		t.Fatalf("off: on=%v status %q", m.mouseOn, m.status)
	}
	cmd = press(m, tea.KeyRight)
	if !m.mouseOn || !makes(cmd, tea.EnableMouseAllMotion) {
		t.Fatalf("on again: %v", m.mouseOn)
	}
	cmd = press(m, tea.KeyLeft)
	if m.mouseOn || !makes(cmd, tea.DisableMouse) {
		t.Fatal("Left toggles too")
	}
	again := New(Options{Store: m.opt.Store, Provider: newFake()})
	if again.mouseOn || !makes(again.Init(), tea.DisableMouse) {
		t.Fatal("kept, and applied at the next start")
	}
}

// TestHelpAndKeysDescribeFeedAndSettings: the notes /help prints and the
// table /keys prints say what /feed off leaves in window 0 and what a
// click, Left / Right and Esc do in the settings menu.
func TestHelpAndKeysDescribeFeedAndSettings(t *testing.T) {
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
	help, keys := printed("/help"), printed("/keys")
	for _, want := range []string{
		"Window 0 is the server window: every chat interleaved, plus notices (/feed off: the notices only)",
		"a row in /list or /win to open it, a row of /settings to set its next value",
	} {
		if !strings.Contains(help, want) {
			t.Errorf("/help lacks %q", want)
		}
	}
	for _, want := range []string{
		"in /settings: the selected setting's previous / next value",
		"close the list, the pager or the settings menu; clear the input",
	} {
		if !strings.Contains(keys, want) {
			t.Errorf("/keys lacks %q", want)
		}
	}
}

func TestSettingsThemeRowSwitchesAndWraps(t *testing.T) {
	defaultTheme(t)
	m := setup(t, newFake())
	names := themeNames()
	openAt(t, m, "theme")
	press(m, tea.KeyEnter)
	if currentName != names[1] || menuValue(m, "theme") != names[1] || m.status != "theme: "+names[1] {
		t.Fatalf("Enter: the next theme in /theme's order, %s: %s %q", names[1], currentName, m.status)
	}
	press(m, tea.KeyLeft)
	press(m, tea.KeyLeft)
	last := names[len(names)-1]
	var kept string
	if _, _ = m.opt.Store.State(themeKey, &kept); currentName != last || kept != last || menuValue(m, "theme") != last {
		t.Fatalf("Left from the first wraps to the last, %s: current %s, kept %s", last, currentName, kept)
	}
	press(m, tea.KeyRight)
	if currentName != names[0] {
		t.Fatalf("Right from the last wraps to the first: %s", currentName)
	}
}

func TestSettingsNarrowByTypedText(t *testing.T) {
	m := setup(t, newFake())
	enter(m, "/settings")
	typ(m, "RECEIPTS")
	if got := menuRows(m); len(got) != 1 || got[0][0] != "read-receipts" {
		t.Fatalf("typing narrows by name, ignoring case: %q", got)
	}
	if v := ansi.Strip(m.View()); !strings.Contains(v, "settings (1): ") {
		t.Fatalf("the rule counts the rows left:\n%s", v)
	}
	press(m, tea.KeyEnter)
	if !m.receipts || string(m.input) != "RECEIPTS" || !m.settingsOpen() {
		t.Fatalf("Enter changes the row left; the filter stays: receipts=%v input=%q", m.receipts, string(m.input))
	}
	m.input, m.curBack = nil, 0
	typ(m, "selection") // clip's description
	if got := menuRows(m); len(got) != 1 || got[0][0] != "clip" {
		t.Fatalf("by description: %q", got)
	}
	m.input, m.curBack = nil, 0
	typ(m, "mo")
	if got := menuRows(m); len(got) != 2 || got[0][0] != "mouse" || got[1][0] != "clip" {
		t.Fatalf("in menu order: %q", got)
	}
	press(m, tea.KeyDown)
	press(m, tea.KeyEnter)
	if m.clipMode != clipOSC52 || !m.mouseOn {
		t.Fatalf("Enter acts on the selected row of the narrowed menu: clip %s mouse %v", m.clipMode, m.mouseOn)
	}
	typ(m, "xyz")
	if v := body(m); !strings.Contains(v, "no setting matches") || len(menuRows(m)) != 0 {
		t.Fatalf("narrowed to nothing:\n%s", v)
	}
	before := m.status
	press(m, tea.KeyEnter)
	press(m, tea.KeyRight)
	if !m.settingsOpen() || m.status != before || m.clipMode != clipOSC52 {
		t.Fatalf("nothing to change: the menu stays, nothing changes: %q", m.status)
	}
	press(m, tea.KeyEsc)
	enter(m, "/set colours") // a short form, narrowed at once: theme's description
	if got := menuRows(m); !m.settingsOpen() || len(got) != 1 || got[0][0] != "theme" || string(m.input) != "colours" {
		t.Fatalf("/set <text> opens narrowed: %q input %q", got, string(m.input))
	}
	press(m, tea.KeyEsc)
	enter(m, "/options feed")
	if got := menuRows(m); len(got) != 1 || got[0][0] != "feed" {
		t.Fatalf("/options <text>: %q", got)
	}
}

func TestSettingsEscLeavesWindow0OnItsOwnView(t *testing.T) {
	m := setup(t, newFake())
	enter(m, "/win 3")
	enter(m, "/settings")
	typ(m, "fe")
	press(m, tea.KeyEsc)
	if m.mode != modeWin || m.cur != 0 || len(m.input) != 0 {
		t.Fatalf("Esc: window 0 stays current, on its view: mode=%v cur=%d input=%q", m.mode, m.cur, string(m.input))
	}
	v := ansi.Strip(m.View())
	if !strings.Contains(v, "first dev line") || !strings.Contains(v, "-!- /help lists commands") || len(menuRows(m)) != 0 {
		t.Fatalf("window 0's messages and notices:\n%s", v)
	}
	if !strings.Contains(v, "[0:server] ") {
		t.Fatalf("the prompt is window 0's:\n%s", v)
	}
}

func TestSettingsCloseOnASwitchToAnotherWindow(t *testing.T) {
	m := setup(t, newFake()) // 0 server, 1 #news, 2 #dev, 3 @alice
	for _, c := range []struct {
		what string
		do   func()
		to   int
	}{
		{"Alt-2", func() { m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}, Alt: true}) }, 2},
		{"Esc 3", func() { press(m, tea.KeyEsc); typ(m, "3") }, 3},
		{"Ctrl-N", func() { press(m, tea.KeyCtrlN) }, 1},
		{"Ctrl-P", func() { press(m, tea.KeyCtrlP) }, 3},
		{"Alt-Right", func() { m.Update(tea.KeyMsg{Type: tea.KeyRight, Alt: true}) }, 1},
		{"Tab", func() { press(m, tea.KeyTab) }, 1},
		{"Shift-Tab", func() { press(m, tea.KeyShiftTab) }, 3},
		{"window bar", func() {
			m.View()
			x, y := find(m, spanWin, "2")
			click(m, x, y)
		}, 2},
		{"/win 1", func() { enter(m, "/win 1") }, 1},
		{"/win @alice", func() { enter(m, "/win @alice") }, 3},
	} {
		enter(m, "/settings")
		if !m.settingsOpen() || m.cur != 0 {
			t.Fatalf("%s: the menu is open in window 0", c.what)
		}
		c.do()
		if m.settingsOpen() || m.mode != modeWin || m.cur != c.to {
			t.Fatalf("%s: the switch to window %d closes the menu: cur=%d mode=%v", c.what, c.to, m.cur, m.mode)
		}
		if v := m.View(); len(menuRows(m)) != 0 || strings.Contains(v, "settings > ") {
			t.Fatalf("%s: window %d shows its own view:\n%s", c.what, c.to, v)
		}
	}
	// Alt-0 is window 0 itself: its own view, so the menu closes too.
	enter(m, "/settings")
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'0'}, Alt: true})
	if m.settingsOpen() || m.cur != 0 {
		t.Fatalf("Alt-0: window 0's own view: mode=%v cur=%d", m.mode, m.cur)
	}
	// With a filter typed, Tab completes it to a setting's name instead,
	// as Tab in /list completes its filter; /list never switches on Tab.
	enter(m, "/settings")
	typ(m, "re")
	press(m, tea.KeyTab)
	if !m.settingsOpen() || m.cur != 0 || string(m.input) != "read-receipts " {
		t.Fatalf("Tab completes the filter: %q", string(m.input))
	}
	m.input, m.curBack = nil, 0
	typ(m, "/fee") // a command typed in the menu completes as one
	press(m, tea.KeyTab)
	if string(m.input) != "/feed " {
		t.Fatalf("a command: %q", string(m.input))
	}
	press(m, tea.KeyEsc)
	enter(m, "/list")
	press(m, tea.KeyTab)
	if m.mode != modeList || m.listSrc != listChats || m.cur != 0 {
		t.Fatalf("/list keeps Tab for its filter: mode=%v cur=%d", m.mode, m.cur)
	}
}

func TestSettingsClickStepsTheClickedRow(t *testing.T) {
	m := setup(t, newFake())
	enter(m, "/settings")
	m.View()
	at := slices.Index(menuOrder, "names")
	x, y := find(m, spanRow, strconv.Itoa(at))
	if x < 0 {
		t.Fatal("each row is a click target")
	}
	if s := m.clicks[y][0]; s.x1 != m.width || len(m.clicks[y]) != 1 {
		t.Fatalf("the whole row, no buttons: %+v", m.clicks[y])
	}
	click(m, x+5, y)
	if !m.byName || m.listCur != at || !m.settingsOpen() || menuValue(m, "names") != "name" {
		t.Fatalf("a click selects the row and sets its next value: byName=%v cur=%d", m.byName, m.listCur)
	}
	m.multi = clickRun{} // a later, separate click
	m.View()
	x, y = find(m, spanRow, strconv.Itoa(at))
	click(m, x, y)
	if m.byName {
		t.Fatal("a second click toggles it back")
	}
	// The selected row is clickable too, and the menu stays open.
	m.multi = clickRun{}
	at = slices.Index(menuOrder, "fx")
	m.View()
	x, y = find(m, spanRow, strconv.Itoa(at))
	click(m, x, y)
	if !m.fx || m.listCur != at || !m.settingsOpen() {
		t.Fatalf("fx: %v cur %d", m.fx, m.listCur)
	}
	// The wheel moves the selection, as in the other lists.
	m.Update(tea.MouseMsg{X: 1, Y: 1, Button: tea.MouseButtonWheelUp, Action: tea.MouseActionPress})
	if m.listCur != at-1 {
		t.Fatalf("wheel: %d", m.listCur)
	}
}

func TestSettingsRowsFollowTheSettings(t *testing.T) {
	defaultTheme(t)
	m := setup(t, newFake())
	check := func(when string) {
		t.Helper()
		enter(m, "/settings")
		got := menuRows(m)
		if len(got) != len(settings) {
			t.Fatalf("%s: %d rows", when, len(got))
		}
		for i, s := range settings {
			if got[i][0] != s.name || got[i][1] != s.value(m) {
				t.Fatalf("%s: row %d is %q, the setting %s is %q", when, i, got[i], s.name, s.value(m))
			}
		}
		press(m, tea.KeyEsc)
	}
	check("at start")
	for _, line := range []string{"/feed off", "/autojoin off", "/read-receipts on", "/names", "/theme nord", "/fx", "/mouse", "/clip tool"} {
		enter(m, line)
	}
	check("after the commands")
	if values := func() []string {
		enter(m, "/settings")
		defer func() { press(m, tea.KeyEsc) }()
		var out []string
		for _, r := range menuRows(m) {
			out = append(out, r[1])
		}
		return out
	}(); !reflect.DeepEqual(values, []string{"off", "off", "on", "name", "nord", "on", "off", "tool"}) {
		t.Fatalf("the values after the commands: %q", values)
	}
	// A command typed in the open menu: it stays open, the row follows and
	// the rule shows what the command said.
	enter(m, "/settings")
	enter(m, "/feed on")
	if !m.settingsOpen() || menuValue(m, "feed") != "on" || !strings.Contains(ansi.Strip(m.View()), "feed on: window 0 shows") {
		t.Fatalf("a command in the menu: open=%v feed %q\n%s", m.settingsOpen(), menuValue(m, "feed"), m.View())
	}
	enter(m, "/theme nope")
	if v := ansi.Strip(m.View()); !m.isErr || !strings.Contains(v, "no theme nope") {
		t.Fatalf("its error shows on the rule:\n%s", v)
	}
	enter(m, "/nope")
	if v := ansi.Strip(m.View()); !m.isErr || !m.settingsOpen() || !strings.Contains(v, "unknown command /nope") {
		t.Fatalf("so does a line that is no command:\n%s", v)
	}
	press(m, tea.KeyDown)
	if v := ansi.Strip(m.View()); !strings.Contains(v, "settings (8): ") {
		t.Fatalf("the next key brings the keys back:\n%s", v)
	}
	press(m, tea.KeyEsc)
	check("after commands in the menu")
}

// TestEveryKeptSettingHasARow fails when a store key for a setting is
// added (a const named ...Key with a string value in this package) without
// a row in the menu, and when a row names no command of its own.
func TestEveryKeptSettingHasARow(t *testing.T) {
	notSettings := map[string]bool{layoutKey: true, sharedKey: true} // the window layout, the chats sharing ids
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]bool{}
	fset := token.NewFileSet()
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range file.Decls {
			g, ok := d.(*ast.GenDecl)
			if !ok || g.Tok != token.CONST {
				continue
			}
			for _, spec := range g.Specs {
				vs := spec.(*ast.ValueSpec)
				for i, n := range vs.Names {
					if !strings.HasSuffix(n.Name, "Key") || i >= len(vs.Values) {
						continue
					}
					if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
						v, _ := strconv.Unquote(lit.Value)
						keys[v] = true
					}
				}
			}
		}
	}
	if len(keys) < 10 {
		t.Fatalf("found too few store keys to trust the scan: %v", keys)
	}
	rows := map[string]bool{}
	for _, s := range settings {
		rows[s.key] = true
		if !keys[s.key] {
			t.Errorf("row %s keeps %q, which is no store key here", s.name, s.key)
		}
		c, err := command.Parse("/" + s.name)
		if err != nil || c.Kind == command.Settings {
			t.Errorf("row %s is not the name of its command: %v", s.name, err)
		}
		if len(s.values()) < 2 || !slices.Contains(s.values(), s.value(setup(t, newFake()))) {
			t.Errorf("row %s: values %q", s.name, s.values())
		}
	}
	for k := range keys {
		if !rows[k] && !notSettings[k] {
			t.Errorf("the store key %q has no row in the settings menu", k)
		}
	}
	var names []string
	for _, s := range settings {
		names = append(names, s.name)
	}
	if !reflect.DeepEqual(names, menuOrder) {
		t.Fatalf("the order: %q", names)
	}
}
