// Package romaji writes Japanese in Latin letters, as Hepburn romaji: kana
// by a table, and kanji by the readings the morphological analyser kagome
// finds in the IPA dictionary, which MixMusic downloads the first time
// romaji is turned on (Dictionary).
package romaji

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// syllables are the romaji of the kana, in katakana: those of one kana
// and a small one, as キャ, before those of one alone.
var syllables = map[string]string{
	"ア": "a", "イ": "i", "ウ": "u", "エ": "e", "オ": "o",
	"カ": "ka", "キ": "ki", "ク": "ku", "ケ": "ke", "コ": "ko",
	"ガ": "ga", "ギ": "gi", "グ": "gu", "ゲ": "ge", "ゴ": "go",
	"サ": "sa", "シ": "shi", "ス": "su", "セ": "se", "ソ": "so",
	"ザ": "za", "ジ": "ji", "ズ": "zu", "ゼ": "ze", "ゾ": "zo",
	"タ": "ta", "チ": "chi", "ツ": "tsu", "テ": "te", "ト": "to",
	"ダ": "da", "ヂ": "ji", "ヅ": "zu", "デ": "de", "ド": "do",
	"ナ": "na", "ニ": "ni", "ヌ": "nu", "ネ": "ne", "ノ": "no",
	"ハ": "ha", "ヒ": "hi", "フ": "fu", "ヘ": "he", "ホ": "ho",
	"バ": "ba", "ビ": "bi", "ブ": "bu", "ベ": "be", "ボ": "bo",
	"パ": "pa", "ピ": "pi", "プ": "pu", "ペ": "pe", "ポ": "po",
	"マ": "ma", "ミ": "mi", "ム": "mu", "メ": "me", "モ": "mo",
	"ヤ": "ya", "ユ": "yu", "ヨ": "yo",
	"ラ": "ra", "リ": "ri", "ル": "ru", "レ": "re", "ロ": "ro",
	"ワ": "wa", "ヰ": "i", "ヱ": "e", "ヲ": "o", "ン": "n",
	"ヴ": "vu", "ヷ": "va", "ヸ": "vi", "ヹ": "ve", "ヺ": "vo",
	// Small kana alone, as in ぁ or ゃ.
	"ァ": "a", "ィ": "i", "ゥ": "u", "ェ": "e", "ォ": "o",
	"ャ": "ya", "ュ": "yu", "ョ": "yo", "ヮ": "wa", "ヵ": "ka", "ヶ": "ke",

	"キャ": "kya", "キュ": "kyu", "キョ": "kyo", "ギャ": "gya", "ギュ": "gyu", "ギョ": "gyo",
	"シャ": "sha", "シュ": "shu", "ショ": "sho", "ジャ": "ja", "ジュ": "ju", "ジョ": "jo",
	"チャ": "cha", "チュ": "chu", "チョ": "cho", "ヂャ": "ja", "ヂュ": "ju", "ヂョ": "jo",
	"ニャ": "nya", "ニュ": "nyu", "ニョ": "nyo", "ヒャ": "hya", "ヒュ": "hyu", "ヒョ": "hyo",
	"ビャ": "bya", "ビュ": "byu", "ビョ": "byo", "ピャ": "pya", "ピュ": "pyu", "ピョ": "pyo",
	"ミャ": "mya", "ミュ": "myu", "ミョ": "myo", "リャ": "rya", "リュ": "ryu", "リョ": "ryo",
	// The kana of the sounds of loanwords.
	"シェ": "she", "ジェ": "je", "チェ": "che", "イェ": "ye",
	"ファ": "fa", "フィ": "fi", "フェ": "fe", "フォ": "fo", "フュ": "fyu",
	"ティ": "ti", "トゥ": "tu", "テュ": "tyu", "ディ": "di", "ドゥ": "du", "デュ": "dyu",
	"ウィ": "wi", "ウェ": "we", "ウォ": "wo", "スィ": "si", "ズィ": "zi",
	"ツァ": "tsa", "ツィ": "tsi", "ツェ": "tse", "ツォ": "tso",
	"ヴァ": "va", "ヴィ": "vi", "ヴェ": "ve", "ヴォ": "vo", "ヴュ": "vyu",
	"クァ": "kwa", "クィ": "kwi", "クェ": "kwe", "クォ": "kwo", "クヮ": "kwa",
	"グァ": "gwa", "グヮ": "gwa", "ニェ": "nye", "ヒェ": "hye", "キェ": "kye",
}

// macrons are the vowels a long vowel mark lengthens, and how.
var macrons = map[rune]rune{'a': 'ā', 'i': 'ī', 'u': 'ū', 'e': 'ē', 'o': 'ō'}

