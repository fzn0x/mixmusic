package romanize

import (
	"strings"
	"unicode"
)

// The Brahmic scripts of India are written by one engine, over the blocks
// Unicode aligns them in, as ISCII did: a letter is at the same place in
// Devanagari, Bengali, Gurmukhi, Gujarati, Odia, Tamil, Telugu, Kannada
// and Malayalam; Sinhala's are mapped onto those places (sinhala.go).
// They are written by Hunterian, India's national system of romanization,
// its long vowels marked as its full form marks them (ā, ī, ū) and its
// retroflex consonants as its plain form writes them, as the dental ones:
// "प्यार" is "pyār". As each language is read:
//
//   - Hindi, Marathi, Nepali, Gujarati and Punjabi drop the vowel a that a
//     consonant carries where it isn't said, at the end of a word and
//     between a vowel and a consonant with one, by Ohala's rule: "कमला" is
//     "kamlā", "समझना" "samajhnā"; Sanskrit keeps it, and so do Odia and
//     the Dravidian languages;
//   - Bengali and Assamese say that vowel o, and drop it as Hindi does:
//     "মন" is "mon"; they mark no length, as their vowels have none;
//   - Tamil writes neither aspiration nor voice: a consonant is voiced
//     between vowels and after a nasal, as said, "அன்பு" "anbu", and
//     doubled, voiceless, "பக்கம்" "pakkam"; ற்ற is "tr" and ன்ற "ndr";
//   - Tamil, Telugu, Kannada and Malayalam tell short e and o from long ē
//     and ō; Malayalam says the virama at the end of a word as u.

// indicStyle is how a language reads its script.
type indicStyle struct {
	// inherent is the vowel a consonant carries, a or o.
	inherent string
	// plain marks no length of vowels; dravidian tells short e and o from
	// long ones.
	plain, dravidian bool
	// drops is set for languages that drop the inherent vowel where it
	// isn't said.
	drops bool
	// tamil reads consonants by where they are; malayalam says a final
	// virama, reads the clusters of ṟ, and its anusvara is m.
	tamil, malayalam bool
	// assamese writes its sibilants x.
	assamese bool
	// ri is how the vowel ṛ is said: ri, or ru in the south.
	ri string
}

// styleOf returns how the language lang reads the script of the block
// base, its own language when lang writes another.
func styleOf(base rune, lang string) indicStyle {
	s := indicStyle{inherent: "a", ri: "ri"}
	switch base {
	case 0x0900:
		s.drops = lang != "sa"
	case 0x0980:
		s.inherent, s.plain, s.drops = "o", true, true
		s.assamese = lang == "as"
	case 0x0A00, 0x0A80:
		s.drops = true
	case 0x0B80:
		s.dravidian, s.tamil, s.ri = true, true, "ru"
	case 0x0C00, 0x0C80:
		s.dravidian, s.ri = true, "ru"
	case 0x0D00:
		s.dravidian, s.malayalam, s.ri = true, true, "ru"
	case 0x0D80:
		s.dravidian = true
	}
	return s
}

// Offsets of the letters within their block.
const (
	oCandrabindu, oAnusvara, oVisarga = 0x01, 0x02, 0x03
	oNukta, oAvagraha, oVirama, oOm   = 0x3C, 0x3D, 0x4D, 0x50
	oTippi, oAddak                    = 0x70, 0x71
)

