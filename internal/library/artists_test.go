package library

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestFollowArtists keeps the artists followed, without the sections of
// their pages, the last followed first, for the next start.
func TestFollowArtists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.json")
	l, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Artists()) != 0 || l.Artists() == nil {
		t.Errorf("a new library follows %v", l.Artists())
	}
	a := media.Artist{ID: "yt:UCa", Source: media.SourceYouTube, Name: "A", Subtitle: "2.1M subscribers",
		Artwork: []media.Image{{URL: "https://example.com/a.jpg", Width: 120, Height: 120}}, URL: "https://music.youtube.com/channel/UCa",
		TopTracks: []media.Track{{ID: "yt:t"}}, Albums: []media.Album{{ID: "yt:MPREa"}}, Following: true}
	b := media.Artist{ID: "sp:b", Source: media.SourceSpotify, Name: "B"}
	for _, x := range []media.Artist{a, b, a} {
		if err := l.SetArtistFollowed(x, true); err != nil {
			t.Fatal(err)
		}
	}
	got := l.Artists()
	if len(got) != 2 || got[0].ID != a.ID || got[1].ID != b.ID || got[0].Name != "A" || len(got[0].Artwork) != 1 ||
		got[0].URL != a.URL || len(got[0].TopTracks) != 0 || got[0].TopTracks == nil || got[1].Artwork == nil || got[0].Following {
		t.Errorf("Artists() = %+v", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, left := range []string{"subscribers", "yt:t", "MPREa", "topTracks"} {
		if strings.Contains(string(data), left) {
			t.Errorf("the library keeps %q of an artist: %s", left, data)
		}
	}
	if err := l.SetArtistFollowed(b, false); err != nil {
		t.Fatal(err)
	}
	again, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if !again.FollowsArtist(a.ID) || again.FollowsArtist(b.ID) || len(again.Artists()) != 1 {
		t.Errorf("after a restart: %+v", again.Artists())
	}
}
