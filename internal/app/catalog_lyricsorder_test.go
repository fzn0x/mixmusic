package app

import (
	"context"
	"net/http"
	"testing"

	"github.com/fzn0x/mixmusic/internal/settings"
)

// withSources has the catalog of ls ask the sources of lyrics in the
// order and with the switches of sources, words first if words.
func withSources(t *testing.T, ls *lyricsSources, words bool, sources ...settings.LyricsSource) {
	t.Helper()
	err := ls.c.a.settings.Update(func(s *settings.Settings) error {
		s.LyricsSources, s.LyricsPreferWords = sources, words
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestLyricsInTheUsersOrder asks the sources in the order the user put
// them, and stops at the first synced lyrics.
func TestLyricsInTheUsersOrder(t *testing.T) {
	ls := newLyricsSources(t)
	ls.unison = status(http.StatusOK, unisonSynced)
	ls.lrclib = status(http.StatusOK, lrclibSynced)
	withSources(t, ls, false, settings.LyricsSource{Key: settings.LyricsLRCLIB}, settings.LyricsSource{Key: settings.LyricsBetterLyrics},
		settings.LyricsSource{Key: settings.LyricsUnison})
	l, err := ls.c.Lyrics(context.Background(), aSong)
	if err != nil || l.Source != "LRCLIB" || !l.Synced {
		t.Fatalf("lyrics = %+v, %v", l, err)
	}
	if n := ls.unisonAsks.Load(); n != 0 {
		t.Errorf("Unison asked %d times after LRCLIB had synced lyrics", n)
	}
}

// TestLyricsSourceOff never asks a source the user switched off, even
// when no other has lyrics.
func TestLyricsSourceOff(t *testing.T) {
	ls := newLyricsSources(t)
	ls.unison = status(http.StatusOK, unisonSynced)
	withSources(t, ls, false, settings.LyricsSource{Key: settings.LyricsUnison, Off: true})
	if _, err := ls.c.Lyrics(context.Background(), aSong); err == nil || err.Error() != "No lyrics found for this track." {
		t.Errorf("lyrics: %v", err)
	}
	if n := ls.unisonAsks.Load(); n != 0 {
		t.Errorf("Unison, switched off, asked %d times", n)
	}
	if n := ls.lrcAsks.Load(); n == 0 {
		t.Error("LRCLIB not asked")
	}
}

// TestLyricsLongerPlain shows the longer of two plain answers, and of two
// as long, the one of the source asked first.
func TestLyricsLongerPlain(t *testing.T) {
	ls := newLyricsSources(t)
	ls.unison = status(http.StatusOK, `{"success":true,"data":{"format":"plain","lyrics":"One\nTwo"}}`)
	ls.lrclib = status(http.StatusOK, `{"plainLyrics":"Uno\nDos\nTres"}`)
	if l, err := ls.c.Lyrics(context.Background(), aSong); err != nil || l.Source != "LRCLIB" || len(l.Lines) != 3 {
		t.Errorf("the longer: %+v, %v", l, err)
	}
	ls = newLyricsSources(t)
	ls.unison = status(http.StatusOK, `{"success":true,"data":{"format":"plain","lyrics":"One\nTwo"}}`)
	ls.lrclib = status(http.StatusOK, `{"plainLyrics":"Uno\nDos"}`)
	if l, err := ls.c.Lyrics(context.Background(), aSong); err != nil || l.Source != "Unison" {
		t.Errorf("as long: %+v, %v", l, err)
	}
}

// TestLyricsWordsFirst takes lyrics timed word by word from a later
// source over lyrics timed line by line from an earlier one only while
// the user prefers them: else the later source isn't asked.
func TestLyricsWordsFirst(t *testing.T) {
	order := []settings.LyricsSource{{Key: settings.LyricsUnison}, {Key: settings.LyricsBetterLyrics}}
	for _, words := range []bool{false, true} {
		ls := newLyricsSources(t)
		asks := withBetterLyrics(t, ls, status(http.StatusOK, betterLyricsWordSynced))
		ls.unison = status(http.StatusOK, unisonSynced)
		withSources(t, ls, words, order...)
		l, err := ls.c.Lyrics(context.Background(), aSong)
		want, wantAsks := "Unison", int32(0)
		if words {
			want, wantAsks = "Better Lyrics", 1
		}
		if err != nil || l.Source != want || asks.Load() != wantAsks {
			t.Errorf("words first %v: lyrics from %q, %v; Better Lyrics asked %d times", words, l.Source, err, asks.Load())
		}
	}
}

// TestLyricsChangedSources looks a track up again once the user changed
// its sources, rather than show the lyrics kept of it.
func TestLyricsChangedSources(t *testing.T) {
	ls := newLyricsSources(t)
	ls.unison = status(http.StatusOK, unisonSynced)
	ls.lrclib = status(http.StatusOK, lrclibSynced)
	if l, err := ls.c.Lyrics(context.Background(), aSong); err != nil || l.Source != "Unison" {
		t.Fatalf("lyrics = %+v, %v", l, err)
	}
	withSources(t, ls, false, settings.LyricsSource{Key: settings.LyricsUnison, Off: true})
	if l, err := ls.c.Lyrics(context.Background(), aSong); err != nil || l.Source != "LRCLIB" {
		t.Errorf("after Unison was switched off: %+v, %v", l, err)
	}
}
