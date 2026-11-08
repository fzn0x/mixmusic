package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/localfiles/localfilestest"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// rescanned reads the folders of music again, and waits for the scan.
func rescanned(t *testing.T, s *shell) {
	t.Helper()
	v := s.svc.LocalFiles.Version()
	s.svc.LocalFiles.Rescan()
	for deadline := time.Now().Add(10 * time.Second); s.svc.LocalFiles.Status().Scanning || s.svc.LocalFiles.Version() == v; {
		if time.Now().After(deadline) {
			t.Fatal("the scan didn't end")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestComputerUnreadFolder tells, in Settings › Playback, of a folder in
// a folder of music that the scan couldn't read, whose tracks stay in the
// library as they were.
func TestComputerUnreadFolder(t *testing.T) {
	s, music := computerShell(t, settings.LayoutTiles, "mix")
	localfilestest.DenyRead(t, filepath.Join(music, "Ada Field", "Kites"))
	rescanned(t, s)
	tt := testerOf(t, s, 1280, 4000)
	show(s, tt, "/settings/playback")
	for _, want := range []string{music, "6 tracks", "1 folder in it can't be read: its tracks stay as they were"} {
		if !tt.HasText(want) {
			t.Errorf("no %q in Settings: %q", want, tt.Texts())
		}
	}
	show(s, tt, "/library/computer")
	if !tt.HasText("Paper Kites") || !tt.HasText("String Theory") {
		t.Errorf("the tracks of the folder that can't be read: %q", tt.Texts())
	}
}

// TestUnreadLine counts the folders that can't be read.
func TestUnreadLine(t *testing.T) {
	if got := unreadLine(3); got != "3 folders in it can't be read: their tracks stay as they were" {
		t.Errorf("3 folders: %q", got)
	}
}
