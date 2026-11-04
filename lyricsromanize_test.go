package main

import (
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/lyrics"
)

// seoulLines are made-up lyrics in Korean, the second with English in it,
// as K-pop's often are; seoulRoman their romanization.
var (
	seoulLines = []lyrics.Line{{TimeMs: 0, Text: "종이 연이 날아"}, {TimeMs: 60000, Text: "하늘로 baby"}}
	seoulRoman = []string{"jongi yeoni nara", "haneullo baby"}
)

// romanizedIn shows made-up lyrics of lines in the language lang in the
// lyrics panel, as lyricsIn does: what it shows comes once their
// romanization, made beside the main thread, is.
func romanizedIn(t *testing.T, lang string, lines []lyrics.Line) (*shell, func() []string) {
	t.Helper()
	s, texts := lyricsIn(t, lang, nil)
	s.loads["lyrics:"+testTracks[0].ID].(*loaded[lyrics.Lyrics]).value.Lines = lines
	return s, func() []string {
		t.Helper()
		for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(time.Millisecond) {
			got := texts()
			made := true
			for key, l := range s.loads {
				if r, ok := l.(*loaded[romanizedLines]); ok && strings.HasPrefix(key, "lyrics-romanized:") && !r.done {
					made = false
				}
			}
			if made {
				return texts()
			}
			if time.Now().After(deadline) {
				t.Fatalf("the romanization wasn't made: %q", got)
			}
		}
	}
}

// asked tells whether the panel asked Unison for a translation of the
// lyrics, which romanization alone of a language MixMusic romanizes
// never does.
func asked(s *shell) bool {
	for key := range s.loads {
		if strings.HasPrefix(key, "lyrics-translation:") {
			return true
		}
	}
	return false
}

// TestLyricsRomanized shows Korean lyrics in Latin letters made on the
// computer, under each line or in its place, at once, asking Unison for
// nothing, its English kept as it is.
func TestLyricsRomanized(t *testing.T) {
	s, texts := romanizedIn(t, "ko", seoulLines)
	if got := texts(); !slices.Contains(got, "romanize") || slices.Contains(got, seoulRoman[0]) {
		t.Fatalf("off: %q", got)
	}
	s.set.LyricsRomanize = map[string]string{"ko": settings.RomanizeUnder}
	got := texts()
	for i, line := range seoulLines {
		if at := slices.Index(got, line.Text); at < 0 || at+1 >= len(got) || got[at+1] != seoulRoman[i] {
			t.Errorf("under: %q", got)
		}
	}
	s.set.LyricsRomanize = map[string]string{"ko": settings.RomanizeOnly}
	if got := texts(); !slices.Contains(got, seoulRoman[1]) || slices.Contains(got, seoulLines[1].Text) {
		t.Errorf("only: %q", got)
	}
	if asked(s) {
		t.Error("romanizing Korean asked Unison for a translation")
	}
}

// TestLyricsRomanizedEveryScript offers the switch at once for lyrics in
// each script MixMusic romanizes, those Unison didn't romanize as Thai,
// Hindi, Greek and Hebrew too, and shows them so, sending nothing.
func TestLyricsRomanizedEveryScript(t *testing.T) {
	for _, c := range []struct{ lang, line, roman string }{
		{"th", "กินข้าว", "kin khao"},
		{"hi", "दिल", "dil"},
		{"el", "Καλημέρα", "Kaliméra"},
		{"he", "שָׁלוֹם", "shalom"},
		{"zh", "银行", "yín háng"},
		{"uk", "Київ", "Kyiv"},
	} {
		s, texts := romanizedIn(t, c.lang, []lyrics.Line{{Text: c.line}})
		s.set.LyricsRomanize = map[string]string{c.lang: settings.RomanizeUnder}
		got := texts()
		if at := slices.Index(got, c.line); !slices.Contains(got, "romanize") || at < 0 || at+1 >= len(got) || got[at+1] != c.roman {
			t.Errorf("%q: %q", c.lang, got)
		}
		if asked(s) {
			t.Errorf("%q asked Unison for a translation", c.lang)
		}
	}
}

