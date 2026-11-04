package romanize

import "strings"

// Lao is written by the BGN/PCGN system of 1966, which builds on that of
// Laos's Commission Nationale de Toponymie, the system Lao names are
// written in Latin letters by on maps: "ຫຼວງພະບາງ" is "louang pha bang",
// "ສະຫວັນນະເຂດ" "sa van na khét". Laos has no national system of its own,
// nor has the UN approved one, as the UN's working group on romanization
// found in 2013. It reads as French spells: ou is u, x is ຊ, gn is ຍ, and
// é, è and ô are the vowels of ເ, ແ and ໂ. Tones aren't written.
//
// Lao writes no spaces between words, and BGN/PCGN, written by word, would
// need a dictionary to find them: laoRun parts a run into syllables by
// rule instead, and writes them a space apart, as thaiRun does. Its rules:
// a syllable is a leading vowel (ເ ແ ໂ ໃ ໄ), a consonant, the second of a
// cluster (ຼ, ລ or ຣ after a stop, and ວ, which is o), the vowel's signs
// and letters (ອ, ວ, ຍ and ຽ), and a final, one of ກ ງ ດ ນ ບ ມ ຍ ວ; ຫ
// before a sonorant only sets its tone, and ອ is a vowel's seat. Lao has
// written every vowel since its spelling reform, so the rules miss only
// what BGN/PCGN knows by the word: the a a few words say without writing
// it (ຂວາວ, "khavao"), and a loanword's consonant that both ends a
// syllable and starts the next (ຄຼິສຕຽນ, "Khlitsatian").

// laoInitials are the sounds of the consonants at the start of a syllable,
// from U+0E80, the Pali letters of Unicode 12 as Lao says them; "-" where
// there is none, and "_" for ອ, which carries a vowel.
var laoInitials = strings.Fields("- k kh - kh - kh ng ch ch x - x gn gn t th d th n d t th th th n b p ph f ph f ph m y r - l - v s s s h l _ h")

// laoFinals are the sounds of the consonants that end a syllable.
var laoFinals = map[rune]string{'ກ': "k", 'ງ': "ng", 'ດ': "t", 'ນ': "n", 'ບ': "p", 'ມ': "m", 'ຍ': "y", 'ວ': "o"}

// laoClusters are the clusters of a stop and ລ or ຣ, by the stop.
var laoClusters = map[rune]string{
	'ກ': "ຣລ", 'ຂ': "ຣລ", 'ຄ': "ຣລ", 'ປ': "ຣລ", 'ພ': "ຣລ", 'ຜ': "ລ", 'ບ': "ຣລ", 'ຟ': "ຣລ", 'ຕ': "ຣ", 'ທ': "ຣ", 'ດ': "ຣ",
}

// laoSonorants are the consonants a silent ຫ goes before: those BGN/PCGN's
// note names, and ງ, which Lao writes so too, as in ເຫງົາ, ngao.
const laoSonorants = "ງຍນມຣລວ"

// laoWoFirsts are the consonants ວ follows in a cluster, as ຂ of ຂວາ: not
// the nasals, which end the syllable before it, as ນ of ບ້ານວັດ, ban vat.
const laoWoFirsts = "ກຂຄຈສຊຕທດບປຜພຝຟ"

const (
	laoHo, laoO, laoWo, laoNyo   = 'ຫ', 'ອ', 'ວ', 'ຍ'
	laoLo, laoSignLo, laoSignNyo = 'ລ', 0x0EBC, 0x0EBD
	laoCancel, laoRepeat         = 0x0ECC, 0x0EC6
)

func laoConsonant(r rune) bool {
	if r >= 0x0EDC && r <= 0x0EDF {
		// ໜ and ໝ, ຫນ and ຫມ, and Khmu's ໞ and ໟ.
		return true
	}
	return r >= 0x0E81 && r <= 0x0EAE && laoInitials[r-0x0E80] != "-"
}

func laoInitial(r rune) string {
	switch r {
	case 0x0EDC:
		return "n"
	case 0x0EDD:
		return "m"
	case 0x0EDE:
		return "k"
	case 0x0EDF:
		return "gn"
	}
	if s := laoInitials[r-0x0E80]; s != "_" {
		return s
	}
	return ""
}

