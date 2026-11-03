package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/settings"
	"github.com/fzn0x/mixmusic/internal/spotify"
	"github.com/fzn0x/mixmusic/internal/youtube"
)

// spotifyEpisode is a Spotify podcast's episode.
var spotifyEpisode = media.Track{ID: "sp:e1", Source: media.SourceSpotify, Title: "Monday", DurationMs: 1830000,
	Episode: &media.Episode{Show: media.Ref{ID: "sp:s1", Name: "The Daily Thing"}}}

// TestSpotifyEpisodePlaysFromSpotify plays a Spotify episode on Spotify's
// devices, by its episode URI, and refuses to look for it on YouTube
// Music, which has no recording of it.
func TestSpotifyEpisodePlaysFromSpotify(t *testing.T) {
	var (
		mu   sync.Mutex
		uris []string
	)
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			URIs []string `json:"uris"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		uris = append(uris, body.URIs...)
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	for mode, want := range map[settings.SpotifyPlayback]PlaybackMode{
		settings.SpotifyPlaybackConnect: PlaybackConnect,
		settings.SpotifyPlaybackHere:    PlaybackSpotify,
	} {
		s := settings.Default()
		s.SpotifyPlayback = mode
		p := testPlayer(t, api, s)
		if src, err := p.Prepare(context.Background(), spotifyEpisode); err != nil || src.Mode != want {
			t.Errorf("%s: Prepare = %+v, %v", mode, src, err)
		}
	}
	s := settings.Default()
	s.SpotifyPlayback = settings.SpotifyPlaybackMatch
	p := testPlayer(t, api, s)
	if _, err := p.Prepare(context.Background(), spotifyEpisode); !errors.Is(err, errSpotifyEpisode) {
		t.Errorf("from YouTube Music: %v", err)
	}
	if (&Catalog{p.a}).SpotifyEpisodesPlay() {
		t.Error("Spotify's episodes play from YouTube Music")
	}
	if err := p.ConnectPlay(context.Background(), spotifyEpisode, 0); err != nil {
		t.Fatal(err)
	}
	song := media.Track{ID: "sp:t1", Source: media.SourceSpotify}
	if err := p.PlayHere(context.Background(), "mixmusic", spotifyEpisode, 0, song); err != nil {
		t.Fatal(err)
	}
	want := []string{"spotify:episode:e1", "spotify:episode:e1", "spotify:track:t1"}
	if !slices.Equal(uris, want) {
		t.Errorf("played %v, want %v", uris, want)
	}
}

// TestKeepPosition keeps where the episode that plays is, as the player
// reports it, but not while it loads, nor of a song.
func TestKeepPosition(t *testing.T) {
	pos, err := library.OpenPositions(filepath.Join(t.TempDir(), "positions.json"))
	if err != nil {
		t.Fatal(err)
	}
	a := &App{positions: pos}
	ep := spotifyEpisode
	a.keepPosition(NowPlaying{Track: &ep, Playing: true, PositionMs: 120000, DurationMs: 1830000})
	if got := (playerServices{a}).ResumeAt(ep); got != 115*time.Second {
		t.Errorf("ResumeAt = %v", got)
	}
	a.keepPosition(NowPlaying{Track: &ep, Loading: true, PositionMs: 300000, DurationMs: 1830000})
	if p, _ := pos.Get(ep.ID); p.PositionMs != 120000 {
		t.Errorf("a load moved the place to %d", p.PositionMs)
	}
	song := media.Track{ID: "yt:s", DurationMs: 200000}
	a.keepPosition(NowPlaying{Track: &song, Playing: true, PositionMs: 60000})
	if _, ok := pos.Get(song.ID); ok {
		t.Error("a song's place was kept")
	}
	(&Collection{a}).SetPlayed(ep, true)
	if p := (&Collection{a}).InProgress(0); len(p) != 0 {
		t.Errorf("in progress: %+v", p)
	}
}

// TestNoScrobbleOfEpisodes tells Last.fm of no episode.
func TestNoScrobbleOfEpisodes(t *testing.T) {
	ep := spotifyEpisode
	if id, _ := lastfmTrack(NowPlaying{Track: &ep, Playing: true}); id != "" {
		t.Errorf("an episode is observed as %q", id)
	}
}

// spotifyAPI is a Spotify Web API that answers each path with what
// answers holds for it, and records the requests.
func spotifyAPI(t *testing.T, answers map[string]string) (*spotify.Client, func() []string) {
	t.Helper()
	var (
		mu  sync.Mutex
		got []string
	)
	p := testPlayer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = append(got, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
		mu.Unlock()
		body, ok := answers[r.Method+" "+r.URL.Path]
		switch {
		case !ok:
			w.WriteHeader(http.StatusNotFound)
		case body == "":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.Write([]byte(body))
		}
	}), settings.Default())
	return p.a.spotify, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(got)
	}
}

// TestFollowSpotifyPodcast follows a show in MixMusic, and saves it to the
// user's library on Spotify, signed in.
func TestFollowSpotifyPodcast(t *testing.T) {
	sp, requests := spotifyAPI(t, map[string]string{
		"PUT /me/library":          "",
		"GET /shows/s1":            `{"id": "s1", "name": "The Daily Thing", "episodes": {"items": [{"id": "e1", "name": "Monday", "type": "episode"}], "next": null}}`,
		"GET /me/library/contains": `[true]`,
	})
	lib, err := library.Open(filepath.Join(t.TempDir(), "library.json"))
	if err != nil {
		t.Fatal(err)
	}
	a := &App{spotify: sp, library: lib}
	p, err := (&Catalog{a}).Podcast(context.Background(), "sp:s1")
	if err != nil || !p.Saved || len(p.Episodes) != 1 || p.Episodes[0].Episode.Show.Name != "The Daily Thing" {
		t.Fatalf("Podcast = %+v, %v", p, err)
	}
	c := &Collection{a}
	if err := c.SetPodcastFollowed(context.Background(), p, true); err != nil {
		t.Fatal(err)
	}
	if !c.Follows(p.ID) || len(c.Podcasts()) != 1 || len(c.Podcasts()[0].Episodes) != 0 {
		t.Errorf("follows %+v", c.Podcasts())
	}
	saved := false
	for _, r := range requests() {
		if strings.HasPrefix(r, "PUT /me/library?") && strings.Contains(r, url.QueryEscape("spotify:show:s1")) {
			saved = true
		}
	}
	if !saved {
		t.Errorf("not saved on Spotify: %q", requests())
	}
}

// TestSpotifyPodcastSignedOut asks the user to sign in to open Spotify's
// podcasts.
func TestSpotifyPodcastSignedOut(t *testing.T) {
	a := &App{spotify: spotify.NewClient(spotify.NewAuth("client"))}
	if _, err := (&Catalog{a}).Podcast(context.Background(), "sp:s1"); !errors.Is(err, errSpotifyShows) {
		t.Errorf("Podcast = %v", err)
	}
}

// TestSearchPodcastsFails tells why the search found nothing when YouTube
// Music fails, and Spotify isn't signed in to.
func TestSearchPodcastsFails(t *testing.T) {
	yt := youtube.New()
	yt.BaseURL = "http://127.0.0.1:9"
	a := &App{youtube: yt, spotify: spotify.NewClient(spotify.NewAuth("client"))}
	res, err := (&Catalog{a}).SearchPodcasts(context.Background(), "daily")
	if err == nil || len(res.Errors) != 1 || res.YouTube.Podcasts == nil || res.SpotifySignedIn {
		t.Errorf("SearchPodcasts = %+v, %v", res, err)
	}
	if res, err := (&Catalog{a}).SearchPodcasts(context.Background(), "  "); err != nil || len(res.Errors) != 0 {
		t.Errorf("an empty search = %+v, %v", res, err)
	}
}
