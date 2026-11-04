// Package romanize writes lines of lyrics in Latin letters on the
// computer, script by script, each by the official or the most used
// standard of its language, as it reads for singing along:
//
//   - Korean: the Revised Romanization of Korean (2000), with its sound
//     changes within a word (hangul.go);
//   - Chinese: Hanyu Pinyin with tone marks, simplified and traditional,
//     a polyphone read by the word it is in (pinyin.go);
//   - Cyrillic, by language: Russian by BGN/PCGN, Ukrainian by its national
//     system of 2010, Belarusian by its national system of 2007, Bulgarian
//     by the Streamlined System, Serbian in Gaj's Latin alphabet,
//     Macedonian by its official system of 2008, Kazakh and Kyrgyz by
//     BGN/PCGN (1979) and Mongolian by MNS 5217:2012 (cyrillic.go);
//   - Greek: ELOT 743, as the UN adopted it (greek.go);
//   - Armenian by BGN/PCGN (1981), and Georgian by its national system of
//     2002 (armenian.go, georgian.go);
//   - the Brahmic scripts of India and Sri Lanka, by one engine over their
//     aligned blocks: Hunterian, India's national system, with its long
//     vowels marked (brahmic.go);
//   - Thai: the Royal Thai General System, syllable by syllable (thai.go);
//   - Lao: BGN/PCGN (1966), syllable by syllable (lao.go);
//   - Tibetan: Extended Wylie (tibetan.go);
//   - Hebrew by the rules of the Academy of the Hebrew Language (2006),
//     and Arabic, Persian and Urdu by the plain form of ALA-LC's (hebrew.go,
//     arabic.go);
//   - Amharic's Ethiopic by BGN/PCGN (1967) (ethiopic.go).
//
// Each run of a script is romanized apart, so that the Latin letters of a
// line, as the English of a line of K-pop, stay as they are. Japanese is
// romaji's (internal/romaji), which needs a dictionary: kana stay as they
// are here. Nothing is downloaded; Chinese reads its data, built in,
// the first time.
package romanize

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Line writes a line of lyrics in the language lang, a code of ISO 639-1
// as "ko", or "" when it isn't known, in Latin letters: "사랑해 baby" is
// "saranghae baby". It is "" for a line with nothing it romanizes.
func Line(s, lang string) string { return romanize(s, lang).text }

// Words writes a line of lyrics in Latin letters, as Line does, and each
// of its words, those of a line synced word by word, as the part of the
// line in Latin letters it became: a word is found in the line after the
// one before, as lyrics time them. A word romanized with the next, as a
// Korean consonant that goes on the next syllable, gives it that part.
func Words(s, lang string, words []string) (string, []string) {
	r := romanize(s, lang)
	if r.text == "" {
		return "", nil
	}
	out := make([]string, len(words))
	from := 0
	for i, w := range words {
		at := strings.Index(s[from:], w)
		if at < 0 {
			break
		}
		start, end := from+at, from+at+len(w)
		out[i] = strings.TrimSpace(r.text[r.outAt(s, start):r.outAt(s, end)])
		from = end
	}
	return r.text, out
}

// Romanizes reports whether s holds letters of a script Line romanizes in
// lyrics of the language lang: of the script lang is written in, when
// Line knows it, as Hangul for "ko"; of any it romanizes, else.
func Romanizes(s, lang string) bool {
	own, known := scriptsOf[lang]
	for _, r := range s {
		if k := scriptOf(r, lang); romanizers[k] != nil && (!known || k == own) {
			return true
		}
	}
	return false
}

// script is a script Line romanizes, latin for what it keeps as it is, and
// mark for what goes with the letter before, as a vowel sign.
type script uint8

const (
	latin script = iota
	mark
	hangul
	han
	cyrillic
	greek
	armenian
	georgian
	brahmic
	sinhala
	thai
	lao
	tibetan
	hebrew
	arabic
	ethiopic
)

