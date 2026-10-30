// Package tui is the terminal front end: one chronological feed across all
// chats, a history pager, a chat list, and a slash-command input line.
package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lazygram/internal/command"
	"lazygram/internal/domain"
	"lazygram/internal/provider"
	"lazygram/internal/store"
)

type mode int

const (
	modeFeed mode = iota
	modeHistory
	modeChats
	modeHelp
)

// Options configures the model.
type Options struct {
	Store    *store.Store
	Provider provider.Provider
	Poll     time.Duration // background sync interval; zero disables
}

// Model is the bubbletea model.
type Model struct {
	opt   Options
	chats []domain.Chat

	mode     mode
	sel      domain.Chat // selected chat; zero ID means all chats
	filter   string      // feed text filter
	histChat string      // chat ID scoping the history pager; empty means all
	input    []rune
	cursor   int // chat list cursor
	scroll   int // lines above the bottom
	width    int
	height   int

	lastLines int // pager/feed line count at the last render
	syncing   bool
	lastSync  time.Time
	status    string
	isErr     bool
	quitting  bool
}

type (
	chatsMsg struct {
		chats []domain.Chat
		err   error
	}
	syncedMsg struct {
		added int
		err   error
		at    time.Time
	}
	tickMsg time.Time
)

// New builds a model. Chats cached in the store are available immediately.
func New(opt Options) *Model {
	return &Model{opt: opt, chats: opt.Store.Chats(), width: 100, height: 30,
		status: "type /help for commands"}
}

// Init starts the first chat fetch, sync, and poll timer.
func (m *Model) Init() tea.Cmd {
	m.syncing = true
	cmds := []tea.Cmd{m.chatsCmd(), m.syncCmd()}
	if m.opt.Poll > 0 {
		cmds = append(cmds, m.tickCmd())
	}
	return tea.Batch(cmds...)
}

func (m *Model) tickCmd() tea.Cmd {
	return tea.Tick(m.opt.Poll, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m *Model) chatsCmd() tea.Cmd {
	p, st := m.opt.Provider, m.opt.Store
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		chats, err := p.Chats(ctx)
		if err == nil {
			err = st.SetChats(chats)
		}
		return chatsMsg{chats, err}
	}
}

func (m *Model) syncCmd() tea.Cmd {
	p, st := m.opt.Provider, m.opt.Store
	since := domain.Latest(st.Messages())
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		msgs, err := p.Sync(ctx, since)
		if err != nil {
			return syncedMsg{err: err, at: time.Now()}
		}
		n, err := st.Add(msgs)
		return syncedMsg{added: n, err: err, at: time.Now()}
	}
}

// Update handles messages.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case chatsMsg:
		if msg.err != nil {
			m.setErr("chats: " + errText(msg.err))
		} else {
			m.chats = msg.chats
		}
	case syncedMsg:
		m.syncing = false
		m.lastSync = msg.at
		switch {
		case msg.err != nil:
			m.setErr("sync: " + errText(msg.err))
		case msg.added > 0:
			m.afterNewMessages(msg.added)
		default:
			m.setInfo("synced, nothing new")
		}
	case tickMsg:
		cmds := []tea.Cmd{m.tickCmd()}
		if !m.syncing {
			m.syncing = true
			cmds = append(cmds, m.syncCmd())
		}
		return m, tea.Batch(cmds...)
	case tea.KeyMsg:
		return m.key(msg)
	}
	return m, nil
}

// afterNewMessages keeps a scrolled-back view anchored on the same lines.
func (m *Model) afterNewMessages(n int) {
	m.setInfo(fmt.Sprintf("%d new message(s)", n))
	if m.scroll > 0 && (m.mode == modeFeed || m.mode == modeHistory) {
		m.scroll += max(len(m.pagerLines(m.opt.Store.Messages()))-m.lastLines, 0)
	}
}

// pagerLines renders the lines of the feed or the history pager.
func (m *Model) pagerLines(all []domain.Message) []string {
	o := lineOpts{width: m.width, aliasOf: m.aliasOf, aliasW: m.aliasWidth()}
	if m.mode == modeHistory {
		q := m.listText()
		o.fullDate, o.highlight = true, q
		return renderLines(domain.Filter(all, domain.Query{ChatID: m.histChat, Text: q}), o)
	}
	o.highlight = m.filter
	return renderLines(domain.Filter(all, domain.Query{ChatID: m.sel.ID, Text: m.filter}), o)
}

