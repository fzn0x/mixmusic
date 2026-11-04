package romanize

import (
	"strings"

	"golang.org/x/text/unicode/norm"
)

// Hebrew is written by the rules of the Academy of the Hebrew Language
// (2006), which the UN adopted in 2007, without their marks: ח is h, as
// ה, and צ ts; a dagesh makes ב כ פ b, k and p, else v, kh and f; and as
// Hebrew is said now, a dagesh doesn't double a letter, nor is a shva
// said but at the start of a word on a sonorant or before a guttural.
// Their vowel points are read where they are written; lyrics mostly have
// none, and then their consonants are written with the long vowels their
// letters write, ו as o and י as i, as "שלום" is "shlom": the short
// vowels aren't written in Hebrew, and only a dictionary knows them.
// Yiddish, "yi", is written by YIVO's rules, whose letters write all
// their vowels.

// Points of Hebrew.
const (
	hShva, hHatafSegol, hHatafPatah, hHatafQamats = 0x05B0, 0x05B1, 0x05B2, 0x05B3
	hHiriq, hTsere, hSegol, hPatah, hQamats       = 0x05B4, 0x05B5, 0x05B6, 0x05B7, 0x05B8
	hHolam, hHolamVav, hQubuts, hDagesh           = 0x05B9, 0x05BA, 0x05BB, 0x05BC
	hShinDot, hSinDot, hQamatsQatan, hMaqaf       = 0x05C1, 0x05C2, 0x05C7, 0x05BE
	hGeresh                                       = 0x05F3
)

// hebrewVowels are the vowels of the points.
var hebrewVowels = map[rune]string{
	hHatafSegol: "e", hHatafPatah: "a", hHatafQamats: "o", hHiriq: "i", hTsere: "e", hSegol: "e",
	hPatah: "a", hQamats: "a", hHolam: "o", hHolamVav: "o", hQubuts: "u", hQamatsQatan: "o",
}

// hebrewLetters are the consonants, without a dagesh.
var hebrewLetters = map[rune]string{
	'א': "", 'ב': "v", 'ג': "g", 'ד': "d", 'ה': "h", 'ו': "v", 'ז': "z", 'ח': "h", 'ט': "t", 'י': "y",
	'כ': "kh", 'ך': "kh", 'ל': "l", 'מ': "m", 'ם': "m", 'נ': "n", 'ן': "n", 'ס': "s", 'ע': "", 'פ': "f",
	'ף': "f", 'צ': "ts", 'ץ': "ts", 'ק': "k", 'ר': "r", 'ש': "sh", 'ת': "t",
}

// hebrewLetter is a letter of Hebrew with its points.
type hebrewLetter struct {
	r           rune
	dagesh, sin bool
	shin, rafe  bool
	vowel       rune
	geresh      bool
	end         int
}

func init() { register(hebrew, hebrewRun, "he yi") }

// hebrewRun romanizes a run of Hebrew, word by word as maqafs part them
// too.
func hebrewRun(run, lang string) []piece {
	var letters []hebrewLetter
	pointed := false
	var pieces []piece
	for i, r0 := range run {
		end := i + len(string(r0))
		for _, r := range norm.NFD.String(string(r0)) {
			var last *hebrewLetter
			if len(letters) > 0 {
				last = &letters[len(letters)-1]
			}
			switch {
			case r >= 0x05D0 && r <= 0x05EA || r >= 0x05F0 && r <= 0x05F2:
				letters = append(letters, hebrewLetter{r: r, end: end})
			case last != nil && hebrewVowels[r] != "" || last != nil && r == hShva:
				if last.vowel == 0 || r != hShva {
					last.vowel = r
				}
				pointed = true
				last.end = end
			case last != nil && r == hDagesh:
				last.dagesh, last.end = true, end
			case last != nil && r == hShinDot:
				last.shin, last.end = true, end
			case last != nil && r == hSinDot:
				last.sin, last.end = true, end
			case last != nil && r == 0x05BF:
				last.rafe, last.end = true, end
			case last != nil && (r == hGeresh || r == '\'' || r == '’'):
				last.geresh, last.end = true, end
			case r >= 0x0591 && r <= 0x05C7 && last != nil && r != hMaqaf:
				// Cantillation and other marks.
				last.end = end
			default:
				pieces = append(pieces, hebrewWord(letters, pointed, lang)...)
				letters = nil
				p := plain(r)
				if r == hMaqaf {
					p = "-"
				}
				pieces = append(pieces, piece{end, p})
			}
		}
	}
	return append(pieces, hebrewWord(letters, pointed, lang)...)
}

// hebrewWord writes the letters of a word.
func hebrewWord(w []hebrewLetter, pointed bool, lang string) []piece {
	if lang == "yi" {
		return yiddishWord(w)
	}
	pieces := make([]piece, 0, len(w))
	for i, l := range w {
		pieces = append(pieces, piece{l.end, hebrewSyllable(w, i, pointed)})
	}
	return pieces
}

