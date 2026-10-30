package tui

import (
	"fmt"
	"hash/fnv"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"

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
	hiStyle   = lipgloss.NewStyle().Reverse(true).Bold(true)
	barStyle  = lipgloss.NewStyle().Background(lipgloss.Color("24")).Foreground(lipgloss.Color("255")).Bold(true)
	infoStyle = lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("252"))
)

// lineOpts controls how messages become terminal lines.
type lineOpts struct {
	width     int
	aliasOf   func(chatID string) string
	aliasW    int    // padded alias column width
	fullDate  bool   // history pager shows the date on every line
	highlight string // case-insensitive text to highlight
}

// renderLines turns chronological messages into wrapped, styled lines. A dim
// separator is inserted when the local calendar day changes.
func renderLines(msgs []domain.Message, o lineOpts) []string {
	var out []string
	lastDay := ""
	for _, m := range msgs {
		t := m.Time.Local()
		if day := t.Format("2006-01-02"); day != lastDay && !o.fullDate {
			out = append(out, dayStyle.Render("--- "+t.Format("Mon 2006-01-02")+" ---"))
			lastDay = day
		}
		m.Sender, m.Text = sanitize(m.Sender, false), sanitize(m.Text, true)
		stamp := t.Format("15:04")
		if o.fullDate {
			stamp = t.Format("2006-01-02 15:04")
		}
		alias := "#" + o.aliasOf(m.ChatID)
		plainPrefix := fmt.Sprintf("%s %-*s <%s> ", stamp, o.aliasW+1, alias, m.Sender)
		prefixW := lipgloss.Width(plainPrefix)
		prefix := dimStyle.Render(stamp) + " " +
			lipgloss.NewStyle().Foreground(colorFor(m.ChatID)).Bold(true).Render(fmt.Sprintf("%-*s", o.aliasW+1, alias)) + " " +
			lipgloss.NewStyle().Foreground(colorFor(m.Sender)).Render("<"+m.Sender+">") + " "
		body := wrapText(m.Text, max(o.width-prefixW, 10))
		indent := strings.Repeat(" ", prefixW)
		for i, l := range body {
			l = highlight(l, o.highlight)
			if i == 0 {
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

// wrapText wraps text to width columns, honouring embedded newlines and
// hard-breaking words longer than a line. It always returns at least one line.
func wrapText(text string, width int) []string {
	var out []string
	for _, para := range strings.Split(strings.ReplaceAll(text, "\r", ""), "\n") {
		var cur string
		flush := func() {
			out = append(out, cur)
			cur = ""
		}
		for _, w := range strings.Fields(para) {
			for lipgloss.Width(w) > width {
				if cur != "" {
					flush()
				}
				head, rest := splitAt(w, width)
				out = append(out, head)
				w = rest
			}
			switch {
			case cur == "":
				cur = w
			case lipgloss.Width(cur)+1+lipgloss.Width(w) <= width:
				cur += " " + w
			default:
				flush()
				cur = w
			}
		}
		flush()
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
