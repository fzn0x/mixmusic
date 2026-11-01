package app

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/fzn0x/mixmusic/internal/match"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/store"
	"github.com/fzn0x/mixmusic/internal/youtube"
	"github.com/fzn0x/mixmusic/lyrics"
)

// lyricsSources are Unison, LRCLIB and YouTube Music answering as the
// test has them, for a catalog asking them for lyrics.
type lyricsSources struct {
	c *Catalog
	// unison and lrclib answer the lookups, which the counts count.
	unison, lrclib      http.HandlerFunc
	unisonAsks, lrcAsks atomic.Int32
	// unisonQuery is the query of Unison's last lookup.
	unisonQuery atomic.Value
}

func newLyricsSources(t *testing.T) *lyricsSources {
	t.Helper()
	ls := &lyricsSources{
		unison: status(http.StatusNotFound, `{"success":false,"code":"NOT_FOUND"}`),
		lrclib: status(http.StatusNotFound, ""),
	}
	unison := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ls.unisonAsks.Add(1)
		ls.unisonQuery.Store(r.URL.RawQuery)
		ls.unison(w, r)
	}))
	t.Cleanup(unison.Close)
	lrclib := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ls.lrcAsks.Add(1)
		ls.lrclib(w, r)
	}))
	t.Cleanup(lrclib.Close)
	// YouTube Music has no lyrics.
	yt := httptest.NewServer(status(http.StatusNotFound, ""))
	t.Cleanup(yt.Close)
	a := &App{lyrics: lyrics.New("test"), unison: lyrics.NewUnison("test"), youtube: youtube.New(), lyricsMemo: newMemo[lyrics.Lyrics](4)}
	a.lyrics.BaseURL = lrclib.URL
	a.unison.BaseURL = unison.URL
	a.youtube.BaseURL = yt.URL
	ls.c = &Catalog{a: a}
	return ls
}

// status answers with code and body.
func status(code int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(code)
		w.Write([]byte(body))
	}
}

const (
	unisonSynced = `{"success":true,"data":{"format":"ttml","syncType":"linesync","lyrics":"<tt><body><div><p begin=\"0:01.000\">From Unison</p></div></body></tt>"}}`
	unisonPlain  = `{"success":true,"data":{"format":"plain","syncType":"plain","lyrics":"Plain from Unison\nSecond line\nThird line"}}`
	unisonDown   = `{"success":false,"error":"Internal error"}`
	unisonMiss   = `{"success":false,"code":"NOT_FOUND"}`
	lrclibSynced = `{"syncedLyrics":"[00:01.00]From LRCLIB"}`
)

var aSong = media.Track{ID: "yt:abcdefghijk", Source: media.SourceYouTube, Title: "Song (Official Video)",
	Artists: []media.Ref{{Name: "Artist"}, {Name: "Guest"}}, Album: &media.Ref{Name: "Album"}, DurationMs: 200400}

// TestLyricsUnisonFirst takes Unison's lyrics, which LRCLIB's then don't
// replace, and credits Unison with its address.
func TestLyricsUnisonFirst(t *testing.T) {
	ls := newLyricsSources(t)
	ls.unison = status(http.StatusOK, unisonSynced)
	ls.lrclib = status(http.StatusOK, lrclibSynced)
	l, err := ls.c.Lyrics(context.Background(), aSong)
	if err != nil || !l.Synced || l.Lines[0].Text != "From Unison" || l.Source != "Unison" || l.SourceURL != "https://unison.boidu.dev" {
		t.Fatalf("lyrics = %+v, %v", l, err)
	}
	if n := ls.lrcAsks.Load(); n != 0 {
		t.Errorf("LRCLIB asked %d times for lyrics Unison has", n)
	}
	// Found once, they aren't asked for again.
	if _, err := ls.c.Lyrics(context.Background(), aSong); err != nil || ls.unisonAsks.Load() != 1 {
		t.Errorf("Unison asked %d times, %v", ls.unisonAsks.Load(), err)
	}
}

// TestLyricsSyncedFirst shows LRCLIB's synced lyrics over Unison's plain
// ones, and Unison's plain ones over none.
func TestLyricsSyncedFirst(t *testing.T) {
	ls := newLyricsSources(t)
	ls.unison = status(http.StatusOK, unisonPlain)
	ls.lrclib = status(http.StatusOK, lrclibSynced)
	if l, err := ls.c.Lyrics(context.Background(), aSong); err != nil || !l.Synced || l.Source != "LRCLIB" || l.SourceURL != "" {
		t.Errorf("synced from LRCLIB: lyrics = %+v, %v", l, err)
	}

	ls = newLyricsSources(t)
	ls.unison = status(http.StatusOK, unisonPlain)
	if l, err := ls.c.Lyrics(context.Background(), aSong); err != nil || l.Synced || l.Source != "Unison" || len(l.Lines) != 3 {
		t.Errorf("plain from Unison: lyrics = %+v, %v", l, err)
	}
}

