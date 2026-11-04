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
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"

	"lazygram/internal/command"
	"lazygram/internal/domain"
)

// menuOrder is the order of the settings menu's rows.
var menuOrder = []string{"feed", "autojoin", "read-receipts", "names", "theme", "fx", "mouse", "clip", "vim", "doubletab", "doubletab-time"}

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
	// The rows under the content: the rule, the status line, the window
	// bar, the input. The bar lists every window whichever is current (the
	// current one stands out only in colour, see
	// TestSettingsMarksWindow0AndTheSelectedRow); the status line names it.
	if rows := strings.Split(ansi.Strip(m.View()), "\n"); !strings.Contains(rows[len(rows)-3], "[2:#dev]") {
		t.Fatalf("the status line names #dev: %q", rows[len(rows)-3])
	}
	enter(m, "/settings")
	if m.cur != 0 || !m.settingsOpen() || m.listCur != 0 {
		t.Fatalf("/settings focuses window 0 and opens the menu there: cur=%d mode=%v src=%v", m.cur, m.mode, m.listSrc)
	}
	v := ansi.Strip(m.View())
	rows := strings.Split(v, "\n")
	status, input := rows[len(rows)-3], rows[len(rows)-1]
	if !strings.Contains(status, "[0:server]") || strings.Contains(status, "[2:#dev]") {
		t.Fatalf("the status line shows window 0: %q", status)
	}
	if !strings.HasPrefix(input, "settings > ") || m.prompt() != "settings > " {
		t.Fatalf("the prompt: %q", input)
	}
	if !strings.Contains(v, "settings (11): Up/Down, Enter or Right next value, Left previous, Esc closes, type to narrow") {
		t.Fatalf("the rule explains the keys:\n%s", v)
	}
	if strings.Contains(v, "[open]") || strings.Contains(v, "[focus]") || strings.Contains(v, "first dev line") {
		t.Fatalf("the menu has no buttons and covers window 0's lines:\n%s", v)
	}
	want := [][2]string{{"feed", "on"}, {"autojoin", "on"}, {"read-receipts", "off"}, {"names", "nick"},
		{"theme", "default"}, {"fx", "off"}, {"mouse", "on"}, {"clip", "auto"}, {"vim", "off"},
		{"doubletab", "off"}, {"doubletab-time", "400ms"}}
	if got := menuRows(m); !reflect.DeepEqual(got, want) {
		t.Fatalf("the eleven rows in order with their values:\n%q\nwant\n%q", got, want)
	}
	// The columns line up (whatever the values, see
	// TestSettingsDescriptionsStayPut).
	descColumn(t, m)
}

// descColumn is the column where the menu's descriptions start; it fails
// unless every row's description starts there.
func descColumn(t *testing.T, m *Model) int {
	t.Helper()
	var descAt []int
	for _, l := range strings.Split(ansi.Strip(body(m)), "\n") {
		for _, s := range settings {
			if f := strings.Fields(l); len(f) > 0 && f[0] == s.name {
				descAt = append(descAt, strings.Index(l, s.desc))
			}
		}
	}
	if len(descAt) != len(settings) || slices.Min(descAt) != slices.Max(descAt) || descAt[0] < 0 {
		t.Fatalf("the descriptions start in one column: %v\n%s", descAt, ansi.Strip(body(m)))
	}
	return descAt[0]
}

