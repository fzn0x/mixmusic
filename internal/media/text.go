package media

import (
	"cmp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Fold makes text compare as a reader reads it: lowercase, and without
// its accents.
func Fold(s string) string {
	var b strings.Builder
	for _, r := range norm.NFKD.String(s) {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// CompareText compares texts as the web interface's collator does, more
// or less: regardless of case and accents, and their numbers by value, so
// that "Track 2" comes before "Track 10".
func CompareText(a, b string) int {
	a, b = Fold(a), Fold(b)
	for a != "" && b != "" {
		ra, na := utf8.DecodeRuneInString(a)
		rb, nb := utf8.DecodeRuneInString(b)
		if isDigit(ra) && isDigit(rb) {
			da, db := leadingDigits(a), leadingDigits(b)
			ta, tb := strings.TrimLeft(da, "0"), strings.TrimLeft(db, "0")
			if c := cmp.Or(cmp.Compare(len(ta), len(tb)), strings.Compare(ta, tb)); c != 0 {
				return c
			}
			a, b = a[len(da):], b[len(db):]
			continue
		}
		if ra != rb {
			return cmp.Compare(ra, rb)
		}
		a, b = a[na:], b[nb:]
	}
	return cmp.Compare(len(a), len(b))
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }

func leadingDigits(s string) string {
	i := 0
	for i < len(s) && isDigit(rune(s[i])) {
		i++
	}
	return s[:i]
}
