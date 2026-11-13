package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/library"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
	"github.com/fzn0x/mixmusic/internal/settings"
)

// testShow is a podcast of YouTube Music with three episodes, and more.
var testShow = media.Podcast{ID: "yt:MPSPPLshow", Source: media.SourceYouTube, Title: "Night Science",
	Authors: []media.Ref{{ID: "yt:UCsci", Name: "The Lab"}}, Description: "Science, at night.",
	PlaylistID: "PLshow", More: "T1", URL: "https://music.youtube.com/playlist?list=PLshow",
	Episodes: []media.Track{
		testEpisode("yt:ep1", "Sleep and the brain", "2026-11-02", 8280000),
		testEpisode("yt:ep2", "Why we dream", "2026-10-26", 2040000),
		testEpisode("yt:ep3", "Light at night", "2025-12-29", 3600000),
	}}

func testEpisode(id, title, day string, ms int) media.Track {
	return media.Track{ID: id, Source: media.SourceYouTube, Title: title, Artists: []media.Ref{{Name: "Night Science"}},
		DurationMs: ms, Video: true, Episode: &media.Episode{Show: media.Ref{ID: "yt:MPSPPLshow", Name: "Night Science"}, Released: day}}
}

// newPodcastShell is a test shell whose data holds where the user left the
// episodes at: ep2 34 minutes in, ep3 played.
func newPodcastShell(t *testing.T) *shell {
	t.Helper()
	return newTestShellOf(t, func(o *app.Options) {
		now := time.Now()
		ep2, ep3 := testShow.Episodes[1], testShow.Episodes[2]
		positions := map[string]library.Position{
			ep2.ID: {Track: ep2, PositionMs: 22 * 60000, DurationMs: ep2.DurationMs, Updated: now},
			ep3.ID: {Track: ep3, DurationMs: ep3.DurationMs, Played: true, Updated: now},
		}
		data, _ := json.Marshal(positions)
		if err := os.WriteFile(filepath.Join(o.DataDir, "positions.json"), data, 0o644); err != nil {
			t.Fatal(err)
		}
	})
}

// TestPodcastPage shows a show's head and its episodes, newest first, with
// their days, lengths and what is left of them, and resumes the one left
// partway.
func TestPodcastPage(t *testing.T) {
	s := newPodcastShell(t)
	s.loads["podcast:"+testShow.ID] = &loaded[media.Podcast]{done: true, value: testShow}
	tt := testerOf(t, s, 1280, 1000)
	show(s, tt, podcastPath(testShow.ID))
	year := time.Now().Year()
	for _, want := range []string{"Night Science", "The Lab", "Science, at night.", "follow", "resume", "load more episodes",
		"Sleep and the brain", "Why we dream", "Light at night", "2 hr 18 min", "12 min left", "played"} {
		if !showsText(tt.Texts(), want) {
			t.Errorf("the page doesn't show %q: %q", want, tt.Texts())
		}
	}
	if year == 2026 && !showsText(tt.Texts(), "Oct 26 · 34 min · 12 min left") {
		t.Errorf("ep2's line: %q", tt.Texts())
	}
	click(t, s, tt, "resume")
	s.settle(t, func(st player.State) bool { cur, ok := st.Current(); return ok && cur.Track.ID == "yt:ep2" })
	if st := s.p.State(); len(st.Queue) != 3 {
		t.Errorf("queued %d episodes", len(st.Queue))
	}
	click(t, s, tt, "follow")
	deadline := time.Now().Add(5 * time.Second)
	for !s.svc.Collection.Follows(testShow.ID) {
		if time.Now().After(deadline) {
			t.Fatal("not followed")
		}
		time.Sleep(5 * time.Millisecond)
		s.drain()
	}
	if p := s.svc.Collection.Podcasts(); len(p) != 1 || len(p[0].Episodes) != 0 {
		t.Errorf("follows %+v", p)
	}
}

