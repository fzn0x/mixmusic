package app

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/localfiles"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/tags"
	"github.com/fzn0x/mixmusic/internal/tagwrite"
)

// The songs here are made up; their files are an MP3 of the audio
// package's testdata and FLAC of the flac package's.

// paperKites are the tags of the made-up song of these tests, on an album.
func paperKites(album string) tagwrite.Tags {
	return tagwrite.Tags{Title: "Paper Kites", Artists: []string{"Ada Field"}, Album: album, AlbumArtists: []string{"Ada Field"}}
}

// mp3Copy puts an MP3 of the song in the music folder, at name.
func mp3Copy(t *testing.T, music, name string) string {
	t.Helper()
	path := filepath.Join(music, filepath.FromSlash(name))
	data, err := os.ReadFile(filepath.Join("..", "audio", "testdata", "tone.mp3"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	tagged(t, music, map[string]tagwrite.Tags{name: paperKites("Paper Kites - Single")})
	return path
}

// TestCopiesKeepWhatWasKept is someone's library before and after a copy
// of a song they played, liked and mixed comes: the MP3 alone, then the
// FLAC beside it. The song shows once and plays the FLAC; what was kept of
// the MP3 is the song's, and what's done with the MP3 as an album shows it
// is done with the song. With the FLAC gone, the song plays the MP3, and
// keeps it all.
func TestCopiesKeepWhatWasKept(t *testing.T) {
	a, music := localApp(t)
	for _, name := range []string{"Shore/01 Slow Tide.flac", "Shore/02 Low Water.flac"} {
		os.Remove(filepath.Join(music, filepath.FromSlash(name)))
	}
	mp3 := mp3Copy(t, music, "Singles/Paper Kites.mp3")
	scanned(t, a)
	l, c := &LocalFiles{a}, &Collection{a}
	single := trackNamed(t, l, "Paper Kites")
	if single.ID != localfiles.TrackID(mp3) {
		t.Fatalf("the MP3 alone is %q", single.ID)
	}
	ctx := context.Background()
	c.RecordPlay(single)
	c.RecordPlay(single)
	if err := c.SetLiked(ctx, single, true); err != nil {
		t.Fatal(err)
	}
	mix, err := c.CreatePlaylist("Kites", "", []media.Track{single})
	if err != nil {
		t.Fatal(err)
	}

	tagged(t, music, map[string]tagwrite.Tags{"Ada Field/Kites/01.flac": paperKites("Kites")})
	flac := filepath.Join(music, "Ada Field", "Kites", "01.flac")
	scanned(t, a)
	tracks := l.Tracks()
	if len(tracks) != 1 || !localfiles.IsSong(tracks[0].ID) {
		t.Fatalf("the songs are %+v", tracks)
	}
	song := tracks[0]
	if p, ok := l.FileOf(song.ID); !ok || p != flac {
		t.Errorf("the song plays %q", p)
	}
	src, err := (&Player{a}).Prepare(ctx, song)
	if err != nil || !strings.HasSuffix(src.URL, "/file/"+strings.TrimPrefix(localfiles.TrackID(flac), "file:")) {
		t.Errorf("the song prepares as %+v, %v", src, err)
	}
	if got := c.Liked(); len(got) != 1 || got[0].ID != song.ID {
		t.Errorf("liked %+v", got)
	}
	if n := c.PlayCounts()[song.ID]; n != 2 {
		t.Errorf("the song played %d times", n)
	}
	native, _ := localID(mix.ID)
	if p, _ := c.a.library.Playlist(native); len(p.Tracks) != 1 || p.Tracks[0].ID != song.ID {
		t.Errorf("the mix holds %+v", p.Tracks)
	}

	// The MP3, as its album shows it.
	c.RecordPlay(single)
	if n := c.PlayCounts(); n[song.ID] != 3 || n[single.ID] != 3 {
		t.Errorf("plays of the song %d, of its MP3 %d", n[song.ID], n[single.ID])
	}
	if !a.IsLiked(single.ID) {
		t.Error("the MP3 of a liked song isn't liked")
	}
	if err := c.SetLiked(ctx, single, false); err != nil {
		t.Fatal(err)
	}
	if a.IsLiked(song.ID) || len(c.Liked()) != 0 {
		t.Errorf("unliking the MP3 left %+v liked", c.Liked())
	}
	c.SetLiked(ctx, song, true)
	if held, err := c.AddToMix(mix.ID, []media.Track{single}); len(held) != 1 || err == nil {
		t.Errorf("adding the MP3 to a mix of its song: held %+v, %v", held, err)
	}

	// The FLAC goes: the MP3 plays the song, which keeps its plays,
	// its like and its place in the mix.
	if err := os.RemoveAll(filepath.Join(music, "Ada Field")); err != nil {
		t.Fatal(err)
	}
	scanned(t, a)
	if got := l.Tracks(); len(got) != 1 || got[0].ID != song.ID {
		t.Fatalf("with the FLAC gone, the songs are %+v", got)
	}
	if p, _ := l.FileOf(song.ID); p != mp3 {
		t.Errorf("the song plays %q", p)
	}
	if !a.IsLiked(song.ID) || c.PlayCounts()[song.ID] != 3 || l.Missing(song.ID) {
		t.Errorf("the song: liked %v, %d plays, missing %v", a.IsLiked(song.ID), c.PlayCounts()[song.ID], l.Missing(song.ID))
	}
}

// TestCopiesTakeTags writes the tags Edit info… gives a song with copies to
// each of them, which stay one song.
func TestCopiesTakeTags(t *testing.T) {
	a, music := localApp(t)
	tagged(t, music, map[string]tagwrite.Tags{"Ada Field/Kites/01.flac": paperKites("Kites")})
	mp3 := mp3Copy(t, music, "Singles/Paper Kites.mp3")
	scanned(t, a)
	l := &LocalFiles{a}
	song := trackNamed(t, l, "Paper Kites")
	if !localfiles.IsSong(song.ID) {
		t.Fatalf("the song is %q", song.ID)
	}
	ft, err := l.EditableTags(song.ID)
	if err != nil || ft.Tags.Album != "Kites" {
		t.Fatalf("the song's tags to edit are %+v, %v", ft.Tags, err)
	}
	if n, err := l.EditTags([]string{song.ID}, tagwrite.Tags{Title: "Paper Kites (Remastered)"}, tagwrite.Title, nil); n != 2 || err != nil {
		t.Fatalf("wrote %d files: %v", n, err)
	}
	if got := fileTitle(t, mp3); got != "Paper Kites (Remastered)" {
		t.Errorf("the MP3's title is %q", got)
	}
	tracks := l.Tracks()
	if !slices.ContainsFunc(tracks, func(tr media.Track) bool { return tr.ID == song.ID && tr.Title == "Paper Kites (Remastered)" }) ||
		slices.ContainsFunc(tracks, func(tr media.Track) bool { return tr.ID == localfiles.TrackID(mp3) }) {
		t.Errorf("after the edit, the songs are %+v", tracks)
	}
	// The queue takes in the song's new tags, as it holds the song.
	retagged := a.withSongs([]media.Track{{ID: localfiles.TrackID(mp3), Source: media.SourceFile}})
	if len(retagged) != 2 || retagged[1].ID != song.ID || retagged[1].Title != "Paper Kites (Remastered)" {
		t.Errorf("the queue takes in %+v", retagged)
	}
}

// fileTitle is the title the tags of the file at path hold.
func fileTitle(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	tg, err := tags.Read(f)
	if err != nil {
		t.Fatal(err)
	}
	return tg.Title
}
