package main

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// testArtist is an artist of YouTube Music, whose page tells that the
// user doesn't follow them there.
var testArtist = media.Artist{ID: "yt:UCharbour", Source: media.SourceYouTube, Name: "Harbour Lights",
	Subtitle: "1.2M subscribers", URL: "https://music.youtube.com/channel/UCharbour", TopTracks: testTracks[:2]}

// serviceFollows answers for the services' follows, as followOnService,
// and records them; err fails them.
type serviceFollows struct {
	mu    sync.Mutex
	calls []string
	err   error
}

func (f *serviceFollows) follow(_ *shell, _ context.Context, id string, on bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if on {
		f.calls = append(f.calls, "follow "+id)
	} else {
		f.calls = append(f.calls, "unfollow "+id)
	}
	return f.err
}

func (f *serviceFollows) made() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

// newFollowShell is a test shell signed in to YouTube Music, or not, that
// shows testArtist's page, knows no releases, and whose service follows f
// answers for.
func newFollowShell(t *testing.T, signedIn bool) (*shell, *ui.Tester, *serviceFollows) {
	t.Helper()
	f := &serviceFollows{}
	was := followOnService
	followOnService = f.follow
	t.Cleanup(func() { followOnService = was })
	s := newTestShell(t)
	s.loads["accounts"] = &loaded[app.AccountsStatus]{done: true, value: app.AccountsStatus{YouTube: app.YouTubeAccount{SignedIn: signedIn}}}
	s.loads["artist:"+testArtist.ID] = &loaded[media.Artist]{done: true, value: testArtist}
	s.loads["releases"] = &loaded[[]media.Release]{done: true}
	tt := testerOf(t, s, 1280, 1000)
	show(s, tt, artistPath(testArtist.ID))
	return s, tt, f
}

// headSays reports whether the artist page's follow button says label.
func headSays(tt *ui.Tester, label string) bool { _, ok := tt.Find(label); return ok }

// moreMenu opens the artist page's More menu, and returns its items.
func moreMenu(t *testing.T, tt *ui.Tester) []string {
	t.Helper()
	if err := tt.Click("More"); err != nil {
		t.Fatal(err)
	}
	return tt.Menu()
}

// TestFollowArtistSignedOut follows an artist in MixMusic, signed out of
// their service, from their page's button and its More menu: nothing
// reaches the service, and Library › Artists lists them.
func TestFollowArtistSignedOut(t *testing.T) {
	s, tt, f := newFollowShell(t, false)
	if !headSays(tt, "follow") || headSays(tt, "following") {
		t.Fatalf("signed out, the page shows %q", tt.Texts())
	}
	click(t, s, tt, "follow")
	tt.Frame()
	if !s.svc.Collection.FollowsArtist(testArtist.ID) || !headSays(tt, "following") {
		t.Fatalf("following: %q", tt.Texts())
	}
	if !tt.HasText("Harbour Lights: following in MixMusic") {
		t.Errorf("no toast: %q", tt.Texts())
	}
	if l := s.loads["releases"].(*loaded[[]media.Release]); !l.again {
		t.Error("the new releases aren't asked again for the artist followed")
	}
	if got := s.svc.Collection.FollowedArtists(); len(got) != 1 || got[0].Name != "Harbour Lights" || got[0].Subtitle != "" {
		t.Errorf("followed %+v", got)
	}
	if menu := moreMenu(t, tt); len(menu) == 0 || menu[0] != "Unfollow" {
		t.Errorf("the More menu holds %q", menu)
	}
	if err := tt.ChooseMenuItem("Unfollow"); err != nil {
		t.Fatal(err)
	}
	s.drain()
	tt.Frame()
	if s.svc.Collection.FollowsArtist(testArtist.ID) || !headSays(tt, "follow") || headSays(tt, "following") {
		t.Errorf("unfollowed from the menu: %q", tt.Texts())
	}
	if menu := moreMenu(t, tt); len(menu) == 0 || menu[0] != "Follow" {
		t.Errorf("the More menu holds %q", menu)
	}
	if err := tt.ChooseMenuItem("Follow"); err != nil {
		t.Fatal(err)
	}
	s.drain()
	show(s, tt, "/library/artists")
	if !showsText(tt.Texts(), "Followed in MixMusic") || !tt.HasText("Harbour Lights") {
		t.Errorf("Library › Artists shows %q", tt.Texts())
	}
	if calls := f.made(); len(calls) != 0 {
		t.Errorf("signed out, the service was asked to %q", calls)
	}
}

