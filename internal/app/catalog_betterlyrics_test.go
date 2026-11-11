package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/fzn0x/mixmusic/lyrics"
)

// betterLyricsWordSynced is made-up lyrics timed word by word, as Better
// Lyrics' API answers.
const betterLyricsWordSynced = `{"ttml":"<tt xmlns=\"http://www.w3.org/ns/ttml\"><body><div><p begin=\"1.000\" end=\"3.000\"><span begin=\"1.000\" end=\"2.000\">From</span> <span begin=\"2.000\" end=\"3.000\">Better</span></p><p begin=\"4.000\" end=\"6.000\"><span begin=\"4.000\" end=\"5.000\">Second</span> <span begin=\"5.000\" end=\"6.000\">line</span></p></div></body></tt>"}`

// withBetterLyrics has Better Lyrics answer ls's lookups with h, and
// counts them.
func withBetterLyrics(t *testing.T, ls *lyricsSources, h http.HandlerFunc) *atomic.Int32 {
	t.Helper()
	asks := new(atomic.Int32)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asks.Add(1)
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	ls.c.a.betterLyrics = lyrics.NewBetterLyrics("test")
	ls.c.a.betterLyrics.BaseURL = srv.URL
	return asks
}

// TestLyricsBetterLyricsFirst takes Better Lyrics' word-synced lyrics over
// Unison's and LRCLIB's, which aren't asked, and credits Better Lyrics.
func TestLyricsBetterLyricsFirst(t *testing.T) {
	ls := newLyricsSources(t)
	asks := withBetterLyrics(t, ls, status(http.StatusOK, betterLyricsWordSynced))
	ls.unison = status(http.StatusOK, unisonSynced)
	ls.lrclib = status(http.StatusOK, lrclibSynced)
	l, err := ls.c.Lyrics(context.Background(), aSong)
	if err != nil || !l.Synced || l.Lines[0].Text != "From Better" || len(l.Lines[0].Words) != 2 ||
		l.Source != "Better Lyrics" || l.SourceURL != "https://betterlyrics.org" {
		t.Fatalf("lyrics = %+v, %v", l, err)
	}
	if ls.unisonAsks.Load() != 0 || ls.lrcAsks.Load() != 0 {
		t.Errorf("Unison asked %d times, LRCLIB %d, for lyrics Better Lyrics has", ls.unisonAsks.Load(), ls.lrcAsks.Load())
	}
	// Found once, they aren't asked for again.
	if _, err := ls.c.Lyrics(context.Background(), aSong); err != nil || asks.Load() != 1 {
		t.Errorf("Better Lyrics asked %d times, %v", asks.Load(), err)
	}
}

// TestLyricsPastBetterLyrics asks Unison, then LRCLIB, for a track Better
// Lyrics has no word-synced lyrics of, or while it is busy or down, and
// credits whichever had them.
func TestLyricsPastBetterLyrics(t *testing.T) {
	for name, bl := range map[string]http.HandlerFunc{
		"not kept":    status(http.StatusUnauthorized, `{"error":"API key required"}`),
		"not found":   status(http.StatusNotFound, `{}`),
		"busy":        status(http.StatusTooManyRequests, "Too Many Requests"),
		"down":        status(http.StatusBadGateway, ""),
		"line synced": status(http.StatusOK, `{"ttml":"<tt><body><div><p begin=\"1.0\">One</p><p begin=\"4.0\">Two</p></div></body></tt>"}`),
	} {
		ls := newLyricsSources(t)
		withBetterLyrics(t, ls, bl)
		ls.unison = status(http.StatusOK, unisonSynced)
		if l, err := ls.c.Lyrics(context.Background(), aSong); err != nil || l.Source != "Unison" || l.Lines[0].Text != "From Unison" {
			t.Errorf("%s, Unison has them: lyrics = %+v, %v", name, l, err)
		}
		ls = newLyricsSources(t)
		withBetterLyrics(t, ls, bl)
		ls.lrclib = status(http.StatusOK, lrclibSynced)
		if l, err := ls.c.Lyrics(context.Background(), aSong); err != nil || l.Source != "LRCLIB" || l.Lines[0].Text != "From LRCLIB" {
			t.Errorf("%s, LRCLIB has them: lyrics = %+v, %v", name, l, err)
		}
	}
}

// TestLyricsBetterLyricsWithoutAlbum doesn't ask Better Lyrics for a track
// of no album or length, which it can't find without a key.
func TestLyricsBetterLyricsWithoutAlbum(t *testing.T) {
	ls := newLyricsSources(t)
	asks := withBetterLyrics(t, ls, status(http.StatusOK, betterLyricsWordSynced))
	ls.unison = status(http.StatusOK, unisonSynced)
	track := aSong
	track.Album = nil
	if l, err := ls.c.Lyrics(context.Background(), track); err != nil || l.Source != "Unison" || asks.Load() != 0 {
		t.Errorf("lyrics = %+v, %v; Better Lyrics asked %d times", l, err, asks.Load())
	}
}
