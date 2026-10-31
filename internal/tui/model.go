// Package tui is the terminal front end, laid out like BitchX/irssi: window
// 1 is the server window with every message and client notices, and a
// window per chat opens when that chat gets a message. A slash-command
// input line with Tab completion drives it.
package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"lazygram/internal/command"
	"lazygram/internal/domain"
	"lazygram/internal/logo"
	"lazygram/internal/opener"
	"lazygram/internal/provider"
	"lazygram/internal/store"
)

type mode int

const (
	modeWin   mode = iota // a window's scrollback
	modeList              // selectable list: chats (/list) or windows (/win)
	modePager             // searchable local history of the current window's chat
)

type listSrc int

const (
	listChats   listSrc = iota // every chat
	listWindows                // the open windows
)

const (
	fxKey          = "fx"    // store state key for the running lights
	mouseKey       = "mouse" // store state key for mouse reporting
	fxTick         = 30 * time.Millisecond
	maxHistory     = 200
	doubleCtrlX    = 600 * time.Millisecond
	layoutKey      = "windows" // store state key for the window layout
	themeKey       = "theme"   // store state key for the colour theme
	defaultBacklog = 2000      // cached messages loaded at start
	recentOnOpen   = 10        // messages fetched when a chat window first opens
	maxRecent      = 100
	maxNotes       = 500
)

// Options configures the model.
type Options struct {
	Store    *store.Store
	Provider provider.Provider
	Poll     time.Duration      // background sync interval; zero disables
	Backlog  int                // cached messages loaded at start; zero means defaultBacklog
	Version  string             // shown in the start banner
	Open     func(string) error // opens a link or file; nil means opener.Open
}

// win is one virtual window. wins[0] is the server window (chatID empty).
type win struct {
	chatID string
	unread int
	scroll int // lines above the bottom
	notes  []note
	loaded bool // recent history fetched once

	enterSends bool // Enter sends plain text (toggled by a double Ctrl-X)
}

// Model is the bubbletea model.
type Model struct {
	opt   Options
	chats []domain.Chat
	msgs  []domain.Message // in-memory mirror of the cache, chronological

	wins []*win
	cur  int
	mode mode

	input []rune
	comp  *completion

	history  []string // lines entered: commands and sent text, oldest first
	histIdx  int      // position while walking the history; len(history) is the new line
	histSave []rune   // the line being typed when the walk began

	clicks [][]span // what a click on each screen row does, as of the last View

	viewDir   string   // private temporary directory holding plain copies for the viewer
	viewFiles []string // the copies made so far

	hover string // the clickable thing under the pointer, as a hover key

	mouseOn bool         // mouse reporting; off lets the terminal select text
	fx      bool         // running lights over clickable text
	hoverAt time.Time    // when the pointer reached the hovered element
	ticking bool         // an fx tick is scheduled
	animOn  bool         // the last View showed something animated
	rcache  *renderCache // rendered messages, reused across ticks

	escPending bool         // an Esc was just pressed: a digit now switches windows
	escSaved   []rune       // the input as it was before that Esc cleared it
	lastCtrlX  time.Time    // for the double Ctrl-X toggle
	pending    *pendingSend // text waiting out the double Ctrl-X window before it is sent
	pendingSeq int

	listCur     int     // selected row in the list view
	listSrc     listSrc // what the list view shows
	showDeleted bool    // /list --show-deleted

	width, height int
	lastLines     int    // rendered line count of the current window at the last View
	modeInfo      string // what the list or pager shows, for the status line

	started  time.Time
	seen     int             // messages that arrived this session
	sentN    int             // messages sent this session
	pressedN int             // bot buttons pressed
	fetchedN int             // attachments fetched
	openedN  int             // windows opened this session
	cmdsN    int             // commands run
	visited  map[string]bool // chat ids of windows shown
	syncing  bool
	loading  map[string]bool // chat ids with a Recent fetch in flight
	lastSync time.Time
	status   string
	isErr    bool
	quitting bool
}

type (
	chatsMsg struct {
		chats []domain.Chat
		err   error
	}
	syncedMsg struct {
		fresh []domain.Message
		err   error
		at    time.Time
	}
	recentMsg struct {
		chatID string
		fresh  []domain.Message
		err    error
	}
	sentMsg struct {
		chatID string
		err    error
	}
	pressedMsg struct {
		chatID string
		label  string
		answer domain.PressAnswer
		err    error
	}
	openedMsg struct {
		target string
		err    error
	}
	sendLaterMsg struct{ seq int } // the double Ctrl-X window has passed
	fetchedMsg   struct {
		chatID, msgID string
		name          string // file name for the viewer
		data          []byte // the attachment
		err           error
	}
	tickMsg   time.Time
	fxTickMsg struct{}
)

// New builds a model. Chats and messages cached in the store are available
// immediately.
func New(opt Options) *Model {
	if opt.Backlog <= 0 {
		opt.Backlog = defaultBacklog
	}
	if opt.Open == nil {
		opt.Open = opener.Open
	}
	m := &Model{opt: opt, width: 100, height: 30, loading: map[string]bool{}, visited: map[string]bool{},
		wins: []*win{{}}, status: "type /help for commands", started: time.Now(), fx: true, rcache: &renderCache{}}
	if found, err := opt.Store.State(fxKey, &m.fx); !found || err != nil {
		m.fx = true
	}
	if found, err := opt.Store.State(mouseKey, &m.mouseOn); !found || err != nil {
		m.mouseOn = true
	}
	m.chats, _ = opt.Store.Chats()
	m.msgs, _ = opt.Store.Messages("", opt.Backlog)
	// The banner has the zero time so it stays above everything, however old
	// the cached messages are.
	m.wins[0].notes = append(m.wins[0].notes, note{text: logo.Start, raw: true})
	m.note(m.wins[0], Tagline(opt.Version))
	m.note(m.wins[0], "/help lists commands, /list the chats")
	m.restoreLayout()
	m.restoreTheme()
	return m
}

func (m *Model) restoreTheme() {
	var name string
	if found, err := m.opt.Store.State(themeKey, &name); !found || err != nil {
		return
	}
	if t, ok := lookupTheme(name); ok {
		applyTheme(t)
	}
}

// setTheme switches the colours and remembers the choice.
func (m *Model) setTheme(name string) {
	t, ok := lookupTheme(name)
	if !ok {
		m.setErr("no theme " + name + "; /theme lists them")
		return
	}
	applyTheme(t)
	if err := m.opt.Store.SetState(themeKey, t.Name); err != nil {
		m.setErr("theme not saved: " + err.Error())
		return
	}
	m.setInfo("theme: " + t.Name)
}

// Tagline is the one-line description under the banners.
func Tagline(version string) string {
	if version == "" {
		version = "dev"
	}
	return "lazygram " + version + " - IRC-style Telegram client"
}

// ExitBanner is what the terminal shows after lazygram ends, BitchX style:
// the wide logo when the terminal has room, then the tagline and the
// session summary.
func ExitBanner(version string, width int, summary string) string {
	art := logo.Exit
	if width >= logo.Width(logo.ExitWide) {
		art = logo.ExitWide
	}
	out := strings.Join(Banner(logo.Lines(art)), "\n") + "\n" + dimStyle.Render(Tagline(version)+" by t0d0r - thanks for using lazygram")
	for _, l := range strings.Split(summary, "\n") {
		if l != "" {
			out += "\n" + dimStyle.Render("  "+l)
		}
	}
	return out + "\n"
}

// Summary is the session's statistics for the exit banner, one item per
// line, in the spirit of BitchX's parting words.
func (m *Model) Summary() string {
	cached, _ := m.opt.Store.Count("")
	return strings.Join([]string{
		fmt.Sprintf("time wasted: %s", time.Since(m.started).Truncate(time.Second)),
		fmt.Sprintf("messages: %d arrived, %d sent, %d buttons pressed, %d files fetched, %d in the cache",
			m.seen, m.sentN, m.pressedN, m.fetchedN, cached),
		fmt.Sprintf("windows: %d open at exit, %d opened, %d chats visited; %d commands run",
			len(m.wins)-1, m.openedN, len(m.visited), m.cmdsN),
	}, "\n")
}

