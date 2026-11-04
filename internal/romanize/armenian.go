package romanize

import (
	"strings"
	"unicode"
)

// Armenian is written by BGN/PCGN (1981): Armenia has no system of its
// own in force, the UN's group on romanization found, and this one is the
// commonest. Its aspirates take an apostrophe, as թ t’; ե is ye and ո is
// vo at the start of a word, ե after a vowel is ye too, ու is u, and և is
// yev or ev.

var armenianLetters = map[rune]string{
	'ա': "a", 'բ': "b", 'գ': "g", 'դ': "d", 'ե': "e", 'զ': "z", 'է': "e", 'ը': "y", 'թ': "t’",
	'ժ': "zh", 'ի': "i", 'լ': "l", 'խ': "kh", 'ծ': "ts", 'կ': "k", 'հ': "h", 'ձ': "dz", 'ղ': "gh",
	'ճ': "ch", 'մ': "m", 'յ': "y", 'ն': "n", 'շ': "sh", 'ո': "o", 'չ': "ch’", 'պ': "p", 'ջ': "j",
	'ռ': "rr", 'ս': "s", 'վ': "v", 'տ': "t", 'ր': "r", 'ց': "ts’", 'ւ': "w", 'փ': "p’", 'ք': "k’",
	'օ': "o", 'ֆ': "f", 'և': "ev",
}

// armenianVowels are the letters after which ե and և start with y.
const armenianVowels = "աեէըիոօ"

func init() { register(armenian, armenianRun, "hy") }

// armenianRun romanizes a run of Armenian.
func armenianRun(run, _ string) []piece {
	runes := []rune(run)
	var pieces []piece
	at := 0
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		lower := unicode.ToLower(r)
		var prev, next rune
		if i > 0 {
			prev = unicode.ToLower(runes[i-1])
		}
		if i+1 < len(runes) {
			next = unicode.ToLower(runes[i+1])
		}
		start := i == 0 || !unicode.IsLetter(prev)
		latin, ok := armenianLetters[lower]
		if !ok {
			latin = plain(r)
		}
		n := 1
		switch {
		case lower == 'ո' && next == 'ւ':
			latin, n = "u", 2
		case lower == 'ո' && start:
			latin = "vo"
			// But ով, who, is ov.
			if next == 'վ' && (i+2 == len(runes) || !unicode.IsLetter(runes[i+2])) {
				latin = "o"
			}
		case (lower == 'ե' || lower == 'և') && (start || strings.ContainsRune(armenianVowels, prev) || prev == 'ւ' && i > 1 && unicode.ToLower(runes[i-2]) == 'ո'):
			latin = "y" + latin
		}
		if unicode.IsUpper(r) {
			var after rune
			if i+n < len(runes) {
				after = runes[i+n]
			}
			latin = capital(latin, after)
		}
		size := 0
		for _, c := range runes[i : i+n] {
			size += len(string(c))
		}
		at += size
		pieces = append(pieces, piece{at, latin})
		i += n - 1
	}
	return pieces
}
