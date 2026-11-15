package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// youtubeCard is a card of a list of the library on YouTube Music that
// opens a page of pageType, browseID's.
func youtubeCard(pageType, browseID, title string) any {
	return map[string]any{"musicTwoRowItemRenderer": map[string]any{
		"title": runs(title),
		"navigationEndpoint": map[string]any{"browseEndpoint": map[string]any{"browseId": browseID,
			"browseEndpointContextSupportedConfigs": map[string]any{"browseEndpointContextMusicConfig": map[string]any{"pageType": pageType}}}},
	}}
}

// gridPage is the first page of a list of the library, holding cards, and
// the token of the page after it, if any.
func gridPage(token string, cards ...any) map[string]any {
	grid := map[string]any{"items": cards}
	if token != "" {
		grid["continuations"] = []any{map[string]any{"nextContinuationData": map[string]any{"continuation": token}}}
	}
	return signedInAnswer(map[string]any{"contents": map[string]any{"singleColumnBrowseResultsRenderer": map[string]any{"tabs": []any{
		map[string]any{"tabRenderer": map[string]any{"content": map[string]any{"sectionListRenderer": map[string]any{"contents": []any{
			map[string]any{"gridRenderer": grid},
		}}}}},
	}}}})
}

// partialYouTube is YouTube Music answering a list of the library with
// whole, the cards of its one page; while partial is set, with first, the
// cards of its first page, then failing the page after it each time it's
// asked. Other endpoints answer 404.
func partialYouTube(t *testing.T, partial *atomic.Bool, whole, first []any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Continuation string `json:"continuation"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		switch {
		case !strings.HasSuffix(r.URL.Path, "/browse"):
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{}`))
		case !partial.Load():
			json.NewEncoder(w).Encode(gridPage("", whole...))
		case body.Continuation == "":
			json.NewEncoder(w).Encode(gridPage("T1", first...))
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"error": {"code": 503, "message": "The service is currently unavailable."}}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// quickPageWaits shortens the waits between the tries of a page.
func quickPageWaits(t *testing.T) {
	waits := media.PageWaits
	media.PageWaits = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	t.Cleanup(func() { media.PageWaits = waits })
}

// TestReleasesOfAPartialYouTubeList takes a list of the artists followed
// on YouTube Music that came in part, its second page failing, as lost:
// the releases known of the artists it misses stay.
func TestReleasesOfAPartialYouTubeList(t *testing.T) {
	quickPageWaits(t)
	var partial atomic.Bool
	harbour := youtubeCard("MUSIC_PAGE_TYPE_ARTIST", "UCharbour", "Harbour")
	tide := youtubeCard("MUSIC_PAGE_TYPE_ARTIST", "UCtide", "Tide")
	yt := partialYouTube(t, &partial, []any{harbour, tide}, []any{tide})
	now := releaseDay
	f := &fakeReleases{asks: map[string]int{}, of: map[string][]media.Release{
		"yt:UCharbour": {release("yt:MPREpier", "Pier", "2026-11-01", media.AlbumKindSingle)},
		"yt:UCtide":    {},
	}}
	a := newReleaseApp(t, t.TempDir(), f, &now)
	a.youtube.BaseURL = yt.URL
	a.youtube.SetCookies(testCookies)
	c := &Catalog{a: a}
	if got, _ := c.NewReleases(context.Background()); !slices.Equal(releaseIDs(got), []string{"yt:MPREpier"}) {
		t.Fatalf("new releases %q", releaseIDs(got))
	}

	partial.Store(true)
	followed, lost := a.followedArtists(context.Background())
	if len(followed) != 1 || followed[0].ID != "yt:UCtide" || !lost[media.SourceYouTube] {
		t.Errorf("a partial list: followed %+v, lost %v", followed, lost)
	}
	if got, _ := c.NewReleases(context.Background()); !slices.Equal(releaseIDs(got), []string{"yt:MPREpier"}) {
		t.Errorf("a partial list forgot Harbour's releases: %q", releaseIDs(got))
	}
	if known := c.KnownReleases(); !slices.Equal(releaseIDs(known), []string{"yt:MPREpier"}) {
		t.Errorf("it knows %q", releaseIDs(known))
	}
}

// TestAssistantPlaylistsOfAPartialList lists for the assistants the
// playlists of YouTube Music's list that came, of one that came in part,
// and tells how many came as a problem.
func TestAssistantPlaylistsOfAPartialList(t *testing.T) {
	quickPageWaits(t)
	var partial atomic.Bool
	partial.Store(true)
	late := youtubeCard("MUSIC_PAGE_TYPE_PLAYLIST", "VLPLlate", "Late night")
	yt := partialYouTube(t, &partial, nil, []any{late})
	now := releaseDay
	a := newReleaseApp(t, t.TempDir(), &fakeReleases{asks: map[string]int{}}, &now)
	a.youtube.BaseURL = yt.URL
	a.youtube.SetCookies(testCookies)
	got := a.userPlaylists(context.Background())
	if len(got.Playlists) != 1 || got.Playlists[0].ID != "yt:PLlate" || got.Playlists[0].Title != "Late night" {
		t.Errorf("the playlists are %+v", got.Playlists)
	}
	if len(got.Problems) != 1 || !strings.Contains(got.Problems[0], "YouTube Music: only 1 came") {
		t.Errorf("the problems are %q", got.Problems)
	}
}
