package romanize

import "strings"

// Khmer is written by the system the UN approved in 1972 (resolution
// II/10), BGN/PCGN's of 1972, which grew from the Service Géographique
// Khmère's of 1959: the system Cambodia's names are written by on maps
// made abroad and in ISO 3166-2, "ភ្នំពេញ" "phnum pénh", "កំពង់ចាម" "kâm
// póng cham". Cambodia's Geography Department has used a provisional
// system of its own since 1997, without marks, but it is published only
// as its differences from the UN's. The marks tell apart vowels a singer
// says apart: â and a, ô and o, ĕ and é.
//
// Each consonant is of one of two series, which read a vowel sign each
// their own way: ា is a after ក, éa after គ. A subscript consonant sets
// the series of the syllable, but for the nasals and យ រ ល វ ស, which
// leave it to the consonant above them; ៉ and ៊ change it. Khmer writes no
// spaces between words, and khmerRun parts a run into its syllables by
// the script's own structure instead, and writes them a space apart: a
// syllable is a consonant and those under it, or a vowel letter, then its
// vowel signs, then a final, a consonant without a vowel. A nasal, or a
// stop, over a subscript of a word of Pali or Sanskrit ends the syllable
// before it, and its subscript starts the next, as in កម្ពុជា, kâm pŭ
// chéa; a cluster that starts a word, as ស្រ, starts a syllable. Of
// consonants without vowels in a row after a syllable without a vowel
// sign, the first of an odd number ends it, as in កក, kâk, and an even
// number make syllables of their own, as in សកល, sâ kâl; after a vowel
// sign, one ends it. អ starting a syllable isn't written, as the standard
// allows at the start of a word.
//
// What only a dictionary knows comes out wrong: the open syllables of
// words of Pali and Sanskrit, as រតនគិរី, the standard's "rôtânôkiri",
// here "rô tân kĭ ri"; words said against their series, as វិហារ,
// "vihéar", here "vĭ har"; and a word's first syllable without a vowel
// sign after an open long vowel, which the rules take for its final, as
// in ស្រីរលូន, srei rô lun, here "sreir lun".

// khmerLetters are the sounds of the consonants, from ក, at the start of
// a syllable and at its end.
var khmerLetters = strings.Fields("k kh k kh ng ch chh ch chh nh d th d th n t th t th n b ph p ph m y r l v s s s h l '")

// khmerSecond are the consonants of the second series, of ô; the others
// are of the first, of â.
const khmerSecond = "គឃងជឈញឌឍទធនពភមយរលវ"

// khmerLight are the subscripts that leave the series to the consonant
// above them.
const khmerLight = "ងញណនមយរលវស"

// breve is U+0306, the combining breve of œ̆, which has no letter of its
// own.
const breve = "\xcc\x86"

// khmerVowels are the vowel signs, from ា, as each series reads them.
var khmerVowels = [16][2]string{
	{"a", "éa"}, {"ĕ", "ĭ"}, {"ei", "i"}, {"œ" + breve, "œ" + breve}, {"œ", "œ"}, {"ŏ", "ŭ"}, {"o", "u"}, {"uŏ", "uŏ"},
	{"aeu", "eu"}, {"œă", "œă"}, {"iĕ", "iĕ"}, {"é", "é"}, {"ê", "ê"}, {"ai", "ey"}, {"aô", "oŭ"}, {"au", "ŏu"},
}

// khmerIndependent are the vowel letters, which carry their vowel alone;
// ឧ is ŏ, though some words say it ŭ, and ឩ, which the UN's table leaves
// out, is ou, as Cambodia's system writes it.
var khmerIndependent = map[rune]string{
	'ឣ': "â", 'ឤ': "a", 'ឥ': "ĕ", 'ឦ': "ei", 'ឧ': "ŏ", 'ឨ': "ŏ", 'ឩ': "ou", 'ឪ': "âu", 'ឫ': "rœ" + breve,
	'ឬ': "rœ", 'ឭ': "lœ" + breve, 'ឮ': "lœ", 'ឯ': "ê", 'ឰ': "ai", 'ឱ': "aô", 'ឲ': "aô", 'ឳ': "au",
}

