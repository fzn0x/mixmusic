package main

import (
	"fmt"
	"slices"
	"testing"

	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/lyrics"
)

// kiteLines are made-up lyrics in Japanese, and kiteTranslation their
// made-up translation and romaji.
var (
	kiteLines       = []lyrics.Line{{TimeMs: 0, Text: "紙の凧"}, {TimeMs: 60000, Text: "空へ"}}
	kiteTranslation = translated{lyrics.Translation{From: "ja", To: "en", Lines: []lyrics.TranslatedLine{
		{Text: "Paper kite", Romanized: "kami no tako"}, {Text: "To the sky", Romanized: "sora e"}}}, true}
)

// lyricsIn shows made-up lyrics in the language lang in the lyrics panel,
// with their translation into English when it is given.
func lyricsIn(t *testing.T, lang string, tr *loaded[translated]) (*shell, func() []string) {
	t.Helper()
	s := newTestShell(t)
	s.loads["lyrics:"+testTracks[0].ID] = &loaded[lyrics.Lyrics]{done: true, value: lyrics.Lyrics{Synced: true, Source: "Unison", Language: lang, Lines: kiteLines}}
	if tr != nil {
		s.loads["lyrics-translation:"+testTracks[0].ID+":en"] = tr
	}
	s.p.PlayTracks(testTracks, 0)
	s.settle(t, func(st player.State) bool { return st.Playing })
	tt := testerOf(t, s, 1280, 900)
	s.open("/lyrics")
	tt.Frame()
	return s, func() []string {
		s.drain()
		tt.Frame()
		return tt.Texts()
	}
}

// TestLyricsSwitches offers to translate lyrics in another language than
// the one they translate to, and to show those in another script in Latin
// letters, where Unison romanizes them.
func TestLyricsSwitches(t *testing.T) {
	for _, c := range []struct {
		lang                string
		translate, romanize string
	}{
		{"ko", "translate", "romanize"},
		{"ru", "translate", "romanize"},
		// Unison romanized no Japanese or Ukrainian when checked: their
		// switch waits for a translation that brings some.
		{"ja", "translate", ""},
		{"uk", "translate", ""},
		{"es", "translate", ""},
		{"en", "", ""},
		// A language MixMusic couldn't tell may be another.
		{"", "translate", ""},
	} {
		_, texts := lyricsIn(t, c.lang, nil)
		got := texts()
		for _, name := range []string{"translate", "romaji", "romanize"} {
			want := name == c.translate || name == c.romanize
			if slices.Contains(got, name) != want {
				t.Errorf("%q lyrics: %q shows: %v", c.lang, name, !want)
			}
		}
		if !slices.Contains(got, "紙の凧") {
			t.Errorf("%q lyrics: the lyrics don't show: %q", c.lang, got)
		}
	}
}

// TestLyricsTranslated shows each line's translation under it, and its
// romaji, under it or in its place, as the user chose for Japanese.
func TestLyricsTranslated(t *testing.T) {
	s, texts := lyricsIn(t, "ja", &loaded[translated]{done: true, value: kiteTranslation})
	if got := texts(); slices.Contains(got, "Paper kite") || slices.Contains(got, "kami no tako") {
		t.Errorf("translated while off: %q", got)
	}

	s.set.LyricsTranslate = []string{"ja"}
	got := texts()
	if i := slices.Index(got, "紙の凧"); i < 0 || i+1 >= len(got) || got[i+1] != "Paper kite" || slices.Contains(got, "kami no tako") {
		t.Errorf("translated: %q", got)
	}

	s.set.LyricsRomanize = map[string]string{"ja": settings.RomanizeUnder}
	got = texts()
	if i := slices.Index(got, "紙の凧"); i < 0 || i+2 >= len(got) || got[i+1] != "kami no tako" || got[i+2] != "Paper kite" {
		t.Errorf("romaji under: %q", got)
	}
	s.set.LyricsRomanize = map[string]string{"ja": settings.RomanizeOnly}
	got = texts()
	if i := slices.Index(got, "kami no tako"); i < 0 || slices.Contains(got, "紙の凧") || got[i+1] != "Paper kite" {
		t.Errorf("romaji only: %q", got)
	}
	if !slices.Contains(got, "romaji under") || !slices.Contains(got, "romaji only") {
		t.Errorf("no choice of romaji under or only: %q", got)
	}

	// Translations of other lyrics of the track don't show on these.
	s.loads["lyrics-translation:"+testTracks[0].ID+":en"] = &loaded[translated]{done: true, value: translated{lyrics.Translation{Lines: kiteTranslation.Lines[:1]}, true}}
	if got := texts(); !slices.Contains(got, "紙の凧") || slices.Contains(got, "Paper kite") {
		t.Errorf("a translation of other lyrics: %q", got)
	}
}