func errText(err error) string {
	if errors.Is(err, provider.ErrNotImplemented) {
		return "live connector not available in this build; run with LAZYGRAM_PROVIDER=demo"
	}
	return err.Error()
}

func (m *Model) setInfo(s string) { m.status, m.isErr = s, false }
func (m *Model) setErr(s string)  { m.status, m.isErr = s, true }

func (m *Model) key(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	page := max(m.contentHeight()-1, 1)
	switch k.Type {
	case tea.KeyCtrlC:
		m.quitting = true
		return m, tea.Quit
	case tea.KeyEsc:
		if m.mode != modeFeed {
			m.mode, m.input, m.scroll = modeFeed, nil, 0
		} else {
			m.input = nil
		}
	case tea.KeyEnter:
		return m.enter()
	case tea.KeyBackspace:
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
			m.cursor = 0
		}
	case tea.KeyCtrlU:
		m.input, m.cursor = nil, 0
	case tea.KeyCtrlW:
		s := strings.TrimRight(string(m.input), " ")
		if i := strings.LastIndex(s, " "); i >= 0 {
			m.input = []rune(s[:i+1])
		} else {
			m.input = nil
		}
	case tea.KeyPgUp:
		m.scroll += page
	case tea.KeyPgDown:
		m.scroll = max(m.scroll-page, 0)
	case tea.KeyHome:
		m.scroll = 1 << 30
	case tea.KeyEnd:
		m.scroll = 0
	case tea.KeyUp:
		if m.mode == modeChats {
			m.cursor = max(m.cursor-1, 0)
		} else {
			m.scroll++
		}
	case tea.KeyDown:
		if m.mode == modeChats {
			m.cursor++
		} else {
			m.scroll = max(m.scroll-1, 0)
		}
	case tea.KeyTab:
		if m.mode == modeChats {
			m.cursor++
		} else {
			m.cycleChat(1)
		}
	case tea.KeyShiftTab:
		if m.mode == modeChats {
			m.cursor = max(m.cursor-1, 0)
		} else {
			m.cycleChat(-1)
		}
	case tea.KeyCtrlR:
		return m, m.refresh()
	case tea.KeyCtrlO:
		m.openHistory(m.sel.ID)
	case tea.KeyCtrlL:
		m.openChats("")
	case tea.KeyF1:
		m.mode = modeHelp
	case tea.KeySpace:
		m.input = append(m.input, ' ')
	case tea.KeyRunes:
		for _, r := range k.Runes {
			if r >= ' ' && r != 0x7f {
				m.input = append(m.input, r)
			}
		}
		m.cursor = 0
	}
	return m, nil
}

func (m *Model) refresh() tea.Cmd {
	if m.syncing {
		m.setInfo("sync already running")
		return nil
	}
	m.syncing = true
	m.setInfo("syncing...")
	return tea.Batch(m.chatsCmd(), m.syncCmd())
}

// cycleChat moves the selection through all, chat1, chat2, ...
func (m *Model) cycleChat(d int) {
	n := len(m.chats) + 1
	cur := 0
	for i, c := range m.chats {
		if c.ID == m.sel.ID {
			cur = i + 1
		}
	}
	next := ((cur+d)%n + n) % n
	m.selectChat(chatAt(m.chats, next))
}

func chatAt(chats []domain.Chat, i int) domain.Chat {
	if i <= 0 || i > len(chats) {
		return domain.Chat{}
	}
	return chats[i-1]
}

func (m *Model) selectChat(c domain.Chat) {
	m.sel, m.scroll = c, 0
	if c.ID == "" {
		m.setInfo("showing all chats")
	} else {
		m.setInfo("showing #" + c.Alias)
	}
}

func (m *Model) openHistory(chatID string) {
	m.mode, m.histChat, m.scroll, m.input = modeHistory, chatID, 0, nil
}

func (m *Model) openChats(filter string) {
	m.mode, m.input, m.cursor = modeChats, []rune(filter), 0
}