// indicConsonants are the consonants by their offsets, as Hunterian writes
// them, the nukta's too.
var indicConsonants = map[int]string{
	0x15: "k", 0x16: "kh", 0x17: "g", 0x18: "gh", 0x19: "n", 0x1A: "ch", 0x1B: "chh", 0x1C: "j", 0x1D: "jh",
	0x1E: "n", 0x1F: "t", 0x20: "th", 0x21: "d", 0x22: "dh", 0x23: "n", 0x24: "t", 0x25: "th", 0x26: "d",
	0x27: "dh", 0x28: "n", 0x29: "n", 0x2A: "p", 0x2B: "ph", 0x2C: "b", 0x2D: "bh", 0x2E: "m", 0x2F: "y",
	0x30: "r", 0x31: "r", 0x32: "l", 0x33: "l", 0x34: "zh", 0x35: "v", 0x36: "sh", 0x37: "sh", 0x38: "s",
	0x39: "h",
	// With a nukta, as क़ q; and Bengali's ৎ, Assamese's ৰ and ৱ.
	0x58: "q", 0x59: "kh", 0x5A: "gh", 0x5B: "z", 0x5C: "r", 0x5D: "rh", 0x5E: "f", 0x5F: "y",
	0x4E + 0x100: "t", 0x70 + 0x100: "r", 0x71 + 0x100: "w",
	// Malayalam's chillus, consonants without a vowel.
	0x7A + 0x200: "n", 0x7B + 0x200: "n", 0x7C + 0x200: "r", 0x7D + 0x200: "l", 0x7E + 0x200: "l", 0x7F + 0x200: "k",
	0x54 + 0x200: "m", 0x55 + 0x200: "y", 0x56 + 0x200: "zh",
}

// nuktaOf is the consonant a nukta makes of the consonant at an offset.
var nuktaOf = map[int]int{0x15: 0x58, 0x16: 0x59, 0x17: 0x5A, 0x1C: 0x5B, 0x21: 0x5C, 0x22: 0x5D, 0x2B: 0x5E, 0x2F: 0x5F, 0x38: 0x36, 0x32: 0x33}

// indicVowels are the vowels by the offsets of their letters: the long
// ones as Hunterian marks them, then as Bengali writes them, and as the
// Dravidian languages tell e and o.
var indicVowels = map[int][3]string{
	0x04: {"a", "a", "a"}, 0x05: {"a", "o", "a"}, 0x06: {"ā", "a", "ā"}, 0x07: {"i", "i", "i"},
	0x08: {"ī", "i", "ī"}, 0x09: {"u", "u", "u"}, 0x0A: {"ū", "u", "ū"}, 0x0B: {"ri", "ri", "ri"},
	0x0C: {"li", "li", "li"}, 0x0D: {"e", "e", "e"}, 0x0E: {"e", "e", "e"}, 0x0F: {"e", "e", "ē"},
	0x10: {"ai", "oi", "ai"}, 0x11: {"o", "o", "o"}, 0x12: {"o", "o", "o"}, 0x13: {"o", "o", "ō"},
	0x14: {"au", "ou", "au"}, 0x60: {"rī", "ri", "rī"}, 0x61: {"lī", "li", "lī"},
	// Sinhala's æ and ǣ.
	0x87: {"æ", "æ", "æ"}, 0x88: {"ǣ", "ǣ", "ǣ"},
}

// signVowel is the vowel of the vowel sign at an offset, by the offset of
// its letter.
var signVowel = map[int]int{
	0x3A: 0x0E, 0x3B: 0x13, 0x3E: 0x06, 0x3F: 0x07, 0x40: 0x08, 0x41: 0x09, 0x42: 0x0A, 0x43: 0x0B, 0x44: 0x60,
	0x45: 0x0D, 0x46: 0x0E, 0x47: 0x0F, 0x48: 0x10, 0x49: 0x11, 0x4A: 0x12, 0x4B: 0x13, 0x4C: 0x14,
	0x4F: 0x14, 0x56: 0x10, 0x57: 0x14, 0x62: 0x0C, 0x63: 0x61,
	// Sinhala's æ and ǣ.
	0x87 + 0x100: 0x87, 0x88 + 0x100: 0x88,
}

// indicUnit is a syllable of a word: the consonants that start it, joined
// by viramas, and its vowel.
type indicUnit struct {
	// cons are the offsets of its consonants.
	cons []int
	// vowel is the offset of its vowel's letter, or vInherent for the vowel
	// its consonant carries, vNone for none, as after a virama.
	vowel int
	// gem doubles its first consonant, as Gurmukhi's addak asks; mods are
	// the anusvara, candrabindu and visarga after it.
	gem  bool
	mods []int
	// text is what it is when it is none of these, as an avagraha or a
	// digit; end is where it ends in its run.
	text string
	end  int
	// open is set while a virama waits for the next consonant of its
	// cluster.
	open bool
}

const (
	vInherent = -1
	vNone     = -2
)

func init() { register(brahmic, brahmicRun, "hi mr ne sa bn as pa gu or ta te kn ml") }

