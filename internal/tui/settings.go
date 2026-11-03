package tui

import (
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"lazygram/internal/domain"
)

// The settings menu (/settings, /set, /options) lists every kept setting,
// one row each: its name (the command without the slash), its current
// value and what it does. It belongs to the client, not to a chat, so it
// opens in window 0, the server window, which /settings focuses first
// wherever it was typed, as /help and /keys do; any switch to a window
// shows that window's own view and so closes it. Enter or Right sets the
// selected row's next value, Left its previous one, wrapping, through the
// command's own code (the same store key, status message and side
// effects), and the menu stays open on the same row so the new value
// shows. There is no button strip: values change in place.

// setting is one row of the menu: a kept setting, its store key, its
// values in the order Enter and Right step through them, and the
// command's own code that reads and sets it. set returns what the command
// returns, such as the command that turns mouse reporting on or off.
type setting struct {
	name, key, desc string
	values          func() []string
	value           func(m *Model) string
	set             func(m *Model, v string) tea.Cmd
}

// onOff are the values of a setting that is on or off.
var onOff = []string{"on", "off"}

func onOffValues() []string { return onOff }

func onOffOf(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// settings are the menu's rows, in order. Every kept setting has one
// (TestEveryKeptSettingHasARow).
var settings = []setting{
	{name: "feed", key: feedKey, desc: "chat messages in window 0; off: notices only",
		values: onOffValues, value: func(m *Model) string { return onOffOf(m.feed) },
		set: func(m *Model, v string) tea.Cmd { m.setFeed(v); return nil }},
	{name: "autojoin", key: autojoinKey, desc: "at start open a window per unread chat",
		values: onOffValues, value: func(m *Model) string { return onOffOf(m.autojoin) },
		set: func(m *Model, v string) tea.Cmd { m.setAutojoin(v); return nil }},
	{name: "read-receipts", key: receiptsKey, desc: "report a chat read when its window is focused",
		values: onOffValues, value: func(m *Model) string { return onOffOf(m.receipts) },
		set: func(m *Model, v string) tea.Cmd { m.readReceipts(v); return nil }},
	{name: "names", key: namesKey, desc: "chats by nick (@alias) or by name (@Title)",
		values: func() []string { return []string{"nick", "name"} },
		value: func(m *Model) string {
			if m.byName {
				return "name"
			}
			return "nick"
		},
		set: func(m *Model, v string) tea.Cmd { m.setNames(v == "name"); return nil }},
	{name: "theme", key: themeKey, desc: "the colours of the text and the bars",
		values: themeNames, value: func(*Model) string { return currentName },
		set: func(m *Model, v string) tea.Cmd { m.setTheme(v); return nil }},
	{name: "fx", key: fxKey, desc: "a running light over clickable text",
		values: onOffValues, value: func(m *Model) string { return onOffOf(m.fx) },
		set: func(m *Model, v string) tea.Cmd { m.setFx(v == "on"); return nil }},
	{name: "mouse", key: mouseKey, desc: "mouse reporting; off: the terminal selects",
		values: onOffValues, value: func(m *Model) string { return onOffOf(m.mouseOn) },
		set: func(m *Model, v string) tea.Cmd { return m.setMouse(v == "on") }},
	{name: "clip", key: clipKey, desc: "where a mouse selection is copied",
		values: func() []string { return clipModes }, value: func(m *Model) string { return m.clipMode },
		set: func(m *Model, v string) tea.Cmd { m.clip(v); return nil }},
	{name: "vim", key: vimKey, desc: "vi keys: Esc for Normal, i for Insert",
		values: onOffValues, value: func(m *Model) string { return onOffOf(m.vi.on) },
		set: func(m *Model, v string) tea.Cmd { m.setVim(v); return nil }},
}

// settingsOpen reports whether the menu is what window 0 shows now.
func (m *Model) settingsOpen() bool { return m.mode == modeList && m.listSrc == listSettings }

// openSettings implements /settings: window 0 is focused, wherever it was
// typed, and shows the menu, narrowed by the text given.
func (m *Model) openSettings(arg string) tea.Cmd {
	cmd := m.switchTo(0)
	m.mode, m.listSrc, m.listCur, m.listAct, m.input, m.curBack = modeList, listSettings, 0, 0, []rune(arg), 0
	m.listSaid = false
	return cmd
}

// settingRows are the rows the typed text leaves: those whose name or
// description holds it, case-insensitively and in either script, as the
// /list filter narrows.
func (m *Model) settingRows() []setting {
	mt := domain.NewMatcher(string(m.input))
	var out []setting
	for _, s := range settings {
		if mt.Match(s.name) || mt.Match(s.desc) {
			out = append(out, s)
		}
	}
	return out
}

// settingLines renders the menu rows: name, value and description in
// aligned columns, wide enough for every value any row can take, so a
// change never moves the descriptions.
func (m *Model) settingLines(rows []setting) []string {
	if len(rows) == 0 {
		return []string{dimStyle.Render("  no setting matches")}
	}
	nameW, valW := 0, 0
	for _, s := range settings {
		nameW = max(nameW, len(s.name))
		for _, v := range s.values() {
			valW = max(valW, len(v))
		}
	}
	out := make([]string, 0, len(rows))
	for _, s := range rows {
		out = append(out, fmt.Sprintf(" %-*s  %-*s  %s", nameW, s.name, valW, s.value(m), s.desc))
	}
	return out // the selected row is highlighted by View
}

// stepSetting sets the selected row's setting to its next value (d = 1)
// or its previous one (d = -1), wrapping, through the command's own code,
// which keeps it and says so; the menu stays open on the same row and the
// rule shows what the command said until the next key.
func (m *Model) stepSetting(d int) (tea.Model, tea.Cmd) {
	rows := m.settingRows()
	if len(rows) == 0 { // narrowed to nothing: nothing to change
		return m, nil
	}
	m.listCur = min(max(m.listCur, 0), len(rows)-1)
	s := rows[m.listCur]
	vals := s.values()
	i := slices.Index(vals, s.value(m))
	if i < 0 { // a value the row does not list: the first (or, going back, the last)
		i = 0
		if d > 0 {
			i = -1
		}
	}
	n := len(vals)
	cmd := s.set(m, vals[((i+d)%n+n)%n])
	m.listSaid = true
	return m, cmd
}

// completeSetting completes the menu's filter to the names of the
// settings, as Tab completes a word anywhere else.
func completeSetting(word string) []string {
	var out []string
	for _, s := range settings {
		if strings.HasPrefix(s.name, word) {
			out = append(out, s.name)
		}
	}
	return out
}

// The handlers below are shared by a command and its row in the menu, so
// both keep the setting the same way and say the same.

// setNames shows chats by name (@Title) or by nick (@alias), kept; /names
// toggles it.
func (m *Model) setNames(byName bool) {
	m.byName = byName
	_ = m.opt.Store.SetState(namesKey, m.byName)
	m.rcache = &renderCache{}
	if m.byName {
		m.setInfo("chats shown by name (@Title); /names switches back to nicks")
	} else {
		m.setInfo("chats shown by nick (@alias)")
	}
}

// setFx turns the running lights on or off, kept; /fx toggles them.
func (m *Model) setFx(on bool) {
	m.fx = on
	_ = m.opt.Store.SetState(fxKey, m.fx)
	if m.fx {
		m.setInfo("running lights on")
	} else {
		m.setInfo("running lights off")
	}
}

// setMouse turns mouse reporting on or off, kept, and returns the command
// that tells the terminal; /mouse toggles it.
func (m *Model) setMouse(on bool) tea.Cmd {
	m.mouseOn = on
	_ = m.opt.Store.SetState(mouseKey, m.mouseOn)
	if m.mouseOn {
		m.setInfo("mouse on: click to act; hold Shift (or Option on macOS) while dragging to select text")
		return tea.EnableMouseAllMotion
	}
	m.hover = ""
	m.dropKept() // the terminal's own selection takes over
	m.setInfo("mouse off: drag selects text; /mouse turns clicks back on")
	return tea.DisableMouse
}