// TestSettingsDescriptionsStayPut: the value column is as wide as the
// longest value any row can take, so no value of any row moves the
// descriptions.
func TestSettingsDescriptionsStayPut(t *testing.T) {
	defaultTheme(t)
	m := setup(t, newFake())
	enter(m, "/settings")
	at := descColumn(t, m)
	press(m, tea.KeyEsc)
	for _, name := range menuOrder {
		s := settingNamed(t, name)
		openAt(t, m, name)
		for range s.values() { // every value, round the whole cycle
			press(m, tea.KeyRight)
			if got := descColumn(t, m); got != at {
				t.Fatalf("%s %s moves the descriptions from column %d to %d", name, s.value(m), at, got)
			}
		}
		press(m, tea.KeyEsc)
	}
	// The longest values at once, set by their commands.
	longest := ""
	for _, n := range themeNames() {
		if len(n) > len(longest) {
			longest = n
		}
	}
	enter(m, "/theme "+longest)
	enter(m, "/clip osc52")
	enter(m, "/doubletab 1000") // a time the row does not list, shown as it is
	enter(m, "/settings")
	if got := descColumn(t, m); got != at || menuValue(m, "theme") != longest || menuValue(m, "clip") != "osc52" ||
		menuValue(m, "doubletab-time") != "1000ms" {
		t.Fatalf("theme %s, clip osc52, doubletab-time %s: the descriptions at %d, not %d",
			menuValue(m, "theme"), menuValue(m, "doubletab-time"), got, at)
	}
}

// trueColour gives one test the TrueColor profile: test output has no
// colours otherwise, so a highlight reads as plain text.
func trueColour(t *testing.T) {
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	applyTheme(themes["default"])
	t.Cleanup(func() { lipgloss.SetColorProfile(old); applyTheme(themes["default"]) })
}

// reversed reports whether the SGR sequence that starts the first tag in
// row turns on reverse video (parameter 7), as windowBar draws the current
// window; the values of an extended colour (38, 48, 58) are skipped.
func reversed(row, tag string) bool {
	at := strings.Index(row, tag)
	if at < 0 {
		return false
	}
	start := strings.LastIndex(row[:at], "\x1b[")
	if start < 0 {
		return false
	}
	end := strings.IndexByte(row[start:], 'm')
	if end < 0 || start+end > at {
		return false
	}
	params := strings.Split(row[start+2:start+end], ";")
	for i := 0; i < len(params); i++ {
		switch params[i] {
		case "38", "48", "58":
			if i+1 < len(params) && params[i+1] == "2" {
				i += 4
			} else if i+1 < len(params) && params[i+1] == "5" {
				i += 2
			}
		case "7":
			return true
		}
	}
	return false
}

// TestSettingsMarksWindow0AndTheSelectedRow: while the menu is open the
// window bar marks 0:server as the current window, and the selected row,
// only it, is highlighted as in the other lists. Both are colour only, so
// the test draws in colour.
func TestSettingsMarksWindow0AndTheSelectedRow(t *testing.T) {
	trueColour(t)
	m := setup(t, newFake())
	enter(m, "/win 2") // #dev
	rows := strings.Split(m.View(), "\n")
	if bar := rows[len(rows)-2]; !reversed(bar, "[2:#dev]") || reversed(bar, "[0:server]") {
		t.Fatalf("in #dev the bar marks #dev: %q", bar)
	}
	enter(m, "/settings")
	press(m, tea.KeyDown)
	rows = strings.Split(m.View(), "\n")
	if bar := rows[len(rows)-2]; !reversed(bar, "[0:server]") || reversed(bar, "[2:#dev]") {
		t.Fatalf("with the menu open the bar marks 0:server as current: %q", bar)
	}
	lines := strings.Split(body(m), "\n")
	for i := range settings {
		plain := ansi.Strip(lines[i]) == lines[i]
		if i == m.listCur && plain || i != m.listCur && !plain {
			t.Fatalf("only the selected row %d is highlighted; row %d plain=%v: %q", m.listCur, i, plain, lines[i])
		}
	}
	if m.listCur != 1 {
		t.Fatalf("Down selects the second row: %d", m.listCur)
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
		"vim": func(v string) string { return "/vim " + v }, "doubletab": func(v string) string { return "/doubletab " + v },
		"doubletab-time": func(v string) string { return "/doubletab " + v },
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
			if v := ansi.Strip(menu.View()); !strings.Contains(v, ansi.Truncate(menu.status, 60, "")) || strings.Contains(v, "settings (11)") {
				t.Fatalf("%s: the rule shows the change's status %q:\n%s", name, menu.status, v)
			}
		}
		press(menu, tea.KeyDown)
		press(menu, tea.KeyUp)
		if v := ansi.Strip(menu.View()); !strings.Contains(v, "settings (11): Up/Down") {
			t.Fatalf("%s: after another key the rule explains the keys again:\n%s", name, v)
		}
	}
}