func (m *Model) enter() (tea.Model, tea.Cmd) {
	line := strings.TrimSpace(string(m.input))
	if !strings.HasPrefix(line, "/") {
		if m.mode == modeChats {
			items := m.chatItems()
			if len(items) > 0 {
				m.selectChat(items[min(m.cursor, len(items)-1)])
			}
			m.mode, m.input = modeFeed, nil
		} else if line != "" {
			m.input = nil
			m.setErr("not a command (start with /): sending is not supported yet")
		}
		return m, nil
	}
	m.input = nil
	cmd, err := command.Parse(line)
	if err != nil {
		m.setErr(err.Error())
		return m, nil
	}
	return m.run(cmd)
}

func (m *Model) run(c command.Command) (tea.Model, tea.Cmd) {
	switch c.Kind {
	case command.Help:
		m.mode = modeHelp
	case command.Quit:
		m.quitting = true
		return m, tea.Quit
	case command.Refresh:
		return m, m.refresh()
	case command.Chats:
		m.openChats(c.Arg)
	case command.Filter:
		m.filter, m.scroll = c.Arg, 0
		if c.Arg == "" {
			m.setInfo("filter cleared")
		} else {
			m.setInfo("filter: " + c.Arg)
		}
	case command.Select:
		ch, err := domain.ResolveChat(m.chats, c.Arg)
		if err != nil {
			m.setErr(err.Error())
			break
		}
		m.mode = modeFeed
		m.selectChat(ch)
	case command.All:
		m.mode, m.filter = modeFeed, ""
		m.selectChat(domain.Chat{})
	case command.History:
		id := m.sel.ID
		if c.Arg != "" {
			ch, err := domain.ResolveChat(m.chats, c.Arg)
			if err != nil {
				m.setErr(err.Error())
				break
			}
			id = ch.ID
		}
		m.openHistory(id)
	case command.Search:
		m.openHistory(m.sel.ID)
		m.input = []rune(c.Arg)
	}
	return m, nil
}

// chatItems is the chat list: an "all" row followed by filtered chats.
func (m *Model) chatItems() []domain.Chat {
	return append([]domain.Chat{{}}, domain.FilterChats(m.chats, m.listText())...)
}

// listText is the typed text when it is not a slash command.
func (m *Model) listText() string {
	s := string(m.input)
	if strings.HasPrefix(s, "/") {
		return ""
	}
	return s
}

func (m *Model) contentHeight() int { return max(m.height-3, 1) }

func (m *Model) aliasOf(chatID string) string {
	for _, c := range m.chats {
		if c.ID == chatID {
			return c.Alias
		}
	}
	return chatID
}

func (m *Model) aliasWidth() int {
	w := 4
	for _, c := range m.chats {
		w = max(w, lipgloss.Width(c.Alias))
	}
	return min(w, 14)
}

// Quitting reports whether the user asked to leave.
func (m *Model) Quitting() bool { return m.quitting }

// View renders the whole screen.
func (m *Model) View() string {
	if m.quitting {
		return ""
	}
	var topic, prompt string
	var body []string
	h := m.contentHeight()
	all := m.opt.Store.Messages()

	switch m.mode {
	case modeHelp:
		topic = "help"
		body = strings.Split(command.HelpText+"\n\n"+keyHelp, "\n")
		body, _ = window(body, h, 0)
		prompt = "press Esc to return > "
	case modeChats:
		items := m.chatItems()
		m.cursor = min(max(m.cursor, 0), len(items)-1)
		topic = fmt.Sprintf("chats (%d) - Up/Down, Enter selects, Esc cancels", len(m.chats))
		body = m.chatLines(items, all)
		body, _ = window(body, h, 0)
		prompt = "filter chats > "
	case modeHistory:
		scope := "all chats"
		if m.histChat != "" {
			scope = "#" + m.aliasOf(m.histChat)
		}
		q := m.listText()
		hits := domain.Filter(all, domain.Query{ChatID: m.histChat, Text: q})
		lines := m.pagerLines(all)
		m.lastLines = len(lines)
		body, m.scroll = window(lines, h, m.scroll)
		topic = fmt.Sprintf("history: %s - %d of %d messages - PgUp/PgDn scroll, type to search, Esc closes",
			scope, len(hits), len(domain.Filter(all, domain.Query{ChatID: m.histChat})))
		prompt = "search > "
	default:
		lines := m.pagerLines(all)
		m.lastLines = len(lines)
		body, m.scroll = window(lines, h, m.scroll)
		topic = "lazygram - " + m.opt.Provider.Name() + " - " + m.scopeLabel()
		prompt = m.scopeLabel() + " > "
	}

	var b strings.Builder
	b.WriteString(barStyle.Width(m.width).MaxWidth(m.width).Render(topic))
	b.WriteByte('\n')
	for _, l := range body {
		b.WriteString(lipgloss.NewStyle().MaxWidth(m.width).Render(l))
		b.WriteByte('\n')
	}
	b.WriteString(m.statusLine(len(all)))
	b.WriteByte('\n')
	b.WriteString(m.inputLine(prompt))
	return b.String()
}