// About prints what lazygram is, who made it and what it runs on.
func (m *Model) about() {
	w := m.wins[m.cur]
	v := m.opt.Version
	if v == "" {
		v = "dev"
	}
	for _, l := range []string{
		"lazygram " + v + " - IRC-style terminal client for Telegram, in the spirit of BitchX and nchat",
		"  by t0d0r (Todor Dragnev) - https://github.com/t0d0r/lazygram - BSD-2-Clause",
		"  built with " + runtime.Version() + " for " + runtime.GOOS + "/" + runtime.GOARCH +
			"; gotd/td (MTProto), bubbletea, lipgloss, modernc.org/sqlite",
		fmt.Sprintf("  provider %s, theme %s, up %s", m.opt.Provider.Name(), currentName, time.Since(m.started).Truncate(time.Second)),
		"  /help for commands, lazygram update for the latest release",
	} {
		m.note(w, l)
	}
}

// Width is the terminal width as last reported.
func (m *Model) Width() int { return m.width }

// Init starts the first chat fetch, sync, and poll timer, and loads the
// restored current window.
func (m *Model) Init() tea.Cmd {
	m.syncing = true
	cmds := []tea.Cmd{m.chatsCmd(), m.syncCmd()}
	if m.opt.Poll > 0 {
		cmds = append(cmds, m.tickCmd())
	}
	if m.cur > 0 {
		cmds = append(cmds, m.switchTo(m.cur))
	}
	if !m.mouseOn { // the program starts with the mouse on; honour the saved choice
		cmds = append(cmds, tea.DisableMouse)
	}
	return tea.Batch(cmds...)
}

// layout is what survives a restart: the open windows, the current one
// and which windows send on Enter.
type layout struct {
	Windows []string `json:"windows"` // chat ids of windows 1..N
	Current int      `json:"current"`
	Enter   []bool   `json:"enter,omitempty"` // Enter sends, per window in Windows
}

func (m *Model) saveLayout() {
	l := layout{Current: m.cur}
	for _, w := range m.wins[1:] {
		l.Windows = append(l.Windows, w.chatID)
		l.Enter = append(l.Enter, w.enterSends)
	}
	if err := m.opt.Store.SetState(layoutKey, l); err != nil {
		m.setErr("layout not saved: " + err.Error())
	}
}

func (m *Model) restoreLayout() {
	var l layout
	if found, err := m.opt.Store.State(layoutKey, &l); !found || err != nil {
		return
	}
	for i, id := range l.Windows {
		if _, w := m.winFor(id); id != "" && w == nil {
			m.wins = append(m.wins, &win{chatID: id, enterSends: i < len(l.Enter) && l.Enter[i]})
		}
	}
	m.cur = min(max(l.Current, 0), len(m.wins)-1)
	if n := len(m.wins) - 1; n > 0 {
		m.note(m.wins[0], fmt.Sprintf("%d windows reopened from the last session", n))
	}
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
	since := domain.Latest(m.msgs)
	return func() tea.Msg {
		// Long enough for a FLOOD_WAIT retry; polls skip while a sync runs.
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		msgs, err := p.Sync(ctx, since)
		// A provider may return what it fetched before failing part-way, and
		// its cursors already cover those messages: store them regardless.
		fresh, addErr := st.Add(msgs)
		if err == nil {
			err = addErr
		}
		return syncedMsg{fresh: fresh, err: err, at: time.Now()}
	}
}

func (m *Model) recentCmd(chatID string, n int) tea.Cmd {
	p, st := m.opt.Provider, m.opt.Store
	m.loading[chatID] = true
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		msgs, err := p.Recent(ctx, chatID, n)
		if err != nil {
			return recentMsg{chatID: chatID, err: err}
		}
		fresh, err := st.Add(msgs)
		return recentMsg{chatID: chatID, fresh: fresh, err: err}
	}
}

func (m *Model) sendCmd(chatID, text string) tea.Cmd {
	p := m.opt.Provider
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return sentMsg{chatID: chatID, err: p.Send(ctx, chatID, text)}
	}
}

func (m *Model) pressCmd(chatID string, msg domain.Message, b domain.Button) tea.Cmd {
	p, st := m.opt.Provider, m.opt.Store
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		a, err := p.Press(ctx, chatID, msg.ID, b)
		if err == nil && a.Updated != nil {
			err = st.Replace(*a.Updated)
		}
		return pressedMsg{chatID: chatID, label: b.Label, answer: a, err: err}
	}
}

// fetchCmd gets an attachment: from the sealed vault when it is there,
// else downloaded and sealed into the vault. A second request for the
// same attachment while one is in flight is ignored.
func (m *Model) fetchCmd(chatID, msgID string) tea.Cmd {
	key := chatID + "\x00" + msgID
	if m.loading[key] {
		m.setInfo("already fetching")
		return nil
	}
	m.loading[key] = true
	p, st := m.opt.Provider, m.opt.Store
	md := m.media(chatID, msgID)
	return func() tea.Msg {
		if data, found, err := st.Media(chatID, msgID); err != nil {
			return fetchedMsg{chatID: chatID, msgID: msgID, err: err}
		} else if found {
			return fetchedMsg{chatID: chatID, msgID: msgID, name: viewName(md, msgID, ""), data: data}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		data, got, err := p.Fetch(ctx, chatID, msgID)
		if err != nil {
			return fetchedMsg{chatID: chatID, msgID: msgID, err: err}
		}
		if err := st.PutMedia(chatID, msgID, data); err != nil {
			return fetchedMsg{chatID: chatID, msgID: msgID, err: err}
		}
		return fetchedMsg{chatID: chatID, msgID: msgID, name: viewName(md, msgID, got), data: data}
	}
}

// media is a message's attachment description, if any.
func (m *Model) media(chatID, msgID string) *domain.Media {
	for _, x := range m.msgs {
		if x.ChatID == chatID && x.ID == msgID {
			return x.Media
		}
	}
	return nil
}

// viewName is the file name of the plain copy made for viewing. Images get
// the provider's name (its extension follows the content type, never the
// sender); other files keep the sender's base name, and are never opened.
func viewName(md *domain.Media, msgID, fetched string) string {
	switch {
	case md != nil && !md.Image() && md.Name != "":
		return filepath.Base(md.Name)
	case fetched != "":
		return filepath.Base(fetched)
	case md.Image():
		return msgID + ".jpg"
	}
	return msgID + ".bin"
}

// viewCopy writes a plain copy of an attachment into a private temporary
// directory for the viewer; copies are removed by Cleanup.
func (m *Model) viewCopy(name string, data []byte) (string, error) {
	if m.viewDir == "" {
		dir, err := os.MkdirTemp("", "lazygram-view-")
		if err != nil {
			return "", err
		}
		if err := os.Chmod(dir, 0o700); err != nil {
			return "", err
		}
		m.viewDir = dir
	}
	path := filepath.Join(m.viewDir, fmt.Sprintf("%d-%s", len(m.viewFiles)+1, sanitizeName(name)))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	m.viewFiles = append(m.viewFiles, path)
	return path, nil
}

// sanitizeName keeps a file name harmless: base name only, control
// characters and leading dots dropped.
func sanitizeName(name string) string {
	name = strings.TrimLeft(filepath.Base(sanitize(name, false)), ".")
	if name == "" || name == "/" || name == "\\" {
		return "attachment"
	}
	return name
}

// imageFile reports whether a plain copy may go to the system viewer: only
// raster image extensions, whatever the message claimed.
func imageFile(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		return true
	}
	return false
}

// Cleanup removes the plain copies made for viewing. Call it on exit.
func (m *Model) Cleanup() {
	for _, f := range m.viewFiles {
		_ = os.Remove(f)
	}
	if m.viewDir != "" {
		_ = os.Remove(m.viewDir)
	}
	m.viewFiles, m.viewDir = nil, ""
}

