package main

import (
	"slices"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// TestArtistPlaysAllSongs gives a YouTube Music artist whose page links
// the list of all their songs Play and Shuffle of it, and Show all beside
// their Top songs, which opens it. One without plays their top songs, and
// has neither.
func TestArtistPlaysAllSongs(t *testing.T) {
	s := newTestShell(t)
	type played struct {
		id       string
		shuffled bool
	}
	var plays []played
	was := playAllSongs
	playAllSongs = func(_ *shell, a media.Artist, shuffled bool) { plays = append(plays, played{a.ID, shuffled}) }
	t.Cleanup(func() { playAllSongs = was })
	all := media.Artist{ID: "yt:UCall", Source: media.SourceYouTube, Name: "Harbour Lights", TopTracks: testTracks[:2],
		SongsID: "yt:OLAK5uy_all"}
	s.loads["artist:"+all.ID] = &loaded[media.Artist]{done: true, value: all}
	s.loads["playlist:"+all.SongsID] = &loaded[media.Playlist]{done: true, value: media.Playlist{ID: all.SongsID,
		Source: media.SourceYouTube, Title: "Top songs", Owner: all.Name, Tracks: testTracks[:3], TrackCount: 3}}
	top := all
	top.ID, top.SongsID = "yt:UCtop", ""
	s.loads["artist:"+top.ID] = &loaded[media.Artist]{done: true, value: top}
	tt := tester(t, s)

	show(s, tt, artistPath(top.ID))
	if !tt.HasText("play top songs") || tt.HasText("shuffle") || tt.HasText("show all") {
		t.Errorf("an artist without the list: %q", tt.Texts())
	}

	show(s, tt, artistPath(all.ID))
	if tt.HasText("play top songs") || !tt.HasText("shuffle") || !tt.HasText("show all") {
		t.Fatalf("an artist with the list: %q", tt.Texts())
	}
	click(t, s, tt, "play")
	click(t, s, tt, "shuffle")
	if want := []played{{all.ID, false}, {all.ID, true}}; !slices.Equal(plays, want) {
		t.Errorf("played %+v, want %+v", plays, want)
	}
	click(t, s, tt, "show all")
	if got := s.router.Path(); got != playlistPath(all.SongsID) {
		t.Errorf("Show all opened %s", got)
	}
}

// TestArtistBarShuffles shows Shuffle in the top bar of an artist whose
// page links the list of all their songs, once the head scrolled away.
func TestArtistBarShuffles(t *testing.T) {
	s := newTestShell(t)
	var albums []media.Album
	for i := range 4 {
		albums = append(albums, media.Album{ID: "al-" + count(i), Source: media.SourceYouTube, Title: "Album " + count(i)})
	}
	a := media.Artist{ID: "yt:UCall", Source: media.SourceYouTube, Name: "Harbour Lights", TopTracks: testTracks[:5],
		Albums: albums, Singles: albums, SongsID: "yt:OLAK5uy_all"}
	s.loads["artist:"+a.ID] = &loaded[media.Artist]{done: true, value: a}
	tt := testerOf(t, s, 1280, 600)
	show(s, tt, artistPath(a.ID))
	scrollBy(tt, 1500)
	if !topBarHas(tt, "Play "+a.Name) || !topBarHas(tt, "Shuffle "+a.Name) {
		t.Errorf("scrolled, the top bar shows %q", tt.Texts())
	}
}

// TestComputerArtistPlaysAllSongs plays and shuffles all the songs of an
// artist of the music on this computer, their top songs.
func TestComputerArtistPlaysAllSongs(t *testing.T) {
	s := newTestShell(t)
	a := media.Artist{ID: "file:artist:ada", Source: media.SourceFile, Name: "Ada Field", TopTracks: testTracks[:3]}
	s.loads["artist:"+a.ID] = &loaded[media.Artist]{done: true, value: a}
	tt := tester(t, s)
	show(s, tt, artistPath(a.ID))
	click(t, s, tt, "play")
	s.settle(t, func(st player.State) bool {
		cur, ok := st.Current()
		return ok && len(st.Queue) == 3 && !st.Shuffle && cur.Track.ID == testTracks[0].ID
	})
	click(t, s, tt, "shuffle")
	s.settle(t, func(st player.State) bool { return len(st.Queue) == 3 && st.Shuffle })
}