const (
	khmerCoeng                   = 0x17D2
	khmerNikahit, khmerReahmuk   = 0x17C6, 0x17C7
	khmerYuukaleapintu           = 0x17C8
	khmerMusikatoan, khmerTrisap = 0x17C9, 0x17CA
	khmerBantoc, khmerRobat      = 0x17CB, 0x17CC
	khmerAhsda, khmerSamyok      = 0x17CF, 0x17D0
	khmerRepeat                  = 0x17D7
)

func khmerConsonant(r rune) bool { return r >= 0x1780 && r <= 0x17A2 }

// khmerSeries is 1 for a consonant of the second series, 0 for the first.
func khmerSeries(r rune) int {
	if strings.ContainsRune(khmerSecond, r) {
		return 1
	}
	return 0
}

// khmerAkshara is a consonant or a vowel letter of a run, with what is
// written on it: the consonants under it, its vowel sign and its marks;
// or, base 0, another character.
type khmerAkshara struct {
	base, other rune
	subs        []rune
	vowel       rune
	// shift is ៉ or ៊, which change the series.
	shift                                                rune
	nikahit, reahmuk, yuuk, bantoc, robat, samyok, ahsda bool
	// mute is set under ៍, which the standard doesn't write, but which
	// tells a final.
	mute bool
	// baseEnd is where its base ends in the run, end where it does.
	baseEnd, end int
}

// anchored tells whether a has a vowel, sign or mark that makes it a
// syllable of its own, not a final.
func (a khmerAkshara) anchored() bool {
	return a.vowel != 0 || a.nikahit || a.reahmuk || a.yuuk || a.samyok || a.ahsda || khmerIndependent[a.base] != ""
}

// plainConsonant tells whether a is a consonant alone, which can end a
// syllable.
func (a khmerAkshara) plainConsonant() bool {
	return khmerConsonant(a.base) && len(a.subs) == 0 && !a.anchored()
}

// khmerAksharas parses a run into its consonants and vowel letters, each
// with what is written on it.
func khmerAksharas(run string) []khmerAkshara {
	var as []khmerAkshara
	coeng := false
	for i, r := range run {
		end := i + len(string(r))
		var a *khmerAkshara
		if len(as) > 0 && as[len(as)-1].base != 0 {
			a = &as[len(as)-1]
		}
		switch {
		case coeng && a != nil && khmerConsonant(r):
			a.subs = append(a.subs, r)
			coeng = false
		case khmerConsonant(r) || khmerIndependent[r] != "":
			as = append(as, khmerAkshara{base: r, baseEnd: end})
			coeng = false
		case r == khmerCoeng:
			coeng = true
		case r >= 0x17B4 && r <= 0x17D3 || r == 0x17DD || r == 0x200C || r == 0x200D:
			// Signs and marks: those of a consonant, else none.
			if a == nil {
				break
			}
			switch {
			case r >= 0x17B6 && r <= 0x17C5:
				if a.vowel == 0 {
					a.vowel = r
				}
			case r == khmerNikahit:
				a.nikahit = true
			case r == khmerReahmuk:
				a.reahmuk = true
			case r == khmerYuukaleapintu:
				a.yuuk = true
			case r == khmerMusikatoan || r == khmerTrisap:
				a.shift = r
			case r == khmerBantoc:
				a.bantoc = true
			case r == khmerRobat:
				a.robat = true
			case r == khmerAhsda:
				a.ahsda = true
			case r == khmerSamyok:
				a.samyok = true
			case r == 0x17CD:
				a.mute = true
			}
		default:
			as = append(as, khmerAkshara{other: r})
			coeng = false
		}
		if len(as) > 0 {
			as[len(as)-1].end = end
		}
	}
	return as
}

// khmerInitial writes the consonants that start a syllable, cons, and
// tells its series: that of the first, or of the last subscript that sets
// it, as shift changes it. above is the consonant over cons[0], which
// ended the syllable before; 0 for none.
func khmerInitial(cons []rune, shift, above rune) (string, int) {
	series := khmerSeries(cons[0])
	var b strings.Builder
	for k, c := range cons {
		l := khmerLetters[c-0x1780]
		over := above
		if k > 0 {
			over = cons[k-1]
		}
		switch {
		case c == 'អ' && k == 0:
			l = ""
		case c == 'ប' && k == 0 && (len(cons) > 1 || shift == khmerMusikatoan):
			// ប with a subscript or ៉ is p: ប្រាប់, prăb.
			l = "p"
		case c == 'ត' && over != 0 && k == len(cons)-1 && !strings.ContainsRune(khmerNasals, over):
			// ត's subscript is written as ដ's, and usually is ដ: ក្តី,
			// kdei; but under a nasal or over another subscript it is t,
			// as the t of note 3's own exception, កន្ត្រ…, kântr….
			l = "d"
		}
		if k > 0 && !strings.ContainsRune(khmerLight, c) {
			series = khmerSeries(c)
		}
		b.WriteString(l)
	}
	switch shift {
	case khmerMusikatoan:
		series = 0
	case khmerTrisap:
		series = 1
	}
	return b.String(), series
}