func (m *Model) openCmd(target string) tea.Cmd {
	open := m.opt.Open
	return func() tea.Msg { return openedMsg{target: target, err: open(target)} }
}

// Update handles messages. After every message the running lights are
// kept going while something animated is on screen.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	model, cmd := m.update(msg)
	// Decide from the hover state as it is now, not from the last render:
	// the first hover after an idle screen must start the clock at once.
	if m.fx && m.hover != "" && !m.ticking {
		m.ticking = true
		tick := tea.Tick(fxTick, func(time.Time) tea.Msg { return fxTickMsg{} })
		if cmd == nil {
			return model, tick
		}
		return model, tea.Batch(cmd, tick)
	}
	return model, cmd
}

func (m *Model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case fxTickMsg:
		m.ticking = false // the next View reads the hover clock
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case chatsMsg:
		if msg.err != nil {
			m.setErr("chats: " + errText(msg.err))
			m.note(m.wins[0], "chats: "+errText(msg.err))
			break
		}
		first := len(m.chats) == 0
		m.chats = msg.chats
		if first {
			m.openUnread()
		}
	case syncedMsg:
		m.syncing = false
		m.lastSync = msg.at
		if len(msg.fresh) > 0 {
			m.arrived(msg.fresh)
		}
		switch {
		case msg.err != nil:
			m.setErr("sync: " + errText(msg.err))
			m.note(m.wins[0], "sync: "+errText(msg.err))
		case len(msg.fresh) == 0:
			// Nothing to say: a status that pops up every poll only flickers.
		}
	case recentMsg:
		delete(m.loading, msg.chatID)
		_, w := m.winFor(msg.chatID)
		if w == nil {
			w = m.wins[0]
		}
		if msg.err != nil {
			m.setErr("history: " + errText(msg.err))
			m.note(w, "history: "+errText(msg.err))
			break
		}
		w.loaded = true
		m.absorb(msg.fresh)
		m.note(w, fmt.Sprintf("fetched %d older message(s)", len(msg.fresh)))
	case pressedMsg:
		_, w := m.winFor(msg.chatID)
		if w == nil {
			w = m.wins[0]
		}
		if msg.err != nil {
			m.setErr("press: " + errText(msg.err))
			m.note(w, "press "+msg.label+": "+errText(msg.err))
			break
		}
		if msg.answer.Updated != nil {
			m.msgs = domain.Replace(m.msgs, *msg.answer.Updated)
		}
		text := "pressed " + msg.label
		if msg.answer.Text != "" {
			text += ": " + msg.answer.Text
		}
		if msg.answer.URL != "" {
			text += " (open: " + msg.answer.URL + ")"
		}
		m.note(w, text)
		m.pressedN++
		m.setInfo("pressed " + msg.label)
	case sentMsg:
		_, w := m.winFor(msg.chatID)
		if w == nil {
			w = m.wins[0]
		}
		if msg.err != nil {
			m.setErr("send: " + errText(msg.err))
			m.note(w, "send failed: "+errText(msg.err))
			break
		}
		m.sentN++
		m.setInfo("sent")
		if !m.syncing { // show it as soon as the service has it
			m.syncing = true
			return m, m.syncCmd()
		}
	case tickMsg:
		cmds := []tea.Cmd{m.tickCmd()}
		if !m.syncing {
			m.syncing = true
			cmds = append(cmds, m.syncCmd())
		}
		return m, tea.Batch(cmds...)
	case fetchedMsg:
		delete(m.loading, msg.chatID+"\x00"+msg.msgID)
		_, w := m.winFor(msg.chatID)
		if w == nil {
			w = m.wins[0]
		}
		if msg.err != nil {
			m.setErr("fetch: " + errText(msg.err))
			m.note(w, "fetch: "+errText(msg.err))
			break
		}
		md := m.media(msg.chatID, msg.msgID)
		m.fetchedN++
		path, err := m.viewCopy(msg.name, msg.data)
		if err != nil {
			m.setErr("view: " + err.Error())
			m.note(w, "view: "+err.Error())
			break
		}
		if md.Image() && imageFile(path) {
			m.note(w, "opening "+path+" (plain copy, removed on exit; the vault keeps it encrypted)")
			return m, m.openCmd(path)
		}
		m.note(w, "decrypted to "+path+" until exit; kept encrypted in the vault")
		m.setInfo("decrypted to " + path)
	case sendLaterMsg:
		if p := m.pending; p != nil && p.seq == msg.seq {
			m.pending = nil
			m.remember(p.text)
			m.setInfo("sending to " + m.nameOf(p.chatID) + "...")
			return m, m.sendCmd(p.chatID, p.text)
		}
	case openedMsg:
		if msg.err != nil {
			m.setErr("open: " + msg.err.Error())
			m.note(m.wins[m.cur], "open "+msg.target+": "+msg.err.Error())
		} else {
			m.setInfo("opened " + msg.target)
		}
	case tea.MouseMsg:
		return m.mouse(msg)
	case tea.KeyMsg:
		return m.key(msg)
	}
	return m, nil
}

// exactChat finds the one chat whose alias or title equals name (case
// folded); a click on a nick never guesses by prefix.
func (m *Model) exactChat(name string) (domain.Chat, bool) {
	var hit domain.Chat
	n := 0
	for _, c := range m.chats {
		if !c.Deleted && (strings.EqualFold(c.Alias, name) || strings.EqualFold(c.Title, name)) {
			hit, n = c, n+1
		}
	}
	return hit, n == 1
}

// hoverKey names what the pointer is over, for the running light.
func hoverKey(s span) string { return fmt.Sprintf("%d:%s", s.kind, s.target) }

// mouse handles wheel scrolling, hovering and clicks on the spans of the
// last View.
func (m *Model) mouse(e tea.MouseMsg) (tea.Model, tea.Cmd) {
	w := m.wins[m.cur]
	if e.Action == tea.MouseActionMotion {
		was := m.hover
		m.hover = ""
		if e.Y >= 0 && e.Y < len(m.clicks) {
			for _, s := range m.clicks[e.Y] {
				if s.hit(e.X) {
					m.hover = hoverKey(s)
				}
			}
		}
		if m.hover != was {
			m.hoverAt = time.Now() // the light starts from the left edge
		}
		return m, nil
	}
	switch {
	case e.Button == tea.MouseButtonWheelUp:
		if m.mode == modeList {
			m.listCur = max(m.listCur-1, 0)
		} else {
			w.scroll += 3
		}
	case e.Button == tea.MouseButtonWheelDown:
		if m.mode == modeList {
			m.listCur++
		} else {
			w.scroll = max(w.scroll-3, 0)
		}
	case e.Action == tea.MouseActionPress && e.Button == tea.MouseButtonLeft:
		if e.Y < 0 || e.Y >= len(m.clicks) {
			return m, nil
		}
		for _, s := range m.clicks[e.Y] {
			if s.hit(e.X) {
				return m.click(s)
			}
		}
	}
	return m, nil
}

func (m *Model) click(s span) (tea.Model, tea.Cmd) {
	switch s.kind {
	case spanChat:
		i, w := m.winFor(s.target)
		if w == nil {
			i, _ = m.openWin(s.target)
		}
		return m, m.switchTo(i)
	case spanNick:
		c, ok := m.exactChat(s.target)
		if !ok {
			m.setErr("no chat named " + s.target)
			return m, nil
		}
		i, w := m.winFor(c.ID)
		if w == nil {
			i, _ = m.openWin(c.ID)
		}
		return m, m.switchTo(i)
	case spanURL:
		m.setInfo("opening " + s.target + "...")
		return m, m.openCmd(s.target)
	case spanMedia:
		chatID, msgID, _ := strings.Cut(s.target, "\x00")
		m.setInfo("fetching...")
		return m, m.fetchCmd(chatID, msgID)
	case spanEnd:
		m.wins[m.cur].scroll = 0
	case spanWin:
		i, _ := strconv.Atoi(s.target)
		return m, m.switchTo(i)
	case spanRow:
		m.listCur, _ = strconv.Atoi(s.target)
		return m.enter()
	}
	return m, nil
}

