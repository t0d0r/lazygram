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
type note struct {
	at   time.Time
	text string
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
			t := notes[j].at.Local()
			day(t)
			prefix := stamp(t) + " "
			for k, l := range wrapText(sanitize(notes[j].text, true), max(o.width-len(prefix)-4, 10)) {
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
		plainPrefix += "<" + m.Sender + "> "
		prefix += lipgloss.NewStyle().Foreground(colorFor(m.Sender)).Render("<"+m.Sender+">") + " "
		prefixW := lipgloss.Width(plainPrefix)
		body := wrapText(m.Text, max(o.width-prefixW, 10))
		indent := strings.Repeat(" ", prefixW)
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

// fitLine truncates s to width columns and pads it with spaces, so bars
// never wrap however long their content is.
func fitLine(s string, width int) string {
	s = ansi.Truncate(s, width, "")
	if pad := width - lipgloss.Width(s); pad > 0 {
		s += strings.Repeat(" ", pad)
	}
	return s
}

// topWindow returns exactly height lines starting at the top, scrolled just
// enough to keep line sel visible; short content is padded at the bottom.
func topWindow(lines []string, height, sel int) []string {
	if height < 1 {
		return nil
	}
	start := max(min(sel, len(lines)-1)-height+1, 0)
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
