package settings

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// keysOf returns the keys of sources, each with "-" after it when it's off.
func keysOf(sources []LyricsSource) string {
	var keys []string
	for _, s := range sources {
		k := s.Key
		if s.Off {
			k += "-"
		}
		keys = append(keys, k)
	}
	return strings.Join(keys, " ")
}

// TestLyricsSourceOrder asks every source, in their own order, until the
// user moves or switches one; a source they never placed, as a later
// version's new one, takes its place after the one it follows by default.
func TestLyricsSourceOrder(t *testing.T) {
	for _, c := range []struct {
		name  string
		saved []LyricsSource
		want  string
	}{
		{"a new install", nil, "betterlyrics unison lrclib youtube captions"},
		{"moved", []LyricsSource{{Key: "lrclib"}, {Key: "betterlyrics", Off: true}, {Key: "unison"}, {Key: "youtube"}, {Key: "captions"}},
			"lrclib betterlyrics- unison youtube captions"},
		// Placed before there were captions: they follow YouTube Music.
		{"a source added since", []LyricsSource{{Key: "youtube"}, {Key: "lrclib"}, {Key: "unison"}, {Key: "betterlyrics"}},
			"youtube captions lrclib unison betterlyrics"},
		// The first by default goes first, and each of the others after the
		// one it follows.
		{"some left out", []LyricsSource{{Key: "lrclib"}, {Key: "unison"}}, "betterlyrics lrclib youtube captions unison"},
		// A later version's source, and one named twice, are left out.
		{"unknown and twice", []LyricsSource{{Key: "someday"}, {Key: "unison", Off: true}, {Key: "unison"}},
			"betterlyrics unison- lrclib youtube captions"},
		// One is always on.
		{"every one off", []LyricsSource{{Key: "captions", Off: true}, {Key: "youtube", Off: true}, {Key: "lrclib", Off: true}, {Key: "unison", Off: true}, {Key: "betterlyrics", Off: true}},
			"captions youtube- lrclib- unison- betterlyrics-"},
	} {
		var l Lyrics
		l.LyricsSources = c.saved
		if got := keysOf(l.SourceOrder()); got != c.want {
			t.Errorf("%s: %s, not %s", c.name, got, c.want)
		}
	}
}

// TestLyricsSourcesRoundTrip writes the sources' order to settings.json
// and reads it back as it was, a later version's source too; a file of
// before, without the key, asks every source.
func TestLyricsSourcesRoundTrip(t *testing.T) {
	s := Default()
	s.LyricsSources = []LyricsSource{{Key: "youtube"}, {Key: "someday", Off: true}, {Key: "unison", Off: true}}
	s.LyricsPreferWords = true
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	read := Default()
	if err := json.Unmarshal(data, &read); err != nil {
		t.Fatal(err)
	}
	if read = read.Clean(); !slices.Equal(read.LyricsSources, s.LyricsSources) || !read.LyricsPreferWords {
		t.Errorf("read back as %+v, %v", read.LyricsSources, read.LyricsPreferWords)
	}
	if !strings.Contains(string(data), `{"key":"unison","off":true}`) || !strings.Contains(string(data), `{"key":"youtube"}`) {
		t.Errorf("written as %s", data)
	}

	before := Default()
	if err := json.Unmarshal([]byte(`{"lyricsPlain": true}`), &before); err != nil {
		t.Fatal(err)
	}
	if got := before.Clean(); got.LyricsSources == nil || len(got.LyricsSources) != 0 || got.LyricsPreferWords || got.LyricsCaptionLanguage != "" {
		t.Errorf("a file of before reads as %#v", got.Lyrics)
	}
}

// TestLyricsSourcesClean keeps each source once, by a key of its own, and
// bounds how many there are; a caption language that isn't one is the
// video's own.
func TestLyricsSourcesClean(t *testing.T) {
	var l Lyrics
	l.LyricsSources = []LyricsSource{{Key: "unison"}, {Key: ""}, {Key: "unison", Off: true}, {Key: strings.Repeat("k", maxLyricsKey+1)}, {Key: "lrclib", Off: true}}
	l.LyricsCaptionLanguage = "German"
	got := l.clean()
	if keysOf(got.LyricsSources) != "unison lrclib-" || got.LyricsCaptionLanguage != "" {
		t.Errorf("cleaned to %s, %q", keysOf(got.LyricsSources), got.LyricsCaptionLanguage)
	}
	l.LyricsSources = nil
	for i := range maxLyricsSources + 5 {
		l.LyricsSources = append(l.LyricsSources, LyricsSource{Key: "later" + strings.Repeat("x", i)})
	}
	if got := l.clean().LyricsSources; len(got) != maxLyricsSources {
		t.Errorf("kept %d sources of %d", len(got), maxLyricsSources+5)
	}
	l.LyricsCaptionLanguage = "zh-Hant"
	if got := l.clean().LyricsCaptionLanguage; got != "zh-Hant" {
		t.Errorf("zh-Hant cleaned to %q", got)
	}
}
