package romanize

import (
	"runtime"
	"testing"
	"time"
)

// TestChinese writes Chinese in pinyin, a character of several readings
// as the word it is in reads it, in simplified and traditional
// characters.
func TestChinese(t *testing.T) {
	for _, c := range []struct{ in, lang, want string }{
		{"重要", "zh", "zhòng yào"},
		{"重新", "zh", "chóng xīn"},
		{"银行", "zh", "yín háng"},
		{"行走", "zh", "xíng zǒu"},
		{"快乐", "zh", "kuài lè"},
		{"音乐", "zh", "yīn yuè"},
		// Traditional characters read as their simplified forms are.
		{"銀行", "zh-Hant", "yín háng"},
		{"音樂很快樂", "zh-Hant", "yīn yuè hěn kuài lè"},
		{"重新開始", "zh-Hant", "chóng xīn kāi shǐ"},
		// Made-up lines, their punctuation as Latin's.
		{"我爱你，你爱我", "zh", "wǒ ài nǐ, nǐ ài wǒ"},
		{"月亮走，我也走。", "zh", "yuè liàng zǒu, wǒ yě zǒu."},
		{"爱你 baby!", "zh", "ài nǐ baby!"},
		{"爱你baby", "", "ài nǐ baby"},
	} {
		if got := Line(c.in, c.lang); got != c.want {
			t.Errorf("%s (%s): %q, want %q", c.in, c.lang, got, c.want)
		}
	}
}

// TestChineseNotJapanese leaves the kanji of Japanese, and of Korean, as
// they are: romaji reads Japanese.
func TestChineseNotJapanese(t *testing.T) {
	for _, lang := range []string{"ja", "ko"} {
		if got := Line("東京", lang); got != "" {
			t.Errorf("%s: %q", lang, got)
		}
	}
	if got := Line("今日は", ""); got != "jīn rì は" {
		t.Errorf("kana in Chinese: %q", got)
	}
}

// TestChineseCost measures what reading pinyin.bin costs, the first time
// Chinese is romanized: the time, and the heap its tables hold.
func TestChineseCost(t *testing.T) {
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	start := time.Now()
	p, err := readPinyin(pinyinData)
	took := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	t.Logf("pinyin.bin: %d bytes, read in %v, %d characters and %d words, %.2f MB of heap held",
		len(pinyinData), took, len(p.chars), len(p.words), float64(after.HeapAlloc-before.HeapAlloc)/1e6)
	if len(p.chars) < 20000 || len(p.words) < 1000 {
		t.Errorf("%d characters, %d words", len(p.chars), len(p.words))
	}
}