// hebrewSyllable writes letter i of a word with its vowel.
func hebrewSyllable(w []hebrewLetter, i int, pointed bool) string {
	l := w[i]
	first, last := i == 0, i == len(w)-1
	var prev, next hebrewLetter
	if i > 0 {
		prev = w[i-1]
	}
	if !last {
		next = w[i+1]
	}
	c := hebrewLetters[l.r]
	switch {
	case l.r == 'ב' && (l.dagesh || !pointed && first):
		c = "b"
	case (l.r == 'כ' || l.r == 'ך') && (l.dagesh || !pointed && first):
		c = "k"
	case (l.r == 'פ' || l.r == 'ף') && (l.dagesh || !pointed && first):
		c = "p"
	case l.r == 'ש' && l.sin:
		c = "s"
	case l.geresh:
		if g, ok := map[rune]string{'ג': "j", 'ז': "zh", 'צ': "ch", 'ץ': "ch", 'ת': "th"}[l.r]; ok {
			c = g
		}
	}
	if !pointed {
		switch l.r {
		case 'ו':
			switch {
			case first || next.r == 'ו' || prev.r == 'ו':
				// A vav at the start, or doubled, is v.
				if prev.r == 'ו' {
					return ""
				}
				return "v"
			}
			return "o"
		case 'י':
			switch {
			case first || next.r == 'י' || prev.r == 'י':
				if prev.r == 'י' {
					return ""
				}
				return "y"
			case next.r == 'ו' || next.r == 'א':
				return "y"
			}
			return "i"
		case 'א', 'ע':
			// Before a consonant, they mostly carry an a; at the end, ע
			// does too.
			if !last && next.r != 'ו' && next.r != 'י' || last && l.r == 'ע' && !first {
				return "a"
			}
			return ""
		case 'ה':
			if last && !first {
				return "a"
			}
		}
		return c
	}
	// Pointed.
	v := hebrewVowels[l.vowel]
	switch l.r {
	case 'ו':
		switch {
		case l.dagesh && l.vowel == 0 && !first:
			return "u"
		case (l.vowel == hHolam || l.vowel == hHolamVav) && !first:
			return "o"
		case l.dagesh && l.vowel == 0:
			return "u"
		}
	case 'י':
		if l.vowel == 0 && !first && (prev.vowel == hHiriq || prev.vowel == hTsere || prev.vowel == hSegol) {
			// The yod of i and e.
			return ""
		}
		if l.vowel == 0 && next.r == 'ו' && last == false && i+2 == len(w) && prev.vowel == hQamats {
			// ָיו, av.
			return ""
		}
	case 'ה':
		if last && !l.dagesh && l.vowel == 0 {
			return ""
		}
	case 'א', 'ע':
		if v != "" && i > 0 && hebrewVowels[prev.vowel] != "" && !first {
			return "'" + v
		}
	}
	if l.vowel == hShva {
		v = ""
		// A shva at the start of a word is said on a sonorant, and
		// before a guttural.
		if first && (strings.ContainsRune("ילמנר", l.r) || strings.ContainsRune("אהחע", next.r)) {
			v = "e"
		}
	}
	if last && l.vowel == hPatah && (l.r == 'ח' || l.r == 'ע' || l.r == 'ה' && l.dagesh) {
		// The furtive patah is said before its letter: רוּחַ, ruah.
		return v + c
	}
	return c + v
}

// yiddishLetters are the letters of Yiddish by YIVO's rules, without their
// points.
var yiddishLetters = map[rune]string{
	'א': "", 'ב': "b", 'ג': "g", 'ד': "d", 'ה': "h", 'ו': "u", 'ז': "z", 'ח': "kh", 'ט': "t", 'י': "i",
	'כ': "kh", 'ך': "kh", 'ל': "l", 'מ': "m", 'ם': "m", 'נ': "n", 'ן': "n", 'ס': "s", 'ע': "e", 'פ': "p",
	'ף': "f", 'צ': "ts", 'ץ': "ts", 'ק': "k", 'ר': "r", 'ש': "sh", 'ת': "s", 'װ': "v", 'ױ': "oy", 'ײ': "ey",
}

// yiddishWord writes a word of Yiddish: its vowels are letters, אַ a and
// אָ o, ע e, ו u, י i, ײ ey and ײַ ay; a rafe makes בֿ v and פֿ f, and a
// dagesh כּ k and תּ t.
func yiddishWord(w []hebrewLetter) []piece {
	pieces := make([]piece, 0, len(w))
	vowelLetter := func(l hebrewLetter) bool {
		return l.r == 'א' && l.vowel != 0 || strings.ContainsRune("עוײױ", l.r) || l.r == 'י' && l.vowel == hHiriq
	}
	for i, l := range w {
		c := yiddishLetters[l.r]
		switch {
		case l.r == 'א' && l.vowel == hPatah:
			c = "a"
		case l.r == 'א' && l.vowel == hQamats:
			c = "o"
		case l.r == 'ײ' && l.vowel == hPatah:
			c = "ay"
		case l.r == 'כ' && l.dagesh:
			c = "k"
		case l.r == 'ת' && l.dagesh:
			c = "t"
		case l.r == 'ב' && l.rafe:
			c = "v"
		case l.r == 'פ' && l.rafe:
			c = "f"
		case l.r == 'ש' && l.sin:
			c = "s"
		case l.r == 'ש' && i > 0 && w[i-1].r == 'ז':
			// זש is zh.
			c = "h"
		case l.r == 'י' && l.vowel != hHiriq && i+1 < len(w) && vowelLetter(w[i+1]):
			c = "y"
		}
		pieces = append(pieces, piece{l.end, c})
	}
	return pieces
}
