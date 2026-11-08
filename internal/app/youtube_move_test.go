package app

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// TestMoveInYouTubePlaylist moves a track of a playlist of YouTube Music
// by its MixMusic ID, before another by their places in the playlist, and
// nothing of another service or signed out.
func TestMoveInYouTubePlaylist(t *testing.T) {
	ctx := context.Background()
	track := media.Track{ID: "yt:v1", Source: media.SourceYouTube, EntryID: "set1"}
	before := media.Track{ID: "yt:v2", Source: media.SourceYouTube, EntryID: "set2"}
	if err := (&Collection{a: &App{youtube: youtube.New()}}).MoveInYouTubePlaylist(ctx, "yt:PLmine", track, before); !errors.Is(err, errNotSignedIn) {
		t.Errorf("signed out: %v", err)
	}
	a, f := signedInYouTube(t, func(string) (int, any) { return http.StatusOK, map[string]any{"status": "STATUS_SUCCEEDED"} })
	col := &Collection{a: a}
	if err := col.MoveInYouTubePlaylist(ctx, "mx:1", track, before); err == nil {
		t.Error("moved a track of a mix on YouTube")
	}
	if err := col.MoveInYouTubePlaylist(ctx, "yt:PLmine", track, before); err != nil {
		t.Fatal(err)
	}
	asks := f.asked()
	if len(asks) != 1 || asks[0].endpoint != "browse/edit_playlist" || asks[0].body["playlistId"] != "PLmine" {
		t.Fatalf("asked %+v", asks)
	}
	if a := asks[0].body["actions"].([]any)[0].(map[string]any); a["setVideoId"] != "set1" || a["movedSetVideoIdSuccessor"] != "set2" {
		t.Errorf("the move = %v", a)
	}
}
