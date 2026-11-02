package romaji

import (
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/ikawaha/kagome-dict/dict"
	"github.com/ikawaha/kagome/v2/tokenizer"
	"golang.org/x/text/unicode/norm"
)

// The features of the words of the IPA dictionary, as kagome's tokens
// have them: their parts of speech, from the widest, then how they
// inflect, the form found in a dictionary, and their reading and their
// pronunciation, in katakana.
const (
	featurePOS, featurePOS2        = 0, 1
	featureInflection, featureBase = 4, 6
	featureReading, featureSpoken  = 7, 8
)

// Romanizer writes lines of Japanese in romaji, its kanji read as the IPA
// dictionary reads them, word by word. It is safe for concurrent use.
type Romanizer struct{ t *tokenizer.Tokenizer }

// NewRomanizer returns a Romanizer reading words in d, the IPA dictionary
// of kagome, as dict.LoadDictFile loads it.
func NewRomanizer(d *dict.Dict) (*Romanizer, error) {
	t, err := tokenizer.New(d, tokenizer.OmitBosEos())
	if err != nil {
		return nil, err
	}
	return &Romanizer{t}, nil
}

// Line writes a line of lyrics in romaji, word by word as romaji is
// written: "今日は良い天気" is "kyō wa yoi tenki". The particles は, へ and
// を are wa, e and o, the endings of verbs join them (食べた, tabeta), and
// long vowels take a macron. Latin letters, digits and punctuation stay as
// they are, Japanese punctuation as its Latin kin (、 is a comma). A line
// with nothing Japanese in it is "".
func (r *Romanizer) Line(s string) string { return line(s, r.words) }

// KanaLine writes the kana of a line in romaji, as Kana does, and leaves
// its kanji as they are, a word apart: what romaji is without the
// dictionary. A line with nothing Japanese in it is "".
func KanaLine(s string) string { return line(s, kanaWords) }

// line writes s in romaji: its parts in Japanese as words writes them, the
// rest as it is, with a space between a word in romaji and one in Latin
// letters that touched.
func line(s string, words func(ja string) string) string {
	if !Japanese(s) {
		return ""
	}
	s = norm.NFKC.String(s)
	var b strings.Builder
	for len(s) > 0 {
		// The part of s up to where it turns Japanese, or turns back.
		ja := japaneseRune(firstRune(s))
		end := strings.IndexFunc(s, func(r rune) bool { return japaneseRune(r) != ja })
		if end < 0 {
			end = len(s)
		}
		part := s[:end]
		s = s[end:]
		if ja {
			part = words(part)
		}
		if part == "" {
			continue
		}
		if b.Len() > 0 && spaced(lastRune(b.String()), firstRune(part)) {
			b.WriteByte(' ')
		}
		b.WriteString(part)
	}
	return strings.TrimSpace(b.String())
}

// japaneseRune tells the runes a part of a line in Japanese holds: kana,
// kanji, and the punctuation of Japanese, which the dictionary knows.
func japaneseRune(r rune) bool {
	return IsKana(r) || unicode.Is(unicode.Han, r) || r >= 0x3000 && r <= 0x303f || r == '・'
}

// spaced tells whether a space goes between what ends with a and what
// starts with b, Japanese in romaji on one side, the rest on the other:
// between two words, but not before punctuation, nor after an opening
// bracket.
func spaced(a, b rune) bool {
	word := func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }
	return (word(a) || closing(a)) && (word(b) || opening(b))
}

func firstRune(s string) rune { r, _ := utf8.DecodeRuneInString(s); return r }
func lastRune(s string) rune  { r, _ := utf8.DecodeLastRuneInString(s); return r }

// punctuation is the Latin kin of the punctuation of Japanese.
var punctuation = map[string]string{
	"、": ",", "。": ".", "「": "“", "」": "”", "『": "“", "』": "”",
	"【": "[", "】": "]", "〔": "(", "〕": ")", "〈": "<", "〉": ">", "《": "«", "》": "»",
	"〜": "~", "〝": "“", "〟": "”", "・": " ", "　": " ",
}

func opening(r rune) bool { return strings.ContainsRune("“‘([{<«", r) }
func closing(r rune) bool { return strings.ContainsRune("”’)]}>»,.!?:;", r) }

// words writes the words of a part of a line in Japanese in romaji, a
// piece at a time, spaced as romaji is.
type words struct {
	b strings.Builder
	// kana is the katakana of the word under way, not yet in romaji, and
	// glue set after an opening bracket, which the next word goes on.
	kana []rune
	glue bool
}

// flush writes the word under way.
func (w *words) flush() {
	if len(w.kana) > 0 {
		w.b.WriteString(kana(w.kana))
		w.kana = w.kana[:0]
	}
}

// word starts a word: after a space, but at the start and after an
// opening bracket.
func (w *words) word() {
	w.flush()
	if w.b.Len() > 0 && !w.glue {
		w.b.WriteByte(' ')
	}
	w.glue = false
}

// literal writes a word as it is, as kanji whose reading is unknown.
func (w *words) literal(s string) {
	w.word()
	w.b.WriteString(s)
}

// symbol writes punctuation as its Latin kin: an opening bracket before
// the next word, other punctuation after the word before.
func (w *words) symbol(s string) {
	for _, r := range s {
		p, ok := punctuation[string(r)]
		if !ok {
			p = string(r)
		}
		switch {
		case strings.TrimSpace(p) == "":
			w.flush()
		case opening(firstRune(p)):
			w.word()
			w.b.WriteString(p)
			w.glue = true
		default:
			w.flush()
			w.b.WriteString(p)
			w.glue = false
		}
	}
}

func (w *words) String() string {
	w.flush()
	return strings.TrimSpace(w.b.String())
}