func errText(err error) string {
	if errors.Is(err, provider.ErrNotImplemented) {
		return "live connector not available; run lazygram demo"
	}
	return err.Error()
}

// setInfo and setErr show a status; remote text in it is stripped of
// control characters, as everywhere else.
func (m *Model) setInfo(s string) { m.status, m.isErr = sanitize(s, false), false }
func (m *Model) setErr(s string)  { m.status, m.isErr = sanitize(s, false), true }

// note appends a client line to a window.
func (m *Model) note(w *win, text string) {
	w.notes = append(w.notes, note{at: time.Now(), text: text})
	if len(w.notes) > maxNotes {
		w.notes = w.notes[len(w.notes)-maxNotes:]
	}
}

// absorb merges messages into the in-memory mirror.
func (m *Model) absorb(fresh []domain.Message) {
	if len(fresh) == 0 {
		return
	}
	before := len(m.lines())
	m.msgs = domain.Merge(m.msgs, fresh)
	if w := m.wins[m.cur]; w.scroll > 0 && m.mode == modeWin {
		w.scroll += max(len(m.lines())-before, 0) // stay anchored while scrolled back
	}
}

// arrived handles messages that just came in: windows open for their chats
// and activity is counted.
func (m *Model) arrived(fresh []domain.Message) {
	m.absorb(fresh)
	for _, msg := range fresh {
		i, w := m.winFor(msg.ChatID)
		if w == nil {
			if c, ok := m.chat(msg.ChatID); ok && c.Deleted {
				continue
			}
			i, w = m.openWin(msg.ChatID)
		}
		if i != m.cur {
			w.unread++
		}
	}
	m.seen += len(fresh)
	m.setInfo(fmt.Sprintf("%d new message(s)", len(fresh)))
}

// openUnread opens windows for chats the service reports as unread.
func (m *Model) openUnread() {
	n := 0
	for _, c := range m.chats {
		if c.Unread == 0 || c.Deleted {
			continue
		}
		if _, w := m.winFor(c.ID); w == nil {
			_, w = m.openWin(c.ID)
			w.unread = c.Unread
			n++
		}
	}
	m.note(m.wins[0], fmt.Sprintf("%d chats, %d with unread messages", len(m.chats), n))
}

func (m *Model) chat(chatID string) (domain.Chat, bool) {
	for _, c := range m.chats {
		if c.ID == chatID {
			return c, true
		}
	}
	return domain.Chat{}, false
}

// nameOf is the chat's IRC-style name, or its id when the chat is unknown.
func (m *Model) nameOf(chatID string) string {
	if c, ok := m.chat(chatID); ok {
		return c.Name()
	}
	return chatID
}

func (m *Model) winFor(chatID string) (int, *win) {
	for i, w := range m.wins {
		if i > 0 && w.chatID == chatID {
			return i, w
		}
	}
	return -1, nil
}

func (m *Model) openWin(chatID string) (int, *win) {
	w := &win{chatID: chatID}
	m.wins = append(m.wins, w)
	m.openedN++
	i := len(m.wins) - 1
	m.note(m.wins[0], fmt.Sprintf("window %d opened for %s", i, m.nameOf(chatID)))
	m.saveLayout()
	return i, w
}

// switchTo makes window i current and fetches its recent history once.
func (m *Model) switchTo(i int) tea.Cmd {
	i = min(max(i, 0), len(m.wins)-1)
	if i != m.cur {
		m.cur = i
		m.saveLayout()
	}
	m.mode = modeWin
	m.lastCtrlX = time.Time{} // a double Ctrl-X never spans two windows
	w := m.wins[i]
	w.unread = 0
	if w.chatID != "" {
		m.visited[w.chatID] = true
	}
	if m.isErr { // a switch clears a stale error; the tag says where we are
		m.setInfo("")
	}
	if w.chatID != "" && !w.loaded && !m.loading[w.chatID] {
		m.note(w, fmt.Sprintf("fetching the last %d messages", recentOnOpen))
		return m.recentCmd(w.chatID, recentOnOpen)
	}
	return nil
}

func (m *Model) winName(i int) string {
	if i == 0 {
		return "server"
	}
	return m.nameOf(m.wins[i].chatID)
}

func (m *Model) closeWin(i int) {
	if i <= 0 || i >= len(m.wins) {
		m.setErr("cannot close that window (0 is the server window)")
		return
	}
	name := m.winName(i)
	m.wins = append(m.wins[:i], m.wins[i+1:]...)
	if m.cur >= i {
		m.cur = max(m.cur-1, 0)
	}
	m.note(m.wins[0], "window closed: "+name)
	m.setInfo("closed " + name)
	m.saveLayout()
}

// moveWin puts the current window at position n; the server window stays
// at 0 and the others shift.
func (m *Model) moveWin(n int) {
	if m.cur == 0 {
		m.setErr("the server window stays at 0")
		return
	}
	if n < 1 || n >= len(m.wins) {
		m.setErr(fmt.Sprintf("window numbers go from 1 to %d", len(m.wins)-1))
		return
	}
	w := m.wins[m.cur]
	rest := append(append([]*win{}, m.wins[:m.cur]...), m.wins[m.cur+1:]...)
	m.wins = append(append(append([]*win{}, rest[:n]...), w), rest[n:]...)
	m.cur = n
	m.saveLayout()
	m.setInfo(fmt.Sprintf("%s is now window %d", m.winName(n), n))
}

func (m *Model) refresh() tea.Cmd {
	if m.syncing {
		m.setInfo("sync already running")
		return nil
	}
	m.syncing = true
	m.setInfo("syncing now")
	return tea.Batch(m.chatsCmd(), m.syncCmd())
}

func (m *Model) key(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if k.Type != tea.KeyTab {
		m.comp = nil
	}
	page := max(m.contentHeight()-1, 1)
	w := m.wins[m.cur]
	// Esc then a digit works like Alt-digit, for terminals without Meta.
	escPending := m.escPending
	m.escPending = false
	digit := k.Type == tea.KeyRunes && len(k.Runes) == 1 && k.Runes[0] >= '0' && k.Runes[0] <= '9'
	switch {
	case k.Type == tea.KeyCtrlC:
		m.quitting = true
		return m, tea.Quit
	case digit && (k.Alt || escPending):
		if escPending {
			m.input = m.escSaved // the Esc was a prefix, not a clear
		}
		n := int(k.Runes[0] - '0') // 0 is the server window
		if n < len(m.wins) {
			return m, m.switchTo(n)
		}
		m.setErr("no window " + strconv.Itoa(n))
		return m, nil
	case k.Alt && k.Type == tea.KeyRight, k.Type == tea.KeyCtrlN:
		return m, m.switchTo((m.cur + 1) % len(m.wins))
	case k.Alt && k.Type == tea.KeyLeft, k.Type == tea.KeyCtrlP:
		return m, m.switchTo((m.cur + len(m.wins) - 1) % len(m.wins))
	}
	switch k.Type {
	case tea.KeyEsc:
		m.escPending, m.escSaved = true, nil
		if m.mode != modeWin { // the input held a filter or search, not a draft
			m.mode, m.input = modeWin, nil
		} else {
			m.escSaved = append([]rune(nil), m.input...)
			m.input = nil
		}
	case tea.KeyEnter:
		return m.enter()
	case tea.KeyCtrlX:
		now := time.Now()
		// A second Ctrl-X within the window toggles "Enter sends"; text held
		// back by the first one is not sent but put back in the input.
		if m.mode == modeWin && w.chatID != "" && now.Sub(m.lastCtrlX) < doubleCtrlX {
			m.lastCtrlX = time.Time{}
			if m.pending != nil {
				m.input = []rune(m.pending.text)
				m.pending = nil
			}
			w.enterSends = !w.enterSends
			m.saveLayout() // the choice is kept per window
			if w.enterSends {
				m.setInfo("Enter now sends text in " + m.winName(m.cur) + " (double Ctrl-X to turn off)")
			} else {
				m.setInfo("Enter runs commands again; Ctrl-X sends")
			}
			return m, nil
		}
		m.lastCtrlX = now
		return m.holdSend()
	case tea.KeyTab:
		if m.mode == modeWin && len(m.input) == 0 { // an empty prompt: Tab walks the windows
			return m, m.switchTo((m.cur + 1) % len(m.wins))
		}
		m.complete()
	case tea.KeyShiftTab:
		if m.mode == modeWin && len(m.input) == 0 {
			return m, m.switchTo((m.cur + len(m.wins) - 1) % len(m.wins))
		}
	case tea.KeyBackspace, tea.KeyCtrlH, tea.KeyDelete:
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
	case tea.KeyCtrlU:
		m.input = nil
	case tea.KeyCtrlW:
		s := strings.TrimRight(string(m.input), " ")
		if i := strings.LastIndex(s, " "); i >= 0 {
			m.input = []rune(s[:i+1])
		} else {
			m.input = nil
		}
	case tea.KeyPgUp:
		w.scroll += page
	case tea.KeyPgDown:
		w.scroll = max(w.scroll-page, 0)
	case tea.KeyHome:
		w.scroll = 1 << 30
	case tea.KeyEnd:
		w.scroll = 0
	case tea.KeyUp:
		switch m.mode {
		case modeList:
			m.listCur = max(m.listCur-1, 0)
		case modePager:
			w.scroll++
		default:
			m.historyStep(-1)
		}
	case tea.KeyDown:
		switch m.mode {
		case modeList:
			m.listCur++
		case modePager:
			w.scroll = max(w.scroll-1, 0)
		default:
			m.historyStep(1)
		}
	case tea.KeyCtrlR:
		return m, m.refresh()
	case tea.KeyF1:
		m.help()
	case tea.KeySpace:
		m.input = append(m.input, ' ')
	case tea.KeyRunes:
		for _, r := range k.Runes {
			if r >= ' ' && r != 0x7f {
				m.input = append(m.input, r)
			}
		}
		m.histIdx = len(m.history) // typing leaves the history walk
		if m.mode == modeList {
			m.listCur = 0
		}
	}
	return m, nil
}

