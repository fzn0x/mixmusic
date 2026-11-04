package romanize

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Greek is written by ELOT 743, as the UN adopted it in 1987 for
// transcription: its stress marks stay, as acutes; ου is ou; αυ, ευ and
// ηυ are av, ev and iv before a vowel and a voiced consonant, and af, ef
// and if before a voiceless one and at the end; μπ is b at the ends of a
// word and mp within it; γγ is ng, γξ nx and γχ nch; ντ and γκ are nt and
// gk. A diaeresis, or a stress on the first of two vowels, parts them.

// greekLetters are the letters of Greek, lowercase and without their
// marks.
var greekLetters = map[rune]string{
	'α': "a", 'β': "v", 'γ': "g", 'δ': "d", 'ε': "e", 'ζ': "z", 'η': "i", 'θ': "th", 'ι': "i",
	'κ': "k", 'λ': "l", 'μ': "m", 'ν': "n", 'ξ': "x", 'ο': "o", 'π': "p", 'ρ': "r", 'σ': "s",
	'ς': "s", 'τ': "t", 'υ': "y", 'φ': "f", 'χ': "ch", 'ψ': "ps", 'ω': "o", 'ϲ': "s", 'ϳ': "j",
}

// greekVoiceless are the consonants before which αυ, ευ and ηυ are af,
// ef and if.
const greekVoiceless = "θκξπστφχψς"

// greekLetter is a letter of Greek as greekRun reads it: lowercase,
// without its marks, which it tells apart.
type greekLetter struct {
	base           rune
	upper, stress  bool
	diaeresis      bool
	letter, length int
}

func init() { register(greek, greekRun, "el") }

// greekRun romanizes a run of Greek.
func greekRun(run, _ string) []piece {
	var letters []greekLetter
	for i, r := range run {
		l := greekLetter{base: r, upper: unicode.IsUpper(r), letter: i, length: len(string(r))}
		for _, d := range norm.NFD.String(string(r)) {
			switch d {
			case 0x0301, 0x0300, 0x0342, 0x0344:
				l.stress = true
			case 0x0308:
				l.diaeresis = true
			default:
				if unicode.IsLetter(d) {
					l.base = unicode.ToLower(d)
				}
			}
		}
		if len(letters) > 0 && !unicode.IsLetter(l.base) && unicode.IsMark(l.base) {
			// A mark apart from its letter, as a combining acute.
			last := &letters[len(letters)-1]
			last.length += l.length
			last.stress = last.stress || r == 0x0301 || r == 0x0300
			last.diaeresis = last.diaeresis || r == 0x0308
			continue
		}
		letters = append(letters, l)
	}
	var pieces []piece
	at := func(i int) rune {
		if i < 0 || i >= len(letters) {
			return 0
		}
		return letters[i].base
	}
	for i := 0; i < len(letters); i++ {
		l, next := letters[i], at(i+1)
		start := i == 0 || !unicode.IsLetter(at(i-1))
		end := i+2 >= len(letters) || !unicode.IsLetter(at(i+2))
		two := i+1 < len(letters) && !letters[i+1].diaeresis
		latin, n := "", 1
		switch {
		case l.base == 'ο' && next == 'υ' && two && !l.stress:
			latin, n = "ou", 2
		case strings.ContainsRune("αεη", l.base) && next == 'υ' && two && !l.stress:
			v := greekLetters[l.base] + "v"
			if after := at(i + 2); after == 0 || !unicode.IsLetter(after) || strings.ContainsRune(greekVoiceless, after) {
				v = greekLetters[l.base] + "f"
			}
			latin, n = v, 2
		case l.base == 'μ' && next == 'π':
			latin, n = "mp", 2
			if start || end {
				latin = "b"
			}
		case l.base == 'γ' && next == 'γ':
			latin, n = "ng", 2
		case l.base == 'γ' && next == 'ξ':
			latin, n = "nx", 2
		case l.base == 'γ' && next == 'χ':
			latin, n = "nch", 2
		default:
			var ok bool
			if latin, ok = greekLetters[l.base]; !ok {
				latin = plain(l.base)
			}
		}
		// The stress mark goes on the vowel that carries it, the second of
		// a pair read as one, as in ού, oú.
		stressed := l.stress || n == 2 && letters[i+1].stress
		if stressed {
			latin = stress(latin, l.base == 'ο' && n == 2)
		}
		if l.diaeresis && latin == "i" {
			latin = "ï"
		} else if l.diaeresis && latin == "y" {
			latin = "ÿ"
		}
		if l.upper {
			var after rune
			if i+n < len(letters) && letters[i+n].upper {
				after = 'A'
			}
			if i+n < len(letters) && !letters[i+n].upper {
				after = 'a'
			}
			latin = capital(latin, after)
		}
		last := letters[i+n-1]
		pieces = append(pieces, piece{last.letter + last.length, latin})
		i += n - 1
	}
	return pieces
}

// stress puts an acute on the vowel of latin that carries the stress: the
// first, or the second of ou.
func stress(latin string, second bool) string {
	acute := map[rune]string{'a': "á", 'e': "é", 'i': "í", 'o': "ó", 'u': "ú", 'y': "ý"}
	runes := []rune(latin)
	seen := 0
	for i, r := range runes {
		if a, ok := acute[r]; ok {
			if second && seen == 0 {
				seen++
				continue
			}
			return string(runes[:i]) + a + string(runes[i+1:])
		}
	}
	return latin
}
