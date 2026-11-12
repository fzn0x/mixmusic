package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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

// addToAnswer is YouTube Music's list of the playlists to add to, of ids.
func addToAnswer(ids ...string) map[string]any {
	var options []any
	for _, id := range ids {
		options = append(options, map[string]any{"playlistAddToOptionRenderer": map[string]any{"playlistId": id, "title": runs(id)}})
	}
	return signedInAnswer(map[string]any{"contents": []any{map[string]any{"addToPlaylistRenderer": map[string]any{"playlists": options}}}})
}

// TestYouTubePlaylistsCollaborations marks the playlists of others that
// YouTube Music offers to add to as ones the user collaborates on, which
// take tracks; not those it doesn't offer. It asks only of a library that
// holds a playlist of another's, and where it can't, they stay saved.
func TestYouTubePlaylistsCollaborations(t *testing.T) {
	addTo := addToAnswer("PLmine", "PLcollab")
	addToStatus := http.StatusOK
	a, f := signedInYouTube(t, func(endpoint string) (int, any) {
		switch endpoint {
		case "account/account_menu":
			return http.StatusOK, accountMenuOf("Ada Lovelace")
		case "playlist/get_add_to_playlist":
			return addToStatus, addTo
		}
		return http.StatusOK, libraryPage(
			libraryCard("PLmine", "Late night", false, "Ada Lovelace", " • ", "12 tracks"),
			libraryCard("PLtheirs", "Chill", true, "Lofi Girl", " • ", "200 tracks"),
			libraryCard("PLcollab", "Road songs", true, "Bo Fieldfare", " • ", "3 tracks"),
		)
	})
	col := &Collection{a: a}
	playlists, err := col.YouTubePlaylists(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range playlists {
		got = append(got, fmt.Sprintf("%s editable %v collaborative %v owner %s", p.Title, p.Editable, p.Collaborative, p.Owner))
	}
	want := []string{
		"Late night editable true collaborative false owner Ada Lovelace",
		"Chill editable false collaborative false owner Lofi Girl",
		"Road songs editable true collaborative true owner Bo Fieldfare",
	}
	if !slices.Equal(got, want) {
		t.Errorf("the playlists are %q", got)
	}
	var asked *youtubeAsk
	for _, ask := range f.asked() {
		if ask.endpoint == "playlist/get_add_to_playlist" {
			asked = &ask
		}
	}
	if asked == nil || asked.body["playlistId"] != "LM" {
		t.Errorf("asked for the playlists to add to as %+v", asked)
	}

	// YouTube doesn't answer: none is another's to add to.
	addToStatus, addTo = http.StatusInternalServerError, map[string]any{"error": map[string]any{"code": 500}}
	playlists, err = col.YouTubePlaylists(context.Background())
	if err != nil || len(playlists) != 3 || playlists[2].Editable || playlists[2].Collaborative || !playlists[2].Saved {
		t.Errorf("unanswered: %+v, %v", playlists, err)
	}
}

// TestYouTubePlaylistsNoneOfOthers doesn't ask which playlists take tracks
// of a library of the user's own alone.
func TestYouTubePlaylistsNoneOfOthers(t *testing.T) {
	a, f := signedInYouTube(t, func(endpoint string) (int, any) {
		if endpoint == "account/account_menu" {
			return http.StatusOK, accountMenuOf("Ada Lovelace")
		}
		return http.StatusOK, libraryPage(libraryCard("PLmine", "Late night", false, "Ada Lovelace", " • ", "12 tracks"))
	})
	if _, err := (&Collection{a: a}).YouTubePlaylists(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, ask := range f.asked() {
		if ask.endpoint == "playlist/get_add_to_playlist" {
			t.Errorf("asked for the playlists to add to")
		}
	}
}

// TestCreateYouTubePlaylist makes a playlist on YouTube Music that is the
// user's, to add to and to delete, as the library would list it.
func TestCreateYouTubePlaylist(t *testing.T) {
	a, f := signedInYouTube(t, func(string) (int, any) { return http.StatusOK, map[string]any{"playlistId": "PLnew"} })
	p, err := (&Collection{a: a}).CreateYouTubePlaylist(context.Background(), " Road trip ", nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "yt:PLnew" || p.Title != "Road trip" || !p.Editable || !p.Deletable {
		t.Errorf("made %+v", p)
	}
	if asks := f.asked(); len(asks) != 1 || asks[0].endpoint != "playlist/create" || asks[0].body["title"] != "Road trip" {
		t.Errorf("asked %+v", asks)
	}
}

// TestDeleteYouTubePlaylist deletes a playlist of YouTube Music by its
// MixMusic ID, and nothing of another service or signed out.
func TestDeleteYouTubePlaylist(t *testing.T) {
	ctx := context.Background()
	if err := (&Collection{a: &App{youtube: youtube.New()}}).DeleteYouTubePlaylist(ctx, "yt:PLmine"); !errors.Is(err, errNotSignedIn) {
		t.Errorf("signed out: %v", err)
	}
	a, f := signedInYouTube(t, func(string) (int, any) { return http.StatusOK, map[string]any{} })
	col := &Collection{a: a}
	for _, id := range []string{"mx:1", "sp:37i9", "PLmine"} {
		if err := col.DeleteYouTubePlaylist(ctx, id); err == nil {
			t.Errorf("deleted %q", id)
		}
	}
	if err := col.DeleteYouTubePlaylist(ctx, "yt:PLmine"); err != nil {
		t.Fatal(err)
	}
	asks := f.asked()
	if len(asks) != 1 || asks[0].endpoint != "playlist/delete" || asks[0].body["playlistId"] != "PLmine" {
		t.Errorf("asked %+v", asks)
	}
}

// recordedYouTube is an answer recorded in ../youtube/testdata.
func recordedYouTube(t *testing.T, name string) map[string]any {
	t.Helper()
	data, err := os.ReadFile("../youtube/testdata/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// TestCollaboratorPlaylist takes tracks into a playlist of another's that
// the user collaborates on, whose page YouTube Music shows them without
// its tools, as it offers it to add to; and not one it doesn't offer.
func TestCollaboratorPlaylist(t *testing.T) {
	page, addTo := recordedYouTube(t, "playlist-collaborator"), recordedYouTube(t, "add-to-playlist")
	a, f := signedInYouTube(t, func(endpoint string) (int, any) {
		if endpoint == "playlist/get_add_to_playlist" {
			return http.StatusOK, addTo
		}
		return http.StatusOK, page
	})
	p, err := a.playlist(context.Background(), "yt:PLtestCollab01")
	if err != nil || !p.Editable || !p.Collaborative || p.Mine || p.Deletable {
		t.Fatalf("the collaborator's playlist: %+v, %v", p, err)
	}
	if tr := p.Tracks; len(tr) != 2 || tr[0].Removable || !tr[1].Removable {
		t.Errorf("its tracks: %+v", tr)
	}
	asks := f.asked()
	if len(asks) != 2 || asks[1].endpoint != "playlist/get_add_to_playlist" {
		t.Errorf("asked %+v", asks)
	}

	addTo = addToAnswer("PLother")
	if p, err := a.playlist(context.Background(), "yt:PLtestCollab01"); err != nil || p.Editable {
		t.Errorf("not offered to add to: editable %v, %v", p.Editable, err)
	}
}