func laoLeading(r rune) bool { return r >= 0x0EC0 && r <= 0x0EC4 }
func laoTone(r rune) bool    { return r >= 0x0EC8 && r <= 0x0ECB }

// laoSign tells the signs that follow a consonant and make it a
// syllable's start: the vowels after, above and below it, ຼ and ຽ, and the
// tones.
func laoSign(r rune) bool {
	return r >= 0x0EB0 && r <= 0x0EB9 || r >= 0x0EBB && r <= 0x0EBD || r == 0x0ECD || laoTone(r)
}

// laoFits tells the vowel signs that go with each leading vowel.
var laoFits = map[rune]string{'ເ': "ະັາິີຶືົ", 'ແ': "ະັ", 'ໂ': "ະ"}

// laoText is a run of Lao, parsed a syllable at a time.
type laoText struct {
	rs []rune
	i  int
}

func (t *laoText) at(i int) rune {
	if i < 0 || i >= len(t.rs) {
		return 0
	}
	return t.rs[i]
}

// anchored tells whether the consonant at i takes signs after it, which
// make it a syllable's start, not a final.
func (t *laoText) anchored(i int) bool { return laoConsonant(t.at(i)) && laoSign(t.at(i+1)) }

// past returns the first place from i that isn't a tone.
func (t *laoText) past(i int) int {
	for laoTone(t.at(i)) {
		i++
	}
	return i
}

func (t *laoText) tones() { t.i = t.past(t.i) }

// starts tells whether the consonant at i starts a syllable whose vowel is
// a letter, as ກ of ກອງ, whose o is ອ, and ຄ of ຄວນ, whose oua is ວ; or
// starts a cluster, as ກ of ກວ່າ; or is a silent ຫ before a sonorant.
func (t *laoText) starts(i int) bool {
	c, next := t.at(i), t.at(i+1)
	if !laoConsonant(c) {
		return false
	}
	switch {
	case next == laoO && !t.anchored(i+1) && !t.starts(i+1):
		return true
	case next == laoWo && t.anchored(i+1) && strings.ContainsRune(laoWoFirsts, c):
		return true
	case next == laoWo && !t.anchored(i+1) && laoConsonant(t.at(i+2)) && !t.anchored(i+2):
		return true
	case c == laoHo && strings.ContainsRune(laoSonorants, next):
		return true
	case next != 0 && strings.ContainsRune(laoClusters[c], next) && t.anchored(i+1):
		return true
	}
	return false
}

// bare counts the consonants from i that start no syllable of their own.
func (t *laoText) bare(i int) int {
	n := 0
	for laoConsonant(t.at(i+n)) && !t.anchored(i+n) && !t.starts(i+n) {
		n++
	}
	return n
}

// final reads the final of a syllable of vowel v, if one follows. A vowel
// that must take one, need, as ັ, takes the consonant after it though it
// could start a syllable, as ນ of ທັນວາ, than va.
func (t *laoText) final(v string, need bool) string {
	t.tones()
	r := t.at(t.i)
	if r == laoSignNyo {
		// ຽ after a vowel: the final y as old spelling writes it, ຂາຽ.
		t.i++
		return v + "y"
	}
	if !laoConsonant(r) || t.anchored(t.i) || !need && t.starts(t.i) {
		return v
	}
	f, ok := laoFinals[r]
	cancel := t.at(t.i+1) == laoCancel
	switch {
	case cancel:
		// A loanword's letter under ໌ is said: ເບີຣ໌, beur.
		f, ok = laoInitial(r), true
	case !ok && !need:
		return v
	case !ok:
		f = laoInitial(r)
	}
	t.i++
	if cancel {
		t.i++
	}
	if f == "o" && strings.HasSuffix(v, "i") {
		// ວ after i is ou: ຄິວ, khiou.
		f = "ou"
	}
	t.tones()
	return v + f
}

