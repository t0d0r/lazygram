// Package tui is the terminal front end, laid out like BitchX/irssi: window
// 1 is the server window with every message and client notices, and a
// window per chat opens when that chat gets a message. A slash-command
// input line with Tab completion drives it.
package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

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
	modeList              // selectable list: chats (/list), windows (/win) or settings (/settings)
	modePager             // searchable local history of the current window's chat
)

type listSrc int

const (
	listChats    listSrc = iota // every chat
	listWindows                 // the open windows
	listSettings                // every kept setting: the menu window 0 shows (settings.go)
)

const (
	fxKey          = "fx"                  // store state key for the running lights
	slowDelay      = 28 * time.Millisecond // between two lines of a slow print
	exitDelay      = 2 * slowDelay         // between two lines of the exit banner
	revealTime     = 2 * time.Second       // the longest a window's start-up reveal takes
	mouseKey       = "mouse"               // store state key for mouse reporting
	namesKey       = "names"               // store state key for showing chats by name
	receiptsKey    = "read_receipts"       // store state key for read receipts; absent means off
	autojoinKey    = "autojoin"            // store state key for opening unread chats at start; absent means on
	feedKey        = "feed"                // store state key for chat messages in the server window; absent means on
	sharedKey      = "shared_chats"        // store state key for the chats with shared message ids
	healEvery      = 2 * time.Minute       // a shown window's history is fetched again after this, in case changes were missed
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
	Rand     func(n int) int    // draws the start-up hints, a number in [0, n); nil means rand.IntN
}

// win is one virtual window. wins[0] is the server window (chatID empty).
type win struct {
	chatID string
	unread int
	scroll int // lines above the bottom
	notes  []note
	loaded bool // recent history fetched once

	healed  time.Time       // when its history was last fetched; changes may have been missed since
	counted map[string]bool // keys of the arrived messages counted in unread, so a deletion takes back only its own

	enterSends bool // Enter sends plain text (toggled by a double Ctrl-X)

	revealing   bool // the content still rolls in line by line (the window shown at start-up)
	revealed    int  // rendered lines shown so far while revealing
	revealTotal int  // rendered lines at the last View, for the step size
}

// clearUnread marks the window read: no unread count, nothing counted.
func (w *win) clearUnread() { w.unread, w.counted = 0, nil }

// Model is the bubbletea model.
type Model struct {
	opt   Options
	chats []domain.Chat
	msgs  []domain.Message // in-memory mirror of the cache, chronological

	wins []*win
	cur  int
	mode mode

	input   []rune
	curBack int // runes to the right of the cursor; 0 puts it at the end
	comp    *completion
	inTop   int  // the first input row shown when the input has more rows than the area
	goal    int  // the column Up and Down keep while they move between input rows
	goalOn  bool // goal holds: the keys so far only moved between input rows

	history  []string // lines entered: commands and sent text, oldest first
	histIdx  int      // position while walking the history; len(history) is the new line
	histSave []rune   // the line being typed when the walk began

	clicks [][]span // what a click on each screen row does, as of the last View

	viewDir   string   // private temporary directory holding plain copies for the viewer
	viewFiles []string // the copies made so far

	hover string // the clickable thing under the pointer, as a hover key

	sel      *selection // the left button is down: a click, drag, word, row or extension until the release
	lastSel  *selection // the last click, drag, word or row; a Shift-click extends it
	kept     *selection // the last selection released, highlighted until the next press or until its rows change
	keptRows []rowLook  // the screen rows the kept selection covers, as the copy read them
	multi    clickRun   // the double or triple click in progress
	screen   []string   // the rows of the last View, styled; the selection reads them
	cursors  []string   // per row of the last View: the cursor drawn on it only in colour, or ""
	clipMode string     // /clip: auto, osc52, tool or off (kept)
	clipSeq  string     // OSC 52 sequence riding on the next frames until clipFlushMsg

	slow     []string // lines still to print, one per slowTick, like a slow serial line
	slowWin  *win     // the window they go to
	slowTick bool     // a slowTickMsg is scheduled

	revealTick bool // a revealTickMsg is scheduled

	mouseOn  bool         // mouse reporting; off lets the terminal select text
	receipts bool         // read receipts to the service when a window with unread messages is focused; off by default
	byName   bool         // show chats as @Title instead of @alias
	fx       bool         // running lights over clickable text
	hoverAt  time.Time    // when the pointer reached the hovered element
	ticking  bool         // an fx tick is scheduled
	animOn   bool         // the last View showed something animated
	rcache   *renderCache // rendered messages, reused across ticks

	now          func() time.Time     // the clock the typing state and double clicks run on; tests replace it
	typing       *domain.TypingState  // who is typing where, from the service's reports
	typingCh     <-chan domain.Typing // the service's feed; nil when it has none or closed it
	changeCh     <-chan domain.Change // edits and deletions from the service; nil without
	typingExpiry bool                 // a typingExpireMsg is scheduled
	typingAnim   bool                 // a typingAnimMsg is scheduled
	typingStep   int                  // the running light's clock

	vi vimState // vim mode: Normal and Insert on the input (vim.go)

	escPending bool         // an Esc was just pressed: a digit now switches windows
	escSaved   []rune       // the input as it was before that Esc cleared it; nil when it cleared nothing
	lastCtrlX  time.Time    // for the double Ctrl-X toggle
	pending    *pendingSend // text waiting out the double Ctrl-X window before it is sent
	pendingSeq int

	listCur     int     // selected row in the list view
	listSrc     listSrc // what the list view shows
	listAct     int     // the row's chosen action button; 0 is the default (open or focus)
	listSaid    bool    // the settings menu: the rule shows the last change's status, not the keys, until the next key
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
	// The member lists of groups, asked once a session when the window is
	// first shown; kept in memory only.
	listed     map[string][]domain.Participant
	listAsked  map[string]bool
	listHidden map[string]bool
	// live holds the keys of the messages that arrived during this
	// session; the server window shows only those, the cache and fetched
	// history stay in the chat windows.
	live map[string]bool
	// Requested read reports in flight, and the next one waiting, per chat.
	reporting map[string]bool
	// startChecked: the first chat list from the service ran the start
	// check (startCheck); autojoin: that check opens windows (/autojoin).
	startChecked bool
	chatsAsked   bool // a chat list request is on its way
	autojoin     bool
	feed         bool            // the server window shows the chats' messages (/feed); off: only its notices
	sharedChats  map[string]bool // every chat ever marked SharedIDs, kept: where a chat-less deletion looks
	gone         map[string]bool // keys deleted at the service this session; a late fetch result does not bring them back
	reportNext   map[string]pendingReport
	status       string
	isErr        bool
	quitting     bool
}

