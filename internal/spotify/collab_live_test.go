package spotify

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
)

// TestLiveCollaboration changes a playlist on a real account as MixMusic
// does, to find what Spotify lets an app in development mode do: it makes
// a private playlist, adds two tracks, moves one, makes the playlist
// collaborative and then the user's alone again, and takes it out of the
// library at the end, which is how Spotify deletes a playlist. It changes
// the account, so it runs only with MIXMUSIC_SPOTIFY_WRITE=1 and
// MIXMUSIC_SPOTIFY_TOKEN, an access token of the account (it isn't
// refreshed), best one kept for tests:
//
//	MIXMUSIC_LIVE=1 MIXMUSIC_SPOTIFY_WRITE=1 MIXMUSIC_SPOTIFY_TOKEN=… go test ./internal/spotify -run LiveCollaboration -v
func TestLiveCollaboration(t *testing.T) {
	token := os.Getenv("MIXMUSIC_SPOTIFY_TOKEN")
	if os.Getenv("MIXMUSIC_LIVE") == "" || os.Getenv("MIXMUSIC_SPOTIFY_WRITE") == "" || token == "" {
		t.Skip("set MIXMUSIC_LIVE=1, MIXMUSIC_SPOTIFY_WRITE=1 and MIXMUSIC_SPOTIFY_TOKEN to change a playlist of a signed-in account")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	c := NewClient(NewAuth(""))
	c.SetToken(Token{AccessToken: token, Expiry: time.Now().Add(time.Hour)})
	me, err := c.Me(ctx)
	if err != nil {
		t.Fatal(err)
	}

	var made struct {
		ID string `json:"id"`
	}
	if err := c.do(ctx, http.MethodPost, "/me/playlists", nil, map[string]any{
		"name": "MixMusic live test " + time.Now().Format("2006-01-02 15:04:05"), "public": false,
		"description": "Made by a test of MixMusic, which deletes it at its end.",
	}, &made); err != nil {
		t.Fatalf("making a playlist: %v", err)
	}
	t.Logf("made %s", made.ID)
	t.Cleanup(func() {
		ctx := context.Background()
		err := c.FollowPlaylist(ctx, made.ID, false)
		t.Logf("taking it out of the library with DELETE /playlists/{id}/followers: %v", err)
		if err != nil {
			err = c.Remove(ctx, "spotify:playlist:"+made.ID)
			t.Logf("with DELETE /me/library: %v", err)
		}
		if err != nil {
			t.Errorf("the playlist %s is left on the account", made.ID)
			return
		}
		mine, err := c.MyPlaylists(ctx, me.ID)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range mine {
			if p.ID == media.ID(media.SourceSpotify, made.ID) {
				t.Errorf("the library still lists %s", made.ID)
			}
		}
	})

	// Two tracks of a royalty-free composer, by search.
	found, err := c.Search(ctx, "Kevin MacLeod", "")
	if err != nil || len(found.Tracks) < 2 {
		t.Fatalf("searching: %d tracks, %v", len(found.Tracks), err)
	}
	var uris []string
	for _, tr := range found.Tracks[:2] {
		_, id, err := media.ParseID(tr.ID)
		if err != nil {
			t.Fatal(err)
		}
		uris = append(uris, "spotify:track:"+id)
	}
	if err := c.AddToPlaylist(ctx, made.ID, uris...); err != nil {
		t.Fatalf("adding: %v", err)
	}
	order := func(name string) media.Playlist {
		t.Helper()
		p, err := c.Playlist(ctx, made.ID, me.ID)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var ids []string
		for _, tr := range p.Tracks {
			ids = append(ids, tr.ID)
		}
		t.Logf("%s: mine %v, editable %v, collaborative %v, tracks %v", name, p.Mine, p.Editable, p.Collaborative, ids)
		return p
	}
	first := order("added")
	if len(first.Tracks) != 2 {
		t.Fatalf("added two tracks, the playlist has %d", len(first.Tracks))
	}

	if err := c.MoveInPlaylist(ctx, made.ID, 1, 0); err != nil {
		t.Fatalf("moving: %v", err)
	}
	if moved := order("moved"); moved.Tracks[0].ID != first.Tracks[1].ID {
		t.Errorf("the second track didn't move to the top")
	}

	if err := c.SetCollaborative(ctx, made.ID, true); err != nil {
		t.Fatalf("making it collaborative: %v", err)
	}
	if p := order("collaborative"); !p.Collaborative {
		t.Errorf("Spotify took the change but doesn't say it's collaborative")
	}
	if err := c.SetCollaborative(ctx, made.ID, false); err != nil {
		t.Fatalf("making it the user's alone: %v", err)
	}
	if p := order("theirs alone"); p.Collaborative {
		t.Errorf("Spotify took the change but says it's still collaborative")
	}
}
