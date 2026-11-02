package youtube

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// at sets the day the tests read "8h ago" from: 3 November 2026, at noon.
func at(t *testing.T) {
	t.Helper()
	was := today
	today = func() time.Time { return time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { today = was })
}

func TestParseLength(t *testing.T) {
	for in, want := range map[string]int{
		"2 hr 18 min": 8280000, "34 min": 2040000, "5 min 51 sec": 351000, "45 sec": 45000,
		"1 hr": 3600000, "3:28": 208000, "Sep 1": 0, "58K views": 0, "": 0,
	} {
		if got := parseLength(in); got != want {
			t.Errorf("parseLength(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestParseReleased(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	for in, want := range map[string]string{
		"8h ago": "2026-11-03", "14h ago": "2026-11-02", "4d ago": "2026-10-30", "2 weeks ago": "2026-10-20",
		"3 months ago": "2026-08-03", "Yesterday": "2026-11-02", "Oct 26": "2026-10-26",
		"Dec 29, 2025": "2025-12-29", "Dec 15": "2025-12-15", "58K views": "", "Huberman Lab": "",
	} {
		if got := parseReleased(in, now); got != want {
			t.Errorf("parseReleased(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestSearchPodcastsParse reads the shows of a podcasts search, with their
// makers when YouTube Music links one, and the episodes of an episodes
// search, which name their show rather than the day they came out.
func TestSearchPodcastsParse(t *testing.T) {
	at(t)
	items := parseSearch(fixture(t, "search-podcasts"))
	if len(items) != 2 || items[0].Podcast == nil || items[1].Podcast == nil {
		t.Fatalf("items = %+v", items)
	}
	p := items[0].Podcast
	if p.ID != "yt:MPSPPLPNW_gerXa4Pc8S2qoUQc5e8Ir97RLuVW" || p.Title != "Huberman Lab" || p.PlaylistID != "PLPNW_gerXa4Pc8S2qoUQc5e8Ir97RLuVW" ||
		len(p.Authors) != 1 || p.Authors[0] != (media.Ref{ID: "yt:UC2D2CMWXMOVWx7giW1n3LIg", Name: "Andrew Huberman"}) ||
		len(p.Artwork) == 0 || p.Artwork[len(p.Artwork)-1].Width != 544 {
		t.Errorf("podcast = %+v", p)
	}
	if rss := items[1].Podcast; rss.Title != "HUBERMAN LAB" || len(rss.Authors) != 0 || rss.Authors == nil {
		t.Errorf("a show without a channel = %+v", rss)
	}

	items = parseSearch(fixture(t, "search-episodes"))
	if len(items) != 2 || items[0].Track == nil {
		t.Fatalf("items = %+v", items)
	}
	e := items[0].Track
	if e.ID != "yt:Y566_T-YlNQ" || !e.IsEpisode() || e.MusicVideo() || e.ArtistNames() != "Figuring Out With Raj Shamani" ||
		e.Episode.Show != (media.Ref{ID: "yt:MPSPPLE0Jo6NF_JYO5-phess8GKafKMtPv3tfZ", Name: "Figuring Out With Raj Shamani"}) ||
		e.Episode.Released != "2026-09-01" || len(e.Artwork) == 0 {
		t.Errorf("episode = %+v %+v", e, e.Episode)
	}
}

// TestParsePodcast reads a show's page: its head, its newest episodes,
// with their days, lengths and views, and the token of the next page.
func TestParsePodcast(t *testing.T) {
	at(t)
	p, err := parsePodcast(fixture(t, "podcast"), "MPSPPLPNW_gerXa4Pc8S2qoUQc5e8Ir97RLuVW")
	if err != nil {
		t.Fatal(err)
	}
	if p.Title != "Huberman Lab" || len(p.Authors) != 1 || p.Authors[0].Name != "Andrew Huberman" || p.Saved ||
		p.Description == "" || len(p.Artwork) == 0 || p.URL != "https://music.youtube.com/playlist?list=PLPNW_gerXa4Pc8S2qoUQc5e8Ir97RLuVW" || p.More == "" {
		t.Errorf("podcast = %+v", p)
	}
	if len(p.Episodes) != 3 {
		t.Fatalf("%d episodes", len(p.Episodes))
	}
	want := []struct {
		id, released string
		ms           int
		plays        int64
	}{{"yt:y-fRFiTrrg4", "2026-11-03", 8280000, 58000}, {"", "2026-10-30", 2040000, 104000}, {"", "2026-10-26", 0, 169000}}
	for i, w := range want {
		e := p.Episodes[i]
		if (w.id != "" && e.ID != w.id) || e.Episode == nil || e.Episode.Released != w.released || (w.ms != 0 && e.DurationMs != w.ms) ||
			e.Plays != w.plays || e.Episode.Show != (media.Ref{ID: p.ID, Name: "Huberman Lab"}) || e.ArtistNames() != "Huberman Lab" {
			t.Errorf("episode %d = %+v %+v", i, e, e.Episode)
		}
	}
	if art := p.Episodes[0].Artwork; len(art) != 3 || art[0].Width != 400 || art[2].Width != 853 {
		t.Errorf("the artwork = %+v", art)
	}
	if _, err := parsePodcast(node{map[string]any{}}, "MPSPx"); err != ErrNotFound {
		t.Errorf("an empty page: %v", err)
	}
}

// TestPodcastEpisodes reads the pages after the first through browse's
// continuation, and the show the episodes of one name.
func TestPodcastEpisodes(t *testing.T) {
	at(t)
	more := fixture(t, "podcast-more").v
	c, requests, _ := fakeYouTube(t, func(string) any { return more })
	show := media.Ref{ID: "yt:MPSPPLPNW_gerXa4Pc8S2qoUQc5e8Ir97RLuVW", Name: "Huberman Lab"}
	page, err := c.PodcastEpisodes(context.Background(), show, "T1")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Episodes) != 2 || page.More != "" || page.Episodes[0].Episode.Released != "2025-12-29" ||
		page.Episodes[0].Episode.Show != show {
		t.Errorf("page = %+v", page)
	}
	if r := requests()[0]; r.path != "/youtubei/v1/browse" || r.body["continuation"] != "T1" {
		t.Errorf("request = %s %v", r.path, r.body)
	}
}

// TestPodcast browses a show's page by its ID, and refuses IDs of other
// pages.
func TestPodcast(t *testing.T) {
	at(t)
	page := fixture(t, "podcast").v
	c, requests, _ := fakeYouTube(t, func(string) any { return page })
	p, err := c.Podcast(context.Background(), "MPSPPLPNW_gerXa4Pc8S2qoUQc5e8Ir97RLuVW")
	if err != nil || len(p.Episodes) != 3 {
		t.Fatalf("Podcast = %+v, %v", p, err)
	}
	if r := requests()[0]; r.body["browseId"] != "MPSPPLPNW_gerXa4Pc8S2qoUQc5e8Ir97RLuVW" {
		t.Errorf("request = %v", r.body)
	}
	if _, err := c.Podcast(context.Background(), "UCx"); err != ErrNotFound {
		t.Errorf("a channel's ID: %v", err)
	}
}

// TestSearchPodcasts asks the two filters at once, and keeps what one
// found when the other fails.
func TestSearchPodcasts(t *testing.T) {
	at(t)
	answers := map[string]any{filterParams[FilterPodcasts]: fixture(t, "search-podcasts").v, filterParams[FilterEpisodes]: fixture(t, "search-episodes").v}
	var (
		mu     sync.Mutex
		asked  []string
		broken string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Query, Params string }
		json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		asked = append(asked, body.Query)
		fail := body.Params == broken
		mu.Unlock()
		if fail {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(answers[body.Params])
	}))
	defer srv.Close()
	c := New()
	c.BaseURL = srv.URL
	res, err := c.SearchPodcasts(context.Background(), "huberman lab")
	if err != nil || len(res.Podcasts) != 2 || len(res.Episodes) != 2 {
		t.Fatalf("SearchPodcasts = %+v, %v", res, err)
	}
	if len(asked) != 2 || asked[0] != "huberman lab" {
		t.Errorf("asked %q", asked)
	}
	mu.Lock()
	broken = filterParams[FilterEpisodes]
	mu.Unlock()
	if res, err := c.SearchPodcasts(context.Background(), "huberman lab"); err != nil || len(res.Podcasts) != 2 || len(res.Episodes) != 0 || res.Episodes == nil {
		t.Errorf("with the episodes' search broken: %+v, %v", res, err)
	}
}

// TestLibraryPodcasts keeps the podcasts of the list of those the user
// saved, which YouTube Music shows as cards, past its own playlists.
func TestLibraryPodcasts(t *testing.T) {
	var page any
	json.Unmarshal([]byte(`{"contents":{"singleColumnBrowseResultsRenderer":{"tabs":[{"tabRenderer":{"content":{"sectionListRenderer":{"contents":[
		{"gridRenderer":{"items":[
			{"musicTwoRowItemRenderer":{"title":{"runs":[{"text":"New Episodes"}]},
				"navigationEndpoint":{"browseEndpoint":{"browseId":"VLRDPN","browseEndpointContextSupportedConfigs":{"browseEndpointContextMusicConfig":{"pageType":"MUSIC_PAGE_TYPE_PLAYLIST"}}}}}},
			{"musicTwoRowItemRenderer":{"title":{"runs":[{"text":"Huberman Lab"}]},
				"subtitle":{"runs":[{"text":"Andrew Huberman","navigationEndpoint":{"browseEndpoint":{"browseId":"UC2D2CMWXMOVWx7giW1n3LIg","browseEndpointContextSupportedConfigs":{"browseEndpointContextMusicConfig":{"pageType":"MUSIC_PAGE_TYPE_USER_CHANNEL"}}}}}]},
				"navigationEndpoint":{"browseEndpoint":{"browseId":"MPSPPLPNW_gerXa4Pc8S2qoUQc5e8Ir97RLuVW","browseEndpointContextSupportedConfigs":{"browseEndpointContextMusicConfig":{"pageType":"MUSIC_PAGE_TYPE_PODCAST_SHOW_DETAIL_PAGE"}}}}}}
		]}}]}}}}]}}}`), &page)
	c, requests, _ := fakeYouTube(t, func(string) any { return page })
	if _, err := c.LibraryPodcasts(context.Background()); err != ErrSignedOut {
		t.Errorf("signed out: %v", err)
	}
	c.SetCookies(session)
	got, err := c.LibraryPodcasts(context.Background())
	if err != nil || len(got) != 1 || got[0].Title != "Huberman Lab" || got[0].Authors[0].Name != "Andrew Huberman" {
		t.Fatalf("LibraryPodcasts = %+v, %v", got, err)
	}
	if r := requests()[0]; r.body["browseId"] != libraryPodcasts {
		t.Errorf("request = %v", r.body)
	}
}