// khmerFinal writes the consonant a as a syllable's final, its robat as r
// before it, and, with subs set, the subscripts under it: អង្គ, 'ângk.
func khmerFinal(a khmerAkshara, subs bool) string {
	l := khmerLetters[a.base-0x1780]
	if a.base == 'អ' {
		l = ""
	}
	if a.robat {
		l = "r" + l
	}
	if subs {
		for _, c := range a.subs {
			l += khmerLetters[c-0x1780]
		}
	}
	return l
}

// khmerPair tells whether the consonant base, over sub, is the final of
// a syllable before sub's, after a syllable with a vowel sign: a nasal, a
// doubled consonant, a stop over another of its row, or a stop over យ, as
// in words of Pali and Sanskrit, ពិន្ទុ, វិទ្យា; where a cluster that
// starts a word, as ស្អ in ស្រីស្អាត, isn't.
func khmerPair(base, sub rune) bool {
	row := func(r rune) int { return int(r-0x1780) / 5 }
	stop := func(r rune) bool { return r >= 0x1780 && r < 0x1799 && int(r-0x1780)%5 != 4 }
	return strings.ContainsRune(khmerNasals, base) || base == sub || stop(base) && (stop(sub) && row(base) == row(sub) || sub == 'យ')
}

// khmerNasals are the nasal consonants.
const khmerNasals = "ងញណនម"

// khmerChain counts the consonants alone at the start of as, up to one
// that ់ or ៌ tells is a final, and one with a subscript after them whose
// consonant can end a syllable too.
func khmerChain(as []khmerAkshara) int {
	n := 0
	for n < len(as) && as[n].plainConsonant() {
		n++
		if as[n-1].bantoc || as[n-1].robat {
			return n
		}
	}
	if n < len(as) && khmerConsonant(as[n].base) && len(as[n].subs) > 0 && khmerPair(as[n].base, as[n].subs[0]) {
		n++
	}
	return n
}

// khmerSyllable is a syllable of Khmer: its initial, as written, and its
// series; its vowel letter's vowel, if it is one; the akshara of its vowel
// sign and marks; and its final, which bantoc shortens.
type khmerSyllable struct {
	initial string
	series  int
	ind     string
	a       khmerAkshara
	final   string
	bantoc  bool
}

// bare tells whether s has no vowel written.
func (s khmerSyllable) bare() bool { return !s.a.anchored() }

// closes tells whether s can take a final: not after ំ, ះ, ៈ or ៏, nor
// after ៃ and ៅ, which end in a glide, as in ទៅរក, tŏu rôk.
func (s khmerSyllable) closes() bool {
	return !s.a.nikahit && !s.a.reahmuk && !s.a.yuuk && !s.a.ahsda && s.a.vowel != 'ៃ' && s.a.vowel != 'ៅ'
}

