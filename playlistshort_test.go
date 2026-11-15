package main

import (
	"strconv"
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
)

// shortPlaylist is a YouTube Music playlist read to its end whose header
// counts count songs while its pages held n (#627).
func shortPlaylist(id string, count, n int) *loaded[media.Playlist] {
	tracks := make([]media.Track, n)
	for i := range tracks {
		tracks[i] = testTracks[i%len(testTracks)]
		tracks[i].ID = "yt:short" + strconv.Itoa(i)
	}
	return &loaded[media.Playlist]{done: true, value: media.Playlist{ID: id, Source: media.SourceYouTube,
		Title: "Short", Owner: "Someone", TrackCount: count, Tracks: tracks}}
}

// TestShortYouTubePlaylistSaysSo tells a playlist that ends far short of
// its count, with Try again, and stays quiet for a small gap, a Spotify
// playlist and one that was read whole.
func TestShortYouTubePlaylistSaysSo(t *testing.T) {
	const id = "yt:PLshort"
	key := "playlist:" + id
	s := newTestShell(t)
	head := shortPlaylist(id, 1527, 100)
	s.loads[key] = head
	tt := testerOf(t, s, 1280, 1200)
	show(s, tt, playlistPath(id))
	want := "Only 100 of this playlist's 1,527 tracks came from YouTube Music."
	if !tt.HasText(want) || !tt.HasText("try again") {
		t.Fatalf("a playlist of 1,527 that ends at 100 shows %q", tt.Texts())
	}
	click(t, s, tt, "try again")
	if !head.again && s.loads[key] == head && head.asks == 0 {
		t.Error("try again didn't ask for the playlist again")
	}

	for _, c := range []struct {
		name     string
		count, n int
		source   media.Source
	}{
		{"a small gap", 125, 120, media.SourceYouTube},
		{"a count of 100", 100, 60, media.SourceYouTube},
		{"a whole playlist", 150, 150, media.SourceYouTube},
		{"Spotify", 1527, 100, media.SourceSpotify},
	} {
		other := shortPlaylist("yt:PLother", c.count, c.n)
		other.value.Source = c.source
		s.loads["playlist:yt:PLother"] = other
		show(s, tt, playlistPath("yt:PLother"))
		if tt.HasText("tracks came from YouTube Music") || tt.HasText("try again") {
			t.Errorf("%s shows %q", c.name, tt.Texts())
		}
	}
}