// TestFollowArtistSignedIn follows an artist on their service, signed in
// there, at once on the page, which goes back if the service refuses;
// Library › Artists lists them under the service, and MixMusic keeps
// nothing of it.
func TestFollowArtistSignedIn(t *testing.T) {
	s, tt, f := newFollowShell(t, true)
	list := &loaded[[]media.Artist]{done: true, value: []media.Artist{}}
	s.loads["youtube:artists"] = list
	click(t, s, tt, "follow")
	eventually(t, s, "the follow on YouTube Music", func() bool { return len(f.made()) == 1 })
	tt.Frame()
	if calls := f.made(); calls[0] != "follow "+testArtist.ID || !headSays(tt, "following") ||
		s.svc.Collection.FollowsArtist(testArtist.ID) || len(list.value) != 1 || list.value[0].ID != testArtist.ID {
		t.Fatalf("followed %q: the page shows %q, the list %+v", calls, tt.Texts(), list.value)
	}
	if !tt.HasText("Harbour Lights: following on YouTube Music") {
		t.Errorf("no toast: %q", tt.Texts())
	}
	show(s, tt, "/library/artists")
	if !showsText(tt.Texts(), "Followed on YouTube Music") || !tt.HasText("Harbour Lights") || showsText(tt.Texts(), "Followed in MixMusic") {
		t.Errorf("Library › Artists shows %q", tt.Texts())
	}

	// Refused, the page says so, and goes back.
	show(s, tt, artistPath(testArtist.ID))
	f.err = errors.New("YouTube Music refused")
	if menu := moreMenu(t, tt); len(menu) == 0 || menu[0] != "Unfollow" {
		t.Fatalf("the More menu holds %q", menu)
	}
	if err := tt.ChooseMenuItem("Unfollow"); err != nil {
		t.Fatal(err)
	}
	eventually(t, s, "the unfollow on YouTube Music", func() bool { return len(f.made()) == 2 })
	s.settle(t, func(player.State) bool { return !s.keeping["artist:"+testArtist.ID] })
	tt.Frame()
	if !headSays(tt, "following") || !tt.HasText("YouTube Music refused") || len(list.value) != 1 {
		t.Errorf("refused: %q, the list %+v", tt.Texts(), list.value)
	}
}

// TestLocalFollowSignedIn keeps an artist followed in MixMusic before the
// user signed in to their service there: the page says following, and
// unfollowing touches the service only where they follow there too.
func TestLocalFollowSignedIn(t *testing.T) {
	s, tt, f := newFollowShell(t, true)
	if err := s.svc.Collection.SetArtistFollowed(testArtist, true); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !headSays(tt, "following") {
		t.Fatalf("the page shows %q", tt.Texts())
	}
	click(t, s, tt, "following")
	tt.Frame()
	if s.svc.Collection.FollowsArtist(testArtist.ID) || !headSays(tt, "follow") || len(f.made()) != 0 {
		t.Errorf("unfollowed: %q, asked the service to %q", tt.Texts(), f.made())
	}

	// Followed in both, both stop.
	if err := s.svc.Collection.SetArtistFollowed(testArtist, true); err != nil {
		t.Fatal(err)
	}
	s.loads["artist:"+testArtist.ID].(*loaded[media.Artist]).value.Following = true
	tt.Frame()
	click(t, s, tt, "following")
	eventually(t, s, "the unfollow on YouTube Music", func() bool { return len(f.made()) == 1 })
	tt.Frame()
	if s.svc.Collection.FollowsArtist(testArtist.ID) || f.made()[0] != "unfollow "+testArtist.ID || !headSays(tt, "follow") {
		t.Errorf("unfollowed in both: %q, asked the service to %q", tt.Texts(), f.made())
	}
}

