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

	"github.com/fzn0x/mixmusic/internal/media"
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

// TestDeletePlaylist deletes a playlist of the user's with playlist/delete,
// as the website does, and nothing that isn't one; a refusal keeps the
// session unless the account's menu says YouTube ended it.
func TestDeletePlaylist(t *testing.T) {
	ctx := context.Background()
	deleted := map[string]any{"command": map[string]any{"handlePlaylistDeletionCommand": map[string]any{"playlistId": "PLtestOwn"}}}
	c, requests, _ := fakeYouTube(t, func(string) any { return deleted })
	if err := c.DeletePlaylist(ctx, "PLtestOwn"); !errors.Is(err, ErrSignedOut) {
		t.Errorf("signed out: %v", err)
	}
	c.SetCookies(session)
	for _, id := range []string{"", "VL", "PL", "LM", "VLLM", "SE", "RDCLAK5uy_test", "OLAK5uy_test", "RDAMPLPLtest"} {
		if err := c.DeletePlaylist(ctx, id); err == nil {
			t.Errorf("deleted %q", id)
		}
	}
	if got := requests(); len(got) != 0 {
		t.Fatalf("asked YouTube %+v", got)
	}
	if err := c.DeletePlaylist(ctx, "VLPLtestOwn"); err != nil {
		t.Fatal(err)
	}
	got := requests()
	if len(got) != 1 || got[0].path != "/youtubei/v1/playlist/delete" || got[0].body["playlistId"] != "PLtestOwn" {
		t.Fatalf("requests = %+v", got)
	}
	if client := got[0].body["context"].(map[string]any)["client"].(map[string]any); client["clientName"] != "WEB_REMIX" {
		t.Errorf("as client %v", client)
	}

	for _, tc := range []struct {
		name            string
		status, account int
		want            error
		asksAccount     bool
	}{
		{"403, the account reads", http.StatusForbidden, http.StatusOK, errDeleteRefused, true},
		{"401, the account reads", http.StatusUnauthorized, http.StatusOK, errDeleteRefused, true},
		{"401, the account doesn't", http.StatusUnauthorized, http.StatusUnauthorized, ErrSessionRejected, true},
		{"404", http.StatusNotFound, http.StatusOK, errDeleteRefused, false},
		{"400", http.StatusBadRequest, http.StatusOK, errDeleteRefused, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, requests, _ := statusYouTube(t, func(path string) (int, any) {
				if path == "/youtubei/v1/account/account_menu" {
					if tc.account != http.StatusOK {
						return tc.account, denied(tc.account)
					}
					return http.StatusOK, accountMenu("1")
				}
				return tc.status, map[string]any{"error": map[string]any{"code": tc.status, "status": "PERMISSION_DENIED"}}
			})
			c.SetCookies(session)
			if err := c.DeletePlaylist(ctx, "PLtestTheirs"); !errors.Is(err, tc.want) {
				t.Errorf("DeletePlaylist = %v, want %v", err, tc.want)
			}
			if got, ended := c.Rejected(), tc.want == ErrSessionRejected; got != ended {
				t.Errorf("rejected %v, want %v", got, ended)
			}
			if got := len(requests()); got != map[bool]int{false: 1, true: 2}[tc.asksAccount] {
				t.Errorf("%d requests", got)
			}
		})
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
	if p, _, err := parsePlaylist(n, "PLx"); err != nil || !p.Editable || p.Title == "" || p.Deletable {
		t.Errorf("the owner's playlist: editable %v, title %q, deletable %v, %v", p.Editable, p.Title, p.Deletable, err)
	}
}

// deleteItem is a menu item that deletes playlist id once a dialog asks,
// as YouTube Music's website reads a deletePlaylistEndpoint. No answer
// signed in has shown where YouTube puts the endpoint: playlistMenu finds
// it at any depth.
func deleteItem(id string) map[string]any {
	return map[string]any{"menuNavigationItemRenderer": map[string]any{
		"text": map[string]any{"runs": []any{map[string]any{"text": "Delete playlist"}}},
		"icon": map[string]any{"iconType": "DELETE"},
		"navigationEndpoint": map[string]any{"confirmDialogEndpoint": map[string]any{"content": map[string]any{"confirmDialogRenderer": map[string]any{
			"confirmButton": map[string]any{"buttonRenderer": map[string]any{
				"text":    map[string]any{"runs": []any{map[string]any{"text": "Delete"}}},
				"command": map[string]any{"deletePlaylistEndpoint": map[string]any{"playlistId": id}},
			}},
		}}}},
	}}
}

// menuItems returns the raw items of a renderer's menu, for a test to
// change, and a function that puts them back changed.
func menuItems(t *testing.T, r node) ([]any, func([]any)) {
	t.Helper()
	m, ok := r.get("menu", "menuRenderer").v.(map[string]any)
	if !ok {
		t.Fatalf("no menu in %v", r.v)
	}
	return m["items"].([]any), func(items []any) { m["items"] = items }
}

// saveToggleOf returns the raw "Save playlist to library" toggle of a
// renderer's menu.
func saveToggleOf(t *testing.T, r node) map[string]any {
	t.Helper()
	items, _ := menuItems(t, r)
	for _, it := range items {
		if toggle, ok := it.(map[string]any)["toggleMenuServiceItemRenderer"].(map[string]any); ok {
			return toggle
		}
	}
	t.Fatalf("no save toggle in %v", items)
	return nil
}

