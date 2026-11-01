package lyrics

import (
	"maps"
	"slices"
	"strings"
	"unicode"
)

// GuessLanguage guesses the language of lyrics from their words, as a
// code of ISO 639-1, as "ja" or "es"; "" when it can't tell. It tells
// lyrics in another script than Latin letters by their script, Japanese by
// its kana, and those in Latin letters by the commonest words of English,
// Spanish, Portuguese, French, German, Italian, Dutch, Indonesian,
// Turkish, Polish, Filipino and Vietnamese.
func GuessLanguage(lines []Line) string {
	var (
		count       = map[string]int{}
		letters     int
		latin, viet int
	)
	for _, l := range lines {
		for _, r := range l.Text {
			if !unicode.IsLetter(r) {
				continue
			}
			letters++
			if unicode.Is(unicode.Latin, r) {
				latin++
				if vietnamese(r) {
					viet++
				}
				continue
			}
			for _, s := range scripts {
				if unicode.Is(s.table, r) {
					count[s.lang]++
					break
				}
			}
		}
	}
	if letters == 0 {
		return ""
	}
	// Lyrics in another script often hold some English: that script tells
	// their language once it has a quarter of the letters.
	if latin*4 < letters*3 {
		return otherScript(count, lines)
	}
	if viet*100 >= latin*3 {
		return "vi"
	}
	return byWords(lines)
}

// scripts are the scripts other than Latin letters, and the language each
// tells when it is the commonest: "ja" is kana, which Japanese writes
// along with Han, Chinese.
var scripts = []struct {
	table *unicode.RangeTable
	lang  string
}{
	{unicode.Hiragana, "ja"}, {unicode.Katakana, "ja"}, {unicode.Han, "zh"}, {unicode.Hangul, "ko"},
	{unicode.Cyrillic, "ru"}, {unicode.Greek, "el"}, {unicode.Arabic, "ar"}, {unicode.Hebrew, "he"},
	{unicode.Thai, "th"}, {unicode.Devanagari, "hi"}, {unicode.Bengali, "bn"}, {unicode.Tamil, "ta"},
	{unicode.Telugu, "te"}, {unicode.Gujarati, "gu"}, {unicode.Gurmukhi, "pa"}, {unicode.Kannada, "kn"},
	{unicode.Malayalam, "ml"}, {unicode.Georgian, "ka"}, {unicode.Armenian, "hy"}, {unicode.Khmer, "km"},
	{unicode.Lao, "lo"}, {unicode.Myanmar, "my"}, {unicode.Sinhala, "si"}, {unicode.Ethiopic, "am"},
	{unicode.Tibetan, "bo"},
}

// otherScript tells the language of lyrics mostly in another script than
// Latin letters, by the count of the letters of each script.
func otherScript(count map[string]int, lines []Line) string {
	// Japanese writes Han among its kana, which Chinese doesn't have.
	if count["ja"] > 0 && count["ja"]*20 >= count["zh"] {
		count["ja"] += count["zh"]
		count["zh"] = 0
	}
	best := ""
	for _, s := range scripts {
		if count[s.lang] > count[best] {
			best = s.lang
		}
	}
	// Some languages write their script with letters of their own.
	has := func(letters string) bool {
		for _, l := range lines {
			if strings.ContainsAny(strings.ToLower(l.Text), letters) {
				return true
			}
		}
		return false
	}
	switch {
	case best == "ru" && has("ґєії"):
		return "uk"
	case best == "ru" && has("ђћџљњј"):
		return "sr"
	case best == "ar" && has("ٹڈڑںے"):
		return "ur"
	case best == "ar" && has("پچژگ"):
		return "fa"
	}
	return best
}

// vietnamese tells the letters of Vietnamese that no other language of
// Latin letters has.
func vietnamese(r rune) bool {
	return r >= 0x1EA0 && r <= 0x1EF9 || strings.ContainsRune("ăĂđĐơƠưƯ", r)
}

