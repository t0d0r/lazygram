package tui

import (
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/rivo/uniseg"

	"lazygram/internal/domain"
	"lazygram/internal/logo"
)

func colorFor(s string) lipgloss.Color {
	h := fnv.New32a()
	h.Write([]byte(s))
	return palette[int(h.Sum32())%len(palette)]
}

// note is a client-side line in a window: "-!- connected", /list output, ...
// A raw note is shown as it is, without time stamp or marker: the banner.
type note struct {
	at    time.Time
	text  string
	raw   bool // the banner: coloured lines, no stamp
	plain bool // a /help or /keys line in the note colour, no stamp or marker; laid out by refRows
	pre   bool // lines already styled (a picture): emitted as they are, indented
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
	spanAct                     // a list row's action button; target is "row:action"
)

// span is a clickable range of columns on one rendered line.
type span struct {
	x0, x1 int // columns, x1 exclusive
	kind   spanKind
	target string
}

func (s span) hit(x int) bool { return x >= s.x0 && x < s.x1 }

// urlRE ends a link at any Unicode white space (\s is ASCII only; \p{Z},
// \v and U+0085 add the rest of unicode.IsSpace), the blank that ends a
// word for /open and for selection, so a no-break space after a link is
// not taken into it; and at a quote, a backtick or an angle bracket,
// which the word rule drops too.
var urlRE = regexp.MustCompile("https?://[^\\s\\p{Z}\\v\\x{85}<>\"'`]+")

// urlLocs finds links in s. A link ends as a double-clicked word does
// (trimWord; specs/ui.md, Mouse): trailing sentence punctuation is dropped, and
// a closing bracket at the end is dropped when the link closes it more
// often than it opens it, so "(https://example.org/a_(b))." gives
// https://example.org/a_(b) and a click opens what a double click copies.
// A match with nothing left after the scheme ("https://!") is no link.
func urlLocs(s string) [][]int {
	var out [][]int
	for _, loc := range urlRE.FindAllStringIndex(s, -1) {
		u := s[loc[0]:loc[1]]
		for u != "" && !linkEnds(u) {
			u = u[:len(u)-1]
		}
		if u != "http://" && u != "https://" {
			out = append(out, []int{loc[0], loc[0] + len(u)})
		}
	}
	return out
}

