package main

import (
	"slices"
	"testing"

	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/lyrics"
)

// TestLyricsRomanizedLaoKhmerBurmese offers the switch at once for lyrics
// in Lao, Khmer and Burmese, and shows each line in Latin letters under
// it, made on the computer, asking Unison for nothing.
func TestLyricsRomanizedLaoKhmerBurmese(t *testing.T) {
	for _, c := range []struct{ lang, line, roman string }{
		{"lo", "ຂ້ອຍຮັກເຈົ້າ", "khoy hak chao"},
		{"km", "បងស្រលាញ់អូន", "bâng srâ loănh on"},
		{"my", "မင်းကို ချစ်တယ်", "min go chit tè"},
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

// TestLyricsRomanizedKhmerRemembered keeps the choice for lyrics of an
// unknown language whose lines are in Khmer under "km", the language of
// their script, and shows them in Latin letters in place of the lines.
func TestLyricsRomanizedKhmerRemembered(t *testing.T) {
	s, texts := romanizedIn(t, "", []lyrics.Line{{Text: "Paper kites over the river"}, {TimeMs: 60000, Text: "ខ្ញុំស្រលាញ់អូន"}})
	texts()
	tt := testerOf(t, s, 1280, 900)
	tt.Frame()
	click(t, s, tt, "Show lyrics in Latin letters")
	if got := s.svc.Preferences.Get().LyricsRomanize; got["km"] != settings.RomanizeUnder || len(got) != 1 {
		t.Fatalf("saved %v", got)
	}
	s.setRomanize("km", settings.RomanizeOnly)
	if got := texts(); !slices.Contains(got, "khnhom srâ loănh on") || slices.Contains(got, "ខ្ញុំស្រលាញ់អូន") {
		t.Errorf("only: %q", got)
	}
}

// TestLyricsRomanizedLaoWords lights each syllable of a line in Lao timed
// word by word at its word's times.
func TestLyricsRomanizedLaoWords(t *testing.T) {
	ly := lyrics.Lyrics{Synced: true, Lines: []lyrics.Line{{Text: "ຂ້ອຍຮັກເຈົ້າ", Words: []lyrics.Word{
		{Text: "ຂ້ອຍ", StartMs: 0, EndMs: 400}, {Text: "ຮັກ", StartMs: 400, EndMs: 800}, {Text: "ເຈົ້າ", StartMs: 800, EndMs: 1500},
	}}}}
	r := romanizeLyrics(ly, "lo")
	want := []lyrics.Word{{Text: "khoy", EndMs: 400}, {Text: "hak", StartMs: 400, EndMs: 800}, {Text: "chao", StartMs: 800, EndMs: 1500}}
	if r.lines[0] != "khoy hak chao" || !slices.Equal(r.words[0], want) {
		t.Errorf("%q %v", r.lines, r.words)
	}
}
