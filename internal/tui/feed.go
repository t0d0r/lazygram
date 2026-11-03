package tui

import "strings"

// The server window shows every chat's messages that arrive during the
// session, interleaved, plus its notices. /feed off keeps the chat
// messages out of it, the IRC way, where the server window holds only
// notices: banner, hints, /help and /keys, errors, windows opened and
// closed. Nothing is dropped: the messages are still recorded as live and
// shown in their own windows, and /feed on brings back this session's
// messages, those that arrived while it was off included, since the
// setting is a filter applied when window 0 is rendered (richLinesOf).
// The notices of the chats' activity (a reaction someone added) go with
// the messages.

// feedOffNotice is the start notice in window 0 when /feed is off.
const feedOffNotice = "/feed is off: chat messages show only in their own windows (/feed on)"

// setFeed implements /feed: no argument shows the setting, on or off sets
// it, takes effect at once and keeps it.
func (m *Model) setFeed(arg string) {
	switch strings.ToLower(strings.TrimSpace(arg)) {
	case "":
		if m.feed {
			m.setInfo("feed on: window 0 shows every chat's messages as they arrive; /feed off")
		} else {
			m.setInfo("feed off: window 0 shows only notices; chat messages stay in their own windows; /feed on")
		}
	case "on":
		m.feed = true
		_ = m.opt.Store.SetState(feedKey, true)
		m.feedChanged()
		m.setInfo("feed on: window 0 shows every chat's messages as they arrive, this session's so far included")
	case "off":
		m.feed = false
		_ = m.opt.Store.SetState(feedKey, false)
		m.feedChanged()
		m.setInfo("feed off: window 0 shows only notices; chat messages stay in their own windows")
	default:
		m.setErr("/feed takes on or off")
	}
}

// withoutChatNotes leaves out the notices about the chats' activity (a
// reaction someone added), which /feed off keeps out of window 0 with the
// messages; the rest of its notices stay.
func withoutChatNotes(notes []note) []note {
	out := make([]note, 0, len(notes))
	for _, n := range notes {
		if !n.chat {
			out = append(out, n)
		}
	}
	return out
}

// feedChanged keeps the server window's scroll offset within its lines,
// whose number the setting just changed: a window scrolled back further
// than it now reaches shows its oldest page.
func (m *Model) feedChanged() {
	w := m.wins[0]
	if w.scroll == 0 {
		return
	}
	lines, _ := m.richLinesOf(w)
	w.scroll = min(w.scroll, max(len(lines)-m.contentHeight(), 0))
}