// text writes s, its vowel as its series reads it.
func (s khmerSyllable) text() string {
	a := s.a
	pick := func(first, second string) string {
		if s.series == 1 {
			return second
		}
		return first
	}
	// shortened is the second series' short a: eă before k, ng and h, oă
	// else.
	shortened := func() string {
		if strings.HasPrefix(s.final, "k") || strings.HasPrefix(s.final, "ng") || strings.HasPrefix(s.final, "h") {
			return "eă"
		}
		return "oă"
	}
	var v string
	switch {
	case s.ind != "":
		v = s.ind
		if a.nikahit {
			v += "m"
		}
		if a.reahmuk {
			v += "h"
		}
	case a.vowel == 0:
		switch {
		case a.nikahit:
			v = pick("âm", "um")
		case a.reahmuk:
			v = pick("ăh", "eăh")
		case a.yuuk:
			v = pick("ă", "eă")
		case a.samyok:
			v = pick("ă", shortened())
		case s.bantoc && s.final != "":
			v = pick("á", "ó")
		default:
			v = pick("â", "ô")
		}
	case a.vowel == 'ា':
		switch {
		case a.nikahit && s.final == "ng":
			return s.initial + pick("ăng", "eăng")
		case a.nikahit:
			v = pick("ăm", "ŏâm")
		case s.bantoc && s.final != "":
			v = pick("ă", shortened())
		default:
			v = pick("a", "éa")
			if a.reahmuk {
				v += "h"
			}
		}
	case a.vowel == 'ុ' && a.nikahit:
		v = pick("om", "ŭm")
	case a.vowel == 'ុ' && a.reahmuk:
		v = pick("ŏh", "ŭh")
	case a.vowel == 'ោ' && a.reahmuk:
		v = pick("aôh", "ŏăh")
	default:
		v = khmerVowels[a.vowel-0x17B6][s.series]
		if a.nikahit {
			v += "m"
		}
		if a.reahmuk {
			v += "h"
		}
	}
	return s.initial + v + s.final
}

func init() { register(khmer, khmerRun, "km") }

// khmerRun romanizes a run of Khmer, a syllable apart.
func khmerRun(run, _ string) []piece {
	as := khmerAksharas(run)
	var b syllables
	// split is set when the consonant of as[i] ended the syllable before:
	// as[i]'s syllable starts at its subscripts.
	split := false
	for i := 0; i < len(as); {
		a := as[i]
		if a.base == 0 {
			if a.other == khmerRepeat {
				// ៗ says the syllable before again.
				b.repeat(a.end)
			} else {
				b.plain(a.end, plain(a.other))
			}
			i++
			split = false
			continue
		}
		s := khmerSyllable{a: a}
		if ind := khmerIndependent[a.base]; ind != "" {
			s.ind = ind
		} else {
			cons, above := append([]rune{a.base}, a.subs...), rune(0)
			if split {
				cons, above = a.subs, a.base
			}
			s.initial, s.series = khmerInitial(cons, a.shift, above)
		}
		end := a.end
		split = false
		i++
		if i < len(as) && khmerConsonant(as[i].base) {
			n := as[i]
			switch {
			case a.vowel == 'ា' && a.nikahit && n.base == 'ង' && n.plainConsonant():
				// ាំង, ăng.
				s.final = "ng"
				end = n.end
				i++
			case !s.closes():
			case len(n.subs) > 0 && (khmerPair(n.base, n.subs[0]) || a.samyok && !n.anchored()):
				// The consonant ends this syllable, and its subscript
				// starts the next: កម្ពុជា, kâm pŭ chéa. ័ takes a final
				// whatever it is: ភ័ព្វ, phoăpv.
				s.final = khmerFinal(n, false)
				end = n.baseEnd
				split = true
				if !n.anchored() && !(i+1 < len(as) && as[i+1].plainConsonant()) {
					// A subscript without a vowel, before no consonant it
					// could end, ends this syllable too: អង្គ, 'ângk.
					s.final = khmerFinal(n, true)
					s.bantoc = n.bantoc
					end = n.end
					split = false
					i++
				}
			case n.plainConsonant():
				// Of consonants alone in a row after a syllable without a
				// vowel sign, the first of an odd number ends it, and an
				// even number make syllables of their own: កក, kâk; សកល,
				// sâ kâl. After a vowel sign, one ends the syllable, as in
				// នឹករលឹក, nœ̆k rô lœ̆k, but two left at the end make one
				// of their own: សាកល, sa kâl.
				take := khmerChain(as[i:])%2 == 1
				if !s.bare() {
					take = i+1 == len(as) || !as[i+1].plainConsonant() || i+2 < len(as) && as[i+2].base != 0
				}
				if n.base == 'ហ' && !n.mute && i+1 < len(as) && as[i+1].base != 0 {
					// ហ ends a syllable only at the end of a word, as in
					// ស្នេហ៍; before a syllable it starts one: សីហនុ, sei hâ
					// nŭ.
					take = false
				}
				if take {
					s.final, s.bantoc = khmerFinal(n, false), n.bantoc
					end = n.end
					i++
				}
			}
		}
		b.add(end, s.text())
	}
	return b.pieces
}
