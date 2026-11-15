package youtube

import "testing"

// TestSearchPlaylistViews reads how many views each playlist of a search
// has, which its row says after the owner's name (#630).
func TestSearchPlaylistViews(t *testing.T) {
	var counted, withViews int
	for _, it := range parseSearch(fixture(t, "search-playlists")) {
		if it.Playlist == nil {
			continue
		}
		counted++
		if it.Playlist.Views > 0 {
			withViews++
		}
		if it.Playlist.Owner == "" || looksLikeCount(it.Playlist.Owner) {
			t.Errorf("%q has owner %q", it.Playlist.Title, it.Playlist.Owner)
		}
	}
	if counted == 0 || withViews == 0 {
		t.Errorf("%d playlists, %d with views; want the views read", counted, withViews)
	}
	var found bool
	for _, it := range parseSearch(fixture(t, "search-playlists")) {
		if it.Playlist != nil && it.Playlist.Views == 5_900_000 {
			found = true
		}
	}
	if !found {
		t.Error("no playlist has the 5.9M views the fixture shows")
	}
}
