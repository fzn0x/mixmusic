package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/localfiles"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/tagwrite"
)

// The songs of these tests are made up; their files are FLAC of the flac
// package's testdata and an MP3 of the audio package's.

// copiesShell is a test shell with a folder of music on this computer,
// scanned: Paper Kites, as a FLAC of the album Kites and an MP3 of its
// single, and Low Water, a FLAC of another album.
func copiesShell(t *testing.T) (*shell, string) {
	t.Helper()
	s := newTestShell(t)
	music := filepath.Join(t.TempDir(), "Music")
	files := map[string]struct {
		from string
		tags tagwrite.Tags
	}{
		"Ada Field/Kites/01.flac": {filepath.Join("flac", "testdata", "stereo-16.flac"),
			tagwrite.Tags{Title: "Paper Kites", Artists: []string{"Ada Field"}, Album: "Kites", AlbumArtists: []string{"Ada Field"}, Track: 1}},
		"Ada Field/Single/01.mp3": {filepath.Join("internal", "audio", "testdata", "tone.mp3"),
			tagwrite.Tags{Title: "Paper Kites", Artists: []string{"Ada Field"}, Album: "Paper Kites - Single", AlbumArtists: []string{"Ada Field"}, Track: 1}},
		"Harbour Lights/Shore/02.flac": {filepath.Join("flac", "testdata", "stereo-16.flac"),
			tagwrite.Tags{Title: "Low Water", Artists: []string{"The Harbour Lights"}, Album: "Shore", Track: 2}},
	}
	for name, f := range files {
		path := filepath.Join(music, filepath.FromSlash(name))
		data, err := os.ReadFile(f.from)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := tagwrite.Write(path, f.tags, tagwrite.All, tagwrite.Options{}); err != nil {
			t.Fatal(err)
		}
	}
	st := s.svc.Preferences.Get()
	st.LocalFiles = settings.LocalFiles{MusicFolders: []string{music}}
	if _, err := s.svc.Preferences.Set(st); err != nil {
		t.Fatal(err)
	}
	s.refresh()
	v := s.svc.LocalFiles.Version()
	s.svc.LocalFiles.Rescan()
	for deadline := time.Now().Add(10 * time.Second); s.svc.LocalFiles.Status().Scanning || s.svc.LocalFiles.Version() == v; {
		if time.Now().After(deadline) {
			t.Fatal("the scan didn't end")
		}
		time.Sleep(10 * time.Millisecond)
	}
	return s, music
}

// kitesSong is the song of Paper Kites, with its copies.
func kitesSong(t *testing.T, s *shell) localfiles.Song {
	t.Helper()
	for _, tr := range s.svc.LocalFiles.Tracks() {
		if tr.Title == "Paper Kites" {
			song, ok := s.svc.LocalFiles.Song(tr.ID)
			if !ok {
				t.Fatalf("Paper Kites is no song with copies: %q", tr.ID)
			}
			return song
		}
	}
	t.Fatal("no Paper Kites")
	return localfiles.Song{}
}

// TestCopiesShowOnce shows the copies of a song once in Library › On this
// computer and in the search, and plays the copy chosen in Play this copy
// of its menu.
func TestCopiesShowOnce(t *testing.T) {
	s, music := copiesShell(t)
	tt := testerOf(t, s, 1280, 1400)
	show(s, tt, "/library/computer")
	until(t, s, tt, "the songs", func() bool { return tt.HasText("Low Water") })
	if n := textCount(tt, "Paper Kites"); n != 1 {
		t.Errorf("the songs show Paper Kites %d times: %q", n, tt.Texts())
	}
	if found := s.svc.LocalFiles.Search("paper kites", 10).Tracks; len(found) != 1 || !localfiles.IsSong(found[0].ID) {
		t.Errorf("the search finds %+v", found)
	}
	song := kitesSong(t, s)
	if len(song.Copies) != 2 || song.Copies[0].Codec != "FLAC" || !song.Copies[0].Best || song.Copies[1].Codec != "MP3" {
		t.Fatalf("the song's copies are %+v", song.Copies)
	}
	labels := copyLabels(song.Copies)
	if want := "FLAC · 16-bit · 44.1 kHz · Kites"; labels[0] != want {
		t.Errorf("the FLAC's item is %q, want %q", labels[0], want)
	}
	if !strings.HasPrefix(labels[1], "MP3 · ") || !strings.HasSuffix(labels[1], " kbit/s · 22.05 kHz · Paper Kites - Single") {
		t.Errorf("the MP3's item is %q", labels[1])
	}
	if err := tt.RightClick("Paper Kites"); err != nil {
		t.Fatal(err)
	}
	if menu := tt.Menu(); !slices.Contains(menu, "Play this copy") || !slices.Contains(menu, "2 copies…") || slices.Contains(menu, "Link a copy…") {
		t.Errorf("the song's menu holds %q", menu)
	}
	if err := tt.ChooseMenuItem("Play this copy", labels[1]); err != nil {
		t.Fatalf("%v: the menu holds %q", err, tt.Menu())
	}
	mp3 := filepath.Join(music, "Ada Field", "Single", "01.mp3")
	s.settle(t, func(st player.State) bool {
		cur, ok := st.Current()
		return ok && cur.Track.ID == localfiles.TrackID(mp3)
	})
}

