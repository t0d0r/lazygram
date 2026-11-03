package domain

import (
	"errors"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode"
	"unicode/utf8"
)

// Fuzz targets for the text other people control: message text (mentions,
// senders, search), names and usernames from the service, and what is
// typed after /whois, /win and Tab. Run one with
// go test ./internal/domain/ -run '^$' -fuzz '^FuzzMentionsIn$'.

// fuzzSeeds are the edge cases every text target starts from: Cyrillic,
// emoji, combining marks, ANSI escapes, C0 and C1 controls, invalid UTF-8,
// spaces of more than one byte and long input.
var fuzzSeeds = []string{
	"",
	" ",
	"Иван Петров",
	"Ива\u0301н",   // a stress mark
	"\u0301\u0300", // combining marks only
	"ЁЛКА Щи Київ Ґанок Ѝ Ўзбек",
	"Stoyan stoyan_k",
	"\U0001F389\U0001F468\u200d\U0001F469\u200d\U0001F467 party",
	"\x1b[31mred\x1b[0m \x1b]8;;http://x\x07link\x1b]8;;\x07",
	"\x00\x07\b\t\r\n\x7f",
	"\u0085\u009b31m\u00a0\u2028\u3000",
	"\xff\xfe\xd0\xe2\x80 \xc2",
	"\u0130\u212a\u1e9e\u01c5", // case mappings that leave their block
	strings.Repeat("Щастие @user_one ", 200),
}

// fuzzSpaces are white space of one to three bytes: the separators of the
// words MentionsIn looks at.
var fuzzSpaces = []string{" ", "\n", "\t", "\u0085", "\u00a0", "\u1680", "\u2028", "\u202f", "\u3000"}

// fuzzBefore and fuzzAfter are what may stand right before an @ and right
// after a username in a mention (specs/ui.md, Members): before, no letter
// or digit in any script, _ or one of / . @ = &; after, no letter, digit
// or /. None makes the word a link.
var (
	fuzzBefore = []string{"", "(", "\u00ab", "\"", "*", "\U0001F389", "\u0301", "\xff"}
	fuzzAfter  = []string{"", ")", "\u00bb", ".", ",", "!", "?", ":", ";", "'", "-", "@", "\U0001F389", "\u0301", "\xff", "\x1b[0m"}
)

// FuzzMentionsIn: MentionsIn finds exactly the mentions the rule in
// specs/ui.md, Members, finds (specMentions), each a username, once and a
// mention where it stands; a mention put between white space into any
// text, with what may stand around one, is found; texts joined by white
// space mention what each mentions; ASCII case never decides what is a
// mention.
func FuzzMentionsIn(f *testing.F) {
	for i, s := range fuzzSeeds {
		f.Add(s, "", byte(i), "", uint(len(s)/2), byte(i*9))
	}
	for i, s := range []string{
		"@petar_1 happy birthday!",
		"thanks, @Ada_L. And @ada_l again",
		"mail@example.com имейл@primer.com @ab @1abc (@grace_h) @@lead_x",
		"https://www.youtube.com/@Some_Channel medium.com/@writer_x/post",
		"www.example.com/(@path_one)",
		"WWW.Example.com/(@path_one) HTTPS://x.org/(@path_two)",
		"@nickя, \u00ab@quoted_one\u00bb, \U0001F389@party_one @bob\u0301",
		"@" + strings.Repeat("a", 32) + " @" + strings.Repeat("b", 33),
		"\x1b[1m@bold_one\x1b[0m \xff@after_bad \u0085@after_nel",
		// specs/ui.md: no mention after _ . = & or before a /.
		"a_@abcd b.@abcd c=@abcd d&@abcd @abcd/x",
	} {
		f.Add(s, "@second_one", byte(4), "ada_l", uint(i*7), byte(i*17))
	}
	// TestMentionsIn's cases built around the fuzzed text: (@grace_h) put
	// before a link word, a quoted mention put into one (at 9), and a
	// mention after an emoji put into a word with a stress mark.
	f.Add("www.example.com/x", "", byte(4), "grace_h", uint(0), byte(1+1*8))
	f.Add("www.example.com/x", "", byte(6), "quoted_one", uint(9), byte(2+2*8))
	f.Add("Ива\u0301н", "", byte(8), "party_one", uint(8), byte(5+12*8))
	// Regression: a link after a space of more than one byte (no-break, as
	// pasted from a web page; the C1 next-line) was read as starting one
	// byte into the space, so www. did not make it a link and (@path_one)
	// counted as a mention. The word starts after the whole space.
	f.Add("see\u00a0www.example.com/(@path_one)", "", byte(0), "", uint(0), byte(0))
	f.Add("", "www.example.com/(@path_one)", byte(3), "", uint(0), byte(0))
	f.Fuzz(func(t *testing.T, a, b string, sep byte, nick string, at uint, edge byte) {
		got := MentionsIn(a)
		if want := specMentions(a); !slices.Equal(got, want) {
			t.Fatalf("MentionsIn(%q) = %q, the spec reads %q", a, got, want)
		}
		seen := map[string]bool{}
		for _, nick := range got {
			if !nickShaped(nick) {
				t.Fatalf("MentionsIn(%q): %q is no username", a, nick)
			}
			if k := strings.ToLower(nick); seen[k] {
				t.Fatalf("MentionsIn(%q): %q twice", a, nick)
			} else {
				seen[k] = true
			}
			if !mentionedAt(a, nick) {
				t.Fatalf("MentionsIn(%q): @%s is not a mention where it stands", a, nick)
			}
		}
		sp := fuzzSpaces[int(sep)%len(fuzzSpaces)]
		joined := MentionsIn(a + sp + b)
		if want := foldUnique(append(MentionsIn(a), MentionsIn(b)...)); !reflect.DeepEqual(joined, want) {
			t.Fatalf("MentionsIn(%q + %q + %q) = %q, the parts mention %q", a, sp, b, joined, want)
		}
		if swapped := lowerAll(MentionsIn(swapASCIICase(a))); !reflect.DeepEqual(swapped, lowerAll(got)) {
			t.Fatalf("MentionsIn(%q) = %q but with ASCII case swapped %q", a, got, swapped)
		}
		// A mention between white space, at a rune boundary of a: the
		// spaces end the words of a around it, so nothing in a can
		// lengthen the username, stand right before the @ or make the word
		// a link.
		nick = fuzzNick(nick)
		k := int(at % uint(len(a)+1))
		for k < len(a) && !utf8.RuneStart(a[k]) {
			k++
		}
		before := fuzzBefore[int(edge)%len(fuzzBefore)]
		after := fuzzAfter[int(edge)/len(fuzzBefore)%len(fuzzAfter)]
		in := a[:k] + sp + before + "@" + nick + after + sp + a[k:]
		found := MentionsIn(in)
		if !slices.Contains(lowerAll(found), strings.ToLower(nick)) {
			t.Fatalf("MentionsIn(%q) = %q, without the @%s put in", in, found, nick)
		}
		if want := specMentions(in); !slices.Equal(found, want) {
			t.Fatalf("MentionsIn(%q) = %q, the spec reads %q", in, found, want)
		}
	})
}