// TestLyricsRomanizedRemembered keeps the romanization of lyrics of a
// language MixMusic told as another, but with lines in Hangul, as that of
// Korean, which the next Korean song shows too.
func TestLyricsRomanizedRemembered(t *testing.T) {
	s, texts := romanizedIn(t, "en", []lyrics.Line{{Text: "Paper kites over the river"}, {TimeMs: 60000, Text: "하늘로 baby"}})
	texts()
	tt := testerOf(t, s, 1280, 900)
	tt.Frame()
	click(t, s, tt, "Show lyrics in Latin letters")
	if got := s.svc.Preferences.Get().LyricsRomanize; got["ko"] != settings.RomanizeUnder || len(got) != 1 {
		t.Fatalf("saved %v", got)
	}
	// The line in English has no line in Latin letters under it.
	got := texts()
	english := 0
	for _, text := range got {
		if text == "Paper kites over the river" {
			english++
		}
	}
	if !slices.Contains(got, seoulRoman[1]) || english != 1 {
		t.Errorf("turned on: %q", got)
	}
}

// TestLyricsRomanizedChineseNotJapanese reads lines in Chinese characters
// alone, of a language MixMusic couldn't tell, as Chinese, in pinyin, and
// Japanese lyrics in romaji, never in pinyin.
func TestLyricsRomanizedChineseNotJapanese(t *testing.T) {
	s, texts := romanizedIn(t, "", []lyrics.Line{{Text: "音乐"}})
	s.set.LyricsRomanize = map[string]string{"zh": settings.RomanizeOnly}
	if got := texts(); !slices.Contains(got, "yīn yuè") || slices.Contains(got, "romaji") {
		t.Errorf("Chinese: %q", got)
	}
	s, texts = lyricsIn(t, "ja", nil)
	s.set.LyricsRomanize = map[string]string{"ja": settings.RomanizeOnly, "zh": settings.RomanizeOnly}
	if got := texts(); !slices.Contains(got, "kami no tako") || !slices.Contains(got, "romaji") {
		t.Errorf("Japanese: %q", got)
	}
}

// TestLyricsRomanizedArabic shows Arabic lyrics in the Latin letters made
// on the computer at once, then in Unison's, which have the vowels Arabic
// doesn't write, once they came; and the computer's when Unison fails,
// without a word of it.
func TestLyricsRomanizedArabic(t *testing.T) {
	lines := []lyrics.Line{{Text: "حبيبي"}}
	s, texts := romanizedIn(t, "ar", lines)
	s.set.LyricsRomanize = map[string]string{"ar": settings.RomanizeUnder}
	waiting := &loaded[translated]{}
	s.loads["lyrics-translation:"+testTracks[0].ID+":en"] = waiting
	if got := texts(); !slices.Contains(got, "hbibi") {
		t.Errorf("before Unison's: %q", got)
	}
	waiting.done, waiting.value = true, translated{lyrics.Translation{Lines: []lyrics.TranslatedLine{{Text: "My darling", Romanized: "habibi"}}}, true}
	if got := texts(); !slices.Contains(got, "habibi") || slices.Contains(got, "hbibi") || slices.Contains(got, "My darling") {
		t.Errorf("with Unison's: %q", got)
	}
	waiting.err, waiting.value = lyrics.ErrRateLimited, translated{}
	if got := texts(); !slices.Contains(got, "hbibi") || slices.Contains(got, "Romanization isn't available right now.") {
		t.Errorf("Unison failed: %q", got)
	}
}

