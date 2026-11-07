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
)

// libraryToggleItem is the save toggle of a song's menu, as YouTube Music
// sends it to a signed-in user: its default side is what a click does.
func libraryToggleItem(icon, first, then string) map[string]any {
	feedback := func(token string) map[string]any {
		return map[string]any{"feedbackEndpoint": map[string]any{"feedbackToken": token}}
	}
	return map[string]any{"toggleMenuServiceItemRenderer": map[string]any{
		"defaultText":            map[string]any{"runs": []any{map[string]any{"text": "Save to library"}}},
		"defaultIcon":            map[string]any{"iconType": icon},
		"defaultServiceEndpoint": feedback(first),
		"toggledServiceEndpoint": feedback(then),
	}}
}

func songMenu(items ...any) map[string]any {
	return map[string]any{"menuRenderer": map[string]any{"items": items}}
}

// The other toggles of a song's menu: a subscription, which a visitor is
// asked to sign in for, and a like.
var (
	subscribeToggle = map[string]any{"toggleMenuServiceItemRenderer": map[string]any{
		"defaultIcon":            map[string]any{"iconType": "SUBSCRIBE"},
		"defaultServiceEndpoint": map[string]any{"modalEndpoint": map[string]any{}},
	}}
	likeToggle = map[string]any{"toggleMenuServiceItemRenderer": map[string]any{
		"defaultIcon":            map[string]any{"iconType": "LIKE"},
		"defaultServiceEndpoint": map[string]any{"likeEndpoint": map[string]any{"status": "LIKE"}},
		"toggledServiceEndpoint": map[string]any{"likeEndpoint": map[string]any{"status": "INDIFFERENT"}},
	}}
)

// TestSongSaveToggle reads whether a song is in the library, and the
// tokens to save it and take it out, from its menu: the website's older
// icons and its bookmarks alike. A menu without it, as a visitor's, has
// none.
func TestSongSaveToggle(t *testing.T) {
	for _, c := range []struct {
		icon, first, then string
		want              songToggle
	}{
		{"LIBRARY_ADD", "add", "remove", songToggle{add: "add", remove: "remove"}},
		{"LIBRARY_SAVED", "remove", "add", songToggle{add: "add", remove: "remove", saved: true}},
		{"BOOKMARK_BORDER", "add", "remove", songToggle{add: "add", remove: "remove"}},
		{"BOOKMARK", "remove", "add", songToggle{add: "add", remove: "remove", saved: true}},
	} {
		got, ok := songSaveToggle(node{songMenu(likeToggle, libraryToggleItem(c.icon, c.first, c.then))})
		if !ok || got != c.want {
			t.Errorf("%s: %+v, %v", c.icon, got, ok)
		}
	}
	if got, ok := songSaveToggle(node{songMenu(subscribeToggle, likeToggle)}); ok {
		t.Errorf("a menu without the toggle gave %+v", got)
	}
}

// nextWith is a "next" answer whose queue holds another video, then
// videoID, wrapped as the website wraps a song beside its video, with
// menu.
func nextWith(videoID string, menu map[string]any) map[string]any {
	other := map[string]any{"playlistPanelVideoRenderer": map[string]any{"videoId": "other",
		"menu": songMenu(libraryToggleItem("LIBRARY_SAVED", "other-remove", "other-add"))}}
	wanted := map[string]any{"playlistPanelVideoWrapperRenderer": map[string]any{"primaryRenderer": map[string]any{
		"playlistPanelVideoRenderer": map[string]any{"videoId": videoID, "menu": menu}}}}
	return map[string]any{"contents": map[string]any{"singleColumnMusicWatchNextResultsRenderer": map[string]any{
		"tabbedRenderer": map[string]any{"watchNextTabbedResultsRenderer": map[string]any{"tabs": []any{
			map[string]any{"tabRenderer": map[string]any{"content": map[string]any{"musicQueueRenderer": map[string]any{
				"content": map[string]any{"playlistPanelRenderer": map[string]any{"contents": []any{other, wanted}}},
			}}}},
		}}},
	}}}
}

// songLibraryServer is YouTube Music keeping whether the song "vid" is in
// the library: "next" shows it in the song's menu, and "feedback" takes
// the token of a click, answering processed. It keeps the requests' paths
// and the tokens sent.
type songLibraryServer struct {
	mu        sync.Mutex
	saved     bool
	processed bool
	paths     []string
	tokens    []string
}