func (m *Model) scopeLabel() string {
	s := "#all"
	if m.sel.ID != "" {
		s = "#" + m.sel.Alias
	}
	if m.filter != "" {
		s += " /" + m.filter
	}
	return s
}

func (m *Model) chatLines(items []domain.Chat, all []domain.Message) []string {
	counts := map[string]int{}
	last := map[string]time.Time{}
	for _, msg := range all {
		counts[msg.ChatID]++
		last[msg.ChatID] = msg.Time
	}
	var out []string
	for i, c := range items {
		name, title, n, seen := "all chats", "everything, interleaved", len(all), domain.Latest(all)
		if c.ID != "" {
			name, title, n, seen = "#"+c.Alias, c.Title, counts[c.ID], last[c.ID]
		}
		when := "-"
		if !seen.IsZero() {
			when = seen.Local().Format("2006-01-02 15:04")
		}
		mark := " "
		if c.ID == m.sel.ID {
			mark = "*"
		}
		row := fmt.Sprintf("%s %-16s %-28s %6d msgs  last %s", mark, name, title, n, when)
		if i == m.cursor {
			row = hiStyle.Render(row)
		}
		out = append(out, row)
	}
	if len(items) == 1 && len(m.chats) > 0 {
		out = append(out, dimStyle.Render("  no chat matches the filter"))
	}
	return out
}

func (m *Model) statusLine(total int) string {
	sync := "never synced"
	if !m.lastSync.IsZero() {
		sync = "synced " + m.lastSync.Format("15:04:05")
	}
	if m.syncing {
		sync = "syncing"
	}
	left := fmt.Sprintf(" [%s] [%s] [%d msgs] [%d chats] [%s] ",
		time.Now().Format("15:04"), m.opt.Provider.Name(), total, len(m.chats), sync)
	if m.scroll > 0 {
		left += fmt.Sprintf("[-%d lines] ", m.scroll)
	}
	text := infoStyle.Render(left)
	if m.status != "" {
		st := lipgloss.NewStyle().Background(lipgloss.Color("236"))
		if m.isErr {
			st = st.Foreground(lipgloss.Color("203")).Bold(true)
		} else {
			st = st.Foreground(lipgloss.Color("252"))
		}
		text += st.Render(m.status)
	}
	return lipgloss.NewStyle().Background(lipgloss.Color("236")).Width(m.width).MaxWidth(m.width).Render(text)
}

func (m *Model) inputLine(prompt string) string {
	room := max(m.width-lipgloss.Width(prompt)-1, 1)
	in := m.input
	if len(in) > room {
		in = in[len(in)-room:]
	}
	return dimStyle.Render(prompt) + string(in) + lipgloss.NewStyle().Reverse(true).Render(" ")
}

const keyHelp = `Keys
  Tab / Shift-Tab   cycle through chats (all, chat1, chat2, ...)
  PgUp / PgDn       scroll the feed or pager one page
  Up / Down         scroll one line (move in the chat list)
  Home / End        jump to oldest / newest
  Ctrl-O            open history for the selected chat
  Ctrl-L            chat list
  Ctrl-R            sync now
  F1                this help
  Esc               close view / clear input
  Ctrl-C            quit

The feed is one IRC-style channel: all chats interleaved by time.
History is the full local cache; type in it to search.`
