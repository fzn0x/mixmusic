package app

import (
	"testing"

	"github.com/egoist/mygo"
)

// TestSpotifyPlaysHereOn has MixMusic play Spotify itself on Windows and
// macOS, whose webviews decrypt Spotify's audio, with Widevine and with
// FairPlay, and not on Linux, whose WebKitGTK has no EME (#295).
func TestSpotifyPlaysHereOn(t *testing.T) {
	for goos, want := range map[string]bool{"windows": true, "darwin": true, "linux": false, "freebsd": false} {
		if got := playsSpotifyOn(goos); got != want {
			t.Errorf("playsSpotifyOn(%q) = %v, want %v", goos, got, want)
		}
	}
}

// TestHerePageRunsHiddenOnMacOS keeps the page of MixMusic's own Spotify
// device running in its hidden window on macOS, where WebKit slows the
// timers of a hidden page, and leaves the page as it is elsewhere: WebView2
// runs it, and its option is the whole browser's.
func TestHerePageRunsHiddenOnMacOS(t *testing.T) {
	if got := herePage("darwin"); got.BackgroundThrottling != mygo.ThrottlingOff || got.Autoplay != mygo.AutoplayUserGesture {
		t.Errorf("herePage(darwin) = throttling %v, autoplay %v, want ThrottlingOff and the webview's autoplay", got.BackgroundThrottling, got.Autoplay)
	}
	for _, goos := range []string{"windows", "linux"} {
		if got := herePage(goos); got.BackgroundThrottling != mygo.ThrottlingOn || got.Autoplay != mygo.AutoplayUserGesture {
			t.Errorf("herePage(%s) = throttling %v, autoplay %v, want the webview's own", goos, got.BackgroundThrottling, got.Autoplay)
		}
	}
}
