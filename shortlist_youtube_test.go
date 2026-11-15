package main

import (
	"errors"
	"slices"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// firstOf is the error of a list of YouTube Music's that came in part:
// the first got items, of a count YouTube Music doesn't tell.
func firstOf(got int) error {
	return &media.ShortList{Got: got, Err: errors.New("youtube music: HTTP 503")}
}

// lateNight is a playlist of the user's on YouTube Music.
var lateNight = media.Playlist{ID: "yt:PLlate", Source: media.SourceYouTube, Title: "Late night", Editable: true, Mine: true,
	Tracks: []media.Track{}}

// youtubeLibraryShell is a window signed in to YouTube Music whose library
// holds playlists and songs, each list as its load says.
func youtubeLibraryShell(t *testing.T, playlists *loaded[[]media.Playlist], songs *loaded[[]media.Track]) *shell {
	t.Helper()
	s := newTestShell(t)
	s.loads["accounts"] = signedInTo(true, false)
	s.loads["youtube:playlists"] = playlists
	s.loads["youtube:albums"] = &loaded[[]media.Album]{done: true, value: []media.Album{}}
	s.loads["youtube:artists"] = &loaded[[]media.Artist]{done: true}
	s.loads["youtube:liked"] = &loaded[media.Playlist]{done: true, value: media.Playlist{ID: "yt:LM", Tracks: []media.Track{testTracks[5]}}}
	s.loads[youtubeSongsKey] = songs
	return s
}

// TestYouTubeLibraryInPart shows in Library › YouTube Music the playlists
// and songs of lists that came in part, says how many came, and asks for a
// list again on its Try again; the new playlist link stays. A list cut at
// the most MixMusic reads says so, with no Try again.
func TestYouTubeLibraryInPart(t *testing.T) {
	listFor(t, func() ([]media.Playlist, error) { return []media.Playlist{lateNight}, firstOf(1) })
	playlists := &loaded[[]media.Playlist]{done: true, value: []media.Playlist{lateNight}, err: firstOf(1)}
	songs := &loaded[[]media.Track]{done: true, value: []media.Track{testTracks[2]}}
	s := youtubeLibraryShell(t, playlists, songs)
	tt := testerOf(t, s, 1280, 1400)
	show(s, tt, "/library/youtube")
	if !tt.HasText("Showing the first 1 playlists") || !tt.HasText(lateNight.Title) || !tt.HasText("new playlist") ||
		tt.HasText(playlists.err.Error()) {
		t.Fatalf("the library shows %q", tt.Texts())
	}
	click(t, s, tt, "try again")
	if playlists.asks != 1 {
		t.Errorf("Try again asked %d times", playlists.asks)
	}

	s.loads["youtube:playlists"] = &loaded[[]media.Playlist]{done: true, value: []media.Playlist{lateNight}}
	songs.err = firstOf(1)
	tt.Frame()
	if !tt.HasText("Showing the first 1 songs") || !tt.HasText(testTracks[2].Title) || tt.HasText("Showing the first 1 playlists") {
		t.Fatalf("the songs show %q", tt.Texts())
	}
	click(t, s, tt, "try again")
	if songs.asks != 1 {
		t.Errorf("Try again asked for the songs %d times", songs.asks)
	}

	s.loads[youtubeSongsKey] = &loaded[[]media.Track]{done: true, value: []media.Track{testTracks[2]},
		err: &media.ShortList{Got: 24013, Cut: true, Err: errors.New("youtube music: MixMusic reads no more pages of a list")}}
	tt.Frame()
	if !tt.HasText("Showing the first 24,013 songs, as far as MixMusic reads a list") || tt.HasText("try again") {
		t.Errorf("a list cut shows %q", tt.Texts())
	}
}

// TestYouTubeArtistsAndPodcastsInPart shows in Library › Artists and
// Podcasts the artists and shows of YouTube Music's lists that came in
// part, and says how many came.
func TestYouTubeArtistsAndPodcastsInPart(t *testing.T) {
	s := newTestShell(t)
	s.loads["accounts"] = signedInTo(true, false)
	s.loads["youtube:artists"] = &loaded[[]media.Artist]{done: true, err: firstOf(1),
		value: []media.Artist{{ID: "yt:UCharbour", Source: media.SourceYouTube, Name: "Harbour"}}}
	s.loads["youtube:podcasts"] = &loaded[[]media.Podcast]{done: true, err: firstOf(1),
		value: []media.Podcast{{ID: "yt:MPSPshow", Source: media.SourceYouTube, Title: "The Daily Thing", Authors: []media.Ref{}, Episodes: []media.Track{}}}}
	tt := testerOf(t, s, 1280, 1000)
	show(s, tt, "/library/artists")
	if !tt.HasText("Showing the first 1 artists") || !tt.HasText("Harbour") {
		t.Errorf("Library › Artists shows %q", tt.Texts())
	}
	show(s, tt, "/library/podcasts")
	if !tt.HasText("Showing the first 1 podcasts") || !tt.HasText("The Daily Thing") {
		t.Errorf("Library › Podcasts shows %q", tt.Texts())
	}
}

// TestAddToYouTubePlaylistInPart offers in a track's Add to YouTube Music
// playlist the playlists of a list that came in part, then how many came,
// which asks for the list again.
func TestAddToYouTubePlaylistInPart(t *testing.T) {
	listFor(t, func() ([]media.Playlist, error) { return []media.Playlist{lateNight}, firstOf(1) })
	s := newTestShell(t)
	s.loads["accounts"] = signedInTo(true, false)
	l := &loaded[[]media.Playlist]{done: true, value: []media.Playlist{lateNight}, err: firstOf(1)}
	s.loads["youtube:playlists"] = l
	s.loads["album:al-x"] = &loaded[media.Album]{done: true, value: media.Album{ID: "al-x", Title: "Mezzanine", Tracks: testTracks[:2]}}
	tt := tester(t, s)
	show(s, tt, "/album/al-x")
	if err := tt.RightClick(testTracks[0].Title); err != nil {
		t.Fatal(err)
	}
	menu := tt.Menu()
	tt.CloseMenu()
	if !slices.Contains(menu, "Add to YouTube Music playlist") {
		t.Fatalf("the menu holds %q", menu)
	}
	if err := trackMenuChoice(t, tt, "Add to YouTube Music playlist", "Showing the first 1 playlists · Try again"); err != nil {
		t.Fatal(err)
	}
	s.drain()
	tt.Frame()
	if l.asks != 1 {
		t.Errorf("Try again asked %d times", l.asks)
	}
}
