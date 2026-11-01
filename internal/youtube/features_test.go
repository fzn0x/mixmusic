package youtube

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
)

// recorded is a request a fake YouTube received.
type recorded struct {
	path   string
	query  url.Values
	header http.Header
	body   map[string]any
}

// fakeYouTube answers each request with answer(path), and records them.
func fakeYouTube(t *testing.T, answer func(path string) any) (*Client, func() []recorded, *httptest.Server) {
	t.Helper()
	return statusYouTube(t, func(path string) (int, any) { return http.StatusOK, answer(path) })
}

// statusYouTube is fakeYouTube answering with the status answer gives too.
func statusYouTube(t *testing.T, answer func(path string) (int, any)) (*Client, func() []recorded, *httptest.Server) {
	t.Helper()
	var (
		mu  sync.Mutex
		got []recorded
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		var body map[string]any
		json.Unmarshal(data, &body)
		mu.Lock()
		got = append(got, recorded{r.URL.Path, r.URL.Query(), r.Header.Clone(), body})
		mu.Unlock()
		status, v := answer(r.URL.Path)
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(v)
	}))
	t.Cleanup(srv.Close)
	c := New()
	c.BaseURL = srv.URL
	return c, func() []recorded {
		mu.Lock()
		defer mu.Unlock()
		return append([]recorded(nil), got...)
	}, srv
}

func TestCreatePlaylist(t *testing.T) {
	c, requests, _ := fakeYouTube(t, func(string) any { return map[string]any{"playlistId": "PLnew"} })
	if _, err := c.CreatePlaylist(context.Background(), "Mine", nil); !errors.Is(err, ErrSignedOut) {
		t.Errorf("signed out: %v", err)
	}
	c.SetCookies(session)
	if _, err := c.CreatePlaylist(context.Background(), "a <b>", nil); err == nil {
		t.Error("a name with < and > was taken")
	}
	id, err := c.CreatePlaylist(context.Background(), "  Late night  ", []string{"v1", "v1", "", "v2"})
	if err != nil || id != "PLnew" {
		t.Fatalf("CreatePlaylist = %q, %v", id, err)
	}
	r := requests()[0]
	if r.path != "/youtubei/v1/playlist/create" || r.body["title"] != "Late night" || r.body["privacyStatus"] != "PRIVATE" {
		t.Errorf("request = %s %v", r.path, r.body)
	}
	if ids, _ := r.body["videoIds"].([]any); len(ids) != 2 || ids[0] != "v1" || ids[1] != "v2" {
		t.Errorf("videoIds = %v", r.body["videoIds"])
	}
}

func TestCreatePlaylistRefused(t *testing.T) {
	c, _, _ := fakeYouTube(t, func(string) any { return map[string]any{} })
	c.SetCookies(session)
	if _, err := c.CreatePlaylist(context.Background(), "Mine", nil); !errors.Is(err, errRefused) {
		t.Errorf("an answer without an ID gave %v", err)
	}
}

func TestEditPlaylist(t *testing.T) {
	status := "STATUS_SUCCEEDED"
	c, requests, _ := fakeYouTube(t, func(string) any { return map[string]any{"status": status} })
	c.SetCookies(session)
	ctx := context.Background()
	if err := c.AddToPlaylist(ctx, "VLPLx", "v1"); err != nil {
		t.Fatal(err)
	}
	if err := c.RemoveFromPlaylist(ctx, "PLx", "", "v1"); err == nil {
		t.Error("removed a track without its place in the playlist")
	}
	if err := c.RemoveFromPlaylist(ctx, "PLx", "set1", "v1"); err != nil {
		t.Fatal(err)
	}
	status = "STATUS_FAILED"
	if err := c.AddToPlaylist(ctx, "PLx", "v1"); err == nil {
		t.Error("a failed status gave no error")
	}
	got := requests()
	if len(got) != 3 {
		t.Fatalf("%d requests", len(got))
	}
	add, remove := got[0].body, got[1].body
	if got[0].path != "/youtubei/v1/browse/edit_playlist" || add["playlistId"] != "PLx" {
		t.Errorf("add went to %s with %v", got[0].path, add["playlistId"])
	}
	if a := add["actions"].([]any)[0].(map[string]any); a["action"] != "ACTION_ADD_VIDEO" || a["addedVideoId"] != "v1" {
		t.Errorf("add action = %v", a)
	}
	if a := remove["actions"].([]any)[0].(map[string]any); a["action"] != "ACTION_REMOVE_VIDEO" || a["setVideoId"] != "set1" || a["removedVideoId"] != "v1" {
		t.Errorf("remove action = %v", a)
	}
}