// pendingSend is typed text held back for the double Ctrl-X window.
type pendingSend struct {
	chatID, text string
	seq          int
}

// sendable checks that the typed text can go to the current window's chat
// and returns it; it reports errors in the status line.
func (m *Model) sendable() (chatID, text string, ok bool) {
	raw := string(m.input)
	text = strings.TrimSpace(raw)
	w := m.wins[m.cur]
	switch {
	case m.mode != modeWin, text == "":
		return "", "", false
	case w.chatID == "":
		m.setErr("the server window has no chat; /win to a chat first")
		return "", "", false
	case strings.HasPrefix(text, "/") && !strings.HasPrefix(raw, " "): // a leading space sends it as text
		m.setErr("that looks like a command: Enter runs it; prefix a space to send it as text")
		return "", "", false
	}
	return w.chatID, text, true
}

// send posts the typed text to the current window's chat at once (Enter
// when Enter sends).
func (m *Model) send() (tea.Model, tea.Cmd) {
	chatID, text, ok := m.sendable()
	if !ok {
		return m, nil
	}
	m.input = nil
	m.remember(text)
	m.setInfo("sending to " + m.nameOf(chatID) + "...")
	return m, m.sendCmd(chatID, text)
}

// holdSend takes the typed text out of the input and sends it after the
// double Ctrl-X window, unless a second Ctrl-X cancels it meanwhile.
func (m *Model) holdSend() (tea.Model, tea.Cmd) {
	chatID, text, ok := m.sendable()
	if !ok {
		return m, nil
	}
	m.input = nil
	m.pendingSeq++
	m.pending = &pendingSend{chatID: chatID, text: text, seq: m.pendingSeq}
	m.setInfo("sending to " + m.nameOf(chatID) + "... (Ctrl-X again to keep the text and toggle)")
	seq := m.pendingSeq
	return m, tea.Tick(doubleCtrlX, func(time.Time) tea.Msg { return sendLaterMsg{seq: seq} })
}

// remember appends an entered line to the input history (no immediate
// duplicates) and leaves the walk at the new line.
func (m *Model) remember(line string) {
	if line == "" {
		return
	}
	if n := len(m.history); n == 0 || m.history[n-1] != line {
		m.history = append(m.history, line)
		if len(m.history) > maxHistory {
			m.history = m.history[len(m.history)-maxHistory:]
		}
	}
	m.histIdx, m.histSave = len(m.history), nil
}

// historyStep walks the input history: Up goes back, Down forward, and
// past the newest entry the line that was being typed comes back.
func (m *Model) historyStep(d int) {
	if len(m.history) == 0 {
		return
	}
	if m.histIdx >= len(m.history) {
		m.histIdx = len(m.history)
		if d < 0 {
			m.histSave = append([]rune(nil), m.input...)
		}
	}
	i := m.histIdx + d
	switch {
	case i < 0:
		return
	case i >= len(m.history):
		m.histIdx = len(m.history)
		m.input = append([]rune(nil), m.histSave...)
		return
	}
	m.histIdx = i
	m.input = []rune(m.history[i])
}

func (m *Model) enter() (tea.Model, tea.Cmd) {
	line := strings.TrimSpace(string(m.input))
	if m.mode == modeList && !strings.HasPrefix(line, "/") {
		if m.listSrc == listWindows {
			m.input = nil
			return m, m.switchTo(m.listCur)
		}
		items := m.listItems() // while the typed filter still applies
		m.input = nil
		if len(items) == 0 {
			m.mode = modeWin
			return m, nil
		}
		c := items[min(m.listCur, len(items)-1)]
		i, w := m.winFor(c.ID)
		if w == nil {
			i, _ = m.openWin(c.ID)
		}
		return m, m.switchTo(i)
	}
	if !strings.HasPrefix(line, "/") || (strings.HasPrefix(string(m.input), " ") && m.wins[m.cur].enterSends) {
		if line != "" {
			w := m.wins[m.cur]
			if w.enterSends && w.chatID != "" && m.mode == modeWin {
				return m.send()
			}
			if w.chatID != "" && m.mode == modeWin {
				m.setErr("Ctrl-X sends the text to " + m.winName(m.cur) + "; Enter runs /commands (double Ctrl-X to swap)")
			} else {
				m.input = nil
				m.setErr("not a command (start with /); Ctrl-X sends text in a chat window")
			}
		}
		return m, nil
	}
	m.input = nil
	m.remember(line)
	cmd, err := command.Parse(line)
	if err != nil {
		m.setErr(err.Error())
		return m, nil
	}
	m.cmdsN++
	return m.run(cmd)
}