// TestSavedPodcastFollowed says "following" on the page of a show the user
// saved on YouTube Music, signed in, followed in another profile or on
// another device, as its page or the shows saved there tell: following
// stops there too, and Library › Podcasts lists it no more. Signed out,
// the page says "follow".
func TestSavedPodcastFollowed(t *testing.T) {
	key := "podcast:" + testShow.ID
	setUp := func(signedIn, saved bool, list []media.Podcast) (*shell, *ui.Tester, *loaded[[]media.Podcast]) {
		s := newPodcastShell(t)
		s.loads["accounts"] = &loaded[app.AccountsStatus]{done: true, value: app.AccountsStatus{YouTube: app.YouTubeAccount{SignedIn: signedIn}}}
		p := testShow
		p.Saved = saved
		s.loads[key] = &loaded[media.Podcast]{done: true, value: p}
		var lib *loaded[[]media.Podcast]
		if list != nil {
			lib = &loaded[[]media.Podcast]{done: true, value: list}
			s.loads["youtube:podcasts"] = lib
		}
		tt := testerOf(t, s, 1280, 1000)
		show(s, tt, podcastPath(testShow.ID))
		return s, tt, lib
	}
	says := func(tt *ui.Tester, label string) bool { _, ok := tt.Find(label); return ok }

	s, tt, lib := setUp(true, true, nil)
	if !says(tt, "following") {
		t.Errorf("a show saved on YouTube Music: %q", tt.Texts())
	}
	if _, asked := s.loads["youtube:podcasts"]; !asked {
		t.Error("the page didn't ask for the shows saved, which a change goes into")
	}

	s, tt, lib = setUp(true, true, []media.Podcast{testShow.Summary()})
	click(t, s, tt, "following")
	s.settle(t, func(player.State) bool { return !s.keeping[key] })
	tt.Frame()
	if !says(tt, "follow") || says(tt, "following") || s.svc.Collection.Follows(testShow.ID) {
		t.Errorf("stopped following: %q", tt.Texts())
	}
	if page := s.loads[key].(*loaded[media.Podcast]); page.value.Saved || len(lib.value) != 0 {
		t.Errorf("stopped following: saved %v, the library holds %+v", page.value.Saved, lib.value)
	}
	show(s, tt, "/library/podcasts")
	if showsText(tt.Texts(), "Saved on YouTube Music") {
		t.Errorf("the library still lists it: %q", tt.Texts())
	}

	if _, tt, _ := setUp(true, false, []media.Podcast{testShow.Summary()}); !says(tt, "following") {
		t.Errorf("a show the saved shows list: %q", tt.Texts())
	}
	if _, tt, _ := setUp(false, true, []media.Podcast{testShow.Summary()}); !says(tt, "follow") || says(tt, "following") {
		t.Errorf("signed out: %q", tt.Texts())
	}

	s, tt, lib = setUp(true, false, []media.Podcast{})
	click(t, s, tt, "follow")
	s.settle(t, func(player.State) bool { return !s.keeping[key] })
	tt.Frame()
	if page := s.loads[key].(*loaded[media.Podcast]); !says(tt, "following") || !page.value.Saved ||
		len(lib.value) != 1 || lib.value[0].ID != testShow.ID || len(lib.value[0].Episodes) != 0 {
		t.Errorf("followed: %q, saved %v, the library holds %+v", tt.Texts(), page.value.Saved, lib.value)
	}
}

// TestSavedSpotifyPodcastFollowed says "following" on the page of a show
// saved on Spotify, signed in there, as the shows saved there tell.
func TestSavedSpotifyPodcastFollowed(t *testing.T) {
	s := newTestShell(t)
	s.loads["accounts"] = &loaded[app.AccountsStatus]{done: true, value: app.AccountsStatus{Spotify: app.SpotifyAccount{SignedIn: true}}}
	sp := media.Podcast{ID: "sp:s1", Source: media.SourceSpotify, Title: "The Daily Thing", Authors: []media.Ref{}, Episodes: []media.Track{}}
	s.loads["podcast:sp:s1"] = &loaded[media.Podcast]{done: true, value: sp}
	s.loads["spotify:podcasts"] = &loaded[[]media.Podcast]{done: true, value: []media.Podcast{sp}}
	tt := testerOf(t, s, 1280, 1000)
	show(s, tt, podcastPath("sp:s1"))
	if _, ok := tt.Find("following"); !ok {
		t.Errorf("a show saved on Spotify: %q", tt.Texts())
	}
}