// TestParsePlaylistMenus reads from playlists' menus which the user made,
// to delete, and which they saved, to take out of the library: on the
// cards of a grid, as the library's page lays them, and on the rows of a
// search.
func TestParsePlaylistMenus(t *testing.T) {
	type flags struct{ deletable, saved, editable bool }
	of := func(p *media.Playlist) flags { return flags{p.Deletable, p.Saved, p.Editable} }

	cards := carouselItems(t, fixture(t, "mood-playlist-menus"), "Coffee shop blends")
	if len(cards) != 2 {
		t.Fatalf("%d cards", len(cards))
	}
	saved, owned := node{cards[0]}.get("musicTwoRowItemRenderer"), node{cards[1]}.get("musicTwoRowItemRenderer")
	library := func() []media.Item {
		shelves := parseShelves(libraryPage(map[string]any{"gridRenderer": map[string]any{"items": cards}}))
		if len(shelves) != 1 || len(shelves[0].Items) != 2 {
			t.Fatalf("library = %+v", shelves)
		}
		return shelves[0].Items
	}
	// Signed out, as recorded: neither.
	for _, it := range library() {
		if got := of(it.Playlist); got != (flags{}) {
			t.Errorf("%s read signed out: %+v", it.Playlist.Title, got)
		}
	}
	saveToggleOf(t, saved)["isToggled"] = true
	items, set := menuItems(t, owned)
	set(append(items, deleteItem("PLowned")))
	got := library()
	if f := of(got[0].Playlist); f != (flags{saved: true}) {
		t.Errorf("the saved playlist: %+v", f)
	}
	if f := of(got[1].Playlist); f != (flags{deletable: true, editable: true}) {
		t.Errorf("the playlist of the user: %+v", f)
	}

	// The rows of a search say the same in the same menus.
	n := fixture(t, "search-playlist-menus")
	rows := n.get("contents", "tabbedSearchResultsRenderer", "tabs", 0, "tabRenderer", "content", "sectionListRenderer", "contents", 0, "musicShelfRenderer", "contents").list()
	saveToggleOf(t, rows[0].get("musicResponsiveListItemRenderer"))["isToggled"] = true
	items, set = menuItems(t, rows[1].get("musicResponsiveListItemRenderer"))
	set(append(items, deleteItem("PLtestBBBBBBBBBBBB")))
	found := parseSearch(n)
	if len(found) != 2 || found[0].Playlist == nil || found[1].Playlist == nil {
		t.Fatalf("search = %+v", found)
	}
	if f := of(found[0].Playlist); f != (flags{saved: true}) {
		t.Errorf("the saved row: %+v", f)
	}
	if f := of(found[1].Playlist); f != (flags{deletable: true, editable: true}) {
		t.Errorf("the row of the user's playlist: %+v", f)
	}

	// A toggle that's on but saves no playlist, and a remove without a
	// target, say nothing.
	if _, s := playlistMenu(parse(t, `{"menuRenderer":{"items":[
		{"toggleMenuServiceItemRenderer":{"isToggled":true,"toggledServiceEndpoint":{"likeEndpoint":{"status":"LIKE","target":{"playlistId":"PLx"}}}}},
		{"menuServiceItemRenderer":{"serviceEndpoint":{"likeEndpoint":{"status":"INDIFFERENT","target":{}}}}}]}}`)); s {
		t.Error("read as saved")
	}
	// A plain item that takes the playlist out says it's saved.
	if _, s := playlistMenu(parse(t, `{"menuRenderer":{"items":[
		{"menuServiceItemRenderer":{"serviceEndpoint":{"likeEndpoint":{"status":"INDIFFERENT","target":{"playlistId":"PLx"}}}}}]}}`)); !s {
		t.Error("a plain Remove from library doesn't read as saved")
	}
}

// TestParsePlaylistDeletable reads from a playlist page's header whether
// YouTube Music offers to delete the playlist, as it does its owner.
func TestParsePlaylistDeletable(t *testing.T) {
	n := fixture(t, "playlist-header")
	if p, _, err := parsePlaylist(n, "PLx"); err != nil || p.Deletable || p.Editable {
		t.Fatalf("a playlist of someone else: deletable %v, editable %v, %v", p.Deletable, p.Editable, err)
	}
	section := n.get("contents", "twoColumnBrowseResultsRenderer", "tabs", 0, "tabRenderer", "content", "sectionListRenderer", "contents").v.([]any)
	header := section[0].(map[string]any)["musicResponsiveHeaderRenderer"]
	section[0] = map[string]any{"musicEditablePlaylistDetailHeaderRenderer": map[string]any{"header": map[string]any{"musicResponsiveHeaderRenderer": header}}}
	var menu map[string]any
	for _, b := range header.(map[string]any)["buttons"].([]any) {
		if m, ok := b.(map[string]any)["menuRenderer"].(map[string]any); ok {
			menu = m
		}
	}
	if menu == nil {
		t.Fatal("the header has no menu")
	}
	menu["items"] = append(menu["items"].([]any), deleteItem("PLx"))
	if p, _, err := parsePlaylist(n, "PLx"); err != nil || !p.Deletable || !p.Editable {
		t.Errorf("the owner's playlist: deletable %v, editable %v, %v", p.Deletable, p.Editable, err)
	}
	// A track's menu, in the other column, doesn't make the playlist one
	// to delete.
	n = fixture(t, "playlist")
	shelf := n.get("contents", "twoColumnBrowseResultsRenderer", "secondaryContents", "sectionListRenderer", "contents", 0, "musicPlaylistShelfRenderer").v.(map[string]any)
	shelf["contents"] = append(shelf["contents"].([]any), map[string]any{"musicResponsiveListItemRenderer": map[string]any{
		"menu": map[string]any{"menuRenderer": map[string]any{"items": []any{deleteItem("PLx")}}},
	}})
	if p, _, err := parsePlaylist(n, "PLx"); err != nil || p.Deletable {
		t.Errorf("a track's menu: deletable %v, %v", p.Deletable, err)
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