// commonWords are words each language sings most, which few others share.
var commonWords = map[string]string{
	"en": "the you and to my your that it's i'm don't can't what know just like with this when never love baby yeah got now want feel tonight there they have was will we all is it of on in but oh me be",
	"es": "el la los las que y de en no me mi tu te es lo por con una un se yo amor como pero más quiero corazón todo cuando vida ya sin si eres estoy hay para del al nada porque",
	"pt": "o os que e de não eu você meu minha com um uma é do da no na te se mais quando amor coração tudo pra sem vou mim isso ela ele nós só também agora aqui",
	"fr": "je tu le les de et que pas ne est un une moi toi dans pour qui mon ma c'est j'ai plus sur nous vous avec au du des ce mais tout il elle on suis comme quand jamais toujours rien",
	"de": "ich du und die der das nicht ist ein eine mich mir dich dir es wir sie zu auf mit wie noch auch nur bin hab kein mein dein wenn alles immer nie so sich den dem",
	"it": "il che di e la non mi ti un una sei è per con ma io tu amore come sono più nel della cuore ho se cosa sempre mai tutto questa questo anche lei lui del ci",
	"nl": "ik je de het een en van niet is dat die mijn jij wat op te met voor maar nog zijn heb me wil ben als er ze we naar dan kan hier nooit altijd",
	"id": "aku kau yang dan di ini itu tak tidak cinta kamu dia untuk dengan akan ada ke dalam hati bisa saja sudah jangan apa kita mereka karena hanya selalu pernah lagi masih ingin bukan semua",
	"tr": "ben sen bir ve bu ne için gibi çok var yok da de seni beni aşk bana sana o mi ki her şey daha gel kalbim değil ama artık benim senin",
	"pl": "nie się i w na to że jak mnie mi ja ty jest co tak do z czy już mój moja ciebie tylko jeszcze bo ale kiedy gdy będzie tu tam",
	"tl": "ang ng sa ko mo ka na at hindi ikaw ako kita mahal lang pa ba kung siya naman ay mga ito iyong",
}

// wordsOf are commonWords as sets, made once.
var wordsOf = func() map[string]map[string]bool {
	sets := map[string]map[string]bool{}
	for lang, words := range commonWords {
		sets[lang] = map[string]bool{}
		for _, w := range strings.Fields(words) {
			sets[lang][w] = true
		}
	}
	return sets
}()

// byWords tells the language of lyrics in Latin letters by the share of
// their words that are the commonest of a language: at least one in
// twelve, and a quarter more than of any other.
func byWords(lines []Line) string {
	score := map[string]int{}
	words := 0
	for _, l := range lines {
		text := strings.ToLower(strings.ReplaceAll(l.Text, "’", "'"))
		for _, w := range strings.FieldsFunc(text, func(r rune) bool { return !unicode.IsLetter(r) && r != '\'' }) {
			w = strings.Trim(w, "'")
			if w == "" {
				continue
			}
			words++
			for lang, set := range wordsOf {
				if set[w] {
					score[lang]++
				}
			}
		}
	}
	best, n, second := "", 0, 0
	for _, lang := range slices.Sorted(maps.Keys(score)) {
		switch m := score[lang]; {
		case m > n:
			best, n, second = lang, m, n
		case m > second:
			second = m
		}
	}
	if n*12 < words || n*4 < second*5 {
		return ""
	}
	return best
}

// nonLatin are the languages written in another script than Latin
// letters, which lyrics can be romanized from.
var nonLatin = map[string]bool{}

func init() {
	for _, s := range scripts {
		nonLatin[s.lang] = true
	}
	for _, lang := range strings.Fields("zh-Hant uk sr bg mk be kk ky mn tg fa ur ps he yi mr ne sa as dv ti") {
		nonLatin[lang] = true
	}
}

// Romanizable reports whether lyrics in the language lang, a code of ISO
// 639-1 as GuessLanguage returns and Unison tells, are written in another
// script than Latin letters, as Japanese, Korean or Russian: lyrics that
// can be shown in Latin letters too.
func Romanizable(lang string) bool { return nonLatin[lang] }
