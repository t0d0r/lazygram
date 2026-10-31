package tui

import (
	"bytes"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"lazygram/internal/store"
)

func TestThemesAreComplete(t *testing.T) {
	if len(themes) < 5 {
		t.Fatalf("want at least 5 themes, have %d", len(themes))
	}
	for name, th := range themes {
		if th.Name != name {
			t.Errorf("%s: name mismatch %q", name, th.Name)
		}
		for role, v := range map[string]string{"dim": th.Dim, "day": th.Day, "note": th.Note, "button": th.Button,
			"barbg": th.BarBG, "barfg": th.BarFG, "statusbg": th.StatusBG, "statusfg": th.StatusFG, "act": th.Act, "error": th.Error,
			"link": th.Link, "spark": th.Spark} {
			if v == "" {
				t.Errorf("%s: %s is empty", name, role)
			}
		}
		if len(th.Palette) < 4 || len(th.Gradient) < 3 {
			t.Errorf("%s: palette %d gradient %d", name, len(th.Palette), len(th.Gradient))
		}
	}
	for _, want := range []string{"default", "solarized", "gruvbox", "dracula", "nord", "monokai", "tokyonight", "vim", "emacs"} {
		if _, ok := themes[want]; !ok {
			t.Errorf("missing theme %s", want)
		}
	}
	if _, ok := lookupTheme("NORD"); !ok {
		t.Error("case-insensitive lookup")
	}
	if _, ok := lookupTheme("gru"); !ok {
		t.Error("unique prefix")
	}
	if _, ok := lookupTheme("x"); ok {
		t.Error("unknown theme")
	}
}

func TestThemeCommandSwitchesListsAndPersists(t *testing.T) {
	t.Cleanup(func() { applyTheme(themes["default"]) })
	st, _ := store.Open(store.Memory, bytes.Repeat([]byte{7}, 32))
	defer func() { _ = st.Close() }()
	m := New(Options{Store: st, Provider: newFake()})
	m.revealAll()
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	enter(m, "/theme")
	v := body(m)
	if !strings.Contains(v, "* default") || !strings.Contains(v, "  nord") || !strings.Contains(v, "emacs") {
		t.Fatalf("list:\n%s", v)
	}
	enter(m, "/theme nord")
	if currentName != "nord" || m.isErr || !strings.Contains(m.status, "theme: nord") {
		t.Fatalf("switch: %s %q", currentName, m.status)
	}
	enter(m, "/theme nope")
	if !m.isErr || currentName != "nord" {
		t.Fatalf("unknown theme: %q", m.status)
	}
	again := New(Options{Store: st, Provider: newFake()})
	again.revealAll()
	_ = again
	if currentName != "nord" {
		t.Fatal("theme must be restored from the store")
	}
	m.input = nil
	typ(m, "/theme dr")
	press(m, tea.KeyTab)
	if got := string(m.input); got != "/theme dracula " {
		t.Fatalf("completion: %q", got)
	}
}