// scriptOf tells the script of r. Han is Chinese but in lyrics of Japanese,
// Korean or Vietnamese, which write it otherwise.
func scriptOf(r rune, lang string) script {
	switch {
	case r < 0x80:
		return latin
	case r == 0x200C || r == 0x200D || unicode.In(r, unicode.Mn, unicode.Me):
		return mark
	case unicode.Is(unicode.Hangul, r):
		return hangul
	case unicode.Is(unicode.Han, r):
		if lang == "ja" || lang == "ko" || lang == "vi" {
			return latin
		}
		return han
	case unicode.Is(unicode.Cyrillic, r):
		return cyrillic
	case unicode.Is(unicode.Greek, r):
		return greek
	case unicode.Is(unicode.Armenian, r):
		return armenian
	case unicode.Is(unicode.Georgian, r):
		return georgian
	case r == 0x0964 || r == 0x0965:
		// The dandas, which all the scripts of India write.
		return latin
	case r >= 0x0900 && r < 0x0D80:
		return brahmic
	case unicode.Is(unicode.Sinhala, r):
		return sinhala
	case unicode.Is(unicode.Thai, r):
		return thai
	case unicode.Is(unicode.Lao, r):
		return lao
	case unicode.Is(unicode.Tibetan, r):
		return tibetan
	case unicode.Is(unicode.Hebrew, r):
		return hebrew
	case unicode.Is(unicode.Arabic, r):
		return arabic
	case unicode.Is(unicode.Ethiopic, r):
		return ethiopic
	case unicode.IsMark(r):
		return mark
	}
	return latin
}

// piece is a part of a run romanized: the run's bytes up to end made text.
// A piece whose text starts with a space is a word of its own.
type piece struct {
	end  int
	text string
}

// romanizers romanize a run of their script in lyrics of a language:
// each script's file adds its own (register).
var romanizers = map[script]func(run, lang string) []piece{}

// lineLanguages tell, of the scripts several languages write, which one a
// line of lyrics in the language lang is in, by all its letters, as
// Ukrainian's і and ї tell it.
var lineLanguages = map[script]func(line, lang string) string{}

// scriptsOf are the scripts of the languages Line romanizes, by their
// codes.
var scriptsOf = map[string]script{}

// register adds the romanizer of a script, and the languages written in
// it, by their codes.
func register(k script, f func(run, lang string) []piece, langs string) {
	romanizers[k] = f
	for _, lang := range strings.Fields(langs) {
		scriptsOf[lang] = k
	}
}

// romanized is a line in Latin letters, with where each part of it came
// from: the line's bytes up to src[i] made those of text up to out[i].
type romanized struct {
	text     string
	src, out []int
}

// outAt returns where the part of r made of the bytes of s before at
// ends, at a piece's end, or as far into the piece at is in as at is,
// by its runes.
func (r romanized) outAt(s string, at int) int {
	prevSrc, prevOut := 0, 0
	for i, end := range r.src {
		if at > end {
			prevSrc, prevOut = end, r.out[i]
			continue
		}
		if at == end {
			return r.out[i]
		}
		part := r.text[prevOut:r.out[i]]
		n := utf8.RuneCountInString(part)
		k := n * utf8.RuneCountInString(s[prevSrc:at]) / max(utf8.RuneCountInString(s[prevSrc:end]), 1)
		off := prevOut
		for j := 0; j < k; j++ {
			_, size := utf8.DecodeRuneInString(r.text[off:])
			off += size
		}
		return off
	}
	return len(r.text)
}

// romanize writes s in Latin letters, run by run of its scripts: those
// several languages write in the one the whole line's letters tell, when
// lang isn't one written in them.
func romanize(s, lang string) romanized {
	langOf := map[script]string{}
	var (
		r    romanized
		b    strings.Builder
		some bool
		// was tells whether the run before was romanized, and prev its
		// script.
		was  bool
		prev script
	)
	for start := 0; start < len(s); {
		first, _ := utf8.DecodeRuneInString(s[start:])
		k := scriptOf(first, lang)
		end := start + len(string(first))
		for end < len(s) {
			c, size := utf8.DecodeRuneInString(s[end:])
			if ck := scriptOf(c, lang); ck != k && (ck != mark || k == latin) {
				// An apostrophe between two letters of a script is its
				// own, as in the Ukrainian м'ята.
				after, _ := utf8.DecodeRuneInString(s[end+size:])
				if k == latin || !strings.ContainsRune("'ʼ’", c) || scriptOf(after, lang) != k {
					break
				}
			}
			end += size
		}
		run := s[start:end]
		var pieces []piece
		f := romanizers[k]
		if f != nil {
			some = true
			runLang := lang
			if lf := lineLanguages[k]; lf != nil {
				if _, ok := langOf[k]; !ok {
					langOf[k] = lf(s, lang)
				}
				runLang = langOf[k]
			}
			pieces = f(run, runLang)
		} else {
			for i, c := range run {
				p := plain(c)
				if c == ';' && (lang == "el" || i == 0 && prev == greek) {
					// Greek's question mark, as it is once normalized.
					p = "?"
				}
				pieces = append(pieces, piece{i + len(string(c)), p})
			}
		}
		for i, p := range pieces {
			text := p.text
			if i == 0 && (f != nil || was) {
				// A word in Latin letters goes a space apart from one
				// of another run that it touches.
				if f != nil {
					text = strings.TrimLeft(text, " ")
				}
				first, _ := utf8.DecodeRuneInString(text)
				last, _ := utf8.DecodeLastRuneInString(b.String())
				if b.Len() > 0 && text != "" && spaced(last, first) {
					text = " " + text
				}
			}
			b.WriteString(text)
			r.src = append(r.src, start+p.end)
			r.out = append(r.out, b.Len())
		}
		was, prev = f != nil, k
		start = end
	}
	if !some {
		return romanized{}
	}
	return tidy(r, b.String())
}

