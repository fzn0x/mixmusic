package library

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestPlaylistCover shows the cover chosen for a playlist in place of its
// tracks', until it's taken back.
func TestPlaylistCover(t *testing.T) {
	l := open(t)
	p, err := l.CreatePlaylist("Mine", "", []media.Track{tr("yt:a", media.SourceYouTube, "A")})
	if err != nil {
		t.Fatal(err)
	}
	_, id, _ := media.ParseID(p.ID)
	if got := p.Artwork; len(got) != 1 || got[0].URL != "https://img/yt:a" {
		t.Fatalf("a new playlist shows %v", got)
	}
	if old, err := l.SetPlaylistCover(id, "abc.jpg"); err != nil || old != "" {
		t.Fatalf("SetPlaylistCover = %q, %v", old, err)
	}
	got, _ := l.Playlist(id)
	if len(got.Artwork) != 1 || got.Artwork[0].URL != "mixcover:abc.jpg" {
		t.Errorf("with a cover, the playlist shows %v", got.Artwork)
	}
	if ps := l.Playlists(); len(ps) != 1 || ps[0].Artwork[0].URL != "mixcover:abc.jpg" {
		t.Errorf("the list of playlists shows %v", ps)
	}
	if name, err := l.PlaylistCover(id); name != "abc.jpg" || err != nil {
		t.Errorf("PlaylistCover = %q, %v", name, err)
	}
	if old, err := l.SetPlaylistCover(id, ""); err != nil || old != "abc.jpg" {
		t.Fatalf("taking it back = %q, %v", old, err)
	}
	if got, _ := l.Playlist(id); got.Artwork[0].URL != "https://img/yt:a" {
		t.Errorf("taken back, the playlist shows %v", got.Artwork)
	}
	if _, err := l.SetPlaylistCover("nope", "x.jpg"); err != ErrNoPlaylist {
		t.Errorf("a playlist that isn't there: %v", err)
	}
}

func TestCoverName(t *testing.T) {
	for url, want := range map[string]string{
		"mixcover:abc.jpg": "abc.jpg", "mixcover:": "", "mixcover:../x.jpg": "", "mixcover:a/b.jpg": "", `mixcover:a\b.jpg`: "",
		"mixcover:C:x.jpg": "", "localart:abc.jpg": "", "https://img/abc.jpg": "",
	} {
		if got, ok := CoverName(url); got != want || ok != (want != "") {
			t.Errorf("CoverName(%q) = %q, %v", url, got, ok)
		}
	}
}