// fuzzNick makes a username out of any string: its Latin letters, digits
// and underscores from the first letter on, at most 32, padded with x to 4.
func fuzzNick(s string) string {
	var b []byte
	for i := 0; i < len(s) && len(b) < 32; i++ {
		c := s[i]
		letter := 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
		if letter || len(b) > 0 && (c == '_' || '0' <= c && c <= '9') {
			b = append(b, c)
		}
	}
	for len(b) < 4 {
		b = append(b, 'x')
	}
	return string(b)
}

// specMentions reads text by the rule in specs/ui.md, Members, on its own
// terms, word by word (a word lies between white space): in a word that
// neither holds :// nor starts with www., ignoring case, an @ at the
// word's start or after a rune that is no letter or digit in any script,
// no _ and none of / . @ = &, then the whole run of Latin letters, digits
// and underscores after it when that is a username (nickShaped), then the
// word's end or a rune that is no letter, digit or /. Each username once,
// ignoring case, the first spelling, in order.
func specMentions(text string) []string {
	var nicks []string
	word := func(w string) {
		if lw := strings.ToLower(w); strings.Contains(lw, "://") || strings.HasPrefix(lw, "www.") {
			return
		}
		prev := ' ' // the word's start: after white space or the text's start
		for i, r := range w {
			if r == '@' && !strings.ContainsRune("_/.@=&", prev) && !unicode.IsLetter(prev) && !unicode.IsDigit(prev) {
				rest := w[i+1:]
				n := strings.IndexFunc(rest, func(r rune) bool {
					return r >= utf8.RuneSelf || r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r)
				})
				if n < 0 {
					n = len(rest)
				}
				next, size := utf8.DecodeRuneInString(rest[n:])
				if nickShaped(rest[:n]) && (size == 0 || next != '/' && !unicode.IsLetter(next) && !unicode.IsDigit(next)) {
					nicks = append(nicks, rest[:n])
				}
			}
			prev = r
		}
	}
	start := 0
	for i, r := range text {
		if unicode.IsSpace(r) {
			word(text[start:i])
			start = i + utf8.RuneLen(r)
		}
	}
	word(text[start:])
	return foldUnique(nicks)
}

// nickShaped reports whether s is a Telegram username as MentionsIn reads
// one: a Latin letter, then 3 to 31 Latin letters, digits or underscores.
func nickShaped(s string) bool {
	if len(s) < 4 || len(s) > 32 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		letter := 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
		if !letter && (i == 0 || c != '_' && (c < '0' || c > '9')) {
			return false
		}
	}
	return true
}

