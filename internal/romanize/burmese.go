package romanize

import "strings"

// Burmese is written by the BGN/PCGN system of 1970, the only system of
// romanization of Burmese the UN's working group knew of in 2013; Myanmar
// has no official one. It writes Burmese as it is said more than as it is
// spelled: a syllable's vowel and final together as their sound, ောင်
// "aung", ိန် "ein", ိုက် "aik"; its aspiration not at all, ခ as က, k; and
// k, s, t, p and ch voiced after a vowel or a nasal, as Burmese says them
// within a word: "မင်္ဂလာပါ" is "min ga la ba". Tones aren't written. The
// Myanmar Language Commission's system of 1980 writes the letters
// instead, "mranma" for မြန်မာ, which reads nothing like it is sung.
//
// Burmese writes no spaces between words, and burmeseRun parts a run into
// its syllables by the script's own marks: a consonant under ် ends the
// syllable before it, as one stacked over another with ္ does, its letter
// written after that syllable's vowel; each other consonant starts one.
// The syllables go a space apart, and the voicing goes on across them,
// as within a word. What only the word tells comes out wrong: ည် is i,
// though some words say it e or in, as ကျောက်ဆည်, Kyaukse, here "kyauk
// si"; voicing is written across words too, as of တင် in ကျေးဇူးတင်ပါတယ်,
// "kye zu din ba de", said tin; and a vowel said short is written as
// spelled, as of ပုသိမ်, Pathein, here "pu thein". Burmese typed in
// Zawgyi, the font encoding many sites still use, isn't Unicode's, and
// comes out wrong.

// burmeseLetters are the sounds of the consonants, from က: aspirated and
// plain alike; "_" for အ, a vowel's seat.
var burmeseLetters = strings.Fields("k k g g ng s s z z ny ny t t d d n t t d d n p p b b m y y l w th h l _")

// burmeseIndependent are the vowel letters, which carry their vowel alone,
// and ၏, which says e.
var burmeseIndependent = map[rune]string{
	'ဣ': "i", 'ဤ': "i", 'ဥ': "u", 'ဦ': "u", 'ဧ': "e", 'ဨ': "e", 'ဩ': "aw", 'ဪ': "aw", '၏': "e",
}

// burmeseWords are the signs that write a word: ၌, in, and ၍, and.
var burmeseWords = map[rune]string{'၌': "hnaik", '၍': "ywe"}

const (
	burmeseAsat, burmeseVirama = 0x103A, 0x1039
	burmeseDot, burmeseVisarga = 0x1037, 0x1038
	burmeseAnusvara            = 0x1036
	burmeseGreatSa             = 0x103F
)

func burmeseConsonant(r rune) bool { return r >= 0x1000 && r <= 0x1021 }
func burmeseTone(r rune) bool      { return r == burmeseDot || r == burmeseVisarga }

// burmeseVoiceable are the consonants said voiced after a vowel or a
// nasal.
const burmeseVoiceable = "ကခစဆဋဌတထပဖ"

// The ways a consonant can end a syllable, which burmeseText.ends tells.
const (
	burmeseOpen = iota
	burmeseAsatFinal
	burmeseKinzi
	burmeseStacked
)

// burmeseText is a run of Burmese, parsed a syllable at a time.
type burmeseText struct {
	rs []rune
	i  int
	// carry is a final that starts the next syllable too, its medial
	// after its ်, as က in ယောက်ျား, yauk kya.
	carry rune
}

func (t *burmeseText) at(i int) rune {
	if i < 0 || i >= len(t.rs) {
		return 0
	}
	return t.rs[i]
}

// ends tells how the consonant at i ends a syllable: under ်, and before
// ္ too as င is in kinzi; stacked over the next, with ္; or not at all.
func (t *burmeseText) ends(i int) int {
	c := t.at(i)
	if !burmeseConsonant(c) && c != 'ဥ' {
		return burmeseOpen
	}
	j := i + 1
	if t.at(j) == burmeseDot {
		j++
	}
	switch {
	case t.at(j) == burmeseAsat && t.at(j+1) == burmeseVirama:
		return burmeseKinzi
	case t.at(j) == burmeseAsat:
		return burmeseAsatFinal
	case t.at(i+1) == burmeseVirama && burmeseConsonant(c):
		return burmeseStacked
	}
	return burmeseOpen
}

// burmeseFinal is the sound class of a final consonant: k, ng, c, ny, nyin
// (ဉ), t (and p), n (and m), y, or "" for one said only as its vowel.
func burmeseFinal(c rune) string {
	switch {
	case strings.ContainsRune("ကခဂဃ", c):
		return "k"
	case c == 'င':
		return "ng"
	case strings.ContainsRune("စဆဇဈ", c):
		return "c"
	case c == 'ည':
		return "ny"
	case c == 'ဉ' || c == 'ဥ':
		return "nyin"
	case strings.ContainsRune("ဋဌဍဎတထဒဓပဖဗဘသ", c):
		return "t"
	case strings.ContainsRune("ဏနမ", c):
		return "n"
	case c == 'ယ':
		return "y"
	}
	return ""
}

