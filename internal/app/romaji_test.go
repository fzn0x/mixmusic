package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/romaji"
	"github.com/fzn0x/mixmusic/lyrics"
)

// TestLyricsRomaji writes Japanese lyrics in romaji on the computer: the
// first downloads the dictionary of the readings of kanji, which is all
// that goes out, into the cache, and, until it is there, the lines' kana
// are in romaji and their kanji as they are.
func TestLyricsRomaji(t *testing.T) {
	var asks atomic.Int32
	var asked atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asks.Add(1)
		asked.Store(r.Method + " " + r.URL.RequestURI())
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	dir := t.TempDir()
	c := &Catalog{a: &App{romaji: newRomaji(Options{CacheDir: dir, RomajiURL: srv.URL + "/ipa.dict"}, "test")}}
	ly := lyrics.Lyrics{Language: "ja", Lines: []lyrics.Line{{Text: "今日はいい天気"}, {Text: "Oh yeah"}}}

	got, err := c.LyricsRomaji(context.Background(), ly)
	if err != nil || got.Readings != romaji.Downloading || !slices.Equal(got.Romaji, []string{"今日 haii 天気", ""}) {
		t.Fatalf("got %+v, %v", got, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := c.WaitRomaji(ctx); err != nil {
		t.Fatal(err)
	}
	if got := asked.Load(); got != "GET /ipa.dict" {
		t.Errorf("asked %v", got)
	}
	// Its download failed: not again at once, but when asked.
	if got, _ := c.LyricsRomaji(context.Background(), ly); got.Readings != romaji.Unavailable || asks.Load() != 1 {
		t.Errorf("after the failure: %+v, %d asks", got, asks.Load())
	}
	c.RomajiAgain()
	if got, _ := c.LyricsRomaji(context.Background(), ly); got.Readings != romaji.Downloading {
		t.Errorf("again: %+v", got)
	}
	if err := c.WaitRomaji(ctx); err != nil {
		t.Fatal(err)
	}
	if n := asks.Load(); n != 2 {
		t.Errorf("%d asks", n)
	}
	if d := newRomaji(Options{CacheDir: dir}, "test"); d.URL != romaji.DictionaryURL {
		t.Errorf("downloads from %s", d.URL)
	}
}

// TestRomajiNotices shows the license of kagome, and the notice of the
// dictionary romaji downloads.
func TestRomajiNotices(t *testing.T) {
	notices := (&System{}).Notices()
	for _, want := range []struct{ name, text string }{
		{"kagome", "Copyright (c) 2020 ikawaha"},
		{"kagome", "Copyright (c) 2021 ikawaha"},
		{"The IPA dictionary", "Nara Institute of Science and Technology (NAIST)"},
		{"The IPA dictionary", "ICOT Free Software"},
	} {
		if !slices.ContainsFunc(notices, func(n Notice) bool {
			return strings.HasPrefix(n.Name, want.name) && strings.Contains(n.Text, want.text)
		}) {
			t.Errorf("no notice of %s with %q", want.name, want.text)
		}
	}
}
