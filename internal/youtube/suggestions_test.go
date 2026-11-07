package youtube

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestParseSuggestedItems reads the songs, videos, albums, artists and
// playlists YouTube Music suggests below the searches, in its order, as
// it answered for "daft p", "abbey road" and "chill playlist" signed out.
func TestParseSuggestedItems(t *testing.T) {
	kinds := func(items []media.Item) []media.ItemKind {
		var out []media.ItemKind
		for _, it := range items {
			out = append(out, it.Kind)
		}
		return out
	}

	daft := parseSuggestions(fixture(t, "suggest"))
	if got := kinds(daft.Items); len(got) != 5 || got[0] != media.ItemArtist || got[1] != media.ItemTrack {
		t.Fatalf("the items of daft p are %v", got)
	}
	if a := daft.Items[0].Artist; a.ID != "yt:UCRr1xG_2WIDs18a6cIiCxeA" || a.Name != "Daft Punk" || len(a.Artwork) == 0 {
		t.Errorf("the artist is %+v", a)
	}
	song := daft.Items[1].Track
	if song.ID != "yt:4D7u5KF7SP8" || song.Video || song.ArtistNames() != "Daft Punk, Pharrell Williams, Nile Rodgers" ||
		song.Album == nil || song.Album.Name != "Random Access Memories" || len(song.Artwork) == 0 {
		t.Errorf("the song is %+v", song)
	}
	if v := daft.Items[2].Track; v.ID != "yt:8gxQCSh8cPY" || !v.Video {
		t.Errorf("the video is %+v", v)
	}

	abbey := parseSuggestions(fixture(t, "suggest-albums"))
	if got := kinds(abbey.Items); len(got) != 3 || got[0] != media.ItemAlbum || got[1] != media.ItemTrack || got[2] != media.ItemAlbum {
		t.Fatalf("the items of abbey road are %v", got)
	}
	if a := abbey.Items[0].Album; a.ID != "yt:MPREb_tQfaWH32ovE" || a.Title != "Abbey Road (Super Deluxe Edition)" || len(a.Artists) != 1 || a.Artists[0].Name != "The Beatles" {
		t.Errorf("the album is %+v", a)
	}

	chill := parseSuggestions(fixture(t, "suggest-playlists"))
	if got := kinds(chill.Items); len(got) != 4 || got[0] != media.ItemPlaylist || got[3] != media.ItemPlaylist {
		t.Fatalf("the items of chill playlist are %v", got)
	}
	if p := chill.Items[0].Playlist; p.ID != "yt:RDCLAK5uy_kTh6wa62hZCbKX0oevcV-_LD-A8l7CInk" || p.Title != "Chill Bangers" || p.Owner != "YouTube Music" {
		t.Errorf("the playlist is %+v", p)
	}
	if len(chill.Searches) == 0 {
		t.Error("the searches went with the items")
	}
}
