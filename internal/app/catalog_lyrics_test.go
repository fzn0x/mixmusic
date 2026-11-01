package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/youtube"
	"github.com/fzn0x/mixmusic/lyrics"
)

// TestLyricsInPlainWords tells a track without lyrics from lyrics that
// can't load, in words for the lyrics panel rather than LRCLIB's.
func TestLyricsInPlainWords(t *testing.T) {
	status, cached := http.StatusServiceUnavailable, http.StatusServiceUnavailable
	lrclib := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/get-cached" {
			w.WriteHeader(cached)
			return
		}
		w.WriteHeader(status)
	}))
	defer lrclib.Close()
	// YouTube Music has no lyrics either.
	yt := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer yt.Close()
	a := &App{lyrics: lyrics.New("test"), youtube: youtube.New(), lyricsMemo: newMemo[lyrics.Lyrics](4)}
	a.lyrics.BaseURL = lrclib.URL
	a.youtube.BaseURL = yt.URL
	c := &Catalog{a: a}
	track := media.Track{ID: "yt:dQw4w9WgXcQ", Source: media.SourceYouTube, Title: "Song", Artists: []media.Ref{{Name: "Artist"}}}

	_, err := c.Lyrics(context.Background(), track)
	if err == nil || err.Error() != "Lyrics can't be loaded right now. Check your connection, or try again in a moment." || !errors.Is(err, lyrics.ErrUnavailable) {
		t.Errorf("LRCLIB down: %v", err)
	}
	// LRCLIB busy, whose own database has no lyrics of the track: no
	// lyrics, though trying again may find some.
	cached = http.StatusNotFound
	_, err = c.Lyrics(context.Background(), track)
	if err == nil || err.Error() != "No lyrics found for this track." || !errors.Is(err, lyrics.ErrNotFoundYet) {
		t.Errorf("LRCLIB busy: %v", err)
	}
	status = http.StatusNotFound
	_, err = c.Lyrics(context.Background(), track)
	if err == nil || err.Error() != "No lyrics found for this track." || !errors.Is(err, lyrics.ErrNotFound) {
		t.Errorf("no lyrics: %v", err)
	}
}
