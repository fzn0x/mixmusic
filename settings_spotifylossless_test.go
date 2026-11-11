package main

import (
	"strings"
	"testing"

	"github.com/fzn0x/mixmusic/internal/settings"
)

// TestSpotifyLosslessHint tells under Spotify tracks' setting how Spotify's
// lossless audio plays in each way they play, with the steps that turn it
// on, and the search of the settings finds it.
func TestSpotifyLosslessHint(t *testing.T) {
	s := newTestShell(t)
	tt := testerOf(t, s, 1280, 1800)
	for _, mode := range []settings.SpotifyPlayback{settings.SpotifyPlaybackHere, settings.SpotifyPlaybackConnect, settings.SpotifyPlaybackMatch} {
		st := s.svc.Preferences.Get()
		st.SpotifyPlayback = mode
		if _, err := s.svc.Preferences.Set(st); err != nil {
			t.Fatal(err)
		}
		s.refresh()
		show(s, tt, "/settings/playback")
		hint := spotifyLossless(mode)
		if !tt.HasText(hint) {
			t.Errorf("%s: Settings › Playback doesn't tell %q: %q", mode, hint, tt.Texts())
		}
		if connect := strings.Contains(hint, "plays only with On Spotify Connect"); connect == (mode == settings.SpotifyPlaybackConnect) {
			t.Errorf("%s: the hint %q", mode, hint)
		}
		if !strings.Contains(hint, "Settings and privacy › Media quality › Lossless") || !strings.Contains(hint, "Change quality settings › Lossless") {
			t.Errorf("%s: the hint doesn't tell how to turn Lossless on: %q", mode, hint)
		}
		if !strings.Contains(hint, "Premium Platinum in the countries where Spotify sells Lite, Standard and Platinum plans") {
			t.Errorf("%s: the hint doesn't tell the plan lossless needs: %q", mode, hint)
		}
	}
	show(s, tt, "/settings?q=lossless")
	until(t, s, tt, "the search", func() bool { return tt.HasText(spotifyLossless(settings.SpotifyPlaybackMatch)) })
}
