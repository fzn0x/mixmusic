package main

import (
	"slices"
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
)

// TestArtistTilePlaysAllSongs gives an artist's tile the circle of
// albums and playlists with the shuffle icon, which shuffles all the
// artist's songs, and Play and Shuffle in its menu. Clicking the tile
// itself still opens the artist.
func TestArtistTilePlaysAllSongs(t *testing.T) {
	s := newTestShell(t)
	type played struct {
		id       string
		shuffled bool
	}
	var plays []played
	was := playArtistID
	playArtistID = func(_ *shell, id string, shuffled bool) { plays = append(plays, played{id, shuffled}) }
	t.Cleanup(func() { playArtistID = was })
	artist := media.Artist{ID: "yt:UCtile", Source: media.SourceYouTube, Name: "Harbour Lights", URL: "https://music.youtube.com/channel/UCtile"}
	s.loads["search:harbour"] = &loaded[app.SearchResponse]{done: true, value: app.SearchResponse{
		YouTube: media.SearchResults{Artists: []media.Artist{artist}}}}
	s.loads["artist:"+artist.ID] = &loaded[media.Artist]{done: true, value: artist}
	tt := testerOf(t, s, 1280, 1400)
	show(s, tt, searchPath("harbour"))
	start := s.router.Path()

	// The circle rises as the pointer comes.
	r, ok := tt.Find(artist.Name)
	if !ok {
		t.Fatalf("the tile isn't shown: %q", tt.Texts())
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Advance(time.Second)
	tt.Frame()
	if err := tt.Click("Shuffle " + artist.Name); err != nil {
		t.Fatalf("the tile's circle: %v; shown %q", err, tt.Texts())
	}
	s.drain()
	tt.Frame()
	if tt.HasText("Play " + artist.Name) {
		t.Error("the circle plays, it should shuffle")
	}

	for _, item := range []string{"Play", "Shuffle"} {
		if err := tt.RightClick(artist.Name); err != nil {
			t.Fatal(err)
		}
		if err := tt.ChooseMenuItem(item); err != nil {
			t.Fatalf("%v: the menu holds %q", err, tt.Menu())
		}
		s.drain()
		tt.Frame()
	}
	if want := []played{{artist.ID, true}, {artist.ID, false}, {artist.ID, true}}; !slices.Equal(plays, want) {
		t.Errorf("played %+v, want %+v", plays, want)
	}
	if got := s.router.Path(); got != start {
		t.Errorf("playing opened %s", got)
	}
	if err := tt.RightClick(artist.Name); err != nil {
		t.Fatal(err)
	}
	for _, item := range []string{"Open", "Follow", "Copy link"} {
		if !slices.Contains(tt.Menu(), item) {
			t.Errorf("the menu holds %q, not %q", tt.Menu(), item)
		}
	}
}

// TestSpotifyArtistPageShuffles gives a Spotify artist with releases
// Play and Shuffle of all their songs on their page, by the path that
// serves YouTube Music too.
func TestSpotifyArtistPageShuffles(t *testing.T) {
	s := newTestShell(t)
	type played struct {
		id       string
		shuffled bool
	}
	var plays []played
	was := playAllSongs
	playAllSongs = func(_ *shell, a media.Artist, shuffled bool) { plays = append(plays, played{a.ID, shuffled}) }
	t.Cleanup(func() { playAllSongs = was })
	a := media.Artist{ID: "sp:a1", Source: media.SourceSpotify, Name: "Dunes", TopTracks: testTracks[:2],
		Albums: []media.Album{{ID: "sp:al1", Source: media.SourceSpotify, Title: "One"}}}
	s.loads["artist:"+a.ID] = &loaded[media.Artist]{done: true, value: a}
	tt := tester(t, s)
	show(s, tt, artistPath(a.ID))
	if tt.HasText("play top songs") || !tt.HasText("shuffle") {
		t.Fatalf("the page shows %q", tt.Texts())
	}
	click(t, s, tt, "play")
	click(t, s, tt, "shuffle")
	if want := []played{{a.ID, false}, {a.ID, true}}; !slices.Equal(plays, want) {
		t.Errorf("played %+v, want %+v", plays, want)
	}
}