// TestLyricsRomanizedWords lights the words of a line synced word by word
// in Latin letters too, each its romanization, at the times of the word
// it came from, the consonant Korean moves to the next syllable with it.
func TestLyricsRomanizedWords(t *testing.T) {
	ly := lyrics.Lyrics{Synced: true, Lines: []lyrics.Line{{Text: "한국어 노래", Words: []lyrics.Word{
		{Text: "한", StartMs: 0, EndMs: 300}, {Text: "국", StartMs: 300, EndMs: 600}, {Text: "어", StartMs: 600, EndMs: 900},
		{Text: "노래", StartMs: 900, EndMs: 1500},
	}}, {TimeMs: 2000, Text: "plain words"}}}
	r := romanizeLyrics(ly, "ko")
	if r.lines[0] != "hangugeo norae" || r.lines[1] != "" || r.words[1] != nil {
		t.Fatalf("lines %q, words %v", r.lines, r.words)
	}
	want := []lyrics.Word{{Text: "han", EndMs: 300}, {Text: "gu", StartMs: 300, EndMs: 600}, {Text: "geo", StartMs: 600, EndMs: 900}, {Text: "norae", StartMs: 900, EndMs: 1500}}
	if !slices.Equal(r.words[0], want) {
		t.Errorf("words %v", r.words[0])
	}
	x := lyricsExtra{local: "ko", roman: &r}
	if shown, under := x.words(0, ly.Lines[0], ly.Lines[0].Text, r.lines[0]); !slices.Equal(shown, ly.Lines[0].Words) || !slices.Equal(under, want) {
		t.Errorf("under: %v, %v", shown, under)
	}
	if shown, under := x.words(0, ly.Lines[0], r.lines[0], ""); !slices.Equal(shown, want) || under != nil {
		t.Errorf("only: %v, %v", shown, under)
	}
	// Unison's Latin letters have no words.
	if shown, under := x.words(0, ly.Lines[0], ly.Lines[0].Text, "han gu geo"); under != nil || shown == nil {
		t.Errorf("Unison's: %v, %v", shown, under)
	}
}

// TestSungWordsRomanizedHere lights the words of a line in Latin letters
// made on the computer as they're sung, in place of the line and under
// it, each word at its times.
func TestSungWordsRomanizedHere(t *testing.T) {
	lines := []lyrics.Line{
		{TimeMs: 1000, Text: "종이 배", Words: []lyrics.Word{word("종이", 1000, 2000), word("배", 2000, 3000)}},
		{TimeMs: 60000, Text: "바다", Words: []lyrics.Word{word("바다", 60000, 61000)}},
	}
	s := newTestShell(t)
	s.loads["lyrics:"+testTracks[0].ID] = &loaded[lyrics.Lyrics]{done: true, value: lyrics.Lyrics{Synced: true, Source: "Unison", Language: "ko", Lines: lines}}
	s.set.LyricsRomanize = map[string]string{"ko": settings.RomanizeOnly}
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing && !st.Loading })
	s.p.Time(1500*time.Millisecond, 200*time.Second)
	s.settle(t, func(st player.State) bool { return st.Position == 1500*time.Millisecond })
	tt := testerOf(t, s, 1280, 900)
	s.open("/lyrics")
	// The romanization is made beside the main thread.
	for deadline := time.Now().Add(10 * time.Second); !tt.HasText("jongi bae") && time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		s.drain()
		tt.Frame()
	}
	at := wordsAt(t, s, tt, "jongi bae", "jongi", "bae")
	if img := tt.Image(); lit(t, s, img, at[1], "bae") || inkIn(img, at[0], s.lk.accent) == 0 {
		t.Error("the line in Latin letters doesn't light word by word")
	}
	if asked(s) {
		t.Error("Korean in Latin letters asked Unison")
	}

	s.set.LyricsRomanize = map[string]string{"ko": settings.RomanizeUnder}
	s.drain()
	tt.Frame()
	under, ok := tt.Find("jongi bae")
	if !ok {
		t.Fatalf("no Latin letters under the line: %q", tt.Texts())
	}
	if img := tt.Image(); inkIn(img, under, s.lk.accent) == 0 {
		t.Error("the Latin letters under the line don't light")
	}
}

