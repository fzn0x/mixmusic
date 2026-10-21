package app

import (
	"testing"
	"time"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/nowplaying"
)

func TestControlsInfo(t *testing.T) {
	at := time.Unix(1000, 0)
	np := NowPlaying{
		Track: &media.Track{
			ID:         "yt:1",
			Title:      "Song",
			Artists:    []media.Ref{{Name: "Ann"}, {Name: "Bo"}},
			Album:      &media.Ref{Name: "Album"},
			DurationMs: 180000,
			Artwork:    []media.Image{{URL: "https://i/60", Width: 60}, {URL: "https://i/544", Width: 544}},
		},
		Playing:    true,
		PositionMs: 5000,
		Volume:     0.4,
	}
	info := controlsInfo(np, at)
	if info.ID != "yt:1" || info.Title != "Song" || info.Album != "Album" || len(info.Artists) != 2 || info.Artists[1] != "Bo" {
		t.Errorf("the track: %+v", info)
	}
	if info.Artwork != "https://i/544" {
		t.Errorf("artwork %q", info.Artwork)
	}
	if info.Duration != 180*time.Second || info.Position != 5*time.Second || !info.At.Equal(at) || !info.Playing || info.Volume != 0.4 {
		t.Errorf("the playback: %+v", info)
	}
	// The player's duration, once it knows it, over the catalog's.
	np.DurationMs = 181500
	if info := controlsInfo(np, at); info.Duration != 181500*time.Millisecond {
		t.Errorf("duration %v", info.Duration)
	}
	np.Track.Album = nil
	if info := controlsInfo(np, at); info.Album != "" {
		t.Errorf("album %q", info.Album)
	}
}

func TestControlCommands(t *testing.T) {
	// Raise, Seek and SetVolume have a case of their own.
	for _, c := range []nowplaying.Command{nowplaying.Play, nowplaying.Pause, nowplaying.Toggle, nowplaying.Next, nowplaying.Previous} {
		if _, ok := controlCommands[c]; !ok {
			t.Errorf("no player command for %v", c)
		}
	}
}
