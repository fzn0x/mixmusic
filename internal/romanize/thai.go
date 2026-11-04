package romanize

import "strings"

// Thai is written by the Royal Thai General System of Transcription (the
// Royal Institute, 1999), which marks neither tones nor length: "สวัสดี"
// is "sa wat di". Thai writes no spaces between words, and RTGS, written
// by word, would need a dictionary to find them: thaiRun parts a run into
// syllables by rule instead, and writes them a space apart.
//
// Its rules: a syllable is a leading vowel (เ แ โ ใ ไ), a consonant, the
// second of a cluster (ร, ล, ว), the vowel's signs, and a final; ห before
// a sonorant and อ before ย are silent, ทร is s, and ร after จ, ซ, ศ and
// ส is silent; a letter under ์ is silent. A run of consonants without a
// vowel reads as Thai does: two, a syllable of o (คน, khon); three, one
// of a then one of o (ถนน, tha non), a cluster then o (กรม, krom).
// It is right on the syllables of common words, and wrong where only a
// dictionary tells, as words of Pali and Sanskrit with an unwritten a
// (สมัย, samai, reads sa mai) and some clusters it can't tell from a
// final and an initial.

// thaiInitials and thaiFinals are the sounds of the consonants at the
// start of a syllable and at its end, from ก.
var (
	thaiInitials = strings.Fields("k kh kh kh kh kh ng ch ch ch s ch y d t th th th n d t th th th n b p ph f ph f ph m y r rue l lue w s s s h l _ h")
	thaiFinals   = strings.Fields("k k k k k k ng t _ t t t n t t t t t n t t t t t n p p _ _ p p p m i n _ n _ o t t t _ n _ _")
)

const (
	thaiFirst, thaiLast = 0x0E01, 0x0E2E
	thaiRue, thaiLue    = 0x0E24, 0x0E26
	thaiO, thaiYo       = 0x0E2D, 0x0E22
	thaiWo, thaiRo      = 0x0E27, 0x0E23
	thaiHo, thaiLo      = 0x0E2B, 0x0E25
	thaiKaran           = 0x0E4C
	thaiRepeat          = 0x0E46
)

func thaiConsonant(r rune) bool {
	return r >= thaiFirst && r <= thaiLast && r != thaiRue && r != thaiLue
}
func thaiLeading(r rune) bool { return r >= 0x0E40 && r <= 0x0E44 }
func thaiTone(r rune) bool    { return r >= 0x0E48 && r <= 0x0E4B }

// thaiAfter tells the signs that follow a consonant and belong to it: the
// vowels after and above and below it, the tones, and the marks.
func thaiAfter(r rune) bool {
	return r == 0x0E30 || r >= 0x0E31 && r <= 0x0E3A || r >= 0x0E45 && r <= 0x0E4E && r != thaiRepeat
}

// thaiClusters are the clusters of two consonants, by their first.
var thaiClusters = map[rune]string{
	'ก': "รลว", 'ข': "รลว", 'ค': "รลว", 'ต': "ร", 'ป': "รล", 'ผ': "ล", 'พ': "รล", 'บ': "รล", 'ด': "ร", 'ฟ': "รล",
}

// thaiText is a run of Thai, parsed a syllable at a time.
type thaiText struct {
	rs []rune
	i  int
}

func (t *thaiText) at(i int) rune {
	if i < 0 || i >= len(t.rs) {
		return 0
	}
	return t.rs[i]
}

// anchored tells whether the consonant at i takes signs after it, which
// make it a syllable's initial, not a final.
func (t *thaiText) anchored(i int) bool { r := t.at(i + 1); return thaiAfter(r) && r != thaiKaran }

// silent tells whether the consonant at i is silent: under ์, or before
// one, with a vowel sign between, as in ศักดิ์; and, pair set, before
// another under one, as the ท of จันทร์.
func (t *thaiText) silent(i int, pair bool) bool {
	if !thaiConsonant(t.at(i)) {
		return false
	}
	return t.at(i+1) == thaiKaran || thaiAfter(t.at(i+1)) && t.at(i+2) == thaiKaran ||
		pair && thaiConsonant(t.at(i+1)) && t.at(i+2) == thaiKaran
}

// after returns the first letter from i that isn't a tone, and past where
// it is.
func (t *thaiText) after(i int) rune { return t.at(t.past(i)) }

func (t *thaiText) past(i int) int {
	for thaiTone(t.at(i)) {
		i++
	}
	return i
}

func (t *thaiText) tones() {
	for thaiTone(t.at(t.i)) {
		t.i++
	}
}

// skipSilent passes over silent letters, pairs too when pair is set.
func (t *thaiText) skipSilent(pair bool) {
	for t.silent(t.i, pair) {
		t.i++
		for thaiAfter(t.at(t.i)) {
			t.i++
		}
	}
	for t.at(t.i) == thaiKaran {
		t.i++
	}
}

