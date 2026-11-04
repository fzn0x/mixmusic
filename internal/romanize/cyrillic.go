package romanize

import (
	"strings"
	"unicode"
)

// Cyrillic is written by the system of its language, the lyrics' when it
// is known, else the one its letters tell (cyrillicLanguage):
//
//   - Russian: BGN/PCGN (1947), without the marks of the hard and soft
//     signs, and ё as yo, as it reads;
//   - Ukrainian: the national system (Cabinet of Ministers, resolution 55,
//     2010);
//   - Belarusian: the national system (2007), which the UN recommends;
//   - Bulgarian: the Streamlined System (the Transliteration Act, 2009);
//   - Serbian: Gaj's Latin alphabet, which Serbian is written in too;
//   - Macedonian: the official system (Government of Macedonia, 2008);
//   - Kazakh and Kyrgyz: BGN/PCGN (1979), without the marks of the signs;
//   - Mongolian: MNS 5217:2012.

// cyrillicTables are the letters of each language's system, lowercase:
// a letter's capital is its capital. Letters a system has no word for
// are read as Russian's.
var cyrillicTables = map[string]map[rune]string{
	"ru": {
		'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "yo", 'ж': "zh", 'з': "z",
		'и': "i", 'й': "y", 'к': "k", 'л': "l", 'м': "m", 'н': "n", 'о': "o", 'п': "p", 'р': "r",
		'с': "s", 'т': "t", 'у': "u", 'ф': "f", 'х': "kh", 'ц': "ts", 'ч': "ch", 'ш': "sh", 'щ': "shch",
		'ъ': "", 'ы': "y", 'ь': "", 'э': "e", 'ю': "yu", 'я': "ya",
	},
	"uk": {
		'а': "a", 'б': "b", 'в': "v", 'г': "h", 'ґ': "g", 'д': "d", 'е': "e", 'є': "ie", 'ж': "zh",
		'з': "z", 'и': "y", 'і': "i", 'ї': "i", 'й': "i", 'к': "k", 'л': "l", 'м': "m", 'н': "n",
		'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u", 'ф': "f", 'х': "kh", 'ц': "ts",
		'ч': "ch", 'ш': "sh", 'щ': "shch", 'ь': "", 'ю': "iu", 'я': "ia", '\'': "", 'ʼ': "", '’': "",
	},
	"be": {
		'а': "a", 'б': "b", 'в': "v", 'г': "h", 'ґ': "g", 'д': "d", 'е': "ie", 'ё': "io", 'ж': "ž",
		'з': "z", 'і': "i", 'й': "j", 'к': "k", 'л': "l", 'м': "m", 'н': "n", 'о': "o", 'п': "p",
		'р': "r", 'с': "s", 'т': "t", 'у': "u", 'ў': "ŭ", 'ф': "f", 'х': "ch", 'ц': "c", 'ч': "č",
		'ш': "š", 'ы': "y", 'ь': "", 'э': "e", 'ю': "iu", 'я': "ia", '\'': "", 'ʼ': "", '’': "",
	},
	"bg": {
		'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ж': "zh", 'з': "z", 'и': "i",
		'й': "y", 'к': "k", 'л': "l", 'м': "m", 'н': "n", 'о': "o", 'п': "p", 'р': "r", 'с': "s",
		'т': "t", 'у': "u", 'ф': "f", 'х': "h", 'ц': "ts", 'ч': "ch", 'ш': "sh", 'щ': "sht", 'ъ': "a",
		'ь': "y", 'ю': "yu", 'я': "ya", 'ѝ': "i",
	},
	"sr": {
		'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'ђ': "đ", 'е': "e", 'ж': "ž", 'з': "z",
		'и': "i", 'ј': "j", 'к': "k", 'л': "l", 'љ': "lj", 'м': "m", 'н': "n", 'њ': "nj", 'о': "o",
		'п': "p", 'р': "r", 'с': "s", 'т': "t", 'ћ': "ć", 'у': "u", 'ф': "f", 'х': "h", 'ц': "c",
		'ч': "č", 'џ': "dž", 'ш': "š",
	},
	"mk": {
		'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'ѓ': "gj", 'е': "e", 'ж': "zh", 'з': "z",
		'ѕ': "dz", 'и': "i", 'ј': "j", 'к': "k", 'л': "l", 'љ': "lj", 'м': "m", 'н': "n", 'њ': "nj",
		'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t", 'ќ': "kj", 'у': "u", 'ф': "f", 'х': "h",
		'ц': "c", 'ч': "ch", 'џ': "dzh", 'ш': "sh", 'ѐ': "e", 'ѝ': "i",
	},
	"kk": {
		'ә': "ä", 'ғ': "gh", 'қ': "q", 'ң': "ng", 'ө': "ö", 'у': "ū", 'ұ': "u", 'ү': "ü", 'һ': "h",
		'і': "i", 'и': "ī", 'э': "ė", 'е': "e",
	},
	"ky": {'ң': "ng", 'ө': "ö", 'ү': "ü", 'е': "e"},
	"mn": {
		'е': "ye", 'ё': "yo", 'ж': "j", 'з': "z", 'й': "i", 'ө': "ö", 'ү': "ü", 'х': "kh", 'щ': "sh",
		'ъ': "i", 'ы': "y", 'ь': "i",
	},
}