// tidy is r with its text as text, its spaces one at a time and none at
// its ends, nor before punctuation that closes.
func tidy(r romanized, text string) romanized {
	var b strings.Builder
	keep := make([]int, len(text)+1)
	for i := 0; i < len(text); i++ {
		keep[i] = b.Len()
		c := text[i]
		if c == ' ' {
			next, _ := utf8.DecodeRuneInString(text[i+1:])
			if b.Len() == 0 || strings.HasSuffix(b.String(), " ") || i+1 == len(text) || next == ' ' || closing(next) && next != '”' {
				continue
			}
		}
		b.WriteByte(c)
	}
	keep[len(text)] = b.Len()
	out := strings.TrimRight(b.String(), " ")
	r.text = out
	for i, o := range r.out {
		r.out[i] = min(keep[o], len(out))
	}
	return r
}

// spaced tells whether a space goes between what ends with a and what
// starts with b, which touched: between two words, but not before
// punctuation, nor after an opening bracket.
func spaced(a, b rune) bool {
	word := func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }
	return (word(a) || closing(a)) && (word(b) || opening(b))
}

func opening(r rune) bool { return strings.ContainsRune("“‘([{<«¿¡", r) }
func closing(r rune) bool { return strings.ContainsRune("”’)]}>»,.!?:;", r) }

// punctuation are the Latin kin of the punctuation of other scripts, and
// of full-width forms.
var punctuation = map[rune]string{
	'、': ",", '。': ".", '，': ",", '．': ".", '！': "!", '？': "?", '：': ":", '；': ";",
	'「': "“", '」': "”", '『': "“", '』': "”", '【': "[", '】': "]", '（': "(", '）': ")",
	'〔': "(", '〕': ")", '〈': "<", '〉': ">", '《': "«", '》': "»", '〜': "~", '～': "~",
	'・': " ", '　': " ", '…': "…", '—': "—",
	// Arabic, Devanagari, Armenian, Ethiopic, Tibetan, Georgian.
	'،': ",", '؛': ";", '؟': "?", '۔': ".", '٪': "%", '।': ".", '॥': ".",
	'։': ".", '՝': ",", '՜': "!", '՞': "?", '፡': " ", '።': ".", '፣': ",", '፤': ";", '፧': "?",
	'་': " ", '༌': " ", '།': "/", '༎': "//", '჻': ".", '๚': ".", '๛': ".",
	// Lao, Khmer and Burmese; and the zero-width space, which parts their
	// words where they're parted.
	'ຯ': "…", '។': ".", '៕': ".", '៖': ":", '៘': "…", '၊': ",", '။': ".", 0x200B: " ",
	// The Greek question mark, and its ano teleia.
	';': "?", '·': ";",
}

// zeros are the zeros of the digits of other scripts, which are written
// as 0 to 9.
var zeros = []rune{0x0660, 0x06F0, 0x0966, 0x09E6, 0x0A66, 0x0AE6, 0x0B66, 0x0BE6, 0x0C66, 0x0CE6, 0x0D66, 0x0DE6, 0x0E50, 0x0ED0, 0x0F20, 0x1040, 0x17E0, 0xFF10}

// plain writes what isn't of a script romanized: punctuation as its
// Latin kin, digits as 0 to 9, the rest as it is.
func plain(r rune) string {
	if p, ok := punctuation[r]; ok {
		return p
	}
	if unicode.IsDigit(r) && r >= 0x80 {
		for _, z := range zeros {
			if r >= z && r <= z+9 {
				return string('0' + r - z)
			}
		}
	}
	if r >= 0xFF01 && r <= 0xFF5E {
		return string(r - 0xFEE0)
	}
	return string(r)
}

// capital writes latin as it shows for a capital letter: "Sh" for Ш in
// "Шум", "SH" in "ШУМ", as next, the letter after it, is a capital too.
func capital(latin string, next rune) string {
	if latin == "" {
		return ""
	}
	if unicode.IsUpper(next) {
		return strings.ToUpper(latin)
	}
	r, size := utf8.DecodeRuneInString(latin)
	return string(unicode.ToUpper(r)) + latin[size:]
}