// TestNotchRomanized shows the lyrics in the notch player in Latin letters
// as the user shows them in the panel: in place of their words, or the
// line sung now under it.
func TestNotchRomanized(t *testing.T) {
	s := newTestShell(t)
	s.loads["lyrics:"+testTracks[0].ID] = &loaded[lyrics.Lyrics]{done: true, value: lyrics.Lyrics{Synced: true, Source: "LRCLIB", Language: "ko", Lines: seoulLines}}
	s.set.LyricsRomanize = map[string]string{"ko": settings.RomanizeOnly}
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	x := newNotchTest(t, s, testHousing)
	x.openPanel()
	// The romanization is made beside the main thread.
	for deadline := time.Now().Add(10 * time.Second); !x.tt.HasText(seoulRoman[0]) && time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		x.frame()
	}
	if !x.tt.HasText(seoulRoman[0]) || x.tt.HasText(seoulLines[0].Text) {
		t.Errorf("only: %q", x.tt.Texts())
	}
	s.set.LyricsRomanize = map[string]string{"ko": settings.RomanizeUnder}
	x.frame()
	if !x.tt.HasText(seoulRoman[0]) || !x.tt.HasText(seoulLines[0].Text) {
		t.Errorf("under: %q", x.tt.Texts())
	}
}

// TestRenderRomanized draws made-up lyrics of a few scripts in the lyrics
// panel, in Latin letters under each line, to MIXMUSIC_RENDER, for a look:
// the Korean line sung now lights word by word.
func TestRenderRomanized(t *testing.T) {
	dir := os.Getenv("MIXMUSIC_RENDER")
	if dir == "" {
		t.Skip("MIXMUSIC_RENDER names no folder")
	}
	for _, c := range []struct {
		name, lang string
		lines      []lyrics.Line
	}{
		{"romanized-korean", "ko", []lyrics.Line{
			{TimeMs: 0, Text: "종이 연이 날아", Words: []lyrics.Word{word("종이", 0, 1500), word("연이", 1500, 3000), word("날아", 3000, 4500)}},
			{TimeMs: 60000, Text: "하늘로 baby"}, {TimeMs: 70000, Text: "밤하늘 별빛이 내려"}}},
		{"romanized-chinese", "zh", []lyrics.Line{{TimeMs: 0, Text: "纸飞机飞过银行"}, {TimeMs: 60000, Text: "我重新出发，音乐响起"}, {TimeMs: 70000, Text: "快乐地行走"}}},
		{"romanized-thai", "th", []lyrics.Line{{TimeMs: 0, Text: "ว่าวกระดาษบนท้องฟ้า"}, {TimeMs: 60000, Text: "เราร้องเพลงด้วยกัน"}, {TimeMs: 70000, Text: "คิดถึงเธอทุกวัน"}}},
		{"romanized-hindi", "hi", []lyrics.Line{{TimeMs: 0, Text: "काग़ज़ की पतंग"}, {TimeMs: 60000, Text: "आसमान में उड़ती है"}, {TimeMs: 70000, Text: "दिल का सपना"}}},
		{"romanized-russian", "ru", []lyrics.Line{{TimeMs: 0, Text: "Бумажный змей над заливом"}, {TimeMs: 60000, Text: "Я жду тебя всю ночь"}, {TimeMs: 70000, Text: "Ещё один рассвет"}}},
	} {
		s := newTestShell(t)
		s.loads["lyrics:"+testTracks[0].ID] = &loaded[lyrics.Lyrics]{done: true, value: lyrics.Lyrics{Synced: true, Source: "Unison", Language: c.lang, Lines: c.lines}}
		s.set.LyricsRomanize = map[string]string{c.lang: settings.RomanizeUnder}
		s.p.PlayTracks(testTracks, 0)
		s.settle(t, func(st player.State) bool { return st.Playing && !st.Loading })
		s.p.Time(2*time.Second, 200*time.Second)
		s.settle(t, func(st player.State) bool { return st.Position == 2*time.Second })
		tt := testerOf(t, s, 1280, 900)
		s.open("/lyrics")
		for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
			s.drain()
			tt.Frame()
			if l, ok := s.loads[romanizedKey(testTracks[0], c.lang)].(*loaded[romanizedLines]); ok && l.done {
				break
			}
		}
		renderTo(t, s, tt, dir, c.name)
	}
}
