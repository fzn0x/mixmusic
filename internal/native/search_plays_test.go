package native

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
)

// TestSearchShowsPlays shows, in a search's songs, how many times each
// played, or how many views a video has, as YouTube Music says, to tell the
// original from the covers; Spotify's songs say none.
func TestSearchShowsPlays(t *testing.T) {
	s := newTestShell(t)
	tracks := []media.Track{
		{ID: "yt-1", Source: media.SourceYouTube, Title: "Instant Crush", Artists: []media.Ref{{Name: "Daft Punk"}}, Plays: 1_200_000_000},
		{ID: "yt-2", Source: media.SourceYouTube, Title: "Instant Crush (video)", Artists: []media.Ref{{Name: "Daft Punk"}}, Plays: 905_000_000, Video: true},
		{ID: "yt-3", Source: media.SourceYouTube, Title: "Instant Crush (cover)", Artists: []media.Ref{{Name: "Ruben Dunn"}}, Plays: 2000},
	}
	spotify := media.Track{ID: "sp-1", Source: media.SourceSpotify, Title: "Instant Crush (Spotify)", Artists: []media.Ref{{Name: "Daft Punk"}}}
	s.loads["search:crush"] = &loaded[app.SearchResponse]{done: true, value: app.SearchResponse{
		YouTube: media.SearchResults{Tracks: tracks}, Spotify: media.SearchResults{Tracks: []media.Track{spotify}}, SpotifySignedIn: true}}
	tt := testerOf(t, s, 1280, 1800)
	show(s, tt, searchPath("crush"))
	for _, want := range []string{"1.2B plays", "905M views", "2K plays"} {
		if !tt.HasText(want) {
			t.Errorf("the search doesn't say %q: %q", want, tt.Texts())
		}
	}
	if !tt.HasText("Instant Crush (Spotify)") {
		t.Fatalf("the search doesn't show Spotify's song: %q", tt.Texts())
	}
	if tt.HasText("0 plays") {
		t.Error("a song without a count says 0 plays")
	}
}
