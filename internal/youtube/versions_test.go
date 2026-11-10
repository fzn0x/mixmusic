package youtube

import (
	"slices"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// groupIDs writes groups of albums as their IDs, for comparing.
func groupIDs(groups [][]media.Album) [][]string {
	out := make([][]string, len(groups))
	for i, g := range groups {
		for _, a := range g {
			out[i] = append(out[i], a.ID)
		}
	}
	return out
}

// TestAlbumsExplicit reads the explicit badge of the albums of recorded
// cards: related.json's, two of whose albums YouTube Music badges.
func TestAlbumsExplicit(t *testing.T) {
	explicit := map[string]bool{}
	for _, sh := range sectionShelves(fixture(t, "related").get("contents", "sectionListRenderer", "contents")) {
		for _, it := range sh.Items {
			if it.Album != nil {
				explicit[it.Album.Title] = it.Album.Explicit
			}
		}
	}
	want := map[string]bool{"Deadbeat": true, "The Slow Rush": false, "Currents": true, "Lonerism": false}
	for title, e := range want {
		if got, ok := explicit[title]; !ok || got != e {
			t.Errorf("%s: explicit %v (found %v), want %v", title, got, ok, e)
		}
	}
}

// TestRecordedAlbumsStayApart groups the albums of recorded answers, none
// of which are versions of one: each stays a tile of its own, as an album
// whose mark isn't a version's, "(short version)", and those of a search
// that share a word.
func TestRecordedAlbumsStayApart(t *testing.T) {
	a, err := parseArtist(fixture(t, "artist"), "UCbM50mQtG58vhLZonicYUrQ")
	if err != nil {
		t.Fatal(err)
	}
	lists := map[string][]media.Album{"artist's albums": a.Albums, "artist's singles": a.Singles}
	for _, it := range parseSearch(fixture(t, "search-albums")) {
		if it.Album != nil {
			lists["search"] = append(lists["search"], *it.Album)
		}
	}
	for _, it := range parseSuggestions(fixture(t, "suggest-albums")).Items {
		if it.Album != nil {
			lists["suggestions"] = append(lists["suggestions"], *it.Album)
		}
	}
	for name, albums := range lists {
		if len(albums) == 0 {
			t.Errorf("%s: no albums", name)
		}
		if groups := media.GroupVersions(albums); len(groups) != len(albums) {
			t.Errorf("%s: %d albums made %v", name, len(albums), groupIDs(groups))
		}
	}
	if v := media.VersionOf("Amigo in blood (short version)"); v.Base != "amigo in blood (short version)" || v.Edition != "" {
		t.Errorf("a short version is a version: %+v", v)
	}
	if v := media.VersionOf("Abbey Road (Super Deluxe Edition)"); v.Base != "abbey road" || v.Edition != "Super Deluxe Edition" {
		t.Errorf("Abbey Road (Super Deluxe Edition): %+v", v)
	}
}

// TestArtistAlbumVersions groups the albums of an artist's page by album:
// the explicit and clean ones and the deluxe edition of one, and not the
// two self-titled albums of different years, nor a single named as an
// album. The fixture is hand-written, as its _fixture key says.
func TestArtistAlbumVersions(t *testing.T) {
	a, err := parseArtist(fixture(t, "artist-versions"), "UCnightharbour00000000000")
	if err != nil {
		t.Fatal(err)
	}
	albums := groupIDs(media.GroupVersions(a.Albums))
	want := [][]string{
		{"yt:MPREb_lanternsE", "yt:MPREb_lanternsC", "yt:MPREb_lanternsD"},
		{"yt:MPREb_tidewater"}, {"yt:MPREb_harbour12"}, {"yt:MPREb_harbour15"},
	}
	if !slices.EqualFunc(albums, want, slices.Equal) {
		t.Errorf("albums %v, want %v", albums, want)
	}
	singles := groupIDs(media.GroupVersions(a.Singles))
	if want := [][]string{{"yt:MPREb_breakwE", "yt:MPREb_breakwC"}, {"yt:MPREb_lanternS"}}; !slices.EqualFunc(singles, want, slices.Equal) {
		t.Errorf("singles %v, want %v", singles, want)
	}
	lanterns := media.GroupVersions(a.Albums)[0]
	if labels := media.VersionLabels(lanterns); !slices.Equal(labels, []string{"Explicit", "Clean", "Deluxe Edition"}) {
		t.Errorf("labels %q", labels)
	}
	if i, told := media.PreferredVersion(lanterns, false); i != 0 || !told {
		t.Errorf("opens on %d (told %v), not the explicit one", i, told)
	}
	if i, told := media.PreferredVersion(lanterns, true); i != 1 || !told {
		t.Errorf("preferring clean versions, opens on %d (told %v)", i, told)
	}
}

// TestSearchAlbumVersions groups the albums a search finds, whose rows
// badge the explicit one: another artist's album of the same title and a
// single stay apart. The fixture is hand-written, as its _fixture key
// says.
func TestSearchAlbumVersions(t *testing.T) {
	var albums []media.Album
	for _, it := range parseSearch(fixture(t, "search-album-versions")) {
		if it.Album != nil {
			albums = append(albums, *it.Album)
		}
	}
	if len(albums) != 5 || !albums[0].Explicit || albums[1].Explicit {
		t.Fatalf("albums %+v", albums)
	}
	groups := groupIDs(media.GroupVersions(albums))
	want := [][]string{
		{"yt:MPREb_lanternsE", "yt:MPREb_lanternsC", "yt:MPREb_lanternsD"},
		{"yt:MPREb_lightsLan"}, {"yt:MPREb_lanternS"},
	}
	if !slices.EqualFunc(groups, want, slices.Equal) {
		t.Errorf("groups %v, want %v", groups, want)
	}
}