func (m *Model) run(c command.Command) (tea.Model, tea.Cmd) {
	w := m.wins[m.cur]
	switch c.Kind {
	case command.Help:
		m.help()
	case command.Quit:
		m.quitting = true
		return m, tea.Quit
	case command.Refresh:
		return m, m.refresh()
	case command.List:
		m.showDeleted = false
		var words []string
		for _, f := range strings.Fields(c.Arg) {
			if f == "--show-deleted" || f == "-a" {
				m.showDeleted = true
			} else {
				words = append(words, f)
			}
		}
		m.mode, m.listSrc, m.listCur, m.input = modeList, listChats, 0, []rune(strings.Join(words, " "))
	case command.Win:
		return m.winCommand(c.Arg)
	case command.Close:
		i := m.cur
		if c.Arg != "" {
			n, err := strconv.Atoi(c.Arg)
			if err != nil {
				m.setErr("/close takes a window number")
				break
			}
			i = n
		}
		m.closeWin(i)
	case command.Msg:
		return m.msg(c.Arg)
	case command.About:
		m.about()
	case command.Mouse:
		m.mouseOn = !m.mouseOn
		_ = m.opt.Store.SetState(mouseKey, m.mouseOn)
		if m.mouseOn {
			m.setInfo("mouse on: click to act; hold Shift (or Option on macOS) while dragging to select text")
			return m, tea.EnableMouseAllMotion
		}
		m.hover = ""
		m.setInfo("mouse off: drag selects text; /mouse turns clicks back on")
		return m, tea.DisableMouse
	case command.Fx:
		m.fx = !m.fx
		_ = m.opt.Store.SetState(fxKey, m.fx)
		if m.fx {
			m.setInfo("running lights on")
		} else {
			m.setInfo("running lights off")
		}
	case command.Move:
		n, err := strconv.Atoi(c.Arg)
		if err != nil {
			m.setErr("/move takes the window number to move this window to")
			break
		}
		m.moveWin(n)
	case command.Last:
		if w.chatID == "" {
			m.setErr("the server window has no chat; use /win first")
			break
		}
		n := recentOnOpen
		if c.Arg != "" {
			v, err := strconv.Atoi(c.Arg)
			if err != nil || v <= 0 {
				m.setErr("/last takes a number of messages")
				break
			}
			n = min(v, maxRecent)
		}
		if m.loading[w.chatID] {
			m.setInfo("already fetching")
			break
		}
		m.note(w, fmt.Sprintf("fetching the last %d messages", n))
		return m, m.recentCmd(w.chatID, n)
	case command.Search:
		m.mode, m.input = modePager, []rune(c.Arg)
		w.scroll = 0
	case command.Stats:
		m.stats()
	case command.Whois:
		m.whois(c.Arg)
	case command.Press:
		return m.press(c.Arg)
	case command.Theme:
		if c.Arg == "" {
			m.note(w, "themes (current: "+currentName+"):")
			for _, n := range themeNames() {
				mark := "  "
				if n == currentName {
					mark = "* "
				}
				m.note(w, "  "+mark+n)
			}
			break
		}
		m.setTheme(c.Arg)
	case command.Open:
		if w.chatID == "" {
			m.setErr("the server window has no chat; /win to the chat first")
			break
		}
		msg, ok := domain.LatestWithMedia(m.msgs, w.chatID)
		if c.Arg != "" {
			ok = false
			for _, x := range m.msgs {
				if x.ChatID == w.chatID && x.ID == c.Arg {
					msg, ok = x, x.Media != nil
				}
			}
		}
		if !ok {
			m.setErr("no photo or file to open in " + m.nameOf(w.chatID))
			break
		}
		m.setInfo("fetching...")
		return m, m.fetchCmd(w.chatID, msg.ID)
	case command.Me:
		if w.chatID == "" {
			m.setErr("the server window has no chat; /win to a chat first")
			break
		}
		if strings.TrimSpace(c.Arg) == "" {
			m.setErr("/me needs some text")
			break
		}
		m.setInfo("sending to " + m.nameOf(w.chatID) + "...")
		return m, m.sendCmd(w.chatID, "* "+strings.TrimSpace(c.Arg))
	}
	return m, nil
}

// press implements /press: "N" presses button N of the latest message with
// buttons in this window's chat, "<msg> N" that of a given message.
func (m *Model) press(arg string) (tea.Model, tea.Cmd) {
	w := m.wins[m.cur]
	if w.chatID == "" {
		m.setErr("the server window has no chat; /win to the bot's window first")
		return m, nil
	}
	f := strings.Fields(arg)
	var msg domain.Message
	var n int
	var err error
	switch len(f) {
	case 1:
		var ok bool
		if msg, ok = domain.LatestWithButtons(m.msgs, w.chatID); !ok {
			m.setErr("no message with buttons in " + m.nameOf(w.chatID))
			return m, nil
		}
		n, err = strconv.Atoi(f[0])
	case 2:
		found := false
		for _, x := range m.msgs {
			if x.ChatID == w.chatID && x.ID == f[0] {
				msg, found = x, true
			}
		}
		if !found {
			m.setErr("no message " + f[0] + " in " + m.nameOf(w.chatID))
			return m, nil
		}
		n, err = strconv.Atoi(f[1])
	default:
		m.setErr("usage: /press N, or /press <msg> N")
		return m, nil
	}
	if err != nil || n < 1 || n > len(msg.Buttons) {
		m.setErr(fmt.Sprintf("message %s has %d button(s); pick 1..%d", msg.ID, len(msg.Buttons), len(msg.Buttons)))
		return m, nil
	}
	b := msg.Buttons[n-1]
	switch b.Kind {
	case domain.ButtonCallback:
		m.setInfo("pressing " + b.Label + "...")
		return m, m.pressCmd(w.chatID, msg, b)
	case domain.ButtonURL:
		m.note(w, b.Label+": open "+b.URL) // never opened by lazygram itself
		m.setInfo("link shown")
	case domain.ButtonText:
		m.setInfo("sending " + b.Label + "...")
		return m, m.sendCmd(w.chatID, b.Label)
	default:
		m.setErr("button " + b.Label + " is of a kind lazygram cannot press")
	}
	return m, nil
}

// msg implements /msg: send text to a chat named by @user or #chan,
// opening and focusing its window when it has none.
func (m *Model) msg(arg string) (tea.Model, tea.Cmd) {
	target, text, _ := strings.Cut(strings.TrimSpace(arg), " ")
	text = strings.TrimSpace(text)
	if target == "" || text == "" {
		m.setErr("usage: /msg <@user|#chan> <text>")
		return m, nil
	}
	c, err := domain.ResolveChat(m.chats, target)
	if err != nil {
		m.setErr(err.Error())
		return m, nil
	}
	var cmds []tea.Cmd
	i, w := m.winFor(c.ID)
	if w == nil {
		i, _ = m.openWin(c.ID)
	}
	if cmd := m.switchTo(i); cmd != nil {
		cmds = append(cmds, cmd)
	}
	m.setInfo("sending to " + c.Name() + "...")
	cmds = append(cmds, m.sendCmd(c.ID, text))
	return m, tea.Batch(cmds...)
}

// stats prints client statistics into the current window.
func (m *Model) stats() {
	w := m.wins[m.cur]
	var users, groups, channels, deleted, unread int
	for _, c := range m.chats {
		switch {
		case c.Deleted:
			deleted++
		case c.Kind == domain.ChatUser:
			users++
		case c.Kind == domain.ChatGroup:
			groups++
		default:
			channels++
		}
		unread += c.Unread
	}
	cached, _ := m.opt.Store.Count("")
	act := 0
	for _, x := range m.wins {
		if x.unread > 0 {
			act++
		}
	}
	sync := "never"
	if !m.lastSync.IsZero() {
		sync = m.lastSync.Format("15:04:05")
	}
	m.note(w, fmt.Sprintf("stats: %s, up %s, last sync %s", m.opt.Provider.Name(),
		time.Since(m.started).Truncate(time.Second), sync))
	m.note(w, fmt.Sprintf("  chats: %d (%d people, %d groups, %d channels, %d deleted), %d unread at the service",
		len(m.chats), users, groups, channels, deleted, unread))
	m.note(w, fmt.Sprintf("  windows: %d open, %d with activity; messages: %d in this session, %d cached",
		len(m.wins), act, len(m.msgs), cached))
}

// whois prints what is known about a chat.
func (m *Model) whois(arg string) {
	w := m.wins[m.cur]
	var c domain.Chat
	if arg == "" {
		if w.chatID == "" {
			m.setErr("/whois needs a name in the server window")
			return
		}
		var ok bool
		if c, ok = m.chat(w.chatID); !ok {
			m.setErr("unknown chat " + w.chatID)
			return
		}
	} else {
		var err error
		if c, err = domain.ResolveChat(m.chats, arg); err != nil {
			m.setErr(err.Error())
			return
		}
	}
	kind := map[domain.ChatKind]string{domain.ChatUser: "person", domain.ChatGroup: "group", domain.ChatChannel: "channel"}[c.Kind]
	msgs := domain.Filter(m.msgs, domain.Query{ChatID: c.ID})
	cached, _ := m.opt.Store.Count(c.ID)
	m.note(w, fmt.Sprintf("whois %s: %s", c.Name(), sanitize(c.Title, false)))
	line := fmt.Sprintf("  %s on %s, id %s", kind, c.Service, c.ID)
	if c.Deleted {
		line += ", deleted account"
	}
	if c.Unread > 0 {
		line += fmt.Sprintf(", %d unread at the service", c.Unread)
	}
	m.note(w, line)
	last := "no messages seen"
	if t := domain.Latest(msgs); !t.IsZero() {
		last = "last message " + t.Local().Format("2006-01-02 15:04")
	}
	win := "no window"
	if i, _ := m.winFor(c.ID); i > 0 {
		win = fmt.Sprintf("window %d", i)
	}
	m.note(w, fmt.Sprintf("  %s, %d messages this session, %d cached, %s", win, len(msgs), cached, last))
}

