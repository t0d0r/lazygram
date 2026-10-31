package telegram

import (
	"strconv"
	"strings"
	"unicode"
)

const maxAlias = 14 // the feed's alias column is capped at this width

// aliasFor builds the IRC-style channel name for a chat: its username when it
// has one, else a slug of its title. Names already in taken get -2, -3, ...
// The chosen name is recorded in taken.
func aliasFor(e entity, taken map[string]bool) string {
	base := slug(e.username)
	if base == "" {
		base = slug(e.title)
	}
	if base == "" {
		base = "chat"
	}
	name := base
	for n := 2; taken[name]; n++ {
		suffix := "-" + strconv.Itoa(n)
		name = trimRunes(base, maxAlias-len(suffix)) + suffix
	}
	taken[name] = true
	return name
}

// slug lowercases s, keeps letters and digits, turns other runs into one
// dash, and caps the length so it fits the feed's alias column.
func slug(s string) string {
	var b []rune
	dash := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if len(b) >= maxAlias {
			break
		}
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b = append(b, r)
			dash = false
		case len(b) > 0 && !dash:
			b = append(b, '-')
			dash = true
		}
	}
	return strings.TrimRight(string(b), "-")
}

func trimRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		r = r[:n]
	}
	return strings.TrimRight(string(r), "-")
}
