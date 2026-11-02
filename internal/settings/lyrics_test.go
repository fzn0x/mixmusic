package settings

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestLyricsTranslation keeps the languages lyrics translate and romanize
// in, and leaves out what isn't one.
func TestLyricsTranslation(t *testing.T) {
	s := Settings{
		LyricsTranslate:   []string{"ja", "", "ja", "zh-Hant", "Japanese", "x"},
		LyricsRomanize:    map[string]string{"ja": RomanizeOnly, "ko": RomanizeUnder, "ru": "sideways", "<b>": RomanizeOnly},
		LyricsTranslateTo: "zh-Hant",
	}.Clean()
	if strings.Join(s.LyricsTranslate, ",") != "ja,,zh-Hant" {
		t.Errorf("translate = %q", s.LyricsTranslate)
	}
	if len(s.LyricsRomanize) != 2 || s.LyricsRomanize["ja"] != RomanizeOnly || s.LyricsRomanize["ko"] != RomanizeUnder {
		t.Errorf("romanize = %v", s.LyricsRomanize)
	}
	if s.LyricsTranslateTo != "zh-Hant" {
		t.Errorf("translate to %q", s.LyricsTranslateTo)
	}
	for _, to := range []string{"english", "EN", "en-us", "../x"} {
		if got := (Settings{LyricsTranslateTo: to}).Clean().LyricsTranslateTo; got != "" {
			t.Errorf("translate to %q kept as %q", to, got)
		}
	}
	// Settings from before translations translate nothing, to the
	// system's language.
	var before Settings
	if err := json.Unmarshal([]byte(`{"theme":"mix"}`), &before); err != nil {
		t.Fatal(err)
	}
	if c := before.Clean(); len(c.LyricsTranslate) != 0 || c.LyricsTranslate == nil || c.LyricsRomanize == nil || c.LyricsTranslateTo != "" {
		t.Errorf("settings from before: %+v", c)
	}
}
