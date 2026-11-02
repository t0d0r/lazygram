package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// TestBottomRowsAreOnTheStatusBackground: in every theme the status
// line, the window bar and the prompt row fill the window width, and
// every cell of them, the padding included, carries a background colour.
// Before, the padding after the last segment followed a reset and showed
// the terminal's own background.
func TestBottomRowsAreOnTheStatusBackground(t *testing.T) {
	old := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor) // test output has no colours otherwise
	t.Cleanup(func() { lipgloss.SetColorProfile(old); applyTheme(themes["default"]) })
	m := setup(t, newFake())
	m.width = 90
	m.input, m.curBack = []rune("hello"), 2
	for _, name := range themeNames() {
		applyTheme(themes[name])
		bar, _ := m.windowBar()
		for row, line := range map[string]string{"status line": m.statusLine(), "window bar": bar, "prompt": m.inputLine("[0:server] ")} {
			if w := lipgloss.Width(line); w != m.width {
				t.Errorf("%s: %s is %d wide, want %d", name, row, w, m.width)
			}
			for _, chunk := range strings.Split(line, "\x1b[0m") {
				if ansi.Strip(chunk) != "" && !strings.Contains(chunk, "48;") {
					t.Errorf("%s: %s has cells without a background: %q", name, row, ansi.Strip(chunk))
				}
			}
		}
	}
}
