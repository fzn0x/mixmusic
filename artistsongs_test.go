package main

import (
	"context"
	"errors"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

func songsOf(ids ...string) []media.Track {
	var out []media.Track
	for _, id := range ids {
		out = append(out, media.Track{ID: id, Title: id, Source: media.SourceSpotify, Artists: []media.Ref{}})
	}
	return out
}

// TestPlayArtistStartsWithTheFirstRelease starts the queue with the songs
// of the first release, and adds those of the next to its end.
func TestPlayArtistStartsWithTheFirstRelease(t *testing.T) {
	s := newTestShell(t)
	a := media.Artist{ID: "sp:a1", Source: media.SourceSpotify, Name: "Dunes", Albums: []media.Album{{ID: "sp:al1"}}}
	if !artistPlaysAll(a) || artistPlaysAll(media.Artist{Source: media.SourceYouTube, Albums: a.Albums}) || artistPlaysAll(media.Artist{Source: media.SourceSpotify}) {
		t.Fatal("Play covers all songs of a Spotify artist with releases only")
	}
	done := make(chan struct{})
	s.playArtist(a, func(ctx context.Context, id string, each func([]media.Track)) ([]media.Track, error) {
		defer close(done)
		each(songsOf("sp:t1", "sp:t2"))
		each(songsOf("sp:t3"))
		return songsOf("sp:t1", "sp:t2", "sp:t3"), nil
	})
	<-done
	waitFor(t, s, func() bool { return len(s.p.State().Queue) == 3 })
	for i, want := range []string{"sp:t1", "sp:t2", "sp:t3"} {
		if got := s.p.State().Queue[i].Track.ID; got != want {
			t.Errorf("queue %d is %s, want %s", i, got, want)
		}
	}
}

// TestPlayArtistFallsBackToTopSongs plays their top songs, with a notice,
// when no release could be read.
func TestPlayArtistFallsBackToTopSongs(t *testing.T) {
	s := newTestShell(t)
	a := media.Artist{ID: "sp:a1", Source: media.SourceSpotify, Name: "Dunes", Albums: []media.Album{{ID: "sp:al1"}}, TopTracks: songsOf("sp:top")}
	done := make(chan struct{})
	s.playArtist(a, func(context.Context, string, func([]media.Track)) ([]media.Track, error) {
		defer close(done)
		return nil, errors.New("spotify: HTTP 503")
	})
	<-done
	waitFor(t, s, func() bool { return len(s.p.State().Queue) == 1 && s.p.State().Queue[0].Track.ID == "sp:top" })
}
