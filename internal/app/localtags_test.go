package app

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/localfiles"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/tagwrite"
)

// The songs here are made up; their files are FLAC of the flac package's
// testdata.

// tagged writes the tags of the files of the music folder, by their names
// in it.
func tagged(t *testing.T, music string, files map[string]tagwrite.Tags) {
	t.Helper()
	for name, tg := range files {
		path := filepath.Join(music, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(path); err != nil {
			data, err := os.ReadFile(filepath.Join("..", "..", "flac", "testdata", "stereo-16.flac"))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if err := tagwrite.Write(path, tg, tagwrite.All, tagwrite.Options{}); err != nil {
			t.Fatal(err)
		}
	}
}

// pngOf is a PNG image, as a cover.
func pngOf(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewGray(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// trackNamed is the track of the library of a title.
func trackNamed(t *testing.T, l *LocalFiles, title string) media.Track {
	t.Helper()
	for _, tr := range l.Tracks() {
		if tr.Title == title {
			return tr
		}
	}
	t.Fatalf("no %q among %q", title, titlesOf(l.Tracks()))
	return media.Track{}
}

func titlesOf(tracks []media.Track) []string {
	var out []string
	for _, tr := range tracks {
		out = append(out, tr.Title)
	}
	return out
}

// albumNamed is the album of the library of a title, with its tracks.
func albumNamed(t *testing.T, l *LocalFiles, title string) media.Album {
	t.Helper()
	for _, a := range l.Albums() {
		if a.Title == title {
			whole, _ := l.a.localFiles.Album(a.ID)
			return whole
		}
	}
	t.Fatalf("no album %q", title)
	return media.Album{}
}

// TestEditTags reads a song's tags for the dialog, with its cover, and
// writes new ones, which the library shows at once.
func TestEditTags(t *testing.T) {
	a, music := localApp(t)
	cover := pngOf(t)
	tagged(t, music, map[string]tagwrite.Tags{
		"Shore/01 Slow Tide.flac": {Title: "Slow Tide", Artists: []string{"The Harbour Lights"}, Album: "Shore", Track: 1, Year: 2019,
			Genre: "Folk", Cover: &tagwrite.Picture{Data: cover}},
	})
	scanned(t, a)
	l := &LocalFiles{a}
	song := trackNamed(t, l, "Slow Tide")
	ft, err := l.EditableTags(song.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ft.Tags.Title != "Slow Tide" || ft.Tags.Genre != "Folk" || ft.Tags.Track != 1 || ft.Tags.Cover == nil || !localfiles.IsArt(ft.CoverURL) {
		t.Errorf("the dialog starts from %+v, %q", ft.Tags, ft.CoverURL)
	}
	if _, ok := a.localFiles.ArtFile(ft.CoverURL); !ok {
		t.Errorf("no file of the cover %q", ft.CoverURL)
	}
	n, err := l.EditTags([]string{song.ID}, tagwrite.Tags{Title: "Slow Tide (Live)", Artists: []string{"The Harbour Lights", "Ada Field"}},
		tagwrite.Title|tagwrite.Artists, nil)
	if n != 1 || err != nil {
		t.Fatalf("wrote %d: %v", n, err)
	}
	got := trackNamed(t, l, "Slow Tide (Live)")
	if got.ID != song.ID || got.ArtistNames() != "The Harbour Lights, Ada Field" || got.Album == nil || got.Album.Name != "Shore" {
		t.Errorf("the library shows %+v", got)
	}
}

// TestEditTagsRefuses tells why a WAV file, a read-only one, and one that
// isn't on this computer can't be changed, before any dialog.
func TestEditTagsRefuses(t *testing.T) {
	a, music := localApp(t)
	wav := filepath.Join(music, "memo.wav")
	if err := os.WriteFile(wav, []byte("RIFF\x24\x00\x00\x00WAVEfmt "), 0o644); err != nil {
		t.Fatal(err)
	}
	ro := filepath.Join(music, "Shore", "02 Low Water.flac")
	if err := os.Chmod(ro, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(ro, 0o644) })
	l := &LocalFiles{a}
	for _, c := range []struct{ id, want string }{
		{localfiles.TrackID(wav), "MixMusic can't change memo.wav: MixMusic doesn't change the tags of this kind of file"},
		{localfiles.TrackID(ro), "MixMusic can't change 02 Low Water.flac: the file is read-only"},
		{"yt:abc", "it isn't a song on this computer"},
	} {
		if _, err := l.EditableTags(c.id); err == nil || err.Error() != c.want {
			if os.Getuid() == 0 && strings.Contains(c.want, "read-only") {
				continue // root writes whatever the permissions
			}
			t.Errorf("%s: %v, want %q", c.id, err, c.want)
		}
	}
	if err := l.Editable([]string{localfiles.TrackID(filepath.Join(music, "Shore", "01 Slow Tide.flac")), localfiles.TrackID(ro)}); err == nil &&
		os.Getuid() != 0 {
		t.Error("an album with a read-only song is editable")
	}
}

// TestEditAlbum changes what an album's songs share, and the album shows
// under its new title, with all its songs.
func TestEditAlbum(t *testing.T) {
	a, music := localApp(t)
	tagged(t, music, map[string]tagwrite.Tags{
		"Shore/01 Slow Tide.flac": {Title: "Slow Tide", Artists: []string{"The Harbour Lights"}, Album: "Shore", Track: 1},
		"Shore/02 Low Water.flac": {Title: "Low Water", Artists: []string{"The Harbour Lights"}, Album: "Shore", Track: 2},
	})
	scanned(t, a)
	l := &LocalFiles{a}
	album := albumNamed(t, l, "Shore")
	ft, err := l.AlbumTags(album)
	if err != nil || ft.Tags.Album != "Shore" {
		t.Fatalf("the dialog starts from %+v, %v", ft.Tags, err)
	}
	var ids []string
	var done []int
	for _, tr := range album.Tracks {
		ids = append(ids, tr.ID)
	}
	n, err := l.EditTags(ids, tagwrite.Tags{Album: "Shore (Deluxe)", AlbumArtists: []string{"The Harbour Lights"}, Year: 2020},
		tagwrite.Album|tagwrite.AlbumArtists|tagwrite.Year, func(n int) { done = append(done, n) })
	if n != 2 || err != nil || !slices.Equal(done, []int{1, 2}) {
		t.Fatalf("wrote %d (%v): %v", n, done, err)
	}
	got := albumNamed(t, l, "Shore (Deluxe)")
	if len(got.Tracks) != 2 || got.Year != "2020" || artistsOf(got.Artists) != "The Harbour Lights" {
		t.Errorf("the album is %+v", got)
	}
}

func artistsOf(rs []media.Ref) string {
	var out []string
	for _, r := range rs {
		out = append(out, r.Name)
	}
	return strings.Join(out, ", ")
}

// TestMoveToAlbum files a song under an album of another folder without
// album artists, whose songs then all take the one it shows, and under a
// new album.
func TestMoveToAlbum(t *testing.T) {
	a, music := localApp(t)
	tagged(t, music, map[string]tagwrite.Tags{
		"Shore/01 Slow Tide.flac": {Title: "Slow Tide", Artists: []string{"The Harbour Lights"}, Album: "Shore", Track: 1},
		"Shore/02 Low Water.flac": {Title: "Low Water", Artists: []string{"The Harbour Lights"}, Album: "Shore", Track: 2},
		"Loose/bonus.flac":        {Title: "Gulls at Noon", Artists: []string{"The Harbour Lights"}, Album: "Singles"},
	})
	scanned(t, a)
	l := &LocalFiles{a}
	bonus := trackNamed(t, l, "Gulls at Noon")
	if err := l.MoveToAlbum(bonus.ID, albumNamed(t, l, "Shore").ID); err != nil {
		t.Fatal(err)
	}
	shore := albumNamed(t, l, "Shore")
	if got := titlesOf(shore.Tracks); len(got) != 3 || artistsOf(shore.Artists) != "The Harbour Lights" {
		t.Errorf("Shore holds %q, by %q", got, artistsOf(shore.Artists))
	}
	if err := l.MoveToNewAlbum(bonus.ID, "Odds and Ends"); err != nil {
		t.Fatal(err)
	}
	if got := titlesOf(albumNamed(t, l, "Odds and Ends").Tracks); !slices.Equal(got, []string{"Gulls at Noon"}) {
		t.Errorf("the new album holds %q", got)
	}
	if got := titlesOf(albumNamed(t, l, "Shore").Tracks); len(got) != 2 {
		t.Errorf("Shore holds %q", got)
	}
}