// accountMenu is the account's menu, as YouTube answers a session it
// takes, with logged_in 1, or one it ended, with 0.
func accountMenu(loggedInValue string) map[string]any {
	menu := loggedIn(loggedInValue)
	menu["actions"] = []any{map[string]any{"openPopupAction": map[string]any{"popup": map[string]any{"multiPageMenuRenderer": map[string]any{
		"header": map[string]any{"activeAccountHeaderRenderer": map[string]any{"accountName": map[string]any{"runs": []any{map[string]any{"text": "Ada"}}}}},
	}}}}}
	return menu
}

// denied is how YouTube answers what it doesn't let a request do.
func denied(status int) map[string]any {
	return map[string]any{"error": map[string]any{"code": status, "message": "You are not authorized to edit this playlist.", "status": "UNAUTHENTICATED"}}
}

// TestRefusedEdit keeps the session through a refused edit of a playlist,
// which YouTube answers with 401 or 403 as it does a session it ended:
// the account's menu tells which it is.
func TestRefusedEdit(t *testing.T) {
	for _, tc := range []struct {
		name          string
		edit, account int
		menu          string
		ended         bool
	}{
		{"401, the account reads", http.StatusUnauthorized, http.StatusOK, "1", false},
		{"403, the account reads", http.StatusForbidden, http.StatusOK, "1", false},
		{"401, the account doesn't", http.StatusUnauthorized, http.StatusUnauthorized, "1", true},
		{"403, the account reads signed out", http.StatusForbidden, http.StatusOK, "0", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, requests, _ := statusYouTube(t, func(path string) (int, any) {
				if path == "/youtubei/v1/account/account_menu" {
					if tc.account != http.StatusOK {
						return tc.account, denied(tc.account)
					}
					return http.StatusOK, accountMenu(tc.menu)
				}
				return tc.edit, denied(tc.edit)
			})
			c.SetCookies(session)
			err := c.AddToPlaylist(context.Background(), "PLx", "v1")
			if got := c.Rejected(); got != tc.ended {
				t.Errorf("rejected %v, want %v", got, tc.ended)
			}
			want := errNotYours
			if tc.ended {
				want = ErrSessionRejected
			}
			if !errors.Is(err, want) {
				t.Errorf("AddToPlaylist = %v, want %v", err, want)
			}
			if got := requests(); len(got) != 2 || got[1].path != "/youtubei/v1/account/account_menu" {
				t.Errorf("requests = %+v", got)
			}
		})
	}
	// Any other endpoint's 401 ends the session at once, as before.
	c, _, _ := statusYouTube(t, func(string) (int, any) { return http.StatusUnauthorized, denied(http.StatusUnauthorized) })
	c.SetCookies(session)
	if err := c.SetSaved(context.Background(), "PLx", false); err == nil || !c.Rejected() {
		t.Errorf("a 401 to removelike: %v, rejected %v", err, c.Rejected())
	}
}

func TestParsePlaylistEditable(t *testing.T) {
	n := fixture(t, "playlist")
	p, _, err := parsePlaylist(n, "PLx")
	if err != nil || p.Editable {
		t.Fatalf("a playlist of someone else: editable %v, %v", p.Editable, err)
	}
	// The same header, as YouTube Music shows it to the playlist's owner.
	section := n.get("contents", "twoColumnBrowseResultsRenderer", "tabs", 0, "tabRenderer", "content", "sectionListRenderer", "contents").v.([]any)
	header := section[0].(map[string]any)["musicResponsiveHeaderRenderer"]
	section[0] = map[string]any{"musicEditablePlaylistDetailHeaderRenderer": map[string]any{"header": map[string]any{"musicResponsiveHeaderRenderer": header}}}
	if p, _, err := parsePlaylist(n, "PLx"); err != nil || !p.Editable || p.Title == "" {
		t.Errorf("the owner's playlist: editable %v, title %q, %v", p.Editable, p.Title, err)
	}
}

func TestEntryID(t *testing.T) {
	item := map[string]any{"musicResponsiveListItemRenderer": map[string]any{
		"flexColumns": []any{map[string]any{"musicResponsiveListItemFlexColumnRenderer": map[string]any{
			"text": map[string]any{"runs": []any{map[string]any{"text": "Song"}}},
		}}},
		"playlistItemData": map[string]any{"videoId": "v1", "playlistSetVideoId": "set1"},
	}}
	items := parseItems([]node{{item}})
	if len(items) != 1 || items[0].Track == nil || items[0].Track.EntryID != "set1" {
		t.Errorf("items = %+v", items)
	}
}