// Kana writes the kana of s in romaji, and leaves the rest as it is, kanji
// among it, but for the width of fullwidth letters and halfwidth kana:
// "きょうは天気" is "kyōha天気". Small kana join the kana before
// them (きゃ, kya), っ doubles the consonant after it (きって, kitte), ん
// is n' before a vowel or y (きんようび, kin'yōbi), and long vowels take a
// macron: ou and oo are ō, uu ū, as ー lengthens the vowel before it
// (コーヒー, kōhī).
//
// Kana alone can't tell ou that is one long vowel from the ending of a verb
// (おもう, omou, is "omō" here), nor は the particle, wa, from the kana ha:
// Romanizer reads those from the dictionary.
func Kana(s string) string { return kana(guessLong(katakana(s))) }

// katakana returns s with its hiragana as katakana, its halfwidth katakana
// as those of full width, and its iteration marks as the kana they repeat.
func katakana(s string) []rune {
	rs := []rune(norm.NFKC.String(s))
	for i, r := range rs {
		if r >= 'ぁ' && r <= 'ゖ' || r == 'ゝ' || r == 'ゞ' {
			rs[i] = r + 'ァ' - 'ぁ'
		}
		switch rs[i] {
		case 'ヽ', 'ヾ':
			if i == 0 || syllables[string(rs[i-1])] == "" {
				break
			}
			prev := rs[i-1]
			if rs[i] == 'ヾ' {
				// The kana voiced, as カ to ガ, where it can be.
				if v := []rune(norm.NFC.String(string(prev) + "゙")); len(v) == 1 {
					prev = v[0]
				}
			}
			rs[i] = prev
		}
	}
	return rs
}

// small tells the small kana that join the kana before them.
func small(r rune) bool { return strings.ContainsRune("ァィゥェォャュョヮ", r) }

// syllable returns the romaji of the kana at rs[i], with the small kana
// after it that join it, and how many kana that is; "" and 1 for what
// isn't a kana of syllables.
func syllable(rs []rune, i int) (string, int) {
	if i+1 < len(rs) && small(rs[i+1]) {
		if syl := syllables[string(rs[i:i+2])]; syl != "" {
			return syl, 2
		}
	}
	return syllables[string(rs[i])], 1
}

// vowelOf returns the vowel a syllable's romaji ends with, 0 for n.
func vowelOf(syl string) rune {
	if syl == "" {
		return 0
	}
	if last := rune(syl[len(syl)-1]); macrons[last] != 0 {
		return last
	}
	return 0
}

// guessLong marks with ー the long vowels of katakana, as kana tells them
// without a dictionary: ウ after o or u, and オ after o.
func guessLong(rs []rune) []rune {
	var vowel rune
	for i := 0; i < len(rs); {
		syl, n := syllable(rs, i)
		if n == 1 && (rs[i] == 'ウ' && (vowel == 'o' || vowel == 'u') || rs[i] == 'オ' && vowel == 'o') {
			rs[i], vowel = 'ー', 0
		} else {
			vowel = vowelOf(syl)
		}
		i += n
	}
	return rs
}

// kana writes the katakana of rs in romaji, as Kana does, but for the
// long vowels: only ー makes them, as guessLong and Romanizer mark them.
func kana(rs []rune) string {
	var out []rune
	// sokuon is set after っ; afterN after ん; vowel is the vowel the last
	// kana ended with, 0 after anything else, or after a long vowel.
	sokuon, afterN := false, false
	var vowel rune
	for i := 0; i < len(rs); {
		r := rs[i]
		switch r {
		case 'ッ':
			sokuon, afterN, vowel = true, false, 0
			i++
			continue
		case 'ー':
			if n := len(out); n > 0 && vowel != 0 {
				out[n-1] = macrons[out[n-1]]
			}
			sokuon, afterN, vowel = false, false, 0
			i++
			continue
		}
		syl, n := syllable(rs, i)
		if syl == "" {
			// Not kana: as it is.
			out = append(out, r)
			sokuon, afterN, vowel = false, false, 0
			i++
			continue
		}
		i += n
		first := rune(syl[0])
		switch {
		case afterN && strings.ContainsRune("aiueoy", first):
			out = append(out, '\'')
		case sokuon && strings.HasPrefix(syl, "ch"):
			out = append(out, 't')
		case sokuon && syl != "n" && !strings.ContainsRune("aiueo", first):
			out = append(out, first)
		}
		out = append(out, []rune(syl)...)
		sokuon, afterN, vowel = false, syl == "n", vowelOf(syl)
	}
	return string(out)
}

// IsKana tells the kana, hiragana or katakana, and the long vowel mark.
func IsKana(r rune) bool {
	return unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) || r == 'ー' || r == 'ｰ'
}

// HasKana tells whether s holds kana, which tell Japanese from Chinese.
func HasKana(s string) bool { return strings.ContainsFunc(s, IsKana) }

// Japanese tells whether s holds kana or kanji: what romaji writes in Latin
// letters.
func Japanese(s string) bool {
	return strings.ContainsFunc(s, func(r rune) bool { return IsKana(r) || unicode.Is(unicode.Han, r) })
}
