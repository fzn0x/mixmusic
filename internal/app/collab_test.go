package app

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
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/spotify"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// editActions returns the actions of the edits of playlists the fake
// YouTube was asked, in order.
func editActions(f *fakeYouTube) []string {
	var out []string
	for _, ask := range f.asked() {
		if ask.endpoint != "browse/edit_playlist" {
			continue
		}
		for _, a := range ask.body["actions"].([]any) {
			out = append(out, a.(map[string]any)["action"].(string))
		}
	}
	return out
}

// TestInviteYouTubeCollaborators makes a private playlist unlisted first,
// as the user agreed to, and then invites, returning YouTube's link; a
// refused invite after the unlisting says the playlist is unlisted now.
func TestInviteYouTubeCollaborators(t *testing.T) {
	const link = "https://music.youtube.com/playlist?list=PLmine&jct=testToken"
	// Once fails is set, the edit after the next one fails.
	var (
		mu    sync.Mutex
		fails bool
		edits int
	)
	a, f := signedInYouTube(t, func(endpoint string) (int, any) {
		mu.Lock()
		defer mu.Unlock()
		status := "STATUS_SUCCEEDED"
		if fails {
			if edits++; edits == 2 {
				status = "STATUS_FAILED"
			}
		}
		return http.StatusOK, signedInAnswer(map[string]any{"status": status, "collaborationInviteLink": link})
	})
	c := &Collection{a: a}
	ctx := context.Background()

	if got, err := c.InviteYouTubeCollaborators(ctx, "yt:PLmine", false); err != nil || got != link {
		t.Fatalf("InviteYouTubeCollaborators = %q, %v", got, err)
	}
	if got := editActions(f); len(got) != 1 || got[0] != "ACTION_CREATE_COLLABORATION_INVITE_LINK" {
		t.Errorf("an unlisted playlist asked %v", got)
	}
	if got, err := c.InviteYouTubeCollaborators(ctx, "yt:PLmine", true); err != nil || got != link {
		t.Fatalf("unlisting first: %q, %v", got, err)
	}
	if got := editActions(f)[1:]; len(got) != 2 || got[0] != "ACTION_SET_PLAYLIST_PRIVACY" || got[1] != "ACTION_CREATE_COLLABORATION_INVITE_LINK" {
		t.Errorf("a private playlist asked %v", got)
	}
	if err := c.StopYouTubeCollaboration(ctx, "yt:PLmine"); err != nil {
		t.Fatal(err)
	}
	if got := editActions(f); got[len(got)-1] != "ACTION_SET_CLOSED_TO_CONTRIBUTIONS" {
		t.Errorf("the stop asked %v", got)
	}

	mu.Lock()
	fails = true
	mu.Unlock()
	if _, err := c.InviteYouTubeCollaborators(ctx, "yt:PLmine", true); err == nil || !strings.Contains(err.Error(), "unlisted now") {
		t.Errorf("refused after the unlisting: %v", err)
	}

	asked := len(f.asked())
	if _, err := c.InviteYouTubeCollaborators(ctx, "sp:p1", false); err == nil {
		t.Error("invited to a Spotify playlist on YouTube Music")
	}
	signedOut := &Collection{a: &App{youtube: youtube.New()}}
	if _, err := signedOut.InviteYouTubeCollaborators(ctx, "yt:PLmine", true); !errors.Is(err, errSignInToCollaborate) {
		t.Errorf("signed out: %v", err)
	}
	if err := signedOut.StopYouTubeCollaboration(ctx, "yt:PLmine"); !errors.Is(err, errSignInToCollaborate) {
		t.Errorf("signed out, stopping: %v", err)
	}
	if len(f.asked()) != asked {
		t.Errorf("asked YouTube %d times more", len(f.asked())-asked)
	}
}

