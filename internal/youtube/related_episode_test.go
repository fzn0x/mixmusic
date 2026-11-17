package youtube

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// episodeNext is a made-up "next" answer of a podcast episode: a Details
// tab and a Related tab of the page type NON_MUSIC_AUDIO_RELATED, whose
// browse ID is relatedID.
func episodeNext(relatedID string) string {
	return `{"contents":{"singleColumnMusicWatchNextResultsRenderer":{"tabbedRenderer":{"watchNextTabbedResultsRenderer":{"tabs":[
	{"tabRenderer":{"title":"Up next"}},
	{"tabRenderer":{"title":"Details","endpoint":{"browseEndpoint":{"browseId":"MPPEEp1Video000","browseEndpointContextSupportedConfigs":{"browseEndpointContextMusicConfig":{"pageType":"MUSIC_PAGE_TYPE_NON_MUSIC_AUDIO_DETAILS"}}}}}},
	{"tabRenderer":{"title":"Related","endpoint":{"browseEndpoint":{"browseId":"` + relatedID + `","browseEndpointContextSupportedConfigs":{"browseEndpointContextMusicConfig":{"pageType":"MUSIC_PAGE_TYPE_NON_MUSIC_AUDIO_RELATED"}}}}}}
	]}}}}}`
}

// episodeRelatedPage is a made-up Related page of an episode: a carousel of
// other episodes.
const episodeRelatedPage = `{"contents":{"sectionListRenderer":{"contents":[{"musicCarouselShelfRenderer":{
	"header":{"musicCarouselShelfBasicHeaderRenderer":{"title":{"runs":[{"text":"You might also like"}]}}},
	"contents":[{"musicMultiRowListItemRenderer":{"title":{"runs":[{"text":"Another Episode"}]},"subtitle":{"runs":[{"text":"3d ago"},{"text":" • "},{"text":"1 hr 9 min"}]},
		"onTap":{"watchEndpoint":{"videoId":"Ep2Video000","watchEndpointMusicSupportedConfigs":{"watchEndpointMusicConfig":{"musicVideoType":"MUSIC_VIDEO_TYPE_PODCAST_EPISODE"}}}}}}]
}}]}}}`

// TestRelatedOfAnEpisode: an episode's Related tab is found by its page
// type whatever its browse ID, and its shelves show as a song's do (#575).
func TestRelatedOfAnEpisode(t *testing.T) {
	for _, id := range []string{"MPPREp1Video000", "MPTRt_Ep1Video000"} {
		var mu sync.Mutex
		browsed := []string{}
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body struct{ BrowseID string }
			json.NewDecoder(r.Body).Decode(&body)
			switch {
			case strings.HasSuffix(r.URL.Path, "/next"):
				w.Write([]byte(episodeNext(id)))
			case strings.HasSuffix(r.URL.Path, "/browse"):
				mu.Lock()
				browsed = append(browsed, body.BrowseID)
				mu.Unlock()
				w.Write([]byte(episodeRelatedPage))
			default:
				http.NotFound(w, r)
			}
		}))
		c := New()
		c.BaseURL = srv.URL
		shelves, err := c.Related(context.Background(), "Ep1Video000")
		srv.Close()
		mu.Lock()
		got := append([]string(nil), browsed...)
		mu.Unlock()
		if err != nil || len(shelves) != 1 {
			t.Fatalf("tab %q: %d shelves, err %v", id, len(shelves), err)
		}
		if len(got) != 1 || got[0] != id {
			t.Errorf("tab %q: browsed %v", id, got)
		}
		items := shelves[0].Items
		if len(items) != 1 || items[0].Track.ID != "yt:Ep2Video000" || items[0].Track.Episode == nil {
			t.Errorf("tab %q: items %+v", id, items)
		}
	}
}

func TestRelatedBrowseIDOfAnEpisode(t *testing.T) {
	if id := relatedBrowseID(parse(t, episodeNext("MPPREp1Video000"))); id != "MPPREp1Video000" {
		t.Errorf("episode: %q", id)
	}
}