// clusterWith tells whether the consonant at i, second after the initial,
// joins it, for a syllable with the leading vowel lead: when it takes the
// signs, or, after a leading vowel, signs that fit it.
func (t *laoText) clusterWith(i int, lead rune) bool {
	if !t.anchored(i) {
		return lead != 0
	}
	s := t.at(t.past(i + 1))
	return lead == 0 || !laoSign(s) || strings.ContainsRune(laoFits[lead], s)
}

// syllable reads the syllable at t.i.
func (t *laoText) syllable() string {
	lead := rune(0)
	if laoLeading(t.at(t.i)) {
		lead = t.at(t.i)
		t.i++
	}
	c := t.at(t.i)
	if !laoConsonant(c) {
		if lead != 0 {
			return map[rune]string{'ເ': "é", 'ແ': "è", 'ໂ': "ô", 'ໃ': "ai", 'ໄ': "ai"}[lead]
		}
		t.i++
		if v, ok := laoAlone[c]; ok {
			// A vowel without its consonant.
			return v
		}
		if laoSign(c) || c == laoCancel || c == 0x0EBA || c == 0x200C || c == 0x200D {
			return ""
		}
		return plain(c)
	}
	t.i++
	initial := laoInitial(c)
	next := t.at(t.i)
	switch {
	case next == laoSignLo:
		// ຼ is l, after ຫ alone: ຫຼາມ, lam.
		if c == laoHo {
			initial = ""
		}
		initial += "l"
		t.i++
	case c == laoHo && strings.ContainsRune(laoSonorants, next) && (next != laoWo || lead != 0 || t.anchored(t.i)):
		// ຫ before a sonorant only sets its tone, and ໜ, ໝ and ຫຼ write it
		// so: ຫລວງ, louang.
		initial = laoInitial(next)
		t.i++
		if t.at(t.i) == laoSignLo {
			initial += "l"
			t.i++
		}
	case next != 0 && strings.ContainsRune(laoClusters[c], next) && t.clusterWith(t.i, lead):
		// The second of a cluster is written, said or not: ພຣະ, phra.
		initial += laoInitial(next)
		t.i++
	case next == laoWo && strings.ContainsRune(laoWoFirsts, c) && (t.anchored(t.i) && t.clusterWith(t.i, lead) || lead != 0 && !t.anchored(t.i) && laoConsonant(t.at(t.i+1)) && !t.anchored(t.i+1) && laoFinals[t.at(t.i+1)] != ""):
		// ວ second is o: ສວາຍ, soay; ແຂວງ, khoèng.
		initial += "o"
		t.i++
	}
	t.tones()
	v := t.at(t.i)
	after := t.at(t.past(t.i + 1))
	switch lead {
	case 'ເ':
		switch {
		case v == 'າ' && after == 'ະ':
			t.i = t.past(t.i+1) + 1
			return initial + "o"
		case v == 'ົ' && after == 'າ':
			t.i = t.past(t.i+1) + 1
			return initial + "ao"
		case v == 'າ':
			// ເ-າ, ao, as old spelling writes ເ-ົາ.
			t.i++
			return initial + "ao"
		case v == 'ະ':
			t.i++
			return initial + "é"
		case v == 'ັ' && after == laoNyo:
			// ເ-ັຍ, ia, as old spelling writes it.
			t.i = t.past(t.i+1) + 1
			if t.at(t.i) == 'ະ' {
				t.i++
			}
			return initial + "ia"
		case v == 'ັ':
			t.i++
			return initial + t.final("é", true)
		case v == 'ິ' || v == 'ີ':
			t.i++
			t.tones()
			if t.at(t.i) == 'ະ' {
				t.i++
				return initial + "eu"
			}
			return initial + t.final("eu", false)
		case v == 'ຶ' || v == 'ື':
			t.i++
			t.tones()
			if t.at(t.i) == laoO && !t.anchored(t.i) {
				t.i++
				if t.at(t.i) == 'ະ' {
					t.i++
					return initial + "ua"
				}
				return initial + t.final("ua", false)
			}
			return initial + t.final("eu", false)
		case v == laoNyo && !t.anchored(t.i):
			t.i++
			return initial + "ia"
		}
		return initial + t.final("é", false)
	case 'ແ':
		switch v {
		case 'ະ':
			t.i++
			return initial + "è"
		case 'ັ':
			t.i++
			return initial + t.final("è", true)
		}
		return initial + t.final("è", false)
	case 'ໂ':
		if v == 'ະ' {
			t.i++
			return initial + "ô"
		}
		return initial + t.final("ô", false)
	case 'ໃ', 'ໄ':
		return initial + "ai"
	}
	switch v {
	case 'ະ':
		t.i++
		return initial + "a"
	case 'ັ':
		t.i++
		t.tones()
		switch r := t.at(t.i); {
		case r == laoWo && !t.anchored(t.i):
			t.i++
			return initial + t.final("oua", true)
		case r == laoO && !t.anchored(t.i):
			t.i++
			return initial + t.final("o", true)
		case r == laoSignNyo:
			t.i++
			return initial + t.final("ia", true)
		}
		return initial + t.final("a", true)
	case 'າ':
		t.i++
		return initial + t.final("a", false)
	case 'ຳ':
		t.i++
		return initial + "am"
	case 'ິ', 'ີ':
		t.i++
		return initial + t.final("i", false)
	case 'ຶ', 'ື':
		t.i++
		return initial + t.final("u", false)
	case 'ຸ', 'ູ':
		t.i++
		return initial + t.final("ou", false)
	case 'ົ':
		t.i++
		t.tones()
		if t.at(t.i) == laoWo && !t.anchored(t.i) {
			// ົວ, oua.
			t.i++
			t.tones()
			if t.at(t.i) == 'ະ' {
				t.i++
				return initial + "oua"
			}
			return initial + t.final("oua", false)
		}
		return initial + t.final("ô", true)
	case 0x0ECD:
		// ໍ, o; ໍາ, am, as ຳ.
		t.i++
		t.tones()
		if t.at(t.i) == 'າ' {
			t.i++
			return initial + "am"
		}
		return initial + "o"
	case laoSignNyo:
		t.i++
		return initial + t.final("ia", false)
	case laoO:
		if !t.anchored(t.i) && !t.starts(t.i) {
			t.i++
			return initial + t.final("o", false)
		}
	case laoWo:
		if !t.anchored(t.i) && laoConsonant(t.at(t.i+1)) && !t.anchored(t.i+1) {
			// ວ between consonants is oua: ຄວນ, khouan.
			t.i++
			return initial + t.final("oua", true)
		}
	}
	// No vowel written: a, as a consonant alone says it within a word; ô
	// before a final, as old spelling leaves out ົ.
	if n := t.bare(t.i); n%2 == 1 && laoFinals[t.at(t.i)] != "" {
		return initial + t.final("ô", true)
	}
	return initial + "a"
}