func TestSettingsMouseRowTurnsReportingOffAndOn(t *testing.T) {
	m := setup(t, newFake())
	openAt(t, m, "mouse")
	m.hover = hoverKey(span{kind: spanWin, target: "1"}) // the pointer was over a tag
	cmd := press(m, tea.KeyEnter)
	if m.mouseOn || !makes(cmd, tea.DisableMouse) || !strings.Contains(m.status, "drag selects text") {
		t.Fatalf("off: on=%v status %q", m.mouseOn, m.status)
	}
	if m.hover != "" {
		t.Fatalf("off forgets the element under the pointer, as /mouse does: %q", m.hover)
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

// TestSettingsNamesRowRedrawsWindow0: the names row drops the rendered
// lines as /names does, so window 0 shows the chats by their new names.
// The name column is capped (15) both by nick and by name here, so the
// rendered lines' cache key does not change with the setting and only the
// reset brings the new names.
func TestSettingsNamesRowRedrawsWindow0(t *testing.T) {
	for _, how := range []string{"the menu", "/names"} {
		f := newFake()
		f.chats = append(f.chats, domain.Chat{ID: "c9", Alias: "averyverylongalias", Title: "A Very Long Title Here", Kind: domain.ChatUser})
		m := setup(t, f)
		m.Update(tea.WindowSizeMsg{Width: 100, Height: 60}) // room for the banner, the messages and the notices
		byNick := m.nameWidth()
		if v := ansi.Strip(body(m)); !strings.Contains(v, "hi there") || strings.Contains(v, "@Alice Smith") {
			t.Fatalf("%s: window 0 by nick, its lines rendered:\n%s", how, v)
		}
		if how == "the menu" {
			openAt(t, m, "names")
			press(m, tea.KeyEnter)
			press(m, tea.KeyEsc)
		} else {
			enter(m, "/names")
		}
		if !m.byName || m.nameWidth() != byNick || byNick != 15 {
			t.Fatalf("%s: by name, the name column as wide as by nick (%d): %v %d", how, byNick, m.byName, m.nameWidth())
		}
		if v := ansi.Strip(body(m)); !strings.Contains(v, "@Alice Smith") {
			t.Fatalf("%s: window 0 shows the chats by name:\n%s", how, v)
		}
	}
}

// TestSettingsFeedRowKeepsWindow0ScrollValid: the feed row keeps window
// 0's scroll offset within its new number of lines as /feed does, before
// anything is drawn.
func TestSettingsFeedRowKeepsWindow0ScrollValid(t *testing.T) {
	f := newFake()
	for i := range 60 {
		f.msgs = append(f.msgs, mk("c1", "x"+itoa(i), "ada", "busy line "+itoa(i), 10+i))
	}
	m := setup(t, f)
	for range 3 {
		press(m, tea.KeyPgUp)
	}
	m.View()
	before := m.wins[0].scroll
	enter(m, "/settings")
	press(m, tea.KeyEnter) // feed, the first row: off
	lines, _ := m.richLinesOf(m.wins[0])
	limit := max(len(lines)-m.contentHeight(), 0)
	if m.feed || !m.settingsOpen() {
		t.Fatalf("the feed row turns the feed off: %v", m.feed)
	}
	if before <= limit {
		t.Fatalf("window 0 is scrolled back (%d) further than it reaches with the feed off (%d)", before, limit)
	}
	if s := m.wins[0].scroll; s > limit || s < 0 {
		t.Fatalf("off: scroll %d outside 0..%d", s, limit)
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
		"  Window 0               the server window: every chat interleaved, plus notices (/feed off: the notices only)",
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
	enter(m, "/settings colours") // narrowed at once: theme's description
	if got := menuRows(m); !m.settingsOpen() || len(got) != 1 || got[0][0] != "theme" || string(m.input) != "colours" {
		t.Fatalf("/settings <text> opens narrowed: %q input %q", got, string(m.input))
	}
	press(m, tea.KeyEsc)
	enter(m, "/SETTINGS feed")
	if got := menuRows(m); len(got) != 1 || got[0][0] != "feed" {
		t.Fatalf("/SETTINGS <text>: %q", got)
	}
}

// TestOnlySettingsOpensTheMenu: /settings is the menu's one name. /set and
// /options are unknown commands, handled as any other unknown command is:
// the same error, no menu, no focus change, nothing kept; and Tab
// completes /set to /settings, the one command it begins.
func TestOnlySettingsOpensTheMenu(t *testing.T) {
	type outcome struct {
		isErr, open bool
		cur         int
		mode        mode
		input       string
		cmds        int
	}
	run := func(line string) (outcome, string) {
		m := setup(t, newFake())
		enter(m, "/win 3")
		enter(m, line)
		return outcome{m.isErr, m.settingsOpen(), m.cur, m.mode, string(m.input), m.cmdsN}, m.status
	}
	for _, c := range []struct{ line, name string }{
		{"/set", "set"}, {"/set colours", "set"}, {"/options", "options"}, {"/OPTIONS feed", "options"},
	} {
		got, status := run(c.line)
		arg := strings.TrimPrefix(strings.ToLower(c.line), "/"+c.name)
		want, wantStatus := run("/xyzzy" + arg)
		if got != want || status != "unknown command /"+c.name+", try /help" ||
			wantStatus != "unknown command /xyzzy, try /help" || !got.isErr || got.open || got.cur != 3 {
			t.Errorf("%q: %+v %q; an unknown command gives %+v %q", c.line, got, status, want, wantStatus)
		}
	}
	m := setup(t, newFake())
	for typed, want := range map[string]string{"/set": "/settings ", "/se": "/se", "/opt": "/opt"} {
		m.input, m.curBack = nil, 0
		typ(m, typed)
		press(m, tea.KeyTab)
		if got := string(m.input); got != want {
			t.Errorf("Tab after %q: %q, want %q", typed, got, want)
		}
	}
	m.input, m.curBack = nil, 0
	typ(m, "/se")
	press(m, tea.KeyTab)
	press(m, tea.KeyTab) // the second Tab lists
	if v := body(m); !strings.Contains(v, "/search  /send  /settings") {
		t.Errorf("/se lists /search, /send and /settings, no /set:\n%s", v)
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
	rows := strings.Split(v, "\n")
	if input := rows[len(rows)-1]; !strings.HasPrefix(input, "[server] ") || m.prompt() != "[server] " {
		t.Fatalf("the prompt is window 0's: %q", input)
	}
	if status := rows[len(rows)-3]; !strings.Contains(status, "[0:server]") {
		t.Fatalf("the status line names window 0: %q", status)
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
	for _, line := range []string{"/feed off", "/autojoin off", "/read-receipts on", "/names", "/theme nord", "/fx", "/mouse", "/clip tool",
		"/doubletab unread", "/doubletab 250"} {
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
	}(); !reflect.DeepEqual(values, []string{"off", "off", "on", "name", "nord", "on", "off", "tool", "off", "unread", "250ms"}) {
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
	if v := ansi.Strip(m.View()); !strings.Contains(v, "settings (11): ") {
		t.Fatalf("the next key brings the keys back:\n%s", v)
	}
	press(m, tea.KeyEsc)
	check("after commands in the menu")
}

// TestEveryKeptSettingHasARow fails when a store key for a setting is
// added (a const named ...Key with a string value in this package) without
// a row in the menu, and when a row runs no command of its own: one named
// after another command (doubletab-time runs /doubletab) starts with that
// command's name.
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
		c, err := command.Parse("/" + s.command())
		if err != nil || c.Kind == command.Settings {
			t.Errorf("row %s runs no command of its own (/%s): %v", s.name, s.command(), err)
		}
		if s.name != s.command() && !strings.HasPrefix(s.name, s.command()+"-") {
			t.Errorf("row %s runs /%s: its name starts with the command's", s.name, s.command())
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
