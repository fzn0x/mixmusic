package youtube

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

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

// songsServer answers for an artist's page, the recorded one, and for the
// list of their songs, the recorded playlist in pages of two: the first,
// then those of the tokens P1 and P2, which holds the first page's first
// song again. fail says how many times a page fails before it's answered,
// by its browse ID or token.
type songsServer struct {
	mu    sync.Mutex
	pages map[string]any
	fail  map[string]int
	asked []string
}

func newSongsServer(t *testing.T) (*Client, *songsServer, []media.Track) {
	t.Helper()
	first := fixture(t, "playlist")
	_, shelf := secondaryShelf(first)
	items := shelf.get("contents").v.([]any)
	want := tracksOf(parseItems(shelf.get("contents").list()))[:6]
	shelf.v.(map[string]any)["contents"] = append(slices.Clone(items[:2]), moreItem("P1"))
	page := func(tracks []any) any {
		return map[string]any{"onResponseReceivedActions": []any{map[string]any{"appendContinuationItemsAction": map[string]any{
			"continuationItems": tracks}}}}
	}
	s := &songsServer{fail: map[string]int{}, pages: map[string]any{
		"UCbM50mQtG58vhLZonicYUrQ":                    fixture(t, "artist").v,
		"VLOLAK5uy_mfyTs_nCOquqcF9CD4KhiuQ0q36-a6YYg": first.v,
		"P1": page(append(slices.Clone(items[2:4]), moreItem("P2"))),
		"P2": page([]any{items[4], items[0], items[5]}),
	}}
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	waits := media.PageWaits
	media.PageWaits = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	t.Cleanup(func() { media.PageWaits = waits })
	c := New()
	c.BaseURL = srv.URL
	return c, s, want
}

func (s *songsServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	data, _ := io.ReadAll(r.Body)
	var body struct {
		BrowseID     string `json:"browseId"`
		Continuation string `json:"continuation"`
	}
	json.Unmarshal(data, &body)
	key := body.BrowseID + body.Continuation
	s.mu.Lock()
	s.asked = append(s.asked, key)
	page, ok := s.pages[key]
	failing := s.fail[key] > 0
	if failing {
		s.fail[key]--
	}
	s.mu.Unlock()
	switch {
	case !ok:
		w.WriteHeader(http.StatusNotFound)
	case failing:
		w.WriteHeader(http.StatusServiceUnavailable)
	default:
		json.NewEncoder(w).Encode(page)
	}
}

// TestArtistSongs reads all an artist's songs from the list their page
// links, a page at a time, giving each page's new songs as they come: a
// song the list holds twice comes once.
func TestArtistSongs(t *testing.T) {
	c, s, want := newSongsServer(t)
	s.fail["P1"] = 2
	var given [][]string
	songs, err := c.ArtistSongs(context.Background(), "UCbM50mQtG58vhLZonicYUrQ", func(batch []media.Track) {
		var ids []string
		for _, t := range batch {
			ids = append(ids, t.ID)
		}
		given = append(given, ids)
	})
	if err != nil {
		t.Fatal(err)
	}
	ids := func(tracks []media.Track) []string {
		var out []string
		for _, t := range tracks {
			out = append(out, t.ID)
		}
		return out
	}
	if !slices.Equal(ids(songs), ids(want)) {
		t.Errorf("the songs are %v, want %v", ids(songs), ids(want))
	}
	if wantGiven := [][]string{ids(want[:2]), ids(want[2:4]), ids(want[4:])}; !slices.EqualFunc(given, wantGiven, slices.Equal) {
		t.Errorf("given %v, want %v", given, wantGiven)
	}
}

// TestArtistSongsInPart returns the songs that came, with a ShortList,
// once a page past the first still fails after its tries; a list whose
// first page fails, and an artist without a list, give none.
func TestArtistSongsInPart(t *testing.T) {
	c, s, want := newSongsServer(t)
	s.fail["P2"] = 4
	songs, err := c.ArtistSongs(context.Background(), "UCbM50mQtG58vhLZonicYUrQ", nil)
	short, ok := media.Short(err)
	if !ok || short.Got != 4 || short.Total != 56 || len(songs) != 4 || songs[3].ID != want[3].ID {
		t.Fatalf("a page that failed four times: %d songs, %v", len(songs), err)
	}

	s.fail["VLOLAK5uy_mfyTs_nCOquqcF9CD4KhiuQ0q36-a6YYg"] = 4
	if songs, err := c.ArtistSongs(context.Background(), "UCbM50mQtG58vhLZonicYUrQ", nil); err == nil || len(songs) != 0 {
		t.Errorf("a first page that failed four times: %d songs, %v", len(songs), err)
	} else if _, short := media.Short(err); short {
		t.Errorf("a first page that failed is a short list: %v", err)
	}

	artist := s.pages["UCbM50mQtG58vhLZonicYUrQ"]
	n := node{artist}
	shelf := n.get("contents", "singleColumnBrowseResultsRenderer", "tabs", 0, "tabRenderer", "content", "sectionListRenderer", "contents", 0,
		"musicShelfRenderer").v.(map[string]any)
	delete(shelf, "bottomEndpoint")
	delete(shelf, "title")
	if _, err := c.ArtistSongs(context.Background(), "UCbM50mQtG58vhLZonicYUrQ", nil); !errors.Is(err, ErrNoSongList) {
		t.Errorf("an artist without a list: %v", err)
	}
}