// final reads the final of a syllable of vowel v, if one follows, and
// returns the syllable. A vowel that can end a syllable, open, leaves two
// consonants alone at the end to the syllable of o they make, as in
// ประชาชน, pra cha chon.
func (t *thaiText) final(v string, open bool) string {
	t.tones()
	t.skipSilent(false)
	r := t.at(t.i)
	if !thaiConsonant(r) || t.anchored(t.i) || t.starts(t.i) || open && t.bare(t.i) == 2 && !thaiConsonant(t.at(t.i+2)) {
		return v
	}
	t.i++
	f := thaiFinals[r-thaiFirst]
	switch {
	case f == "i" && strings.HasSuffix(v, "i"):
		f = ""
	case f == "o" && v == "a":
		f = "o"
	case f == "_":
		f = ""
	}
	t.tones()
	t.skipSilent(true)
	return v + f
}

// bare counts the consonants from i that have no signs after them, nor
// the vowel อ, as in ตลอด, ta lot.
func (t *thaiText) bare(i int) int {
	n := 0
	for thaiConsonant(t.at(i+n)) && !t.anchored(i+n) && !t.silent(i+n, true) && !t.starts(i+n) {
		n++
	}
	return n
}

// starts tells whether the consonant at i starts a syllable whose vowel
// is a letter, as ล of ตลอด, whose o is อ, and ส of สวย, whose ua is ว; or
// is a silent ห before a sonorant with its vowel, as in หนาว.
func (t *thaiText) starts(i int) bool {
	c, next := t.at(i), t.at(i+1)
	switch {
	case next == thaiO && !t.anchored(i+1):
		return true
	case next == thaiWo && !t.anchored(i+1) && thaiConsonant(t.at(i+2)) && !t.anchored(i+2):
		return true
	case c == thaiHo && strings.ContainsRune("งญนมยรลว", next) && (t.anchored(i+1) || t.at(i+2) == thaiO):
		return true
	}
	return false
}

