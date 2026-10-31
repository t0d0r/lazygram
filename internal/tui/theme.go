package tui

import (
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// theme is a colour scheme: the roles lazygram paints, as hex or ANSI
// colour values lipgloss understands. Themes only colour text and bars;
// the terminal's own background stays.
type theme struct {
	Name     string
	Dim      string // time stamps, prompts, separators' text
	Day      string // day separators
	Note     string // -!- notices
	Button   string // bot buttons
	BarBG    string // topic bar
	BarFG    string
	StatusBG string // status line and window bar
	StatusFG string
	Act      string   // activity: windows with unread messages
	Error    string   // error text in the status line
	Link     string   // clickable links and attachment tokens
	Spark    string   // the running light over clickable text
	Palette  []string // nicks and chat names, by hash
	Gradient []string // banner lines, top to bottom
}

// themes are the built-in schemes, after well-known editor and terminal
// palettes (official colour values of each scheme).
var themes = map[string]theme{
	"default": {
		Name: "default", Dim: "244", Day: "244", Note: "110", Button: "214",
		BarBG: "24", BarFG: "255", StatusBG: "236", StatusFG: "252", Act: "220", Error: "203", Link: "51", Spark: "231",
		Palette:  []string{"39", "208", "41", "170", "220", "81", "203", "141"},
		Gradient: []string{"81", "75", "69", "99", "135", "171", "207"},
	},
	"solarized": {
		Name: "solarized", Dim: "#586e75", Day: "#586e75", Note: "#2aa198", Button: "#b58900",
		BarBG: "#073642", BarFG: "#93a1a1", StatusBG: "#073642", StatusFG: "#839496", Act: "#b58900", Error: "#dc322f", Link: "#2aa198", Spark: "#fdf6e3",
		Palette:  []string{"#268bd2", "#cb4b16", "#859900", "#d33682", "#b58900", "#2aa198", "#dc322f", "#6c71c4"},
		Gradient: []string{"#2aa198", "#268bd2", "#6c71c4", "#d33682", "#cb4b16", "#b58900", "#859900"},
	},
	"gruvbox": {
		Name: "gruvbox", Dim: "#928374", Day: "#928374", Note: "#8ec07c", Button: "#fe8019",
		BarBG: "#3c3836", BarFG: "#ebdbb2", StatusBG: "#3c3836", StatusFG: "#d5c4a1", Act: "#fabd2f", Error: "#fb4934", Link: "#8ec07c", Spark: "#fbf1c7",
		Palette:  []string{"#83a598", "#fe8019", "#b8bb26", "#d3869b", "#fabd2f", "#8ec07c", "#fb4934", "#d65d0e"},
		Gradient: []string{"#8ec07c", "#83a598", "#d3869b", "#fb4934", "#fe8019", "#fabd2f", "#b8bb26"},
	},
	"dracula": {
		Name: "dracula", Dim: "#6272a4", Day: "#6272a4", Note: "#8be9fd", Button: "#ffb86c",
		BarBG: "#44475a", BarFG: "#f8f8f2", StatusBG: "#44475a", StatusFG: "#f8f8f2", Act: "#f1fa8c", Error: "#ff5555", Link: "#8be9fd", Spark: "#ffffff",
		Palette:  []string{"#8be9fd", "#ffb86c", "#50fa7b", "#ff79c6", "#f1fa8c", "#bd93f9", "#ff5555", "#6272a4"},
		Gradient: []string{"#8be9fd", "#bd93f9", "#ff79c6", "#ff5555", "#ffb86c", "#f1fa8c", "#50fa7b"},
	},
	"nord": {
		Name: "nord", Dim: "#4c566a", Day: "#4c566a", Note: "#88c0d0", Button: "#d08770",
		BarBG: "#3b4252", BarFG: "#eceff4", StatusBG: "#3b4252", StatusFG: "#d8dee9", Act: "#ebcb8b", Error: "#bf616a", Link: "#88c0d0", Spark: "#eceff4",
		Palette:  []string{"#88c0d0", "#d08770", "#a3be8c", "#b48ead", "#ebcb8b", "#8fbcbb", "#bf616a", "#81a1c1"},
		Gradient: []string{"#8fbcbb", "#88c0d0", "#81a1c1", "#5e81ac", "#b48ead", "#d08770", "#ebcb8b"},
	},
	"monokai": {
		Name: "monokai", Dim: "#75715e", Day: "#75715e", Note: "#66d9ef", Button: "#fd971f",
		BarBG: "#3e3d32", BarFG: "#f8f8f2", StatusBG: "#3e3d32", StatusFG: "#f8f8f2", Act: "#e6db74", Error: "#f92672", Link: "#66d9ef", Spark: "#f8f8f2",
		Palette:  []string{"#66d9ef", "#fd971f", "#a6e22e", "#f92672", "#e6db74", "#ae81ff", "#f92672", "#a6e22e"},
		Gradient: []string{"#66d9ef", "#ae81ff", "#f92672", "#fd971f", "#e6db74", "#a6e22e", "#66d9ef"},
	},
	"tokyonight": {
		Name: "tokyonight", Dim: "#565f89", Day: "#565f89", Note: "#7dcfff", Button: "#ff9e64",
		BarBG: "#24283b", BarFG: "#c0caf5", StatusBG: "#24283b", StatusFG: "#a9b1d6", Act: "#e0af68", Error: "#f7768e", Link: "#7dcfff", Spark: "#ffffff",
		Palette:  []string{"#7aa2f7", "#ff9e64", "#9ece6a", "#bb9af7", "#e0af68", "#7dcfff", "#f7768e", "#1abc9c"},
		Gradient: []string{"#7dcfff", "#7aa2f7", "#bb9af7", "#f7768e", "#ff9e64", "#e0af68", "#9ece6a"},
	},
	"vim": {
		// Vim's classic dark terminal look: grey comments, cyan identifiers,
		// yellow statements, magenta constants, reverse-video status line.
		Name: "vim", Dim: "#808080", Day: "#5f87af", Note: "#5fd7ff", Button: "#ffaf00",
		BarBG: "#585858", BarFG: "#ffffff", StatusBG: "#303030", StatusFG: "#d0d0d0", Act: "#ffff00", Error: "#ff5f5f", Link: "#5fd7ff", Spark: "#ffffff",
		Palette:  []string{"#5fd7ff", "#ff8700", "#5fff5f", "#ff5fff", "#ffff5f", "#87afff", "#ff5f5f", "#af87ff"},
		Gradient: []string{"#5fd7ff", "#87afff", "#af87ff", "#ff5fff", "#ff5f5f", "#ff8700", "#ffff5f"},
	},
	"emacs": {
		// Emacs default faces on a dark terminal: chocolate comments, cyan
		// keywords, salmon strings, pale green types; grey75 mode line.
		Name: "emacs", Dim: "#d2691e", Day: "#d2691e", Note: "#b0c4de", Button: "#eedd82",
		BarBG: "#bfbfbf", BarFG: "#000000", StatusBG: "#bfbfbf", StatusFG: "#000000", Act: "#8b0000", Error: "#b22222", Link: "#00ffff", Spark: "#ffffff",
		Palette:  []string{"#00ffff", "#ffa07a", "#98fb98", "#7fffd4", "#eedd82", "#87cefa", "#ff7f50", "#b0c4de"},
		Gradient: []string{"#87cefa", "#00ffff", "#7fffd4", "#98fb98", "#eedd82", "#ffa07a", "#ff7f50"},
	},
}

// themeNames lists the built-in themes, sorted.
func themeNames() []string {
	out := make([]string, 0, len(themes))
	for n := range themes {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Styles painted by the renderer. applyTheme sets them; the default theme
// is applied at package init.
var (
	dimStyle    lipgloss.Style
	dayStyle    lipgloss.Style
	noteStyle   lipgloss.Style
	buttonStyle lipgloss.Style
	hiStyle     = lipgloss.NewStyle().Reverse(true).Bold(true)
	infoStyle   lipgloss.Style
	actStyle    lipgloss.Style
	errStyle    lipgloss.Style
	linkStyle   lipgloss.Style
	sparkStyle  lipgloss.Style
	palette     []lipgloss.Color
	gradient    []lipgloss.Color
	currentName string
)

func init() { applyTheme(themes["default"]) }

// applyTheme repaints every style from t.
func applyTheme(t theme) {
	c := func(s string) lipgloss.Color { return lipgloss.Color(s) }
	dimStyle = lipgloss.NewStyle().Foreground(c(t.Dim))
	dayStyle = lipgloss.NewStyle().Foreground(c(t.Day)).Italic(true)
	noteStyle = lipgloss.NewStyle().Foreground(c(t.Note))
	buttonStyle = lipgloss.NewStyle().Foreground(c(t.Button))
	infoStyle = lipgloss.NewStyle().Background(c(t.StatusBG)).Foreground(c(t.StatusFG))
	actStyle = lipgloss.NewStyle().Background(c(t.StatusBG)).Foreground(c(t.Act)).Bold(true)
	errStyle = lipgloss.NewStyle().Background(c(t.StatusBG)).Foreground(c(t.Error)).Bold(true)
	linkStyle = lipgloss.NewStyle().Foreground(c(t.Link)).Underline(true)
	sparkStyle = lipgloss.NewStyle().Foreground(c(t.Spark)).Bold(true).Underline(true)
	palette = palette[:0]
	for _, p := range t.Palette {
		palette = append(palette, c(p))
	}
	gradient = gradient[:0]
	for _, g := range t.Gradient {
		gradient = append(gradient, c(g))
	}
	currentName = t.Name
}

// statusBG is the background colour shared by the status line and bar.
func statusBG() lipgloss.Color { return lipgloss.Color(themes[currentName].StatusBG) }

// lookupTheme finds a theme by name, case-insensitively, by exact name or
// unique prefix.
func lookupTheme(name string) (theme, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if t, ok := themes[name]; ok {
		return t, true
	}
	var hit theme
	n := 0
	for _, k := range themeNames() {
		if strings.HasPrefix(k, name) {
			hit, n = themes[k], n+1
		}
	}
	return hit, n == 1
}
