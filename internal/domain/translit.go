package domain

import (
	"bytes"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Searching Cyrillic text with a Latin keyboard: a query matches a text
// when the text contains it after both are lower-cased, or after both are
// transliterated to Latin. Cyrillic letters map by the Bulgarian
// streamlined system (a b v g d e zh z i y k l m n o p r s t u f h ts ch
// sh sht a y yu ya; the stressed i as i), extended with the Russian and
// Ukrainian letters that system lacks: io as yo, the hard i (yery) as y,
// the reversed e as e, the Ukrainian ie as ye, the dotted i as i, yi as
// yi and ghe with upturn as g; shcha stays sht as in Bulgarian. Combining
// marks (stress accents) are dropped. Latin text is left as it is, so a
// Latin query finds a name written in Cyrillic and a Cyrillic query finds
// the same name written in Latin.
//
// The transliteration maps rune by rune, so a plain match is always also
// a transliterated match: comparing the transliterations alone is the
// whole rule, and that is what Matcher does.

var cyrillic = map[rune]string{
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ж': "zh", 'з': "z",
	'и': "i", 'й': "y", 'к': "k", 'л': "l", 'м': "m", 'н': "n", 'о': "o", 'п': "p",
	'р': "r", 'с': "s", 'т': "t", 'у': "u", 'ф': "f", 'х': "h", 'ц': "ts", 'ч': "ch",
	'ш': "sh", 'щ': "sht", 'ъ': "a", 'ь': "y", 'ю': "yu", 'я': "ya", 'ѝ': "i",
	// Russian and Ukrainian letters outside the Bulgarian alphabet.
	'ё': "yo", 'ы': "y", 'э': "e", 'є': "ye", 'і': "i", 'ї': "yi", 'ґ': "g",
}

// latinOf is cyrillic as a table over the Cyrillic block (U+0400..U+04FF),
// where every mapped letter lives; "" means not mapped.
var latinOf [0x100]string

func init() {
	for r, s := range cyrillic {
		latinOf[r-0x400] = s
	}
}

// appendTranslit appends the lower-cased transliteration of s to dst:
// Cyrillic letters in Latin, combining marks (U+0300..U+036F) dropped,
// everything else lower-cased and kept.
func appendTranslit(dst []byte, s string) []byte {
	for i := 0; i < len(s); {
		if c := s[i]; c < utf8.RuneSelf {
			if 'A' <= c && c <= 'Z' {
				c += 'a' - 'A'
			}
			dst = append(dst, c)
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		r = unicode.ToLower(r)
		switch {
		case r >= 0x400 && r <= 0x4ff && latinOf[r-0x400] != "":
			dst = append(dst, latinOf[r-0x400]...)
		case r >= 0x300 && r <= 0x36f:
		default:
			dst = utf8.AppendRune(dst, r)
		}
	}
	return dst
}

// Translit lower-cases s, writes its Cyrillic letters in Latin and drops
// combining marks; other characters pass through unchanged.
func Translit(s string) string { return string(appendTranslit(make([]byte, 0, len(s)), s)) }

// Matcher matches one query against many texts in either script. The
// query is transliterated once and every text is transliterated into a
// buffer the matcher reuses, so a search allocates nothing per text. Not
// safe for concurrent use.
type Matcher struct {
	latin []byte  // the query transliterated; empty matches everything
	buf   *[]byte // scratch for the text being matched
}

// NewMatcher prepares a query; surrounding space is ignored.
func NewMatcher(query string) Matcher {
	buf := make([]byte, 0, 256)
	return Matcher{latin: appendTranslit(nil, strings.TrimSpace(query)), buf: &buf}
}

// Empty reports whether the query is empty, which matches everything.
func (mt Matcher) Empty() bool { return len(mt.latin) == 0 }

// Match reports whether hay contains the query, case-insensitively, as
// typed or with both transliterated to Latin.
func (mt Matcher) Match(hay string) bool {
	if len(mt.latin) == 0 {
		return true
	}
	*mt.buf = appendTranslit((*mt.buf)[:0], hay)
	return bytes.Contains(*mt.buf, mt.latin)
}

// MatchText reports whether needle occurs in hay, case-insensitively, in
// either script. An empty needle matches everything. Prefer a Matcher
// when one query meets many texts.
func MatchText(hay, needle string) bool { return NewMatcher(needle).Match(hay) }
