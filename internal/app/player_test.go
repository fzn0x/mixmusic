package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/spotify"
	"github.com/fzn0x/mixmusic/internal/store"
)

// testPlayer returns a player whose Spotify client calls api, with the
// settings s.
func testPlayer(t *testing.T, api http.Handler, s settings.Settings) *Player {
	t.Helper()
	srv := httptest.NewServer(api)
	t.Cleanup(srv.Close)
	c := spotify.NewClient(spotify.NewAuth("client"))
	c.BaseURL = srv.URL
	c.SetToken(spotify.Token{AccessToken: "ACCESS", Expiry: time.Now().Add(time.Hour), Scope: "streaming user-read-private"})
	f, err := store.Open(filepath.Join(t.TempDir(), "settings.json"), s)
	if err != nil {
		t.Fatal(err)
	}
	return &Player{a: &App{spotify: c, settings: f}}
}

func TestPrepareSpotifyTrack(t *testing.T) {
	track := media.Track{ID: "sp:t1", Source: media.SourceSpotify, Title: "One More Time"}
	for mode, want := range map[settings.SpotifyPlayback]PlaybackMode{
		settings.SpotifyPlaybackConnect: PlaybackConnect,
		settings.SpotifyPlaybackHere:    PlaybackSpotify,
	} {
		s := settings.Default()
		s.SpotifyPlayback = mode
		p := testPlayer(t, http.NotFoundHandler(), s)
		src, err := p.Prepare(context.Background(), track)
		if err != nil || src.Mode != want || src.Track.ID != track.ID {
			t.Errorf("%s: Prepare = %+v, %v; want mode %s", mode, src, err, want)
		}
	}
}

// TestPlayHereAhead plays the Spotify tracks ahead after the track, in
// their order, up to the first that isn't one.
func TestPlayHereAhead(t *testing.T) {
	var body struct {
		URIs       []string `json:"uris"`
		PositionMs int      `json:"position_ms"`
	}
	p := testPlayer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		w.WriteHeader(http.StatusNoContent)
	}), settings.Default())
	sp := func(id string) media.Track { return media.Track{ID: "sp:" + id, Source: media.SourceSpotify} }
	err := p.PlayHere(context.Background(), "mixmusic", sp("t1"), 1500, sp("t2"), sp("t3"),
		media.Track{ID: "yt:v1", Source: media.SourceYouTube}, sp("t4"))
	want := []string{"spotify:track:t1", "spotify:track:t2", "spotify:track:t3"}
	if err != nil || !slices.Equal(body.URIs, want) || body.PositionMs != 1500 {
		t.Errorf("PlayHere = %v, playing %v from %d; want %v from 1500", err, body.URIs, body.PositionMs, want)
	}
}

func TestPlayHereWaitsForTheDevice(t *testing.T) {
	playHereRetry = time.Millisecond
	t.Cleanup(func() { playHereRetry = time.Second })
	var calls atomic.Int32
	p := testPlayer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/me/player/play" || r.URL.Query().Get("device_id") != "mixmusic" {
			t.Errorf("asked %s %s", r.Method, r.URL)
		}
		// Spotify does not know a device that just connected, for a moment.
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error": {"status": 404, "message": "Device not found"}}`))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}), settings.Default())
	track := media.Track{ID: "sp:t1", Source: media.SourceSpotify}
	if err := p.PlayHere(context.Background(), "mixmusic", track, 0); err != nil || calls.Load() != 3 {
		t.Errorf("PlayHere = %v after %d calls, want nil after 3", err, calls.Load())
	}

	// It gives up on a device Spotify never knows.
	calls.Store(-100)
	if err := p.PlayHere(context.Background(), "mixmusic", track, 0); err == nil || calls.Load() != -95 {
		t.Errorf("PlayHere = %v after %d calls, want an error after 5", err, calls.Load()+100)
	}
	// And plays only Spotify tracks.
	if err := p.PlayHere(context.Background(), "mixmusic", media.Track{ID: "yt:v1", Source: media.SourceYouTube}, 0); err == nil {
		t.Error("PlayHere played a YouTube track")
	}
}