// burmeseRhyme writes the vowel v of a syllable, its key as burmeseVowel
// makes it, with the final of the class final, after the consonant c; w is
// set for one with ွ, or ဝ, whose w the rhyme can hold, as in ကွတ်, kut.
func burmeseRhyme(v, final string, w bool, c rune) (rhyme string, holdsW bool) {
	open := map[string]string{"": "a", "a": "a", "i": "i", "u": "u", "e": "e", "è": "è", "aw": "aw", "o": "o"}
	switch final {
	case "n":
		switch {
		case w && v == "":
			return "un", true
		case v == "i":
			return "ein", false
		case v == "u" || v == "o":
			return "ôn", false
		}
		return "an", false
	case "t":
		switch {
		case w && v == "":
			return "ut", true
		case v == "i":
			return "eik", false
		case v == "u":
			return "ôk", false
		case v == "e":
			return "it", false
		case v == "o":
			return "aik", false
		}
		return "at", false
	case "k":
		switch v {
		case "o":
			return "aik", false
		case "aw":
			return "auk", false
		case "i":
			return "eik", false
		case "u":
			return "ôk", false
		}
		return "et", false
	case "ng":
		switch v {
		case "o":
			return "aing", false
		case "aw":
			return "aung", false
		case "i":
			return "ein", false
		case "u":
			return "ôn", false
		}
		return "in", false
	case "c":
		if v == "i" {
			return "eik", false
		}
		return "it", false
	case "ny":
		// ည် is i, but e after လ, as in လည်း, also.
		if c == 'လ' {
			return "e", false
		}
		return "i", false
	case "nyin":
		return "in", false
	case "y":
		// ယ် is è, but after a vowel only sets its tone: ကိုယ်, ko.
		if v == "" {
			return "è", false
		}
	}
	return open[v], false
}