func (s *songLibraryServer) client(t *testing.T) *Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		var body map[string]any
		json.Unmarshal(data, &body)
		s.mu.Lock()
		defer s.mu.Unlock()
		s.paths = append(s.paths, r.URL.Path)
		var answer any
		switch r.URL.Path {
		case "/youtubei/v1/next":
			first, then, icon := "add-vid", "remove-vid", "LIBRARY_ADD"
			if s.saved {
				first, then, icon = then, first, "LIBRARY_SAVED"
			}
			answer = nextWith(body["videoId"].(string), songMenu(likeToggle, libraryToggleItem(icon, first, then)))
		case "/youtubei/v1/feedback":
			tokens, _ := body["feedbackTokens"].([]any)
			for _, tok := range tokens {
				s.tokens = append(s.tokens, tok.(string))
				if s.processed {
					s.saved = tok == "add-vid"
				}
			}
			answer = map[string]any{"feedbackResponses": []any{map[string]any{"isProcessed": s.processed}}}
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(answer)
	}))
	t.Cleanup(srv.Close)
	c := New()
	c.BaseURL = srv.URL
	return c
}

func (s *songLibraryServer) sent() (paths, tokens []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.paths), slices.Clone(s.tokens)
}

// TestSetSongSaved saves a song to the library and takes it out, with the
// token its menu holds for each, from the song's own entry in its queue; a
// song already as asked sends nothing. Signed out, nothing is asked, and
// a change YouTube didn't take fails.
func TestSetSongSaved(t *testing.T) {
	s := &songLibraryServer{processed: true}
	c := s.client(t)
	ctx := context.Background()
	if err := c.SetSongSaved(ctx, "vid", true); !errors.Is(err, ErrSignedOut) {
		t.Errorf("signed out, saving gave %v", err)
	}
	if paths, _ := s.sent(); len(paths) != 0 {
		t.Fatalf("signed out, saving asked %v", paths)
	}
	c.SetCookies(session)

	if saved, err := c.SongSaved(ctx, "vid"); err != nil || saved {
		t.Fatalf("before saving: saved %v, %v", saved, err)
	}
	if err := c.SetSongSaved(ctx, "vid", true); err != nil {
		t.Fatal(err)
	}
	if saved, err := c.SongSaved(ctx, "vid"); err != nil || !saved {
		t.Fatalf("once saved: saved %v, %v", saved, err)
	}
	if err := c.SetSongSaved(ctx, "vid", true); err != nil {
		t.Fatal(err)
	}
	if err := c.SetSongSaved(ctx, "vid", false); err != nil {
		t.Fatal(err)
	}
	if _, tokens := s.sent(); !slices.Equal(tokens, []string{"add-vid", "remove-vid"}) {
		t.Errorf("the tokens sent are %v", tokens)
	}

	s.mu.Lock()
	s.processed = false
	s.mu.Unlock()
	if err := c.SetSongSaved(ctx, "vid", true); err == nil {
		t.Error("a change YouTube didn't take went as done")
	}
}

// TestLibrarySongs lists the songs of the library's Songs page, past its
// row that shuffles them.
func TestLibrarySongs(t *testing.T) {
	row := func(id, title string) map[string]any {
		return map[string]any{"musicResponsiveListItemRenderer": map[string]any{
			"flexColumns": []any{map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{
				map[string]any{"text": title, "navigationEndpoint": map[string]any{"watchEndpoint": map[string]any{"videoId": id}}},
			}}}}},
			"playlistItemData": map[string]any{"videoId": id},
		}}
	}
	shuffle := map[string]any{"musicResponsiveListItemRenderer": map[string]any{
		"flexColumns": []any{map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{"text": map[string]any{"runs": []any{
			map[string]any{"text": "Shuffle all"},
		}}}}},
		"navigationEndpoint": map[string]any{"watchPlaylistEndpoint": map[string]any{"playlistId": "RDAT"}},
	}}
	page := libraryPage(map[string]any{"musicShelfRenderer": map[string]any{"contents": []any{shuffle, row("a", "Song a"), row("b", "Song b")}}})
	var (
		mu      sync.Mutex
		browsed []any
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		browsed = append(browsed, body["browseId"])
		mu.Unlock()
		json.NewEncoder(w).Encode(page.v)
	}))
	t.Cleanup(srv.Close)
	c := New()
	c.BaseURL = srv.URL
	if _, err := c.LibrarySongs(context.Background()); !errors.Is(err, ErrSignedOut) {
		t.Errorf("signed out: %v", err)
	}
	c.SetCookies(session)
	songs, err := c.LibrarySongs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(songs) != 2 || songs[0].ID != "yt:a" || songs[1].Title != "Song b" {
		t.Errorf("songs = %+v", songs)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(browsed) != 1 || browsed[0] != "FEmusic_liked_videos" {
		t.Errorf("browsed %v", browsed)
	}
}
