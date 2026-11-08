package app

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// TestPlaylistSuggestions reads the songs YouTube Music suggests for a
// playlist of the user's, signed in, without those the user dislikes. Its
// answer, ../youtube/testdata/playlist-suggestions.json, is written by
// hand from the documented shape of YouTube Music's, not recorded.
func TestPlaylistSuggestions(t *testing.T) {
	ctx := context.Background()
	if _, err := (&Collection{a: &App{youtube: youtube.New()}}).PlaylistSuggestions(ctx, "S1"); !errors.Is(err, errNotSignedIn) {
		t.Errorf("signed out: %v", err)
	}
	var asked []string
	a := dislikeApp(t, func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		w.Write(recorded(t, "playlist-suggestions"))
	}, nil)
	if _, err := a.library.SetDisliked(media.Track{ID: "yt:sugVideo002", Source: media.SourceYouTube, Title: "Paper Kites"}, true); err != nil {
		t.Fatal(err)
	}
	got, err := (&Collection{a: a}).PlaylistSuggestions(ctx, "S1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Tracks) != 4 || got.Refresh != "SUGGESTIONS-REFRESH-1" || len(asked) != 1 || asked[0] != "/youtubei/v1/browse" {
		t.Fatalf("suggestions: %d tracks, refresh %q, asked %v", len(got.Tracks), got.Refresh, asked)
	}
	for _, tr := range got.Tracks {
		if tr.ID == "yt:sugVideo002" {
			t.Error("a song the user dislikes is suggested")
		}
	}
}
