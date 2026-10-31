package tui

import (
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"lazygram/internal/domain"
)

func colorFor(s string) lipgloss.Color {
	h := fnv.New32a()
	h.Write([]byte(s))
	return palette[int(h.Sum32())%len(palette)]
}

// note is a client-side line in a window: "-!- connected", /list output, ...
// A raw note is shown as it is, without time stamp or marker: the banner.
type note struct {
	at   time.Time
	text string
	raw  bool
}

// Banner colours figlet lines with the gradient; used for the start banner
// in the server window and for the exit banner on the terminal.
func Banner(lines []string) []string {
	out := make([]string, 0, len(lines))
	for i, l := range lines {
		out = append(out, lipgloss.NewStyle().Foreground(gradient[i%len(gradient)]).Bold(true).Render(l))
	}
	return out
}

// spanKind says what a click on a span does.
type spanKind int

const (
	spanChat    spanKind = iota // open the chat's window; target is the chat id
	spanNick                    // open the chat named like the sender; target is the sender
	spanURL                     // open the link; target is the URL
	spanMedia                   // fetch and open the attachment; target is "chat\x00msg"
	spanWin                     // focus window; target is its index
	spanRow                     // list row; target is its index
	spanEnd                     // jump to the newest line
	spanButtons                 // a message's button line; target is the message key
)

// span is a clickable range of columns on one rendered line.
type span struct {
	x0, x1 int // columns, x1 exclusive
	kind   spanKind
	target string
}

func (s span) hit(x int) bool { return x >= s.x0 && x < s.x1 }

var urlRE = regexp.MustCompile(`https?://[^\s<>"']+`)

// urlLocs finds links in s without the sentence punctuation that often
// follows them.
func urlLocs(s string) [][]int {
	locs := urlRE.FindAllStringIndex(s, -1)
	for _, loc := range locs {
		for loc[1] > loc[0]+8 && strings.ContainsRune(".,;:!?)]'\"", rune(s[loc[1]-1])) {
			loc[1]--
		}
	}
	return locs
}

// urlsOf lists the links in s.
func urlsOf(s string) []string {
	var out []string
	for _, loc := range urlLocs(s) {
		out = append(out, s[loc[0]:loc[1]])
	}
	return out
}

// wholeURL maps a link fragment to its whole URL when wrapping cut it: an
// exact entry wins; otherwise only a fragment that ends its line may be
// the start of a longer URL.
func wholeURL(fragment string, atLineEnd bool, full []string) string {
	for _, u := range full {
		if u == fragment {
			return u
		}
	}
	if atLineEnd {
		for _, u := range full {
			if strings.HasPrefix(u, fragment) {
				return u
			}
		}
	}
	return fragment
}

// lineOpts controls how messages become terminal lines.
type lineOpts struct {
	width     int
	showChat  bool                       // prefix lines with the chat name (server window)
	nameOf    func(chatID string) string // chat name with sigil
	nameW     int                        // padded chat column width
	fullDate  bool                       // history pager shows the date on every line
	highlight string                     // case-insensitive text to highlight
	phase     int                        // running-light position; -1 means no animation
	hover     string                     // hover key of the element under the pointer; only it animates
	cache     *renderCache               // optional per-message cache
}

// hovered is the phase to render an element with: the running light when
// it is the hovered one, none otherwise.
func (o lineOpts) hovered(kind spanKind, target string) int {
	if o.phase < 0 || o.hover == "" || o.hover != fmt.Sprintf("%d:%s", kind, target) {
		return -1
	}
	return o.phase
}

// sparkPeriod is the distance between two lights running over a span.
const sparkPeriod = 7

// renderCache remembers rendered messages; a message without animated
// spans renders once per width and theme, an animated one once per phase.
type renderCache struct {
	theme string
	m     map[string]cached
}

type cached struct {
	lines []string
	spans [][]span
	anim  bool
}

func (c *renderCache) get(key string) (cached, bool) {
	if c == nil || c.theme != currentName {
		return cached{}, false
	}
	v, ok := c.m[key]
	return v, ok
}

func (c *renderCache) put(key string, v cached) {
	if c == nil {
		return
	}
	if c.theme != currentName || len(c.m) > 20000 {
		c.theme, c.m = currentName, map[string]cached{}
	}
	c.m[key] = v
}

