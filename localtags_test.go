package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/tags"
	"github.com/fzn0x/mixmusic/internal/tagwrite"
)

// The songs of these tests are made up; their files are FLAC of the flac
// package's testdata, and a WAV of a tone.

// tagShell is a test shell with a folder of music on this computer,
// scanned: Shore, two songs of a folder without an album artist; Kites, a
// song of Ada Field's of another folder; and a voice memo, a WAV.
func tagShell(t *testing.T) (*shell, string) {
	t.Helper()
	s := newTestShell(t)
	music := filepath.Join(t.TempDir(), "Music")
	flac, err := os.ReadFile(filepath.Join("flac", "testdata", "stereo-16.flac"))
	if err != nil {
		t.Fatal(err)
	}
	for name, tg := range map[string]tagwrite.Tags{
		"Harbour Lights/Shore/01.flac": {Title: "Slow Tide", Artists: []string{"The Harbour Lights"}, Album: "Shore", Track: 1, Year: 2019},
		"Harbour Lights/Shore/02.flac": {Title: "Low Water", Artists: []string{"The Harbour Lights"}, Album: "Shore", Track: 2, Year: 2019},
		"Ada Field/Kites/01.flac":      {Title: "Paper Kites", Artists: []string{"Ada Field"}, Album: "Kites", AlbumArtists: []string{"Ada Field"}},
	} {
		path := filepath.Join(music, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, flac, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := tagwrite.Write(path, tg, tagwrite.All, tagwrite.Options{}); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(music, "Voice memo.wav"), localWAV("", "", "", "", 330), 0o644); err != nil {
		t.Fatal(err)
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

// fileTags reads the tags of a file of the music folder.
func fileTags(t *testing.T, path string) tags.Tags {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got, err := tags.Read(f)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// choose chooses item of the menu of what shows label.
func choose(t *testing.T, s *shell, tt interface {
	RightClick(string) error
	ChooseMenuItem(...string) error
	Menu() []string
}, label, item string) {
	t.Helper()
	if err := tt.RightClick(label); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem(item); err != nil {
		t.Fatalf("%v: the menu of %s holds %q", err, label, tt.Menu())
	}
	s.drain()
}

// albumOf is the album of the library of a title.
func albumOf(t *testing.T, s *shell, title string) media.Album {
	t.Helper()
	for _, a := range s.svc.LocalFiles.Albums() {
		if a.Title == title {
			return a
		}
	}
	t.Fatalf("no album %q", title)
	return media.Album{}
}

// TestEditInfo edits a song's tags from its menu: its title, genre and
// cover, which an image file gives; the library shows them at once, and
// the file holds them.
func TestEditInfo(t *testing.T) {
	s, music := tagShell(t)
	cover := filepath.Join(t.TempDir(), "cover.png")
	if err := os.WriteFile(cover, coverPNG(t, 40, 40), 0o644); err != nil {
		t.Fatal(err)
	}
	dialog := openFileDialog
	openFileDialog = func(mygo.OpenDialogOptions) ([]string, error) { return []string{cover}, nil }
	t.Cleanup(func() { openFileDialog = dialog })
	tt := testerOf(t, s, 1280, 1400)
	show(s, tt, "/library/computer")
	choose(t, s, tt, "Slow Tide", "Edit info…")
	e := &s.tagDialogs.edit
	until(t, s, tt, "the dialog", func() bool { return e.open && tt.HasText("edit info") })
	if e.form.title != "Slow Tide" || e.form.artists != "The Harbour Lights" || e.form.track != "1" || e.form.year != "2019" {
		t.Errorf("the dialog starts from %+v", e.form)
	}
	if !hasAll(t, tt, "Title", "Artists", "Album artist", "Track", "Disc", "Year", "Genre", "Cover", "None") {
		t.Fatalf("the dialog shows %q", tt.Texts())
	}
	click(t, s, tt, "choose an image…")
	until(t, s, tt, "the cover chosen", func() bool { return e.cover != nil && tt.HasText("remove the cover") })
	e.form.title, e.form.genre = "Slow Tide (Live)", "Folk"
	click(t, s, tt, "save")
	until(t, s, tt, "the save", func() bool { return tt.HasText("Saved the tags of Slow Tide") })
	if e.open {
		t.Error("the dialog stays open")
	}
	if !tt.HasText("Slow Tide (Live)") {
		t.Errorf("the library shows %q", tt.Texts())
	}
	got := fileTags(t, filepath.Join(music, "Harbour Lights", "Shore", "01.flac"))
	if got.Title != "Slow Tide (Live)" || got.Genre != "Folk" || got.Year != 2019 || got.Picture == nil {
		t.Errorf("the file holds %+v", got)
	}
}

// TestEditInfoProblems tells, before any dialog, why a WAV file's tags
// can't be changed, and in the dialog why a year isn't one.
func TestEditInfoProblems(t *testing.T) {
	s, _ := tagShell(t)
	tt := testerOf(t, s, 1280, 1400)
	show(s, tt, "/library/computer")
	choose(t, s, tt, "Voice memo", "Edit info…")
	until(t, s, tt, "the notice", func() bool {
		return tt.HasText("MixMusic can't change Voice memo.wav: MixMusic doesn't change the tags of this kind of file")
	})
	if s.tagDialogs.edit.open {
		t.Error("a dialog opened of a WAV file")
	}
	choose(t, s, tt, "Low Water", "Edit info…")
	e := &s.tagDialogs.edit
	until(t, s, tt, "the dialog", func() bool { return e.open })
	e.form.year = "19"
	click(t, s, tt, "save")
	if !tt.HasText("The year must be a number from 1000 to 9999") || !e.open {
		t.Errorf("a year of two digits: %q", tt.Texts())
	}
	click(t, s, tt, "cancel")
	if e.open {
		t.Error("cancel left the dialog open")
	}
}

// TestEditAlbum changes what the songs of an album share, from its tile,
// then from its page, which shows the album under its new title.
func TestEditAlbum(t *testing.T) {
	s, music := tagShell(t)
	tt := testerOf(t, s, 1280, 1400)
	show(s, tt, "/library/computer")
	s.computer.view = 1
	tt.Frame()
	choose(t, s, tt, "Shore", "Edit album…")
	e := &s.tagDialogs.edit
	until(t, s, tt, "the dialog", func() bool { return e.open && tt.HasText("edit album") })
	if e.form.album != "Shore" || e.form.year != "2019" || len(e.ids) != 2 || tt.HasText("Title") {
		t.Errorf("the dialog starts from %+v of %d songs: %q", e.form, len(e.ids), tt.Texts())
	}
	e.form.albumArtists = "The Harbour Lights"
	click(t, s, tt, "save")
	until(t, s, tt, "the save", func() bool { return tt.HasText("Saved Shore: 2 songs") })
	for _, name := range []string{"01.flac", "02.flac"} {
		if got := fileTags(t, filepath.Join(music, "Harbour Lights", "Shore", name)); !slices.Equal(got.AlbumArtists, []string{"The Harbour Lights"}) {
			t.Errorf("%s holds %+v", name, got)
		}
	}

	shore := albumOf(t, s, "Shore")
	show(s, tt, albumPath(shore.ID))
	until(t, s, tt, "the album's page", func() bool { return tt.HasText("Slow Tide") })
	if err := tt.Click("More"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("Edit album…"); err != nil {
		t.Fatalf("%v: the menu holds %q", err, tt.Menu())
	}
	until(t, s, tt, "the dialog", func() bool { return e.open })
	e.form.album = "Shore (Deluxe)"
	click(t, s, tt, "save")
	until(t, s, tt, "the save", func() bool { return tt.HasText("Saved Shore: 2 songs") && !e.open })
	deluxe := albumOf(t, s, "Shore (Deluxe)")
	until(t, s, tt, "the album's new page", func() bool {
		return s.router.Path() == albumPath(deluxe.ID) && tt.HasText("Slow Tide") && !tt.HasText("this album is no longer on this computer")
	})
}

// TestMoveToAlbum moves a song to an album of another folder, without an
// album artist, whose songs all take the one it shows, as the dialog
// says, then to a new album.
func TestMoveToAlbum(t *testing.T) {
	s, music := tagShell(t)
	tt := testerOf(t, s, 1280, 1400)
	show(s, tt, "/library/computer")
	choose(t, s, tt, "Paper Kites", "Move to album…")
	mv := &s.tagDialogs.move
	until(t, s, tt, "the dialog", func() bool { return mv.open && tt.HasText("move to album") })
	tt.Frame()
	tt.Type("sho")
	until(t, s, tt, "the albums found", func() bool {
		return mv.query == "sho" && len(mv.found) == 1 && mv.found[0].Title == "Shore" && tt.HasText("Shore, The Harbour Lights")
	})
	click(t, s, tt, "Shore, The Harbour Lights")
	if !tt.HasText("The songs of Shore have no album artist, and are in another folder: they all take The Harbour Lights as theirs, so that they stay together.") {
		t.Errorf("no note in %q", tt.Texts())
	}
	click(t, s, tt, "move")
	until(t, s, tt, "the move", func() bool { return tt.HasText("Moved Paper Kites to Shore") })
	whole, err := s.svc.Catalog.Album(t.Context(), albumOf(t, s, "Shore").ID)
	if err != nil || len(whole.Tracks) != 3 {
		t.Errorf("Shore holds %d songs: %v", len(whole.Tracks), err)
	}
	if got := fileTags(t, filepath.Join(music, "Harbour Lights", "Shore", "01.flac")); !slices.Equal(got.AlbumArtists, []string{"The Harbour Lights"}) {
		t.Errorf("a song of Shore holds %+v", got)
	}

	choose(t, s, tt, "Paper Kites", "Move to album…")
	until(t, s, tt, "the dialog", func() bool { return mv.open })
	tt.Frame()
	tt.Type("Odds and Ends")
	until(t, s, tt, "no album found", func() bool { return tt.HasText("No album of this computer is called that: Move makes a new one.") })
	click(t, s, tt, "move to a new album")
	until(t, s, tt, "the move", func() bool { return tt.HasText("Moved Paper Kites to Odds and Ends") })
	if got := fileTags(t, filepath.Join(music, "Ada Field", "Kites", "01.flac")); got.Album != "Odds and Ends" {
		t.Errorf("the song holds %+v", got)
	}
}

// TestTagForm reads the form back: only what changed, several artists
// apart by ";", and why a number isn't one.
func TestTagForm(t *testing.T) {
	was := formOf(tagwrite.Tags{Title: "Slow Tide", Artists: []string{"A", "B"}, Track: 3, Year: 2019})
	if was.artists != "A; B" || was.track != "3" || was.disc != "" {
		t.Errorf("the form of the tags: %+v", was)
	}
	if _, fields, err := was.tags(was); fields != 0 || err != nil {
		t.Errorf("unchanged: %v, %v", fields, err)
	}
	f := was
	f.artists, f.track, f.title = " A ;; C ", "", " Slow Tide "
	tg, fields, err := f.tags(was)
	if err != nil || fields != tagwrite.Artists|tagwrite.Track || !slices.Equal(tg.Artists, []string{"A", "C"}) || tg.Track != 0 {
		t.Errorf("%+v, %v, %v", tg, fields, err)
	}
	f.disc = "two"
	if _, _, err := f.tags(was); err == nil || !strings.Contains(err.Error(), "The disc must be a number") {
		t.Errorf("a disc of letters: %v", err)
	}
	if sentenceOf("couldn't save x: the disk is full") != "Couldn't save x: the disk is full." {
		t.Error(sentenceOf("couldn't save x: the disk is full"))
	}
}