// mentionedAt reports whether text holds @nick somewhere that the spec
// calls a mention: no letter, digit, _ / . @ = & right before it, no
// letter, digit, _ or / right after it, and not in a word (between white
// space) that holds :// or starts with www.
func mentionedAt(text, nick string) bool {
	at := "@" + nick
	for off := 0; ; {
		k := strings.Index(text[off:], at)
		if k < 0 {
			return false
		}
		i := off + k
		off = i + 1
		if i > 0 {
			r, _ := utf8.DecodeLastRuneInString(text[:i])
			if strings.ContainsRune("_/.@=&", r) || unicode.IsLetter(r) || unicode.IsDigit(r) {
				continue
			}
		}
		if j := i + len(at); j < len(text) {
			r, _ := utf8.DecodeRuneInString(text[j:])
			if r == '_' || r == '/' || unicode.IsLetter(r) || unicode.IsDigit(r) {
				continue
			}
		}
		start := i
		for start > 0 {
			r, n := utf8.DecodeLastRuneInString(text[:start])
			if unicode.IsSpace(r) {
				break
			}
			start -= n
		}
		end := i
		for end < len(text) {
			r, n := utf8.DecodeRuneInString(text[end:])
			if unicode.IsSpace(r) {
				break
			}
			end += n
		}
		if w := strings.ToLower(text[start:end]); strings.Contains(w, "://") || strings.HasPrefix(w, "www.") {
			continue
		}
		return true
	}
}

// foldUnique keeps the first of the strings equal ignoring case, in order.
func foldUnique(ss []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, s := range ss {
		if k := strings.ToLower(s); !seen[k] {
			seen[k] = true
			out = append(out, s)
		}
	}
	return out
}

func lowerAll(ss []string) []string {
	var out []string
	for _, s := range ss {
		out = append(out, strings.ToLower(s))
	}
	return out
}

// swapASCIICase turns ASCII upper case into lower and back; other bytes
// stay, invalid UTF-8 included.
func swapASCIICase(s string) string {
	b := []byte(s)
	for i, c := range b {
		switch {
		case 'a' <= c && c <= 'z':
			b[i] = c - 'a' + 'A'
		case 'A' <= c && c <= 'Z':
			b[i] = c - 'A' + 'a'
		}
	}
	return string(b)
}

// FuzzTranslit: the transliteration is valid UTF-8 whatever comes in, has
// no upper case, no mapped Cyrillic letter and no combining mark left, is
// idempotent, maps rune by rune (so a plain match is a transliterated one,
// specs/ui.md, Search in either script) and leaves ASCII as it is, lower
// case.
func FuzzTranslit(f *testing.F) {
	for _, s := range fuzzSeeds {
		f.Add(s, "Ива\u0301н")
	}
	f.Fuzz(func(t *testing.T, a, b string) {
		out := Translit(a)
		if !utf8.ValidString(out) {
			t.Fatalf("Translit(%q) = %q, not UTF-8", a, out)
		}
		for _, r := range out {
			switch {
			case unicode.ToLower(r) != r:
				t.Fatalf("Translit(%q) = %q keeps upper case %q", a, out, r)
			case r >= 0x300 && r <= 0x36f:
				t.Fatalf("Translit(%q) = %q keeps combining mark %U", a, out, r)
			case r >= 0x400 && r <= 0x4ff && latinOf[r-0x400] != "":
				t.Fatalf("Translit(%q) = %q keeps Cyrillic %q", a, out, r)
			}
		}
		if again := Translit(out); again != out {
			t.Fatalf("Translit(%q) = %q, again %q", a, out, again)
		}
		if utf8.ValidString(a) {
			if whole, parts := Translit(a+b), out+Translit(b); whole != parts {
				t.Fatalf("Translit(%q + %q) = %q, the parts give %q", a, b, whole, parts)
			}
		}
		ascii := true
		for i := 0; i < len(a); i++ {
			ascii = ascii && a[i] < utf8.RuneSelf
		}
		if ascii && out != strings.ToLower(a) {
			t.Fatalf("Translit(%q) = %q, ASCII stays as it is", a, out)
		}
	})
}

// FuzzMatcher: a search query matches a text exactly when the text's
// transliteration contains the trimmed query's, a plain case-insensitive
// match always matches, a query of nothing (after transliteration)
// matches everything, and reusing the matcher's buffer changes nothing.
func FuzzMatcher(f *testing.F) {
	for i, s := range fuzzSeeds {
		f.Add(s, fuzzSeeds[(i+1)%len(fuzzSeeds)], "иван")
		f.Add("Среща утре в София с Иван", s, s)
	}
	f.Add("Meeting with Ivan", "", " Ivan ")
	f.Add("Ива\u0301н", "x", "\u0301")
	f.Fuzz(func(t *testing.T, hay, other, query string) {
		mt := NewMatcher(query)
		q := strings.TrimSpace(query)
		got := mt.Match(hay)
		if want := strings.Contains(Translit(hay), Translit(q)); got != want {
			t.Fatalf("Match(%q, %q) = %v, the transliterations say %v", hay, query, got, want)
		}
		if strings.Contains(strings.ToLower(hay), strings.ToLower(q)) && !got {
			t.Fatalf("Match(%q, %q): a plain match must match", hay, query)
		}
		if mt.Empty() != (Translit(q) == "") || mt.Empty() && !got {
			t.Fatalf("NewMatcher(%q): Empty %v, Match(%q) %v", query, mt.Empty(), hay, got)
		}
		mt.Match(other)
		if mt.Match(hay) != got || MatchText(hay, query) != got {
			t.Fatalf("Match(%q, %q) changed after Match(%q)", hay, query, other)
		}
	})
}

