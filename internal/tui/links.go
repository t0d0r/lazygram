package tui

import "lazygram/internal/opener"

// recentLinkCount is how many of a chat's links Tab offers after /open.
const recentLinkCount = 10

// recentLinks lists the distinct http(s) links in a chat's messages,
// newest message first, at most n of them. Links come from the same
// sanitised text the renderer shows (control characters removed, so none
// of a sender's escape sequences can reach the input line) and are cut the
// way it cuts them (trailing sentence punctuation is not part of a link),
// so a candidate is exactly what a click on it would open.
func (m *Model) recentLinks(chatID string, n int) []string {
	var out []string
	seen := map[string]bool{}
	for i := len(m.msgs) - 1; i >= 0 && len(out) < n; i-- {
		x := m.msgs[i]
		if x.ChatID != chatID {
			continue
		}
		for _, u := range urlsOf(sanitize(x.Text, true)) {
			if !seen[u] && isWebLink(u) {
				seen[u] = true
				out = append(out, u)
				if len(out) == n {
					break
				}
			}
		}
	}
	return out
}

// isWebLink is the one rule for what lazygram hands to the browser: an
// http or https URL with a host (specs/security.md). It is the opener's
// own rule, which the opener enforces again for a click.
func isWebLink(s string) bool { return opener.IsLink(s) }
