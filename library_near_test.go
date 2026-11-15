package main

import (
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
)

// TestServiceLibraryBuildsNearTiles builds of the shelves of a long
// Library › Spotify the tiles near the view alone, and the others as the
// shelf scrolls to them.
func TestServiceLibraryBuildsNearTiles(t *testing.T) {
	const n = 40
	var playlists []media.Playlist
	var albums []media.Album
	var artists []media.Artist
	for i := range n {
		playlists = append(playlists, media.Playlist{ID: "sp:p" + count(i), Source: media.SourceSpotify, Title: "Playlist " + count(i), Mine: true, Editable: true})
		albums = append(albums, media.Album{ID: "sp:al" + count(i), Source: media.SourceSpotify, Title: "Album " + count(i)})
		artists = append(artists, media.Artist{ID: "sp:a" + count(i), Source: media.SourceSpotify, Name: "Artist " + count(i)})
	}
	s := newTestShell(t)
	s.loads["accounts"] = signedInTo(false, true)
	s.loads["spotify:playlists"] = &loaded[[]media.Playlist]{done: true, value: playlists}
	s.loads["spotify:albums"] = &loaded[[]media.Album]{done: true, value: albums}
	s.loads["spotify:artists"] = &loaded[[]media.Artist]{done: true, value: artists}
	s.loads["spotify:liked"] = &loaded[app.TrackPage]{done: true}
	tt := testerOf(t, s, 1280, 1400)
	show(s, tt, "/library/spotify")
	for _, kind := range []string{"Playlist ", "Album ", "Artist "} {
		if !tt.HasText(kind+"0") || tt.HasText(kind+count(n-1)) {
			t.Fatalf("at the start, the library shows %q", tt.Texts())
		}
	}
	for range 24 {
		click(t, s, tt, "Scroll Playlists on")
		tt.Advance(time.Second)
	}
	if !tt.HasText("Playlist "+count(n-1)) || tt.HasText("Playlist 0") {
		t.Errorf("at the end, the shelf shows %q", tt.Texts())
	}
}

// TestFollowedAndSavedBuildNearTiles builds of the shelves of the artists
// followed and the shows saved on Spotify, in Library › Artists and
// Podcasts, the tiles near the view alone.
func TestFollowedAndSavedBuildNearTiles(t *testing.T) {
	const n = 40
	var artists []media.Artist
	var shows []media.Podcast
	for i := range n {
		artists = append(artists, media.Artist{ID: "sp:a" + count(i), Source: media.SourceSpotify, Name: "Artist " + count(i)})
		shows = append(shows, media.Podcast{ID: "sp:s" + count(i), Source: media.SourceSpotify, Title: "Show " + count(i),
			Authors: []media.Ref{}, Episodes: []media.Track{}})
	}
	s := newTestShell(t)
	s.loads["accounts"] = signedInTo(false, true)
	s.loads["spotify:artists"] = &loaded[[]media.Artist]{done: true, value: artists}
	s.loads["spotify:podcasts"] = &loaded[[]media.Podcast]{done: true, value: shows}
	tt := testerOf(t, s, 1280, 1000)
	for path, kind := range map[string]string{"/library/artists": "Artist ", "/library/podcasts": "Show "} {
		show(s, tt, path)
		if !tt.HasText(kind+"0") || tt.HasText(kind+count(n-1)) {
			t.Errorf("%s shows %q", path, tt.Texts())
		}
	}
}