// syllable reads the syllable at t.i: its initial, whether it can be
// voiced, its rhyme, and the letter of a consonant stacked after it.
func (t *burmeseText) syllable() (initial string, voiceable bool, rhyme, upper string) {
	c := t.at(t.i)
	var y, w, h bool
	v := ""
	ends := t.ends(t.i)
	switch {
	case t.carry != 0:
		// The final before starts this syllable too; its medials follow.
		c, t.carry = t.carry, 0
		initial = burmeseLetters[c-0x1000]
		voiceable = strings.ContainsRune(burmeseVoiceable, c)
	case ends == burmeseAsatFinal || ends == burmeseKinzi:
		// A final with no syllable before it: its rhyme alone.
	case burmeseIndependent[c] != "":
		t.i++
		if c == 'ဥ' && t.at(t.i) == 'ီ' {
			// ဦ, written as ဥ and ီ.
			t.i++
		}
		rhyme = burmeseIndependent[c]
		for burmeseTone(t.at(t.i)) {
			t.i++
		}
		if t.ends(t.i) == burmeseAsatFinal {
			// A vowel letter's final only sets its tone: ဧည့်, e.
			t.i++
			for burmeseTone(t.at(t.i)) || t.at(t.i) == burmeseAsat {
				t.i++
			}
		}
		if t.ends(t.i) == burmeseStacked {
			upper = burmeseLetters[t.at(t.i)-0x1000]
			t.i += 2
		}
		return "", false, rhyme, upper
	case burmeseWords[c] != "":
		t.i++
		return "", false, burmeseWords[c], ""
	case c == '၎':
		// ၎င်း, lagaung, written short.
		t.i++
		if t.at(t.i) == 'င' && t.at(t.i+1) == burmeseAsat {
			t.i += 2
			for burmeseTone(t.at(t.i)) {
				t.i++
			}
		}
		return "", false, "lagaung", ""
	case c == burmeseGreatSa:
		// ဿ is သ stacked under သ, said tth.
		t.i++
		initial = "tth"
	case burmeseConsonant(c):
		t.i++
		initial = burmeseLetters[c-0x1000]
		if initial == "_" {
			initial = ""
		}
		voiceable = strings.ContainsRune(burmeseVoiceable, c)
	case !burmeseConsonant(c):
		t.i++
		return "", false, plain(c), ""
	}
	// The medials: ျ and ြ, y; ွ, w; ှ, h.
medials:
	for ; ; t.i++ {
		switch t.at(t.i) {
		case 0x103B, 0x103C:
			y = true
		case 0x103D:
			w = true
		case 0x103E:
			h = true
		case 0x200C, 0x200D:
		default:
			break medials
		}
	}
	switch {
	case c == 'ခ' && y:
		// ချ and ခြ, ch.
		initial, y = "ch", false
	case h && (c == 'ရ' || y && (c == 'လ' || c == 'သ')):
		// ရှ, လျှ and သျှ, sh.
		initial, y, voiceable = "sh", false, false
	case h:
		initial, voiceable = "h"+initial, false
	}
	if y {
		initial += "y"
	}
	if c == 'ဝ' {
		w = true
	}
	// The vowel signs, and the tones, which aren't written.
	var e, i, u, aa, ai, nasal bool
vowels:
	for ; ; t.i++ {
		switch r := t.at(t.i); {
		case r == 0x1031:
			e = true
		case r == 0x102D || r == 0x102E:
			i = true
		case r == 0x102F || r == 0x1030:
			u = true
		case r == 0x102B || r == 0x102C:
			aa = true
		case r == 0x1032:
			ai = true
		case r == burmeseAnusvara:
			nasal = true
		case burmeseTone(r), r == 0x200C, r == 0x200D, r == burmeseVirama:
		case r == burmeseAsat && aa:
			// ော်, aw.
		default:
			break vowels
		}
	}
	switch {
	case e && aa:
		v = "aw"
	case i && u:
		v = "o"
	case e:
		v = "e"
	case i:
		v = "i"
	case u:
		v = "u"
	case aa:
		v = "a"
	case ai:
		v = "è"
	}
	final := ""
	if nasal {
		final = "n"
	}
	switch t.ends(t.i) {
	case burmeseAsatFinal:
		f := t.at(t.i)
		final = burmeseFinal(f)
		t.i++
		for burmeseTone(t.at(t.i)) || t.at(t.i) == burmeseAsat {
			t.i++
		}
		if r := t.at(t.i); r >= 0x103B && r <= 0x103E && burmeseConsonant(f) {
			// A medial after the final: it starts the next syllable too.
			t.carry = f
		}
	case burmeseKinzi:
		// Kinzi, င over the next consonant, ends this syllable: ဘင်္ဂလား,
		// bin ga la.
		final = "ng"
		t.i += 3
	}
	for burmeseTone(t.at(t.i)) {
		t.i++
	}
	if t.ends(t.i) == burmeseStacked {
		upper = burmeseLetters[t.at(t.i)-0x1000]
		if upper == "_" {
			upper = ""
		}
		t.i += 2
	}
	rhyme, holds := burmeseRhyme(v, final, w, c)
	if w && !holds && c != 'ဝ' {
		initial += "w"
	}
	return initial, voiceable, rhyme, upper
}

// burmeseVoiced writes an initial voiced: k g, s z, t d, p b, ch gy.
func burmeseVoiced(initial string) string {
	if strings.HasPrefix(initial, "ch") {
		return "gy" + initial[2:]
	}
	if initial == "" {
		return ""
	}
	v, ok := map[byte]string{'k': "g", 's': "z", 't': "d", 'p': "b"}[initial[0]]
	if !ok {
		return initial
	}
	return v + initial[1:]
}

func init() { register(myanmar, burmeseRun, "my") }

// burmeseRun romanizes a run of Burmese, a syllable apart.
func burmeseRun(run, _ string) []piece {
	t := &burmeseText{rs: []rune(run)}
	var b syllables
	end := 0
	// soft is set when the syllable before ends in a vowel or a nasal,
	// which voices the next.
	soft := false
	for t.i < len(t.rs) {
		from := t.i
		r := t.rs[t.i]
		if r >= 0x1040 && r <= 0x1049 || r == '၊' || r == '။' {
			t.i++
			end += bytesOf(t.rs[from:t.i])
			b.plain(end, plain(r))
			soft = false
			continue
		}
		initial, voiceable, rhyme, upper := t.syllable()
		if t.i == from {
			t.i++
		}
		end += bytesOf(t.rs[from:t.i])
		if voiceable && soft {
			initial = burmeseVoiced(initial)
		}
		latin := initial + rhyme + upper
		switch {
		case upper != "":
			// A stacked consonant voices the next only if it is a nasal:
			// ပဉ္စ, panyza; သေတ္တာ, thetta.
			soft = strings.ContainsAny(upper[:1], "nm")
		case latin == "":
		default:
			soft = !strings.HasSuffix(rhyme, "t") && !strings.HasSuffix(rhyme, "k") && rhyme != "" && !isPunct(rhyme)
		}
		b.add(end, latin)
	}
	return b.pieces
}

// isPunct tells whether s, plain's text, is no letter.
func isPunct(s string) bool {
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= 0xC0 {
			return false
		}
	}
	return true
}