// clickable paints text as something the mouse can act on, with the
// running light at phase (none when phase < 0).
func clickable(text string, base lipgloss.Style, phase int) string {
	if phase < 0 {
		return base.Render(text)
	}
	var b strings.Builder
	for i, r := range []rune(text) {
		if (i+phase)%sparkPeriod == 0 {
			b.WriteString(sparkStyle.Render(string(r)))
		} else {
			b.WriteString(base.Render(string(r)))
		}
	}
	return b.String()
}

// styleText paints one plain line: search hits highlighted and links drawn
// clickable, the hovered one with the running light. full maps a link cut
// by wrapping to its whole URL.
func styleText(l string, o lineOpts, full []string) string {
	locs := urlLocs(l)
	if len(locs) == 0 {
		return highlight(l, o.highlight)
	}
	var b strings.Builder
	last := 0
	for _, loc := range locs {
		b.WriteString(highlight(l[last:loc[0]], o.highlight))
		target := wholeURL(l[loc[0]:loc[1]], loc[1] == len(l), full)
		b.WriteString(clickable(l[loc[0]:loc[1]], linkStyle, o.hovered(spanURL, target)))
		last = loc[1]
	}
	b.WriteString(highlight(l[last:], o.highlight))
	return b.String()
}

// renderLines turns chronological messages and notes into wrapped, styled
// lines, interleaved by time. A dim separator is inserted when the local
// calendar day changes.
func renderLines(msgs []domain.Message, notes []note, o lineOpts) []string {
	lines, _ := renderRich(msgs, notes, o)
	return lines
}

// renderRich is renderLines plus, for every line, the spans a click acts on.
func renderRich(msgs []domain.Message, notes []note, o lineOpts) ([]string, [][]span) {
	var out []string
	var spans [][]span
	add := func(line string, sp ...span) {
		out = append(out, line)
		spans = append(spans, sp)
	}
	lastDay := ""
	day := func(t time.Time) {
		if d := t.Format("2006-01-02"); d != lastDay && !o.fullDate {
			add(dayStyle.Render("--- " + t.Format("Mon 2006-01-02") + " ---"))
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
				for _, l := range Banner(strings.Split(sanitize(notes[j].text, true), "\n")) {
					add(l)
				}
				j++
				continue
			}
			t := notes[j].at.Local()
			day(t)
			prefix := stamp(t) + " "
			room := max(o.width-len(prefix)-4, 10)
			text := sanitize(notes[j].text, true)
			full := urlsOf(text)
			for k, l := range wrapText(text, room, room) {
				base := len(prefix) + 4
				if k == 0 {
					add(dimStyle.Render(prefix)+noteStyle.Render("-!- "+l), linkSpans(l, base, full)...)
				} else {
					add(strings.Repeat(" ", base)+noteStyle.Render(l), linkSpans(l, base, full)...)
				}
			}
			j++
			continue
		}
		m := msgs[i]
		i++
		t := m.Time.Local()
		day(t)
		key := cacheKey(m, o)
		if c, ok := o.cache.get(key); ok {
			for k := range c.lines {
				add(c.lines[k], c.spans[k]...)
			}
			continue
		}
		mark := len(out)
		anim := renderMessage(m, o, add)
		if o.cache != nil {
			o.cache.put(key, cached{lines: append([]string(nil), out[mark:]...), spans: append([][]span(nil), spans[mark:]...), anim: anim})
		}
	}
	return out, spans
}

// cacheKey identifies a rendered message: its identity and content, the
// rendering options, and the phase only when the message holds the hovered
// element.
func cacheKey(m domain.Message, o lineOpts) string {
	h := fnv.New32a()
	h.Write([]byte(m.Sender))
	h.Write([]byte{0})
	h.Write([]byte(m.Text))
	phase := -1
	if o.phase >= 0 && o.hover != "" && hasElement(m, o.hover) {
		phase = o.phase % sparkPeriod
	}
	return fmt.Sprintf("%s|%x|%d|%d|%t|%t|%s|%d|%d", m.Key(), h.Sum32(), len(m.Buttons), o.width, o.showChat, o.fullDate, o.highlight, o.nameW, phase)
}

