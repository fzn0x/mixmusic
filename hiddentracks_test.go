package main

import (
	"errors"
	"slices"
	"testing"

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
