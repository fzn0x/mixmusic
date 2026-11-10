package localfiles

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

// TestReread reads a file again at once, though its size and time of
// change are those the scan saw, and tells the watchers; a file the index
// doesn't hold, it leaves alone.
func TestReread(t *testing.T) {
	ti := newIndex(t)
	path := ti.write(t, "Shore/01.mp3", "Slow Tide|The Harbour Lights|Shore||1", time.Hour)
	ti.scan(t)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// The same size and time of change, other tags.
	if err := os.WriteFile(path, []byte("Fast Tide|The Harbour Lights|Shore||1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	var told atomic.Int32
	defer ti.Watch(func() { told.Add(1) })()
	v := ti.Version()
	ti.Reread([]string{path, ti.dir + "/elsewhere.mp3"})
	if e, _ := ti.Entry(path); e.Title != "Fast Tide" {
		t.Errorf("the title is %q", e.Title)
	}
	if ti.Version() == v || told.Load() == 0 {
		t.Errorf("version %d, was %d; told %d times", ti.Version(), v, told.Load())
	}
	if got := titles(ti.Tracks()); !slices.Equal(got, []string{"Fast Tide"}) {
		t.Errorf("the tracks are %q", got)
	}
}

// TestMoveTo tells what the tags of a song are to say for it to join an
// album: its title and album artists, or, of an album grouped by its
// folder, its title, with the album artist it shows written to its songs
// too when the song is in another folder.
func TestMoveTo(t *testing.T) {
	ti := newIndex(t)
	song := ti.write(t, "Loose/song.mp3", "Lost Song|Ada Field|Singles|Ada Field|1", time.Hour)
	ti.write(t, "Kites/01.mp3", "Paper Kites|Ada Field|Kites|Ada Field|1", time.Hour)
	shore1 := ti.write(t, "Shore/01.mp3", "Slow Tide|The Harbour Lights|Shore||1", time.Hour)
	shore2 := ti.write(t, "Shore/02.mp3", "Low Water|The Harbour Lights|Shore||2", time.Hour)
	beside := ti.write(t, "Shore/bonus.mp3", "Bonus|The Harbour Lights|Extras|Someone|1", time.Hour)
	ti.scan(t)
	album := func(title string) string {
		for _, a := range ti.Albums() {
			if a.Title == title {
				return a.ID
			}
		}
		t.Fatalf("no album %s", title)
		return ""
	}

	for _, c := range []struct {
		what, path, album string
		want              Move
	}{
		{"an album of an album artist", song, "Kites", Move{Album: "Kites", AlbumArtists: []string{"Ada Field"}}},
		{"an album of its folder, from the folder", beside, "Shore", Move{Album: "Shore"}},
		{"an album of its folder, from another", song, "Shore",
			Move{Album: "Shore", AlbumArtists: []string{"The Harbour Lights"}, Others: []string{shore2, shore1}}},
	} {
		got, err := ti.MoveTo(c.path, album(c.album))
		slices.Sort(got.Others)
		slices.Sort(c.want.Others)
		if err != nil || !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %+v, %v; want %+v", c.what, got, err, c.want)
		}
	}
	if _, err := ti.MoveTo(shore1, album("Shore")); err == nil {
		t.Error("moved to its own album")
	}
}

// TestMoveOpenedFile tells how a file the index doesn't hold, as one
// opened, joins an album of it.
func TestMoveOpenedFile(t *testing.T) {
	ti := newIndex(t)
	ti.write(t, "Kites/01.mp3", "Paper Kites|Ada Field|Kites|Ada Field|1", time.Hour)
	ti.scan(t)
	opened := filepath.Join(t.TempDir(), "opened.mp3")
	if err := os.WriteFile(opened, []byte("Loose|Ada Field|||"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ti.MoveTo(opened, ti.Albums()[0].ID)
	if err != nil || got.Album != "Kites" || !slices.Equal(got.AlbumArtists, []string{"Ada Field"}) {
		t.Errorf("%+v, %v", got, err)
	}
}
