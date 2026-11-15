package youtube

import "testing"

// TestArtistSongsList keeps the playlist of all an artist's songs that
// the title of their Top songs and its Show all link open, and nothing
// that opens another kind of page.
func TestArtistSongsList(t *testing.T) {
	a, err := parseArtist(fixture(t, "artist"), "UCbM50mQtG58vhLZonicYUrQ")
	if err != nil {
		t.Fatal(err)
	}
	if want := "yt:OLAK5uy_mfyTs_nCOquqcF9CD4KhiuQ0q36-a6YYg"; a.SongsID != want {
		t.Errorf("the songs' list is %q, want %q", a.SongsID, want)
	}

	end := func(id, page string) map[string]any {
		return map[string]any{"browseEndpoint": map[string]any{"browseId": id, "params": "ggMCCAI%3D",
			"browseEndpointContextSupportedConfigs": map[string]any{"browseEndpointContextMusicConfig": map[string]any{"pageType": page}}}}
	}
	for _, c := range []struct {
		name  string
		shelf map[string]any
		want  string
	}{
		{"show all alone", map[string]any{"title": map[string]any{"runs": []any{map[string]any{"text": "Top songs"}}},
			"bottomEndpoint": end("VLOLAK5uy_all", pageTypePlaylist)}, "yt:OLAK5uy_all"},
		{"title alone", map[string]any{"title": map[string]any{"runs": []any{map[string]any{"text": "Top songs",
			"navigationEndpoint": end("VLOLAK5uy_title", pageTypePlaylist)}}}}, "yt:OLAK5uy_title"},
		{"an album", map[string]any{"bottomEndpoint": end("MPREb_1", pageTypeAlbum)}, ""},
		{"a playlist page without VL", map[string]any{"bottomEndpoint": end("OLAK5uy_bare", pageTypePlaylist)}, ""},
		{"no link", map[string]any{"title": map[string]any{"runs": []any{map[string]any{"text": "Songs"}}}}, ""},
	} {
		if got := songsList(node{c.shelf}); got != c.want {
			t.Errorf("%s: the songs' list is %q, want %q", c.name, got, c.want)
		}
	}
}
