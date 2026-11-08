package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/localfiles"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// watchedShell is a test shell with a folder of music of a song, scanned,
// whose watch answers as watch does.
func watchedShell(t *testing.T, watch localfiles.WatchFolder) (*shell, string) {
	t.Helper()
	s := newTestShellOf(t, func(o *app.Options) { o.WatchMusic = watch })
	music := filepath.Join(t.TempDir(), "Music")
	if err := os.MkdirAll(music, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(music, "01.wav"), localWAV("Slow Tide", "The Harbour Lights", "Shore", "2019", 220), 0o644); err != nil {
		t.Fatal(err)
	}
	st := s.svc.Preferences.Get()
	st.LocalFiles = settings.LocalFiles{MusicFolders: []string{music}}
	if _, err := s.svc.Preferences.Set(st); err != nil {
		t.Fatal(err)
	}
	s.refresh()
	s.svc.LocalFiles.Rescan()
	return s, music
}

// TestComputerUnwatched tells, in Settings › Playback, of a folder of
// music MixMusic can't watch for changes, as on a network drive, and that
// a rescan shows them.
func TestComputerUnwatched(t *testing.T) {
	s, music := watchedShell(t, func(string, localfiles.Changes) (func(), error) {
		return nil, localfiles.ErrNetworkDrive
	})
	tt := testerOf(t, s, 1280, 4000)
	show(s, tt, "/settings/playback")
	want := "MixMusic can't watch " + music + " for changes: it's on a network drive. Rescan to see what changed in it."
	until(t, s, tt, "the folder unwatched", func() bool { return tt.HasText(want) && tt.HasText("1 track") })
	// The search of the settings finds it.
	show(s, tt, "/settings/playback?q=network+drive")
	until(t, s, tt, "the folder unwatched, searched", func() bool { return tt.HasText(want) })
}

// TestComputerWatched tells of no folder MixMusic watches.
func TestComputerWatched(t *testing.T) {
	var watched atomic.Bool
	s, _ := watchedShell(t, func(string, localfiles.Changes) (func(), error) {
		watched.Store(true)
		return func() {}, nil
	})
	tt := testerOf(t, s, 1280, 4000)
	show(s, tt, "/settings/playback")
	until(t, s, tt, "the folder watched", func() bool { return watched.Load() && tt.HasText("1 track") })
	for _, text := range tt.Texts() {
		if strings.Contains(text, "can't watch") {
			t.Errorf("a folder watched is told of: %q", text)
		}
	}
}
