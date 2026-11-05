package tui

import (
	"bytes"
	"regexp"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"lazygram/internal/command"
	"lazygram/internal/store"
)

// hintsIn are the hint notices of a window, in order.
func hintsIn(w *win) []string {
	var out []string
	for _, n := range w.notes {
		if strings.HasPrefix(n.text, "hint: ") {
			out = append(out, n.text)
		}
	}
	return out
}

// lastPicks draws the last pool entry each time, recording the sizes asked.
func lastPicks(sizes *[]int) func(int) int {
	return func(n int) int {
		*sizes = append(*sizes, n)
		return n - 1
	}
}

func hintModel(t *testing.T, opt Options) *Model {
	t.Helper()
	st, err := store.Open(store.Memory, bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	opt.Store = st
	if opt.Provider == nil {
		opt.Provider = newFake()
	}
	return New(opt)
}

func TestStartHintsFixedFirstThenTwoDrawn(t *testing.T) {
	var sizes []int
	m := hintModel(t, Options{Rand: lastPicks(&sizes)})
	n := len(hintPool)
	want := []string{hintsFixed[0], hintsFixed[1], hintPool[n-1], hintPool[n-2]}
	if got := hintsIn(m.wins[0]); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("hints:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if len(sizes) != 2 || sizes[0] != n || sizes[1] != n-1 {
		t.Fatalf("two draws, the second from the pool without the first: %v", sizes)
	}
	// They follow the tagline and the /help line on the start-up screen.
	m.revealAll()
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m.Update(m.chatsCmd()())
	m.Update(m.syncCmd()())
	v := ansi.Strip(m.View())
	at := strings.Index(v, "-!- /help lists commands, /keys the shortcuts")
	for _, h := range want {
		i := strings.Index(v, "-!- "+h)
		if i < 0 || i < at || at < 0 {
			t.Fatalf("%q not shown in order after the /help line:\n%s", h, v)
		}
		at = i
	}
	if tag := strings.Index(v, "-!- lazygram "); tag < 0 || tag > strings.Index(v, "-!- "+want[0]) {
		t.Fatalf("hints come after the tagline:\n%s", v)
	}
}

func TestStartHintsNeverRepeat(t *testing.T) {
	inPool := map[string]bool{}
	for _, h := range hintPool {
		inPool[h] = true
	}
	pool := strings.Join(hintPool, "\n")
	// Every pair of draws, the same index twice included.
	for i := range hintPool {
		for j := range len(hintPool) - 1 {
			seq := []int{i, j}
			got := startHints(func(n int) int {
				k := seq[0]
				seq = seq[1:]
				return k
			})
			if len(got) != 4 || got[0] != hintsFixed[0] || got[1] != hintsFixed[1] {
				t.Fatalf("draws %d,%d: fixed hints first: %q", i, j, got)
			}
			if got[2] == got[3] || !inPool[got[2]] || !inPool[got[3]] {
				t.Fatalf("draws %d,%d: two different pool hints: %q", i, j, got[2:])
			}
		}
	}
	if got := startHints(func(int) int { return 0 }); got[2] != hintPool[0] || got[3] != hintPool[1] {
		t.Fatalf("index 0 twice is the first two pool hints: %q", got[2:])
	}
	if strings.Join(hintPool, "\n") != pool {
		t.Fatal("drawing must not change the pool itself")
	}
}

func TestStartHintsDifferBetweenStarts(t *testing.T) {
	st, err := store.Open(store.Memory, bytes.Repeat([]byte{8}, 32))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	seen := map[string]bool{}
	for range 40 {
		h := hintsIn(New(Options{Store: st, Provider: newFake()}).wins[0])
		if len(h) != 4 || h[0] != hintsFixed[0] || h[1] != hintsFixed[1] || h[2] == h[3] {
			t.Fatalf("each start: the fixed two, then two different ones: %q", h)
		}
		seen[h[2]+"\n"+h[3]] = true
	}
	if len(seen) < 2 {
		t.Fatal("without Options.Rand the drawn hints vary between starts")
	}
}

func TestStartHintsOnlyOnceAndOnlyInTheServerWindow(t *testing.T) {
	var sizes []int
	m := hintModel(t, Options{Rand: lastPicks(&sizes)})
	m.revealAll()
	m.fx = false
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	m.Update(m.chatsCmd()())
	m.Update(m.syncCmd()())
	enter(m, "/refresh")
	m.Update(m.chatsCmd()()) // what /refresh asks for: a later chat list
	m.Update(m.syncCmd()())
	m.Update(m.chatsCmd()())
	enter(m, "/win 0")
	if got := hintsIn(m.wins[0]); len(got) != 4 || len(sizes) != 2 {
		t.Fatalf("one block of four hints per start, drawn once: %d hints, %d draws", len(got), len(sizes))
	}
	if len(m.wins) < 2 {
		t.Fatal("the fake opens chat windows")
	}
	for i, w := range m.wins[1:] {
		if h := hintsIn(w); len(h) != 0 {
			t.Fatalf("window %d shows hints: %q", i+1, h)
		}
	}
}

func TestStartHintsRollInWithTheStartScreen(t *testing.T) {
	var sizes []int
	m := hintModel(t, Options{Rand: lastPicks(&sizes)})
	m.fx = false
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	if !m.wins[0].revealing {
		t.Fatal("the server window rolls in at start")
	}
	bottom := map[string]bool{} // what arrived at the bottom row, tick by tick
	for i := 0; i < 200 && m.wins[0].revealing; i++ {
		m.Update(revealTickMsg{})
		rows := strings.Split(ansi.Strip(body(m)), "\n")
		bottom[strings.TrimSpace(rows[len(rows)-1])] = true
	}
	if m.wins[0].revealing {
		t.Fatal("the reveal ends")
	}
	hints := hintsIn(m.wins[0])
	if len(hints) != 4 {
		t.Fatalf("four hints at start: %q", hints)
	}
	for _, h := range hints {
		found := false
		for b := range bottom {
			found = found || strings.HasSuffix(b, "-!- "+h)
		}
		if !found {
			t.Fatalf("%q did not roll in as a line of its own", h)
		}
	}
}

func TestHintsAreWellFormed(t *testing.T) {
	if len(hintsFixed) != 2 || !strings.HasPrefix(hintsFixed[0], "hint: /list ") || !strings.HasPrefix(hintsFixed[1], "hint: /win ") {
		t.Fatalf("the fixed hints are /list, then /win: %q", hintsFixed)
	}
	if len(hintPool) < 12 {
		t.Fatalf("the pool holds at least 12 hints: %d", len(hintPool))
	}
	slash := regexp.MustCompile(`(?:^|[\s(])(/[a-z?-]+)`)
	// The words Tab offers after a command are the settings it takes.
	settings := map[string]bool{}
	for _, ws := range argWords {
		for _, w := range ws {
			settings[w] = true
		}
	}
	withArg := regexp.MustCompile(`(?:^|[\s(])(/[a-z?-]+) ([a-z0-9-]+)`)
	thisOne := regexp.MustCompile(`(?i)\bthis (chat|window)`)
	seen := map[string]bool{}
	for _, h := range append(append([]string(nil), hintsFixed...), hintPool...) {
		if seen[h] {
			t.Errorf("duplicate hint %q", h)
		}
		seen[h] = true
		body := strings.TrimPrefix(h, "hint: ")
		if body == h || strings.TrimSpace(body) != body || body == "" || strings.HasSuffix(body, ".") {
			t.Errorf("%q: \"hint: \", then text without surrounding space or a final period", h)
		}
		// One row of an 80-column window after the "HH:MM -!- " stamp.
		if len(h) > 70 {
			t.Errorf("%q: %d columns, at most 70", h, len(h))
		}
		for _, r := range h {
			if r < ' ' || r > '~' {
				t.Errorf("%q: not plain ASCII: %q", h, r)
			}
		}
		if sanitize(h, false) != h {
			t.Errorf("%q: changed by sanitize", h)
		}
		// Read in window 0, which has no chat: an action that needs one
		// is said to work in a chat window.
		if thisOne.MatchString(h) {
			t.Errorf("%q: read in window 0, it must not speak of this chat or window", h)
		}
		needsChat := strings.Contains(h, "Ctrl-X")
		for _, m := range slash.FindAllStringSubmatch(h, -1) {
			c, err := command.Parse(m[1])
			if err != nil {
				t.Errorf("%q names %s: %v", h, m[1], err)
				continue
			}
			_, ok := hintNeedsChat[c.Kind]
			needsChat = needsChat || ok
		}
		if needsChat && !strings.Contains(h, "chat window") {
			t.Errorf("%q: window 0 refuses it, so the hint says \"chat window\"", h)
		}
		// "/autojoin off" is fine; "/mouse off" is not: /mouse toggles.
		for _, m := range withArg.FindAllStringSubmatch(h, -1) {
			c, err := command.Parse(m[1])
			if err != nil || !settings[m[2]] {
				continue
			}
			if !slices.Contains(argWords[c.Kind], m[2]) {
				t.Errorf("%q: %s takes no %q", h, m[1], m[2])
			}
		}
	}
}

// hintNeedsChat are the commands the server window refuses, with a line
// that runs each; with Ctrl-X they need a chat window.
var hintNeedsChat = map[command.Kind]string{
	command.SendFile: "/send /nonexistent", command.MarkRead: "/mark-as-read",
	command.Last: "/last 5", command.Move: "/move 2", command.Close: "/close",
	command.Edit: "/edit x", command.Delete: "/delete", command.Me: "/me x",
	command.Press: "/press 1", command.CloseOthers: "/close-others",
	command.Like: "/like",
}

// TestHintNeedsChatHolds keeps hintNeedsChat true of the code: each of
// those commands, and Ctrl-X twice, fails in window 0.
func TestHintNeedsChatHolds(t *testing.T) {
	m := hintModel(t, Options{Rand: func(int) int { return 0 }})
	m.revealAll()
	m.fx = false
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	m.Update(m.chatsCmd()())
	m.Update(m.syncCmd()())
	enter(m, "/win 0")
	n := len(m.wins)
	if m.cur != 0 || n < 2 {
		t.Fatalf("in window 0 with chat windows open: cur %d, %d windows", m.cur, n)
	}
	for k, line := range hintNeedsChat {
		if c, err := command.Parse(line); err != nil || c.Kind != k {
			t.Fatalf("%q does not run the command it stands for: %v", line, err)
		}
		m.status, m.isErr = "", false
		enter(m, line)
		if !m.isErr || m.cur != 0 || len(m.wins) != n {
			t.Errorf("%q in window 0: %q; want an error, no window moved or closed", line, m.status)
		}
	}
	m.status, m.isErr = "", false
	typ(m, "hi")
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	if !m.isErr || m.wins[0].enterSends {
		t.Fatalf("Ctrl-X twice in window 0: %q, enter sends %v; want an error and no toggle", m.status, m.wins[0].enterSends)
	}
}
