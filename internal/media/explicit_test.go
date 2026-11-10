package media

import (
	"slices"
	"testing"
)

// idsOf are the IDs of tracks, or of albums.
func idsOf[T Track | Album](list []T) []string {
	out := make([]string, len(list))
	for i, v := range list {
		switch v := any(v).(type) {
		case Track:
			out[i] = v.ID
		case Album:
			out[i] = v.ID
		}
	}
	return out
}

// TestWithoutExplicit leaves out the songs and videos marked explicit, but
// not a podcast's episode, and keeps a list without any as it is.
func TestWithoutExplicit(t *testing.T) {
	tracks := []Track{
		{ID: "yt:calm", Title: "Paper Boats"},
		{ID: "yt:loud", Title: "Paper Boats", Explicit: true},
		{ID: "yt:video", Title: "Paper Boats (Live)", Video: true, Explicit: true},
		{ID: "sp:episode", Title: "Harbour Talk 12", Explicit: true, Episode: &Episode{}},
		{ID: "file:song", Title: "Low Tide", Source: SourceFile, Explicit: true},
	}
	got := WithoutExplicit(tracks)
	if want := []string{"yt:calm", "sp:episode"}; !slices.Equal(idsOf(got), want) {
		t.Errorf("kept %v, not %v", idsOf(got), want)
	}
	if idsOf(tracks)[1] != "yt:loud" {
		t.Errorf("the list itself changed: %v", idsOf(tracks))
	}
	clean := tracks[:1]
	if got := WithoutExplicit(clean); &got[0] != &clean[0] {
		t.Error("a list without explicit songs was copied")
	}
	if got := WithoutExplicit(nil); got != nil {
		t.Errorf("no tracks became %v", got)
	}
}

// TestExplicitAlbum marks explicit the albums their service badges, and
// those whose titles say so, but not a title that only holds the word.
func TestExplicitAlbum(t *testing.T) {
	for _, c := range []struct {
		album Album
		want  bool
	}{
		{Album{Title: "Kites"}, false},
		{Album{Title: "Kites", Explicit: true}, true},
		{Album{Title: "Kites (Explicit)"}, true},
		{Album{Title: "Kites [EXPLICIT VERSION]"}, true},
		{Album{Title: "Kites (Clean)"}, false},
		{Album{Title: "Explicit Kites"}, false},
		{Album{Title: "Nothing Explicit About It (Deluxe)"}, false},
	} {
		if got := c.album.ExplicitAlbum(); got != c.want {
			t.Errorf("%q, explicit %v: %v, not %v", c.album.Title, c.album.Explicit, got, c.want)
		}
	}
}

// TestWithoutExplicitItems leaves out of a shelf its explicit songs and
// albums, and keeps its artists and playlists.
func TestWithoutExplicitItems(t *testing.T) {
	items := []Item{
		{Kind: ItemTrack, Track: &Track{ID: "yt:calm"}},
		{Kind: ItemTrack, Track: &Track{ID: "yt:loud", Explicit: true}},
		{Kind: ItemAlbum, Album: &Album{ID: "yt:kites-e", Title: "Kites", Explicit: true}},
		{Kind: ItemAlbum, Album: &Album{ID: "yt:kites-c", Title: "Kites"}},
		{Kind: ItemArtist, Artist: &Artist{ID: "yt:ada", Name: "Ada Field"}},
		{Kind: ItemPlaylist, Playlist: &Playlist{ID: "yt:mix", Title: "Harbour Mix"}},
	}
	var got []string
	for _, it := range WithoutExplicitItems(items) {
		got = append(got, string(it.Kind))
	}
	if want := []string{"track", "album", "artist", "playlist"}; !slices.Equal(got, want) {
		t.Errorf("kept %v, not %v", got, want)
	}
}

// TestSearchAndArtistWithoutExplicit leaves out the explicit songs, videos
// and albums of a search's results and of an artist's page.
func TestSearchAndArtistWithoutExplicit(t *testing.T) {
	loud, calm := Track{ID: "yt:loud", Explicit: true}, Track{ID: "yt:calm"}
	kitesE, kitesC := Album{ID: "yt:kites-e", Title: "Kites", Explicit: true}, Album{ID: "yt:kites-c", Title: "Kites"}
	r := SearchResults{Tracks: []Track{loud, calm}, Videos: []Track{loud}, Albums: []Album{kitesE, kitesC},
		Artists: []Artist{{ID: "yt:ada"}}, Playlists: []Playlist{{ID: "yt:mix"}}}.WithoutExplicit()
	if !slices.Equal(idsOf(r.Tracks), []string{"yt:calm"}) || len(r.Videos) != 0 || !slices.Equal(idsOf(r.Albums), []string{"yt:kites-c"}) ||
		len(r.Artists) != 1 || len(r.Playlists) != 1 {
		t.Errorf("the search kept %+v", r)
	}
	a := Artist{TopTracks: []Track{loud, calm}, Albums: []Album{kitesE, kitesC}, Singles: []Album{{ID: "yt:s", Title: "Gulls (Explicit)"}}}.WithoutExplicit()
	if !slices.Equal(idsOf(a.TopTracks), []string{"yt:calm"}) || !slices.Equal(idsOf(a.Albums), []string{"yt:kites-c"}) || len(a.Singles) != 0 {
		t.Errorf("the artist's page kept %+v", a)
	}
}

// TestExplicitVersions tells the explicit versions of an album: marked so,
// or, beside a version marked clean, not marked.
func TestExplicitVersions(t *testing.T) {
	versions := []Album{{ID: "a", Title: "Kites"}, {ID: "b", Title: "Kites (Clean)"}, {ID: "c", Title: "Kites (Deluxe)", Explicit: true}}
	if got := ExplicitVersions(versions); !slices.Equal(got, []bool{true, false, true}) {
		t.Errorf("explicit %v", got)
	}
}