// help prints the command and key reference into the current window.
func (m *Model) help() {
	w := m.wins[m.cur]
	m.note(w, command.HelpText)
	m.note(w, keyHelp)
	w.scroll = 0
}

// winCommand implements /win: no argument (or "list") opens the selectable
// windows list, a number switches, a name opens or switches to that chat's
// window.
func (m *Model) winCommand(arg string) (tea.Model, tea.Cmd) {
	if arg == "" || strings.EqualFold(arg, "list") {
		m.mode, m.listSrc, m.listCur, m.input = modeList, listWindows, m.cur, nil
		return m, nil
	}
	if n, err := strconv.Atoi(arg); err == nil {
		if n < 0 || n >= len(m.wins) {
			m.setErr("no window " + arg)
			return m, nil
		}
		return m, m.switchTo(n)
	}
	c, err := domain.ResolveChat(m.chats, arg)
	if err != nil {
		m.setErr(err.Error())
		return m, nil
	}
	i, w := m.winFor(c.ID)
	if w == nil {
		i, _ = m.openWin(c.ID)
	}
	return m, m.switchTo(i)
}

// completion remembers that the last Tab could not extend the word, so
// the next Tab lists the candidates, as bash does.
type completion struct {
	word string
}

// complete expands the word at the end of the input the way bash does: a
// command name when the line starts with one, a theme after /theme, else a
// chat name (@user or #channel). One candidate is filled in with a space;
// several are extended to their longest common prefix, and a second Tab
// that cannot extend further prints them into the window.
func (m *Model) complete() {
	start := 0
	for i := len(m.input) - 1; i >= 0; i-- {
		if m.input[i] == ' ' {
			start = i + 1
			break
		}
	}
	word := strings.ToLower(string(m.input[start:]))
	var cands []string
	line := strings.ToLower(string(m.input[:start]))
	switch {
	case start == 0 && strings.HasPrefix(word, "/"):
		for _, n := range command.Names() {
			if strings.HasPrefix(n, word) {
				cands = append(cands, n)
			}
		}
	case strings.HasPrefix(line, "/theme "):
		for _, n := range themeNames() {
			if strings.HasPrefix(n, word) {
				cands = append(cands, n)
			}
		}
	default:
		bare := strings.TrimLeft(word, "@#")
		seen := map[string]bool{}
		for _, c := range m.chats {
			if c.Deleted {
				continue
			}
			n := c.Name()
			ok := strings.HasPrefix(strings.ToLower(n), word) ||
				(word == bare && bare != "" && strings.HasPrefix(strings.ToLower(c.Alias), bare))
			if ok && !seen[n] {
				seen[n] = true
				cands = append(cands, n)
			}
		}
		sort.Strings(cands)
	}
	if len(cands) == 0 {
		return
	}
	if len(cands) == 1 {
		m.input = append(append(m.input[:start], []rune(cands[0])...), ' ')
		m.comp = nil
		return
	}
	prefix := commonPrefix(cands)
	if len([]rune(prefix)) > len(m.input)-start {
		m.input = append(m.input[:start], []rune(prefix)...)
		m.comp = &completion{word: prefix}
		return
	}
	if m.comp != nil && m.comp.word == prefix { // second Tab: list them
		m.note(m.wins[m.cur], strings.Join(cands, "  "))
		m.comp = nil
		return
	}
	m.comp = &completion{word: prefix}
}

// commonPrefix is the longest prefix shared by all strings (case kept from
// the first).
func commonPrefix(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	p := []rune(ss[0])
	for _, s := range ss[1:] {
		r := []rune(s)
		n := 0
		for n < len(p) && n < len(r) && strings.EqualFold(string(p[n]), string(r[n])) {
			n++
		}
		p = p[:n]
	}
	return string(p)
}

// listItems is the chat list for /list: deleted accounts hidden unless
// asked for, narrowed by the typed text.
func (m *Model) listItems() []domain.Chat {
	var out []domain.Chat
	q := strings.TrimLeft(strings.ToLower(strings.TrimSpace(string(m.input))), "@#")
	for _, c := range m.chats {
		if c.Deleted && !m.showDeleted {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(c.Alias), q) && !strings.Contains(strings.ToLower(c.Title), q) {
			continue
		}
		out = append(out, c)
	}
	return out
}

func (m *Model) contentHeight() int { return max(m.height-3, 1) }

func (m *Model) nameWidth() int {
	w := 4
	for _, c := range m.chats {
		w = max(w, lipgloss.Width(c.Name()))
	}
	return min(w, 15)
}

// lines renders the current window's scrollback.
func (m *Model) lines() []string {
	lines, _ := m.richLines()
	return lines
}

// phaseOpt is the light's clock for rendering: milliseconds since the
// pointer reached the hovered element, or -1 when lights are off or
// nothing is hovered.
func (m *Model) phaseOpt() int {
	if !m.fx || m.hover == "" {
		return -1
	}
	return int(time.Since(m.hoverAt) / time.Millisecond)
}

func (m *Model) richLines() ([]string, [][]span) {
	w := m.wins[m.cur]
	o := lineOpts{width: m.width, nameOf: m.nameOf, nameW: m.nameWidth(), showChat: w.chatID == "", phase: m.phaseOpt(), hover: m.hover, cache: m.rcache}
	msgs := m.msgs
	if w.chatID != "" {
		msgs = domain.Filter(m.msgs, domain.Query{ChatID: w.chatID})
	}
	return renderRich(msgs, w.notes, o)
}

// Quitting reports whether the user asked to leave.
func (m *Model) Quitting() bool { return m.quitting }

// View renders the whole screen.
func (m *Model) View() string {
	if m.quitting {
		return ""
	}
	h := m.contentHeight()
	w := m.wins[m.cur]
	var prompt string
	var body []string
	bodySpans := make([][]span, h) // one entry per content row
	m.modeInfo = ""

	switch m.mode {
	case modeList:
		var rows []string
		if m.listSrc == listWindows {
			m.listCur = min(max(m.listCur, 0), len(m.wins)-1)
			m.modeInfo = fmt.Sprintf("windows (%d): Up/Down, Enter focuses, Esc returns", len(m.wins))
			rows = m.winLines()
			prompt = "windows > "
		} else {
			items := m.listItems()
			m.listCur = min(max(m.listCur, 0), max(len(items)-1, 0))
			m.modeInfo = fmt.Sprintf("chats (%d): Up/Down, Enter opens, Esc returns, type to narrow", len(items))
			rows = m.listLines(items)
			prompt = "list > "
		}
		body = topWindow(rows, h, m.listCur)
		start := topWindowStart(len(rows), h, m.listCur)
		for y := range body {
			if i := start + y; i < len(rows) && body[y] != "" {
				bodySpans[y] = []span{{0, m.width, spanRow, strconv.Itoa(i)}}
			}
		}
	case modePager:
		q := string(m.input)
		hits := domain.Filter(m.msgs, domain.Query{ChatID: w.chatID, Text: q})
		lines, sp := renderRich(hits, nil, lineOpts{width: m.width, nameOf: m.nameOf, nameW: m.nameWidth(),
			showChat: w.chatID == "", fullDate: true, highlight: q, phase: m.phaseOpt(), hover: m.hover, cache: m.rcache})
		body, w.scroll = window(lines, h, w.scroll)
		bodySpans, _ = window(sp, h, w.scroll)
		m.modeInfo = fmt.Sprintf("history of %s: %d of %d messages; type to search, Esc returns",
			m.winName(m.cur), len(hits), len(domain.Filter(m.msgs, domain.Query{ChatID: w.chatID})))
		prompt = "search > "
	default:
		lines, sp := m.richLines()
		m.lastLines = len(lines)
		body, w.scroll = window(lines, h, w.scroll)
		bodySpans, _ = window(sp, h, w.scroll)
		if w.scroll > 0 && h > 1 { // not at the bottom: a short centred marker
			marker := fmt.Sprintf("vvv %d more vvv", w.scroll)
			pad := max((m.width-lipgloss.Width(marker))/2, 0)
			body[h-1] = actStyle.Render(fitLine(strings.Repeat(" ", pad)+marker, m.width))
			bodySpans[h-1] = []span{{pad, pad + lipgloss.Width(marker), spanEnd, ""}}
		}
		prompt = "[" + m.winName(m.cur) + "] "
		if w.enterSends {
			prompt = "[" + m.winName(m.cur) + "]> "
		}
	}

	var b strings.Builder
	for _, l := range body {
		b.WriteString(lipgloss.NewStyle().MaxWidth(m.width).Render(l))
		b.WriteByte('\n')
	}
	bar, barSpans := m.windowBar()
	b.WriteString(m.statusLine())
	b.WriteByte('\n')
	b.WriteString(bar)
	b.WriteByte('\n')
	b.WriteString(m.inputLine(prompt))
	// Screen rows: h content rows, status, window bar, input.
	m.clicks = append(append([][]span{}, bodySpans...), nil, barSpans, nil)
	m.animOn = m.hover != "" // the light runs only under the pointer
	return b.String()
}

