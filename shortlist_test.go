package main

import (
	"errors"
	"testing"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
)

// partOf is the error of a list that came in part, got of total items.
func partOf(got, total int) error {
	return &media.ShortList{Got: got, Total: total, Err: errors.New("spotify: HTTP 503")}
}

// TestSpotifyLibraryInPart shows the playlists of a list that came in part
// in Library › Spotify, says how many of how many, and asks for the list
// again on Try again; a list that stopped at the most Spotify gives says
// so, with no Try again.
func TestSpotifyLibraryInPart(t *testing.T) {
	s := newTestShell(t)
	s.loads["accounts"] = signedInTo(false, true)
	playlists := &loaded[[]media.Playlist]{done: true, value: []media.Playlist{roadList}, err: partOf(1, 120)}
	s.loads["spotify:playlists"] = playlists
	s.loads["spotify:albums"] = &loaded[[]media.Album]{done: true, value: []media.Album{}}
	s.loads["spotify:artists"] = &loaded[[]media.Artist]{done: true}
	s.loads["spotify:liked"] = &loaded[app.TrackPage]{done: true}
	tt := testerOf(t, s, 1280, 1400)
	show(s, tt, "/library/spotify")
	if !tt.HasText("Showing 1 of 120 playlists") || !tt.HasText(roadList.Title) || tt.HasText(playlists.err.Error()) {
		t.Fatalf("the library shows %q", tt.Texts())
	}
	click(t, s, tt, "try again")
	if playlists.asks != 1 || playlists.again {
		t.Errorf("Try again asked %d times", playlists.asks)
	}

	s.loads["spotify:playlists"] = &loaded[[]media.Playlist]{done: true, value: []media.Playlist{roadList},
		err: &media.ShortList{Got: 1, Total: 120, Cut: true, Err: errors.New("spotify gives apps no more of a list")}}
	tt.Frame()
	if !tt.HasText("Showing 1 of 120 playlists, the most Spotify gives apps") || tt.HasText("try again") {
		t.Errorf("a list cut shows %q", tt.Texts())
	}
}

// TestArtistLibraryInPart shows in Library › Artists the artists followed
// on Spotify of a list that came in part, and says how many of how many.
func TestArtistLibraryInPart(t *testing.T) {
	s := newTestShell(t)
	s.loads["accounts"] = signedInTo(false, true)
	artists := &loaded[[]media.Artist]{done: true, err: partOf(1, 120),
		value: []media.Artist{{ID: "sp:dunes", Source: media.SourceSpotify, Name: "Dunes"}}}
	s.loads["spotify:artists"] = artists
	tt := tester(t, s)
	show(s, tt, "/library/artists")
	if !tt.HasText("Showing 1 of 120 artists") || !tt.HasText("Dunes") || tt.HasText(artists.err.Error()) {
		t.Fatalf("Library › Artists shows %q", tt.Texts())
	}
	click(t, s, tt, "try again")
	if artists.asks != 1 {
		t.Errorf("Try again asked %d times", artists.asks)
	}
}