// hasElement reports whether a message renders the element behind a hover
// key: its chat name, its sender, one of its links, its attachment token or
// its buttons.
func hasElement(m domain.Message, hover string) bool {
	keys := []string{
		fmt.Sprintf("%d:%s", spanChat, m.ChatID),
		fmt.Sprintf("%d:%s", spanNick, m.Sender),
		fmt.Sprintf("%d:%s", spanMedia, m.ChatID+"\x00"+m.ID),
		fmt.Sprintf("%d:%s", spanButtons, m.Key()),
	}
	for _, u := range urlsOf(m.Text) {
		keys = append(keys, fmt.Sprintf("%d:%s", spanURL, u))
	}
	for _, k := range keys {
		if k == hover {
			return true
		}
	}
	return false
}

// renderMessage emits one message's lines through add and reports whether
// it holds anything clickable.
func renderMessage(m domain.Message, o lineOpts, add func(string, ...span)) bool {
	stamp := func(t time.Time) string {
		if o.fullDate {
			return t.Format("2006-01-02 15:04")
		}
		return t.Format("15:04")
	}
	anim := false
	m.Sender, m.Text = sanitize(m.Sender, false), sanitize(m.Text, true)
	t := m.Time.Local()
	plainPrefix := stamp(t) + " "
	prefix := dimStyle.Render(stamp(t)) + " "
	var first []span
	if o.showChat {
		name := o.nameOf(m.ChatID)
		first = append(first, span{len(plainPrefix), len(plainPrefix) + lipgloss.Width(name), spanChat, m.ChatID})
		plainPrefix += fmt.Sprintf("%-*s ", o.nameW, name)
		pad := strings.Repeat(" ", max(o.nameW-lipgloss.Width(name), 0))
		prefix += clickable(name, lipgloss.NewStyle().Foreground(colorFor(m.ChatID)).Bold(true), o.hovered(spanChat, m.ChatID)) + pad + " "
	}
	text := m.Text
	nickX := lipgloss.Width(plainPrefix)
	if action, ok := strings.CutPrefix(m.Text, "* "); ok && strings.TrimSpace(action) != "" {
		// IRC-style action (/me): "* sender does something"
		plainPrefix += "* " + m.Sender + " "
		prefix += clickable("* "+m.Sender, lipgloss.NewStyle().Foreground(colorFor(m.Sender)).Italic(true), o.hovered(spanNick, m.Sender)) + " "
		text = action
	} else {
		plainPrefix += "<" + m.Sender + "> "
		prefix += clickable("<"+m.Sender+">", lipgloss.NewStyle().Foreground(colorFor(m.Sender)), o.hovered(spanNick, m.Sender)) + " "
	}
	first = append(first, span{nickX, lipgloss.Width(plainPrefix) - 1, spanNick, m.Sender})
	prefixW := lipgloss.Width(plainPrefix)
	token := ""
	if m.Media != nil { // the attachment token leads the text and is a click target
		token = mediaToken(m.Media)
		first = append(first, span{prefixW, prefixW + lipgloss.Width(token), spanMedia, m.ChatID + "\x00" + m.ID})
		anim = true
	}
	// Continuation lines sit just past the time column, BitchX style,
	// rather than under the text, so long messages waste no width.
	indent := strings.Repeat(" ", len(stamp(t))+1)
	urls := urlsOf(text)
	firstW := max(o.width-prefixW, 10)
	if token != "" {
		firstW = max(firstW-lipgloss.Width(token)-1, 10)
	}
	body := wrapText(text, firstW, max(o.width-len(indent), 10))
	anim = anim || len(urls) > 0
	for k, l := range body {
		styled := styleText(l, o, urls)
		if k == 0 {
			lead := prefix
			base := prefixW
			if token != "" {
				lead += clickable(token, linkStyle.Bold(true), o.hovered(spanMedia, m.ChatID+"\x00"+m.ID))
				base += lipgloss.Width(token)
				if l != "" {
					lead += " "
					base++
				}
			}
			add(lead+styled, append(first, linkSpans(l, base, urls)...)...)
		} else {
			add(indent+styled, linkSpans(l, len(indent), urls)...)
		}
	}
	if len(m.Buttons) > 0 {
		anim = true
		for _, l := range wrapText(buttonLine(m), max(o.width-len(indent), 10), max(o.width-len(indent), 10)) {
			add(indent+clickable(l, buttonStyle, o.hovered(spanButtons, m.Key())), span{len(indent), o.width, spanButtons, m.Key()})
		}
	}
	return anim
}

