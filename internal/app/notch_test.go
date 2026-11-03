package app

import (
	"testing"

	"github.com/fzn0x/mixmusic/internal/media"
	"github.com/fzn0x/mixmusic/internal/player"
)

// TestNotchShows shows the player around the notch while the setting is
// on, the main window is open and a track is loaded, and else not: it
// alone never keeps MixMusic running.
func TestNotchShows(t *testing.T) {
	loaded := player.State{Queue: []player.Item{{Track: media.Track{ID: "yt:1"}}}, Index: 0}
	ended := player.State{Index: -1}
	for _, c := range []struct {
		name     string
		on, main bool
		st       player.State
		want     bool
	}{
		{"a track loaded", true, true, loaded, true},
		{"a track paused", true, true, player.State{Queue: loaded.Queue, Index: 0, Playing: false}, true},
		{"nothing to play", true, true, ended, false},
		{"turned off", false, true, loaded, false},
		{"the main window closed", true, false, loaded, false},
	} {
		if got := notchShows(c.on, c.main, c.st); got != c.want {
			t.Errorf("%s: shows %v", c.name, got)
		}
	}
}