// TestCopiesDialog shows a song's copies side by side, the one that plays
// marked, unlinks one, which shows apart then, and links it again from the
// search of the dialog.
func TestCopiesDialog(t *testing.T) {
	s, music := copiesShell(t)
	tt := testerOf(t, s, 1280, 1400)
	show(s, tt, "/library/computer")
	until(t, s, tt, "the songs", func() bool { return tt.HasText("Paper Kites") })
	choose(t, s, tt, "Paper Kites", "2 copies…")
	v := &s.copies
	until(t, s, tt, "the dialog", func() bool { return v.open && tt.HasText("copies of Paper Kites") })
	if !hasAll(t, tt, "FLAC · 16-bit · 44.1 kHz", "Plays", "Kites", "Paper Kites - Single", filepath.Join(music, "Ada Field", "Kites", "01.flac")) {
		t.Fatalf("the dialog shows %q", tt.Texts())
	}
	// The first Unlink is the FLAC's: the song keeps the MP3.
	click(t, s, tt, "Unlink this copy")
	until(t, s, tt, "the unlink", func() bool { return tt.HasText("Unlinked the copy in Kites") })
	if n := textCount(tt, "Paper Kites"); n < 2 {
		t.Errorf("unlinked, the songs show Paper Kites %d times", n)
	}
	if tt.HasText("Unlink this copy") || tt.HasText("Plays") {
		t.Errorf("a song of one copy offers to unlink it: %q", tt.Texts())
	}
	v.query = "paper kites"
	until(t, s, tt, "the search", func() bool { return tt.HasText("Link Paper Kites, Ada Field") })
	click(t, s, tt, "Link Paper Kites, Ada Field")
	until(t, s, tt, "the link", func() bool { return tt.HasText("Linked Paper Kites as a copy") })
	if song := kitesSong(t, s); len(song.Copies) != 2 || !slices.ContainsFunc(song.Copies, func(c localfiles.Copy) bool { return c.Linked }) {
		t.Errorf("linked again, the song's copies are %+v", song.Copies)
	}
	click(t, s, tt, "close")
	if v.open {
		t.Error("close left the dialog open")
	}
}

// TestLinkACopy links a file without copies to another song from its menu.
func TestLinkACopy(t *testing.T) {
	s, _ := copiesShell(t)
	tt := testerOf(t, s, 1280, 1400)
	show(s, tt, "/library/computer")
	until(t, s, tt, "the songs", func() bool { return tt.HasText("Low Water") })
	choose(t, s, tt, "Low Water", "Link a copy…")
	until(t, s, tt, "the dialog", func() bool { return s.copies.open && tt.HasText("copies of Low Water") })
	s.copies.query = "kites"
	until(t, s, tt, "the search", func() bool { return tt.HasText("Link Paper Kites, Ada Field") })
	click(t, s, tt, "Link Paper Kites, Ada Field")
	until(t, s, tt, "the link", func() bool { return tt.HasText("Linked Paper Kites as a copy") })
	if song := kitesSong(t, s); len(song.Copies) != 3 {
		t.Errorf("the song's copies are %+v", song.Copies)
	}
	if tracks := s.svc.LocalFiles.Tracks(); len(tracks) != 1 {
		t.Errorf("the songs are %+v", tracks)
	}
}

// TestCopiesTrackInfo shows a song's copies in its Track info, side by
// side, the one that plays marked.
func TestCopiesTrackInfo(t *testing.T) {
	s, music := copiesShell(t)
	tt := testerOf(t, s, 1280, 1400)
	show(s, tt, "/library/computer")
	until(t, s, tt, "the songs", func() bool { return tt.HasText("Paper Kites") })
	choose(t, s, tt, "Paper Kites", "Track info")
	until(t, s, tt, "the dialog", func() bool { return tt.HasText("its copies") })
	flac := filepath.Join(music, "Ada Field", "Kites", "01.flac")
	if !hasAll(t, tt, "Plays", "Copy", "FLAC · 16-bit · 44.1 kHz · Kites\n"+flac) {
		t.Errorf("Track info shows %q", tt.Texts())
	}
	if !tt.HasText(flac) {
		t.Errorf("Track info doesn't tell the file that plays: %q", tt.Texts())
	}
}