// linkSpans finds the links in one plain line of text that starts at
// column base. A link cut by wrapping maps to the full URL it belongs to
// when full is given.
func linkSpans(l string, base int, full []string) []span {
	var out []span
	for _, loc := range urlLocs(l) {
		target := wholeURL(l[loc[0]:loc[1]], loc[1] == len(l), full)
		out = append(out, span{base + lipgloss.Width(l[:loc[0]]), base + lipgloss.Width(l[:loc[1]]), spanURL, target})
	}
	return out
}

// mediaToken is how an attachment shows in the line: "[photo]" or
// "[file: name]".
func mediaToken(md *domain.Media) string {
	switch {
	case md.Kind == "photo":
		return "[photo]"
	case md.Name != "":
		return "[" + md.Kind + ": " + sanitize(md.Name, false) + "]"
	}
	return "[" + md.Kind + "]"
}

// buttonLine lists a message's buttons as "[1] OK  [2] Docs (link)  (msg 42)"
// so /press can refer to them.
func buttonLine(m domain.Message) string {
	var b strings.Builder
	for i, btn := range m.Buttons {
		if i > 0 {
			b.WriteString("  ")
		}
		fmt.Fprintf(&b, "[%d] %s", i+1, sanitize(btn.Label, false))
		switch btn.Kind {
		case domain.ButtonURL:
			b.WriteString(" (link)")
		case domain.ButtonText:
			b.WriteString(" (reply)")
		case domain.ButtonOther:
			b.WriteString(" (unsupported)")
		}
	}
	fmt.Fprintf(&b, "  (msg %s)", m.ID)
	return b.String()
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
		// A long line keeps its leading indent on every piece.
		lead := para[:len(para)-len(strings.TrimLeft(para, " "))]
		if len(lead) > limit()/2 {
			lead = ""
		}
		room := func() int { return max(limit()-len(lead), 1) }
		var cur string
		flush := func() {
			out = append(out, lead+cur)
			cur = ""
		}
		for _, w := range strings.Fields(para) {
			for lipgloss.Width(w) > room() {
				if cur != "" {
					flush()
				}
				head, rest := splitAt(w, room())
				out = append(out, lead+head)
				w = rest
			}
			switch {
			case cur == "":
				cur = w
			case lipgloss.Width(cur)+1+lipgloss.Width(w) <= room():
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

// topWindowStart is the first visible row of a list for topWindow.
func topWindowStart(n, height, sel int) int {
	sel = min(max(sel, 0), max(n-1, 0))
	return min(max(sel-height/2, 0), max(n-height, 0))
}

// topWindow returns exactly height lines of a list. The selected line sel
// moves down the screen until it reaches the middle, then stays there while
// the list scrolls under it, and moves on to the bottom only when the list
// ends (the same on the way up). Short content is padded at the bottom.
func topWindow(lines []string, height, sel int) []string {
	if height < 1 {
		return nil
	}
	start := topWindowStart(len(lines), height, sel)
	end := min(start+height, len(lines))
	view := append([]string(nil), lines[start:end]...)
	for len(view) < height {
		view = append(view, "")
	}
	return view
}

// window returns exactly height lines: the slice of lines ending offset lines
// above the bottom, padded at the top. It returns the clamped offset so
// callers can store it. It works for the lines' spans too.
func window[T any](lines []T, height, offset int) ([]T, int) {
	if height < 1 {
		return nil, 0
	}
	maxOff := max(len(lines)-height, 0)
	offset = min(max(offset, 0), maxOff)
	end := len(lines) - offset
	start := max(end-height, 0)
	view := make([]T, 0, height)
	for i := 0; i < height-(end-start); i++ {
		var zero T
		view = append(view, zero)
	}
	return append(view, lines[start:end]...), offset
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