// brahmicRun romanizes a run of the Brahmic scripts of India, block by
// block.
func brahmicRun(run, lang string) []piece {
	var pieces []piece
	from := 0
	for from < len(run) {
		base := rune(0)
		end := from
		for i, r := range run[from:] {
			b := r &^ 0x7F
			if r >= 0x0900 && r < 0x0D80 {
				if base == 0 {
					base = b
				} else if b != base {
					break
				}
			}
			end = from + i + len(string(r))
		}
		part := run[from:end]
		for _, p := range indicWord(units(part, base, offsetIn(base)), styleOf(base, lang)) {
			pieces = append(pieces, piece{from + p.end, p.text})
		}
		from = end
	}
	return pieces
}

// offsetIn returns the offset of a letter of the block base, as the
// aligned blocks have them; -1 for others.
func offsetIn(base rune) func(r rune) int {
	return func(r rune) int {
		if r&^0x7F != base {
			return -1
		}
		return int(r - base)
	}
}

// units parses a word of a block into its syllables, its letters at the
// offsets offset returns.
func units(word string, base rune, offset func(rune) int) []indicUnit {
	var us []indicUnit
	cur := func() *indicUnit {
		if len(us) == 0 {
			us = append(us, indicUnit{vowel: vNone})
		}
		return &us[len(us)-1]
	}
	gem := false
	for i, r := range word {
		end := i + len(string(r))
		o := offset(r)
		// Bengali's ৎ, ৰ and ৱ, and Malayalam's chillus, are consonants
		// of their own.
		switch {
		case base == 0x0980 && (o == 0x4E || o == 0x70 || o == 0x71), base == 0x0B00 && o == 0x71:
			o += 0x100
		case base == 0x0D00 && (o >= 0x7A && o <= 0x7F || o >= 0x54 && o <= 0x56):
			o += 0x200
		}
		_, consonant := indicConsonants[o]
		_, vowel := indicVowels[o]
		switch {
		case r == 0x200C || r == 0x200D:
			cur().end = end
		case consonant && o >= 0x200 && o < 0x300:
			// A chillu: a consonant without a vowel.
			u := cur()
			if u.open {
				u.cons = append(u.cons, o)
				u.vowel, u.open, u.end = vNone, false, end
				continue
			}
			us = append(us, indicUnit{cons: []int{o}, vowel: vNone, end: end})
		case consonant:
			if u := cur(); u.open {
				u.cons = append(u.cons, o)
				u.vowel, u.open, u.end = vInherent, false, end
				continue
			}
			us = append(us, indicUnit{cons: []int{o}, vowel: vInherent, gem: gem, end: end})
			gem = false
			if o == 0x4E+0x100 {
				cur().vowel = vNone
			}
		case o == oNukta:
			if u := cur(); len(u.cons) > 0 {
				last := &u.cons[len(u.cons)-1]
				if n, ok := nuktaOf[*last]; ok {
					*last = n
				}
				u.end = end
			}
		case o == oVirama:
			u := cur()
			if len(u.cons) > 0 && u.vowel == vInherent {
				u.vowel, u.open = vNone, true
			}
			u.end = end
		case signVowel[o] != 0 || o == 0x55:
			u := cur()
			v, ok := signVowel[o]
			switch {
			case o == 0x55:
				// Kannada's length mark lengthens e and o.
				v, ok = map[int]int{0x07: 0x08, 0x0E: 0x0F, 0x12: 0x13}[u.vowel]
			case (u.vowel == 0x0E || u.vowel == 0x0F) && (o == 0x3E || o == 0x57):
				// The two parts of o and au, written apart.
				v = map[int]int{0x3E: u.vowel + 4, 0x57: 0x14}[o]
			}
			if ok {
				u.vowel = v
			}
			u.open, u.end = false, end
		case vowel || base == 0x0A00 && (o == 0x72 || o == 0x73):
			if base == 0x0A00 && (o == 0x72 || o == 0x73) {
				// Gurmukhi's ੲ and ੳ: i and u, or the vowel of the sign
				// after them.
				o = map[int]int{0x72: 0x07, 0x73: 0x09}[o]
			}
			us = append(us, indicUnit{vowel: o, end: end})
		case o == oCandrabindu || o == oAnusvara || o == oVisarga || o == 0x00 || base == 0x0A00 && o == oTippi:
			u := cur()
			u.mods = append(u.mods, o)
			u.end = end
		case base == 0x0A00 && o == oAddak:
			gem = true
			cur().end = end
		case o == oAvagraha:
			us = append(us, indicUnit{vowel: vNone, text: "'", end: end})
		case o == oOm:
			us = append(us, indicUnit{vowel: vNone, text: "om", end: end})
		case o >= 0x51 && o <= 0x54 || unicode.IsMark(r):
			// Marks of accent.
			cur().end = end
		default:
			us = append(us, indicUnit{vowel: vNone, text: plain(r), end: end})
		}
	}
	return us
}