func TestParseChannels(t *testing.T) {
	account := func(name string, extra map[string]any) map[string]any {
		item := map[string]any{"accountName": map[string]any{"simpleText": name}}
		for k, v := range extra {
			item[k] = v
		}
		return map[string]any{"accountItem": item}
	}
	tokens := func(t ...any) map[string]any {
		return map[string]any{"selectActiveIdentityEndpoint": map[string]any{"supportedTokens": t}}
	}
	n := node{map[string]any{"actions": []any{map[string]any{"getMultiPageMenuAction": map[string]any{"menu": map[string]any{"multiPageMenuRenderer": map[string]any{
		"sections": []any{map[string]any{"accountSectionListRenderer": map[string]any{"contents": []any{map[string]any{"accountItemSectionRenderer": map[string]any{"contents": []any{
			account("Me", map[string]any{
				"isSelected":      true,
				"channelHandle":   map[string]any{"simpleText": "@me"},
				"accountPhoto":    map[string]any{"thumbnails": []any{map[string]any{"url": "https://a/s"}, map[string]any{"url": "https://a/l"}}},
				"serviceEndpoint": tokens(map[string]any{"accountStateToken": map[string]any{"hasChannel": true}}),
			}),
			account("My Band", map[string]any{"serviceEndpoint": tokens(map[string]any{"pageIdToken": map[string]any{"pageId": "1234"}})}),
			account("Gone", map[string]any{"isDisabled": true}),
		}}}}}},
		}}}}}}}}
	got := parseChannels(n)
	if len(got) != 2 {
		t.Fatalf("channels = %+v", got)
	}
	if me := got[0]; me.Name != "Me" || me.Handle != "@me" || me.PageID != "" || !me.Current || me.Photo != "https://a/l" {
		t.Errorf("the account = %+v", me)
	}
	if band := got[1]; band.Name != "My Band" || band.PageID != "1234" || band.Current {
		t.Errorf("the brand channel = %+v", band)
	}
}

func TestChannelRequests(t *testing.T) {
	c, requests, _ := fakeYouTube(t, func(string) any { return map[string]any{} })
	c.SetCookies(session)
	c.Channels(context.Background())
	c.SetChannel("1234")
	c.Home(context.Background())
	got := requests()
	list, home := got[0], got[1]
	if list.path != "/youtubei/v1/account/accounts_list" || list.header.Get("X-YouTube-Client-Name") != "1" {
		t.Errorf("channels asked %s as client %s", list.path, list.header.Get("X-YouTube-Client-Name"))
	}
	if name := list.body["context"].(map[string]any)["client"].(map[string]any)["clientName"]; name != "WEB" {
		t.Errorf("channels asked as %v", name)
	}
	if home.header.Get("X-Goog-PageId") != "1234" {
		t.Errorf("X-Goog-PageId = %q", home.header.Get("X-Goog-PageId"))
	}
	if user := home.body["context"].(map[string]any)["user"].(map[string]any); user["onBehalfOfUser"] != "1234" {
		t.Errorf("user = %v", user)
	}
	c.SetCookies(nil)
	if c.Channel() != "" {
		t.Error("signing out kept the channel")
	}
}

func TestReportPlay(t *testing.T) {
	var srv *httptest.Server
	c, requests, srv := fakeYouTube(t, func(path string) any {
		if path == "/youtubei/v1/player" {
			return map[string]any{"playbackTracking": map[string]any{"videostatsPlaybackUrl": map[string]any{
				"baseUrl": srv.URL + "/api/stats/playback?docid=v1&ei=x",
			}}}
		}
		return map[string]any{}
	})
	saved := reportsTo
	reportsTo = func(*url.URL) bool { return true }
	defer func() { reportsTo = saved }()

	if err := c.ReportPlay(context.Background(), "v1"); err != nil || len(requests()) != 0 {
		t.Fatalf("signed out: %v, %d requests", err, len(requests()))
	}
	c.SetCookies(session)
	if err := c.ReportPlay(context.Background(), "v1"); err != nil {
		t.Fatal(err)
	}
	got := requests()
	if len(got) != 2 || got[0].body["videoId"] != "v1" {
		t.Fatalf("requests = %+v", got)
	}
	ping := got[1]
	if ping.path != "/api/stats/playback" || ping.query.Get("docid") != "v1" || ping.query.Get("ver") != "2" || ping.query.Get("c") != "WEB_REMIX" || len(ping.query.Get("cpn")) != 16 {
		t.Errorf("report = %s?%s", ping.path, ping.query.Encode())
	}
	if ping.header.Get("Cookie") == "" || ping.header.Get("Authorization") == "" {
		t.Error("the report went without the session")
	}

	reportsTo = saved
	if err := c.ReportPlay(context.Background(), "v1"); err == nil {
		t.Error("reported a play to a host that is not YouTube's")
	}
}