// kanaWords writes a part of a line in Japanese as KanaLine does: each run
// of kana in romaji, each of kanji as it is, a word apart.
func kanaWords(s string) string {
	var w words
	for len(s) > 0 {
		r := firstRune(s)
		kind := runeKind(r)
		end := len(string(r))
		if kind != 0 {
			if i := strings.IndexFunc(s, func(r rune) bool { return runeKind(r) != kind }); i > 0 {
				end = i
			} else if i < 0 {
				end = len(s)
			}
		}
		part := s[:end]
		s = s[end:]
		switch kind {
		case 'k':
			w.word()
			w.kana = append(w.kana, guessLong(katakana(part))...)
		case 'h':
			w.literal(part)
		default:
			w.symbol(part)
		}
	}
	return w.String()
}

// runeKind tells kana, 'k', from kanji, 'h', and from the rest, 0.
func runeKind(r rune) rune {
	switch {
	case IsKana(r):
		return 'k'
	case unicode.Is(unicode.Han, r):
		return 'h'
	}
	return 0
}

// words writes a part of a line in Japanese in romaji, as Line does.
func (r *Romanizer) words(s string) string {
	var w words
	// prev are the features of the word before, nil after punctuation.
	var prev []string
	for _, t := range r.t.Tokenize(s) {
		switch {
		case !Japanese(t.Surface):
			w.symbol(t.Surface)
			prev = nil
			continue
		case strings.Trim(t.Surface, "ーｰ") == "":
			// Long vowel marks alone lengthen the word before.
			w.kana = append(w.kana, katakana(t.Surface)...)
			continue
		}
		f := t.Features()
		k := reading(t.Surface, f)
		switch {
		case k == nil:
			w.literal(t.Surface)
			prev = nil
			continue
		case !joins(f, prev, t.Surface):
			w.word()
		case f[featurePOS] == "助動詞" && string(k) == "ウ" && len(w.kana) > 0:
			// The う of will lengthens the o before it: 行こう, ikō.
			if syl, _ := syllable(w.kana, len(w.kana)-1); vowelOf(syl) == 'o' {
				k = []rune{'ー'}
			}
		}
		w.kana = append(w.kana, k...)
		prev = f
	}
	return w.String()
}

// particles are the particles kana write otherwise than they sound.
var particles = map[string]string{"は": "ワ", "へ": "エ", "を": "オ"}

// reading returns the reading of a word in katakana, its long vowels
// marked with ー where its pronunciation has them, as ou of 東京, tōkyō,
// but not that of 思う, omou; nil when the dictionary has none and its
// surface is not kana alone.
func reading(surface string, f []string) []rune {
	if len(f) > featurePOS && f[featurePOS] == "助詞" {
		if k, ok := particles[surface]; ok {
			return []rune(k)
		}
	}
	if len(f) <= featureSpoken || f[featureReading] == "*" || f[featureReading] == "" {
		if strings.ContainsFunc(surface, func(r rune) bool { return !IsKana(r) }) {
			return nil
		}
		return guessLong(katakana(surface))
	}
	read, spoken := []rune(f[featureReading]), []rune(f[featureSpoken])
	if len(read) != len(spoken) {
		return guessLong(read)
	}
	for i := 1; i < len(read); i++ {
		switch {
		case spoken[i] == 'ー' && read[i] != 'ー':
			// ou, oo, uu, aa and ee take a macron; ii and ei stay.
			syl, _ := syllable(read, i-1)
			if long(vowelOf(syl), read[i]) {
				read[i] = 'ー'
			}
		case read[i] == 'ハ' && spoken[i] == 'ワ', read[i] == 'ヘ' && spoken[i] == 'エ':
			// こんにちは, konnichiwa.
			read[i] = spoken[i]
		}
	}
	return read
}

// long tells whether the kana k after the vowel v lengthens it, in the
// words of the dictionary whose pronunciation makes them one long vowel.
func long(v, k rune) bool {
	switch k {
	case 'ウ', 'ゥ':
		return v == 'o' || v == 'u'
	case 'オ', 'ォ':
		return v == 'o'
	case 'ア', 'ァ':
		return v == 'a'
	case 'エ', 'ェ':
		return v == 'e'
	}
	return false
}

// joins tells whether a word of the features f goes on the word before,
// of the features prev, without a space, as romaji writes them: endings
// and auxiliaries on their verb (食べた, tabeta; 食べて, tabete; 愛してる,
// aishiteru), suffixes on their noun (君たち, kimitachi), and what follows
// a prefix. Particles and the rest are words of their own.
func joins(f, prev []string, surface string) bool {
	if prev == nil || len(f) <= featurePOS2 || len(prev) <= featurePOS2 {
		return false
	}
	pos, sub, before := f[featurePOS], f[featurePOS2], prev[featurePOS]
	inflects := before == "動詞" || before == "形容詞" || before == "助動詞"
	switch {
	case before == "接頭詞":
		return true
	case sub == "接尾":
		return true
	case pos == "助動詞":
		// But the copula, as でしょう in 晴れるでしょう, hareru deshō, and
		// not the だ of the past, as in 飲んだ, nonda.
		return inflects && (len(f) <= featureBase || !slices.Contains([]string{"特殊・ダ", "特殊・デス"}, f[featureInflection]) && f[featureBase] != "らしい")
	case pos == "助詞" && sub == "接続助詞":
		return inflects && slices.Contains([]string{"て", "で", "ば", "ちゃ", "じゃ"}, surface)
	case pos == "動詞" && sub == "非自立":
		// The contractions of て and a verb: てる, ちゃう, とく.
		return inflects && strings.ContainsRune("てでちじとど", firstRune(surface))
	}
	return false
}
