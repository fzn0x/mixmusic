package app

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// TestSetSongSaved saves a YouTube Music song to the library, by its
// MixMusic ID, with the token of its menu, and nothing of another service,
// an episode, or signed out.
func TestSetSongSaved(t *testing.T) {
	ctx := context.Background()
	song := media.Track{ID: "yt:vid", Source: media.SourceYouTube, Title: "Strobe"}
	if err := (&Collection{a: &App{youtube: youtube.New()}}).SetSongSaved(ctx, song, true); !errors.Is(err, errNotSignedIn) {
		t.Errorf("signed out: %v", err)
	}

	toggle := map[string]any{"toggleMenuServiceItemRenderer": map[string]any{
		"defaultIcon":            map[string]any{"iconType": "LIBRARY_ADD"},
		"defaultServiceEndpoint": map[string]any{"feedbackEndpoint": map[string]any{"feedbackToken": "add-vid"}},
		"toggledServiceEndpoint": map[string]any{"feedbackEndpoint": map[string]any{"feedbackToken": "remove-vid"}},
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
	a, f := signedInYouTube(t, func(endpoint string) (int, any) {
		if endpoint == "next" {
			return http.StatusOK, signedInAnswer(next)
		}
		return http.StatusOK, signedInAnswer(map[string]any{"feedbackResponses": []any{map[string]any{"isProcessed": true}}})
	})
	col := &Collection{a: a}
	for _, other := range []media.Track{
		{ID: media.ID(media.SourceSpotify, "2"), Source: media.SourceSpotify, Title: "Teardrop"},
		{ID: "yt:ep", Source: media.SourceYouTube, Title: "An episode", Episode: &media.Episode{}},
	} {
		if err := col.SetSongSaved(ctx, other, true); !errors.Is(err, errSongsOnYouTube) {
			t.Errorf("%s: %v", other.Title, err)
		}
	}
	if asks := f.asked(); len(asks) != 0 {
		t.Fatalf("for other tracks, YouTube was asked %+v", asks)
	}
	if err := col.SetSongSaved(ctx, song, true); err != nil {
		t.Fatal(err)
	}
	asks := f.asked()
	if len(asks) != 2 || asks[0].endpoint != "next" || asks[0].body["videoId"] != "vid" || asks[1].endpoint != "feedback" {
		t.Fatalf("asked %+v", asks)
	}
	if tokens, _ := asks[1].body["feedbackTokens"].([]any); len(tokens) != 1 || tokens[0] != "add-vid" {
		t.Errorf("the feedback sent %v", asks[1].body["feedbackTokens"])
	}
}
