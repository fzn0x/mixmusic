package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"sync"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/spotify"
	"github.com/fzn0x/mixmusic/internal/youtube"
	"github.com/fzn0x/mixmusic/locales"
)

// The song of the recorded radio next-song, which starts with it.
const linkedSong = "yt:khnokW3Mw24"

// TestLinkYouTube reads a YouTube song from its radio, whose first track
// it is, and the song of a radio link with that radio after it, without
// the songs the user disliked, as other radios leave them out.
func TestLinkYouTube(t *testing.T) {
	ctx := context.Background()
	answer := recorded(t, "next-song")
	var (
		mu    sync.Mutex
		lists []string
	)
	a := dislikeApp(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct{ VideoID, PlaylistID string }
		json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		lists = append(lists, body.PlaylistID)
		mu.Unlock()
		if r.URL.Path == "/youtubei/v1/next" && "yt:"+body.VideoID == linkedSong {
			w.Write(answer)
			return
		}
		w.Write(nothing)
	}, func(w http.ResponseWriter, r *http.Request) { t.Errorf("asked Spotify for %s", r.URL.Path) })
	c := &Catalog{a}

	got, err := c.Link(ctx, media.Link{Kind: media.LinkSong, Source: media.SourceYouTube, ID: linkedSong})
	if err != nil || got.Item.Track == nil || got.Item.Track.ID != linkedSong || got.Item.Track.Title == "" || got.Radio != nil {
		t.Fatalf("the song's link = %+v, %v", got, err)
	}
	radio := media.Link{Kind: media.LinkRadio, Source: media.SourceYouTube, ID: linkedSong, Radio: "yt:RDEMlinkedradio"}
	got, err = c.Link(ctx, radio)
	if err != nil || got.Item.Track == nil || got.Item.Track.ID != linkedSong || len(got.Radio) != 4 {
		t.Fatalf("the radio's link = %+v, %v", got, err)
	}
	disliked := got.Radio[1]
	if _, err := a.library.SetDisliked(disliked, true); err != nil {
		t.Fatal(err)
	}
	got, err = c.Link(ctx, radio)
	if err != nil || len(got.Radio) != 3 || slices.ContainsFunc(got.Radio, func(t media.Track) bool { return t.ID == disliked.ID }) {
		t.Errorf("the radio after a dislike = %v, %v", trackIDs(got.Radio), err)
	}
	// A video the radio doesn't start with isn't YouTube's.
	if _, err := c.Link(ctx, media.Link{Kind: media.LinkSong, Source: media.SourceYouTube, ID: "yt:zzzzzzzzzzz"}); !errors.Is(err, youtube.ErrNotFound) {
		t.Errorf("a song YouTube doesn't have: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if want := []string{"RDAMVMkhnokW3Mw24", "RDEMlinkedradio", "RDEMlinkedradio", "RDAMVMzzzzzzzzzzz"}; !slices.Equal(lists, want) {
		t.Errorf("the radios asked = %q, want %q", lists, want)
	}
}

