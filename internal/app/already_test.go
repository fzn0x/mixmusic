package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// TestAddToYouTubePlaylistAlready tells a song the playlist holds already,
// which YouTube Music refuses, and adds it anyway when asked. YouTube's
// answer, ../youtube/testdata/edit-playlist-duplicate.json, is written by
// hand from the documented shape of YouTube Music's, not recorded.
func TestAddToYouTubePlaylistAlready(t *testing.T) {
	var duplicate map[string]any
	if err := json.Unmarshal(recorded(t, "edit-playlist-duplicate"), &duplicate); err != nil {
		t.Fatal(err)
	}
	answer := duplicate
	a, f := signedInYouTube(t, func(string) (int, any) { return http.StatusOK, answer })
	c := &Collection{a: a}
	ctx := context.Background()
	song := media.Track{ID: "yt:v1", Source: media.SourceYouTube, Title: "Strobe"}
	if err := c.AddToYouTubePlaylist(ctx, "yt:PLmine", song); !errors.Is(err, ErrAlreadyInPlaylist) {
		t.Fatalf("a song the playlist holds: %v", err)
	}
	answer = signedInAnswer(map[string]any{"status": "STATUS_SUCCEEDED"})
	if err := c.AddToYouTubePlaylistAnyway(ctx, "yt:PLmine", song); err != nil {
		t.Fatal(err)
	}
	asks := f.asked()
	if len(asks) != 2 || asks[1].endpoint != "browse/edit_playlist" || asks[1].body["playlistId"] != "PLmine" {
		t.Fatalf("asked %+v", asks)
	}
	if a := asks[1].body["actions"].([]any)[0].(map[string]any); a["addedVideoId"] != "v1" || a["dedupeOption"] != "DEDUPE_OPTION_SKIP" {
		t.Errorf("add anyway sent %v", a)
	}
	// Signed out, nothing is asked.
	if err := (&Collection{a: &App{youtube: youtube.New()}}).AddToYouTubePlaylistAnyway(ctx, "yt:PLmine", song); !errors.Is(err, errNotSignedIn) {
		t.Errorf("signed out: %v", err)
	}
}

// TestAddToMixAlready leaves out of a mix the tracks it holds, tells when
// it held them all, and adds them anyway when asked.
func TestAddToMixAlready(t *testing.T) {
	lib, err := library.Open(filepath.Join(t.TempDir(), "library.json"))
	if err != nil {
		t.Fatal(err)
	}
	c := &Collection{a: &App{library: lib}}
	a := media.Track{ID: "yt:a", Source: media.SourceYouTube, Title: "A"}
	b := media.Track{ID: "sp:b", Source: media.SourceSpotify, Title: "B"}
	mix, err := c.CreatePlaylist("Road trip", "", []media.Track{a})
	if err != nil {
		t.Fatal(err)
	}
	held, err := c.AddToMix(mix.ID, []media.Track{a, b})
	if err != nil || len(held) != 1 || held[0].ID != a.ID {
		t.Fatalf("some held: %+v, %v", held, err)
	}
	if held, err := c.AddToMix(mix.ID, []media.Track{b}); !errors.Is(err, ErrAlreadyInPlaylist) || len(held) != 1 {
		t.Errorf("all held: %+v, %v", held, err)
	}
	if err := c.AddToMixAnyway(mix.ID, []media.Track{b}); err != nil {
		t.Fatal(err)
	}
	_, native, _ := media.ParseID(mix.ID)
	got, _ := lib.Playlist(native)
	if len(got.Tracks) != 3 || got.Tracks[2].ID != b.ID {
		t.Errorf("the mix holds %d tracks", len(got.Tracks))
	}
	if _, err := c.AddToMix("yt:PLx", []media.Track{a}); !errors.Is(err, library.ErrNoPlaylist) {
		t.Errorf("a playlist that isn't a mix: %v", err)
	}
}
