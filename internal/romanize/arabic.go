package romanize

import (
	"strings"
	"unicode"
)

// Arabic is written by the romanization of ALA-LC (the Library of
// Congress), in the plain form English writes it, without its dots and
// macrons: ح is h, as ه, ع and ء an apostrophe within a word; and the
// article al- takes the sound of a sun letter after it, as said:
// "الشمس" is "ash-shms". Persian is written by the same letters as the
// UN's system of 2012 reads them (و v, ق gh, ذ ض ظ z, its short vowels a, e
// and o, and ā for its long a, which sounds apart), and Urdu with its
// own letters (ٹ t, ڈ d, ڑ r, ں n, ے e, ھ aspirating the letter before).
// Their vowel points are read where they are written; lyrics mostly have
// none, and then the consonants are written with the long vowels their
// letters write, ا a, و u and ي i, as "حبيبي" is "hbibi": the short ones
// aren't written, and only a dictionary knows them.

// arabicLetters are the consonants of Arabic, Persian and Urdu.
var arabicLetters = map[rune]string{
	'ء': "'", 'آ': "a", 'أ': "'", 'ؤ': "'", 'إ': "'", 'ئ': "'", 'ا': "a", 'ب': "b", 'ة': "a", 'ت': "t",
	'ث': "th", 'ج': "j", 'ح': "h", 'خ': "kh", 'د': "d", 'ذ': "dh", 'ر': "r", 'ز': "z", 'س': "s", 'ش': "sh",
	'ص': "s", 'ض': "d", 'ط': "t", 'ظ': "z", 'ع': "'", 'غ': "gh", 'ف': "f", 'ق': "q", 'ك': "k", 'ل': "l",
	'م': "m", 'ن': "n", 'ه': "h", 'و': "w", 'ى': "a", 'ي': "y", 'ٱ': "", 'پ': "p", 'چ': "ch", 'ژ': "zh",
	'ڤ': "v", 'گ': "g", 'ک': "k", 'ی': "y", 'ٹ': "t", 'ڈ': "d", 'ڑ': "r", 'ں': "n", 'ے': "e", 'ۓ': "e",
	'ہ': "h", 'ھ': "h", 'ۃ': "a", 'ۀ': "e", 'ڭ': "ng", 'ۆ': "o", 'ێ': "ê",
}

// persianLetters are the letters Persian reads otherwise than Arabic.
var persianLetters = map[rune]string{'ث': "s", 'ذ': "z", 'ض': "z", 'ظ': "z", 'ق': "gh", 'و': "v", 'ة': "e", 'ص': "s", 'آ': "ā", 'ا': "ā"}

// sunLetters are those the article al- takes the sound of.
const sunLetters = "تثدذرزسشصضطظلن"

// Harakat.
const (
	aFathatan, aDammatan, aKasratan = 0x064B, 0x064C, 0x064D
	aFatha, aDamma, aKasra          = 0x064E, 0x064F, 0x0650
	aShadda, aSukun, aDaggerAlef    = 0x0651, 0x0652, 0x0670
	aMadda, aHamzaAbove             = 0x0653, 0x0654
)

// arabicLetter is a letter with its marks.
type arabicLetter struct {
	r      rune
	vowel  rune
	shadda bool
	end    int
}

func init() {
	register(arabic, arabicRun, "ar fa ur")
	lineLanguages[arabic] = lineArabic
}

// lineArabic is the language of the Arabic script of a line of lyrics in
// the language lang: Persian or Urdu, when lang is, else the one its
// letters tell, Urdu by those only it writes, Persian by those it shares
// with Urdu, and Arabic else.
func lineArabic(s, lang string) string {
	switch {
	case lang == "fa" || lang == "ur":
		return lang
	case strings.ContainsAny(s, "ٹڈڑںےھہ"):
		return "ur"
	case strings.ContainsAny(s, "پچژگکی"):
		return "fa"
	}
	return "ar"
}

// arabicRun romanizes a run of Arabic script in the language lang, ar, fa
// or ur.
func arabicRun(run, lang string) []piece {
	pointed := strings.ContainsFunc(run, func(r rune) bool { return r >= aFathatan && r <= aSukun || r == aDaggerAlef })
	var (
		pieces []piece
		word   []arabicLetter
	)
	flush := func() {
		pieces = append(pieces, arabicWord(word, pointed, lang)...)
		word = nil
	}
	for i, r := range run {
		end := i + len(string(r))
		var last *arabicLetter
		if len(word) > 0 {
			last = &word[len(word)-1]
		}
		switch {
		case arabicLetters[r] != "" || r == 'ٱ':
			word = append(word, arabicLetter{r: r, end: end})
		case last != nil && r == aShadda:
			last.shadda, last.end = true, end
		case last != nil && (r >= aFathatan && r <= aKasra || r == aSukun || r == aDaggerAlef):
			last.vowel, last.end = r, end
		case last != nil && (r == 0x0640 || r == 0x200C || r == 0x200D || unicode.IsMark(r)):
			// The tatweel, the joiners, and marks of recitation.
			last.end = end
		default:
			flush()
			pieces = append(pieces, piece{end, plain(r)})
		}
	}
	flush()
	return pieces
}