// FuzzPrefixMatch: Tab completion's prefix test holds for the empty
// prefix, a string itself and every string it starts (whole runes), is
// transitive, takes every plain case-insensitive prefix, and never offers
// what a search for the same text would not find.
func FuzzPrefixMatch(f *testing.F) {
	for _, s := range fuzzSeeds {
		f.Add(s, "iv", " Петров")
	}
	f.Add("Иван Петров", "ИВ", "i")
	f.Add("Gracelyn", "grace ", "grace")
	f.Add("#новини", "#nov", "#")
	f.Add("Ж\xd0", "\xd0", "zh")
	f.Fuzz(func(t *testing.T, s, prefix, rest string) {
		got := PrefixMatch(s, prefix)
		if !PrefixMatch(s, "") || !PrefixMatch(s, s) {
			t.Fatalf("PrefixMatch(%q, \"\" or itself) is false", s)
		}
		if utf8.ValidString(s) && !PrefixMatch(s+rest, s) {
			t.Fatalf("PrefixMatch(%q, %q) is false", s+rest, s)
		}
		if strings.HasPrefix(strings.ToLower(s), strings.ToLower(prefix)) && !got {
			t.Fatalf("PrefixMatch(%q, %q): a plain prefix must match", s, prefix)
		}
		if got && PrefixMatch(prefix, rest) && !PrefixMatch(s, rest) {
			t.Fatalf("PrefixMatch is not transitive: %q, %q, %q", s, prefix, rest)
		}
		if got && !MatchText(s, prefix) {
			t.Fatalf("PrefixMatch(%q, %q) but MatchText is false", s, prefix)
		}
	})
}

// fuzzMemberList reads one member per line, "name\tnick"; the line number
// is the id, so results can be told apart.
func fuzzMemberList(list string) []Member {
	var ms []Member
	for i, line := range strings.Split(list, "\n") {
		if i == 32 {
			break
		}
		name, nick, _ := strings.Cut(line, "\t")
		ms = append(ms, Member{Name: name, Nick: nick, ID: strconv.Itoa(i)})
	}
	return ms
}

// FuzzResolveMember: /whois finds a member only when the query matches
// them in one of the spec's tiers (specs/ui.md, /whois), finds the one
// member any tier matches, says ambiguous only when several match, lets a
// unique exact name (or with @ a unique exact username) win, and does not
// depend on the order of the members.
func FuzzResolveMember(f *testing.F) {
	list := "Alice Smith\talice_s\nAlan\t\nИван Петров\t\nBob\talan_b\nalan\talan\n\u0301\t\nОливия\t"
	for _, q := range []string{"@alice_s", "ALICE SMITH", "alan", "@alan", "al", "iv", "петр", "\u0301", "@", "", " @ bob ", "#dev"} {
		f.Add(list, q)
	}
	for _, s := range fuzzSeeds {
		f.Add(s+"\t"+s+"\nx\t"+s, s)
	}
	f.Fuzz(func(t *testing.T, list, query string) {
		ms := fuzzMemberList(list)
		lq := strings.ToLower(strings.TrimSpace(query))
		at := strings.HasPrefix(lq, "@")
		q := strings.TrimPrefix(lq, "@")
		mt := NewMatcher(q)
		hit := func(m Member) bool {
			if strings.Contains(strings.ToLower(m.Name), q) || m.Nick != "" && strings.HasPrefix(strings.ToLower(m.Nick), q) {
				return true
			}
			return !mt.Empty() && (mt.Match(m.Name) || m.Nick != "" && PrefixMatch(m.Nick, q))
		}
		var hits, exact []Member
		for _, m := range ms {
			if hit(m) {
				hits = append(hits, m)
			}
			if at && m.Nick != "" && strings.ToLower(m.Nick) == q || !at && strings.ToLower(m.Name) == q {
				exact = append(exact, m)
			}
		}
		got, err := ResolveMember(ms, query)
		switch {
		case q == "":
			if !errors.Is(err, ErrNoMember) {
				t.Fatalf("ResolveMember(%q): an empty query finds %+v, %v", query, got, err)
			}
			return
		case err == nil:
			if n, _ := strconv.Atoi(got.ID); n >= len(ms) || !reflect.DeepEqual(ms[n], got) || !hit(got) {
				t.Fatalf("ResolveMember(%q) = %+v, which does not match", query, got)
			}
		case errors.Is(err, ErrNoMember):
			if len(hits) > 0 {
				t.Fatalf("ResolveMember(%q): %v, yet %+v match", query, err, hits)
			}
		case errors.Is(err, ErrAmbiguousMember):
			if len(hits) < 2 {
				t.Fatalf("ResolveMember(%q): %v, yet only %+v match", query, err, hits)
			}
		default:
			t.Fatalf("ResolveMember(%q): unexpected error %v", query, err)
		}
		if len(hits) == 1 && (err != nil || got.ID != hits[0].ID) {
			t.Fatalf("ResolveMember(%q) = %+v, %v; only %+v matches", query, got, err, hits[0])
		}
		if len(exact) == 1 && (err != nil || got.ID != exact[0].ID) {
			t.Fatalf("ResolveMember(%q) = %+v, %v; the exact match is %+v", query, got, err, exact[0])
		}
		rev := make([]Member, len(ms))
		for i, m := range ms {
			rev[len(ms)-1-i] = m
		}
		got2, err2 := ResolveMember(rev, query)
		if got2.ID != got.ID || errClass(err2) != errClass(err) {
			t.Fatalf("ResolveMember(%q) depends on the order: %+v, %v reversed %+v, %v", query, got, err, got2, err2)
		}
	})
}