// listLines renders the /list rows: window number, name, title, unread.
func (m *Model) listLines(items []domain.Chat) []string {
	var out []string
	for i, c := range items {
		mark := "   "
		if wi, _ := m.winFor(c.ID); wi > 0 {
			mark = fmt.Sprintf("%2d ", wi)
		}
		row := fmt.Sprintf("%s %-16s %-30s", mark, c.Name(), sanitize(c.Title, false))
		if c.Unread > 0 {
			row += fmt.Sprintf("  %d unread", c.Unread)
		}
		if c.Deleted {
			row += "  (deleted)"
		}
		if i == m.listCur {
			row = hiStyle.Render(row)
		}
		out = append(out, row)
	}
	if len(out) == 0 {
		out = append(out, dimStyle.Render("  no chat matches"))
	}
	return out
}

// winLines renders the /win rows: number, name, title, unread.
func (m *Model) winLines() []string {
	var out []string
	for i, w := range m.wins {
		title := "all chats and notices"
		if c, ok := m.chat(w.chatID); ok {
			title = sanitize(c.Title, false)
		}
		row := fmt.Sprintf(" %2d  %-16s %-30s", i, m.winName(i), title)
		if w.unread > 0 {
			row += fmt.Sprintf("  %d unread", w.unread)
		}
		if i == m.cur {
			row += "  (current)"
		}
		if i == m.listCur {
			row = hiStyle.Render(row)
		}
		out = append(out, row)
	}
	return out
}

// activity lists the windows with unread messages, irssi style.
func (m *Model) activity() string {
	var act []string
	for i, w := range m.wins {
		if w.unread > 0 {
			act = append(act, strconv.Itoa(i))
		}
	}
	if len(act) == 0 {
		return ""
	}
	return "Act: " + strings.Join(act, ",")
}

// seg paints one status segment in a palette colour on the status
// background; i picks the colour, so segments differ but stay stable.
func seg(text string, i int, bold bool) string {
	st := lipgloss.NewStyle().Background(statusBG()).Foreground(palette[i%len(palette)]).Bold(bold)
	return st.Render(text)
}

func (m *Model) statusLine() string {
	// The sync segment keeps its width and colour across polls, so the
	// rest of the line does not jump every ten seconds.
	sync, syncColor := "synced: never   ", 6
	if !m.lastSync.IsZero() {
		sync, syncColor = "synced: "+m.lastSync.Format("15:04:05"), 2
		if m.syncing {
			sync = "syncing " + m.lastSync.Format("15:04:05")
		}
	}
	w := m.wins[m.cur]
	gap := infoStyle.Render("  ")
	text := infoStyle.Render(" ") +
		seg(time.Now().Format("15:04"), 5, false) + gap +
		seg(m.opt.Provider.Name(), 0, false) + gap +
		seg(fmt.Sprintf("[%d:%s]", m.cur, m.winName(m.cur)), 3, true) + gap
	if act := m.activity(); act != "" {
		text += actStyle.Render(act) + gap
	}
	text += seg(fmt.Sprintf("%d msgs", len(m.msgs)), 1, false) + gap +
		seg(sync, syncColor, false) + gap
	if w.scroll > 0 && m.mode == modeWin {
		text += actStyle.Render(fmt.Sprintf("scrolled up %d lines", w.scroll)) + gap
	}
	switch {
	case m.modeInfo != "":
		text += infoStyle.Render(m.modeInfo) + gap
	case m.status != "":
		if m.isErr {
			text += errStyle.Render(m.status)
		} else {
			text += infoStyle.Render(m.status)
		}
		text += gap
	}
	if w.chatID != "" && m.mode == modeWin {
		how := "ctrl-x: to send"
		if w.enterSends {
			how = "enter: to send"
		}
		text += seg(how, 2, false)
	}
	return lipgloss.NewStyle().Background(statusBG()).Render(fitLine(text, m.width))
}

// windowBar lists every window as [N:name]; the current one is
// highlighted and windows with unread messages stand out. Each tag is a
// click target.
func (m *Model) windowBar() (string, []span) {
	var b strings.Builder
	var spans []span
	x := 1
	for i, w := range m.wins {
		tag := fmt.Sprintf("[%d:%s]", i, m.winName(i))
		spans = append(spans, span{x, x + lipgloss.Width(tag), spanWin, strconv.Itoa(i)})
		x += lipgloss.Width(tag) + 1
		st := infoStyle
		switch {
		case i == m.cur: // only the active window stands out
			st = hiStyle
		case w.unread > 0: // the indicator for new messages
			st = actStyle
		}
		if m.fx && m.hover == hoverKey(span{kind: spanWin, target: strconv.Itoa(i)}) {
			b.WriteString(clickable(tag, st, m.phaseOpt()))
		} else {
			b.WriteString(st.Render(tag))
		}
		b.WriteString(infoStyle.Render(" "))
	}
	return lipgloss.NewStyle().Background(statusBG()).Render(fitLine(" "+b.String(), m.width)), spans
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
  Tab                    on an empty prompt: next window (Shift-Tab previous);
                         otherwise complete like bash: a command, a @user /
                         #chan or a theme; Tab again lists the possibilities
  Ctrl-X                 send the typed text to this chat (after a short pause)
  Ctrl-X twice (fast)    toggle "Enter sends" for this window; nothing is sent
  Ctrl-N / Ctrl-P        next / previous window (Alt-Right / Alt-Left too)
  Alt-0..9, Esc 0..9     window 0..9 (0 is the server window)
  Up / Down              walk the input history (commands and sent lines)
  PgUp / PgDn            scroll a page (the wheel scrolls too)
  Home / End             oldest / newest
  Ctrl-R                 sync now
  F1                     this help
  Esc                    close the list or pager; clear the input
  Ctrl-C                 quit
Themes: /theme lists the colour schemes, /theme <name> switches (kept).
Clickable text is bright and underlined; a running light plays over the
one under the mouse pointer. /fx turns the light on or off (kept).
To select text with the mouse, hold Shift (Option on macOS) while
dragging, or /mouse to turn mouse reporting off and on (kept).
Mouse: click a chat name or nick in window 0 to open its window, a tag in
the window bar to focus it, a row in a list to open it, a link or a
[photo] / [file] token to open it with the system; the wheel scrolls.
Window 0 is the server window: every chat interleaved, plus notices;
it stays at 0 and cannot be closed. /move N puts this window at N.
A window opens for a chat when it gets a message or has unread ones;
[Act: 1,2] in the status line lists windows with unread messages.
People are @name, groups and channels are #name.`