// TestLinkAlbumPlaylist reads the link of the playlist of an album's
// tracks (OLAK5uy_…) as the album's, and that of a playlist of its own as
// the playlist.
func TestLinkAlbumPlaylist(t *testing.T) {
	ctx := context.Background()
	answers := map[string][]byte{
		"VLOLAK5uy_mByMItZzRVpiPOPfqNdCyzqCEs1k7Ow14": recorded(t, "playlist-album"),
		"MPREb_I0lA998lMLT":                           recorded(t, "album"),
		"VLOLAK5uy_songs":                             recorded(t, "playlist"),
	}
	a := dislikeApp(t, func(w http.ResponseWriter, r *http.Request) {
		var body struct{ BrowseID string }
		json.NewDecoder(r.Body).Decode(&body)
		if answer, ok := answers[body.BrowseID]; ok && r.URL.Path == "/youtubei/v1/browse" {
			w.Write(answer)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}, func(w http.ResponseWriter, r *http.Request) { t.Errorf("asked Spotify for %s", r.URL.Path) })
	c := &Catalog{a}
	got, err := c.Link(ctx, media.Link{Kind: media.LinkPlaylist, Source: media.SourceYouTube, ID: "yt:OLAK5uy_mByMItZzRVpiPOPfqNdCyzqCEs1k7Ow14"})
	if al := got.Item.Album; err != nil || al == nil || al.ID != "yt:MPREb_I0lA998lMLT" || al.Title != "Q-ENA II" {
		t.Errorf("the album's playlist = %+v, %v", got, err)
	}
	got, err = c.Link(ctx, media.Link{Kind: media.LinkPlaylist, Source: media.SourceYouTube, ID: "yt:OLAK5uy_songs"})
	if p := got.Item.Playlist; err != nil || p == nil || p.ID != "yt:OLAK5uy_songs" || p.Title == "" {
		t.Errorf("a playlist of its own = %+v, %v", got, err)
	}
}

// TestLinkSpotify reads a Spotify track with Get Track, a playlist of
// others, whose page says why its tracks don't show, and the podcast of
// a show and of an episode. Signed out, it asks the user to sign in, and
// asks Spotify nothing.
func TestLinkSpotify(t *testing.T) {
	ctx := context.Background()
	var asked requestLog
	a := dislikeApp(t, func(w http.ResponseWriter, r *http.Request) { t.Errorf("asked YouTube for %s", r.URL.Path) },
		func(w http.ResponseWriter, r *http.Request) {
			asked.note(r)
			switch r.URL.Path {
			case "/me":
				w.Write([]byte(`{"id": "me", "display_name": "Me"}`))
			case "/tracks/t1":
				w.Write([]byte(`{"id": "t1", "name": "One More Time", "type": "track", "duration_ms": 320357,
					"artists": [{"id": "a1", "name": "Daft Punk"}], "album": {"id": "al1", "name": "Discovery", "images": []}}`))
			case "/tracks/t2":
				w.WriteHeader(http.StatusForbidden)
			case "/playlists/p1":
				w.Write([]byte(`{"id": "p1", "name": "Thai Viral Song", "owner": {"id": "them", "display_name": "Them"}}`))
			case "/episodes/e1":
				w.Write([]byte(`{"id": "e1", "name": "Monday", "type": "episode", "show": {"id": "s1", "name": "The Daily Thing"}}`))
			case "/shows/s1":
				w.Write([]byte(`{"id": "s1", "name": "The Daily Thing", "type": "show", "total_episodes": 120}`))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		})
	c := &Catalog{a}
	link := func(kind media.LinkKind, id string) media.Link {
		return media.Link{Kind: kind, Source: media.SourceSpotify, ID: "sp:" + id}
	}

	got, err := c.Link(ctx, link(media.LinkSong, "t1"))
	if err != nil || got.Item.Track == nil || got.Item.Track.ID != "sp:t1" || got.Item.Track.Title != "One More Time" {
		t.Fatalf("the track's link = %+v, %v", got, err)
	}
	if got := asked.take(); !slices.Equal(got, []string{"GET /tracks/t1"}) {
		t.Errorf("asked %q", got)
	}
	var refused *spotify.APIError
	if _, err := c.Link(ctx, link(media.LinkSong, "t2")); !errors.As(err, &refused) || refused.Status != http.StatusForbidden {
		t.Errorf("a track Spotify refuses: %v", err)
	}
	got, err = c.Link(ctx, link(media.LinkPlaylist, "p1"))
	if p := got.Item.Playlist; err != nil || p == nil || p.ID != "sp:p1" || p.Title != "Thai Viral Song" || p.Source != media.SourceSpotify {
		t.Errorf("a playlist of others = %+v, %v", got, err)
	}
	for _, l := range []media.Link{link(media.LinkPodcast, "s1"), link(media.LinkEpisode, "e1")} {
		got, err = c.Link(ctx, l)
		if p := got.Item.Podcast; err != nil || p == nil || p.ID != "sp:s1" || p.Title != "The Daily Thing" {
			t.Errorf("the link of %s = %+v, %v", l.ID, got, err)
		}
	}

	asked.take()
	a.spotify.SetToken(spotify.Token{})
	_, err = c.Link(ctx, link(media.LinkSong, "t1"))
	if !errors.Is(err, errSpotifyLinks) || !errors.Is(err, spotify.ErrNotSignedIn) || ErrorText(err) != locales.TopbarLinkSpotifySignIn() {
		t.Errorf("signed out: %v, which says %q", err, ErrorText(err))
	}
	if got := asked.take(); len(got) > 0 {
		t.Errorf("signed out, it asked Spotify %q", got)
	}
}

// TestSearchKeepsNoLinks keeps a search among the recent searches, but
// not a link, which a search box opens, or searches when it isn't one of
// the services' (#614).
func TestSearchKeepsNoLinks(t *testing.T) {
	a := dislikeApp(t, func(w http.ResponseWriter, r *http.Request) { w.Write(nothing) },
		func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNotFound) })
	a.spotify.SetToken(spotify.Token{})
	c := &Catalog{a}
	for _, q := range []string{"https://youtu.be/dQw4w9WgXcQ", "https://example.com/page", "Instant Crush", "spotify:track:t1"} {
		if _, err := c.Search(context.Background(), q); err != nil {
			t.Fatalf("searching %q: %v", q, err)
		}
	}
	if got := a.library.Searches(); !slices.Equal(got, []string{"Instant Crush"}) {
		t.Errorf("the recent searches = %q", got)
	}
}