// errClass names the lookup error kinds, for comparing two lookups.
func errClass(err error) string {
	switch {
	case err == nil:
		return "found"
	case errors.Is(err, ErrNoMember), errors.Is(err, ErrNoChat):
		return "none"
	case errors.Is(err, ErrAmbiguousMember), errors.Is(err, ErrAmbiguous):
		return "ambiguous"
	}
	return err.Error()
}

// fuzzChatList reads one chat per line, "id\talias\ttitle"; the kind goes
// person, group, channel by line.
func fuzzChatList(list string) []Chat {
	var cs []Chat
	for i, line := range strings.Split(list, "\n") {
		if i == 32 {
			break
		}
		f := strings.SplitN(line, "\t", 3)
		for len(f) < 3 {
			f = append(f, "")
		}
		cs = append(cs, Chat{ID: f[0], Alias: f[1], Title: f[2], Kind: ChatKind(i % 3)})
	}
	return cs
}

// FuzzResolveChat: a chat name given to /win, /msg or /whois finds a chat
// only when the query matches it by id, alias or title, as typed or in
// either script, of the kind its sigil asks for; it finds the one chat
// that matches, says ambiguous only when several do, lets a unique exact
// id or alias win, then a unique exact title, then the one chat matched
// as typed (the either-script tiers never override it), and, when nothing
// matches as typed, a unique exact alias or title in either script; and
// it does not depend on the order of the chats.
func FuzzResolveChat(f *testing.F) {
	list := "c1\talice\tAlice Smith\nc2\tdev\tDev Talk\nc3\tnews\tНовини\nc9\tivan\tИван Петров\nc5\tdevops\tOps\nc6\tанна\tАнна"
	for _, q := range []string{"petrov", "@petrov", "#nov", "иван", "смит", "a", "dev", "#DEV", "c3", "\u0301", "@\u0301", "@", "#", "", "@Иван Петров", "ivan petrov", "novini"} {
		f.Add(list, q)
	}
	for _, s := range fuzzSeeds {
		f.Add(s+"\t"+s+"\t"+s+"\nx\t"+s, s)
	}
	f.Add("c1\tdevops\tOps\nc2\tb\tDev", "dev")            // an exact title before another alias's prefix
	f.Add("c1\tиван\tИван\nc2\tиванка\tИванка", "ivan")    // exact in either script before a prefix
	f.Add("a\tthe-ivan-show\tShow\nb\tиван\tИван", "ivan") // a plain substring before exact in either script
	f.Fuzz(func(t *testing.T, list, query string) {
		chats := fuzzChatList(list)
		q := strings.ToLower(strings.TrimSpace(query))
		pool, sigil := chats, byte(0)
		if q != "" && (q[0] == '@' || q[0] == '#') {
			pool, sigil = nil, q[0]
			for _, c := range chats {
				if (c.Kind == ChatUser) == (sigil == '@') {
					pool = append(pool, c)
				}
			}
			q = q[1:]
		}
		mt := NewMatcher(q)
		hit := func(c Chat) bool {
			if strings.ToLower(c.ID) == q || strings.Contains(strings.ToLower(c.Alias), q) || strings.Contains(strings.ToLower(c.Title), q) {
				return true
			}
			return !mt.Empty() && (mt.Match(c.Alias) || mt.Match(c.Title))
		}
		var hits, exact, titled, plain, either []Chat
		for _, c := range pool {
			if hit(c) {
				hits = append(hits, c)
			}
			if strings.ToLower(c.ID) == q || strings.ToLower(c.Alias) == q {
				exact = append(exact, c)
			}
			if strings.ToLower(strings.TrimSpace(c.Title)) == q {
				titled = append(titled, c)
			}
			if strings.ToLower(c.ID) == q || strings.Contains(strings.ToLower(c.Alias), q) || strings.Contains(strings.ToLower(c.Title), q) {
				plain = append(plain, c)
			}
			if !mt.Empty() && (mt.MatchExact(c.Alias) || mt.MatchExact(strings.TrimSpace(c.Title))) {
				either = append(either, c)
			}
		}
		got, err := ResolveChat(chats, query)
		switch {
		case q == "":
			if !errors.Is(err, ErrNoChat) {
				t.Fatalf("ResolveChat(%q): an empty query finds %+v, %v", query, got, err)
			}
			return
		case err == nil:
			if !hit(got) || sigil == '@' && got.Kind != ChatUser || sigil == '#' && got.Kind == ChatUser {
				t.Fatalf("ResolveChat(%q) = %+v, which does not match", query, got)
			}
		case errors.Is(err, ErrNoChat):
			if len(hits) > 0 {
				t.Fatalf("ResolveChat(%q): %v, yet %+v match", query, err, hits)
			}
		case errors.Is(err, ErrAmbiguous):
			if len(hits) < 2 {
				t.Fatalf("ResolveChat(%q): %v, yet only %+v match", query, err, hits)
			}
		default:
			t.Fatalf("ResolveChat(%q): unexpected error %v", query, err)
		}
		if len(hits) == 1 && (err != nil || got != hits[0]) {
			t.Fatalf("ResolveChat(%q) = %+v, %v; only %+v matches", query, got, err, hits[0])
		}
		if len(exact) == 1 && (err != nil || got != exact[0]) {
			t.Fatalf("ResolveChat(%q) = %+v, %v; the exact match is %+v", query, got, err, exact[0])
		}
		if len(exact) == 0 && len(titled) == 1 && (err != nil || got != titled[0]) {
			t.Fatalf("ResolveChat(%q) = %+v, %v; the exact title is %+v", query, got, err, titled[0])
		}
		if len(plain) == 1 && (err != nil || got != plain[0]) {
			t.Fatalf("ResolveChat(%q) = %+v, %v; the one match as typed is %+v", query, got, err, plain[0])
		}
		if len(plain) == 0 && len(either) == 1 && (err != nil || got != either[0]) {
			t.Fatalf("ResolveChat(%q) = %+v, %v; the exact match in either script is %+v", query, got, err, either[0])
		}
		rev := make([]Chat, len(chats))
		for i, c := range chats {
			rev[len(chats)-1-i] = c
		}
		got2, err2 := ResolveChat(rev, query)
		if got2 != got || errClass(err2) != errClass(err) {
			t.Fatalf("ResolveChat(%q) depends on the order: %+v, %v reversed %+v, %v", query, got, err, got2, err2)
		}
	})
}

