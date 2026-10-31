package tui

import (
	"fmt"
	"hash/fnv"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"lazygram/internal/domain"
)

var palette = []lipgloss.Color{"39", "208", "41", "170", "220", "81", "203", "141"}

func colorFor(s string) lipgloss.Color {
	h := fnv.New32a()
	h.Write([]byte(s))
	return palette[int(h.Sum32())%len(palette)]
}

var (
	dimStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	dayStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Italic(true)
	noteStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("110"))
	hiStyle   = lipgloss.NewStyle().Reverse(true).Bold(true)
	barStyle  = lipgloss.NewStyle().Background(lipgloss.Color("24")).Foreground(lipgloss.Color("255")).Bold(true)
	infoStyle = lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("252"))
	actStyle  = lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("220")).Bold(true)
)

// note is a client-side line in a window: "-!- connected", /list output, ...
// A raw note is shown as it is, without time stamp or marker: the banner.
type note struct {
	at   time.Time
	text string
	raw  bool
}

// gradient colours banner lines from top to bottom.
var gradient = []lipgloss.Color{"81", "75", "69", "99", "135", "171", "207"}

// Banner colours figlet lines with the gradient; used for the start banner
// in the server window and for the exit banner on the terminal.
func Banner(lines []string) []string {
	out := make([]string, 0, len(lines))
	for i, l := range lines {
		out = append(out, lipgloss.NewStyle().Foreground(gradient[i%len(gradient)]).Bold(true).Render(l))
	}
	return out
}

// lineOpts controls how messages become terminal lines.
type lineOpts struct {
	width     int
	showChat  bool                       // prefix lines with the chat name (server window)
	nameOf    func(chatID string) string // chat name with sigil
	nameW     int                        // padded chat column width
	fullDate  bool                       // history pager shows the date on every line
	highlight string                     // case-insensitive text to highlight
}

// renderLines turns chronological messages and notes into wrapped, styled
// lines, interleaved by time. A dim separator is inserted when the local
// calendar day changes.
func renderLines(msgs []domain.Message, notes []note, o lineOpts) []string {
	var out []string
	lastDay := ""
	day := func(t time.Time) {
		if d := t.Format("2006-01-02"); d != lastDay && !o.fullDate {
			out = append(out, dayStyle.Render("--- "+t.Format("Mon 2006-01-02")+" ---"))
			lastDay = d
		}
	}
	stamp := func(t time.Time) string {
		if o.fullDate {
			return t.Format("2006-01-02 15:04")
		}
		return t.Format("15:04")
	}
	i, j := 0, 0
	for i < len(msgs) || j < len(notes) {
		if j < len(notes) && (i >= len(msgs) || !notes[j].at.After(msgs[i].Time)) {
			if notes[j].raw {
				out = append(out, Banner(strings.Split(sanitize(notes[j].text, true), "\n"))...)
				j++
				continue
			}
			t := notes[j].at.Local()
			day(t)
			prefix := stamp(t) + " "
			room := max(o.width-len(prefix)-4, 10)
			for k, l := range wrapText(sanitize(notes[j].text, true), room, room) {
				if k == 0 {
					out = append(out, dimStyle.Render(prefix)+noteStyle.Render("-!- "+l))
				} else {
					out = append(out, strings.Repeat(" ", len(prefix)+4)+noteStyle.Render(l))
				}
			}
			j++
			continue
		}
		m := msgs[i]
		i++
		m.Sender, m.Text = sanitize(m.Sender, false), sanitize(m.Text, true)
		t := m.Time.Local()
		day(t)
		plainPrefix := stamp(t) + " "
		prefix := dimStyle.Render(stamp(t)) + " "
		if o.showChat {
			name := o.nameOf(m.ChatID)
			plainPrefix += fmt.Sprintf("%-*s ", o.nameW, name)
			prefix += lipgloss.NewStyle().Foreground(colorFor(m.ChatID)).Bold(true).Render(fmt.Sprintf("%-*s", o.nameW, name)) + " "
		}
		text := m.Text
		if action, ok := strings.CutPrefix(m.Text, "* "); ok && strings.TrimSpace(action) != "" {
			// IRC-style action (/me): "* sender does something"
			plainPrefix += "* " + m.Sender + " "
			prefix += lipgloss.NewStyle().Foreground(colorFor(m.Sender)).Italic(true).Render("* "+m.Sender) + " "
			text = action
		} else {
			plainPrefix += "<" + m.Sender + "> "
			prefix += lipgloss.NewStyle().Foreground(colorFor(m.Sender)).Render("<"+m.Sender+">") + " "
		}
		prefixW := lipgloss.Width(plainPrefix)
		// Continuation lines sit just past the time column, BitchX style,
		// rather than under the text, so long messages waste no width.
		indent := strings.Repeat(" ", len(stamp(t))+1)
		body := wrapText(text, max(o.width-prefixW, 10), max(o.width-len(indent), 10))
		for k, l := range body {
			l = highlight(l, o.highlight)
			if k == 0 {
				out = append(out, prefix+l)
			} else {
				out = append(out, indent+l)
			}
		}
	}
	return out
}

