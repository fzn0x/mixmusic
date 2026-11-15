package main

import (
	"errors"
	"testing"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
)

// partOf is the error of a list that came in part, got of total items.
func partOf(got, total int) error {
	return &media.ShortList{Got: got, Total: total, Err: errors.New("spotify: HTTP 503")}
}

// TestSpotifyLibraryInPart shows the playlists of a list that came in part
// in Library › Spotify, says how many of how many, and asks for the list
// again on Try again; a list that stopped at the most Spotify gives says
// so, with no Try again.
func TestSpotifyLibraryInPart(t *testing.T) {
	s := newTestShell(t)
	s.loads["accounts"] = signedInTo(false, true)
	playlists := &loaded[[]media.Playlist]{done: true, value: []media.Playlist{roadList}, err: partOf(1, 120)}
	s.loads["spotify:playlists"] = playlists
	s.loads["spotify:albums"] = &loaded[[]media.Album]{done: true, value: []media.Album{}}
	s.loads["spotify:artists"] = &loaded[[]media.Artist]{done: true}
	s.loads["spotify:liked"] = &loaded[app.TrackPage]{done: true}
	tt := testerOf(t, s, 1280, 1400)
	show(s, tt, "/library/spotify")
	if !tt.HasText("Showing 1 of 120 playlists") || !tt.HasText(roadList.Title) || tt.HasText(playlists.err.Error()) {
		t.Fatalf("the library shows %q", tt.Texts())
	}
	click(t, s, tt, "try again")
	if playlists.asks != 1 || playlists.again {
		t.Errorf("Try again asked %d times", playlists.asks)
	}

	s.loads["spotify:playlists"] = &loaded[[]media.Playlist]{done: true, value: []media.Playlist{roadList},
		err: &media.ShortList{Got: 1, Total: 120, Cut: true, Err: errors.New("spotify gives apps no more of a list")}}
	tt.Frame()
	if !tt.HasText("Showing 1 of 120 playlists, the most Spotify gives apps") || tt.HasText("try again") {
		t.Errorf("a list cut shows %q", tt.Texts())
	}
}

// TestArtistLibraryInPart shows in Library › Artists the artists followed
// on Spotify of a list that came in part, and says how many of how many.
func TestArtistLibraryInPart(t *testing.T) {
	s := newTestShell(t)
	s.loads["accounts"] = signedInTo(false, true)
	artists := &loaded[[]media.Artist]{done: true, err: partOf(1, 120),
		value: []media.Artist{{ID: "sp:dunes", Source: media.SourceSpotify, Name: "Dunes"}}}
	s.loads["spotify:artists"] = artists
	tt := tester(t, s)
	show(s, tt, "/library/artists")
	if !tt.HasText("Showing 1 of 120 artists") || !tt.HasText("Dunes") || tt.HasText(artists.err.Error()) {
		t.Fatalf("Library › Artists shows %q", tt.Texts())
	}
	click(t, s, tt, "try again")
	if artists.asks != 1 {
		t.Errorf("Try again asked %d times", artists.asks)
	}
}

// TestAddToSpotifyPlaylistInPart offers in a track's Add to Spotify
// playlist the playlists of a list that came in part, then how many of
// how many came, which asks for the list again.
func TestAddToSpotifyPlaylistInPart(t *testing.T) {
	answerCollab(t)
	s, tt := collabShell(t, signedInTo(false, true), followedList, roadList, bandPractice)
	l := s.loads["spotify:playlists"].(*loaded[[]media.Playlist])
	l.err = partOf(2, 120)
	track := followedList.Tracks[0]
	choose := func(item string) error {
		t.Helper()
		if err := tt.RightClick(track.Title); err != nil {
			t.Fatal(err)
		}
		defer tt.CloseMenu()
		return tt.ChooseMenuItem("Add to Spotify playlist", item)
	}
	if err := choose(roadList.Title); err != nil {
		t.Fatal(err)
	}
	if err := choose("Showing 2 of 120 playlists · Try again"); err != nil {
		t.Fatal(err)
	}
	s.drain()
	tt.Frame()
	if l.asks != 1 {
		t.Errorf("Try again asked %d times", l.asks)
	}
}

// TestAddAShelfInPart offers in Add a shelf the playlists and artists of
// lists that came in part, and how many playlists of how many came, which
// asks for the list again.
func TestAddAShelfInPart(t *testing.T) {
	s := newTestShell(t)
	s.loads["accounts"] = signedInTo(false, true)
	playlists := &loaded[[]media.Playlist]{done: true, value: []media.Playlist{roadList}, err: partOf(1, 120)}
	s.loads["spotify:playlists"] = playlists
	s.loads["spotify:artists"] = &loaded[[]media.Artist]{done: true, err: partOf(1, 90),
		value: []media.Artist{{ID: "sp:dunes", Source: media.SourceSpotify, Name: "Dunes"}}}
	tt := homeWindow(t, s, nil)
	click(t, s, tt, "edit home")
	choose := func(path ...string) {
		t.Helper()
		click(t, s, tt, "add a shelf")
		if err := tt.ChooseMenuItem(path...); err != nil {
			t.Fatalf("choosing %q: %v", path, err)
		}
		s.drain()
		tt.Frame()
	}
	choose("An artist's new releases", "New from Dunes")
	choose("A Spotify playlist", roadList.Title)
	choose("A Spotify playlist", "Showing 1 of 120 playlists · Try again")
	if playlists.asks != 1 {
		t.Errorf("Try again asked %d times", playlists.asks)
	}
}

// TestPodcastLibraryInPart shows in Library › Podcasts the shows saved on
// Spotify of a list that came in part, says how many of how many, and
// tells a show of it as followed on its page.
func TestPodcastLibraryInPart(t *testing.T) {
	s := newTestShell(t)
	s.loads["accounts"] = signedInTo(false, true)
	sp := media.Podcast{ID: "sp:s1", Source: media.SourceSpotify, Title: "The Daily Thing", Authors: []media.Ref{}, Episodes: []media.Track{}}
	saved := &loaded[[]media.Podcast]{done: true, value: []media.Podcast{sp}, err: partOf(1, 120)}
	s.loads["spotify:podcasts"] = saved
	tt := testerOf(t, s, 1280, 1000)
	show(s, tt, "/library/podcasts")
	if !tt.HasText("Showing 1 of 120 podcasts") || !tt.HasText(sp.Title) || tt.HasText(saved.err.Error()) {
		t.Fatalf("Library › Podcasts shows %q", tt.Texts())
	}
	click(t, s, tt, "try again")
	if saved.asks != 1 {
		t.Errorf("Try again asked %d times", saved.asks)
	}

	s.loads["spotify:podcasts"] = &loaded[[]media.Podcast]{done: true, value: []media.Podcast{sp}, err: partOf(1, 120)}
	s.loads["podcast:sp:s1"] = &loaded[media.Podcast]{done: true, value: sp}
	show(s, tt, podcastPath("sp:s1"))
	if _, ok := tt.Find("following"); !ok {
		t.Errorf("a show saved on Spotify, of a list that came in part: %q", tt.Texts())
	}
}