// syllable reads the syllable at t.i.
func (t *thaiText) syllable() string {
	lead := rune(0)
	if thaiLeading(t.at(t.i)) {
		lead = t.at(t.i)
		t.i++
	}
	c := t.at(t.i)
	switch {
	case c == thaiRue || c == thaiLue:
		t.i++
		if t.at(t.i) == 0x0E45 {
			t.i++
		}
		return map[rune]string{thaiRue: "rue", thaiLue: "lue"}[c]
	case !thaiConsonant(c):
		if lead != 0 {
			return map[rune]string{'เ': "e", 'แ': "ae", 'โ': "o", 'ใ': "ai", 'ไ': "ai"}[lead]
		}
		t.i++
		if thaiAfter(c) {
			// A vowel without its consonant.
			return map[rune]string{'ะ': "a", 'ั': "a", 'า': "a", 'ำ': "am", 'ิ': "i", 'ี': "i", 'ึ': "ue", 'ื': "ue", 'ุ': "u", 'ู': "u"}[c]
		}
		return plain(c)
	}
	start := t.i
	t.i++
	initial := thaiInitials[c-thaiFirst]
	if initial == "_" {
		initial = ""
	}
	next := t.at(t.i)
	switch {
	case c == thaiHo && strings.ContainsRune("งญนมยรลว", next) && (t.anchored(t.i) || lead != 0 || t.bare(t.i) > 0 || t.starts(t.i)):
		// ห before a sonorant only sets its tone.
		initial = thaiInitials[next-thaiFirst]
		t.i++
	case c == thaiO && next == thaiYo && (t.at(t.i+1) == '่' || t.at(t.i+1) == 'ู' || t.at(t.i+1) == 'า' && t.at(t.i+2) == 'ก'):
		// อย่า, อยู่, อย่าง and อยาก.
		initial = "y"
		t.i++
	case next == thaiRo && strings.ContainsRune("จซศส", c) && (t.anchored(t.i) || lead != 0):
		t.i++
	case next == thaiRo && c == 'ท' && (t.anchored(t.i) || lead != 0):
		initial = "s"
		t.i++
	case strings.ContainsRune(thaiClusters[c], next) && next != 0 && (t.anchored(t.i) || lead != 0 || t.at(t.i+1) == thaiO && !t.anchored(t.i+1)):
		initial += thaiInitials[next-thaiFirst]
		t.i++
	}
	t.tones()
	v := t.at(t.i)
	switch lead {
	case 'เ':
		switch {
		case v == 'า' && t.at(t.i+1) == 'ะ':
			t.i += 2
			return initial + "o"
		case v == 'า':
			t.i++
			return initial + "ao"
		case v == 'ี' && t.after(t.i+1) == thaiYo:
			t.i = t.past(t.i+1) + 1
			return initial + t.final("ia", true)
		case v == 'ื' && t.after(t.i+1) == thaiO:
			t.i = t.past(t.i+1) + 1
			return initial + t.final("uea", true)
		case v == 'ิ':
			t.i++
			return initial + t.final("oe", false)
		case v == '็':
			t.i++
			return initial + t.final("e", false)
		case v == thaiO && !t.anchored(t.i):
			// เ-อ takes no final: เ-ิ does.
			t.i++
			if t.at(t.i) == 'ะ' {
				t.i++
			}
			return initial + "oe"
		case v == thaiYo && !t.anchored(t.i):
			t.i++
			return initial + "oei"
		case v == 'ะ':
			t.i++
			return initial + "e"
		case v == thaiWo && !t.anchored(t.i):
			t.i++
			return initial + "eo"
		}
		return initial + t.final("e", true)
	case 'แ':
		switch {
		case v == 'ะ':
			t.i++
			return initial + "ae"
		case v == '็':
			t.i++
		case v == thaiWo && !t.anchored(t.i):
			t.i++
			return initial + "aeo"
		}
		return initial + t.final("ae", true)
	case 'โ':
		if v == 'ะ' {
			t.i++
			return initial + "o"
		}
		return initial + t.final("o", true)
	case 'ใ', 'ไ':
		// The ย of ไทย is silent.
		if v == thaiYo && !t.anchored(t.i) && t.bare(t.i) == 1 && !thaiConsonant(t.at(t.i+1)) {
			t.i++
		}
		return initial + "ai"
	}
	switch v {
	case 'ะ':
		t.i++
		return initial + "a"
	case 'ั':
		t.i++
		t.tones()
		if t.at(t.i) == thaiWo {
			t.i++
			if t.at(t.i) == 'ะ' {
				t.i++
				return initial + "ua"
			}
			return initial + t.final("ua", true)
		}
		return initial + t.final("a", false)
	case 'า':
		t.i++
		return initial + t.final("a", true)
	case 'ำ':
		t.i++
		return initial + "am"
	case 'ิ', 'ี':
		t.i++
		return initial + t.final("i", true)
	case 'ึ':
		t.i++
		return initial + t.final("ue", true)
	case 'ื':
		t.i++
		t.tones()
		if t.at(t.i) == thaiO && !t.anchored(t.i) {
			t.i++
		}
		return initial + t.final("ue", true)
	case 'ุ', 'ู':
		t.i++
		return initial + t.final("u", true)
	case '็':
		// ก็, ko.
		t.i++
		return initial + "o"
	case thaiO:
		if !t.anchored(t.i) {
			t.i++
			return initial + t.final("o", true)
		}
	case thaiWo:
		if !t.anchored(t.i) && (thaiConsonant(t.at(t.i+1)) && !t.anchored(t.i+1) || t.at(t.i+1) == 0) {
			t.i++
			return initial + t.final("ua", true)
		}
	case thaiRo:
		if t.at(t.i+1) == thaiRo && !t.anchored(t.i+1) {
			// รร: an, or a before a final.
			t.i += 2
			if f := t.final("", false); f != "" {
				return initial + "a" + f
			}
			return initial + "an"
		}
	}
	// No vowel written: a, or o before a final, as the consonants without
	// vowels after the initial pair.
	n := t.bare(t.i)
	cluster := n == 2 && t.i == start+1 && strings.ContainsRune(thaiClusters[c], t.at(t.i)) && !thaiConsonant(t.at(t.i+2))
	if n == 0 || t.i == start+1 && n%2 == 0 && !cluster {
		return initial + "a"
	}
	if cluster {
		// A cluster then a final: กรม, krom.
		initial += thaiInitials[t.at(t.i)-thaiFirst]
		t.i++
	}
	return initial + t.final("o", false)
}

func init() { register(thai, thaiRun, "th") }

// thaiRun romanizes a run of Thai, a syllable apart.
func thaiRun(run, _ string) []piece {
	t := &thaiText{rs: []rune(run)}
	var pieces []piece
	at, last := 0, ""
	for t.i < len(t.rs) {
		from := t.i
		var latin string
		if t.rs[t.i] == thaiRepeat {
			// ๆ says the word before again.
			t.i++
			latin = last
		} else if t.rs[t.i] == 0x0E2F {
			t.i++
		} else {
			latin = t.syllable()
			last = latin
		}
		if t.i == from {
			t.i++
		}
		for _, r := range t.rs[from:t.i] {
			at += len(string(r))
		}
		if len(pieces) > 0 && latin != "" {
			latin = " " + latin
		}
		pieces = append(pieces, piece{at, latin})
	}
	return pieces
}
