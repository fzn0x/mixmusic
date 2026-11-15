package app

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// TestSongSaved asks YouTube Music whether a song is in the library, by
// its MixMusic ID, from its menu, and nothing of another service, an
// episode, or signed out.
func TestSongSaved(t *testing.T) {
	ctx := context.Background()
	song := media.Track{ID: "yt:vid", Source: media.SourceYouTube, Title: "Strobe"}
	if _, err := (&Collection{a: &App{youtube: youtube.New()}}).SongSaved(ctx, song); !errors.Is(err, errNotSignedIn) {
		t.Errorf("signed out: %v", err)
	}
	toggle := map[string]any{"toggleMenuServiceItemRenderer": map[string]any{
		"defaultIcon":            map[string]any{"iconType": "BOOKMARK_BORDER"},
		"defaultServiceEndpoint": map[string]any{"feedbackEndpoint": map[string]any{"feedbackToken": "add-vid"}},
		"toggledServiceEndpoint": map[string]any{"feedbackEndpoint": map[string]any{"feedbackToken": "remove-vid"}},
		"isToggled":              true,
	}}
	entry := map[string]any{"playlistPanelVideoRenderer": map[string]any{"videoId": "vid",
		"menu": map[string]any{"menuRenderer": map[string]any{"items": []any{toggle}}}}}
	next := map[string]any{"contents": map[string]any{"singleColumnMusicWatchNextResultsRenderer": map[string]any{
		"tabbedRenderer": map[string]any{"watchNextTabbedResultsRenderer": map[string]any{"tabs": []any{
			map[string]any{"tabRenderer": map[string]any{"content": map[string]any{"musicQueueRenderer": map[string]any{
				"content": map[string]any{"playlistPanelRenderer": map[string]any{"contents": []any{entry}}},
			}}}},
		}}},
	}}}
	a, f := signedInYouTube(t, func(string) (int, any) { return http.StatusOK, signedInAnswer(next) })
	col := &Collection{a: a}
	for _, other := range []media.Track{
		{ID: media.ID(media.SourceSpotify, "2"), Source: media.SourceSpotify, Title: "Teardrop"},
		{ID: "yt:ep", Source: media.SourceYouTube, Title: "An episode", Episode: &media.Episode{}},
	} {
		if _, err := col.SongSaved(ctx, other); !errors.Is(err, errSongsOnYouTube) {
			t.Errorf("%s: %v", other.Title, err)
		}
	}
	if saved, err := col.SongSaved(ctx, song); err != nil || !saved {
		t.Fatalf("saved %v, %v", saved, err)
	}
	if asks := f.asked(); len(asks) != 1 || asks[0].endpoint != "next" || asks[0].body["videoId"] != "vid" {
		t.Errorf("asked %+v", asks)
	}
}