// TestLyricsPastUnison asks LRCLIB for a track Unison has no lyrics of, or
// while Unison is down.
func TestLyricsPastUnison(t *testing.T) {
	for name, unison := range map[string]http.HandlerFunc{
		"not found": status(http.StatusNotFound, unisonMiss),
		"down":      status(http.StatusBadGateway, unisonDown),
	} {
		ls := newLyricsSources(t)
		ls.unison = unison
		ls.lrclib = status(http.StatusOK, lrclibSynced)
		if l, err := ls.c.Lyrics(context.Background(), aSong); err != nil || l.Source != "LRCLIB" || l.Lines[0].Text != "From LRCLIB" {
			t.Errorf("Unison %s: lyrics = %+v, %v", name, l, err)
		}
	}
}

// TestLyricsInPlainWords tells a track without lyrics from lyrics that
// can't load, in words for the lyrics panel rather than LRCLIB's or
// Unison's.
func TestLyricsInPlainWords(t *testing.T) {
	cached := http.StatusServiceUnavailable
	lrclibBusy := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/get-cached" {
			w.WriteHeader(cached)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	const (
		none = "No lyrics found for this track."
		cant = "Lyrics can't be loaded right now. Check your connection, or try again in a moment."
	)
	for _, c := range []struct {
		name           string
		unison, lrclib http.HandlerFunc
		text           string
		is             error
	}{
		// Neither could be reached.
		{"both down", status(http.StatusBadGateway, unisonDown), lrclibBusy, cant, lyrics.ErrUnavailable},
		// One has none, the other couldn't be asked: none found, though
		// trying again may find some.
		{"LRCLIB down", status(http.StatusNotFound, unisonMiss), lrclibBusy, none, lyrics.ErrNotFoundYet},
		{"Unison down", status(http.StatusBadGateway, unisonDown), status(http.StatusNotFound, ""), none, lyrics.ErrNotFoundYet},
		{"Unison out of reach", nil, status(http.StatusNotFound, ""), none, lyrics.ErrNotFoundYet},
		{"none anywhere", status(http.StatusNotFound, unisonMiss), status(http.StatusNotFound, ""), none, lyrics.ErrNotFound},
	} {
		ls := newLyricsSources(t)
		if c.unison == nil {
			ls.c.a.unison.BaseURL = "http://127.0.0.1:1"
		}
		ls.unison, ls.lrclib = c.unison, c.lrclib
		_, err := ls.c.Lyrics(context.Background(), aSong)
		if err == nil || err.Error() != c.text || !errors.Is(err, c.is) {
			t.Errorf("%s: %v", c.name, err)
		}
		// The panel offers to try again for all but lyrics none has.
		if c.is != lyrics.ErrNotFound && errors.Is(err, lyrics.ErrNotFound) {
			t.Errorf("%s: %v reads as no lyrics anywhere", c.name, err)
		}
	}

	// LRCLIB busy, whose own database has no lyrics of the track: no
	// lyrics, though trying again may find some.
	ls := newLyricsSources(t)
	ls.lrclib, cached = lrclibBusy, http.StatusNotFound
	_, err := ls.c.Lyrics(context.Background(), aSong)
	if err == nil || err.Error() != none || !errors.Is(err, lyrics.ErrNotFoundYet) {
		t.Errorf("LRCLIB busy: %v", err)
	}
}

// noSearch is YouTube Music finding nothing.
type noSearch struct{}

func (noSearch) Search(context.Context, string, youtube.Filter) ([]media.Item, error) {
	return nil, nil
}

// TestLyricsByVideo asks Unison for a YouTube Music track by its video ID,
// for a Spotify track by its match's once there is one, and else by the
// track's title, first artist, album and length.
func TestLyricsByVideo(t *testing.T) {
	ls := newLyricsSources(t)
	ls.c.Lyrics(context.Background(), aSong)
	if got := ls.unisonQuery.Load(); got != "v=abcdefghijk" {
		t.Errorf("a YouTube Music track: query %q", got)
	}

	known, err := store.Open(filepath.Join(t.TempDir(), "matches.json"), map[string]media.Track{})
	if err != nil {
		t.Fatal(err)
	}
	ls.c.a.matcher = match.New(noSearch{}, known)
	ls.c.Lyrics(context.Background(), media.Track{ID: "spotify:1", Source: media.SourceSpotify, Title: "Song",
		Artists: []media.Ref{{Name: "Artist"}, {Name: "Guest"}}, Album: &media.Ref{Name: "Album"}, DurationMs: 200400})
	if got := ls.unisonQuery.Load(); got != "album=Album&artist=Artist&duration=200&song=Song" {
		t.Errorf("a Spotify track never matched: query %q", got)
	}
	known.Set(map[string]media.Track{"spotify:2": {ID: "yt:lmnopqrstuv", Source: media.SourceYouTube}})
	ls.c.Lyrics(context.Background(), media.Track{ID: "spotify:2", Source: media.SourceSpotify, Title: "Song", Artists: []media.Ref{{Name: "Artist"}}})
	if got := ls.unisonQuery.Load(); got != "v=lmnopqrstuv" {
		t.Errorf("a Spotify track matched: query %q", got)
	}
}
