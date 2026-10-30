package tui

import tea "github.com/charmbracelet/bubbletea"

// Snapshot syncs once and renders one screen without a terminal. It backs the
// non-interactive smoke mode and the model tests.
func Snapshot(opt Options, width, height int) string {
	m := New(opt)
	m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	for _, c := range []tea.Cmd{m.chatsCmd(), m.syncCmd()} {
		m.Update(c())
	}
	return m.View()
}