// highlight marks case-insensitive occurrences of needle in s.
func highlight(s, needle string) string {
	needle = strings.ToLower(needle)
	low := strings.ToLower(s)
	if needle == "" || len(low) != len(s) || !strings.Contains(low, needle) {
		return s
	}
	var b strings.Builder
	for {
		i := strings.Index(low, needle)
		if i < 0 {
			b.WriteString(s)
			break
		}
		b.WriteString(s[:i])
		b.WriteString(hiStyle.Render(s[i : i+len(needle)]))
		s, low = s[i+len(needle):], low[i+len(needle):]
	}
	return b.String()
}

// wrapText wraps text: the first output line may use first columns, every
// later line width columns. Embedded newlines start new lines, empty lines
// are dropped, lines that fit keep their spacing, and words longer than a
// line are hard-broken. It always returns at least one line.
func wrapText(text string, first, width int) []string {
	var out []string
	limit := func() int {
		if len(out) == 0 {
			return first
		}
		return width
	}
	for _, para := range strings.Split(strings.ReplaceAll(text, "\r", ""), "\n") {
		if strings.TrimSpace(para) == "" {
			continue
		}
		if lipgloss.Width(para) <= limit() {
			out = append(out, strings.TrimRight(para, " ")) // keep internal spacing
			continue
		}
		var cur string
		flush := func() {
			out = append(out, cur)
			cur = ""
		}
		for _, w := range strings.Fields(para) {
			for lipgloss.Width(w) > limit() {
				if cur != "" {
					flush()
				}
				head, rest := splitAt(w, limit())
				out = append(out, head)
				w = rest
			}
			switch {
			case cur == "":
				cur = w
			case lipgloss.Width(cur)+1+lipgloss.Width(w) <= limit():
				cur += " " + w
			default:
				flush()
				cur = w
			}
		}
		if cur != "" {
			flush()
		}
	}
	if len(out) == 0 {
		out = append(out, "")
	}
	return out
}

// splitAt splits s after at most width display columns (at least one rune).
func splitAt(s string, width int) (string, string) {
	w := 0
	for i, r := range s {
		rw := lipgloss.Width(string(r))
		if w+rw > width && i > 0 {
			return s[:i], s[i:]
		}
		w += rw
	}
	return s, ""
}

// fitLine truncates s to width columns and pads it with spaces, so bars
// never wrap however long their content is.
func fitLine(s string, width int) string {
	s = ansi.Truncate(s, width, "")
	if pad := width - lipgloss.Width(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

// topWindow returns exactly height lines of a list. The selected line sel
// moves down the screen until it reaches the middle, then stays there while
// the list scrolls under it, and moves on to the bottom only when the list
// ends (the same on the way up). Short content is padded at the bottom.
func topWindow(lines []string, height, sel int) []string {
	if height < 1 {
		return nil
	}
	sel = min(max(sel, 0), max(len(lines)-1, 0))
	start := min(max(sel-height/2, 0), max(len(lines)-height, 0))
	end := min(start+height, len(lines))
	view := append([]string(nil), lines[start:end]...)
	for len(view) < height {
		view = append(view, "")
	}
	return view
}

// window returns exactly height lines: the slice of lines ending offset lines
// above the bottom. It returns the clamped offset so callers can store it.
func window(lines []string, height, offset int) ([]string, int) {
	if height < 1 {
		return nil, 0
	}
	maxOff := max(len(lines)-height, 0)
	offset = min(max(offset, 0), maxOff)
	end := len(lines) - offset
	start := max(end-height, 0)
	view := append([]string(nil), lines[start:end]...)
	for len(view) < height {
		view = append([]string{""}, view...)
	}
	return view, offset
}

// sanitize drops control characters (including ESC) so remote text cannot
// inject terminal escape sequences. Newlines survive when keepNL is set;
// tabs become spaces.
func sanitize(s string, keepNL bool) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\n' && keepNL:
			return r
		case r == '\t':
			return ' '
		case unicode.IsControl(r):
			return -1
		}
		return r
	}, s)
}
