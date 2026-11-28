package spotify

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTrackLyricsSyncedUnsyncedAndHoldoff(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("app-platform") != "WebPlayer" || r.Header.Get("Origin") != "https://open.spotify.com" {
			t.Errorf("missing web player headers: app-platform=%q origin=%q", r.Header.Get("app-platform"), r.Header.Get("Origin"))
		}
		if r.Header.Get("Authorization") != "Bearer tok123" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/color-lyrics/v2/track/synced1":
			w.Write([]byte(`{
				"lyrics": {
					"syncType": "LINE_SYNCED",
					"language": "en",
					"lines": [
						{"startTimeMs": "0", "words": "♪"},
						{"startTimeMs": "13570", "words": "Look at the stars"},
						{"startTimeMs": "16500", "words": "Look how they shine"}
					]
				}
			}`))
		case "/color-lyrics/v2/track/plain1":
			w.Write([]byte(`{
				"lyrics": {
					"syncType": "UNSYNCED",
					"language": "ja",
					"lines": [
						{"startTimeMs": "0", "words": "無敵の笑顔"},
						{"startTimeMs": "0", "words": "天才的なアイドル様"}
					]
				}
			}`))
		case "/color-lyrics/v2/track/forbidden":
			w.WriteHeader(http.StatusForbidden)
		case "/color-lyrics/v2/track/rate429":
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := NewClient(NewAuth("client"))
	c.LyricsURL = srv.URL

	// Non-web session must not call Spotify lyrics.
	c.SetToken(Token{AccessToken: "tok123", Expiry: time.Now().Add(time.Hour)})
	if _, err := c.TrackLyrics(context.Background(), "synced1"); !errors.Is(err, ErrNotSignedIn) {
		t.Fatalf("non-web session err = %v, want ErrNotSignedIn", err)
	}

	// Web session (sp_dc present).
	c.SetToken(Token{AccessToken: "tok123", SPDC: "cookie", Expiry: time.Now().Add(time.Hour)})
	got, err := c.TrackLyrics(context.Background(), "synced1")
	if err != nil {
		t.Fatalf("TrackLyrics(synced1): %v", err)
	}
	if !got.Synced || got.Language != "en" || len(got.Lines) != 2 || got.Lines[0].StartMs != 13570 || got.Lines[0].Words != "Look at the stars" {
		t.Fatalf("unexpected synced lyrics: %+v", got)
	}

	plain, err := c.TrackLyrics(context.Background(), "plain1")
	if err != nil {
		t.Fatalf("TrackLyrics(plain1): %v", err)
	}
	if plain.Synced || plain.Language != "ja" || len(plain.Lines) != 2 {
		t.Fatalf("unexpected plain lyrics: %+v", plain)
	}

	if _, err := c.TrackLyrics(context.Background(), "missing"); !errors.Is(err, ErrLyricsNotFound) {
		t.Errorf("404 err = %v, want ErrLyricsNotFound", err)
	}
	if _, err := c.TrackLyrics(context.Background(), "forbidden"); !errors.Is(err, ErrLyricsNotFound) {
		t.Errorf("403 err = %v, want ErrLyricsNotFound", err)
	}

	if _, err := c.TrackLyrics(context.Background(), "rate429"); !errors.Is(err, ErrLyricsRateLimited) {
		t.Errorf("429 err = %v, want ErrLyricsRateLimited", err)
	}
	// Holdoff active: must return rate limited immediately.
	if _, err := c.TrackLyrics(context.Background(), "synced1"); !errors.Is(err, ErrLyricsRateLimited) {
		t.Errorf("during holdoff err = %v, want ErrLyricsRateLimited", err)
	}
}