// TestFollowArtistFromTile follows an artist from the menu of their tile,
// as search finds it, and stops.
func TestFollowArtistFromTile(t *testing.T) {
	s, tt, _ := newFollowShell(t, false)
	s.loads["search:harbour"] = &loaded[app.SearchResponse]{done: true, value: app.SearchResponse{NetEase: media.SearchResults{
		Artists: []media.Artist{{ID: testArtist.ID, Source: media.SourceYouTube, Name: testArtist.Name}}}}}
	show(s, tt, searchPath("harbour"))
	for _, want := range []struct{ item, other string }{{"Follow", "Unfollow"}, {"Unfollow", "Follow"}} {
		if err := tt.RightClick(testArtist.Name); err != nil {
			t.Fatal(err)
		}
		if menu := tt.Menu(); !slices.Contains(menu, want.item) || slices.Contains(menu, want.other) {
			t.Fatalf("the tile's menu holds %q", menu)
		}
		if err := tt.ChooseMenuItem(want.item); err != nil {
			t.Fatal(err)
		}
		s.drain()
		tt.Frame()
		if follows := s.svc.Collection.FollowsArtist(testArtist.ID); follows != (want.item == "Follow") {
			t.Errorf("after %s, following is %v", want.item, follows)
		}
	}
}

// TestNewReleasesShelf shows the new releases of the artists followed on
// Home, those of the service the pivot chooses, and in Library › Artists,
// each with the day it came out or its year; before this run's look, as
// the last run's found them. Unfollowing drops theirs at once.
func TestNewReleasesShelf(t *testing.T) {
	s, tt, _ := newFollowShell(t, false)
	if err := s.svc.Collection.SetArtistFollowed(testArtist, true); err != nil {
		t.Fatal(err)
	}
	dunes := media.Ref{ID: "sp:dunes", Name: "Dunes"}
	harbour := media.Ref{ID: testArtist.ID, Name: testArtist.Name}
	releases := []media.Release{
		{Album: media.Album{ID: "sp:week", Source: media.SourceSpotify, Title: "Low Tide", Kind: media.AlbumKindSingle, Artists: []media.Ref{dunes}},
			Released: "2026-11-01", Of: dunes},
		{Album: media.Album{ID: "ne:1", Source: media.SourceNetEase, Title: "Pier at Night", Kind: media.AlbumKindAlbum, Artists: []media.Ref{harbour}},
			Released: "2026", Of: harbour},
	}
	s.loads["releases"] = &loaded[[]media.Release]{}
	s.loads["releases:known"] = &loaded[[]media.Release]{done: true, value: releases}
	show(s, tt, "/")
	if !showsText(tt.Texts(), "New from artists you follow") || !tt.HasText("Low Tide") || !tt.HasText("Pier at Night") {
		t.Fatalf("Home shows %q", tt.Texts())
	}
	if !showsText(tt.Texts(), "single · Nov 1") && !showsText(tt.Texts(), "single · Nov 1, 2026") {
		t.Errorf("the day Low Tide came out: %q", tt.Texts())
	}
	if !showsText(tt.Texts(), "2026 · Harbour Lights") {
		t.Errorf("the year Pier at Night came out: %q", tt.Texts())
	}
	s.homeFilter = slices.IndexFunc(homeFilters, func(f struct {
		label  string
		source media.Source
	}) bool {
		return f.source == media.SourceNetEase
	})
	tt.Frame()
	if tt.HasText("Low Tide") || !tt.HasText("Pier at Night") {
		t.Errorf("Home › 网易云音乐 shows %q", tt.Texts())
	}
	s.homeFilter = 0
	// This run's look found one.
	s.loads["releases"] = &loaded[[]media.Release]{done: true, value: releases[1:]}
	show(s, tt, "/library/artists")
	if !showsText(tt.Texts(), "New from artists you follow") || tt.HasText("Low Tide") || !tt.HasText("Pier at Night") {
		t.Errorf("Library › Artists shows %q", tt.Texts())
	}
	show(s, tt, artistPath(testArtist.ID))
	click(t, s, tt, "following")
	show(s, tt, "/")
	if showsText(tt.Texts(), "New from artists you follow") || tt.HasText("Pier at Night") {
		t.Errorf("unfollowed, Home shows %q", tt.Texts())
	}
}