// showsText reports whether a text of the window holds sub, in any case:
// the layouts write the words of the pages in their own.
func showsText(texts []string, sub string) bool {
	return slices.ContainsFunc(texts, func(s string) bool { return strings.Contains(strings.ToLower(s), strings.ToLower(sub)) })
}

// TestEpisodeMenu offers an episode's show and its played mark, and no
// radio, nor a download.
func TestEpisodeMenu(t *testing.T) {
	s := newPodcastShell(t)
	s.loads["podcast:"+testShow.ID] = &loaded[media.Podcast]{done: true, value: testShow}
	tt := testerOf(t, s, 1280, 1000)
	show(s, tt, podcastPath(testShow.ID))
	if err := tt.RightClick("Sleep and the brain"); err != nil {
		t.Fatal(err)
	}
	menu := tt.Menu()
	for _, want := range []string{"Go to podcast", "Mark as played"} {
		if !slices.Contains(menu, want) {
			t.Errorf("the menu lacks %q: %q", want, menu)
		}
	}
	for _, not := range []string{"Start radio", "Download"} {
		if slices.Contains(menu, not) {
			t.Errorf("the menu holds %q", not)
		}
	}
	if err := tt.ChooseMenuItem("Mark as played"); err != nil {
		t.Fatal(err)
	}
	s.drain()
	if pos, ok := s.svc.Collection.Position("yt:ep1"); !ok || !pos.Played {
		t.Errorf("ep1 = %+v, %v", pos, ok)
	}
}

// TestPodcastSearch lists the podcasts and the episodes a search found
// under its podcasts pivot, and counts them.
func TestPodcastSearch(t *testing.T) {
	s := newPodcastShell(t)
	s.loads["search:night"] = &loaded[app.SearchResponse]{done: true, value: app.SearchResponse{
		NetEase: media.SearchResults{Tracks: []media.Track{}, Albums: []media.Album{}, Artists: []media.Artist{}, Playlists: []media.Playlist{}},
		Spotify: media.SearchResults{Tracks: []media.Track{}, Albums: []media.Album{}, Artists: []media.Artist{}, Playlists: []media.Playlist{}}}}
	s.loads["search-podcasts:night"] = &loaded[app.PodcastSearch]{done: true, value: app.PodcastSearch{
		YouTube: media.PodcastResults{Podcasts: []media.Podcast{testShow.Summary()}, Episodes: testShow.Episodes[:2]},
		Spotify: media.EmptyPodcastResults()}}
	s.searchPivot = slices.Index(searchPivots, "podcasts")
	tt := testerOf(t, s, 1280, 1000)
	show(s, tt, searchPath("night"))
	for _, want := range []string{"Night Science", "Episodes", "Sleep and the brain", "Why we dream"} {
		if !showsText(tt.Texts(), want) {
			t.Errorf("the search doesn't show %q: %q", want, tt.Texts())
		}
	}
	if got := s.podcastCount("night"); got != 3 {
		t.Errorf("the pivot counts %d", got)
	}
	if err := tt.Click("Night Science"); err != nil {
		t.Fatal(err)
	}
	s.drain()
	if s.router.Path() != podcastPath(testShow.ID) {
		t.Errorf("opened %s", s.router.Path())
	}
}