// indicWord writes the syllables of a word as style reads them.
func indicWord(us []indicUnit, style indicStyle) []piece {
	if style.drops {
		dropInherent(us)
	}
	pieces := make([]piece, 0, len(us))
	for i := range us {
		u := &us[i]
		if u.text != "" || len(u.cons) == 0 && u.vowel == vNone {
			pieces = append(pieces, piece{u.end, u.text})
			continue
		}
		var b strings.Builder
		for k := range u.cons {
			latin := consonantOf(us, i, k, style)
			if k == 0 && u.gem && latin != "" {
				latin = latin[:1] + latin
			}
			b.WriteString(latin)
		}
		switch {
		case u.vowel == vInherent:
			b.WriteString(style.inherent)
		case u.vowel == vNone:
			if style.malayalam && len(u.cons) > 0 && u.cons[len(u.cons)-1] < 0x100 && (i+1 == len(us) || us[i+1].text != "") {
				// Malayalam says a word's last virama as a short u.
				b.WriteString("u")
			}
		default:
			b.WriteString(vowelOf(u.vowel, style))
		}
		for _, m := range u.mods {
			switch m {
			case oVisarga:
				b.WriteString("h")
			case oCandrabindu:
				b.WriteString("n")
			default:
				b.WriteString(nasalBefore(us, i, style))
			}
		}
		pieces = append(pieces, piece{u.end, b.String()})
	}
	return pieces
}

// vowelOf writes the vowel at an offset as style marks vowels.
func vowelOf(o int, style indicStyle) string {
	v := indicVowels[o]
	switch {
	case o == 0x0B && style.ri == "ru":
		return "ru"
	case style.plain:
		return v[1]
	case style.dravidian:
		return v[2]
	}
	return v[0]
}

// nasalBefore writes an anusvara or a candrabindu on syllable i: the nasal
// of the consonant after it, m before p, b and m; n elsewhere, but the
// ng of Bengali, and Malayalam's and the end of a Dravidian word's m.
func nasalBefore(us []indicUnit, i int, style indicStyle) string {
	if style.plain {
		return "ng"
	}
	if i+1 < len(us) && len(us[i+1].cons) > 0 {
		if c := us[i+1].cons[0]; c >= 0x2A && c <= 0x2E {
			return "m"
		}
		if style.malayalam {
			return "m"
		}
		return "n"
	}
	if style.dravidian || style.malayalam {
		return "m"
	}
	return "n"
}

// dropInherent drops the vowels consonants carry where they aren't said:
// at the end of a word of more than a syllable, after one consonant; and,
// from the end back, in a syllable of one consonant after a vowel and
// before a consonant with a vowel, but in the first syllable.
func dropInherent(us []indicUnit) {
	n := len(us)
	if n < 2 {
		return
	}
	said := func(u indicUnit) bool { return u.vowel != vNone && u.text == "" }
	// After a cluster, the last a stays when its last consonant is r, y
	// or v, as in मित्र, mitra, and सत्य, satya, and goes else, as in
	// फ़िल्म, film.
	if last := &us[n-1]; last.vowel == vInherent && len(last.mods) == 0 && said(us[n-2]) {
		if c := last.cons[len(last.cons)-1]; len(last.cons) == 1 || c != 0x30 && c != 0x2F && c != 0x35 {
			last.vowel = vNone
		}
	}
	for i := n - 2; i >= 1; i-- {
		u := &us[i]
		if u.vowel != vInherent || len(u.cons) != 1 || len(u.mods) > 0 || u.gem {
			continue
		}
		prev, next := us[i-1], us[i+1]
		// After an open syllable: one closed by a nasal keeps it, as
		// ज़िंदगी, zindagī.
		if len(prev.mods) == 0 && (said(prev) && len(prev.cons) > 0 || len(prev.cons) == 0 && prev.vowel >= 0) {
			if len(next.cons) == 1 && said(next) {
				u.vowel = vNone
			}
		}
	}
}