type (
	chatsMsg struct {
		chats []domain.Chat
		err   error
	}
	syncedMsg struct {
		fresh, known []domain.Message // known: cached messages that learned their sender
		err          error
		at           time.Time
	}
	recentMsg struct {
		chatID       string
		fresh, known []domain.Message
		gone         []string // ids deleted at the service
		quiet        bool     // a heal: no notice unless something came
		err          error
	}
	memberListMsg struct {
		chatID string
		list   []domain.Participant
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
	editedMsg    struct {
		chatID, msgID, text string
		err                 error
	}
	deletedMsg struct {
		chatID, msgID, text string
		err                 error
	}
	fetchedMsg struct {
		chatID, msgID string
		name          string // file name for the viewer
		data          []byte // the attachment
		external      bool   // open with the system viewer instead of in the terminal
		err           error
	}
	tickMsg       time.Time
	fxTickMsg     struct{}
	slowTickMsg   struct{} // the next line of a slow print is due
	revealTickMsg struct{} // one more line of a window still rolling in is due
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
	if opt.Rand == nil {
		opt.Rand = rand.IntN
	}
	m := &Model{opt: opt, width: 100, height: 30, loading: map[string]bool{}, visited: map[string]bool{},
		listed: map[string][]domain.Participant{}, listAsked: map[string]bool{}, listHidden: map[string]bool{},
		live: map[string]bool{}, reporting: map[string]bool{}, reportNext: map[string]pendingReport{}, gone: map[string]bool{},
		wins: []*win{{}}, status: "type /help for commands", started: time.Now(), fx: true, rcache: &renderCache{},
		now: time.Now, typing: domain.NewTypingState(domain.DefaultTypingTTL)}
	if found, err := opt.Store.State(fxKey, &m.fx); !found || err != nil {
		m.fx = true
	}
	if found, err := opt.Store.State(mouseKey, &m.mouseOn); !found || err != nil {
		m.mouseOn = true
	}
	if found, err := opt.Store.State(clipKey, &m.clipMode); !found || err != nil || !validClipMode(m.clipMode) {
		m.clipMode = clipAuto
	}
	_, _ = opt.Store.State(namesKey, &m.byName)
	_, _ = opt.Store.State(receiptsKey, &m.receipts) // absent or unreadable: off
	if found, err := opt.Store.State(autojoinKey, &m.autojoin); !found || err != nil {
		m.autojoin = true
	}
	if found, err := opt.Store.State(feedKey, &m.feed); !found || err != nil {
		m.feed = true
	}
	if found, err := opt.Store.State(vimKey, &m.vi.on); !found || err != nil {
		m.vi.on = false // absent or unreadable: off; on starts in Insert
	}
	var shared []string
	_, _ = opt.Store.State(sharedKey, &shared)
	m.sharedChats = map[string]bool{}
	for _, id := range shared {
		m.sharedChats[id] = true
	}
	m.chats, _ = opt.Store.Chats()
	m.msgs, _ = opt.Store.Messages("", opt.Backlog)
	// The banner has the zero time so it stays above everything, however old
	// the cached messages are.
	m.wins[0].notes = append(m.wins[0].notes, note{text: logo.Wide, raw: true})
	m.note(m.wins[0], Tagline(opt.Version))
	m.note(m.wins[0], "/help lists commands, /keys the shortcuts") // /list has its own hint below
	// Printed here, once per start, so they roll in with the rest of the
	// start-up screen; no chat list or /refresh prints them again.
	for _, h := range startHints(opt.Rand) {
		m.note(m.wins[0], h)
	}
	if !m.feed { // after the hints, so their order on the start-up screen stays
		m.note(m.wins[0], feedOffNotice)
	}
	m.restoreLayout()
	m.restoreTheme()
	m.wins[m.cur].revealing = true // only the first screen rolls in; switching windows shows them at once
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
		if hits := themeMatches(name); len(hits) > 1 {
			m.setErr("ambiguous: " + strings.Join(hits, ", "))
			return
		}
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
// one of the logo faces that fit the terminal, at random, then the
// tagline and the session summary.
func ExitBanner(version string, width int, summary string) string {
	out := strings.Join(Banner(logo.Lines(logo.Random(width))), "\n") + "\n" + dimStyle.Render(Tagline(version)+" by t0d0r - thanks for using lazygram")
	for _, l := range strings.Split(summary, "\n") {
		if l != "" {
			out += "\n" + dimStyle.Render("  "+l)
		}
	}
	return out + "\n"
}

// PrintSlowly writes text one line at a time with a pause between them,
// like a terminal on a slow serial line; the exit banner is shown so.
func PrintSlowly(w io.Writer, text string) {
	for i, l := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		if i > 0 {
			time.Sleep(exitDelay)
		}
		_, _ = fmt.Fprintln(w, l)
	}
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
		"  thanks to @zechkata and @pau4o for their feedback",
		"  built with " + runtime.Version() + " for " + runtime.GOOS + "/" + runtime.GOARCH +
			"; gotd/td (MTProto), bubbletea, lipgloss, modernc.org/sqlite",
		fmt.Sprintf("  provider %s, theme %s, up %s", m.opt.Provider.Name(), currentName, time.Since(m.started).Truncate(time.Second)),
		"  /help for the commands, /keys for the shortcuts, lazygram update for the latest release",
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
	m.chatsAsked = true
	cmds := []tea.Cmd{m.chatsCmd(), m.syncCmd()}
	if cmd := m.changesInit(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if cmd := m.typingInit(); cmd != nil {
		cmds = append(cmds, cmd)
	}
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
		known, idErr := st.Refresh(msgs)
		if err == nil {
			err = errors.Join(addErr, idErr)
		}
		return syncedMsg{fresh: fresh, known: known, err: err, at: time.Now()}
	}
}

func (m *Model) recentCmd(chatID string, n int) tea.Cmd { return m.fetchRecent(chatID, n, false) }

// fetchRecent fetches a chat's last n messages; quiet ones (a heal) say
// nothing in the window unless they fail or bring something.
func (m *Model) fetchRecent(chatID string, n int, quiet bool) tea.Cmd {
	p, st := m.opt.Provider, m.opt.Store
	m.loading[chatID] = true
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		// A cached message above the page newer than this may simply have
		// arrived after the page was taken; the margin covers clock skew.
		asked := time.Now().Add(-2 * time.Minute)
		msgs, err := p.Recent(ctx, chatID, n)
		if err != nil {
			return recentMsg{chatID: chatID, quiet: quiet, err: err}
		}
		// The page is the chat's newest history without gaps: copies the
		// cache holds are brought up to date (edits made elsewhere, the
		// sender's username) and what it holds inside the page's range but
		// the page lacks was deleted at the service.
		fresh, err := st.Add(msgs)
		known, refErr := st.Refresh(msgs)
		gone, pruneErr := st.Prune(chatID, msgs, asked)
		return recentMsg{chatID: chatID, quiet: quiet, fresh: fresh, known: known, gone: gone, err: errors.Join(err, refErr, pruneErr)}
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
func (m *Model) fetchCmd(chatID, msgID string, external bool) tea.Cmd {
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
			return fetchedMsg{chatID: chatID, msgID: msgID, name: viewName(md, msgID, ""), data: data, external: external}
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
		return fetchedMsg{chatID: chatID, msgID: msgID, name: viewName(md, msgID, got), data: data, external: external}
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

func (m *Model) editCmd(msg domain.Message, text string) tea.Cmd {
	p, st := m.opt.Provider, m.opt.Store
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := p.Edit(ctx, msg.ChatID, msg.ID, text)
		if err == nil {
			msg.Text = text
			err = st.Replace(msg)
		}
		return editedMsg{chatID: msg.ChatID, msgID: msg.ID, text: text, err: err}
	}
}

func (m *Model) deleteCmd(msg domain.Message) tea.Cmd {
	p, st := m.opt.Provider, m.opt.Store
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := p.Delete(ctx, msg.ChatID, msg.ID)
		if err == nil {
			err = st.Delete(msg.ChatID, msg.ID)
		}
		return deletedMsg{chatID: msg.ChatID, msgID: msg.ID, text: msg.Text, err: err}
	}
}

func (m *Model) openCmd(target string) tea.Cmd {
	open := m.opt.Open
	return func() tea.Msg { return openedMsg{target: target, err: open(target)} }
}

// Update handles messages. After every message the running lights are
// kept going while something animated is on screen.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	model, cmd := m.update(msg)
	if tcmds := m.typingTimers(); len(tcmds) > 0 { // the typing timers follow whatever the message changed
		cmd = tea.Batch(append(tcmds, cmd)...)
	}
	if len(m.slow) > 0 && !m.slowTick { // keep a slow print going
		m.slowTick = true
		tick := tea.Tick(slowDelay, func(time.Time) tea.Msg { return slowTickMsg{} })
		if cmd == nil {
			cmd = tick
		} else {
			cmd = tea.Batch(cmd, tick)
		}
	}
	if m.mode == modeWin && m.wins[m.cur].revealing && !m.revealTick { // keep a start-up reveal going
		m.revealTick = true
		tick := tea.Tick(slowDelay, func(time.Time) tea.Msg { return revealTickMsg{} })
		if cmd == nil {
			cmd = tick
		} else {
			cmd = tea.Batch(cmd, tick)
		}
	}
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
	if cmd, ok := m.typingUpdate(msg); ok {
		return m, cmd
	}
	switch msg := msg.(type) {
	case slowTickMsg:
		m.slowTick = false
		m.slowStep()
	case revealTickMsg:
		m.revealTick = false
		if w := m.wins[m.cur]; w.revealing {
			// A long scrollback rolls in several lines per tick so the whole
			// reveal stays within revealTime.
			w.revealed += max(1, int((time.Duration(w.revealTotal)*slowDelay+revealTime-1)/revealTime))
		}
	case fxTickMsg:
		m.ticking = false // the next View reads the hover clock
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.dropKept() // the screen is laid out anew: its cells no longer hold what was copied
	case chatsMsg:
		m.chatsAsked = false
		if msg.err != nil {
			m.fail("chats: " + errText(msg.err))
			break
		}
		m.chats = msg.chats
		m.learnShared()
		if !m.startChecked { // the first chat list from the service this session; a cached one does not count
			m.startChecked = true
			m.startCheck()
		}
	case syncedMsg:
		m.syncing = false
		m.lastSync = msg.at
		m.identify(msg.known)
		if len(msg.fresh) > 0 {
			m.arrived(msg.fresh)
		}
		switch {
		case msg.err != nil:
			m.fail("sync: " + errText(msg.err))
		case len(msg.fresh) == 0:
			// Nothing to say: a status that pops up every poll only flickers.
		}
	case memberListMsg:
		m.gotMemberList(msg)
	case changeMsg:
		cmd := m.applyChange(msg.c)
		return m, tea.Batch(cmd, waitChange(m.changeCh))
	case changeDoneMsg:
		m.changeCh = nil
	case recentMsg:
		delete(m.loading, msg.chatID)
		_, w := m.winFor(msg.chatID)
		if w == nil {
			w = m.wins[0]
		}
		// What the cache took is shown even when part of the work failed,
		// as for a sync: a stored message is never fresh again.
		empty := w.chatID != "" && len(domain.Filter(m.msgs, domain.Query{ChatID: w.chatID})) == 0
		m.identify(msg.known)
		m.dropMsgs(msg.chatID, msg.gone)
		fresh := msg.fresh
		if msg.quiet && w.loaded {
			// A heal can store a message before the sync does: one newer
			// than everything the window had is an arrival, with its status,
			// unread count and typing settled, not older history.
			newest := domain.Latest(domain.Filter(m.msgs, domain.Query{ChatID: msg.chatID}))
			var older, news []domain.Message
			for _, x := range fresh {
				if x.Time.After(newest) {
					news = append(news, x)
				} else {
					older = append(older, x)
				}
			}
			m.arrived(news)
			fresh = older
		}
		for _, x := range fresh { // what a fetch brings first of this session's messages is live too
			if !x.Time.Before(m.started) {
				m.live[x.Key()] = true
			}
		}
		m.absorb(fresh)
		if msg.err != nil {
			m.fail("history for " + m.nameOf(msg.chatID) + ": " + errText(msg.err))
			break
		}
		w.loaded = true
		if _, cw := m.winFor(msg.chatID); cw != nil {
			cw.healed = m.now()
		}
		if !msg.quiet || len(fresh) > 0 {
			m.note(w, fmt.Sprintf("fetched %d older message(s)", len(fresh)))
		}
		if empty && len(msg.fresh) > 0 { // a window's first content prints like at start-up
			w.revealing, w.revealed = true, 0
		}
	case pressedMsg:
		_, w := m.winFor(msg.chatID)
		if w == nil {
			w = m.wins[0]
		}
		if msg.err != nil {
			m.fail("press " + msg.label + ": " + errText(msg.err))
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
		if msg.err != nil {
			m.fail("send to " + m.nameOf(msg.chatID) + ": " + errText(msg.err))
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
		if cmd := m.healCmd(m.wins[m.cur]); cmd != nil {
			cmds = append(cmds, cmd)
		}
		if !m.startChecked && !m.chatsAsked { // the first chat list failed: ask again, so the start check runs on connect
			m.chatsAsked = true
			cmds = append(cmds, m.chatsCmd())
		}
		return m, tea.Batch(cmds...)
	case fetchedMsg:
		delete(m.loading, msg.chatID+"\x00"+msg.msgID)
		_, w := m.winFor(msg.chatID)
		if w == nil {
			w = m.wins[0]
		}
		if msg.err != nil {
			m.fail("fetch: " + errText(msg.err))
			break
		}
		md := m.media(msg.chatID, msg.msgID)
		m.fetchedN++
		if md.Image() && !msg.external { // the picture stays in memory: rendered as cells
			lines, size, err := renderImage(msg.data, min(m.width-8, 80), maxImageRows)
			if err != nil {
				m.fail("picture: " + err.Error() + "; /open out shows it in the system viewer")
				break
			}
			w.notes = append(w.notes, note{at: time.Now(), text: strings.Join(lines, "\n"), pre: true})
			m.note(w, fmt.Sprintf("picture %s, %dx%d px (msg %s); /open out %s shows it in the system viewer", msg.name, size.X, size.Y, msg.msgID, msg.msgID))
			w.scroll = 0
			m.setInfo("picture shown")
			break
		}
		path, err := m.viewCopy(msg.name, msg.data)
		if err != nil {
			m.fail("view: " + err.Error())
			break
		}
		if md.Image() && imageFile(path) {
			m.note(w, "opening "+filepath.Base(path)+" in the viewer (temporary copy; kept encrypted in the vault)")
			return m, m.openCmd(path)
		}
		m.note(w, "decrypted to "+path+" (until exit; kept encrypted in the vault)")
		m.setInfo("decrypted to " + path)
	case editedMsg:
		if msg.err != nil {
			m.fail("edit: " + errText(msg.err))
			break
		}
		for i := range m.msgs {
			if m.msgs[i].ChatID == msg.chatID && m.msgs[i].ID == msg.msgID {
				m.msgs[i].Text = msg.text
			}
		}
		m.rcache = &renderCache{}
		m.setInfo("edited")
	case deletedMsg:
		if msg.err != nil {
			m.fail("delete: " + errText(msg.err))
			break
		}
		m.msgs = domain.Remove(m.msgs, msg.chatID, msg.msgID)
		if _, w := m.winFor(msg.chatID); w != nil {
			m.note(w, "deleted your message: "+msg.text)
		}
		m.setInfo("deleted")
	case readMsg:
		switch {
		case msg.err != nil:
			m.fail("read receipt for " + m.nameOf(msg.chatID) + ": " + errText(msg.err))
		case msg.markErr != nil:
			m.fail("unread mark for " + m.nameOf(msg.chatID) + ": " + errText(msg.markErr))
		case msg.manual:
			m.setInfo("reported " + m.nameOf(msg.chatID) + " read")
		}
		if msg.manual {
			return m, m.reported(msg.chatID)
		}
	case unreadMsg:
		if msg.err != nil {
			m.fail("unread mark for " + m.nameOf(msg.chatID) + ": " + errText(msg.err))
		} else {
			m.setInfo("reported " + m.nameOf(msg.chatID) + " unread")
		}
		return m, m.reported(msg.chatID)
	case sendLaterMsg:
		if p := m.pending; p != nil && p.seq == msg.seq {
			m.pending = nil
			m.remember(p.text)
			m.setInfo("sending to " + m.nameOf(p.chatID) + asText(p.text) + "...")
			return m, m.sendCmd(p.chatID, p.text)
		}
	case openedMsg:
		if msg.err != nil {
			m.fail("open " + msg.target + ": " + msg.err.Error())
		} else {
			m.setInfo("opened " + msg.target)
		}
	case clipFlushMsg:
		m.clipSeq = "" // the sequence has been flushed with a frame by now
	case clipDoneMsg:
		if msg.err != nil {
			m.setErr(msg.tool + " failed: " + msg.err.Error())
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

// mouse handles wheel scrolling, hovering, left-button drags (a text
// selection, copied on release and kept highlighted), double and triple
// clicks (a word, a row), Shift-clicks (the last selection extended) and
// clicks on the spans of the last View. A click acts on release, so that
// a drag that starts on a link selects instead of opening it; the later
// presses of a double or triple click select and never act again.
func (m *Model) mouse(e tea.MouseMsg) (tea.Model, tea.Cmd) {
	w := m.wins[m.cur]
	if e.Action == tea.MouseActionMotion {
		if m.sel != nil && e.Button == tea.MouseButtonLeft { // dragging
			if u := m.sel.unit; u == byCell || u == byShift { // a word or a row stays as picked
				m.sel.extend(e.X, e.Y)
			}
			return m, nil
		}
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
		m.leftPress(e)
	case e.Action == tea.MouseActionRelease:
		if m.sel == nil {
			return m, nil
		}
		s := *m.sel
		m.sel = nil
		if s.unit != byShift { // every Shift-click extends the same base
			m.lastSel = &s
		}
		if s.shown() {
			if s.unit == byCell { // a drag is no part of a double click
				m.multi = clickRun{}
			}
			if (s.unit == byWord || s.unit == byRow) && m.plainRow(s.y0) != s.row {
				// The screen changed between the press and the release (a
				// picture landed, a message arrived): what is there now was
				// never under the pointer, so it is neither copied nor kept.
				if m.clipMode != clipOff {
					m.setInfo("nothing copied: the screen changed")
				}
				return m, nil
			}
			m.keep(s) // with /clip off too: the highlight marks what was selected
			if m.clipMode == clipOff {
				return m, nil
			}
			return m.copySelection(s)
		}
		if s.y0 < 0 || s.y0 >= len(m.clicks) {
			return m, nil
		}
		for _, sp := range m.clicks[s.y0] {
			if sp.hit(s.x0) {
				m.multi.acted = true
				return m.click(sp)
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
		return m, m.fetchCmd(chatID, msgID, false)
	case spanEnd:
		m.wins[m.cur].scroll = 0
	case spanWin:
		i, _ := strconv.Atoi(s.target)
		return m, m.switchTo(i)
	case spanRow:
		m.listCur, _ = strconv.Atoi(s.target)
		if m.settingsOpen() { // the clicked setting's next value, as Enter
			return m.stepSetting(1)
		}
		m.listAct = 0 // a click on the row itself is the default action
		return m.enter()
	case spanAct:
		row, act, _ := strings.Cut(s.target, ":")
		m.listCur, _ = strconv.Atoi(row)
		m.listAct, _ = strconv.Atoi(act)
		return m.runListAction()
	}
	return m, nil
}

func errText(err error) string {
	if errors.Is(err, provider.ErrNotImplemented) {
		return "live connector not available; run lazygram demo"
	}
	return err.Error()
}

// fail reports a failure in the status line and in the server window,
// where every error is collected whatever window is current.
func (m *Model) fail(text string) {
	m.setErr(text)
	m.note(m.wins[0], "error: "+text)
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
	fresh = m.notGone(fresh)
	if len(fresh) == 0 {
		return
	}
	before := len(m.lines())
	m.msgs = domain.Merge(m.msgs, fresh)
	if w := m.wins[m.cur]; w.scroll > 0 && m.mode == modeWin {
		w.scroll += max(len(m.lines())-before, 0) // stay anchored while scrolled back
	}
}

// identify puts the sender ids and usernames that cached messages just
// learned (Store.Refresh: an edit made elsewhere, the sender's id and
// username) into the same messages in memory.
func (m *Model) identify(known []domain.Message) {
	if len(known) == 0 {
		return
	}
	byKey := make(map[string]domain.Message, len(known))
	for _, k := range known {
		byKey[k.Key()] = k
	}
	for i := range m.msgs {
		if k, ok := byKey[m.msgs[i].Key()]; ok {
			m.msgs[i] = k
		}
	}
	m.rcache = &renderCache{}
}

// dropMsgs removes messages of chatID from memory: deleted at the service.
func (m *Model) dropMsgs(chatID string, ids []string) {
	if len(ids) > 0 {
		m.dropAcross([]string{chatID}, ids)
	}
}

// notGone leaves out the messages deleted at the service this session: a
// fetch taken before the deletion can answer after it.
func (m *Model) notGone(msgs []domain.Message) []domain.Message {
	if len(m.gone) == 0 {
		return msgs
	}
	out := make([]domain.Message, 0, len(msgs))
	for _, x := range msgs {
		if !m.gone[x.Key()] {
			out = append(out, x)
		}
	}
	return out
}

// arrived handles messages that just came in: windows open for their chats
// and activity is counted.
func (m *Model) arrived(fresh []domain.Message) {
	fresh = m.notGone(fresh)
	for _, msg := range fresh {
		m.live[msg.Key()] = true
	}
	m.absorb(fresh)
	m.typingSettled(fresh)
	var order []int // windows that got something, in order of appearance
	count := map[int]int{}
	n := 0 // messages from others; what this account sent comes back through sync and is not news
	for _, msg := range fresh {
		if msg.Sender == "me" {
			continue
		}
		n++
		i, w := m.winFor(msg.ChatID)
		if w == nil {
			if c, ok := m.chat(msg.ChatID); ok && c.Deleted {
				continue
			}
			i, w = m.openWin(msg.ChatID)
		}
		if i != m.cur {
			w.unread++
			if w.counted == nil {
				w.counted = map[string]bool{}
			}
			w.counted[msg.Key()] = true
		}
		if count[i] == 0 {
			order = append(order, i)
		}
		count[i]++
	}
	m.seen += n
	if n == 0 {
		return
	}
	sort.Ints(order)
	// The status says where the messages went: "3 new messages in 2:#dev,
	// 4:@alice (2)"; a long list is cut after four windows.
	where := m.whereList(order, count)
	word := "messages"
	if n == 1 {
		word = "message"
	}
	if len(where) == 0 {
		m.setInfo(fmt.Sprintf("%d new %s", n, word))
		return
	}
	m.setInfo(fmt.Sprintf("%d new %s in %s", n, word, strings.Join(where, ", ")))
}

// whereList names windows for a status or notice, "2:#dev (3)", with the
// count when it is more than one, cut after four as "+N more".
func (m *Model) whereList(order []int, count map[int]int) []string {
	var where []string
	for n, i := range order {
		if n == 4 {
			where = append(where, fmt.Sprintf("+%d more", len(order)-n))
			break
		}
		s := fmt.Sprintf("%d:%s", i, m.winName(i))
		if count[i] > 1 {
			s += fmt.Sprintf(" (%d)", count[i])
		}
		where = append(where, s)
	}
	return where
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
		return m.display(c)
	}
	return chatID
}

// display is how a chat is shown: @alias, or @Title when /names chose
// names (the sigil stays, the title is sanitized).
func (m *Model) display(c domain.Chat) string {
	if !m.byName || strings.TrimSpace(c.Title) == "" {
		return c.Name()
	}
	return c.Name()[:1] + shownTitle(c)
}

// shownTitle is the chat's title as /names shows it: trimmed, without
// control characters.
func shownTitle(c domain.Chat) string { return sanitize(strings.TrimSpace(c.Title), false) }

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
	receipt := m.readCmd(w) // a focus that clears unread messages is what a receipt reports
	w.clearUnread()
	if w.chatID != "" {
		m.visited[w.chatID] = true
	}
	if m.isErr { // a switch clears a stale error; the tag says where we are
		m.setInfo("")
	}
	cmds := m.typingTimers() // the window shown decides whether the light runs
	if receipt != nil {
		cmds = append(cmds, receipt)
	}
	if w.chatID != "" && !w.loaded && !m.loading[w.chatID] {
		n := m.firstFetch(w.chatID)
		m.note(w, fmt.Sprintf("fetching the last %d messages", n))
		cmds = append(cmds, m.recentCmd(w.chatID, n))
	} else if cmd := m.healCmd(w); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if cmd := m.memberListCmd(w.chatID); cmd != nil {
		cmds = append(cmds, cmd)
	}
	return tea.Batch(cmds...)
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
	m.chatsAsked = true
	m.setInfo("syncing now")
	return tea.Batch(m.chatsCmd(), m.syncCmd())
}

func (m *Model) key(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Vim mode (vim.go) looks first: Normal mode (the input's, a list's)
	// reads a run of typed characters one at a time and j and k as Down and
	// Up, and keeps its cursor on a character once the key is done.
	var vop rune
	if m.vi.on {
		if (m.inNormal() || m.listNormal()) && k.Type == tea.KeyRunes && len(k.Runes) > 1 && !k.Paste && !k.Alt {
			return m.vimEach(k)
		}
		k, vop = m.vimBegin(k)
		defer m.vimSettle()
	}
	if k.Type != tea.KeyTab {
		m.comp = nil
	}
	m.listSaid = false // the settings menu's rule shows its keys again; a change says itself anew
	if k.Type != tea.KeyUp && k.Type != tea.KeyDown {
		m.goalOn = false // any other key ends the moves between input rows
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
		if escPending && m.escSaved != nil {
			m.input, m.curBack = m.escSaved, 0 // the Esc was a prefix, not a clear
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
	if m.vi.on {
		if done, cmd := m.vimKey(k, vop); done { // Esc in Insert, the keys of Normal mode
			return m, cmd
		}
	}
	switch k.Type {
	case tea.KeyEsc:
		m.escPending, m.escSaved = true, nil
		if m.mode != modeWin { // the input held a filter or search, not a draft
			m.mode, m.input, m.curBack = modeWin, nil, 0
		} else {
			m.escSaved = append([]rune(nil), m.input...)
			m.input, m.curBack = nil, 0
		}
	case tea.KeyEnter:
		return m.enter()
	case tea.KeyCtrlX:
		if m.mode != modeWin { // the list and the pager: nothing to send, no toggle armed
			return m, nil
		}
		now := time.Now()
		// A second Ctrl-X within the window toggles "Enter sends"; text held
		// back by the first one is not sent but put back in the input.
		if m.mode == modeWin && w.chatID != "" && now.Sub(m.lastCtrlX) < doubleCtrlX {
			m.lastCtrlX = time.Time{}
			if m.pending != nil {
				m.input, m.curBack = []rune(m.pending.text), 0
				m.pending = nil
			}
			if n := len(m.input); n > 0 && m.input[n-1] == '\n' { // the first press added a line
				m.input = m.input[:n-1]
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
		typed := strings.TrimSpace(string(m.input))
		if w.enterSends && w.chatID != "" && typed != "" &&
			(!strings.HasPrefix(typed, "/") || strings.HasPrefix(string(m.input), " ")) {
			// Enter sends: Ctrl-X adds a line instead. A line that starts
			// with "/" is the exception: Enter would run it as a command,
			// so Ctrl-X sends it as text. With a leading space Enter sends
			// it as text too, so that form keeps the line break.
			m.vimBreakPoint() // in vim's Normal mode: after the character under the cursor (vim.go)
			m.insert('\n')
			m.setInfo("line added; Enter sends the message")
			return m, nil
		}
		return m.holdSend()
	case tea.KeyTab:
		// An empty prompt: Tab walks the windows, from the settings menu
		// too, which is window 0's view (the switch closes it); in /list
		// and /win it completes the filter.
		if (m.mode == modeWin || m.settingsOpen()) && len(m.input) == 0 {
			return m, m.switchTo((m.cur + 1) % len(m.wins))
		}
		m.complete()
	case tea.KeyShiftTab:
		if (m.mode == modeWin || m.settingsOpen()) && len(m.input) == 0 {
			return m, m.switchTo((m.cur + len(m.wins) - 1) % len(m.wins))
		}
	case tea.KeyBackspace, tea.KeyCtrlH:
		m.cut(m.pos()-1, m.pos())
	case tea.KeyDelete:
		// Forward delete; at the end of the line it acts as Backspace,
		// because some terminals send this code for the Backspace key.
		if m.curBack == 0 {
			m.cut(m.pos()-1, m.pos())
		} else {
			m.cut(m.pos(), m.pos()+1)
		}
	case tea.KeyLeft:
		if m.settingsOpen() { // the selected setting's previous value
			return m.stepSetting(-1)
		}
		if m.mode == modeList { // in a list the keys pick the row's action button
			m.listAct = max(m.listAct-1, 0)
			break
		}
		m.curBack = min(m.curBack+1, len(m.input))
	case tea.KeyRight:
		if m.settingsOpen() { // its next value, as Enter
			return m.stepSetting(1)
		}
		if m.mode == modeList {
			m.listAct = min(m.listAct+1, len(m.listActions())-1)
			break
		}
		m.curBack = max(m.curBack-1, 0)
	case tea.KeyCtrlA: // the start of the cursor's line, not of the whole message
		m.curBack = len(m.input) - m.lineStart(m.pos())
	case tea.KeyCtrlE:
		m.curBack = len(m.input) - m.lineEnd(m.pos())
	case tea.KeyCtrlU:
		m.input, m.curBack = nil, 0
	case tea.KeyCtrlK:
		m.cut(m.pos(), m.lineEnd(m.pos()))
	case tea.KeyCtrlW: // the word before the cursor, with the spaces after it
		p, i := m.pos(), m.pos()
		for i > 0 && m.input[i-1] == ' ' {
			i--
		}
		for i > 0 && m.input[i-1] != ' ' {
			i--
		}
		m.cut(i, p)
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
		default: // between the rows of the input first, the history from its first row
			if !m.inputStep(-1) {
				m.historyStep(-1)
			}
		}
	case tea.KeyDown:
		switch m.mode {
		case modeList:
			m.listCur++
		case modePager:
			w.scroll = max(w.scroll-1, 0)
		default:
			if !m.inputStep(1) {
				m.historyStep(1)
			}
		}
	case tea.KeyCtrlR:
		return m, m.refresh()
	case tea.KeyCtrlL: // /redraw
		m.redraw(w)
	case tea.KeyF1:
		return m, m.help()
	case tea.KeySpace:
		m.insert(' ')
	case tea.KeyRunes:
		// A paste arrives as one run of runes, line breaks included: in a
		// window they stay line breaks of the message (CR LF is one), in a
		// list or the pager they are dropped like other control characters.
		for i, r := range k.Runes {
			switch {
			case r == '\n' || r == '\r':
				if r == '\n' && i > 0 && k.Runes[i-1] == '\r' {
					continue
				}
				if m.mode == modeWin {
					m.insert('\n')
				}
			case r >= ' ' && r != 0x7f:
				m.insert(r)
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
// and returns it; it reports errors in the status line. The text goes as
// typed (surrounding whitespace trimmed): a line that starts with "/" is
// a message like any other here, never a command. Enter is what runs
// commands.
func (m *Model) sendable() (chatID, text string, ok bool) {
	text = strings.TrimSpace(string(m.input))
	w := m.wins[m.cur]
	switch {
	case m.mode != modeWin, text == "":
		return "", "", false
	case w.chatID == "":
		m.setErr("the server window has no chat; /win to a chat first")
		return "", "", false
	}
	return w.chatID, text, true
}

// asText names the way a line goes out when it could be mistaken for a
// command, so the status line makes it plain.
func asText(text string) string {
	if strings.HasPrefix(text, "/") {
		return " as text, not a command"
	}
	return ""
}

// send posts the typed text to the current window's chat at once (Enter
// when Enter sends).
func (m *Model) send() (tea.Model, tea.Cmd) {
	chatID, text, ok := m.sendable()
	if !ok {
		return m, nil
	}
	m.input, m.curBack = nil, 0
	m.remember(text)
	m.setInfo("sending to " + m.nameOf(chatID) + asText(text) + "...")
	return m, m.sendCmd(chatID, text)
}

// holdSend takes the typed text out of the input and sends it after the
// double Ctrl-X window, unless a second Ctrl-X cancels it meanwhile.
func (m *Model) holdSend() (tea.Model, tea.Cmd) {
	chatID, text, ok := m.sendable()
	if !ok {
		return m, nil
	}
	m.input, m.curBack = nil, 0
	m.pendingSeq++
	m.pending = &pendingSend{chatID: chatID, text: text, seq: m.pendingSeq}
	m.setInfo("sending to " + m.nameOf(chatID) + asText(text) + "... (Ctrl-X again to keep the text and toggle)")
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
		if d > 0 {
			// Not walking: nothing is newer, and the typed text stays (a
			// Down past the last input row must not drop a draft).
			return
		}
		m.histIdx = len(m.history)
		m.histSave = append([]rune(nil), m.input...)
	}
	i := m.histIdx + d
	switch {
	case i < 0:
		return
	case i >= len(m.history):
		m.histIdx = len(m.history)
		m.input, m.curBack = append([]rune(nil), m.histSave...), 0
		return
	}
	m.histIdx = i
	m.input, m.curBack = []rune(m.history[i]), 0
}

func (m *Model) enter() (tea.Model, tea.Cmd) {
	line := strings.TrimSpace(string(m.input))
	if m.mode == modeList && !strings.HasPrefix(line, "/") {
		if m.listSrc == listSettings { // the next value; the menu stays open, the filter too
			return m.stepSetting(1)
		}
		if m.listAct > 0 { // a chosen action button: the list stays open
			return m.runListAction()
		}
		if m.listSrc == listWindows {
			m.input, m.curBack = nil, 0
			return m, m.switchTo(m.listCur)
		}
		items := m.listItems() // while the typed filter still applies
		m.input, m.curBack = nil, 0
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
			if w.chatID != "" && m.mode == modeWin { // Ctrl-X sends: Enter adds a line to the message
				m.input = append(m.input, '\n')
				m.setInfo("line added; Ctrl-X sends the message")
				return m, nil
			}
			{
				m.input, m.curBack = nil, 0
				m.setErr("not a command (start with /); Ctrl-X sends text in a chat window")
			}
		}
		return m, nil
	}
	m.input, m.curBack = nil, 0
	m.remember(line)
	// Vim mode: a command line leaves a list's Insert mode, so the list it
	// opens, or the one it leaves open, is in Normal, as every list opens.
	m.vi.listIns = false
	// A command typed in the settings menu that leaves it open (/clip,
	// /theme nord, a typo) has the rule show what it said, as a change
	// made in the menu does.
	inMenu := m.settingsOpen()
	cmd, err := command.Parse(line)
	if err != nil {
		m.setErr(err.Error())
		m.listSaid = inMenu
		return m, nil
	}
	m.cmdsN++
	model, run := m.run(cmd)
	if inMenu && m.settingsOpen() && cmd.Kind != command.Settings {
		m.listSaid = true
	}
	return model, run
}

func (m *Model) run(c command.Command) (tea.Model, tea.Cmd) {
	w := m.wins[m.cur]
	switch c.Kind {
	case command.Help:
		return m, m.help()
	case command.Keys:
		return m, m.keys()
	case command.Redraw:
		m.redraw(w)
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
		m.mode, m.listSrc, m.listCur, m.listAct, m.input, m.curBack = modeList, listChats, 0, 0, []rune(strings.Join(words, " ")), 0
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
	case command.SendFile:
		return m.sendFile(c.Arg)
	case command.Edit, command.Delete:
		if w.chatID == "" {
			m.setErr("the server window has no chat; /win to a chat first")
			break
		}
		last, ok := domain.LastOwn(m.msgs, w.chatID)
		if !ok {
			m.setErr("no message of yours in " + m.nameOf(w.chatID) + " to " + map[bool]string{true: "edit", false: "delete"}[c.Kind == command.Edit])
			break
		}
		if c.Kind == command.Delete {
			m.setInfo("deleting your last message...")
			return m, m.deleteCmd(last)
		}
		if c.Arg == "" { // put the text in the prompt for changing
			m.input, m.curBack = []rune("/edit "+last.Text), 0
			m.setInfo("change the text and press Enter")
			break
		}
		m.setInfo("editing...")
		return m, m.editCmd(last, c.Arg)
	case command.About:
		m.about()
	case command.Nicks:
		m.setNames(!m.byName)
	case command.Mouse:
		return m, m.setMouse(!m.mouseOn)
	case command.Clip:
		m.clip(c.Arg)
	case command.Receipts:
		m.readReceipts(c.Arg)
	case command.AutoJoin:
		m.setAutojoin(c.Arg)
	case command.Feed:
		m.setFeed(c.Arg)
	case command.Vim:
		m.setVim(c.Arg)
	case command.MarkRead:
		return m.markAsRead()
	case command.Fx:
		m.setFx(!m.fx)
	case command.Settings:
		return m, m.openSettings(c.Arg)
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
		m.mode, m.input, m.curBack = modePager, []rune(c.Arg), 0
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
		// /open <link>: one http(s) link with a host goes to the browser,
		// like a click; a link-looking argument that is not exactly that is
		// refused, so no text with spaces ever reaches the opener.
		if f := strings.Fields(c.Arg); len(f) > 0 && strings.HasPrefix(strings.ToLower(f[0]), "http") {
			if len(f) != 1 || !isWebLink(f[0]) {
				m.setErr("only one http or https link with a host can be opened")
				break
			}
			m.setInfo("opening " + f[0] + "...")
			return m, m.openCmd(f[0])
		}
		if w.chatID == "" {
			m.setErr("the server window has no chat; /win to the chat first")
			break
		}
		external := false
		arg := c.Arg
		if f := strings.Fields(arg); len(f) > 0 && (f[0] == "out" || f[0] == "ext" || f[0] == "viewer") {
			external, arg = true, strings.Join(f[1:], " ")
		}
		msg, ok := domain.LatestWithMedia(m.msgs, w.chatID)
		if arg != "" {
			ok = false
			for _, x := range m.msgs {
				if x.ChatID == w.chatID && x.ID == arg {
					msg, ok = x, x.Media != nil
				}
			}
		}
		if !ok {
			m.setErr("no photo or file to open in " + m.nameOf(w.chatID))
			break
		}
		m.setInfo("fetching...")
		return m, m.fetchCmd(w.chatID, msg.ID, external)
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
	target, text := m.msgTarget(strings.TrimSpace(arg))
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

// msgTarget splits the argument of /msg into the chat's name and the
// text. A title may have spaces, so the name is the longest leading part,
// ending at a space, that is a chat's name as /names shows it, its
// @Title or #Title form or its title alone, ignoring case and in either
// script as domain.ResolveChat compares names ("@Ivan Petrov hi", and
// "@ivan petrov hi" for the title in Cyrillic rather than "@ivan", which
// may be another chat's exact name); else the first word, as "@alice hi
// there" always was. An argument that is all such a name has no text,
// rather than its last word as text.
func (m *Model) msgTarget(arg string) (target, text string) {
	names := map[string]bool{}
	for _, c := range m.chats {
		if t := strings.TrimSpace(c.Title); t != "" {
			names[domain.Translit(m.display(c))] = true
			names[domain.Translit(c.Name()[:1]+t)] = true
			names[domain.Translit(t)] = true
		}
	}
	for i := len(arg); i > 0; i = strings.LastIndexByte(arg[:i], ' ') {
		if names[domain.Translit(arg[:i])] {
			return arg[:i], strings.TrimSpace(arg[i:])
		}
	}
	target, text, _ = strings.Cut(arg, " ")
	return target, strings.TrimSpace(text)
}

// sendFile implements /send <path> [caption]: the file is read here and
// sent to the current window's chat, as a photo when its name says so.
func (m *Model) sendFile(arg string) (tea.Model, tea.Cmd) {
	w := m.wins[m.cur]
	if w.chatID == "" {
		m.setErr("the server window has no chat; /win to a chat first")
		return m, nil
	}
	path, caption, _ := strings.Cut(strings.TrimSpace(arg), " ")
	if path == "" {
		m.setErr("usage: /send <path> [caption]")
		return m, nil
	}
	path = expandHome(path)
	st, err := os.Stat(path)
	switch {
	case err != nil:
		m.setErr("send: " + err.Error())
		return m, nil
	case !st.Mode().IsRegular():
		m.setErr("send: not a regular file")
		return m, nil
	case st.Size() > 64<<20:
		m.setErr("send: files are capped at 64 MiB")
		return m, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		m.setErr("send: " + err.Error())
		return m, nil
	}
	name := filepath.Base(path)
	kind := "file"
	if domain.IsImageName(name) {
		kind = "photo"
	}
	m.setInfo(fmt.Sprintf("sending %s %s (%d KiB) to %s...", kind, name, (len(data)+1023)/1024, m.nameOf(w.chatID)))
	p := m.opt.Provider
	chatID := w.chatID
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		return sentMsg{chatID: chatID, err: p.SendFile(ctx, chatID, name, data, strings.TrimSpace(caption))}
	}
}

// expandHome replaces a leading ~/ with the home directory.
func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
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
		if m.whoisMember(w, arg) {
			return
		}
		var err error
		if c, err = domain.ResolveChat(m.chats, arg); err != nil {
			m.setErr(err.Error())
			return
		}
	}
	kind := map[domain.ChatKind]string{domain.ChatUser: "person", domain.ChatGroup: "group", domain.ChatChannel: "channel"}[c.Kind]
	msgs := domain.Filter(m.msgs, domain.Query{ChatID: c.ID})
	cached, _ := m.opt.Store.Count(c.ID)
	m.note(w, fmt.Sprintf("whois %s: %s (/names switches between the two)", c.Name(), sanitize(c.Title, false)))
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
	if arg == "" && c.Kind != domain.ChatUser {
		ms, nicks := m.members(c.ID), 0
		for _, mb := range ms {
			if mb.Nick != "" {
				nicks++
			}
		}
		from := "the known messages"
		switch {
		case len(m.listed[c.ID]) > 0:
			from = "the member list and the known messages"
		case m.listHidden[c.ID]:
			from = "the known messages; the member list is hidden"
		}
		m.note(w, fmt.Sprintf("  %d members known from %s, %d with an @username (in its window Tab offers those in text)",
			len(ms), from, nicks))
	}
}

// redraw implements /redraw: the window keeps its messages and loses
// everything else, notices, help text, pictures, the banner, and anything
// still queued for it, as a clean screen in an old terminal would.
func (m *Model) redraw(w *win) {
	w.notes = nil
	w.scroll = 0
	w.revealing = false
	if m.slowWin == w {
		m.slow, m.slowWin = nil, nil
	}
	m.setInfo("")
}

// help prints the notes on the mouse, themes and windows and then the
// command reference into the server window, line by line with a small
// delay, like a terminal on a slow line, and focuses that window so the
// text is in view wherever /help was typed. The commands come last, so
// they are what the window shows when printing ends. The keys are a
// separate list (/keys), and the last help line says so.
func (m *Model) help() tea.Cmd {
	cmd := m.switchTo(0)
	m.slowPrint(m.wins[0], append(strings.Split(helpNotes, "\n"), strings.Split(command.HelpText, "\n")...))
	return cmd
}

// keys prints the keyboard shortcut reference the same way /help prints
// the commands: into the server window, slowly, as plain lines.
func (m *Model) keys() tea.Cmd {
	cmd := m.switchTo(0)
	m.slowPrint(m.wins[0], strings.Split(keyHelp, "\n"))
	return cmd
}

// slowPrint queues plain lines for a window; slowTick delivers them.
func (m *Model) slowPrint(w *win, lines []string) {
	if m.slowWin != nil && m.slowWin != w {
		m.flushSlow()
	}
	m.slowWin = w
	m.slow = append(m.slow, lines...)
}

// flushSlow prints whatever is still queued at once.
func (m *Model) flushSlow() {
	for _, l := range m.slow {
		m.slowWin.notes = append(m.slowWin.notes, note{at: time.Now(), text: l, plain: true})
	}
	m.slow, m.slowWin = nil, nil
}

// slowStep prints the next queued line.
func (m *Model) slowStep() {
	if len(m.slow) == 0 || m.slowWin == nil {
		m.slow, m.slowWin = nil, nil
		return
	}
	m.slowWin.notes = append(m.slowWin.notes, note{at: time.Now(), text: m.slow[0], plain: true})
	if len(m.slowWin.notes) > maxNotes {
		m.slowWin.notes = m.slowWin.notes[len(m.slowWin.notes)-maxNotes:]
	}
	m.slowWin.scroll = 0
	m.slow = m.slow[1:]
	if len(m.slow) == 0 {
		m.slowWin = nil
	}
}

// winCommand implements /win: no argument (or "list") opens the selectable
// windows list, a number switches, a name opens or switches to that chat's
// window.
func (m *Model) winCommand(arg string) (tea.Model, tea.Cmd) {
	if arg == "" || strings.EqualFold(arg, "list") {
		m.mode, m.listSrc, m.listCur, m.listAct, m.input, m.curBack = modeList, listWindows, m.cur, 0, nil, 0
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

// pos is the cursor's rune index in the input.
func (m *Model) pos() int { return len(m.input) - min(max(m.curBack, 0), len(m.input)) }

// insert puts runes at the cursor and leaves the cursor after them.
func (m *Model) insert(rs ...rune) {
	p := m.pos()
	m.input = append(m.input[:p], append(append([]rune(nil), rs...), m.input[p:]...)...)
}

// cut removes the runes [from, to) of the input, keeping the cursor on the
// same text.
func (m *Model) cut(from, to int) {
	from, to = max(from, 0), min(to, len(m.input))
	if from >= to {
		return
	}
	p := m.pos()
	m.input = append(m.input[:from], m.input[to:]...)
	switch {
	case p >= to:
		m.curBack = len(m.input) - (p - (to - from))
	case p >= from:
		m.curBack = len(m.input) - from
	}
}

// completion remembers that the last Tab could not extend the word, so
// the next Tab lists the candidates, as bash does.
type completion struct {
	word string
}

// argWords are the fixed words a command takes as its first argument,
// whatever its spelling (/win, /w, /j list; /clip, /clipboard tool).
var argWords = map[command.Kind][]string{
	command.Win:      {"list"},
	command.List:     {"--show-deleted"},
	command.Receipts: {"on", "off"},
	command.AutoJoin: {"on", "off"},
	command.Feed:     {"on", "off"},
	command.Vim:      {"on", "off"},
	command.Clip:     {"auto", "osc52", "tool", "off"},
}

// chatArgs says where a command's words complete chat names: its first
// argument (a chat to open, describe, write to or filter by), or all of
// its text (/me, /edit: the words of a message, like a line without a
// command). After any other command Tab offers chat names nowhere: its
// arguments are numbers, settings or free text.
var chatArgs = map[command.Kind]string{
	command.Win: "first", command.Msg: "first", command.Whois: "first", command.List: "first",
	command.Me: "text", command.Edit: "text",
}

// openCmds are the spellings of /open; after them Tab completes "out", the
// chat's recent links and the ids of this window's messages with
// attachments.
var openCmds = map[string]bool{"/open": true, "/fetch": true, "/download": true}

// complete expands the word at the end of the input the way bash does: a
// command name when the line starts with one, a theme after /theme, "out",
// the chat's recent links and attachment message ids after /open, a
// command's fixed words (/win
// list), a local path after /send, else a chat name (@user or #channel). One candidate is filled in with a
// space; several are extended to their longest common prefix, and a second
// Tab that cannot extend further prints them into the window.
func (m *Model) complete() {
	// Only the text before the cursor takes part; what follows it is put
	// back unchanged, with the cursor still in front of it.
	tail := append([]rune(nil), m.input[m.pos():]...)
	m.input = m.input[:m.pos()]
	defer func() { m.input, m.curBack = append(m.input, tail...), len(tail) }()
	start := 0
	for i := len(m.input) - 1; i >= 0; i-- {
		if m.input[i] == ' ' {
			start = i + 1
			break
		}
	}
	var cands []string
	shown := func(s string) string { return s } // how a listed candidate prints
	sep := "  "                                 // between listed candidates
	line := strings.ToLower(string(m.input[:start]))
	fields := strings.Fields(line) // the words before the current one; may be empty
	// In a group or channel window's own input (not a list filter or the
	// pager's query) the argument of /whois (all of it: a display name may
	// have spaces) and message text, /me's too, complete the chat's members
	// instead of chat names; a # in front of the /whois argument still asks
	// for a chat. The whole line, not just the part before the cursor,
	// decides whether it is message text.
	group, inGroup := m.groupChat()
	inGroup = inGroup && m.mode == modeWin
	whoisAt, whoisArg := commandArg(m.input, command.Whois)
	whoisArg = whoisArg && inGroup && !strings.HasPrefix(string(m.input[whoisAt:]), "#")
	meAt, meText := commandArg(m.input, command.Me)
	text := inGroup && (meText || !strings.HasPrefix(string(m.input)+string(tail), "/"))
	var members []domain.Member
	if whoisArg || text {
		members = m.members(group.ID) // once per Tab: it reads every message
	}
	switch {
	case whoisArg:
		start = whoisAt
	case text && meText:
		start = memberStart(members, m.input, meAt, start)
	case text:
		start = memberStart(members, m.input, 0, start)
	}
	word := strings.ToLower(string(m.input[start:]))
	switch {
	case whoisArg || text:
		cands, shown = memberCandidates(members, string(m.input[start:]), whoisArg)
		sep = ", " // display names have spaces
	case m.settingsOpen() && !strings.HasPrefix(string(m.input), "/"):
		// The settings menu's filter: the names of the settings. A line
		// starting with / is a command and completes as one.
		cands = completeSetting(word)
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
	case len(fields) > 0 && openCmds[fields[0]]:
		// /open [out] <msg> or /open <link>: "out" as the first word, the
		// chat's recent links (newest message first) and the ids of the
		// messages with attachments in this window's chat; after "out" only
		// the ids; nothing in the server window, which has no chat (an
		// /open <link> typed there still works).
		w := m.wins[m.cur]
		if w.chatID == "" {
			return
		}
		prior := fields[1:]
		if len(prior) == 0 && strings.HasPrefix("out", word) {
			cands = append(cands, "out")
		}
		if len(prior) == 0 { // the chat's recent links, newest first
			// Matched case-insensitively (word is lower-cased) and filled in
			// with their own case; when some match the typed case exactly,
			// only those count, so two links differing in case stay apart.
			typed := string(m.input[start:])
			var any, exact []string
			for _, u := range m.recentLinks(w.chatID, recentLinkCount) {
				if strings.HasPrefix(strings.ToLower(u), word) {
					any = append(any, u)
					if strings.HasPrefix(u, typed) {
						exact = append(exact, u)
					}
				}
			}
			if len(exact) > 0 {
				any = exact
			}
			cands = append(cands, any...)
		}
		if len(prior) == 0 || len(prior) == 1 && (prior[0] == "out" || prior[0] == "ext" || prior[0] == "viewer") {
			for _, x := range m.msgs {
				if x.ChatID == w.chatID && x.Media != nil && strings.HasPrefix(x.ID, word) {
					cands = append(cands, x.ID)
				}
			}
		}
	case strings.HasPrefix(line, "/send ") || strings.HasPrefix(line, "/file ") || strings.HasPrefix(line, "/upload "):
		if strings.Count(line, " ") > 1 { // the caption is free text
			return
		}
		cands = pathCandidates(string(m.input[start:]))
		shown = func(s string) string { // bash lists the names, not the paths
			if strings.HasSuffix(s, "/") {
				return filepath.Base(s) + "/"
			}
			return filepath.Base(s)
		}
		if len(cands) == 1 && strings.HasSuffix(cands[0], "/") { // keep going into the directory
			m.input = append(m.input[:start], []rune(cands[0])...)
			m.comp = nil
			return
		}
	default:
		names, from := true, 0 // a line without a command is message text, all of it
		if len(fields) > 0 && strings.HasPrefix(fields[0], "/") {
			c, err := command.Parse(fields[0])
			first := len(fields) == 1
			if err == nil && first {
				for _, n := range argWords[c.Kind] {
					if strings.HasPrefix(n, word) {
						cands = append(cands, n)
					}
				}
			}
			where, at := chatArgs[c.Kind], argStart(m.input)
			// A title may have spaces: the first argument is one chat name
			// when all of it begins one ("/msg @Ivan P").
			if where == "first" && !first && m.beginsChat(string(m.input[at:])) {
				start, first = at, true
			}
			names = err == nil && (where == "text" || where == "first" && first)
			from = at
		}
		listed := map[string]string{} // how the second Tab lists each name
		if names {
			// In text the name being typed may span words, as a title does.
			start = nameStart(m.input, from, start, m.beginsChat)
			for _, cc := range m.chatCompletions(string(m.input[start:])) {
				if _, dup := listed[cc.fill]; !dup {
					listed[cc.fill] = cc.shown
					cands = append(cands, cc.fill)
					if cc.titled {
						sep = ", " // titles have spaces
					}
				}
			}
		}
		shown = func(s string) string {
			if l, ok := listed[s]; ok {
				return l
			}
			return s
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
	if extends(prefix, string(m.input[start:])) {
		m.input = append(m.input[:start], []rune(prefix)...)
		m.comp = &completion{word: prefix}
		return
	}
	if m.comp != nil && m.comp.word == prefix { // second Tab: list them
		names := make([]string, len(cands))
		for i, c := range cands {
			names[i] = shown(c)
		}
		m.note(m.wins[m.cur], strings.Join(names, sep))
		m.comp = nil
		return
	}
	m.comp = &completion{word: prefix}
}

// argStart is where the first argument of the command line in input
// begins: past the command and the spaces after it.
func argStart(input []rune) int {
	i := 0
	for i < len(input) && input[i] == ' ' {
		i++
	}
	for i < len(input) && input[i] != ' ' {
		i++
	}
	for i < len(input) && input[i] == ' ' {
		i++
	}
	return i
}

// chatCompletion is a chat Tab offers: fill is what is filled in, shown
// how the second Tab lists it, and titled whether shown has a title.
type chatCompletion struct {
	chatID string
	fill   string
	shown  string
	titled bool
}

// chatBegins reports whether typed begins the chat's alias and whether it
// begins its title as shown, in either script ("kam" begins Kamen written
// in Cyrillic). A
// typed @ or # must be the chat's own sigil; without one the alias or the
// title alone is matched.
func chatBegins(c domain.Chat, typed string) (alias, title bool) {
	if typed != "" && (typed[0] == '@' || typed[0] == '#') {
		if typed[0] != c.Name()[0] {
			return false, false
		}
		typed = typed[1:]
	}
	t := shownTitle(c)
	return domain.PrefixMatch(c.Alias, typed), t != "" && domain.PrefixMatch(t, typed)
}

// beginsChat reports whether typed begins the alias or title of a chat
// that is not deleted.
func (m *Model) beginsChat(typed string) bool {
	for _, c := range m.chats {
		if a, t := chatBegins(c, typed); !c.Deleted && (a || t) {
			return true
		}
	}
	return false
}

// chatCompletions are the chats Tab offers for typed: those not deleted
// whose alias or title begins with it (chatBegins), in both /names modes.
// Each is filled in as /names shows it, unless that name would not
// resolve to the chat (two chats show it, or the shown title lost control
// characters): then as its @alias or #alias, so whatever Tab fills in
// finds the chat it was offered for. One filled in by its alias that
// matched by its title alone is listed with the title after it,
// "@stuckoff (Kamen)", as members are.
func (m *Model) chatCompletions(typed string) []chatCompletion {
	var out []chatCompletion
	for _, c := range m.chats {
		byAlias, byTitle := chatBegins(c, typed)
		if c.Deleted || !byAlias && !byTitle {
			continue
		}
		cc := chatCompletion{chatID: c.ID, fill: m.display(c)}
		if cc.fill != c.Name() {
			if r, err := domain.ResolveChat(m.chats, cc.fill); err != nil || r.ID != c.ID {
				cc.fill = c.Name()
			}
		}
		cc.shown, cc.titled = cc.fill, cc.fill != c.Name()
		if !cc.titled && byTitle && !byAlias {
			cc.shown, cc.titled = cc.fill+" ("+shownTitle(c)+")", true
		}
		out = append(out, cc)
	}
	return out
}

// extends reports whether the candidates' common prefix says more than
// the typed word and still begins with it, sigils aside and in either
// script: "Живк" extends "zhiv" and "#ne" extends "ne", but "@van" does not
// extend "ив" when the members matched by their names, not their usernames.
// Lengths compare in transliteration, so a Latin digraph (zh, sh) does not
// make the typed word look longer than the Cyrillic prefix.
func extends(prefix, typed string) bool {
	bare := func(s string) string { return strings.TrimLeft(s, "@#") }
	return len(domain.Translit(prefix)) > len(domain.Translit(typed)) &&
		domain.PrefixMatch(bare(prefix), bare(typed))
}

// pathCandidates lists the local files and directories that start with
// the typed path (~ expands; directories end with /), the way a shell does.
// It splits what was typed, not the expanded path: expanding "~/" drops
// the trailing slash, so the home directory's own name would pass for
// the typed file name.
func pathCandidates(typed string) []string {
	if typed == "~" {
		return []string{"~/"}
	}
	shown, base := filepath.Split(typed) // keep ~/ as typed
	dir := expandHome(shown)
	if dir == "" {
		dir = "."
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		n := e.Name()
		if !strings.HasPrefix(n, base) || (base == "" && strings.HasPrefix(n, ".")) {
			continue
		}
		if e.IsDir() {
			n += "/"
		}
		out = append(out, shown+n)
	}
	sort.Strings(out)
	return out
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
	mt := domain.NewMatcher(strings.TrimLeft(strings.TrimSpace(string(m.input)), "@#"))
	for _, c := range m.chats {
		if c.Deleted && !m.showDeleted {
			continue
		}
		if !mt.Empty() && !mt.Match(c.Alias) && !mt.Match(c.Title) { // either script
			continue
		}
		out = append(out, c)
	}
	return out
}

// contentHeight leaves room for the rule, status, window bar and the
// input area, which takes one row per row of input up to its cap.
func (m *Model) contentHeight() int { return max(m.height-3-m.inputHeight(), 1) }

func (m *Model) nameWidth() int {
	w := 4
	for _, c := range m.chats {
		w = max(w, lipgloss.Width(m.display(c)))
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

func (m *Model) richLines() ([]string, [][]span) { return m.richLinesOf(m.wins[m.cur]) }

// richLinesOf renders a window's scrollback: a chat window its chat's
// messages, the server window this session's messages of every chat
// (none with /feed off) and, in both, the window's notices.
func (m *Model) richLinesOf(w *win) ([]string, [][]span) {
	o := lineOpts{width: m.width, nameOf: m.nameOf, nameW: m.nameWidth(), showChat: w.chatID == "", phase: m.phaseOpt(), hover: m.hover, cache: m.rcache}
	var msgs []domain.Message
	switch {
	case w.chatID != "":
		msgs = domain.Filter(m.msgs, domain.Query{ChatID: w.chatID})
	case m.feed:
		msgs = m.liveMsgs()
	}
	return renderRich(msgs, w.notes, o)
}

// liveMsgs are the messages that arrived during this session, every chat,
// in order: what the server window shows.
func (m *Model) liveMsgs() []domain.Message {
	out := make([]domain.Message, 0, len(m.live))
	for _, x := range m.msgs {
		if m.live[x.Key()] {
			out = append(out, x)
		}
	}
	return out
}

// revealAll shows every window's content at once: snapshots and tests
// have no slow line to imitate.
func (m *Model) revealAll() {
	for _, w := range m.wins {
		w.revealing = false
	}
}

// Quitting reports whether the user asked to leave.
func (m *Model) Quitting() bool { return m.quitting }

// View renders the whole screen.
func (m *Model) View() string {
	if m.quitting {
		return ""
	}
	// The input area is laid out first: its height is what the content
	// gives up, so the rows below it stay where the clicks expect them.
	prompt := m.prompt()
	input := m.inputArea(prompt)
	h := max(m.height-3-len(input), 1)
	w := m.wins[m.cur]
	var body []string
	bodySpans := make([][]span, h) // one entry per content row
	listY, listLook := -1, ""      // the content row the list cursor is on, and its chosen button
	m.modeInfo = ""

	switch m.mode {
	case modeList:
		var rows []string
		selectable, buttons := true, true
		switch m.listSrc {
		case listWindows:
			m.listCur = min(max(m.listCur, 0), len(m.wins)-1)
			m.modeInfo = fmt.Sprintf("windows (%d): Up/Down, Left/Right pick focus/read/unread/close, Enter does it, Esc returns", len(m.wins))
			rows = m.winLines()
		case listSettings: // values change in place: no button strip
			items := m.settingRows()
			selectable, buttons = len(items) > 0, false
			m.listCur = min(max(m.listCur, 0), max(len(items)-1, 0))
			m.modeInfo = fmt.Sprintf("settings (%d): Up/Down, Enter or Right next value, Left previous, Esc closes, %s narrow", len(items), m.vimTypeTo())
			if m.listSaid && m.status != "" { // what the last change said, until the next key
				m.modeInfo = ""
			}
			rows = m.settingLines(items)
		default:
			items := m.listItems()
			selectable = len(items) > 0
			m.listCur = min(max(m.listCur, 0), max(len(items)-1, 0))
			m.modeInfo = fmt.Sprintf("chats (%d): Up/Down, Left/Right pick open/read/unread, Enter does it, Esc returns, %s narrow", len(items), m.vimTypeTo())
			rows = m.listLines(items)
		}
		// The selected row is highlighted and carries the action buttons;
		// each button is a click target of its own, and the row's own
		// target stops where the buttons start, so nothing overlaps.
		var actSpans []span
		actX0 := m.width
		if selectable && m.listCur < len(rows) {
			plain := rows[m.listCur]
			rows[m.listCur] = hiStyle.Render(plain)
			listLook = "list"
			if buttons {
				actX0 = lipgloss.Width(plain) + 2
				strip, spans := m.listButtons(actX0)
				rows[m.listCur] += "  " + strip
				actSpans = spans
				listLook = "list, button " + strconv.Itoa(m.listAct)
			}
		}
		body = topWindow(rows, h, m.listCur)
		start := topWindowStart(len(rows), h, m.listCur)
		for y := range body {
			if i := start + y; i < len(rows) && body[y] != "" {
				bodySpans[y] = []span{{0, m.width, spanRow, strconv.Itoa(i)}}
				if i == m.listCur {
					bodySpans[y] = append([]span{{0, actX0, spanRow, strconv.Itoa(i)}}, actSpans...)
					if listLook != "" {
						listY = y
					}
				}
			}
		}
	case modePager:
		q := string(m.input)
		hits := domain.Filter(m.msgs, domain.Query{ChatID: w.chatID, Text: q})
		lines, sp := renderRich(hits, nil, lineOpts{width: m.width, nameOf: m.nameOf, nameW: m.nameWidth(),
			showChat: w.chatID == "", fullDate: true, highlight: q, phase: m.phaseOpt(), hover: m.hover, cache: m.rcache})
		body, w.scroll = window(lines, h, w.scroll)
		bodySpans, _ = window(sp, h, w.scroll)
		m.modeInfo = fmt.Sprintf("history of %s: %d of %d messages; %s search, Esc returns",
			m.winName(m.cur), len(hits), len(domain.Filter(m.msgs, domain.Query{ChatID: w.chatID})), m.vimTypeTo())
	default:
		lines, sp := m.richLines()
		m.lastLines = len(lines)
		if w.revealing { // start-up: each line arrives at the bottom and pushes the rest up
			w.revealTotal = len(lines)
			if w.revealed >= len(lines) {
				w.revealing = false
			} else {
				lines, sp = lines[:w.revealed], sp[:w.revealed]
				w.scroll = 0
			}
		}
		body, w.scroll = window(lines, h, w.scroll)
		bodySpans, _ = window(sp, h, w.scroll)
		if w.scroll > 0 && h > 1 { // not at the bottom: a short centred marker
			marker := fmt.Sprintf("vvv %d more vvv", w.scroll)
			pad := max((m.width-lipgloss.Width(marker))/2, 0)
			body[h-1] = actStyle.Render(fitLine(strings.Repeat(" ", pad)+marker, m.width))
			bodySpans[h-1] = []span{{pad, pad + lipgloss.Width(marker), spanEnd, ""}}
		}
	}

	rows := make([]string, 0, len(body)+3+len(input))
	for _, l := range body {
		rows = append(rows, lipgloss.NewStyle().MaxWidth(m.width).Render(l))
	}
	bar, barSpans := m.windowBar()
	// A rule separates the content from the status rows; it carries the
	// send hint at its right end, like a label.
	hint := ""
	if w.chatID != "" && m.mode == modeWin {
		hint = "ctrl-x: to send"
		if w.enterSends {
			hint = "enter: to send"
		}
	}
	rows = append(append(rows, m.rule(hint), m.statusLine(), bar), input...)
	// Screen rows: h content rows, rule, status, window bar, the input rows.
	m.clicks = append(append([][]span{}, bodySpans...), nil, nil, barSpans)
	m.clicks = append(m.clicks, make([][]span, len(input))...)
	m.screen = rows
	// The cursors drawn only in colour: the list's row and its chosen
	// button, the active window's tag, the input cursor's column. A kept
	// selection over one would hide it, so keptSel compares these too.
	m.cursors = make([]string, len(rows))
	if listY >= 0 {
		m.cursors[listY] = listLook
	}
	m.cursors[h+2] = "window " + strconv.Itoa(m.cur)
	if in, cur, top, _ := m.inputLayout(prompt); cur >= top && h+3+cur-top < len(rows) {
		m.cursors[h+3+cur-top] = "input " + strconv.Itoa(in[cur].col(m.pos()))
	}
	// The selection being made shows while the button is down; once it is
	// released, the kept one, for as long as its rows read the same.
	mark := m.sel
	if mark == nil || !mark.shown() {
		mark = m.keptSel()
	}
	var b strings.Builder
	for y, l := range rows {
		if mark != nil && mark.shown() {
			if from, to, ok := mark.cols(y, m.width); ok {
				l = markCells(l, from, to)
			}
		}
		if y == h && m.clipSeq != "" {
			// The OSC 52 sequence rides on the rule, which the renderer
			// repaints anyway for the "copied" status; the terminal copies
			// when the frame is flushed, in order with everything else.
			l = m.clipSeq + l
		}
		if y > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(l)
	}
	m.animOn = m.hover != "" // the light runs only under the pointer
	return b.String()
}

// listLines renders the /list rows: window number, name, title, unread.
func (m *Model) listLines(items []domain.Chat) []string {
	var out []string
	for _, c := range items {
		mark := "   "
		if wi, _ := m.winFor(c.ID); wi > 0 {
			mark = fmt.Sprintf("%2d ", wi)
		}
		row := fmt.Sprintf("%s %-16s %-30s", mark, m.display(c), sanitize(c.Title, false))
		if c.Unread > 0 {
			row += fmt.Sprintf("  %d unread", c.Unread)
		}
		if c.Deleted {
			row += "  (deleted)"
		}
		out = append(out, row) // the selected row is highlighted by View, with its buttons
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
		out = append(out, row) // the selected row is highlighted by View, with its buttons
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
	head := infoStyle.Render(" ") +
		seg(time.Now().Format("15:04"), 5, false) + gap +
		seg(m.opt.Provider.Name(), 0, false) + gap +
		seg(fmt.Sprintf("[%d:%s]", m.cur, m.winName(m.cur)), 3, true) + gap
	if tag := m.vimTag(); tag != "" { // NORMAL, while the input is in vim's Normal mode
		head += seg(tag, 4, true) + gap
	}
	if act := m.activity(); act != "" {
		head += actStyle.Render(act) + gap
	}
	tail := seg(fmt.Sprintf("%d msgs", len(m.msgs)), 1, false) + gap +
		seg(sync, syncColor, false) + gap
	if w.scroll > 0 && m.mode == modeWin {
		tail += actStyle.Render(fmt.Sprintf("scrolled up %d lines", w.scroll)) + gap
	}
	// The typing indicator sits between the activity list and the
	// counters and takes only the room they leave, so a narrow terminal
	// keeps the counters and a short form of the indicator.
	text := head
	if t := m.typingSegment(m.width - lipgloss.Width(head) - lipgloss.Width(tail) - 2); t != "" {
		text += t + gap
	}
	text += tail
	// The message itself is shown on the rule above, so this line never
	// has to truncate it.
	return barRow(text, m.width)
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
		case i == m.cur: // only the active window stands out, in the theme's status colours reversed
			st = infoStyle.Reverse(true).Bold(true)
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
	return barRow(infoStyle.Render(" ")+b.String(), m.width), spans
}

// rule is the thin line above the status rows. It carries the status
// message (what a view holds, an error in the error colour, or the last
// notice) at its left and a label at its right end.
func (m *Model) rule(label string) string {
	dash := func(n int) string { return dimStyle.Render(strings.Repeat("\u2500", max(n, 0))) }
	right := ""
	if label != "" {
		right = " " + label + " \u2500"
	}
	msg, msgStyle := m.modeInfo, infoStyle.Background(lipgloss.NoColor{})
	if msg == "" {
		msg = m.status
		if m.isErr {
			msgStyle = errStyle.Background(lipgloss.NoColor{})
		}
	}
	room := m.width - lipgloss.Width(right) - 4
	if msg == "" || room < 8 {
		return dash(m.width-lipgloss.Width(right)) + dimStyle.Render(right)
	}
	msg = ansi.Truncate(msg, room, "...")
	fill := m.width - lipgloss.Width(right) - lipgloss.Width(msg) - 4
	return dash(2) + " " + msgStyle.Render(msg) + " " + dash(fill) + dimStyle.Render(right)
}

// helpNotes is what /help prints before the command reference: how the
// client is used beyond its commands (themes, the mouse, selection,
// windows). One logical line each, wrapped when drawn.
const helpNotes = `Mouse, themes and windows
Themes: /theme lists the colour schemes, /theme <name> switches (kept).
Clickable text is bright and underlined; a running light plays over the one under the mouse pointer. /fx turns the light on or off (kept).
Drag with the left button to select text; on release it is copied to the clipboard: through the terminal (OSC 52) and the system tool when there is one (pbcopy, wl-copy, xclip, xsel, clip); /clip picks (kept). The selection stays highlighted until the next click, or until the text under it changes or a cursor in it moves.
Double click copies the word under the pointer, triple click the row; on a link or a name the first click acts and the next ones only select. Shift-click extends the selection where the terminal passes Shift on; most Linux terminals keep Shift-drag (Option on macOS) for their own selection, as does /mouse off.
Mouse: click a chat name or nick in window 0 to open its window, a tag in the window bar to focus it, a row in /list or /win to open it, a row of /settings to set its next value, a link or a [photo] / [file] token to open it with the system; the wheel scrolls.
Window 0 is the server window: every chat interleaved, plus notices (/feed off: the notices only); it stays at 0 and cannot be closed. /move N puts this window at N.
A window opens for a chat when it gets a message or has unread ones; [Act: 1,2] in the status line lists windows with unread messages.
People are @name, groups and channels are #name.`

// keyHelp is what /keys prints: the Keys table alone. Entries are one
// logical line each, laid out as in command.HelpText and wrapped when
// drawn.
const keyHelp = `Keys
  Tab                    on an empty prompt: next window (Shift-Tab previous); otherwise complete like bash: a command, a @user / #chan or a theme (in a group or channel window its @nicks, in the text and after /whois); Tab again lists the possibilities
  Ctrl-X                 send the typed text to this chat (after a short pause), as typed: a line starting with / goes out as a message, never as a command (Enter runs commands); Enter then adds a line to the message
  Ctrl-X twice (fast)    toggle "Enter sends" for this window; nothing is sent; Ctrl-X then adds a line instead, except to a line starting with / (no leading space), which it still sends as text
  Ctrl-N / Ctrl-P        next / previous window (Alt-Right / Alt-Left too)
  Alt-0..9, Esc 0..9     window 0..9 (0 is the server window)
  Left / Right           move the cursor; Ctrl-A / Ctrl-E start / end of the cursor's line; Backspace / Delete remove before / under the cursor; Ctrl-W the word before it, Ctrl-K to the end of the line, Ctrl-U all; in /list or /win: pick the row's action button (open or focus, read, unread, close), Enter does it; in /settings: the selected setting's previous / next value
  Up / Down              in a message of several rows: the row above / below, keeping the column; on the first / last row: walk the input history (commands and sent lines); a recalled line is input again: Ctrl-X sends it, Enter runs it when it starts with /
  PgUp / PgDn            scroll a page (the wheel scrolls too)
  Home / End             oldest / newest
  Ctrl-R                 sync now
  Ctrl-L                 redraw: drop notices and pictures (/redraw)
  F1                     the notes and the command list (/help); /keys prints this
  Esc                    close the list, the pager or the settings menu; clear the input
  vim: Normal mode       /vim on: Esc enters it, i a I A o O leave it; typed keys are commands, never text: h l w b e 0 ^ $ move (Esc 0 is still window 0), j k as Down / Up, x X D dd dw cw cc S C r edit, u undoes, p P put back; Ctrl-E Y D U F B and gg G scroll; gt gT switch windows; : / ? a command, a search
  vim: lists, pager      /vim on: they open in Normal: j k move, h l as Left / Right, gg G ends, Ctrl-D / Ctrl-U half a page, q closes; / or i to type, Esc stops
  Ctrl-C                 quit`
