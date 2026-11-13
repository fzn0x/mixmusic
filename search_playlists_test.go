package main

import (
	"slices"
	"testing"

	"github.com/fzn0x/mixmusic/internal/app"
)

// TestSearchTellsOfSpotifyPlaylists tells, among Spotify's playlists of a
// search, why those of others aren't there (#530).
func TestSearchTellsOfSpotifyPlaylists(t *testing.T) {
	const note = "Spotify shows apps the tracks of the playlists you own or collaborate on only, so only those are listed."
	s := newTestShell(t)
	s.loads["search:kites"] = &loaded[app.SearchResponse]{done: true, value: app.SearchResponse{SpotifySignedIn: true}}
	s.searchPivot = slices.Index(searchPivots, "playlists")
	tt := tester(t, s)
	show(s, tt, searchPath("kites"))
	if tt.HasText(note) {
		t.Errorf("both services' playlists tell of Spotify's: %q", tt.Texts())
	}
	s.searchScope = slices.Index(searchScopes, "Spotify")
	tt.Frame()
	if !tt.HasText(note) {
		t.Errorf("Spotify's playlists don't tell why: %q", tt.Texts())
	}
}