// consonantOf writes consonant k of syllable i as style reads it: as
// Hunterian has it, Bengali's ব and য as b and j, and Tamil's by where it
// is.
func consonantOf(us []indicUnit, i, k int, style indicStyle) string {
	u := us[i]
	c := u.cons[k]
	latin := indicConsonants[c]
	if style.plain {
		switch {
		case c == 0x35:
			latin = "b"
		case c == 0x2F && k == 0:
			latin = "j"
		case c == 0x38 || c == 0x36 || c == 0x37:
			latin = "sh"
			if k+1 < len(u.cons) || k > 0 {
				latin = "s"
			}
			if style.assamese {
				latin = "x"
			}
		}
		return latin
	}
	if style.malayalam {
		next := -1
		if k+1 < len(u.cons) {
			next = u.cons[k+1]
		}
		switch {
		case c == 0x24 || c == 0x25:
			latin = "th"
		case c == 0x31 && k > 0 && (u.cons[k-1] == 0x31 || u.cons[k-1] == 0x28):
			// റ്റ is tt, and ന്റ nt.
			latin = "t"
		case c == 0x31 && next == 0x31:
			latin = "t"
		}
		return latin
	}
	if !style.tamil {
		return latin
	}
	// Tamil: what comes before the consonant, a consonant of its cluster,
	// a vowel, or nothing.
	before, afterVowel := -1, false
	switch {
	case k > 0:
		before = u.cons[k-1]
	case i > 0 && us[i-1].text == "" && us[i-1].vowel != vNone:
		afterVowel = true
	case i > 0 && us[i-1].text == "" && len(us[i-1].cons) > 0:
		before = us[i-1].cons[len(us[i-1].cons)-1]
	}
	nasal := map[int]bool{0x19: true, 0x1E: true, 0x23: true, 0x28: true, 0x29: true, 0x2E: true}[before]
	sonorant := map[int]bool{0x2F: true, 0x30: true, 0x32: true, 0x33: true, 0x34: true, 0x35: true}[before]
	// Before a stop of its cluster, as the first of a pair, a stop is
	// voiceless.
	stop := func(c int) bool { return c == 0x15 || c == 0x1A || c == 0x1F || c == 0x24 || c == 0x2A || c == 0x31 }
	voiced := (afterVowel || nasal || sonorant) && !(k+1 < len(u.cons) && stop(u.cons[k+1]))
	switch c {
	case 0x15:
		if voiced {
			return "g"
		}
		return "k"
	case 0x1A:
		switch {
		case before == 0x1E:
			return "j"
		case before == 0x1A || before == 0x1F || before == 0x31:
			return "ch"
		case k+1 < len(u.cons) && u.cons[k+1] == 0x1A:
			return "ch"
		}
		return "s"
	case 0x1E:
		if k+1 < len(u.cons) && u.cons[k+1] == 0x1A {
			return "n"
		}
		return "nj"
	case 0x19:
		if k+1 < len(u.cons) && u.cons[k+1] == 0x15 {
			return "n"
		}
		return "ng"
	case 0x1F:
		if voiced {
			return "d"
		}
		return "t"
	case 0x24:
		switch {
		case voiced:
			return "dh"
		case k+1 < len(u.cons) && u.cons[k+1] == 0x24:
			return "t"
		}
		return "th"
	case 0x2A:
		if voiced {
			return "b"
		}
		return "p"
	case 0x31:
		switch {
		case before == 0x31:
			return "r"
		case before == 0x29:
			return "dr"
		case k+1 < len(u.cons) && u.cons[k+1] == 0x31:
			return "t"
		}
		return "r"
	}
	return latin
}
