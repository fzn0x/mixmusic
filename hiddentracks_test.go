package main

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/app"
	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/spotify"
)

// TestHiddenTracksPage shows a Spotify playlist of others, whose tracks
// Spotify shows apps none of, with what Spotify told of it, Save and a way
// to open it on Spotify, rather than an error to try again (#530).
func TestHiddenTracksPage(t *testing.T) {
	s := newTestShell(t)
	s.loads["accounts"] = signedInTo(false, true)
	p := media.Playlist{ID: "sp:kites", Source: media.SourceSpotify, Title: "Northern Kites radio", Owner: "Bo Fieldfare",
		Description: "Songs for a windy day.", Saved: true, URL: "https://open.spotify.com/playlist/kites"}
	s.loads["playlist:"+p.ID] = &loaded[media.Playlist]{done: true, value: p, err: spotify.ErrPlaylistUnavailable}
	tt := tester(t, s)
	show(s, tt, playlistPath(p.ID))
	until(t, s, tt, "the page of "+p.Title, func() bool { return tt.HasText("open on Spotify") })
	for _, want := range []string{"Spotify · Bo Fieldfare", p.Description, "saved",
		"Spotify shows apps the tracks of the playlists you own or collaborate on only: open this one on Spotify to play it."} {
		if !tt.HasText(want) {
			t.Errorf("the page lacks %q: %q", want, tt.Texts())
		}
	}
	if tt.HasText("try again") || tt.HasText(spotify.ErrPlaylistUnavailable.Error()) {
		t.Errorf("the page shows an error: %q", tt.Texts())
	}
	if menu := pageMenu(t, tt); !slices.Contains(menu, "Open on Spotify") {
		t.Errorf("the menu is %q", menu)
	}

	// Another error is still one, with its way to try again.
	s.loads["playlist:"+p.ID] = &loaded[media.Playlist]{done: true, value: p, err: errors.New("spotify: HTTP 500")}
	tt.Frame()
	if !tt.HasText("try again") {
		t.Errorf("another error shows %q", tt.Texts())
	}
}

// TestHiddenTracksTile has the tile of a Spotify playlist of others, as
// the library lists those the user follows, neither play nor download it,
// and Home not offer it as a shelf (#530).
func TestHiddenTracksTile(t *testing.T) {
	s := newTestShell(t)
	s.loads["accounts"] = signedInTo(false, true)
	s.loads["spotify:playlists"] = &loaded[[]media.Playlist]{done: true, value: []media.Playlist{roadList, followedList}}
	s.loads["spotify:albums"] = &loaded[[]media.Album]{done: true}
	s.loads["spotify:artists"] = &loaded[[]media.Artist]{done: true}
	s.loads["spotify:liked"] = &loaded[app.TrackPage]{done: true}
	tt := testerOf(t, s, 1280, 1400)
	show(s, tt, "/library/spotify")
	for title, plays := range map[string]bool{roadList.Title: true, followedList.Title: false} {
		if err := tt.RightClick(title); err != nil {
			t.Fatal(err)
		}
		menu := tt.Menu()
		tt.CloseMenu()
		if !slices.Contains(menu, "Open") || slices.Contains(menu, "Play") != plays || slices.Contains(menu, "Download") != plays {
			t.Errorf("the menu of %s holds %q", title, menu)
		}
	}

	tt = homeWindow(t, s, nil)
	click(t, s, tt, "edit home")
	click(t, s, tt, "add a shelf")
	if err := tt.ChooseMenuItem("A Spotify playlist", followedList.Title); err == nil {
		t.Errorf("Home offers %s as a shelf", followedList.Title)
	}
	click(t, s, tt, "add a shelf")
	if err := tt.ChooseMenuItem("A Spotify playlist", "Spotify shows apps the tracks of your own and shared playlists only"); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Errorf("Home doesn't tell why: %v", err)
	}
	click(t, s, tt, "add a shelf")
	if err := tt.ChooseMenuItem("A Spotify playlist", roadList.Title); err != nil {
		t.Errorf("Home doesn't offer %s: %v", roadList.Title, err)
	}
}

// TestNotShownPage shows a playlist of Spotify's own, which Spotify tells
// apps nothing of, by what the user's library tells of it, or by its
// address alone, with a way to open it on Spotify (#530).
func TestNotShownPage(t *testing.T) {
	s := newTestShell(t)
	s.loads["accounts"] = signedInTo(false, true)
	daily := media.Playlist{ID: "sp:daily", Source: media.SourceSpotify, Title: "Daily Mix 1", Owner: "Spotify"}
	s.loads["spotify:playlists"] = &loaded[[]media.Playlist]{done: true, value: []media.Playlist{roadList, daily}}
	for id, title := range map[string]string{daily.ID: "Daily Mix 1", "sp:gone": "Spotify playlist"} {
		p := media.Playlist{ID: id, Source: media.SourceSpotify, URL: "https://open.spotify.com/playlist/" + id[3:]}
		s.loads["playlist:"+id] = &loaded[media.Playlist]{done: true, value: p, err: spotify.ErrPlaylistNotShown}
		tt := tester(t, s)
		show(s, tt, playlistPath(id))
		until(t, s, tt, "the page of "+title, func() bool { return tt.HasText("open on Spotify") })
		for _, want := range []string{strings.ToLower(title), "Spotify shows apps nothing of its own playlists, nor of those deleted or made private: open this one on Spotify to play it."} {
			if !tt.HasText(want) {
				t.Errorf("the page of %s lacks %q: %q", id, want, tt.Texts())
			}
		}
		if tt.HasText("try again") {
			t.Errorf("the page of %s shows an error: %q", id, tt.Texts())
		}
	}
}
