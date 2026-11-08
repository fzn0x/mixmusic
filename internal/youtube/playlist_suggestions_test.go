package youtube

import (
	"context"
	"errors"
	"testing"
)

// The answers of the suggestions, playlist-suggestions.json for the first
// five and playlist-suggestions-refresh.json for a refresh, are written by
// hand from the documented shape of YouTube Music's answers, not recorded:
// a playlist's suggestions need a signed-in account that owns it.

// ownPlaylist is the recorded playlist, with its header as YouTube Music
// shows it to the playlist's owner.
func ownPlaylist(t *testing.T) node {
	t.Helper()
	n := fixture(t, "playlist")
	section := n.get("contents", "twoColumnBrowseResultsRenderer", "tabs", 0, "tabRenderer", "content", "sectionListRenderer", "contents").v.([]any)
	header := section[0].(map[string]any)["musicResponsiveHeaderRenderer"]
	section[0] = map[string]any{"musicEditablePlaylistDetailHeaderRenderer": map[string]any{"header": map[string]any{"musicResponsiveHeaderRenderer": header}}}
	return n
}

// TestPlaylistSuggestToken reads where the suggestions of a playlist of
// the user's come from: what YouTube Music shows below its songs. Another
// account's playlist has related playlists there, which aren't read.
func TestPlaylistSuggestToken(t *testing.T) {
	theirs, _, err := parsePlaylist(fixture(t, "playlist"), "PLx")
	if err != nil || theirs.Suggest != "" {
		t.Errorf("another account's playlist suggests %q, %v", theirs.Suggest, err)
	}
	p, _, err := parsePlaylist(ownPlaylist(t), "PLx")
	want := "4qmFsgI0EiRWTFBMZlA2aTVUMC1Ea0lNTE5Sd21KcFJCczRQSnZ4Zmd3QmcaDGtnRURDTTBHOEFFQQ%3D%3D"
	if err != nil || !p.Editable || p.Suggest != want {
		t.Errorf("the owner's playlist suggests %q, editable %v, %v", p.Suggest, p.Editable, err)
	}
	// The songs don't change: the suggestions are apart from them.
	if len(p.Tracks) == 0 || len(p.Tracks) != len(theirs.Tracks) {
		t.Errorf("the owner's playlist has %d tracks, another account's %d", len(p.Tracks), len(theirs.Tracks))
	}
}

func TestParsePlaylistSuggestions(t *testing.T) {
	tracks, refresh := parsePlaylistSuggestions(fixture(t, "playlist-suggestions"))
	if len(tracks) != 5 || refresh != "SUGGESTIONS-REFRESH-1" {
		t.Fatalf("the first suggestions: %d tracks, refresh %q", len(tracks), refresh)
	}
	first := tracks[0]
	if first.ID != "yt:sugVideo001" || first.Title != "Harbour Lights" || first.DurationMs != 221000 ||
		len(first.Artists) != 1 || first.Artists[0].Name != "The Low Tides" || first.Artists[0].ID != "yt:UCsugArtist0001" ||
		first.Album == nil || first.Album.ID != "yt:MPREb_sugAlbum01" || first.Album.Name != "Night Ferry" || len(first.Artwork) == 0 {
		t.Errorf("the first suggestion = %+v", first)
	}
	// A suggestion isn't in the playlist: it has no place there.
	if first.EntryID != "" {
		t.Errorf("a suggestion has the place %q", first.EntryID)
	}

	tracks, refresh = parsePlaylistSuggestions(fixture(t, "playlist-suggestions-refresh"))
	if len(tracks) != 5 || tracks[0].ID != "yt:sugVideo006" || refresh != "SUGGESTIONS-REFRESH-2" {
		t.Errorf("a refresh: %d tracks, refresh %q", len(tracks), refresh)
	}

	// Below another account's playlist, related playlists: no songs.
	related := node{map[string]any{"continuationContents": map[string]any{"sectionListContinuation": map[string]any{
		"contents": []any{map[string]any{"musicCarouselShelfRenderer": map[string]any{"contents": []any{}}}},
	}}}}
	if tracks, refresh := parsePlaylistSuggestions(related); len(tracks) != 0 || refresh != "" {
		t.Errorf("related playlists: %d tracks, refresh %q", len(tracks), refresh)
	}
}

// TestPlaylistSuggestions asks for the suggestions by the playlist's token,
// then for five more by the refresh's, signed in.
func TestPlaylistSuggestions(t *testing.T) {
	c, asked := pages(t, map[string]any{
		"FIRST":                 fixture(t, "playlist-suggestions").v,
		"SUGGESTIONS-REFRESH-1": fixture(t, "playlist-suggestions-refresh").v,
	})
	ctx := context.Background()
	if _, _, err := c.PlaylistSuggestions(ctx, "FIRST"); !errors.Is(err, ErrSignedOut) {
		t.Errorf("signed out: %v", err)
	}
	c.SetCookies(session)
	tracks, refresh, err := c.PlaylistSuggestions(ctx, "FIRST")
	if err != nil || len(tracks) != 5 || refresh != "SUGGESTIONS-REFRESH-1" {
		t.Fatalf("the first suggestions: %d tracks, refresh %q, %v", len(tracks), refresh, err)
	}
	tracks, refresh, err = c.PlaylistSuggestions(ctx, refresh)
	if err != nil || len(tracks) != 5 || tracks[0].Title != "Northbound" || refresh != "SUGGESTIONS-REFRESH-2" {
		t.Fatalf("the refresh: %d tracks, refresh %q, %v", len(tracks), refresh, err)
	}
	before := asked.Load()
	if tracks, refresh, err := c.PlaylistSuggestions(ctx, ""); err != nil || len(tracks) != 0 || refresh != "" || asked.Load() != before {
		t.Errorf("no token: %d tracks, refresh %q, %v, asked %v", len(tracks), refresh, err, asked.Load() != before)
	}
}