var fuzzStart = time.Date(2026, 1, 2, 3, 0, 0, 0, time.UTC)

// fuzzLog reads one message per line, "sender\tid\tnick\ttext", into chat
// g a minute apart; a line starting with ~ is in chat h. A sender without
// an id has no nick, as the connectors give them.
func fuzzLog(log string) []Message {
	var msgs []Message
	for i, line := range strings.Split(log, "\n") {
		if i == 64 {
			break
		}
		chat := "g"
		if rest, ok := strings.CutPrefix(line, "~"); ok {
			chat, line = "h", rest
		}
		f := strings.SplitN(line, "\t", 4)
		for len(f) < 4 {
			f = append(f, "")
		}
		m := Message{ID: strconv.Itoa(i), ChatID: chat, Sender: f[0], SenderID: f[1], SenderNick: f[2], Text: f[3],
			Time: fuzzStart.Add(time.Duration(i) * time.Minute)}
		if m.SenderID == "" {
			m.SenderNick = ""
		}
		msgs = append(msgs, m)
	}
	return msgs
}

// fuzzParticipants reads one listed person per line, "id\tname\tnick", as
// the service gives them: an id, ids unique, usernames unique ignoring
// case.
func fuzzParticipants(list string) []Participant {
	var ps []Participant
	ids, nicks := map[string]bool{}, map[string]bool{}
	for i, line := range strings.Split(list, "\n") {
		if i == 32 {
			break
		}
		f := strings.SplitN(line, "\t", 3)
		for len(f) < 3 {
			f = append(f, "")
		}
		p := Participant{ID: f[0], Name: f[1], Nick: f[2]}
		if p.ID == "" || ids[p.ID] {
			continue
		}
		ids[p.ID] = true
		if k := strings.ToLower(p.Nick); nicks[k] {
			p.Nick = ""
		} else if k != "" {
			nicks[k] = true
		}
		ps = append(ps, p)
	}
	return ps
}

