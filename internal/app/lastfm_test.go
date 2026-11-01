package app

import (
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/lastfm"
	"github.com/fzn0x/mixmusic/internal/media"
)

// TestLastfmTrack tells the scrobbler what plays: with the player's
// length, else the catalog's, and with none for a live stream, which the
// scrobbler then never scrobbles.
func TestLastfmTrack(t *testing.T) {
	track := &media.Track{ID: "yt:a", Title: "Song", Artists: []media.Ref{{Name: "Ann"}, {Name: "Bo"}},
		Album: &media.Ref{Name: "Album"}, DurationMs: 180000}
	for _, c := range []struct {
		name string
		np   NowPlaying
		want lastfm.Track
	}{
		{"the catalog's length", NowPlaying{Track: track}, lastfm.Track{Title: "Song", Artist: "Ann", Album: "Album", Duration: 3 * time.Minute}},
		{"the player's", NowPlaying{Track: track, DurationMs: 181500}, lastfm.Track{Title: "Song", Artist: "Ann", Album: "Album", Duration: 181500 * time.Millisecond}},
		{"live", NowPlaying{Track: track, Live: true, PositionMs: 3_600_000}, lastfm.Track{Title: "Song", Artist: "Ann", Album: "Album"}},
	} {
		if id, got := lastfmTrack(c.np); id != "yt:a" || got != c.want {
			t.Errorf("%s: %q, %+v; want %+v", c.name, id, got, c.want)
		}
	}
	if id, got := lastfmTrack(NowPlaying{}); id != "" || got != (lastfm.Track{}) {
		t.Errorf("nothing playing: %q, %+v", id, got)
	}

	// A live stream that played long is never scrobbled, where a track
	// is.
	for _, live := range []bool{false, true} {
		s := lastfm.NewScrobbler(nil, "a session")
		id, t2 := lastfmTrack(NowPlaying{Track: track, Live: live})
		now := time.Now()
		for i := range 100 {
			at := time.Duration(i) * 2 * time.Second
			s.Observe(now.Add(at), id, t2, true, at)
		}
		want := 1
		if live {
			want = 0
		}
		if n := len(s.Queue()); n != want {
			t.Errorf("live %v: %d scrobbles", live, n)
		}
	}
}
