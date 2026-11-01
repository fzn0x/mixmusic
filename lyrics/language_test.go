package lyrics

import (
	"strings"
	"testing"
)

// lines makes lyrics of made-up lines, one per line of text.
func lines(text string) []Line {
	var out []Line
	for t := range strings.SplitSeq(text, "\n") {
		out = append(out, Line{Text: t})
	}
	return out
}

func TestGuessLanguage(t *testing.T) {
	for want, text := range map[string]string{
		"en": "Paper kites over the bay\nI know you're waiting for the night\nAnd when it's over, don't look back",
		"es": "Las cometas de papel sobre la bahía\nY yo te espero en la noche\nPorque todo lo que quiero es tu amor",
		"pt": "As pipas de papel sobre a baía\nEu não sei se você vem\nMas o meu coração é teu",
		"fr": "Les cerfs-volants de papier\nJe ne sais pas si tu viens\nC'est toujours toi dans mon cœur",
		"de": "Papierdrachen über der Bucht\nIch weiß nicht, ob du kommst\nUnd ich bin immer noch hier",
		"id": "Layang-layang kertas di atas teluk\nAku tak tahu apa kau datang\nCinta ini masih di hati",
		"vi": "Những cánh diều giấy trên vịnh\nAnh không biết em có đến\nTrái tim này vẫn chờ đợi",
		// Japanese, with some English in it, as songs have.
		"ja": "紙の飛行機が空を飛ぶ\n今日は良い天気だね\nOh baby, ずっと待ってる",
		"zh": "纸飞机飞过海湾\n我一直在等你\n今天天气很好",
		"ko": "종이 비행기가 날아가\n오늘은 좋은 날씨야\n널 기다릴게",
		"ru": "Бумажный змей над заливом\nЯ жду тебя всю ночь",
		"uk": "Паперовий змій над затокою\nЯ чекаю на тебе всю ніч, її",
		"el": "Χάρτινοι χαρταετοί πάνω από τον κόλπο",
		"th": "ว่าวกระดาษเหนืออ่าว",
		"ar": "طائرات ورقية فوق الخليج",
		"fa": "بادبادک کاغذی روی خلیج پرواز می‌کند",
		"hi": "खाड़ी के ऊपर कागज़ की पतंगें",
	} {
		if got := GuessLanguage(lines(text)); got != want {
			t.Errorf("%s: guessed %q", want, got)
		}
	}
	// Too little to tell, or nothing but names and sounds.
	for _, text := range []string{"", "♪", "Oh oh oh\nLa la la", "Marco Polo\nKilimanjaro"} {
		if got := GuessLanguage(lines(text)); got != "" {
			t.Errorf("%q: guessed %q", text, got)
		}
	}
}

func TestRomanizable(t *testing.T) {
	for _, lang := range []string{"ja", "ko", "zh", "zh-Hant", "ru", "uk", "el", "ar", "th", "hi"} {
		if !Romanizable(lang) {
			t.Errorf("%s isn't romanizable", lang)
		}
	}
	for _, lang := range []string{"en", "es", "vi", "id", "tr", ""} {
		if Romanizable(lang) {
			t.Errorf("%q is romanizable", lang)
		}
	}
}