const fuzzLogSeed = "Al\tu1\t\tfirst\n" +
	"Alice\tu1\talice\thappy birthday @Grace_H and @ALICE\n" +
	"Alice\t\t\tcached without an id\n" +
	"Bob\t\t\tno id at all, mail@example.com\n" +
	"me\t\t\t@grace_h, @grace_h!\n" +
	"channel\t\t\tunsigned post www.example.com/(@path_one)\n" +
	"~Bob\tu9\tbobby\t@elsewhere in another chat\n" +
	"Sam\tu5\tRobin_X\thi\n" +
	"Sam\tu6\trobin_x\they @robin_x \u00a0@nbsp_one\n" +
	"Иван\tu7\t\tИва\u0301н @ivan_p\n" +
	"\x1b[31mEve\x1b[0m\tu8\t\u0085evil\t\xff@bad_utf"

// FuzzMembers: whoever writes and whatever they write, every countable
// message counts once, every mention once, each member has a name or a
// nick, a nick belongs to one member, names, nicks and ids come from the
// messages, a member who never wrote is a nick only mentioned, the most
// recently active come first, and the order of the messages does not
// matter.
func FuzzMembers(f *testing.F) {
	f.Add(fuzzLogSeed)
	for _, s := range fuzzSeeds {
		f.Add(s + "\tu1\t" + s + "\t" + s + "\n" + s + "\t\t\t@" + s + " @user_two")
	}
	f.Fuzz(func(t *testing.T, log string) {
		msgs := fuzzLog(log)
		ms := Members(msgs, "g")
		checkMemberList(t, ms)
		counted, mentions := 0, 0
		names, nicks, ids := map[string]bool{}, map[string]bool{}, map[string]bool{}
		for _, m := range msgs {
			if m.ChatID != "g" {
				continue
			}
			for _, n := range MentionsIn(m.Text) {
				nicks[n] = true
				mentions++
			}
			if m.Sender == "" || m.SenderID == "" && (m.Sender == "me" || m.Sender == unsignedPost) {
				continue
			}
			counted++
			names[m.Sender], ids[m.SenderID], nicks[m.SenderNick] = true, true, true
		}
		if c, n := memberSums(ms); c != counted || n != mentions {
			t.Fatalf("%d messages and %d mentions counted as %d and %d: %+v", counted, mentions, c, n, ms)
		}
		for _, mb := range ms {
			if mb.Name != "" && !names[mb.Name] || mb.Nick != "" && !nicks[mb.Nick] || mb.ID != "" && !ids[mb.ID] {
				t.Fatalf("member not from the messages: %+v", mb)
			}
			if (mb.Count == 0) != (mb.Name == "") || mb.Count == 0 && (mb.ID != "" || mb.Mentions == 0) {
				t.Fatalf("a member who never wrote must be a nick only mentioned: %+v", mb)
			}
		}
		rev := make([]Message, len(msgs))
		for i, m := range msgs {
			rev[len(msgs)-1-i] = m
		}
		if again := Members(rev, "g"); !sameMembers(again, ms) {
			t.Fatalf("the order of the messages changed the members:\n%+v\n%+v", ms, again)
		}
	})
}

