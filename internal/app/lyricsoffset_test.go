package app

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/lyrics"
)

// offsetFiles counts the files of the timings in dir.
func offsetFiles(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return len(entries)
}

// TestLyricsOffsetKept keeps a track's timing for the lyrics of its
// source alone, writes it a moment after the last change, or at once as
// MixMusic quits, reads it back in the next run, and keeps no file of a
// timing of 0.
func TestLyricsOffsetKept(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "lyrics-offsets")
	a := &App{lyricsOffset: newLyricsOffsets(dir)}
	a.lyricsOffset.wait = time.Hour
	c := &Catalog{a: a}
	unison := lyrics.Lyrics{Synced: true, Source: "Unison"}
	lrclib := lyrics.Lyrics{Synced: true, Source: "LRCLIB"}
	c.SetLyricsOffset(aSong, unison, 350*time.Millisecond)
	if got := c.LyricsOffset(aSong, unison); got != 350*time.Millisecond {
		t.Errorf("the timing set reads %v", got)
	}
	if got := c.LyricsOffset(aSong, lrclib); got != 0 {
		t.Errorf("LRCLIB's lyrics of the track move by %v", got)
	}
	if n := offsetFiles(t, dir); n != 0 {
		t.Errorf("%d files written at once", n)
	}
	a.lyricsOffset.flush()
	if n := offsetFiles(t, dir); n != 1 {
		t.Fatalf("%d files written as MixMusic quits", n)
	}
	data, err := os.ReadFile(a.lyricsOffset.path(aSong.ID))
	if err != nil {
		t.Fatal(err)
	}
	var k keptOffset
	if err := json.Unmarshal(data, &k); err != nil || k != (keptOffset{Track: aSong.ID, Source: "Unison", Ms: 350}) {
		t.Errorf("the file keeps %s (%v)", data, err)
	}
	next := &Catalog{a: &App{lyricsOffset: newLyricsOffsets(dir)}}
	if got := next.LyricsOffset(aSong, unison); got != 350*time.Millisecond {
		t.Errorf("the next run reads %v", got)
	}
	// Another source's timing takes the track's place.
	c.SetLyricsOffset(aSong, lrclib, -2*time.Second)
	if c.LyricsOffset(aSong, unison) != 0 || c.LyricsOffset(aSong, lrclib) != -2*time.Second {
		t.Errorf("Unison's %v, LRCLIB's %v", c.LyricsOffset(aSong, unison), c.LyricsOffset(aSong, lrclib))
	}
	// Past 120 s, it stops there.
	c.SetLyricsOffset(aSong, lrclib, -10*time.Minute)
	if got := c.LyricsOffset(aSong, lrclib); got != -settings.MaxLyricsOffset*time.Millisecond {
		t.Errorf("10 min later keeps %v", got)
	}
	c.SetLyricsOffset(aSong, lrclib, 0)
	a.lyricsOffset.flush()
	if n := offsetFiles(t, dir); n != 0 {
		t.Errorf("%d files of a timing of 0", n)
	}
	// Without a flush, the change is written once it rested.
	a.lyricsOffset.wait = 20 * time.Millisecond
	c.SetLyricsOffset(aSong, unison, 100*time.Millisecond)
	for deadline := time.Now().Add(5 * time.Second); offsetFiles(t, dir) == 0; time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the change wasn't written after the wait")
		}
	}
	a.lyricsOffset.flush()
}

// TestLyricsOffsetCaptions keeps the timing of a video's captions for the
// caption track of its language alone.
func TestLyricsOffsetCaptions(t *testing.T) {
	c := &Catalog{a: &App{lyricsOffset: newLyricsOffsets(t.TempDir())}}
	// Written before the folder goes.
	t.Cleanup(c.a.lyricsOffset.flush)
	en := lyrics.Lyrics{Synced: true, Source: CaptionsSource, Language: "en"}
	de := lyrics.Lyrics{Synced: true, Source: CaptionsSource, Language: "de"}
	c.SetLyricsOffset(aSong, en, time.Second)
	if c.LyricsOffset(aSong, en) != time.Second || c.LyricsOffset(aSong, de) != 0 {
		t.Errorf("English captions %v, German %v", c.LyricsOffset(aSong, en), c.LyricsOffset(aSong, de))
	}
}

// TestLyricsOffsetPrune keeps the timings of the tracks used last, those
// read or set, past the most it keeps.
func TestLyricsOffsetPrune(t *testing.T) {
	dir := t.TempDir()
	o := newLyricsOffsets(dir)
	o.keep, o.wait = 3, time.Hour
	day := time.Now().Add(-24 * time.Hour)
	for i, id := range []string{"yt:a", "yt:b", "yt:c"} {
		o.set(id, "Unison", 100)
		o.flush()
		at := day.Add(time.Duration(i) * time.Minute)
		os.Chtimes(o.path(id), at, at)
	}
	// The oldest is read in another run, and so used last but one.
	if k := newLyricsOffsets(dir).get("yt:a"); k.Ms != 100 {
		t.Fatalf("yt:a reads %+v", k)
	}
	o.set("yt:d", "Unison", 200)
	o.flush()
	if n := offsetFiles(t, dir); n != 3 {
		t.Errorf("%d timings kept, want 3", n)
	}
	fresh := newLyricsOffsets(dir)
	for id, want := range map[string]int{"yt:a": 100, "yt:b": 0, "yt:c": 100, "yt:d": 200} {
		if got := fresh.get(id).Ms; got != want {
			t.Errorf("%s keeps %d ms, want %d", id, got, want)
		}
	}
}

// TestLyricsMoved gives plugins the lyrics of a track with how far they
// show moved: the track's own timing of them and Settings' of every song.
func TestLyricsMoved(t *testing.T) {
	ls := newLyricsSources(t)
	ls.unison = status(http.StatusOK, unisonSynced)
	ls.c.a.lyricsOffset = newLyricsOffsets(t.TempDir())
	t.Cleanup(ls.c.a.lyricsOffset.flush)
	if err := ls.c.a.settings.Update(func(s *settings.Settings) error { s.LyricsOffset = -200; return nil }); err != nil {
		t.Fatal(err)
	}
	l, err := ls.c.LyricsMoved(context.Background(), aSong)
	if err != nil || l.Source != "Unison" || l.OffsetMs != -200 {
		t.Fatalf("lyrics = %+v, %v", l, err)
	}
	ls.c.SetLyricsOffset(aSong, l.Lyrics, 1500*time.Millisecond)
	l, _ = ls.c.LyricsMoved(context.Background(), aSong)
	data, err := json.Marshal(l)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Source   string `json:"source"`
		Synced   bool   `json:"synced"`
		OffsetMs *int   `json:"offsetMs"`
	}
	if err := json.Unmarshal(data, &got); err != nil || got.Source != "Unison" || !got.Synced || got.OffsetMs == nil || *got.OffsetMs != 1300 {
		t.Errorf("plugins get %s", data)
	}
	// Another track's lyrics move by Settings' alone.
	other := media.Track{ID: "yt:zyxwvutsrqp", Source: media.SourceYouTube, Title: "Other", Artists: []media.Ref{{Name: "Artist"}}, DurationMs: 200400}
	if l, err := ls.c.LyricsMoved(context.Background(), other); err != nil || l.OffsetMs != -200 {
		t.Errorf("another track's: %+v, %v", l, err)
	}
}
