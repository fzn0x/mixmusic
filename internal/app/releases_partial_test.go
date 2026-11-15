package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/spotify"
)

// TestReleasesOfAPartialArtistList takes a list of the artists followed on
// Spotify that came in part, its second page failing, as lost: the
// releases known of the artists it misses stay.
func TestReleasesOfAPartialArtistList(t *testing.T) {
	waits := media.PageWaits
	media.PageWaits = []time.Duration{time.Millisecond, time.Millisecond, time.Millisecond}
	t.Cleanup(func() { media.PageWaits = waits })
	var partial atomic.Bool
	sp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path != "/me/following":
			w.WriteHeader(http.StatusNotFound)
		case !partial.Load():
			w.Write([]byte(`{"artists": {"items": [{"id": "dunes", "name": "Dunes"}, {"id": "tide", "name": "Tide"}], "total": 2, "cursors": {}}}`))
		case r.URL.Query().Get("after") == "":
			w.Write([]byte(`{"artists": {"items": [{"id": "tide", "name": "Tide"}], "total": 2,
				"next": "http://` + r.Host + `/me/following?type=artist&after=tide", "cursors": {"after": "tide"}}}`))
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer sp.Close()
	now := releaseDay
	f := &fakeReleases{asks: map[string]int{}, of: map[string][]media.Release{
		"sp:dunes": {release("sp:week", "Low Tide", "2026-11-01", media.AlbumKindSingle)},
		"sp:tide":  {},
	}}
	a := newReleaseApp(t, t.TempDir(), f, &now)
	a.spotify.BaseURL = sp.URL
	a.spotify.SetToken(spotify.Token{AccessToken: "ACCESS", Expiry: time.Now().Add(time.Hour)})
	c := &Catalog{a: a}
	if got, _ := c.NewReleases(context.Background()); !slices.Equal(releaseIDs(got), []string{"sp:week"}) {
		t.Fatalf("new releases %q", releaseIDs(got))
	}

	partial.Store(true)
	followed, lost := a.followedArtists(context.Background())
	if len(followed) != 1 || followed[0].ID != "sp:tide" || !lost[media.SourceSpotify] {
		t.Errorf("a partial list: followed %+v, lost %v", followed, lost)
	}
	if got, _ := c.NewReleases(context.Background()); !slices.Equal(releaseIDs(got), []string{"sp:week"}) {
		t.Errorf("a partial list forgot Dunes' releases: %q", releaseIDs(got))
	}
	if known := c.KnownReleases(); !slices.Equal(releaseIDs(known), []string{"sp:week"}) {
		t.Errorf("it knows %q", releaseIDs(known))
	}
}
