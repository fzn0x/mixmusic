package romanize

import "strings"

// Tibetan is written by Extended Wylie (EWTS, the Tibetan and Himalayan
// Library's), a syllable at a time as the tsheg parts them, which it
// writes as a space: "བཀྲ་ཤིས" is "bkra shis". A syllable without a vowel
// sign carries a after its root, which tibetanSyllable finds as the
// grammar of its prefixes and suffixes has it: "བསམ" is "bsam", "ལགས"
// "lags"; a prefix ག before the root ཡ is "g." as EWTS writes it, apart
// from གྱ, "gy".

// tibetanLetters are the consonants of Tibetan, from ཀ, and their
// subjoined forms, from ྐ.
var tibetanLetters = strings.Fields("k kh g gh ng c ch j _ ny T Th D Dh N t th d dh n p ph b bh m ts tsh dz dzh w zh z ' y r l sh Sh s h a kSh R kk rr")

// tibetanVowels are the vowel signs and marks, from ཱ.
var tibetanVowels = map[rune]string{
	0x0F71: "A", 0x0F72: "i", 0x0F73: "I", 0x0F74: "u", 0x0F75: "U", 0x0F76: "r-i", 0x0F77: "r-I",
	0x0F78: "l-i", 0x0F79: "l-I", 0x0F7A: "e", 0x0F7B: "ai", 0x0F7C: "o", 0x0F7D: "au", 0x0F80: "-i",
	0x0F81: "-I",
}

// tibetanMarks are the marks that follow the vowel.
var tibetanMarks = map[rune]string{0x0F7E: "M", 0x0F7F: "H", 0x0F82: "~M`", 0x0F83: "~M", 0x0F84: "?"}

// tibetanPrefixes are the prefixes, and the roots each goes before.
var tibetanPrefixes = map[rune]string{
	'ག': "ཅཉཏདནཙཞཟཡཤས", 'ད': "ཀགངཔབམ", 'བ': "ཀགཅཏདཙཞཟཤས", 'མ': "ཁགངཆཇཉཐདནཚཛ", 'འ': "ཁགཆཇཐདཕབཚཛ",
}

// tibetanSuffixes are the letters that end a syllable.
const tibetanSuffixes = "གངདནབམའརལས"

// tibetanStack is a stack of letters: its base, those under it, its vowel
// and its marks.
type tibetanStack struct {
	letters       []rune
	vowel, marks  string
	stacked       bool
	firstBaseRune rune
}

func init() { register(tibetan, tibetanRun, "bo dz") }

// tibetanRun romanizes a run of Tibetan, syllable by syllable.
func tibetanRun(run, _ string) []piece {
	var (
		pieces []piece
		syl    []tibetanStack
		end    int
	)
	flush := func() {
		if len(syl) > 0 {
			pieces = append(pieces, piece{end, tibetanSyllable(syl)})
			syl = nil
		}
	}
	for i, r := range run {
		switch {
		case r >= 0x0F40 && r <= 0x0F6C:
			syl = append(syl, tibetanStack{letters: []rune{r}, firstBaseRune: r})
		case r >= 0x0F90 && r <= 0x0FBC && len(syl) > 0:
			s := &syl[len(syl)-1]
			s.letters = append(s.letters, r-0x50)
			s.stacked = true
		case tibetanVowels[r] != "" && len(syl) > 0:
			s := &syl[len(syl)-1]
			switch v := tibetanVowels[r]; {
			case s.vowel == "A" && (v == "i" || v == "u"):
				// ཱི and ཱུ, written apart: I and U.
				s.vowel = strings.ToUpper(v)
			default:
				s.vowel += v
			}
		case tibetanMarks[r] != "" && len(syl) > 0:
			syl[len(syl)-1].marks += tibetanMarks[r]
		default:
			flush()
			pieces = append(pieces, piece{i + len(string(r)), plain(r)})
		}
		end = i + len(string(r))
	}
	flush()
	return pieces
}

// letter writes a letter of Tibetan.
func tibetanLetter(r rune) string {
	if r == 0x0FBA {
		return "w"
	}
	if r == 0x0FBB {
		return "y"
	}
	if r == 0x0FBC {
		return "r"
	}
	if r < 0x0F40 || int(r-0x0F40) >= len(tibetanLetters) {
		return ""
	}
	l := tibetanLetters[r-0x0F40]
	switch l {
	case "_":
		return ""
	case "a":
		// ཨ carries the vowel alone.
		return ""
	}
	return l
}

// tibetanSyllable writes a syllable, its stacks in order, with a after its
// root when it has no vowel sign.
func tibetanSyllable(syl []tibetanStack) string {
	root := -1
	for i, s := range syl {
		if s.vowel != "" {
			root = i
			break
		}
	}
	if root < 0 {
		root = 0
		for i, s := range syl {
			if s.stacked {
				root = i
				break
			}
		}
		single := root == 0 && !syl[0].stacked
		if single && len(syl) >= 3 {
			// A prefix before the root, when the first is one the second
			// takes; else the root is first, as in ལགས.
			first, second := syl[0].firstBaseRune, syl[1].firstBaseRune
			if strings.ContainsRune(tibetanPrefixes[first], second) || len(syl) == 4 {
				root = 1
			}
		}
		if single && len(syl) == 2 && !strings.ContainsRune(tibetanSuffixes, syl[1].firstBaseRune) && strings.ContainsRune(tibetanPrefixes[syl[0].firstBaseRune], syl[1].firstBaseRune) {
			// The second can't end a syllable: it is the root, as in གཡ.
			root = 1
		}
		syl[root].vowel = "a"
	}
	var b strings.Builder
	for i, s := range syl {
		for _, r := range s.letters {
			b.WriteString(tibetanLetter(r))
		}
		// A letter before the root that isn't its prefix carries a, as in
		// Sanskrit's པདྨེ, padme.
		if i < root && s.vowel == "" && !(i == root-1 && tibetanPrefixes[s.firstBaseRune] != "" && !s.stacked) {
			b.WriteByte('a')
		}
		if i == 0 && root == 1 && s.firstBaseRune == 'ག' && syl[1].firstBaseRune == 'ཡ' && !syl[1].stacked {
			b.WriteByte('.')
		}
		b.WriteString(s.vowel)
		b.WriteString(s.marks)
	}
	return b.String()
}
