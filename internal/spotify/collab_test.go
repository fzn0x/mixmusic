package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// change is a request to change a playlist, as a fake API got it.
type change struct {
	method, path string
	body         map[string]any
}

// changes is a fake API that answers every change with status and body,
// and records them.
func changes(t *testing.T, status *int, body *string) (*Client, func() []change) {
	t.Helper()
	var (
		mu  sync.Mutex
		got []change
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		var b map[string]any
		json.Unmarshal(data, &b)
		mu.Lock()
		got = append(got, change{r.Method, r.URL.Path, b})
		mu.Unlock()
		w.WriteHeader(*status)
		w.Write([]byte(*body))
	}))
	t.Cleanup(srv.Close)
	return testClient(t, srv), func() []change {
		mu.Lock()
		defer mu.Unlock()
		return append([]change(nil), got...)
	}
}

// TestSetCollaborative makes a playlist collaborative, which Spotify takes
// only of a playlist off its owner's profile, and its owner's alone again.
func TestSetCollaborative(t *testing.T) {
	status, body := http.StatusOK, ""
	c, got := changes(t, &status, &body)
	ctx := context.Background()
	if err := c.SetCollaborative(ctx, "p1", true); err != nil {
		t.Fatal(err)
	}
	if err := c.SetCollaborative(ctx, "p1", false); err != nil {
		t.Fatal(err)
	}
	asked := got()
	if len(asked) != 2 || asked[0].method != http.MethodPut || asked[0].path != "/playlists/p1" {
		t.Fatalf("asked %+v", asked)
	}
	if b := asked[0].body; b["collaborative"] != true || b["public"] != false || len(b) != 2 {
		t.Errorf("making it collaborative sent %v", b)
	}
	if b := asked[1].body; b["collaborative"] != false || len(b) != 1 {
		t.Errorf("making it the owner's alone sent %v", b)
	}
}

// TestMoveInPlaylist moves an item of a playlist up and down, as Spotify
// takes a move: by the place of the item it goes before.
func TestMoveInPlaylist(t *testing.T) {
	status, body := http.StatusOK, `{"snapshot_id": "s2"}`
	c, got := changes(t, &status, &body)
	ctx := context.Background()
	for _, m := range [][2]int{{0, 2}, {3, 1}, {2, 2}} {
		if err := c.MoveInPlaylist(ctx, "p1", m[0], m[1]); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.MoveInPlaylist(ctx, "p1", -1, 2); err == nil {
		t.Error("moved from no place")
	}
	asked := got()
	if len(asked) != 2 {
		t.Fatalf("asked %+v", asked)
	}
	for i, want := range [][2]float64{{0, 3}, {3, 1}} {
		b := asked[i].body
		if asked[i].method != http.MethodPut || asked[i].path != "/playlists/p1/items" ||
			b["range_start"] != want[0] || b["insert_before"] != want[1] || b["range_length"] != 1.0 {
			t.Errorf("move %d sent %s %s %v", i, asked[i].method, asked[i].path, b)
		}
	}
}

// TestPlaylistRefused tells a change of a playlist Spotify refused with
// 403 from the refusals of an app in development mode, which keep their
// own words, and gives Spotify's own when it has some.
func TestPlaylistRefused(t *testing.T) {
	status, body := http.StatusForbidden, ""
	c, _ := changes(t, &status, &body)
	ctx := context.Background()
	for _, tc := range []struct {
		name, body string
		want       error
		says       string
	}{
		{"no words", "", ErrPlaylistRefused, "only its owner can make it collaborative"},
		{"plain text", "Forbidden", ErrPlaylistRefused, "(Spotify says: Forbidden)"},
		{"an error object", `{"error": {"status": 403, "message": "You cannot change a playlist you don't own"}}`, ErrPlaylistRefused, "you don't own"},
		{"not one of the app's users", "Check settings on https://developer.spotify.com/dashboard, the user may not be registered.", ErrUnregistered, "Users Management"},
		{"an owner without Premium", "Active premium subscription required for the owner of the app.", ErrOwnerWithoutPremium, "Premium"},
	} {
		body = tc.body
		for what, err := range map[string]error{
			"SetCollaborative": c.SetCollaborative(ctx, "p1", true),
			"MoveInPlaylist":   c.MoveInPlaylist(ctx, "p1", 0, 1),
			"AddToPlaylist":    c.AddToPlaylist(ctx, "p1", "spotify:track:t1"),
		} {
			var api *APIError
			if !errors.Is(err, tc.want) || !errors.As(err, &api) || api.Status != http.StatusForbidden || !strings.Contains(err.Error(), tc.says) {
				t.Errorf("%s, %s: %v", tc.name, what, err)
			}
			if tc.want != ErrPlaylistRefused && errors.Is(err, ErrPlaylistRefused) {
				t.Errorf("%s, %s: a refusal of development mode reads as the playlist's: %v", tc.name, what, err)
			}
		}
	}
	// Other errors stay as they are.
	status, body = http.StatusNotFound, `{"error": {"status": 404, "message": "Not found"}}`
	if err := c.SetCollaborative(ctx, "p1", true); errors.Is(err, ErrPlaylistRefused) || !strings.Contains(err.Error(), "Not found") {
		t.Errorf("a 404: %v", err)
	}
}

// TestPlaylistOwnership tells the playlists the user made from those they
// follow, which may be collaborative, and who may open them.
func TestPlaylistOwnership(t *testing.T) {
	c, _ := newFake(t, map[string]string{
		"GET /me/playlists": `{"items": [
			{"id": "p1", "name": "Mine", "owner": {"id": "me", "display_name": "Me"}, "public": true, "collaborative": false, "items": {"total": 2}},
			{"id": "p2", "name": "Ours", "owner": {"id": "me", "display_name": "Me"}, "public": false, "collaborative": true, "items": {"total": 3}},
			{"id": "p3", "name": "Band practice", "owner": {"id": "ana", "display_name": "Ana Testwood"}, "public": false, "collaborative": true, "items": {"total": 4}},
			{"id": "p4", "name": "Followed", "owner": {"id": "bo", "display_name": "Bo Fieldfare"}, "public": true, "collaborative": false, "items": {"total": 5}},
			{"id": "p5", "name": "Unsaid", "owner": {"id": "bo", "display_name": "Bo Fieldfare"}}
		]}`,
	})
	got, err := c.MyPlaylists(context.Background(), "me")
	if err != nil || len(got) != 5 {
		t.Fatalf("MyPlaylists = %+v, %v", got, err)
	}
	type flags struct {
		mine, collaborative, editable bool
		privacy                       media.Privacy
	}
	for i, want := range []flags{
		{true, false, true, media.Public},
		{true, true, true, media.Private},
		{false, true, true, media.Private},
		{false, false, false, media.Public},
		{false, false, false, ""},
	} {
		p := got[i]
		if f := (flags{p.Mine, p.Collaborative, p.Editable, p.Privacy}); f != want {
			t.Errorf("%s: %+v, want %+v", p.Title, f, want)
		}
	}
	if got[2].Owner != "Ana Testwood" {
		t.Errorf("the owner of a playlist of another account: %q", got[2].Owner)
	}
	// Signed out, nothing is the user's.
	if p := toPlaylist(&playlist{ID: "p1", Owner: owner{ID: ""}, Collaborative: true}, ""); p.Mine || p.Editable {
		t.Errorf("without a user: %+v", p)
	}
}