// laoAlone are the vowels written without their consonant.
var laoAlone = map[rune]string{
	'ະ': "a", 'ັ': "a", 'າ': "a", 'ຳ': "am", 'ິ': "i", 'ີ': "i", 'ຶ': "u", 'ື': "u", 'ຸ': "ou", 'ູ': "ou", 'ົ': "ô", 0x0ECD: "o", laoSignNyo: "ia",
}

func init() { register(lao, laoRun, "lo") }

// laoRun romanizes a run of Lao, a syllable apart.
func laoRun(run, _ string) []piece {
	t := &laoText{rs: []rune(run)}
	var b syllables
	end := 0
	for t.i < len(t.rs) {
		from := t.i
		switch r := t.rs[t.i]; {
		case r == laoRepeat:
			// ໆ says the syllable before again: ແຊວໆ, xèo xèo.
			t.i++
			end += bytesOf(t.rs[from:t.i])
			b.repeat(end)
		case r >= 0x0ED0 && r <= 0x0ED9 || r == 0x0EAF:
			t.i++
			end += bytesOf(t.rs[from:t.i])
			b.plain(end, plain(r))
		default:
			latin := t.syllable()
			if t.i == from {
				t.i++
			}
			end += bytesOf(t.rs[from:t.i])
			b.add(end, latin)
		}
	}
	return b.pieces
}