// TestShareOnYouTube makes a playlist on YouTube Music that anyone with
// its link can open, the user's own.
func TestShareOnYouTube(t *testing.T) {
	a, f := signedInYouTube(t, func(endpoint string) (int, any) {
		return http.StatusOK, signedInAnswer(map[string]any{"playlistId": "PLshared"})
	})
	p, err := (&Collection{a: a}).ShareOnYouTube(context.Background(), " Road songs ", nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "yt:PLshared" || p.Privacy != media.Unlisted || !p.Mine || !p.Editable || p.Title != "Road songs" {
		t.Errorf("the playlist made = %+v", p)
	}
	if asked := f.asked(); len(asked) != 1 || asked[0].endpoint != "playlist/create" || asked[0].body["privacyStatus"] != "UNLISTED" {
		t.Errorf("YouTube was asked %+v", asked)
	}
}

// spotifyChange is a request a fake Spotify got.
type spotifyChange struct {
	method, path string
	body         map[string]any
}

// signedInSpotify is an App signed in to a fake Spotify that answers each
// request with status, and records them.
func signedInSpotify(t *testing.T, status int) (*App, func() []spotifyChange) {
	t.Helper()
	var (
		mu  sync.Mutex
		got []spotifyChange
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		var body map[string]any
		json.Unmarshal(data, &body)
		mu.Lock()
		got = append(got, spotifyChange{r.Method, r.URL.Path, body})
		mu.Unlock()
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	client := spotify.NewClient(spotify.NewAuth("client"))
	client.BaseURL = srv.URL
	client.SetToken(spotify.Token{AccessToken: "ACCESS", Expiry: time.Now().Add(time.Hour)})
	return &App{spotify: client}, func() []spotifyChange {
		mu.Lock()
		defer mu.Unlock()
		return append([]spotifyChange(nil), got...)
	}
}

// TestSpotifyPlaylistChanges makes a Spotify playlist collaborative, moves
// its tracks and adds Spotify's to it, by their MixMusic IDs; signed out,
// or of another service, nothing is asked.
func TestSpotifyPlaylistChanges(t *testing.T) {
	a, got := signedInSpotify(t, http.StatusOK)
	c := &Collection{a: a}
	ctx := context.Background()
	if err := c.SetSpotifyCollaborative(ctx, "sp:p1", true); err != nil {
		t.Fatal(err)
	}
	if err := c.MoveInSpotifyPlaylist(ctx, "sp:p1", 2, 0); err != nil {
		t.Fatal(err)
	}
	song := media.Track{ID: "sp:t1", Source: media.SourceSpotify, Title: "Paper Lanterns"}
	video := media.Track{ID: "yt:v1", Source: media.SourceYouTube, Title: "Paper Lanterns (live)"}
	if err := c.AddToSpotifyPlaylist(ctx, "sp:p1", []media.Track{song, video}); err != nil {
		t.Fatal(err)
	}
	asked := got()
	want := []string{"PUT /playlists/p1", "PUT /playlists/p1/items", "POST /playlists/p1/items"}
	if len(asked) != len(want) {
		t.Fatalf("asked %+v", asked)
	}
	for i, w := range want {
		if asked[i].method+" "+asked[i].path != w {
			t.Errorf("request %d = %s %s, want %s", i, asked[i].method, asked[i].path, w)
		}
	}
	if uris, _ := asked[2].body["uris"].([]any); len(uris) != 1 || uris[0] != "spotify:track:t1" {
		t.Errorf("the add sent %v", asked[2].body)
	}

	if err := c.AddToSpotifyPlaylist(ctx, "sp:p1", []media.Track{video}); !errors.Is(err, errSpotifyOnly) {
		t.Errorf("a YouTube Music track alone: %v", err)
	}
	if err := c.SetSpotifyCollaborative(ctx, "yt:PLmine", true); !errors.Is(err, errNotSpotify) {
		t.Errorf("a playlist of YouTube Music: %v", err)
	}
	signedOut := &Collection{a: &App{spotify: spotify.NewClient(spotify.NewAuth("client"))}}
	if err := signedOut.MoveInSpotifyPlaylist(ctx, "sp:p1", 0, 1); !errors.Is(err, errSignInToCollaborate) {
		t.Errorf("signed out: %v", err)
	}
	if n := len(got()); n != len(want) {
		t.Errorf("asked Spotify %d times more", n-len(want))
	}

	// Spotify's refusal says what may be wrong.
	refused, _ := signedInSpotify(t, http.StatusForbidden)
	if err := (&Collection{a: refused}).SetSpotifyCollaborative(ctx, "sp:p1", true); !errors.Is(err, spotify.ErrPlaylistRefused) {
		t.Errorf("refused: %v", err)
	}
}