// TestPodcastsLibrary keeps on listening to the episodes left partway, and
// shows the podcasts followed, or invites to follow one.
func TestPodcastsLibrary(t *testing.T) {
	s := newPodcastShell(t)
	tt := testerOf(t, s, 1280, 1000)
	show(s, tt, "/library/podcasts")
	for _, want := range []string{"Keep listening", "Why we dream", "nothing followed yet"} {
		if !showsText(tt.Texts(), want) {
			t.Errorf("the library doesn't show %q: %q", want, tt.Texts())
		}
	}
	if tt.HasText("Light at night") {
		t.Error("a played episode keeps listening")
	}
	if err := s.svc.Collection.SetPodcastFollowed(t.Context(), testShow, true); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !showsText(tt.Texts(), "Following") || !tt.HasText("Night Science") {
		t.Errorf("the library shows %q", tt.Texts())
	}
}

// TestSpotifyPodcastFromYouTube says, on a Spotify show's page, that its
// episodes don't play from YouTube Music.
func TestSpotifyPodcastFromYouTube(t *testing.T) {
	s := newTestShell(t)
	if _, err := s.svc.Preferences.Update(func(st *settings.Settings) { st.SpotifyPlayback = settings.SpotifyPlaybackMatch }); err != nil {
		t.Fatal(err)
	}
	sp := media.Podcast{ID: "sp:s1", Source: media.SourceSpotify, Title: "The Daily Thing", Authors: []media.Ref{},
		Episodes: []media.Track{{ID: "sp:e1", Source: media.SourceSpotify, Title: "Monday", Episode: &media.Episode{Released: "2026-11"}}}}
	s.loads["podcast:sp:s1"] = &loaded[media.Podcast]{done: true, value: sp}
	tt := testerOf(t, s, 1280, 1000)
	show(s, tt, podcastPath("sp:s1"))
	if !showsText(tt.Texts(), "Settings › Playback") || !tt.HasText("Monday") || !showsText(tt.Texts(), "Nov 2026") {
		t.Errorf("the page shows %q", tt.Texts())
	}
}

func TestReleasedLabel(t *testing.T) {
	now := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	for in, want := range map[string]string{"2026-10-26": "Oct 26", "2025-12-29": "Dec 29, 2025", "2026-10": "Oct 2026", "2019": "2019", "": ""} {
		if got := releasedLabel(in, now); got != want {
			t.Errorf("releasedLabel(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestRenderPodcasts draws a show's page, the search's podcasts and
// Library › Podcasts to MIXMUSIC_RENDER, for a look.
func TestRenderPodcasts(t *testing.T) {
	dir := os.Getenv("MIXMUSIC_RENDER")
	if dir == "" {
		t.Skip("MIXMUSIC_RENDER names no folder")
	}
	s := newPodcastShell(t)
	s.loads["podcast:"+testShow.ID] = &loaded[media.Podcast]{done: true, value: testShow}
	s.loads["search:night"] = &loaded[app.SearchResponse]{done: true}
	s.loads["search-podcasts:night"] = &loaded[app.PodcastSearch]{done: true, value: app.PodcastSearch{
		YouTube: media.PodcastResults{Podcasts: []media.Podcast{testShow.Summary()}, Episodes: testShow.Episodes},
		Spotify: media.EmptyPodcastResults()}}
	tt := testerOf(t, s, 1280, 820)
	show(s, tt, podcastPath(testShow.ID))
	renderTo(t, s, tt, dir, "podcast")
	s.p.PlayTracks(testShow.Episodes, 1)
	s.settle(t, func(st player.State) bool { return st.Playing })
	renderTo(t, s, tt, dir, "podcast-playing")
	s.searchPivot = slices.Index(searchPivots, "podcasts")
	show(s, tt, searchPath("night"))
	renderTo(t, s, tt, dir, "search-podcasts")
	if err := s.svc.Collection.SetPodcastFollowed(t.Context(), testShow, true); err != nil {
		t.Fatal(err)
	}
	show(s, tt, "/library/podcasts")
	renderTo(t, s, tt, dir, "library-podcasts")
}
