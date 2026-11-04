package romanize

import (
	"slices"
	"testing"
)

// TestLine keeps what is in Latin letters as it is, and writes a line
// with nothing to romanize as "".
func TestLine(t *testing.T) {
	for _, c := range []struct{ in, lang, want string }{
		{"Paper kites over the river", "en", ""},
		{"東京", "ja", ""},
		{"", "ko", ""},
		{"  사랑해   baby  ", "ko", "saranghae baby"},
		{"Привет — 사랑해!", "", "Privet — saranghae!"},
		{"１２３ 사랑", "ko", "123 sarang"},
	} {
		if got := Line(c.in, c.lang); got != c.want {
			t.Errorf("%q (%s): %q, want %q", c.in, c.lang, got, c.want)
		}
	}
}

// TestWords gives each word of a line synced word by word the part of
// the line in Latin letters it became: a consonant that Korean moves on
// goes with the syllable it moves to, a Chinese character is a syllable,
// a Thai word its syllables, and a word romanized with a few others gets
// its share.
func TestWords(t *testing.T) {
	for _, c := range []struct {
		in, lang  string
		words     []string
		line      string
		romanized []string
	}{
		{"한국어 사랑해", "ko", []string{"한", "국", "어", "사", "랑", "해"}, "hangugeo saranghae", []string{"han", "gu", "geo", "sa", "rang", "hae"}},
		{"我爱你 baby", "zh", []string{"我", "爱", "你", "baby"}, "wǒ ài nǐ baby", []string{"wǒ", "ài", "nǐ", "baby"}},
		{"ฉันรักเธอ", "th", []string{"ฉัน", "รัก", "เธอ"}, "chan rak thoe", []string{"chan", "rak", "thoe"}},
		{"Привет мир", "ru", []string{"При", "вет", "мир"}, "Privet mir", []string{"Pri", "vet", "mir"}},
		{"Αύριο", "el", []string{"Αύ", "ριο"}, "Ávrio", []string{"Áv", "rio"}},
	} {
		line, words := Words(c.in, c.lang, c.words)
		if line != c.line || !slices.Equal(words, c.romanized) {
			t.Errorf("%s: %q %q, want %q %q", c.in, line, words, c.line, c.romanized)
		}
	}
	if line, words := Words("Paper kites", "en", []string{"Paper", "kites"}); line != "" || words != nil {
		t.Errorf("nothing to romanize: %q %q", line, words)
	}
}

// TestRomanizes tells the lines with letters of the script of their
// language.
func TestRomanizes(t *testing.T) {
	for _, c := range []struct {
		in, lang string
		want     bool
	}{
		{"사랑해 baby", "ko", true},
		{"紙の凧", "ko", false},
		{"紙の凧", "uk", false},
		{"Паперовий змій", "uk", true},
		{"Paper kites", "", false},
		{"银行", "", true},
		{"東京", "ja", false},
	} {
		if got := Romanizes(c.in, c.lang); got != c.want {
			t.Errorf("%q (%s): %v", c.in, c.lang, got)
		}
	}
}

// BenchmarkLine measures a line of each of a few scripts.
func BenchmarkLine(b *testing.B) {
	lines := [][2]string{{"밤하늘 별빛이 내려 baby", "ko"}, {"月亮走，我也走", "zh"}, {"Бумажный змей летит", "ru"}, {"ฉันรักเธอตลอดไป", "th"}, {"दिल का सपना", "hi"}}
	Line(lines[1][0], "zh")
	b.ResetTimer()
	for range b.N {
		for _, l := range lines {
			Line(l[0], l[1])
		}
	}
}