// FuzzWithParticipants: merging a member list from the service leaves the
// members it was given alone, keeps every message and mention counted,
// makes each listed person one member with their id, name and username,
// changes nobody else but to take away a username a listed person holds,
// keeps nicks unique and the order, and a second merge of the same list
// changes nothing.
func FuzzWithParticipants(f *testing.F) {
	f.Add(fuzzLogSeed, "u1\tAlice A.\talice_new\nu2\tBob\tbob_b\nu4\tDave\tgrace_h\nu6\tFrank\tROBIN_X\nu7\tGina\t\nu8\t\t")
	f.Add("Sam\t\t\thi\nCarol\tu3\t\tthanks @bob_b @sam_two", "u1\tSam\tsam_one\nu2\tSam\tsam_two\nu9\tBob\tbob_b")
	// Senders not on the list: Carol keeps her username, Dan loses his to
	// the listed Daniel, who holds it in other case.
	f.Add("Carol\tu3\tcarol_c\thi @dave_d\nDan\tu4\tdan_d\thello", "u9\tDaniel\tDAN_D")
	// The same with the case on the unlisted side.
	f.Add("Dan\tu4\tDan_D\thello", "u9\tDaniel\tdan_d")
	for _, s := range fuzzSeeds {
		f.Add(s+"\tu1\t"+s+"\t@"+s+"\n"+s+"\t\t\t"+s, "u1\t"+s+"\t"+s+"\nu2\t"+s+"\tx_"+s)
	}
	f.Fuzz(func(t *testing.T, log, list string) {
		ms := Members(fuzzLog(log), "g")
		ps := fuzzParticipants(list)
		before := append(make([]Member, 0, len(ms)), ms...)
		out := WithParticipants(ms, ps)
		if !reflect.DeepEqual(ms, before) {
			t.Fatalf("the members given changed:\n%+v\n%+v", before, ms)
		}
		checkMemberList(t, out)
		c0, n0 := memberSums(ms)
		if c, n := memberSums(out); c != c0 || n != n0 {
			t.Fatalf("%d messages and %d mentions became %d and %d:\n%+v\n%+v", c0, n0, c, n, ms, out)
		}
		// A listed person with neither name nor username is a member only
		// when their messages give them a name.
		onList, held := map[string]bool{}, map[string]bool{}
		for _, p := range ps {
			onList[p.ID] = true
			if p.Nick != "" {
				held[strings.ToLower(p.Nick)] = true
			}
			var mine []Member
			for _, mb := range out {
				if mb.ID == p.ID {
					mine = append(mine, mb)
				}
			}
			if p.Name == "" && p.Nick == "" && len(mine) == 0 {
				continue
			}
			if len(mine) != 1 || !mine[0].Listed || mine[0].Nick != p.Nick || p.Name != "" && mine[0].Name != p.Name {
				t.Fatalf("listed %+v is %+v in %+v", p, mine, out)
			}
		}
		for _, mb := range out {
			if mb.Listed != onList[mb.ID] {
				t.Fatalf("%+v: listed %v, on the list %v: %+v", mb, mb.Listed, onList[mb.ID], ps)
			}
			if mb.Listed {
				continue
			}
			// Unchanged, but that the username goes when a listed person
			// holds it now, ignoring case: theirs is the current one.
			found := false
			for _, old := range ms {
				was := old
				if mb.Nick == "" && held[strings.ToLower(old.Nick)] {
					was.Nick = ""
				}
				found = found || reflect.DeepEqual(was, mb)
			}
			if !found {
				t.Fatalf("%+v is not listed and was no member before: %+v", mb, ms)
			}
		}
		if again := WithParticipants(out, ps); !sameMembers(again, out) {
			t.Fatalf("a second merge changed the members:\n%+v\n%+v", out, again)
		}
	})
}

// checkMemberList checks what every member list keeps: each member has a
// name or a nick, a nick belongs to one member (ignoring case), and the
// most recently active come first, then by name and nick.
func checkMemberList(t *testing.T, ms []Member) {
	t.Helper()
	held := map[string]bool{}
	for i, mb := range ms {
		if mb.Name == "" && mb.Nick == "" {
			t.Fatalf("member %d has neither name nor nick: %+v", i, ms)
		}
		if mb.Nick != "" {
			k := strings.ToLower(mb.Nick)
			if held[k] {
				t.Fatalf("nick %q held twice: %+v", mb.Nick, ms)
			}
			held[k] = true
		}
		if i > 0 {
			a := ms[i-1]
			if a.Last.Before(mb.Last) || a.Last.Equal(mb.Last) && (a.Name > mb.Name || a.Name == mb.Name && a.Nick > mb.Nick) {
				t.Fatalf("out of order at %d: %+v", i, ms)
			}
		}
	}
}

func memberSums(ms []Member) (count, mentions int) {
	for _, mb := range ms {
		count += mb.Count
		mentions += mb.Mentions
	}
	return count, mentions
}

// sameMembers compares two member lists as sets: members that tie on
// time, name and nick may come in either order.
func sameMembers(a, b []Member) bool {
	canon := func(ms []Member) []Member {
		c := append([]Member{}, ms...)
		sort.Slice(c, func(i, j int) bool {
			x, y := c[i], c[j]
			switch {
			case x.ID != y.ID:
				return x.ID < y.ID
			case x.Name != y.Name:
				return x.Name < y.Name
			case x.Nick != y.Nick:
				return x.Nick < y.Nick
			case !x.Last.Equal(y.Last):
				return x.Last.Before(y.Last)
			case x.Count != y.Count:
				return x.Count < y.Count
			}
			return x.Mentions < y.Mentions
		})
		return c
	}
	return reflect.DeepEqual(canon(a), canon(b))
}

// FuzzIsImageName: a file name another user sent is an image exactly when
// it ends in one of the raster extensions, ignoring case, whatever
// directory it names.
func FuzzIsImageName(f *testing.F) {
	for _, s := range append(fuzzSeeds, "a.JPG", "dir/b.png", "../a.gif", "x.tar.gz", "a.png/", "a\\b.webp", ".jpeg", "a.G\u0130F", "a.p\xffng") {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, name string) {
		got := IsImageName(name)
		want := false
		for _, ext := range []string{".jpg", ".jpeg", ".png", ".gif", ".webp"} {
			want = want || strings.HasSuffix(strings.ToLower(name), ext)
		}
		if got != want {
			t.Fatalf("IsImageName(%q) = %v, want %v", name, got, want)
		}
		if IsImageName("dir/"+name) != got || IsImageName(`dir\`+name) != got {
			t.Fatalf("IsImageName(%q) = %v, but not in a directory", name, got)
		}
	})
}