// TestLyricsRomajiWithTranslation offers romaji of Japanese lyrics once
// their translation brought some, and not when it brought none, as
// Unison's translations of Japanese do now.
func TestLyricsRomajiWithTranslation(t *testing.T) {
	none := kiteTranslation
	none.Lines = []lyrics.TranslatedLine{{Text: "Paper kite"}, {Text: "To the sky"}}
	none.romanized = false
	s, texts := lyricsIn(t, "ja", &loaded[translated]{done: true, value: none})
	s.set.LyricsTranslate = []string{"ja"}
	s.set.LyricsRomanize = map[string]string{"ja": settings.RomanizeOnly}
	if got := texts(); slices.Contains(got, "romaji") || !slices.Contains(got, "Paper kite") || !slices.Contains(got, "紙の凧") {
		t.Errorf("a translation without romaji: %q", got)
	}
	s.loads["lyrics-translation:"+testTracks[0].ID+":en"] = &loaded[translated]{done: true, value: kiteTranslation}
	if got := texts(); !slices.Contains(got, "romaji") || !slices.Contains(got, "kami no tako") || slices.Contains(got, "紙の凧") {
		t.Errorf("a translation with romaji: %q", got)
	}
	// Romaji of Japanese alone asks for nothing.
	s.set.LyricsTranslate = nil
	delete(s.loads, "lyrics-translation:"+testTracks[0].ID+":en")
	if got := texts(); slices.Contains(got, "romaji") || s.loads["lyrics-translation:"+testTracks[0].ID+":en"] != nil {
		t.Errorf("romaji alone: %q", got)
	}
}

// TestLyricsTranslateRemembered remembers the translate switch, and the
// romaji one, for the next song in the same language.
func TestLyricsTranslateRemembered(t *testing.T) {
	s, texts := lyricsIn(t, "ja", &loaded[translated]{done: true, value: kiteTranslation})
	texts()
	tt := testerOf(t, s, 1280, 900)
	tt.Frame()
	click(t, s, tt, "Translate lyrics")
	click(t, s, tt, "Show lyrics in Latin letters")
	saved := s.svc.Preferences.Get()
	if !slices.Equal(saved.LyricsTranslate, []string{"ja"}) || saved.LyricsRomanize["ja"] != settings.RomanizeUnder {
		t.Fatalf("saved %q, %v", saved.LyricsTranslate, saved.LyricsRomanize)
	}
	if got := texts(); !slices.Contains(got, "Paper kite") || !slices.Contains(got, "kami no tako") {
		t.Errorf("turned on: %q", got)
	}
	click(t, s, tt, "romaji only")
	if s.svc.Preferences.Get().LyricsRomanize["ja"] != settings.RomanizeOnly {
		t.Error("romaji only wasn't kept")
	}
	click(t, s, tt, "Translate lyrics")
	if got := s.svc.Preferences.Get().LyricsTranslate; len(got) != 0 {
		t.Errorf("turned off, translated: %q", got)
	}
}

// TestLyricsTranslationFails shows the lyrics untranslated when their
// translation fails, says so in one line, and offers to try again.
func TestLyricsTranslationFails(t *testing.T) {
	failed := &loaded[translated]{done: true, err: fmt.Errorf("Translation isn't available right now: %w", lyrics.ErrRateLimited)}
	s, texts := lyricsIn(t, "ja", failed)
	s.set.LyricsTranslate = []string{"ja"}
	got := texts()
	if !slices.Contains(got, "Translation isn't available right now.") || !slices.Contains(got, "try again") || !slices.Contains(got, "紙の凧") {
		t.Fatalf("a failed translation: %q", got)
	}
	tt := testerOf(t, s, 1280, 900)
	tt.Frame()
	click(t, s, tt, "try again")
	if s.loads["lyrics-translation:"+testTracks[0].ID+":en"] == failed {
		t.Error("try again didn't ask for the translation again")
	}
	// Romanization alone says so of romanization.
	s, texts = lyricsIn(t, "ko", failed)
	s.set.LyricsRomanize = map[string]string{"ko": settings.RomanizeUnder}
	if got := texts(); !slices.Contains(got, "Romanization isn't available right now.") {
		t.Errorf("failed romanization: %q", got)
	}
}

// TestTranslateTo chooses the language lyrics translate to in Settings ›
// Playback, the system's by default.
func TestTranslateTo(t *testing.T) {
	s := newTestShell(t)
	// Another language chosen, the system's still names the first choice.
	s.set.LyricsTranslateTo = "ja"
	if options := s.translateOptions(); options[0] != "System language (English)" {
		t.Errorf("first option %q", options[0])
	}
	s.set.LyricsTranslateTo = ""
	if got := s.translateTarget(); got != "en" {
		t.Errorf("translate to %q by default", got)
	}
	if options := s.translateOptions(); options[0] != "System language (English)" || len(options) != len(translateLanguages)+1 {
		t.Errorf("options = %q", options[:3])
	}
	tt := testerOf(t, s, 1280, 1800)
	show(s, tt, "/settings/playback")
	if !tt.HasText("Translate lyrics to") {
		t.Fatalf("no setting: %q", tt.Texts())
	}
	s.set.LyricsTranslateTo = "ja"
	if got := s.translateTarget(); got != "ja" {
		t.Errorf("translate to %q", got)
	}
}