// arabicWord writes the letters of a word.
func arabicWord(w []arabicLetter, pointed bool, lang string) []piece {
	pieces := make([]piece, 0, len(w))
	start := 0
	// The article.
	if len(w) > 2 && (w[0].r == 'ا' || w[0].r == 'ٱ') && w[1].r == 'ل' && lang == "ar" {
		article := "al-"
		if strings.ContainsRune(sunLetters, w[2].r) {
			article = "a" + arabicLetters[w[2].r] + "-"
			w[2].shadda = false
		}
		pieces = append(pieces, piece{w[0].end, ""}, piece{w[1].end, article})
		start = 2
	}
	for i := start; i < len(w); i++ {
		pieces = append(pieces, piece{w[i].end, arabicSyllable(w, i, start, pointed, lang)})
	}
	return pieces
}

// arabicSyllable writes letter i of a word, whose letters start at start,
// with its vowel.
func arabicSyllable(w []arabicLetter, i, start int, pointed bool, lang string) string {
	l := w[i]
	first, last := i == start, i == len(w)-1
	var prev, next arabicLetter
	if i > 0 {
		prev = w[i-1]
	}
	if !last {
		next = w[i+1]
	}
	c := arabicLetters[l.r]
	if lang == "fa" {
		if p, ok := persianLetters[l.r]; ok {
			c = p
		}
	}
	short := map[rune]string{aFatha: "a", aDamma: "u", aKasra: "i", aFathatan: "an", aDammatan: "un", aKasratan: "in", aDaggerAlef: "a"}
	if lang != "ar" {
		short = map[rune]string{aFatha: "a", aDamma: "o", aKasra: "e", aFathatan: "an", aDammatan: "on", aKasratan: "en", aDaggerAlef: "ā"}
		if lang == "ur" {
			short[aDamma], short[aKasra] = "u", "i"
		}
	}
	long := "a"
	if lang == "fa" {
		long = "ā"
	}
	// The aspiration of Urdu, as in بھ, bh.
	if l.r == 'ھ' {
		return "h"
	}
	switch l.r {
	case 'ا', 'ٱ', 'آ':
		switch {
		case l.r == 'ٱ':
			return ""
		case pointed && prev.vowel == aFathatan:
			// The alef of tanwin.
			return ""
		case first && l.vowel != 0 && l.vowel != aSukun:
			return short[l.vowel]
		case first && l.r == 'ا' && pointed:
			return ""
		case first && l.r == 'ا':
			// An alef that starts a word carries a short vowel.
			return "a"
		}
		if pointed && prev.vowel == aFatha && l.r == 'ا' {
			// A long a after its fatha, written once.
			return strings.TrimPrefix(long, "a")
		}
		return long
	case 'أ', 'إ', 'ؤ', 'ئ', 'ء':
		v := short[l.vowel]
		if l.vowel == 0 && l.r == 'إ' {
			v = "i"
		}
		if first {
			if v == "" {
				v = "a"
				if l.r == 'إ' {
					v = "i"
				}
			}
			return v
		}
		return "'" + v
	case 'و':
		switch {
		case pointed && l.vowel == 0 && prev.vowel == aDamma:
			return ""
		case pointed && l.vowel == aSukun && prev.vowel == aFatha:
			return "w"
		case !pointed && !first && next.r != 'ا' && next.r != 'ي' && next.r != 'ی':
			if lang == "ar" || lang == "ur" {
				return "u"
			}
			if prev.r != 0 {
				return "u"
			}
		case !pointed && first && lang != "fa":
			return "w"
		}
	case 'ي', 'ی':
		switch {
		case pointed && l.vowel == 0 && prev.vowel == aKasra:
			return ""
		case pointed && l.vowel == aSukun && prev.vowel == aFatha:
			return "y"
		case !pointed && !first && next.r != 'ا' && next.r != 'و':
			return "i"
		}
		c = "y"
	case 'ى':
		if pointed && prev.vowel == aFatha {
			return ""
		}
		return "a"
	case 'ة', 'ۃ':
		if l.vowel != 0 && l.vowel != aSukun {
			return "t" + short[l.vowel]
		}
		if pointed && prev.vowel == aFatha {
			return ""
		}
		return c
	case 'ه', 'ہ':
		if last && !first && lang == "fa" && l.vowel == 0 {
			return "e"
		}
	case 'ع':
		if first {
			c = ""
		}
	}
	if l.shadda {
		c += c
	}
	return c + short[l.vowel]
}