// cyrillicLanguage tells the language of Cyrillic by the letters only it
// writes, or by the letters it doesn't: Russian when they tell none.
func cyrillicLanguage(s string) string {
	s = strings.ToLower(s)
	has := func(letters string) bool { return strings.ContainsAny(s, letters) }
	switch {
	case has("әғқұһ"):
		return "kk"
	case has("ң") || has("өү") && !has("ә"):
		if has("ң") {
			return "ky"
		}
		return "mn"
	case has("ў"):
		return "be"
	case has("ґєї"):
		return "uk"
	case has("ђћ"):
		return "sr"
	case has("ѓќѕ"):
		return "mk"
	case has("џљњј"):
		// Serbian and Macedonian share these.
		if has("ѐѝ") {
			return "mk"
		}
		return "sr"
	case has("і"):
		// Ukrainian, or Belarusian, which has no и.
		if has("и") {
			return "uk"
		}
		return "be"
	case !has("ыэё") && bulgarianHardSign(s):
		return "bg"
	}
	return "ru"
}

// bulgarianHardSign tells Bulgarian, whose ъ is a vowel within words, as
// in път; Russian's ends a prefix before a soft vowel, as in подъезд.
func bulgarianHardSign(s string) bool {
	runes := []rune(s)
	for i, r := range runes {
		if r != 'ъ' || i+1 >= len(runes) {
			continue
		}
		if next := runes[i+1]; unicode.Is(unicode.Cyrillic, next) && !strings.ContainsRune("еёюя", next) {
			return true
		}
	}
	return false
}

// cyrillicVowels are the vowels of Cyrillic, and the signs and letters
// after which е, ё, ю and я start with y or j.
const (
	cyrillicVowels = "аеёиоуыэюяіїєәөүұ"
	cyrillicSoft   = "ъьйў'ʼ’"
)

func init() {
	register(cyrillic, cyrillicRun, "ru uk be bg sr mk kk ky mn")
	lineLanguages[cyrillic] = lineCyrillic
}

// lineCyrillic is the language of the Cyrillic of a line of lyrics in the
// language lang: the one its letters tell, unless lang is another than
// Russian written in Cyrillic, which Unison or GuessLanguage tell from
// more lines.
func lineCyrillic(s, lang string) string {
	if _, ok := cyrillicTables[lang]; ok && lang != "ru" {
		return lang
	}
	return cyrillicLanguage(s)
}

// cyrillicRun romanizes a run of Cyrillic in the language lang.
func cyrillicRun(run, lang string) []piece {
	table, ok := cyrillicTables[lang]
	if !ok {
		lang, table = "ru", cyrillicTables["ru"]
	}
	runes := []rune(run)
	var pieces []piece
	at := 0
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		lower := unicode.ToLower(r)
		var prev, next rune
		if i > 0 {
			prev = unicode.ToLower(runes[i-1])
		}
		if i+1 < len(runes) {
			next = runes[i+1]
		}
		start := i == 0 || !unicode.IsLetter(prev) && !strings.ContainsRune(cyrillicSoft, prev)
		afterVowel := i > 0 && (strings.ContainsRune(cyrillicVowels, prev) || strings.ContainsRune(cyrillicSoft, prev))
		latin, ok := table[lower]
		if !ok {
			latin, ok = cyrillicTables["ru"][lower]
		}
		if !ok {
			latin = plain(r)
		}
		switch lang {
		case "ru", "ky":
			// е is ye at the start, after a vowel and after the signs;
			// ё after ж, ч, ш and щ is o.
			switch {
			case lower == 'е' && (start || afterVowel):
				latin = "ye"
			case lower == 'ё' && strings.ContainsRune("жчшщ", prev):
				latin = "o"
			}
		case "uk":
			// є, ї, й, ю and я start a word with y; зг is zgh.
			initial := map[rune]string{'є': "ye", 'ї': "yi", 'й': "y", 'ю': "yu", 'я': "ya"}
			if s, ok := initial[lower]; ok && start {
				latin = s
			}
			if lower == 'г' && prev == 'з' {
				latin = "gh"
			}
		case "be":
			// е, ё, ю and я are je, jo, ju and ja at the start, after a
			// vowel, an apostrophe, ь and ў; a consonant before ь takes an
			// acute.
			if strings.ContainsRune("еёюя", lower) && (start || afterVowel) {
				latin = "j" + latin[1:]
			}
			if acute, ok := map[rune]string{'з': "ź", 'л': "ĺ", 'н': "ń", 'с': "ś", 'ц': "ć"}[lower]; ok && unicode.ToLower(next) == 'ь' {
				latin = acute
			}
		case "bg":
			// ия at the end of a word is ia, as in София, Sofia.
			if lower == 'я' && prev == 'и' && (i+1 == len(runes) || !unicode.IsLetter(next)) {
				latin = "a"
			}
		}
		if unicode.IsUpper(r) {
			latin = capital(latin, next)
		}
		pieces = append(pieces, piece{at + len(string(r)), latin})
		at += len(string(r))
	}
	return pieces
}