// linkEnds reports whether u may end in its last byte: not sentence
// punctuation, and a closing bracket only when u opens it at least as
// often. The scheme's "//" always stays.
func linkEnds(u string) bool {
	c := u[len(u)-1:]
	switch {
	case strings.Contains(wordStop, c):
		return false
	case strings.Contains(wordClose, c):
		return strings.Count(u, partner(c)) >= strings.Count(u, c)
	}
	return true
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
	phase     int                        // hover time in milliseconds for the light; -1 means none
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

// travelMs is how long the light takes from one end of an element to the
// other, whatever its length: long links and short nicks take the same.
const travelMs = 400

// bouncePos is where the single light sits on an element of n characters
// after ms milliseconds of hovering: it runs to the end and back,
// ping-pong, at a speed that keeps the travel time constant.
func bouncePos(ms, n int) int {
	if n <= 1 {
		return 0
	}
	t := ms % (2 * travelMs)
	if t > travelMs {
		t = 2*travelMs - t
	}
	return (t*(n-1) + travelMs/2) / travelMs
}

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

// clickable paints text as something the mouse can act on, with one bright
// character bouncing back and forth over it; phase is the hover time in
// milliseconds (none when phase < 0).
func clickable(text string, base lipgloss.Style, phase int) string {
	if phase < 0 {
		return base.Render(text)
	}
	runes := []rune(text)
	at := bouncePos(phase, len(runes))
	var b strings.Builder
	for i, r := range runes {
		if i == at {
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
			if notes[j].raw { // the banner, in the widest face that fits
				for _, l := range Banner(strings.Split(sanitize(logo.Pick(o.width), true), "\n")) {
					add(l)
				}
				j++
				continue
			}
			if notes[j].pre {
				for _, l := range strings.Split(notes[j].text, "\n") {
					add("      " + l)
				}
				j++
				continue
			}
			if notes[j].plain {
				text := sanitize(notes[j].text, true)
				full := urlsOf(text)
				for _, l := range refRows(text, max(o.width, 10)) {
					add(noteStyle.Render(l), linkSpans(l, 0, full)...)
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
		if c, ok := o.cache.get(key); ok && key != "" {
			for k := range c.lines {
				add(c.lines[k], c.spans[k]...)
			}
			continue
		}
		mark := len(out)
		anim := renderMessage(m, o, add)
		if o.cache != nil && key != "" {
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
	if o.phase >= 0 && o.hover != "" && hasElement(m, o.hover) {
		return "" // the hovered message changes every tick: not cached
	}
	return fmt.Sprintf("%s|%x|%d|%d|%t|%t|%s|%d", m.Key(), h.Sum32(), len(m.Buttons), o.width, o.showChat, o.fullDate, o.highlight, o.nameW)
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

// minDescRoom is the fewest columns a reference entry's description gets
// beside its names; with less it goes on the rows under them instead.
const minDescRoom = 20

// refEntry splits a line of the /help or /keys reference into the lead
// (two spaces, the names and the spaces after them) and the description.
// An entry starts with exactly two spaces and ends its names at the
// first run of two or more spaces; any other line is no entry, and so is
// one holding a line break, which would split the lead's row.
func refEntry(l string) (lead, names, desc string, ok bool) {
	if !strings.HasPrefix(l, "  ") || len(l) < 3 || l[2] == ' ' || strings.Contains(l, "\n") {
		return "", "", "", false
	}
	names, rest, found := strings.Cut(l[2:], "  ")
	desc = strings.TrimLeft(rest, " ")
	if !found || desc == "" {
		return "", "", "", false
	}
	return l[:len(l)-len(desc)], names, desc, true
}

// refRows lays out one logical line of the /help or /keys reference in a
// window width columns wide. An entry's description wraps with its
// continuation rows indented to where it starts (a hanging indent), so it
// uses the whole width and re-flows on resize; when the description would
// get fewer than minDescRoom columns beside the names, it goes on the rows
// under them, indented 4. Any other line is a paragraph wrapped to the
// width.
func refRows(l string, width int) []string {
	lead, names, desc, ok := refEntry(l)
	if !ok {
		return wrapText(l, width, width)
	}
	hang := lipgloss.Width(lead)
	if width-hang < minDescRoom {
		rows := wrapText("  "+names, width, width)
		room := max(width-4, 1)
		for _, d := range wrapText(desc, room, room) {
			rows = append(rows, "    "+d)
		}
		return rows
	}
	rows := wrapText(desc, width-hang, width-hang)
	for k := range rows {
		if k == 0 {
			rows[k] = lead + rows[k]
		} else {
			rows[k] = strings.Repeat(" ", hang) + rows[k]
		}
	}
	return rows
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
	hits := hitSpans(s, needle)
	if len(hits) == 0 {
		return s
	}
	var b strings.Builder
	last := 0
	for _, h := range hits {
		b.WriteString(s[last:h[0]])
		b.WriteString(hiStyle.Render(s[h[0]:h[1]]))
		last = h[1]
	}
	b.WriteString(s[last:])
	return b.String()
}

// hitSpans lists the byte ranges of s, left to right and not overlapping,
// that equal needle lower-cased rune by rune (as strings.ToLower does).
// It compares on s itself: lower-casing changes the byte length of some
// letters (U+0130, U+212A, U+023A), so offsets into strings.ToLower(s)
// are not offsets into s and could cut a character in two.
func hitSpans(s, needle string) [][2]int {
	want := []rune(needle)
	for k, r := range want {
		want[k] = unicode.ToLower(r)
	}
	var out [][2]int
	for i := 0; i < len(s) && len(want) > 0; {
		j := i
		for _, w := range want {
			r, n := utf8.DecodeRuneInString(s[j:])
			if n == 0 || unicode.ToLower(r) != w {
				j = -1
				break
			}
			j += n
		}
		if j >= 0 {
			out = append(out, [2]int{i, j})
			i = j
			continue
		}
		_, n := utf8.DecodeRuneInString(s[i:])
		i += n
	}
	return out
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

// splitAt splits s after at most width display columns (at least one
// character). It steps by grapheme clusters, measured with lipgloss.Width
// as wrapText measures lines: counted rune by rune, an emoji with a
// variation selector (U+2764 U+FE0F) came out one column short, so a row
// of them overflowed the line.
func splitAt(s string, width int) (string, string) {
	w, at, state := 0, 0, -1
	for rest := s; rest != ""; {
		var c string
		c, rest, _, state = uniseg.FirstGraphemeClusterInString(rest, state)
		cw := lipgloss.Width(c)
		if w+cw > width && at > 0 {
			return s[:at], s[at:]
		}
		w += cw
		at += len(c)
	}
	return s, ""
}

// fitLine truncates s to width columns and pads it with spaces, so bars
// never wrap however long their content is. It pads until the row
// measures width: a prepend mark at the end (U+0600..U+0605, Arabic
// number signs) joins the first space into its grapheme cluster, so one
// round of padding comes up a column short. A space adds at most one
// column, so the loop never overshoots.
func fitLine(s string, width int) string {
	s = ansi.Truncate(s, width, "")
	for w := lipgloss.Width(s); w < width; w = lipgloss.Width(s) {
		s += strings.Repeat(" ", width-w)
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
