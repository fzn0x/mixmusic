package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"

	"github.com/fzn0x/mixmusic/internal/youtube"
)

// fakeYouTube is a YouTube Music that answers each request with what
// answer gives for its endpoint, as "browse", and records the endpoints
// and bodies it was asked.
type fakeYouTube struct {
	mu   sync.Mutex
	asks []youtubeAsk
}

type youtubeAsk struct {
	endpoint string
	body     map[string]any
}

func (f *fakeYouTube) asked() []youtubeAsk {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.asks)
}

// signedInYouTube is an App whose YouTube client is signed in to a fake
// YouTube Music, which answers with answer.
func signedInYouTube(t *testing.T, answer func(endpoint string) (int, any)) (*App, *fakeYouTube) {
	t.Helper()
	f := &fakeYouTube{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		endpoint := r.URL.Path[len("/youtubei/v1/"):]
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.asks = append(f.asks, youtubeAsk{endpoint, body})
		f.mu.Unlock()
		status, v := answer(endpoint)
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(v)
	}))
	t.Cleanup(srv.Close)
	c := youtube.New()
	c.BaseURL = srv.URL
	c.SetCookies(testCookies)
	return &App{youtube: c}, f
}

// runs is a text of runs.
func runs(texts ...string) map[string]any {
	var rs []any
	for _, s := range texts {
		rs = append(rs, map[string]any{"text": s})
	}
	return map[string]any{"runs": rs}
}

// signedInAnswer is an answer YouTube gives a session it takes.
func signedInAnswer(v map[string]any) map[string]any {
	v["responseContext"] = map[string]any{"serviceTrackingParams": []any{map[string]any{
		"service": "GFEEDBACK", "params": []any{map[string]any{"key": "logged_in", "value": "1"}},
	}}}
	return v
}

// accountMenuOf is the account's menu of the account named name.
func accountMenuOf(name string) map[string]any {
	return signedInAnswer(map[string]any{"actions": []any{map[string]any{"openPopupAction": map[string]any{"popup": map[string]any{
		"multiPageMenuRenderer": map[string]any{"header": map[string]any{"activeAccountHeaderRenderer": map[string]any{"accountName": runs(name)}}},
	}}}}})
}

// libraryCard is a playlist's card in the library: its title, and its
// subtitle's runs, as "Ada", " • ", "12 tracks".
func libraryCard(id, title string, saved bool, subtitle ...string) any {
	return map[string]any{"musicTwoRowItemRenderer": map[string]any{
		"title":    runs(title),
		"subtitle": runs(subtitle...),
		"navigationEndpoint": map[string]any{"browseEndpoint": map[string]any{"browseId": "VL" + id,
			"browseEndpointContextSupportedConfigs": map[string]any{"browseEndpointContextMusicConfig": map[string]any{"pageType": "MUSIC_PAGE_TYPE_PLAYLIST"}}}},
		"menu": map[string]any{"menuRenderer": map[string]any{"items": []any{map[string]any{"toggleMenuServiceItemRenderer": map[string]any{
			"defaultText": runs("Save playlist to library"), "toggledText": runs("Remove playlist from library"),
			"isToggled":              saved,
			"toggledServiceEndpoint": map[string]any{"likeEndpoint": map[string]any{"status": "INDIFFERENT", "target": map[string]any{"playlistId": id}}},
		}}}}},
	}}
}

// libraryPage is the library's page of playlists, holding cards.
func libraryPage(cards ...any) map[string]any {
	return signedInAnswer(map[string]any{"contents": map[string]any{"singleColumnBrowseResultsRenderer": map[string]any{"tabs": []any{
		map[string]any{"tabRenderer": map[string]any{"content": map[string]any{"sectionListRenderer": map[string]any{"contents": []any{
			map[string]any{"gridRenderer": map[string]any{"items": cards}},
		}}}}},
	}}}})
}

// TestYouTubePlaylistsYours marks the library's playlists of the account's
// name, or of none, as the user's, for tracks to go into them, and not
// those of another, those the user saved, nor the automatic ones.
func TestYouTubePlaylistsYours(t *testing.T) {
	a, _ := signedInYouTube(t, func(endpoint string) (int, any) {
		if endpoint == "account/account_menu" {
			return http.StatusOK, accountMenuOf("Ada Lovelace")
		}
		return http.StatusOK, libraryPage(
			libraryCard("PLmine", "Late night", false, "Ada Lovelace", " • ", "12 tracks"),
			libraryCard("PLcount", "Untitled", false, "5 tracks"),
			libraryCard("PLtheirs", "Chill", true, "Lofi Girl", " • ", "200 tracks"),
			libraryCard("PLnamesake", "Same name", true, "ada lovelace", " • ", "8 tracks"),
			libraryCard("SE", "Episodes for later", false, "2 episodes"),
		)
	})
	playlists, err := (&Collection{a: a}).YouTubePlaylists(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var mine []string
	for _, p := range playlists {
		if p.Editable {
			mine = append(mine, p.Title)
		}
	}
	if !slices.Equal(mine, []string{"Late night", "Untitled"}) {
		t.Errorf("the user's playlists are %q, of %+v", mine, playlists)
	}
}
