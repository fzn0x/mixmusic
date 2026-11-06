package app

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/builtin"
	"github.com/fzn0x/mixmusic/internal/localfiles"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// localApp is an app in a folder of its own, with a folder of music,
// whose index the test scans: its two tracks are FLAC of the flac
// package's testdata, of made-up tags.
func localApp(t *testing.T) (*App, string) {
	t.Helper()
	dir := t.TempDir()
	music := filepath.Join(dir, "music")
	if err := os.MkdirAll(filepath.Join(music, "Shore"), 0o755); err != nil {
		t.Fatal(err)
	}
	flacData, err := os.ReadFile(filepath.Join("..", "..", "flac", "testdata", "stereo-16.flac"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Shore/01 Slow Tide.flac", "Shore/02 Low Water.flac"} {
		if err := os.WriteFile(filepath.Join(music, filepath.FromSlash(name)), flacData, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a, err := New(Options{DataDir: filepath.Join(dir, "data"), CacheDir: filepath.Join(dir, "cache"), Builtin: builtin.FS, Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	st := a.settings.Get()
	st.LocalFiles = settings.LocalFiles{MusicFolders: []string{music}}
	if err := a.settings.Set(st); err != nil {
		t.Fatal(err)
	}
	return a, music
}

// scanned scans the app's folders, and waits for the scan to end.
func scanned(t *testing.T, a *App) {
	t.Helper()
	l := &LocalFiles{a}
	v := l.Version()
	l.Rescan()
	deadline := time.Now().Add(10 * time.Second)
	for l.Status().Scanning || l.Version() == v {
		if time.Now().After(deadline) {
			t.Fatalf("the scan didn't end: %+v", l.Status())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestNewScansNothing leaves the index alone until the app starts: tests
// of the app, and a new install, write no index.
func TestNewScansNothing(t *testing.T) {
	a, _ := localApp(t)
	a.apply(a.settings.Get())
	time.Sleep(50 * time.Millisecond)
	if _, err := os.Stat(filepath.Join(a.opts.DataDir, "local-files.json")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("an index before the start: %v", err)
	}
}

// TestLocalTracksPlay plays a track of the index from its file, without
// the network, which the engine opens, and one whose file is gone not at
// all, as missing from then on.
func TestLocalTracksPlay(t *testing.T) {
	a, music := localApp(t)
	scanned(t, a)
	l := &LocalFiles{a}
	tracks := l.Tracks()
	if len(tracks) != 2 || tracks[0].Source != media.SourceFile || tracks[0].Title != "01 Slow Tide" {
		t.Fatalf("tracks %+v", tracks)
	}
	src, err := (&Player{a}).Prepare(context.Background(), tracks[0])
	if err != nil || src.Mode != PlaybackStream || !src.Cached || !strings.Contains(src.URL, "/file/") {
		t.Fatalf("Prepare = %+v, %v", src, err)
	}
	if !a.playsOffline(tracks[0]) {
		t.Error("a track on this computer doesn't play offline")
	}
	f, err := a.openNative(context.Background(), src.URL, func() bool { return false })
	if err != nil {
		t.Fatal(err)
	}
	head := make([]byte, 4)
	if _, err := io.ReadFull(f, head); err != nil || string(head) != "fLaC" {
		t.Errorf("the engine read %q, %v", head, err)
	}
	f.Close()
	os.Remove(filepath.Join(music, "Shore", "02 Low Water.flac"))
	if _, err := (&Player{a}).PrepareStream(context.Background(), tracks[1]); !errors.Is(err, errFileMissing) {
		t.Errorf("a file gone: %v", err)
	}
	if !l.Missing(tracks[1].ID) || l.Missing(tracks[0].ID) {
		t.Error("the file gone isn't missing, or the other is")
	}
	// A rescan drops it.
	scanned(t, a)
	if n := len(l.Tracks()); n != 1 {
		t.Errorf("%d tracks after the rescan", n)
	}
}

// TestLocalTracksGoNowhere sends a track on this computer to no service:
// no recording of it on YouTube Music, no radio, no cast device, and no
// scrobble without an artist.
func TestLocalTracksGoNowhere(t *testing.T) {
	a, _ := localApp(t)
	scanned(t, a)
	tr := (&LocalFiles{a}).Tracks()[0]
	if _, err := a.matcher.Match(context.Background(), tr); err == nil {
		t.Error("a track on this computer was matched")
	}
	if _, ok := a.matcher.Matched(tr); ok {
		t.Error("a track on this computer has a match")
	}
	if _, err := (&Catalog{a}).Radio(context.Background(), tr); err == nil {
		t.Error("a radio of a track on this computer started")
	}
	src, _ := (&Player{a}).Prepare(context.Background(), tr)
	if _, _, err := videoOf(src.URL); !errors.Is(err, errLocalCast) {
		t.Errorf("cast: %v", err)
	}
	if id, _ := lastfmTrack(NowPlaying{Track: &tr, DurationMs: 200}); id != "" {
		t.Error("a track of no artist scrobbles")
	}
	named := tr
	named.Artists = []media.Ref{{Name: "Ada Field"}}
	if id, lt := lastfmTrack(NowPlaying{Track: &named, DurationMs: 200}); id == "" || lt.Artist != "Ada Field" {
		t.Error("a track of an artist doesn't scrobble, as the settings ask")
	}
}

// TestLocalOpen reads files the user opens, and the music of a folder
// dropped, in the order of their names, but what doesn't play.
func TestLocalOpen(t *testing.T) {
	a, music := localApp(t)
	junk := filepath.Join(music, "notes.mp3")
	os.WriteFile(junk, []byte("not music"), 0o644)
	got := (&LocalFiles{a}).Open([]string{music, filepath.Join(music, "nothing.flac")})
	if len(got.Tracks) != 2 || got.Skipped != 1 || got.Tracks[0].Title != "01 Slow Tide" {
		t.Errorf("opened %+v", got)
	}
	if p, ok := localfiles.PathOf(got.Tracks[1].ID); !ok || filepath.Base(p) != "02 Low Water.flac" {
		t.Errorf("the second names %q", p)
	}
}

// TestLocalCatalog opens the pages of an album and an artist of the
// index, and of none.
func TestLocalCatalog(t *testing.T) {
	a, music := localApp(t)
	scanned(t, a)
	l := &LocalFiles{a}
	if len(l.Albums()) != 0 {
		t.Errorf("albums of untagged files: %+v", l.Albums())
	}
	// Untagged, the files have no album: one that does is one opened.
	if _, err := (&Catalog{a}).Album(context.Background(), "file:album:eA"); err == nil {
		t.Error("an album the index doesn't hold")
	}
	if _, err := (&Catalog{a}).Artist(context.Background(), "file:artist:eA"); err == nil {
		t.Error("an artist the index doesn't hold")
	}
	_ = music
}

// TestLocalNotices shows the licenses of the decoders of the music on this
// computer that MixMusic didn't write: go-mp3's and jfreymuth/vorbis's.
func TestLocalNotices(t *testing.T) {
	for _, want := range []struct{ name, text string }{
		{"go-mp3", "Apache License"},
		{"jfreymuth/vorbis", "Copyright (c) 2016 Johann Freymuth"},
	} {
		found := false
		for _, n := range (&System{}).Notices() {
			if strings.HasPrefix(n.Name, want.name) {
				found = strings.Contains(n.Text, want.text)
			}
		}
		if !found {
			t.Errorf("no notice of %s with its license", want.name)
		}
	}
}
